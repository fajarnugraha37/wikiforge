package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
)

type evidenceArtifacts struct {
	Inventory  string
	Semantic   string
	Identities string
	Discovery  string
	Plan       string
	Index      string
	Impact     string
	Coverage   string
}

func componentEvidenceArtifacts(workspace, id string) evidenceArtifacts {
	root := filepath.Join(workspace, ".wikiforge", "components", id)
	return evidenceArtifacts{
		Inventory:  filepath.Join(root, "inventory.json"),
		Semantic:   filepath.Join(root, "semantic-discovery.json"),
		Identities: filepath.Join(root, "semantic-identities.json"),
		Discovery:  filepath.Join(root, "discovery.json"),
		Plan:       filepath.Join(root, "plan.json"),
		Index:      filepath.Join(root, "evidence-index.json"),
		Impact:     filepath.Join(root, "impact-index.json"),
		Coverage:   filepath.Join(root, "coverage.json"),
	}
}

func systemEvidenceArtifacts(workspace, id string) evidenceArtifacts {
	root := filepath.Join(workspace, ".wikiforge", "system", id)
	return evidenceArtifacts{
		Discovery: filepath.Join(root, "discovery.json"),
		Plan:      filepath.Join(root, "plan.json"),
		Index:     filepath.Join(root, "evidence-index.json"),
		Impact:    filepath.Join(root, "impact-index.json"),
		Coverage:  filepath.Join(root, "coverage.json"),
	}
}

func prepareEvidence(root, repositoryID string, include, exclude []string, cachePath string, maxBytes int64, plan planner.ComponentPlan, target model.TargetState, artifacts evidenceArtifacts) (evidence.Index, evidence.ChangeImpact, error) {
	currentRevision := gitHead(root)
	previousRevision := target.EvidenceRevision
	previous, loadErr := evidence.LoadIndex(artifacts.Index)
	if _, impactErr := evidence.LoadImpact(artifacts.Impact); impactErr != nil {
		loadErr = impactErr
	}
	if loadErr == nil && previousRevision == "" {
		previousRevision = previous.Revision
	}
	index, err := evidence.BuildIndexCached(root, repositoryID, currentRevision, include, exclude, cachePath, maxBytes)
	if err != nil {
		return evidence.Index{}, evidence.ChangeImpact{}, err
	}
	if loadErr == nil && previous.RepositoryID != repositoryID {
		loadErr = fmt.Errorf("evidence index repository mismatch")
	}
	docsRoot := filepath.Join(root, "openwiki")
	if _, statErr := os.Stat(docsRoot); statErr == nil {
		index, err = evidence.AttachDocumentation(docsRoot, index)
		if err != nil {
			return evidence.Index{}, evidence.ChangeImpact{}, err
		}
	}
	changed, fullScan, reason, err := evidence.ChangedPaths(root, previousRevision, currentRevision)
	if err != nil {
		return evidence.Index{}, evidence.ChangeImpact{}, err
	}
	if loadErr != nil {
		fullScan = true
		reason = "evidence index unavailable; full-scope scan"
	}
	previousIndex := evidence.Index{}
	if loadErr == nil {
		previousIndex = previous
	}
	impact := evidence.BuildImpactWithPrevious(index, previousIndex, plan, previousRevision, currentRevision, changed, fullScan, reason)
	if err := saveEvidenceArtifacts(artifacts, index, impact); err != nil {
		return evidence.Index{}, evidence.ChangeImpact{}, err
	}
	return index, impact, nil
}

func finalizeEvidence(root, repositoryID string, include, exclude []string, cachePath string, maxBytes int64, plan planner.ComponentPlan, artifacts evidenceArtifacts) (evidence.Index, error) {
	index, err := evidence.BuildIndexCached(root, repositoryID, gitHead(root), include, exclude, cachePath, maxBytes)
	if err != nil {
		return evidence.Index{}, err
	}
	docsRoot := filepath.Join(root, "openwiki")
	if _, err := os.Stat(docsRoot); err == nil {
		index, err = evidence.AttachDocumentation(docsRoot, index)
		if err != nil {
			return evidence.Index{}, err
		}
	}
	coverage := evidence.BuildCoverage(repositoryID, plan, index)
	if err := evidence.SaveJSON(artifacts.Index, index); err != nil {
		return evidence.Index{}, err
	}
	if err := evidence.SaveJSON(artifacts.Coverage, coverage); err != nil {
		return evidence.Index{}, err
	}
	return index, nil
}

func evidenceCachePath(directory, repositoryID string) string {
	if directory == "" {
		return ""
	}
	return filepath.Join(directory, sanitizeArtifactName(repositoryID)+".json")
}

func sanitizeArtifactName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "repository"
	}
	return b.String()
}

func saveEvidenceArtifacts(artifacts evidenceArtifacts, index evidence.Index, impact evidence.ChangeImpact) error {
	if err := evidence.SaveJSON(artifacts.Index, index); err != nil {
		return err
	}
	return evidence.SaveJSON(artifacts.Impact, impact)
}

func savePlanArtifacts(plan planner.ComponentPlan, discovery planner.DiscoveryManifest, artifacts evidenceArtifacts) error {
	if err := evidence.SaveJSON(artifacts.Discovery, discovery); err != nil {
		return err
	}
	if err := evidence.SaveJSON(artifacts.Plan, plan); err != nil {
		return err
	}
	return nil
}

func affectedPagePaths(plan planner.ComponentPlan, impact evidence.ChangeImpact) []string {
	if impact.FullScan {
		return plannedPaths(plan.Pages)
	}
	set := map[string]bool{}
	for path := range impact.AffectedPages {
		set[filepath.ToSlash(path)] = true
	}
	paths := make([]string, 0, len(set))
	for _, page := range plan.Pages {
		path := filepath.ToSlash(page.Path)
		if set[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func ensureAffectedPages(plan planner.ComponentPlan, impact *evidence.ChangeImpact, pages []string) []string {
	if len(impact.ChangedPaths) == 0 {
		return pages
	}
	if len(pages) > 0 || impact.FullScan {
		return pages
	}
	impact.FullScan = true
	impact.Reason = "changed evidence has no mapped documentation page; full-scope fallback"
	return plannedPaths(plan.Pages)
}

func evidenceSourceHash(index evidence.Index) string {
	return evidence.Fingerprint(index)
}

func planFingerprint(plan any) string {
	b, _ := json.Marshal(plan)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func forceFullImpact(plan planner.ComponentPlan, impact *evidence.ChangeImpact, reason string) {
	impact.FullScan = true
	impact.Reason = reason
	impact.AffectedPages = map[string][]string{}
	for _, page := range plan.Pages {
		path := filepath.ToSlash(page.Path)
		impact.AffectedPages[path] = []string{reason}
	}
}
