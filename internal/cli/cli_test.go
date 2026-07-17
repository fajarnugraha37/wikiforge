package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/wikiforge/internal/config"
)

func TestDiscoverAndExplainPlanPersistAdaptiveArtifacts(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(repo, "modules", "orders"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "modules", "orders", "api.go"), []byte("http handler kafka producer redis cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "wikiforge.yaml")
	writeConfig(t, cfgPath, `version: 3
workspace: .
openwiki:
  command: npx
mermaid:
  mode: basic
components:
  - id: app
    type: modular-monolith
    repository: ./app
    enabled: true
    packs: [workflow]
documentationUnits:
  - id: orders
    component: app
    kind: domain
    sourceRoots: [modules/orders]
system:
  enabled: false
  output: ./system
`)
	var out, errOut bytes.Buffer
	command := CLI{Out: &out, Err: &errOut}
	if code := command.Run(context.Background(), []string{"discover", "--config", cfgPath, "--component", "app"}); code != 0 {
		t.Fatalf("discover code=%d err=%s", code, errOut.String())
	}
	for _, name := range []string{"discovery.json", "plan.json"} {
		path := filepath.Join(root, ".wikiforge", "components", "app", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	out.Reset()
	errOut.Reset()
	if code := command.Run(context.Background(), []string{"plan", "--config", cfgPath, "--component", "app", "--skip-system", "--explain"}); code != 0 {
		t.Fatalf("plan code=%d err=%s", code, errOut.String())
	}
	text := out.String()
	for _, wanted := range []string{"adaptive plan component=app", "catalogs/interfaces/index.md", "orders", "skip"} {
		if !strings.Contains(text, wanted) {
			t.Errorf("plan missing %q:\n%s", wanted, text)
		}
	}
}

func TestConfigMigrateWritesNormalizedV3(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "v2.yaml")
	output := filepath.Join(root, "v3.json")
	writeConfig(t, cfgPath, `version: 2
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
`)
	var out, errOut bytes.Buffer
	command := CLI{Out: &out, Err: &errOut}
	if code := command.Run(context.Background(), []string{"config", "migrate", "--config", cfgPath, "--output", output}); code != 0 {
		t.Fatalf("code=%d err=%s", code, errOut.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var migrated struct {
		Version            int   `json:"version"`
		DocumentationUnits []any `json:"documentationUnits"`
	}
	if err := json.Unmarshal(data, &migrated); err != nil {
		t.Fatal(err)
	}
	if migrated.Version != 3 {
		t.Fatalf("version=%d", migrated.Version)
	}
	if strings.Contains(string(data), filepath.ToSlash(root)+"/") {
		t.Fatalf("migration unexpectedly embedded absolute workspace paths: %s", data)
	}
	if !strings.Contains(out.String(), "source version 2 to version 3") {
		t.Fatalf("output=%s", out.String())
	}
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "README.md"), []byte("# app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var secondOut, secondErr bytes.Buffer
	if code := (CLI{Out: &secondOut, Err: &secondErr}).Run(context.Background(), []string{"plan", "--config", output, "--component", "app", "--skip-system"}); code != 0 {
		t.Fatalf("migrated config is not reloadable: code=%d err=%s", code, secondErr.String())
	}
}

func writeConfig(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanReturnsFailureForUnknownComponent(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "wikiforge.yaml")
	writeConfig(t, cfgPath, `version: 3
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
`)
	var out, errOut bytes.Buffer
	code := (CLI{Out: &out, Err: &errOut}).Run(context.Background(), []string{"plan", "--config", cfgPath, "--component", "missing", "--skip-system"})
	if code == 0 || !strings.Contains(errOut.String(), "no enabled component matched") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestCheckComponentEnvironmentRejectsMissingDocumentationUnitRoot(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: repo, Enabled: true}
	units := []config.DocumentationUnitConfig{{ID: "orders", Component: "app", Kind: "domain", SourceRoots: []string{"modules/orders"}}}
	err := checkComponentEnvironment(component, units)
	if err == nil || !strings.Contains(err.Error(), "documentation unit orders source root") {
		t.Fatalf("unexpected error: %v", err)
	}
}
