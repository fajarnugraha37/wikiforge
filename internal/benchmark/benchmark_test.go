package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
)

type benchmarkCase struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"`
	Profile             string   `json:"profile"`
	MinimumPages        int      `json:"minimumPages"`
	RequiredViews       []string `json:"requiredViews"`
	MinimumPlanCoverage float64  `json:"minimumPlanCoverage"`
	RequiredPacks       []string `json:"requiredPacks"`
	MinimumIndexes      int      `json:"minimumIndexes"`
	RequiresCollection  bool     `json:"requiresCollection"`
}

func TestPhase4PlannerBenchmarkThresholds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "phase4-benchmark.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []benchmarkCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range cases {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# "+fixture.ID+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if fixture.ID == "bpmn-application" {
				if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "workflows", "submit-order.bpmn"), []byte("<definitions/>"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if fixture.ID == "large-modular-monolith" {
				for _, domain := range []string{"orders", "pricing"} {
					if err := os.MkdirAll(filepath.Join(root, "modules", domain), 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			cfg := config.Defaults()
			cfg.Workspace = root
			cfg.System.Enabled = false
			configuredPacks := append([]string{}, fixture.RequiredPacks...)
			configuredPacks = append(configuredPacks, "api", "configuration", "data", "jobs", "messaging", "security", "deployment", "telemetry")
			cfg.Components = []config.ComponentConfig{{ID: fixture.ID, Type: fixture.Type, Profile: fixture.Profile, Repository: root, Enabled: true, Capabilities: []string{"order-management"}, Packs: configuredPacks}}
			semantic := map[string]model.SemanticDiscovery{fixture.ID: {SchemaVersion: model.DiscoverySchemaVersion, ComponentID: fixture.ID, RepositoryID: fixture.ID, DiscoveryMode: "explicit", Repository: model.RepositoryFinding{Profile: fixture.Profile, Status: model.StatusExplicitEnabled, Confidence: "high"}}}
			result, err := (planner.Planner{Config: cfg, Semantic: semantic}).Plan(fixture.ID, false)
			if err != nil || len(result.Components) != 1 {
				t.Fatalf("planning failed: %v", err)
			}
			plan := result.Components[0]
			if len(plan.Pages) < fixture.MinimumPages {
				t.Fatalf("plan pages=%d below threshold=%d", len(plan.Pages), fixture.MinimumPages)
			}
			views := map[string]bool{}
			for _, view := range plan.Views {
				views[string(view)] = true
			}
			for _, required := range fixture.RequiredViews {
				if !views[required] {
					t.Fatalf("required view %q is missing from plan", required)
				}
			}
			coverage := float64(len(plan.Pages)) / float64(fixture.MinimumPages)
			if coverage < fixture.MinimumPlanCoverage {
				t.Fatalf("plan coverage=%.2f below threshold=%.2f", coverage, fixture.MinimumPlanCoverage)
			}
			packs := map[string]bool{}
			for _, pack := range plan.Packs {
				packs[pack] = true
			}
			for _, required := range fixture.RequiredPacks {
				if !packs[required] {
					t.Fatalf("required capability pack %q is missing: %v", required, plan.Packs)
				}
			}
			indexes := 0
			collections := 0
			for _, page := range plan.Pages {
				if page.Kind == planner.PageIndex {
					indexes++
				}
				if page.Kind == planner.PageCollection {
					collections++
				}
			}
			if indexes < fixture.MinimumIndexes {
				t.Fatalf("indexes=%d below threshold=%d", indexes, fixture.MinimumIndexes)
			}
			if fixture.RequiresCollection && collections == 0 {
				t.Fatal("expected at least one typed collection page")
			}
		})
	}
}
