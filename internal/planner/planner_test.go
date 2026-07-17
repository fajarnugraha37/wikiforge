package planner

import (
	"testing"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
)

func TestEverySupportedPackHasCanonicalPlanningOutcome(t *testing.T) {
	cfg := config.Defaults()
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: t.TempDir(), Enabled: true, Packs: config.SupportedCapabilityPacks()}
	manifest := model.DiscoveryManifest{Component: model.Component{ID: component.ID}, Packs: config.SupportedCapabilityPacks()}
	plan := Build(cfg, component, manifest)
	for _, pack := range config.SupportedCapabilityPacks() {
		if !hasPackPage(plan, pack) {
			t.Errorf("pack %s has no canonical page: %+v", pack, plan.Decisions)
		}
		if hasDecision(plan, pack, "defer") {
			t.Errorf("pack %s was unexpectedly deferred", pack)
		}
	}
	if len(plan.Pages) != len(uniquePaths(plan.Pages)) {
		t.Fatal("planner emitted duplicate page paths")
	}
}

func TestPlannerDefersPackWhenRequiredViewDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Documentation.Views = []string{"component", "catalog"}
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: t.TempDir(), Enabled: true, Packs: []string{"cache"}}
	plan := Build(cfg, component, model.DiscoveryManifest{Packs: []string{"cache"}})
	if !hasDecision(plan, "cache", "defer") {
		t.Fatalf("expected cache deferral: %+v", plan.Decisions)
	}
	if hasPackPage(plan, "cache") {
		t.Fatal("cache page should not be planned with platform view disabled")
	}
}

func TestProfilesProduceDifferentComposablePlans(t *testing.T) {
	cfg := config.Defaults()
	app := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Enabled: true}
	lib := config.ComponentConfig{ID: "lib", Type: "framework", Profile: "reusable", Enabled: true}
	infra := config.ComponentConfig{ID: "infra", Type: "iac", Profile: "infrastructure", Enabled: true}
	appPlan := Build(cfg, app, model.DiscoveryManifest{})
	libPlan := Build(cfg, lib, model.DiscoveryManifest{})
	infraPlan := Build(cfg, infra, model.DiscoveryManifest{})
	if equalStrings(appPlan.SelectedPacks, libPlan.SelectedPacks) || equalStrings(appPlan.SelectedPacks, infraPlan.SelectedPacks) || equalStrings(libPlan.SelectedPacks, infraPlan.SelectedPacks) {
		t.Fatalf("profiles should produce different packs: app=%v lib=%v infra=%v", appPlan.SelectedPacks, libPlan.SelectedPacks, infraPlan.SelectedPacks)
	}
}

func TestDocumentationUnitsAndShardPolicyArePreserved(t *testing.T) {
	cfg := config.Defaults()
	cfg.Documentation.Catalogs.ShardBy = []string{"owner", "domain"}
	cfg.Documentation.Catalogs.MaximumRowsPerPage = 42
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Enabled: true}
	manifest := model.DiscoveryManifest{Units: []model.DocumentationUnit{{ID: "submit-order", ComponentID: "app", Kind: "flow", OutputPath: "flows/submit-order", Origin: "configured"}}}
	plan := Build(cfg, component, manifest)
	if plan.MaximumRowsPerPage != 42 || len(plan.ShardBy) != 2 {
		t.Fatalf("shard policy lost: %+v", plan)
	}
	if !hasPath(plan, "flows/submit-order.md") {
		t.Fatalf("flow page missing: %+v", plan.Pages)
	}
	page, ok := pageByPath(plan, "catalogs/interfaces/index.md")
	if !ok || page.Kind != "collection" || page.MaximumRowsPerPage != 42 || len(page.ShardBy) != 2 {
		t.Fatalf("collection shard policy was not attached to the planned page: %+v", page)
	}
}

func pageByPath(plan model.DocumentationPlan, path string) (model.PlanPage, bool) {
	for _, page := range plan.Pages {
		if page.Path == path {
			return page, true
		}
	}
	return model.PlanPage{}, false
}

func hasPackPage(plan model.DocumentationPlan, pack string) bool {
	for _, page := range plan.Pages {
		if page.Pack == pack {
			return true
		}
	}
	return false
}
func hasDecision(plan model.DocumentationPlan, subject, action string) bool {
	for _, decision := range plan.Decisions {
		if decision.Subject == subject && decision.Action == action {
			return true
		}
	}
	return false
}
func hasPath(plan model.DocumentationPlan, path string) bool {
	for _, page := range plan.Pages {
		if page.Path == path {
			return true
		}
	}
	return false
}
func uniquePaths(pages []model.PlanPage) map[string]bool {
	out := map[string]bool{}
	for _, page := range pages {
		out[page.Path] = true
	}
	return out
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSystemPlanAggregatesComponentPacksAndUnits(t *testing.T) {
	cfg := config.Defaults()
	plans := []model.DocumentationPlan{
		{ComponentID: "app", SelectedPacks: []string{"api", "messaging"}, Units: []model.DocumentationUnit{{ID: "orders", ComponentID: "app", Kind: "domain"}}},
		{ComponentID: "worker", SelectedPacks: []string{"jobs", "messaging"}},
	}
	plan := BuildSystem(cfg, plans)
	if plan.Profile != "system" || !hasPath(plan, "system/component-landscape.md") || !hasPath(plan, "system/domain-map.md") {
		t.Fatalf("system plan=%+v", plan)
	}
	if len(plan.SelectedPacks) != 3 || len(plan.Units) != 1 {
		t.Fatalf("aggregation packs=%v units=%v", plan.SelectedPacks, plan.Units)
	}
}

func TestQuickstartRemainsPlannedWhenDetailedComponentViewDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Documentation.Views = []string{"domain"}
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Enabled: true}
	plan := Build(cfg, component, model.DiscoveryManifest{})
	if !hasPath(plan, "quickstart.md") {
		t.Fatal("root navigation entry point must not disappear with the detailed component view")
	}
	if hasPath(plan, "components/app/index.md") {
		t.Fatal("detailed component page should respect the disabled component view")
	}
}

func TestPlannerReportsOutputCollisionInsteadOfSilentlyDroppingUnit(t *testing.T) {
	cfg := config.Defaults()
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Enabled: true}
	manifest := model.DiscoveryManifest{Units: []model.DocumentationUnit{{ID: "conflict", ComponentID: "app", Kind: "domain", OutputPath: "domains/index.md", Origin: "configured"}}}
	plan := Build(cfg, component, manifest)
	if !hasDecision(plan, "app:unit:conflict", "defer") {
		t.Fatalf("expected explicit collision decision: %+v", plan.Decisions)
	}
}

func TestUnitOutputAcceptsExplicitMarkdownPath(t *testing.T) {
	cfg := config.Defaults()
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Enabled: true}
	manifest := model.DiscoveryManifest{Units: []model.DocumentationUnit{{ID: "orders", ComponentID: "app", Kind: "domain", OutputPath: "domains/orders/overview.md", Origin: "configured"}}}
	plan := Build(cfg, component, manifest)
	if !hasPath(plan, "domains/orders/overview.md") {
		t.Fatalf("explicit Markdown output was rewritten incorrectly: %+v", plan.Pages)
	}
}
