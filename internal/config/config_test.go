package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadV2ComponentsAndScopes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 2
workspace: .
openwiki:
  command: npx
execution:
  parallelComponents: 3
documentation:
  language: English
mermaid:
  mode: basic
components:
  - id: app
    type: microservice
    repository: ./mono
    scope: apps/app
    enabled: true
  - id: lib
    type: framework
    repository: ./mono
    scope: packages/lib
    enabled: true
    dependsOn: [app]
system:
  enabled: true
  output: ./system
  factsPath: ./facts
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Components) != 2 || cfg.Components[0].Profile != "application" || cfg.Components[1].Profile != "reusable" {
		t.Fatalf("unexpected components: %+v", cfg.Components)
	}
	if !filepath.IsAbs(cfg.Components[0].Repository) || cfg.Components[0].WorkDir() != filepath.Join(dir, "mono", "apps", "app") {
		t.Fatalf("paths not normalized: %+v", cfg.Components[0])
	}
	if cfg.Execution.ParallelComponents != 3 {
		t.Fatalf("parallelComponents=%d", cfg.Execution.ParallelComponents)
	}
	if !cfg.Components[0].IsIncludedInSystem() {
		t.Fatal("includeInSystem should default true")
	}
}

func TestLoadLegacyServicesAsMicroservices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 1
openwiki:
  command: npx
mermaid:
  mode: basic
services:
  - id: alpha
    path: ./alpha
    enabled: true
system:
  enabled: false
  output: ./system
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Components) != 1 || cfg.Components[0].Type != "microservice" || cfg.Components[0].Profile != "application" {
		t.Fatalf("legacy service not normalized: %+v", cfg.Components)
	}
	if cfg.Components[0].Repository != filepath.Join(dir, "alpha") {
		t.Fatalf("legacy path not resolved: %s", cfg.Components[0].Repository)
	}
}

func TestRejectEscapingScope(t *testing.T) {
	cfg := Defaults()
	cfg.Components = []ComponentConfig{{ID: "bad", Type: "library", Profile: "reusable", Repository: t.TempDir(), Scope: "../outside", Enabled: true}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid scope")
	}
}

func TestAllDocumentedTypesHaveKnownProfiles(t *testing.T) {
	for _, componentType := range SupportedTypes() {
		profile := ProfileForType(componentType)
		if !KnownProfile(profile) {
			t.Fatalf("type %s maps to unknown profile %s", componentType, profile)
		}
	}
}

func TestLoadNormalizesCrossPlatformSeparatorsSpacesAndUnicode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Workspace With Spaces", "資料")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 2
workspace: ./run data
openwiki:
  command: npx
mermaid:
  mode: basic
components:
  - id: app
    type: monolith
    repository: ./Repository With Spaces
    scope: modules\\order/api
    enabled: true
system:
  enabled: false
  output: ./System Output
  factsPath: ./Facts 資料
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	wantWorkdir := filepath.Join(dir, "Repository With Spaces", "modules", "order", "api")
	if cfg.Components[0].WorkDir() != wantWorkdir {
		t.Fatalf("workdir got %q want %q", cfg.Components[0].WorkDir(), wantWorkdir)
	}
	if !filepath.IsAbs(cfg.Workspace) || !filepath.IsAbs(cfg.System.Output) || !filepath.IsAbs(cfg.System.FactsPath) {
		t.Fatalf("paths were not resolved absolutely: workspace=%q output=%q facts=%q", cfg.Workspace, cfg.System.Output, cfg.System.FactsPath)
	}
}

func TestRejectsPortableAbsoluteScopeOnEveryHost(t *testing.T) {
	for _, scope := range []string{`C:\\outside`, `C:/outside`, `\\\\server\\share\\outside`, `/outside`} {
		cfg := Defaults()
		cfg.Components = []ComponentConfig{{ID: "bad", Type: "library", Profile: "reusable", Repository: t.TempDir(), Scope: scope, Enabled: true}}
		if err := Validate(cfg); err == nil {
			t.Fatalf("expected scope %q to be rejected", scope)
		}
	}
}

func TestRejectsComponentIDsUnsafeAsCrossPlatformPathSegments(t *testing.T) {
	for _, id := range []string{"../escape", `a\\b`, "a/b", "CON", "name.", "a:b"} {
		cfg := Defaults()
		cfg.Components = []ComponentConfig{{ID: id, Type: "library", Profile: "reusable", Repository: t.TempDir(), Enabled: true}}
		if err := Validate(cfg); err == nil {
			t.Fatalf("expected id %q to be rejected", id)
		}
	}
}

func TestLoadV3DocumentationUnitsPacksViewsAndEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 3
workspace: .
openwiki:
  command: npx
mermaid:
  mode: basic
documentation:
  views: [component, domain, flow, catalog, platform]
  catalogs:
    shardBy: [domain, owner]
    maximumRowsPerPage: 50
  evidence:
    include: [src/**, workflows/**]
    exclude: [generated/**]
    maxFileSizeBytes: 10000
components:
  - id: app
    type: modular-monolith
    repository: ./app
    enabled: true
    owners: [team-b, team-a]
    capabilities: [pricing, order-management]
    packs: [workflow, messaging]
documentationUnits:
  - id: order-management
    component: app
    kind: domain
    sourceRoots: [modules/order, workflows/order]
    output: domains/order-management
  - id: submit-order
    component: app
    kind: flow
    sourceRoots: [workflows/order/submit.bpmn]
    relatedUnits: [order-management]
system:
  enabled: false
  output: ./system
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 3 || cfg.SourceVersion != 3 {
		t.Fatalf("versions=%d/%d", cfg.Version, cfg.SourceVersion)
	}
	if len(cfg.DocumentationUnits) != 2 || len(cfg.UnitsForComponent("app")) != 2 {
		t.Fatalf("units=%+v", cfg.DocumentationUnits)
	}
	if cfg.Documentation.Catalogs.MaximumRowsPerPage != 50 || cfg.Documentation.Evidence.MaxFileSizeBytes != 10000 {
		t.Fatalf("documentation config=%+v", cfg.Documentation)
	}
	if got := cfg.Components[0].Packs; len(got) != 2 || got[0] != "messaging" || got[1] != "workflow" {
		t.Fatalf("packs=%v", got)
	}
	if cfg.DocumentationUnits[0].SourceRoots[0] != "modules/order" {
		t.Fatalf("root normalization=%v", cfg.DocumentationUnits[0].SourceRoots)
	}
}

func TestV2LoadsThroughV3CompatibilityAdapter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 2
openwiki:
  command: npx
mermaid:
  mode: basic
components:
  - id: app
    type: microservice
    repository: ./app
    enabled: true
system:
  enabled: false
  output: ./system
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 3 || cfg.SourceVersion != 2 {
		t.Fatalf("versions=%d/%d", cfg.Version, cfg.SourceVersion)
	}
	data, err := cfg.NormalizedJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || cfg.Services != nil {
		t.Fatal("normalized migration is empty or retained legacy services")
	}
}

func TestRejectsInvalidCapabilityPackAndDocumentationUnitReferences(t *testing.T) {
	cfg := Defaults()
	cfg.Components = []ComponentConfig{{ID: "app", Type: "microservice", Profile: "application", Repository: t.TempDir(), Enabled: true, Packs: []string{"not-a-pack"}}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid pack")
	}
	cfg.Components[0].Packs = nil
	cfg.DocumentationUnits = []DocumentationUnitConfig{{ID: "orders", Component: "missing", Kind: "domain"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected unknown component")
	}
	cfg.DocumentationUnits = []DocumentationUnitConfig{{ID: "orders", Component: "app", Kind: "domain", RelatedUnits: []string{"missing"}}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected unknown related unit")
	}
}

func TestDocumentationUnitIDsAreComponentScopedAndCrossReferencesCanBeQualified(t *testing.T) {
	cfg := Defaults()
	cfg.Components = []ComponentConfig{
		{ID: "a", Type: "microservice", Profile: "application", Repository: filepath.Join(t.TempDir(), "a"), Enabled: true},
		{ID: "b", Type: "microservice", Profile: "application", Repository: filepath.Join(t.TempDir(), "b"), Enabled: true},
	}
	cfg.DocumentationUnits = []DocumentationUnitConfig{
		{ID: "orders", Component: "a", Kind: "domain", RelatedUnits: []string{"b/orders"}},
		{ID: "orders", Component: "b", Kind: "domain"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("component-scoped duplicate should be valid: %v", err)
	}
	cfg.DocumentationUnits[0].RelatedUnits = []string{"orders"}
	// Same-component resolution wins and is therefore not ambiguous.
	if err := Validate(cfg); err != nil {
		t.Fatalf("same-component relation should resolve: %v", err)
	}
}

func TestPublishedV3ExamplesLoad(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "wikiforge.example.yaml"),
		filepath.Join("..", "..", "examples", "wikiforge.yaml"),
		filepath.Join("..", "assets", "templates", "wikiforge.yaml"),
	}
	for _, path := range paths {
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			if cfg.Version != CurrentVersion || cfg.SourceVersion != CurrentVersion {
				t.Fatalf("versions=%d/%d", cfg.Version, cfg.SourceVersion)
			}
			if len(cfg.EnabledComponents()) != 1 || len(cfg.DocumentationUnits) != 2 {
				t.Fatalf("unexpected example shape: components=%d units=%d", len(cfg.EnabledComponents()), len(cfg.DocumentationUnits))
			}
		})
	}
}

func TestDocumentationUnitPathsRemainCanonicalAcrossSeparatorStyles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 3
openwiki:
  command: npx
mermaid:
  mode: basic
components:
  - id: app
    type: microservice
    repository: ./app
    enabled: true
documentationUnits:
  - id: orders
    component: app
    kind: domain
    sourceRoots: [modules\orders]
    output: domains\orders
system:
  enabled: false
  output: ./system
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	unit := cfg.DocumentationUnits[0]
	if unit.SourceRoots[0] != "modules/orders" || unit.Output != "domains/orders" {
		t.Fatalf("documentation paths are not canonical: roots=%v output=%q", unit.SourceRoots, unit.Output)
	}
}

func TestRejectsCaseOnlyIdentifierAndOutputCollisions(t *testing.T) {
	root := t.TempDir()
	cfg := Defaults()
	cfg.Components = []ComponentConfig{
		{ID: "App", Type: "microservice", Profile: "application", Repository: filepath.Join(root, "a"), Enabled: true},
		{ID: "app", Type: "microservice", Profile: "application", Repository: filepath.Join(root, "b"), Enabled: true},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected case-only component id collision")
	}

	cfg.Components = []ComponentConfig{{ID: "app", Type: "microservice", Profile: "application", Repository: filepath.Join(root, "app"), Enabled: true}}
	cfg.DocumentationUnits = []DocumentationUnitConfig{
		{ID: "Orders", Component: "app", Kind: "domain", Output: "domains/orders"},
		{ID: "orders", Component: "app", Kind: "flow", Output: "flows/orders"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected case-only documentation unit id collision")
	}

	cfg.DocumentationUnits = []DocumentationUnitConfig{
		{ID: "orders-a", Component: "app", Kind: "domain", Output: "Domains/Orders"},
		{ID: "orders-b", Component: "app", Kind: "domain", Output: "domains/orders"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected case-only documentation output collision")
	}
}

func TestExplicitEmptyAdaptiveListsArePreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.json")
	content := `{
  "version": 3,
  "openwiki": {"command": "npx"},
  "documentation": {
    "views": [],
    "catalogs": {"shardBy": [], "maximumRowsPerPage": 50},
    "evidence": {"include": [], "exclude": [], "maxFileSizeBytes": 1000}
  },
  "mermaid": {"mode": "basic"},
  "components": [{"id": "app", "type": "microservice", "repository": "./app", "enabled": true}],
  "system": {"enabled": false, "output": "./system"}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Documentation.Views == nil || len(cfg.Documentation.Views) != 0 {
		t.Fatalf("explicit empty views were not preserved: %#v", cfg.Documentation.Views)
	}
	if cfg.Documentation.Catalogs.ShardBy == nil || len(cfg.Documentation.Catalogs.ShardBy) != 0 {
		t.Fatalf("explicit empty shard policy was not preserved: %#v", cfg.Documentation.Catalogs.ShardBy)
	}
	if cfg.Documentation.Evidence.Include == nil || cfg.Documentation.Evidence.Exclude == nil {
		t.Fatalf("explicit empty evidence lists were not preserved: include=%#v exclude=%#v", cfg.Documentation.Evidence.Include, cfg.Documentation.Evidence.Exclude)
	}
}

func TestRejectsUnsupportedShardDimensionAndCriticality(t *testing.T) {
	cfg := Defaults()
	cfg.Components = []ComponentConfig{{ID: "app", Type: "microservice", Profile: "application", Repository: t.TempDir(), Enabled: true}}
	cfg.Documentation.Catalogs.ShardBy = []string{"not-a-dimension"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid shard dimension")
	}
	cfg.Documentation.Catalogs.ShardBy = []string{"domain"}
	cfg.DocumentationUnits = []DocumentationUnitConfig{{ID: "orders", Component: "app", Kind: "domain", Criticality: "urgent"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid criticality")
	}
}
