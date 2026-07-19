package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/model"
)

type RunReport struct {
	RunID       string                            `json:"runId"`
	GeneratedAt time.Time                         `json:"generatedAt"`
	Components  map[string]model.ValidationResult `json:"components"`
	System      *model.ValidationResult           `json:"system,omitempty"`
	Failures    map[string]string                 `json:"failures,omitempty"`
	Metrics     model.RunMetrics                  `json:"metrics"`
}

func Write(root string, r RunReport) (string, error) {
	dir := filepath.Join(root, ".wikiforge", "reports", r.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	jb, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), jb, 0o644); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# WikiForge Run %s\n\nGenerated: %s\n\n", r.RunID, r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "## Runtime Metrics\n\n| Metric | Value |\n|---|---:|\n| Duration (ms) | %d |\n| OpenWiki calls | %d |\n| Discovery stages | %d |\n| Discovery calls | %d |\n| Discovery cache hits | %d |\n| Discovery cache misses | %d |\n| Discovery accepted | %d |\n| Discovery uncertain | %d |\n| Discovery conflicting | %d |\n| Discovery unknown | %d |\n| Discovery modules | %d |\n| Discovery domains | %d |\n| Discovery flows | %d |\n| Discovery concerns | %d |\n| Discovery ownership | %d |\n| Discovery relationships | %d |\n| Discovery inventory versions | %s |\n| Discovery prompt versions | %s |\n| Discovery model IDs | %s |\n| Pages generated | %d |\n| Pages updated | %d |\n| Evidence files | %d |\n| Evidence cache hits | %d |\n| Evidence cache misses | %d |\n| Input tokens | %d |\n| Output tokens | %d |\n| Usage reported | %t |\n\n", r.Metrics.DurationMillis, r.Metrics.OpenWikiCalls, r.Metrics.DiscoveryStages, r.Metrics.DiscoveryCalls, r.Metrics.DiscoveryCacheHits, r.Metrics.DiscoveryCacheMisses, r.Metrics.DiscoveryAccepted, r.Metrics.DiscoveryUncertain, r.Metrics.DiscoveryConflicting, r.Metrics.DiscoveryUnknown, r.Metrics.DiscoveryCounts.Modules, r.Metrics.DiscoveryCounts.Domains, r.Metrics.DiscoveryCounts.Flows, r.Metrics.DiscoveryCounts.Concerns, r.Metrics.DiscoveryCounts.Ownership, r.Metrics.DiscoveryCounts.Relationships, strings.Join(r.Metrics.DiscoveryInventoryVersions, ","), strings.Join(r.Metrics.DiscoveryPromptVersions, ","), strings.Join(r.Metrics.DiscoveryModelIDs, ","), r.Metrics.PagesGenerated, r.Metrics.PagesUpdated, r.Metrics.EvidenceFiles, r.Metrics.EvidenceCacheHits, r.Metrics.EvidenceCacheMisses, r.Metrics.InputTokens, r.Metrics.OutputTokens, r.Metrics.UsageReported)
	b.WriteString("## Components\n\n| Component | Profile | Score | Accepted | Findings |\n|---|---|---:|:---:|---:|\n")
	ids := make([]string, 0, len(r.Components))
	for id := range r.Components {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := r.Components[id]
		fmt.Fprintf(&b, "| %s | %s | %d | %t | %d |\n", id, v.Profile, v.Score, v.Accepted, len(v.Findings))
		writeDimensions(&b, id, v)
	}
	if r.System != nil {
		fmt.Fprintf(&b, "\n## Whole System\n\nScore: **%d**  \nAccepted: **%t**  \nFindings: **%d**\n", r.System.Score, r.System.Accepted, len(r.System.Findings))
		writeDimensions(&b, "system", *r.System)
	}
	if len(r.Failures) > 0 {
		b.WriteString("\n## Failures\n\n")
		keys := make([]string, 0, len(r.Failures))
		for k := range r.Failures {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- **%s:** %s\n", k, r.Failures[k])
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	latest := filepath.Join(root, ".wikiforge", "reports", "latest.txt")
	_ = os.MkdirAll(filepath.Dir(latest), 0o755)
	_ = os.WriteFile(latest, []byte(dir+"\n"), 0o644)
	return dir, nil
}

func writeDimensions(b *strings.Builder, id string, result model.ValidationResult) {
	if len(result.Dimensions) == 0 {
		return
	}
	names := make([]string, 0, len(result.Dimensions))
	for name := range result.Dimensions {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintf(b, "\n### %s Dimensions\n\n| Dimension | Score | Findings |\n|---|---:|---|\n", id)
	for _, name := range names {
		dimension := result.Dimensions[name]
		fmt.Fprintf(b, "| %s | %d | %s |\n", name, dimension.Score, strings.Join(dimension.FindingCodes, ", "))
	}
}
