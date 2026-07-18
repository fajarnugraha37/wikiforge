package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadV3ComponentsAndScopes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 3
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

func TestLoadV3DocumentationUnitsAndPlanningFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wikiforge.yaml")
	content := `version: 3
openwiki:
  command: npx
mermaid:
  mode: basic
documentation:
  views:
    - component
    - domain
    - catalog
  catalogs:
    shardBy:
      - domain
    maximumRowsPerPage: 120
  evidence:
    include:
      - src/**
    exclude:
      - vendor/**
components:
  - id: commerce-core
    type: modular-monolith
    repository: ./mono
    enabled: true
    owners: [commerce-team]
    capabilities: [order-management, pricing]
    packs: [workflow, telemetry]
documentationUnits:
  - id: submit-order
    component: commerce-core
    kind: flow
    sourceRoots:
      - workflows/order
    relatedUnits:
      - submit-order
    output: flows/submit-order
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
	if cfg.Version != 3 {
		t.Fatalf("version=%d", cfg.Version)
	}
	if cfg.Documentation.Catalogs.MaximumRowsPerPage != 120 {
		t.Fatalf("unexpected documentation config: %+v", cfg.Documentation)
	}
	if len(cfg.Documentation.Views) != 3 || cfg.Documentation.Views[0] != "component" {
		t.Fatalf("views not loaded: %+v", cfg.Documentation.Views)
	}
	if len(cfg.Components[0].Packs) != 2 || cfg.Components[0].Packs[0] != "telemetry" || cfg.Components[0].Packs[1] != "workflow" {
		t.Fatalf("packs not normalized/sorted: %+v", cfg.Components[0].Packs)
	}
	if len(cfg.DocumentationUnits) != 1 || cfg.DocumentationUnits[0].Output != "flows/submit-order" {
		t.Fatalf("documentation unit not loaded: %+v", cfg.DocumentationUnits)
	}
}
