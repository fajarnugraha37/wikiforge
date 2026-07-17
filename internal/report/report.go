package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/example/wikiforge/internal/model"
)

type RunReport struct {
	RunID       string                            `json:"runId"`
	GeneratedAt time.Time                         `json:"generatedAt"`
	Components  map[string]model.ValidationResult `json:"components"`
	System      *model.ValidationResult           `json:"system,omitempty"`
	Failures    map[string]string                 `json:"failures,omitempty"`
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
	b.WriteString("## Components\n\n| Component | Profile | Score | Accepted | Findings |\n|---|---|---:|:---:|---:|\n")
	ids := make([]string, 0, len(r.Components))
	for id := range r.Components {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := r.Components[id]
		fmt.Fprintf(&b, "| %s | %s | %d | %t | %d |\n", id, v.Profile, v.Score, v.Accepted, len(v.Findings))
	}
	if r.System != nil {
		fmt.Fprintf(&b, "\n## Whole System\n\nScore: **%d**  \nAccepted: **%t**  \nFindings: **%d**\n", r.System.Score, r.System.Accepted, len(r.System.Findings))
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
