package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
)

func TestDiscoverIsDeterministicForTheSameRepositoryState(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "commerce-core")
	mkdirAll(t, filepath.Join(repo, "modules", "order"))
	mkdirAll(t, filepath.Join(repo, "modules", "pricing"))

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = []config.ComponentConfig{
		{ID: "commerce-core", Type: "modular-monolith", Profile: "modular-application", Repository: repo, Enabled: true},
	}

	p := Planner{Config: cfg, Semantic: testSemantic(cfg)}
	first, err := p.Discover("")
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.Discover("")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("discovery is not deterministic\nfirst=%s\nsecond=%s", a, b)
	}
}

func TestPlanSelectsAdaptiveViewsByProfile(t *testing.T) {
	root := t.TempDir()
	appRepo := filepath.Join(root, "app")
	modularRepo := filepath.Join(root, "modular")
	libraryRepo := filepath.Join(root, "library")
	iacRepo := filepath.Join(root, "iac")
	for _, repo := range []string{appRepo, modularRepo, libraryRepo, iacRepo} {
		mkdirAll(t, repo)
	}
	mkdirAll(t, filepath.Join(modularRepo, "modules", "ordering"))
	mkdirAll(t, filepath.Join(modularRepo, "modules", "pricing"))

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Documentation.Catalogs.ShardBy = []string{"domain"}
	cfg.Components = []config.ComponentConfig{
		{ID: "app", Type: "microservice", Profile: "application", Repository: appRepo, Enabled: true},
		{ID: "modular", Type: "modular-monolith", Profile: "modular-application", Repository: modularRepo, Enabled: true},
		{ID: "lib", Type: "framework", Profile: "reusable", Repository: libraryRepo, Enabled: true},
		{ID: "iac", Type: "iac", Profile: "infrastructure", Repository: iacRepo, Enabled: true},
	}

	for i := range cfg.Components {
		cfg.Components[i].Packs = []string{"api", "configuration", "data", "domain", "jobs", "messaging", "security", "deployment", "telemetry"}
	}
	plan, err := (Planner{Config: cfg, Semantic: testSemantic(cfg)}).Plan("", true)
	if err != nil {
		t.Fatal(err)
	}
	byComponent := map[string]ComponentPlan{}
	for _, componentPlan := range plan.Components {
		byComponent[componentPlan.ComponentID] = componentPlan
	}

	if hasPath(byComponent["app"].Pages, "domains/ordering/index.md") {
		t.Fatal("application plan unexpectedly created derived domain pages")
	}
	if !hasDecision(byComponent["app"].Decisions, "flows/", "skip") {
		t.Fatal("application plan should explicitly skip flow pages without discovered flow units")
	}
	if !hasPath(byComponent["modular"].Pages, "domains/ordering/index.md") {
		t.Fatal("modular application should derive domain pages from module roots")
	}
	if !hasDecision(byComponent["modular"].Decisions, "catalogs/interfaces", "shard") {
		t.Fatal("modular application should shard catalogs by domain when configured")
	}
	if hasPath(byComponent["lib"].Pages, "operations/index.md") {
		t.Fatal("reusable library should not get operations view by default")
	}
	if !hasPath(byComponent["iac"].Pages, "platform/containerization.md") {
		t.Fatal("infrastructure plan should include platform containerization page")
	}
	if plan.System == nil || !hasPath(plan.System.Pages, "system/component-landscape.md") {
		t.Fatal("system plan missing component-landscape page")
	}
	for _, path := range []string{"components/index.md", "catalogs/index.md"} {
		if !hasPath(byComponent["app"].Pages, path) {
			t.Fatalf("application plan missing hierarchical view index %s", path)
		}
	}
	if pageKind(byComponent["app"].Pages, "components/app/index.md") != PageIndex {
		t.Fatal("component unit index must be typed as an index page")
	}
}

func TestExplicitDocumentationUnitsArePreserved(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "commerce-core")
	mkdirAll(t, repo)

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = []config.ComponentConfig{
		{ID: "commerce-core", Type: "modular-monolith", Profile: "modular-application", Repository: repo, Enabled: true},
	}
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{
		{ID: "submit-order", Component: "commerce-core", Kind: "flow", SourceRoots: []string{"workflows/order"}, Output: "flows/submit-order"},
	}

	manifest, err := (Planner{Config: cfg, Semantic: testSemantic(cfg)}).Discover("")
	if err != nil {
		t.Fatal(err)
	}
	if !hasUnit(manifest.CandidateDocumentationUnits, "submit-order") {
		t.Fatal("explicit flow unit missing from discovery manifest")
	}
}

func TestExplicitUnitIdentityMergesMatchingInferredFindingWithoutDuplicate(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = []config.ComponentConfig{{ID: "commerce", Type: "modular-monolith", Profile: "modular-application", Repository: root, Enabled: true}}
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{{ID: "orders-domain", Component: "commerce", Kind: "domain", Domain: "Orders", Output: "domains/orders"}}
	semantic := model.SemanticDiscovery{
		SchemaVersion: model.DiscoverySchemaVersion, ComponentID: "commerce", RepositoryID: "commerce", DiscoveryMode: "hybrid",
		Domains: []model.DomainFinding{{
			FindingBase: model.FindingBase{ID: "orders", Candidate: model.SemanticCandidate{CandidateKey: "orders-candidate", Name: "Orders", EvidenceIDs: []string{"ev"}}, Status: model.StatusObserved, Confidence: "high", EvidenceIDs: []string{"ev"}},
			SourceRoots: []string{"modules/orders"},
		}},
	}
	manifest, err := (Planner{Config: cfg, Semantic: map[string]model.SemanticDiscovery{"commerce": semantic}}).Discover("commerce")
	if err != nil {
		t.Fatal(err)
	}
	var matches []DocumentationUnit
	for _, unit := range manifest.CandidateDocumentationUnits {
		if unit.Kind == UnitDomain && unit.Domain == "Orders" {
			matches = append(matches, unit)
		}
	}
	if len(matches) != 1 || matches[0].ID != "orders-domain" || len(matches[0].EvidenceIDs) != 1 || len(matches[0].SourceRoots) != 1 {
		t.Fatalf("explicit and inferred units were not merged: %+v", matches)
	}
}

func TestExplicitModePlansWithoutSemanticArtifact(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Documentation.Discovery.Mode = "explicit"
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{{ID: "orders", Component: "app", Kind: "domain", Output: "domains/orders"}}
	cfg.Components = []config.ComponentConfig{{ID: "app", Profile: "application", Repository: root, Enabled: true}}
	if _, err := (Planner{Config: cfg}).Plan("app", false); err != nil {
		t.Fatalf("explicit plan unexpectedly requires semantic artifact: %v", err)
	}
}

func TestCatalogsCanShardByOwnerAndExposeTypedContracts(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "commerce-core")
	mkdirAll(t, repo)
	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Documentation.Catalogs.ShardBy = []string{"owner"}
	cfg.Components = []config.ComponentConfig{{ID: "commerce-core", Type: "modular-monolith", Profile: "modular-application", Repository: repo, Enabled: true, Packs: []string{"api"}}}
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{
		{ID: "orders", Component: "commerce-core", Kind: "domain", Owners: []string{"orders-team"}, Output: "domains/orders"},
		{ID: "pricing", Component: "commerce-core", Kind: "domain", Owners: []string{"pricing-team"}, Output: "domains/pricing"},
	}
	result, err := (Planner{Config: cfg, Semantic: testSemantic(cfg)}).Plan("commerce-core", false)
	if err != nil {
		t.Fatal(err)
	}
	plan := result.Components[0]
	if !hasDecision(plan.Decisions, "catalogs/interfaces", "shard") {
		t.Fatal("expected owner-sharded interface catalog")
	}
	if !hasPath(plan.Pages, "catalogs/interfaces/orders-team.md") || !hasPath(plan.Pages, "catalogs/interfaces/pricing-team.md") {
		t.Fatal("owner shard pages missing")
	}
	for _, page := range plan.Pages {
		if page.Path == "catalogs/interfaces/orders-team.md" {
			if page.Contract.Kind != PageShard || len(page.Contract.ShardDimensions) != 1 || page.Contract.ShardDimensions[0] != model.ShardOwner {
				t.Fatalf("owner shard contract not typed: %+v", page.Contract)
			}
			return
		}
	}
	t.Fatal("owner shard contract not found")
}

func testSemantic(cfg config.Config) map[string]model.SemanticDiscovery {
	result := map[string]model.SemanticDiscovery{}
	for _, component := range cfg.Components {
		d := model.SemanticDiscovery{SchemaVersion: model.DiscoverySchemaVersion, ComponentID: component.ID, RepositoryID: component.ID, DiscoveryMode: "explicit", Repository: model.RepositoryFinding{Profile: component.Profile, Status: model.StatusExplicitEnabled, Confidence: "high"}}
		if component.Profile == "modular-application" {
			for _, name := range []string{"ordering", "pricing"} {
				d.Domains = append(d.Domains, model.DomainFinding{FindingBase: model.FindingBase{ID: name, Candidate: model.SemanticCandidate{CandidateKey: name, Name: name}, Status: model.StatusObserved, Confidence: "high"}, SourceRoots: []string{"modules/" + name}})
			}
		}
		result[component.ID] = d
	}
	return result
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func hasPath(pages []PlannedPage, path string) bool {
	for _, page := range pages {
		if page.Path == path {
			return true
		}
	}
	return false
}

func hasDecision(decisions []PlanDecision, target, action string) bool {
	for _, decision := range decisions {
		if decision.Target == target && decision.Action == action {
			return true
		}
	}
	return false
}

func hasUnit(units []DocumentationUnit, id string) bool {
	for _, unit := range units {
		if unit.ID == id {
			return true
		}
	}
	return false
}

func pageKind(pages []PlannedPage, path string) PageKind {
	for _, page := range pages {
		if page.Path == path {
			return page.Kind
		}
	}
	return ""
}
