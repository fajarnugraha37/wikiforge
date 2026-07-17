package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
	"github.com/example/wikiforge/internal/pathutil"
	"github.com/example/wikiforge/internal/prompts"
)

var (
	linkRE         = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	mermaidRE      = regexp.MustCompile("(?s)```mermaid\\s*\\n(.*?)```")
	backtickPathRE = regexp.MustCompile("`([^`]+[/\\\\][^`]+|[^`]+\\.[A-Za-z0-9]{1,12}(?::[0-9]+)?)`")
	lineSuffixRE   = regexp.MustCompile(`(?i)(?::[0-9]+(?:-[0-9]+)?|#L[0-9]+(?:-L[0-9]+)?)$`)
)

type Validator struct {
	Config config.Config
}

func (v Validator) ValidateComponent(ctx context.Context, component config.ComponentConfig) model.ValidationResult {
	profile, err := prompts.GetProfile(component.Profile)
	if err != nil {
		return model.ValidationResult{Root: component.DocumentationRoot(), Profile: component.Profile, Score: 0, Accepted: false, Findings: []model.Finding{{Code: "PROFILE-UNKNOWN", Severity: "error", Message: err.Error()}}}
	}
	return v.validate(ctx, component.ID, component.DocumentationRoot(), component.WorkDir(), prompts.ExpectedFiles(profile), prompts.ComponentPageContracts(profile), profile.ID, profile.MinimumPages, profile.MinimumMermaid)
}

// ValidateService remains as a source-compatible wrapper for v1 callers.
func (v Validator) ValidateService(ctx context.Context, service config.ServiceConfig) model.ValidationResult {
	component := config.ComponentConfig{ID: service.ID, Type: "microservice", Profile: "application", Repository: service.Path, Enabled: service.Enabled}
	return v.ValidateComponent(ctx, component)
}

func (v Validator) ValidateSystem(ctx context.Context) model.ValidationResult {
	return v.validate(ctx, v.Config.System.ID, filepath.Join(v.Config.System.Output, "openwiki"), v.Config.System.Output, prompts.ExpectedSystemFiles(), prompts.SystemPageContracts(), "system", len(prompts.ExpectedSystemFiles()), 8)
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

	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
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
				if !allowedFrontmatter[key] {
					add("DOC-FRONTMATTER-UNSUPPORTED", "error", rel, "unsupported OpenWiki front matter field: "+key)
				}
			}
		}
		upper := strings.ToUpper(content)
		if strings.Contains(upper, "TODO") || strings.Contains(upper, "TBD") {
			add("DOC-PLACEHOLDER", "warning", rel, "contains TODO/TBD; use an explicit Knowledge Gaps entry instead")
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
						add("DOC-SOURCE-PATH", "warning", rel, "source reference does not resolve in the configured scope: "+ref)
					}
				}
			}
		}
		for _, m := range linkRE.FindAllStringSubmatch(content, -1) {
			target := strings.TrimSpace(strings.Split(m[1], "#")[0])
			if target == "" || isExternalReference(target) || strings.HasPrefix(target, "/") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			if _, err := os.Stat(resolved); err != nil {
				add("DOC-BROKEN-LINK", "error", rel, "broken relative link: "+target)
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
				}
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
				if err := v.renderMermaid(ctx, targetID, rel, i, block[1]); err != nil {
					add("MERMAID-RENDER", "error", rel, fmt.Sprintf("diagram %d failed to render: %v", i+1, err))
				}
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

	quickstartPath := filepath.Join(root, "quickstart.md")
	if b, err := os.ReadFile(quickstartPath); err == nil {
		linked := map[string]bool{}
		for _, match := range linkRE.FindAllStringSubmatch(string(b), -1) {
			target := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(strings.Split(match[1], "#")[0], "./")))
			linked[target] = true
		}
		for _, expectedPath := range expected {
			if expectedPath == "quickstart.md" {
				continue
			}
			if !linked[filepath.ToSlash(expectedPath)] {
				add("DOC-NAVIGATION", "error", "quickstart.md", "canonical page is not linked: "+expectedPath)
			}
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

	errorsCount, warningsCount := 0, 0
	for _, f := range result.Findings {
		if f.Severity == "error" {
			errorsCount++
		} else {
			warningsCount++
		}
	}
	result.Score = max(0, 100-errorsCount*8-warningsCount*2)
	result.Accepted = errorsCount == 0 && result.Score >= v.Config.Documentation.MinimumQualityScore
	return result
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
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "-") && current != "" {
			out[current] = strings.TrimSpace(out[current] + " " + strings.TrimPrefix(line, "-"))
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
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
	// Catch accidental enormous generated identifiers that make diagrams unusable.
	for i, line := range lines {
		if len(line) > 500 {
			return fmt.Errorf("diagram line %d exceeds 500 characters", i+1)
		}
	}
	return nil
}

func (v Validator) renderMermaid(ctx context.Context, targetID, rel string, index int, content string) error {
	if v.Config.Mermaid.Command == "" {
		return fmt.Errorf("mermaid.command is empty")
	}
	if _, err := exec.LookPath(v.Config.Mermaid.Command); err != nil {
		return err
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, v.Config.Mermaid.Command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stat, err := os.Stat(output); err != nil || stat.Size() == 0 {
		return fmt.Errorf("renderer produced no output")
	}
	return nil
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
