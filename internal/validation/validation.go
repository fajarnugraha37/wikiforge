package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/pathutil"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
	"github.com/fajarnugraha37/wikiforge/internal/prompts"
)

var (
	linkRE         = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	mermaidRE      = regexp.MustCompile("(?s)```mermaid\\s*\\n(.*?)```")
	backtickPathRE = regexp.MustCompile("`([^`]+[/\\\\][^`]+|[^`]+\\.[A-Za-z0-9]{1,12}(?::[0-9]+)?)`")
	lineSuffixRE   = regexp.MustCompile(`(?i)(?::[0-9]+(?:-[0-9]+)?|#L[0-9]+(?:-L[0-9]+)?)$`)
	privateKeyRE   = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	credentialRE   = regexp.MustCompile(`(?im)\b(api[_-]?key|access[_-]?token|password|client[_-]?secret)\s*[:=]\s*["']?([A-Za-z0-9_./+=-]{12,})`)
	progressMu     sync.Mutex
)

type Validator struct {
	Config config.Config
}

type mermaidRenderJob struct {
	rel     string
	index   int
	total   int
	content string
}

func (v Validator) ValidateAdaptiveComponent(ctx context.Context, component config.ComponentConfig, plan planner.ComponentPlan) model.ValidationResult {
	contracts := map[string]model.PageContract{}
	expected := make([]string, 0, len(plan.Pages))
	for _, page := range plan.Pages {
		expected = append(expected, filepath.ToSlash(page.Path))
		contracts[filepath.ToSlash(page.Path)] = prompts.AdaptivePageContract(page.Path, string(page.Kind))
	}
	result := v.validate(ctx, component.ID, component.DocumentationRoot(), component.WorkDir(), expected, contracts, "adaptive/"+plan.Profile, 0, 0)
	validateHierarchy(component.ID, component.DocumentationRoot(), plan.Pages, &result)
	return finalizeValidationResult(result, v.Config.Documentation.MinimumQualityScore)
}

func (v Validator) ValidateAdaptiveSystem(ctx context.Context, root string, plan planner.SystemPlan) model.ValidationResult {
	contracts := map[string]model.PageContract{}
	expected := make([]string, 0, len(plan.Pages))
	for _, page := range plan.Pages {
		expected = append(expected, filepath.ToSlash(page.Path))
		contracts[filepath.ToSlash(page.Path)] = prompts.AdaptivePageContract(page.Path, string(page.Kind))
	}
	result := v.validate(ctx, "system", root, filepath.Dir(root), expected, contracts, "adaptive/system", 0, 0)
	validateHierarchy("system", root, plan.Pages, &result)
	return finalizeValidationResult(result, v.Config.Documentation.MinimumQualityScore)
}

func (v Validator) validate(ctx context.Context, targetID, root, sourceBase string, expected []string, contracts map[string]model.PageContract, profile string, profileMinimumPages, profileMinimumMermaid int) model.ValidationResult {
	result := model.ValidationResult{Root: root, Profile: profile, Score: 100}
	add := func(code, severity, path, message string) {
		result.Findings = append(result.Findings, model.Finding{Code: code, Severity: severity, Path: path, Message: message})
	}

	for _, rel := range expected {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			add("DOC-MISSING", "error", rel, "required canonical document is missing")
		}
	}

	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	result.MarkdownFiles = len(files)
	validationProgress(targetID, "validation pass started | profile=%s | files=%d | expected=%d | mermaid-mode=%s", profile, len(files), len(expected), v.Config.Mermaid.Mode)
	defer func(started time.Time) {
		validationProgress(targetID, "validation pass returned | elapsed=%s", compactValidationDuration(time.Since(started)))
	}(time.Now())

	minimumPages := max(profileMinimumPages, len(expected))
	if v.Config.Documentation.MinimumPages > minimumPages {
		minimumPages = v.Config.Documentation.MinimumPages
	}
	if len(files) < minimumPages {
		add("DOC-PAGE-COUNT", "error", root, fmt.Sprintf("found %d Markdown pages; minimum for profile %s is %d", len(files), profile, minimumPages))
	}

	contractByOutput := map[string]model.PageContract{}
	for path, contract := range contracts {
		contractByOutput[filepath.ToSlash(path)] = contract
	}
	var mermaidJobs []mermaidRenderJob

	for fileIndex, path := range files {
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		validationProgress(targetID, "checking file %d/%d | %s", fileIndex+1, len(files), rel)
		b, err := os.ReadFile(path)
		if err != nil {
			add("DOC-READ", "error", rel, err.Error())
			continue
		}
		content := string(b)
		if rel == "INSTRUCTIONS.md" {
			continue
		}
		if v.Config.Documentation.RequireFrontMatter {
			front, keys := parseFrontMatter(content)
			for _, key := range []string{"type", "title", "description", "tags"} {
				if strings.TrimSpace(front[key]) == "" {
					add("DOC-FRONTMATTER", "error", rel, "missing front matter field: "+key)
				}
			}
			allowedFrontmatter := map[string]bool{"type": true, "title": true, "description": true, "resource": true, "tags": true}
			for _, key := range keys {
				allowed := allowedFrontmatter[key]
				if v.Config.Documentation.FrontMatterPolicy == "namespaced" && (key == "wikiforge" || strings.HasPrefix(key, "wikiforge.")) {
					allowed = true
				}
				if !allowed {
					add("DOC-FRONTMATTER-UNSUPPORTED", "error", rel, "unsupported OpenWiki front matter field: "+key)
				}
			}
		}
		upper := strings.ToUpper(content)
		if strings.Contains(upper, "TODO") || strings.Contains(upper, "TBD") {
			add("DOC-PLACEHOLDER", "warning", rel, "contains TODO/TBD; use an explicit Knowledge Gaps entry instead")
		}
		if privateKeyRE.MatchString(content) || credentialValuePresent(content) {
			add("SECRET-LEAK", "error", rel, "generated documentation contains a private key or credential-like value")
		}
		if v.Config.Documentation.RequireSourceReferences {
			sourceSection := sectionContent(content, "Source References")
			if sourceSection == "" {
				add("DOC-SOURCE-SECTION", "error", rel, "missing or empty ## Source References section")
			} else if !backtickPathRE.MatchString(sourceSection) && !linkRE.MatchString(sourceSection) {
				add("DOC-SOURCE-EMPTY", "warning", rel, "Source References section has no recognizable path or link")
			} else if v.Config.Documentation.ValidateSourcePaths {
				for _, ref := range extractSourcePaths(sourceSection) {
					if !sourceReferenceExists(sourceBase, path, ref) {
						severity := "warning"
						code := "DOC-SOURCE-PATH"
						if v.Config.Documentation.RequireVerifiedEvidence && strings.Contains(strings.ToLower(content), "verified") {
							severity = "error"
							code = "DOC-SOURCE-VERIFIED-UNRESOLVED"
						}
						add(code, severity, rel, "source reference does not resolve in the configured scope: "+ref)
					}
				}
			}
		}
		for _, m := range linkRE.FindAllStringSubmatch(content, -1) {
			target := strings.TrimSpace(strings.Split(m[1], "#")[0])
			if target == "" || isExternalReference(target) {
				continue
			}
			var resolved string
			if strings.HasPrefix(target, "/") {
				resolved = filepath.Clean(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(target, "/"))))
			} else {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			}
			if _, err := os.Stat(resolved); err != nil {
				code := "DOC-BROKEN-LINK"
				if strings.HasPrefix(target, "/") {
					code = "DOC-BROKEN-ABSOLUTE-LINK"
				}
				add(code, "error", rel, "broken internal link: "+target)
			}
		}

		if contract, ok := contractByOutput[rel]; ok {
			for _, heading := range contract.RequiredHeadings {
				if !hasHeading(content, heading) {
					add("DOC-REQUIRED-SECTION", "error", rel, "missing required section: "+heading)
				} else if strings.TrimSpace(sectionContent(content, heading)) == "" {
					add("DOC-EMPTY-SECTION", "error", rel, "required section is empty: "+heading)
				}
			}
			if contract.RequiredTableHeader != "" {
				if !strings.Contains(content, contract.RequiredTableHeader) {
					add("DOC-REQUIRED-TABLE", "error", rel, "missing required catalog table header: "+contract.RequiredTableHeader)
				} else if !catalogHasDataRow(content, contract.RequiredTableHeader) {
					add("DOC-CATALOG-EMPTY", "error", rel, "catalog must contain at least one evidence-backed or explicit Not Observed/Unknown row")
				} else {
					rows, bytes := catalogTableStats(content, contract.RequiredTableHeader)
					if contract.MaximumRowsPerShard > 0 && rows > contract.MaximumRowsPerShard {
						add("CATALOG-ROW-LIMIT", "error", rel, fmt.Sprintf("catalog contains %d data rows; maximum is %d", rows, contract.MaximumRowsPerShard))
					}
					if contract.MaximumBytes > 0 && bytes > contract.MaximumBytes {
						add("CATALOG-BYTE-LIMIT", "error", rel, fmt.Sprintf("catalog table is %d bytes; maximum is %d", bytes, contract.MaximumBytes))
					}
				}
			}
			adaptiveSemantic := strings.HasPrefix(profile, "adaptive/") || profile == "adaptive/system"
			semanticFindings := validateSemanticTables(rel, content, adaptiveSemantic && v.Config.Documentation.RequireCatalogIdentity, adaptiveSemantic && v.Config.Documentation.RequireRelationshipEvidence)
			for _, finding := range semanticFindings {
				add(finding.Code, finding.Severity, finding.Path, finding.Message)
			}
		}

		blocks := mermaidRE.FindAllStringSubmatch(content, -1)
		result.MermaidBlocks += len(blocks)
		allowed := toSet(v.Config.Documentation.AllowedDiagramTypes)
		for i, block := range blocks {
			typeName := firstMermaidToken(block[1])
			if !diagramAllowed(typeName, allowed) {
				add("MERMAID-TYPE", "error", rel, fmt.Sprintf("diagram %d uses unsupported or missing type %q", i+1, typeName))
			}
			if err := basicMermaidCheck(block[1]); err != nil {
				add("MERMAID-BASIC", "error", rel, fmt.Sprintf("diagram %d: %v", i+1, err))
			}
			if v.Config.Mermaid.Mode == "render" {
				mermaidJobs = append(mermaidJobs, mermaidRenderJob{rel: rel, index: i, total: len(blocks), content: block[1]})
			}
		}

		if contract, ok := contractByOutput[rel]; ok && contract.RequiredDiagram != "" {
			if len(blocks) == 0 {
				add("MERMAID-REQUIRED", "error", rel, "required Mermaid diagram is missing")
			} else if contract.RequiredDiagram != "any" {
				found := false
				for _, block := range blocks {
					actual := firstMermaidToken(block[1])
					if actual == contract.RequiredDiagram || (contract.RequiredDiagram == "flowchart" && actual == "flowchart") {
						found = true
					}
				}
				if !found {
					add("MERMAID-CONTRACT", "error", rel, "required diagram type not found: "+contract.RequiredDiagram)
				}
			}
		}
	}
	for _, result := range v.renderMermaidJobs(ctx, targetID, mermaidJobs) {
		if result.err != nil {
			add("MERMAID-RENDER", "error", result.job.rel, fmt.Sprintf("diagram %d failed to render: %v", result.job.index+1, result.err))
		}
	}

	minimumMermaid := profileMinimumMermaid
	if v.Config.Documentation.MinimumMermaidBlocks > minimumMermaid {
		minimumMermaid = v.Config.Documentation.MinimumMermaidBlocks
	}
	if v.Config.Documentation.RequireMermaid && result.MermaidBlocks < minimumMermaid {
		add("MERMAID-COUNT", "error", root, fmt.Sprintf("found %d Mermaid blocks; minimum for profile %s is %d", result.MermaidBlocks, profile, minimumMermaid))
	}

	relationshipPath := filepath.Join(root, "knowledge", "relationships.md")
	if b, err := os.ReadFile(relationshipPath); err == nil {
		content := string(b)
		if !strings.Contains(content, "| Subject | Relationship | Object |") {
			add("GRAPH-RELATIONSHIP-TABLE", "error", "knowledge/relationships.md", "missing standardized Subject/Relationship/Object table")
		}
	}

	errorsCount := 0
	for _, f := range result.Findings {
		if f.Severity == "error" {
			errorsCount++
		}
	}
	setValidationDimensions(&result)
	result.Score = dimensionAverage(result.Dimensions)
	result.Accepted = errorsCount == 0 && result.Score >= v.Config.Documentation.MinimumQualityScore
	validationProgress(targetID, "validation pass completed | accepted=%t | score=%d | findings=%d | mermaid=%d", result.Accepted, result.Score, len(result.Findings), result.MermaidBlocks)
	return result
}

func credentialValuePresent(content string) bool {
	for _, match := range credentialRE.FindAllStringSubmatch(content, -1) {
		value := strings.ToLower(strings.TrimSpace(match[2]))
		if value == "replace-me" || value == "change-me" || value == "example-value" || strings.Contains(value, "example") || strings.Contains(value, "<") {
			continue
		}
		return true
	}
	return false
}

func validateHierarchy(targetID, root string, pages []planner.PlannedPage, result *model.ValidationResult) {
	add := func(path, message string) {
		result.Findings = append(result.Findings, model.Finding{Code: "DOC-NAVIGATION", Severity: "error", Path: path, Message: message})
	}
	pageByPath := map[string]planner.PlannedPage{}
	for _, page := range pages {
		page.Path = filepath.ToSlash(filepath.Clean(page.Path))
		pageByPath[page.Path] = page
	}
	linksFor := func(from string) map[string]bool {
		out := map[string]bool{}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from)))
		if err != nil {
			return out
		}
		for _, match := range linkRE.FindAllStringSubmatch(string(b), -1) {
			target := strings.TrimSpace(strings.Split(match[1], "#")[0])
			if target == "" || isExternalReference(target) {
				continue
			}
			candidate := filepath.Join(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(target))
			if strings.HasPrefix(target, "/") {
				candidate = filepath.FromSlash(strings.TrimPrefix(target, "/"))
			}
			resolved, err := filepath.Rel(root, filepath.Join(root, candidate))
			if err != nil {
				continue
			}
			out[filepath.ToSlash(filepath.Clean(resolved))] = true
		}
		return out
	}
	for path, page := range pageByPath {
		if page.Kind != planner.PageIndex && page.Kind != planner.PageCollection {
			continue
		}
		children := directNavigationChildren(path, page, pageByPath)
		if len(children) == 0 {
			continue
		}
		links := linksFor(path)
		for _, child := range children {
			if !links[child] {
				add(path, "index must link its direct child: "+child)
			}
		}
		if path == "quickstart.md" {
			for linked := range links {
				if child, ok := pageByPath[linked]; ok && child.Kind != planner.PageIndex {
					add(path, "root quickstart must link view indexes, not leaf pages: "+linked)
				}
			}
		}
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || d.Name() == "INSTRUCTIONS.md" || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, planned := pageByPath[rel]; !planned {
			add(rel, "Markdown page is not part of the planned documentation hierarchy")
		}
		return nil
	})
	_ = targetID
}

func directNavigationChildren(path string, page planner.PlannedPage, pages map[string]planner.PlannedPage) []string {
	var children []string
	if path == "quickstart.md" {
		for candidate, child := range pages {
			if child.Kind == planner.PageIndex && candidate != path && strings.Count(candidate, "/") == 1 {
				children = append(children, candidate)
			}
		}
	} else if page.Kind == planner.PageCollection {
		base := strings.TrimSuffix(path, "/index.md")
		for candidate, child := range pages {
			if child.Kind == planner.PageShard && filepath.ToSlash(filepath.Dir(candidate)) == base {
				children = append(children, candidate)
			}
		}
	} else {
		base := strings.TrimSuffix(path, "/index.md")
		for candidate := range pages {
			if candidate == path {
				continue
			}
			candidateDir := filepath.ToSlash(filepath.Dir(candidate))
			if candidateDir == base {
				children = append(children, candidate)
				continue
			}
			if strings.Count(path, "/") == 1 && page.Kind == planner.PageIndex && childIsUnitOrCollectionIndex(candidate, base, pages) {
				children = append(children, candidate)
			}
		}
	}
	sort.Strings(children)
	return children
}

func childIsUnitOrCollectionIndex(candidate, base string, pages map[string]planner.PlannedPage) bool {
	if !strings.HasPrefix(candidate, base+"/") || !strings.HasSuffix(candidate, "/index.md") {
		return false
	}
	child, ok := pages[candidate]
	return ok && child.Kind == planner.PageIndex || ok && child.Kind == planner.PageCollection
}

func finalizeValidationResult(result model.ValidationResult, minimumScore int) model.ValidationResult {
	errorsCount := 0
	for _, finding := range result.Findings {
		if finding.Severity == "error" {
			errorsCount++
		}
	}
	setValidationDimensions(&result)
	result.Score = dimensionAverage(result.Dimensions)
	result.Accepted = errorsCount == 0 && result.Score >= minimumScore
	return result
}

// Recalculate applies the same acceptance policy after external findings are added.
func Recalculate(result model.ValidationResult, minimumScore int) model.ValidationResult {
	return finalizeValidationResult(result, minimumScore)
}

func Strict(result model.ValidationResult, minimumScore int) model.ValidationResult {
	for index := range result.Findings {
		if result.Findings[index].Severity == "warning" {
			result.Findings[index].Severity = "error"
		}
	}
	return finalizeValidationResult(result, minimumScore)
}

func ValidateEvidenceBacked(root string, index evidence.Index) []model.Finding {
	findings := evidenceFindings(root, index)
	return findings
}

func evidenceFindings(root string, index evidence.Index) []model.Finding {
	depsByPage := map[string]int{}
	knownEvidence := map[string]bool{}
	var findings []model.Finding
	for _, ref := range index.References {
		knownEvidence[ref.ID] = true
	}
	for _, dep := range index.Dependencies {
		depsByPage[dep.PageID]++
		if !knownEvidence[dep.EvidenceID] {
			findings = append(findings, model.Finding{Code: "EVIDENCE-BROKEN", Severity: "error", Path: dep.PageID, Message: "evidence dependency does not resolve: " + dep.EvidenceID})
		}
	}
	conceptOwners := map[string]string{}
	verifiedRE := regexp.MustCompile(`(?i)\bVerified\b`)
	conceptRE := regexp.MustCompile("(?im)^\\s*(concept\\s+id|id)\\s*:\\s*`?([A-Za-z0-9._/-]+)`?\\s*$")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") || d.Name() == "INSTRUCTIONS.md" {
			return nil
		}
		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		pageID := evidence.CanonicalPath(rel)
		content := string(contentBytes)
		if verifiedRE.MatchString(content) && depsByPage[pageID] == 0 {
			findings = append(findings, model.Finding{Code: "EVIDENCE-UNRESOLVED", Severity: "error", Path: filepath.ToSlash(rel), Message: "Verified claim has no resolvable evidence dependency"})
		}
		for _, match := range conceptRE.FindAllStringSubmatch(content, -1) {
			conceptID := strings.ToLower(match[2])
			if owner, exists := conceptOwners[conceptID]; exists && owner != filepath.ToSlash(rel) {
				findings = append(findings, model.Finding{Code: "GRAPH-DUPLICATE-CONCEPT", Severity: "error", Path: filepath.ToSlash(rel), Message: "duplicate concept identity " + conceptID + "; first declared in " + owner})
			} else {
				conceptOwners[conceptID] = filepath.ToSlash(rel)
			}
		}
		findings = append(findings, catalogIdentityFindings(filepath.ToSlash(rel), content)...)
		return nil
	})
	return findings
}

func catalogIdentityFindings(path, content string) []model.Finding {
	var findings []model.Finding
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.Contains(strings.ToLower(trimmed), " id ") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) == 0 || !strings.Contains(strings.ToLower(strings.TrimSpace(cells[0])), "id") {
			continue
		}
		for _, row := range lines[i+2:] {
			row = strings.TrimSpace(row)
			if !strings.HasPrefix(row, "|") {
				break
			}
			rowCells := strings.Split(strings.Trim(row, "|"), "|")
			if len(rowCells) == 0 || strings.Trim(strings.TrimSpace(rowCells[0]), "-:") == "" {
				continue
			}
			id := strings.TrimSpace(rowCells[0])
			if id == "" || strings.EqualFold(id, "unknown") || strings.EqualFold(id, "not observed") {
				continue
			}
			if seen[id] {
				findings = append(findings, model.Finding{Code: "CATALOG-DUPLICATE-ID", Severity: "error", Path: path, Message: "duplicate catalog entry ID: " + id})
			}
			seen[id] = true
		}
	}
	return findings
}

func setValidationDimensions(result *model.ValidationResult) {
	dimensions := map[string]model.DimensionResult{}
	counts := map[string][2]int{}
	for _, finding := range result.Findings {
		name := validationDimension(finding.Code)
		entry := dimensions[name]
		entry.FindingCodes = append(entry.FindingCodes, finding.Code)
		dimensions[name] = entry
		count := counts[name]
		if finding.Severity == "error" {
			count[0]++
		} else {
			count[1]++
		}
		counts[name] = count
	}
	for _, name := range []string{"structure", "navigation", "evidence", "coverage", "semantic consistency", "catalog integrity", "graph integrity", "security", "freshness", "diagrams"} {
		entry := dimensions[name]
		entry.FindingCodes = uniqueStrings(entry.FindingCodes)
		count := counts[name]
		denominator := result.MarkdownFiles
		if denominator < 1 {
			denominator = 1
		}
		penalty := ((count[0] * 100) + (count[1] * 50)) / denominator
		entry.Score = max(0, 100-penalty)
		dimensions[name] = entry
	}
	result.Dimensions = dimensions
}

func dimensionAverage(dimensions map[string]model.DimensionResult) int {
	if len(dimensions) == 0 {
		return 100
	}
	total := 0
	for _, dimension := range dimensions {
		total += dimension.Score
	}
	return total / len(dimensions)
}

func validationDimension(code string) string {
	switch {
	case strings.HasPrefix(code, "DOC-NAVIGATION") || strings.Contains(code, "LINK"):
		return "navigation"
	case strings.HasPrefix(code, "EVIDENCE-") || strings.HasPrefix(code, "DOC-SOURCE"):
		return "evidence"
	case strings.HasPrefix(code, "CATALOG-") || strings.HasPrefix(code, "DOC-REQUIRED-TABLE"):
		return "catalog integrity"
	case strings.HasPrefix(code, "GRAPH-"):
		return "graph integrity"
	case strings.HasPrefix(code, "MERMAID-"):
		return "diagrams"
	case strings.Contains(code, "SECURITY") || strings.HasPrefix(code, "SECRET-"):
		return "security"
	case strings.HasPrefix(code, "COVERAGE-"):
		return "coverage"
	default:
		return "structure"
	}
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func parseFrontMatter(content string) (map[string]string, []string) {
	out := map[string]string{}
	var keys []string
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return out, keys
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	current := ""
	seen := map[string]bool{}
	for i := 1; i < len(lines); i++ {
		rawLine := lines[i]
		line := strings.TrimSpace(rawLine)
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "-") && current != "" {
			out[current] = strings.TrimSpace(out[current] + " " + strings.TrimPrefix(line, "-"))
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			indent := len(rawLine) - len(strings.TrimLeft(rawLine, " \t"))
			if indent > 0 && current != "" && current != "tags" {
				key = current + "." + key
			}
			val := strings.Trim(strings.TrimSpace(line[idx+1:]), "\"'")
			out[key] = val
			current = key
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	return out, keys
}

func validateSemanticTables(path, content string, requireIdentity, requireEvidence bool) []model.Finding {
	if !requireIdentity && !requireEvidence {
		return nil
	}
	var findings []model.Finding
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") || !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "|") || !strings.Contains(lines[i+1], "---") {
			continue
		}
		headers := tableCells(lines[i])
		if len(headers) == 0 {
			continue
		}
		lower := make([]string, len(headers))
		identityIndex, evidenceIndex := -1, -1
		for index, header := range headers {
			lower[index] = strings.ToLower(strings.TrimSpace(header))
			if identityIndex < 0 && (strings.Contains(lower[index], "id") || lower[index] == "key") {
				identityIndex = index
			}
			if evidenceIndex < 0 && strings.Contains(lower[index], "evidence") {
				evidenceIndex = index
			}
		}
		relationshipTable := len(headers) >= 3 && strings.Contains(lower[0], "subject") && strings.Contains(lower[1], "relationship") && strings.Contains(lower[2], "object")
		seen := map[string]bool{}
		for rowIndex := i + 2; rowIndex < len(lines); rowIndex++ {
			trimmed := strings.TrimSpace(lines[rowIndex])
			if !strings.HasPrefix(trimmed, "|") {
				break
			}
			cells := tableCells(trimmed)
			if len(cells) < len(headers) {
				continue
			}
			if identityIndex >= 0 && requireIdentity {
				id := strings.TrimSpace(cells[identityIndex])
				if id == "" || strings.EqualFold(id, "unknown") || strings.EqualFold(id, "not observed") {
					continue
				}
				if seen[id] {
					findings = append(findings, model.Finding{Code: "CATALOG-DUPLICATE-ID", Severity: "error", Path: path, Message: "duplicate catalog entry ID: " + id})
				}
				seen[id] = true
			}
			if evidenceIndex >= 0 && requireEvidence {
				if strings.TrimSpace(cells[evidenceIndex]) == "" && !rowIsUnknown(cells) {
					findings = append(findings, model.Finding{Code: "EVIDENCE-CATALOG-ROW", Severity: "error", Path: path, Message: "catalog row has no evidence reference"})
				}
			}
			if relationshipTable {
				relationship := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(cells[1]), " ", "_"))
				if !controlledRelationship(relationship) {
					findings = append(findings, model.Finding{Code: "GRAPH-RELATIONSHIP-VOCAB", Severity: "error", Path: path, Message: "unsupported relationship vocabulary: " + relationship})
				}
				if requireEvidence && len(cells) < 6 {
					findings = append(findings, model.Finding{Code: "GRAPH-RELATIONSHIP-EVIDENCE", Severity: "error", Path: path, Message: "relationship row must include evidence, authority, and confidence columns"})
				}
			}
		}
	}
	return findings
}

var controlledRelationshipVocabulary = map[string]bool{
	"AUTHORIZES": true, "CALLS": true, "CONSUMES": true, "DEPENDS_ON": true, "EXPOSES": true,
	"EXTENDS": true, "IMPLEMENTS": true, "IMPORTS": true, "LINKS_TO": true, "OWNS": true,
	"PART_OF": true, "PERSISTS": true, "PRODUCES": true, "REFERENCES": true, "RELATED_TO": true,
	"VALIDATES": true,
}

func controlledRelationship(value string) bool { return controlledRelationshipVocabulary[value] }

func tableCells(line string) []string {
	line = strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func rowIsUnknown(cells []string) bool {
	for _, cell := range cells {
		value := strings.ToLower(strings.TrimSpace(cell))
		if value == "unknown" || value == "not observed" || value == "not applicable" {
			return true
		}
	}
	return false
}

func hasHeading(content, heading string) bool {
	needle := "## " + strings.TrimSpace(heading)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == needle {
			return true
		}
	}
	return false
}

func sectionContent(content, heading string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	needle := "## " + heading
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == needle {
			start = i + 1
			continue
		}
		if start >= 0 && strings.HasPrefix(strings.TrimSpace(line), "## ") {
			return strings.TrimSpace(strings.Join(lines[start:i], "\n"))
		}
	}
	if start >= 0 {
		return strings.TrimSpace(strings.Join(lines[start:], "\n"))
	}
	return ""
}

func extractSourcePaths(section string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range backtickPathRE.FindAllStringSubmatch(section, -1) {
		ref := strings.TrimSpace(m[1])
		if ref == "" || seen[ref] || isExternalReference(ref) {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	for _, m := range linkRE.FindAllStringSubmatch(section, -1) {
		ref := strings.TrimSpace(strings.Split(m[1], "#")[0])
		if ref == "" || seen[ref] || isExternalReference(ref) {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

func sourceReferenceExists(sourceBase, documentPath, ref string) bool {
	ref = strings.TrimSpace(ref)
	ref = lineSuffixRE.ReplaceAllString(ref, "")
	ref = strings.TrimPrefix(ref, "./")
	if ref == "" || strings.ContainsAny(ref, "{}") {
		return true
	}
	candidates := []string{
		filepath.Join(sourceBase, filepath.FromSlash(ref)),
		filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(ref)),
	}
	for _, candidate := range candidates {
		if strings.ContainsAny(candidate, "*?") {
			matches, _ := filepath.Glob(candidate)
			if len(matches) > 0 {
				return true
			}
			continue
		}
		if _, err := os.Stat(filepath.Clean(candidate)); err == nil {
			return true
		}
	}
	return false
}

func isExternalReference(target string) bool {
	lower := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "urn:")
}

func catalogHasDataRow(content, header string) bool {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != strings.TrimSpace(header) {
			continue
		}
		seenSeparator := false
		for _, candidate := range lines[i+1:] {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" {
				if seenSeparator {
					break
				}
				continue
			}
			if !strings.HasPrefix(trimmed, "|") {
				break
			}
			if !seenSeparator {
				seenSeparator = strings.Contains(trimmed, "---")
				continue
			}
			if strings.Count(trimmed, "|") >= 2 {
				return true
			}
		}
	}
	return false
}

func catalogTableStats(content, header string) (rows, bytes int) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != strings.TrimSpace(header) {
			continue
		}
		seenSeparator := false
		bytes += len([]byte(line)) + 1
		for _, candidate := range lines[i+1:] {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" {
				if seenSeparator {
					break
				}
				continue
			}
			if !strings.HasPrefix(trimmed, "|") {
				break
			}
			bytes += len([]byte(candidate)) + 1
			if !seenSeparator {
				seenSeparator = strings.Contains(trimmed, "---")
				continue
			}
			if strings.Count(trimmed, "|") >= 2 {
				rows++
			}
		}
		return rows, bytes
	}
	return 0, 0
}

func firstMermaidToken(block string) string {
	for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%%") || strings.HasPrefix(line, "---") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return ""
		}
		if parts[0] == "flowchart" {
			return "flowchart"
		}
		return parts[0]
	}
	return ""
}

func diagramAllowed(t string, allowed map[string]bool) bool {
	return allowed[t]
}

func basicMermaidCheck(block string) error {
	trim := strings.TrimSpace(block)
	if trim == "" {
		return fmt.Errorf("empty diagram")
	}
	pairs := [][2]string{{"[", "]"}, {"(", ")"}}
	for _, p := range pairs {
		if strings.Count(trim, p[0]) != strings.Count(trim, p[1]) {
			return fmt.Errorf("unbalanced %s%s delimiters", p[0], p[1])
		}
	}
	lines := strings.Split(trim, "\n")
	if len(lines) > 250 {
		return fmt.Errorf("diagram exceeds 250 lines")
	}
	for i, line := range lines {
		if len(line) > 500 {
			return fmt.Errorf("diagram line %d exceeds 500 characters", i+1)
		}
	}
	return nil
}

type mermaidRenderResult struct {
	job mermaidRenderJob
	err error
}

type boundedMermaidOutput struct {
	data []byte
}

func (b *boundedMermaidOutput) Write(p []byte) (int, error) {
	const maxBytes = 64 * 1024
	if len(p) >= maxBytes {
		b.data = append(b.data[:0], p[len(p)-maxBytes:]...)
		return len(p), nil
	}
	if len(b.data)+len(p) > maxBytes {
		remove := len(b.data) + len(p) - maxBytes
		b.data = append(b.data[:0], b.data[remove:]...)
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *boundedMermaidOutput) String() string { return string(b.data) }

func (v Validator) renderMermaidJobs(ctx context.Context, targetID string, jobs []mermaidRenderJob) []mermaidRenderResult {
	if len(jobs) == 0 {
		return nil
	}
	workers := v.Config.Mermaid.MaxWorkers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	results := make([]mermaidRenderResult, len(jobs))
	jobCh := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobCh {
				job := jobs[index]
				results[index] = mermaidRenderResult{job: job, err: v.renderMermaid(ctx, targetID, job.rel, job.index, job.total, job.content)}
			}
		}()
	}
	for index := range jobs {
		select {
		case jobCh <- index:
		case <-ctx.Done():
			results[index] = mermaidRenderResult{job: jobs[index], err: ctx.Err()}
		}
	}
	close(jobCh)
	wg.Wait()
	return results
}

func (v Validator) renderMermaid(ctx context.Context, targetID, rel string, index, total int, content string) error {
	if v.Config.Mermaid.Command == "" {
		return fmt.Errorf("mermaid.command is empty")
	}
	if _, err := exec.LookPath(v.Config.Mermaid.Command); err != nil {
		return err
	}
	cachePath := v.mermaidCachePath(content)
	if cachePath != "" {
		if stat, err := os.Stat(cachePath); err == nil && stat.Size() > 0 {
			validationProgress(targetID, "Mermaid cache hit | file=%s | diagram=%d/%d", rel, index+1, total)
			return nil
		}
	}
	tmpDir, err := os.MkdirTemp("", "wikiforge-mermaid-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	input := filepath.Join(tmpDir, fmt.Sprintf("%s-%d.mmd", sanitize(targetID+"-"+rel), index))
	output := filepath.Join(tmpDir, fmt.Sprintf("%s-%d.svg", sanitize(targetID+"-"+rel), index))
	if err := os.WriteFile(input, []byte(content), 0o644); err != nil {
		return err
	}
	inputArg, err := pathutil.ExternalToolPath(input)
	if err != nil {
		return fmt.Errorf("portable Mermaid input path: %w", err)
	}
	outputArg, err := pathutil.ExternalToolPath(output)
	if err != nil {
		return fmt.Errorf("portable Mermaid output path: %w", err)
	}
	args := rendererArgs(v.Config.Mermaid.Args, inputArg, outputArg)
	timeout := time.Duration(v.Config.Mermaid.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	renderCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(renderCtx, v.Config.Mermaid.Command, args...)
	var stderr boundedMermaidOutput
	cmd.Stderr = &stderr

	started := time.Now()
	validationProgress(targetID, "Mermaid render started | file=%s | diagram=%d/%d | timeout=%s", rel, index+1, total, timeout)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-renderCtx.Done():
				return
			case <-ticker.C:
				validationProgress(targetID, "Mermaid render still running | file=%s | diagram=%d/%d | elapsed=%s", rel, index+1, total, compactValidationDuration(time.Since(started)))
			}
		}
	}()
	runErr := cmd.Run()
	close(done)
	if runErr != nil {
		return fmt.Errorf("%w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if stat, err := os.Stat(output); err != nil || stat.Size() == 0 {
		return fmt.Errorf("renderer produced no output")
	}
	if cachePath != "" {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return fmt.Errorf("create Mermaid cache: %w", err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			return fmt.Errorf("read Mermaid output for cache: %w", err)
		}
		tmp := cachePath + fmt.Sprintf(".%d.tmp", time.Now().UnixNano())
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return fmt.Errorf("write Mermaid cache: %w", err)
		}
		if err := os.Rename(tmp, cachePath); err != nil {
			_ = os.Remove(tmp)
			if _, statErr := os.Stat(cachePath); statErr != nil {
				return fmt.Errorf("publish Mermaid cache: %w", err)
			}
		}
	}
	validationProgress(targetID, "Mermaid render completed | file=%s | diagram=%d/%d | elapsed=%s", rel, index+1, total, compactValidationDuration(time.Since(started)))
	return nil
}

func (v Validator) mermaidCachePath(content string) string {
	if strings.TrimSpace(v.Config.Mermaid.CacheDirectory) == "" {
		return ""
	}
	key := sha256.Sum256([]byte(v.Config.Mermaid.Command + "\x00" + strings.Join(v.Config.Mermaid.Args, "\x00") + "\x00" + content))
	return filepath.Join(v.Config.Mermaid.CacheDirectory, hex.EncodeToString(key[:])+".svg")
}

func rendererArgs(template []string, input, output string) []string {
	args := make([]string, len(template))
	for i, arg := range template {
		arg = strings.ReplaceAll(arg, "{input}", input)
		arg = strings.ReplaceAll(arg, "{output}", output)
		args[i] = arg
	}
	return args
}

func WriteResult(path string, result model.ValidationResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func FindingsText(findings []model.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "- [%s] %s (%s): %s\n", strings.ToUpper(f.Severity), f.Code, f.Path, f.Message)
	}
	return b.String()
}

func validationProgress(targetID, format string, args ...any) {
	progressMu.Lock()
	defer progressMu.Unlock()
	values := append([]any{targetID}, args...)
	_, _ = fmt.Fprintf(os.Stdout, "[%s/validation] "+format+"\n", values...)
}

func compactValidationDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func toSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
