package planner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/discovery"
	"github.com/example/wikiforge/internal/model"
	"github.com/example/wikiforge/internal/planner"
)

func TestRepresentativeRepositoryTypesProduceRelevantAdaptivePlans(t *testing.T) {
	cases := []struct {
		name, componentType, profile, rel, content, expectedPack, expectedUnit string
	}{
		{"monolith", "monolith", "application", "db/schema.sql", "create table orders(id bigint);", "database", ""},
		{"modular", "modular-monolith", "modular-application", "modules/orders/README.md", "bounded context and business rules", "domain", "orders"},
		{"microservice", "microservice", "application", "api/openapi.yaml", "openapi: 3.1.0", "api", ""},
		{"library", "library", "reusable", "src/pool.go", "mutex semaphore thread safe", "concurrency", ""},
		{"framework", "framework", "reusable", "src/telemetry.go", "opentelemetry tracing metrics", "telemetry", ""},
		{"infrastructure", "iac", "infrastructure", "deploy/deployment.yaml", "kind: Deployment kubernetes", "container-runtime", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(tc.rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := config.Defaults()
			component := config.ComponentConfig{ID: tc.name, Type: tc.componentType, Profile: tc.profile, Repository: root, Enabled: true}
			cfg.Components = []config.ComponentConfig{component}
			manifest, err := discovery.Discover(cfg, component)
			if err != nil {
				t.Fatal(err)
			}
			plan := planner.Build(cfg, component, manifest)
			if !contains(plan.SelectedPacks, tc.expectedPack) {
				t.Fatalf("packs=%v want %s", plan.SelectedPacks, tc.expectedPack)
			}
			if !pageForPack(plan, tc.expectedPack) {
				t.Fatalf("pack %s has no page: %+v", tc.expectedPack, plan.Pages)
			}
			if tc.expectedUnit != "" && !unitExists(plan.Units, tc.expectedUnit) {
				t.Fatalf("units=%+v want %s", plan.Units, tc.expectedUnit)
			}
		})
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
func pageForPack(plan model.DocumentationPlan, pack string) bool {
	for _, page := range plan.Pages {
		if page.Pack == pack {
			return true
		}
	}
	return false
}
func unitExists(units []model.DocumentationUnit, id string) bool {
	for _, unit := range units {
		if unit.ID == id {
			return true
		}
	}
	return false
}
