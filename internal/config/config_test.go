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
