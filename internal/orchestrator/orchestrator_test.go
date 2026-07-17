package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
	"github.com/example/wikiforge/internal/prompts"
)

type fakeRunner struct {
	mu                  sync.Mutex
	calls               int
	profilesByWorkdir   map[string]string
	activeByRepo        map[string]int
	maxActiveByRepo     map[string]int
	repositoryByWorkdir map[string]string
}

func (f *fakeRunner) Check(context.Context) error { return nil }
func (f *fakeRunner) Run(_ context.Context, workdir, operation, prompt string) (string, error) {
	f.mu.Lock()
	f.calls++
	repo := f.repositoryByWorkdir[filepath.Clean(workdir)]
	f.activeByRepo[repo]++
	if f.activeByRepo[repo] > f.maxActiveByRepo[repo] {
		f.maxActiveByRepo[repo] = f.activeByRepo[repo]
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.activeByRepo[repo]--
		f.mu.Unlock()
	}()

	// Make accidental same-repository concurrency observable.
	time.Sleep(2 * time.Millisecond)
	if fileExists(filepath.Join(workdir, "sources", "manifest.json")) {
		if err := writeSystemWikiFixture(workdir); err != nil {
			return "", err
		}
		return "ok", nil
	}
	profileID := f.profilesByWorkdir[filepath.Clean(workdir)]
	profile, err := prompts.GetProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := writeComponentWikiFixture(workdir, profile); err != nil {
		return "", err
	}
	return "ok", nil
}

func TestGenerateAllProfilesMonorepoAndSystemEndToEnd(t *testing.T) {
	root := t.TempDir()
	mono := filepath.Join(root, "repositories", "mono")
	standalone := filepath.Join(root, "repositories", "standalone")
	for _, repo := range []string{mono, standalone} {
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		gitInit(t, repo)
	}

	components := []config.ComponentConfig{
		{ID: "app", Type: "microservice", Profile: "application", Repository: mono, Scope: "apps/app", Enabled: true},
		{ID: "modular", Type: "modular-monolith", Profile: "modular-application", Repository: mono, Scope: "apps/modular", Enabled: true},
		{ID: "lib", Type: "framework", Profile: "reusable", Repository: mono, Scope: "packages/lib", Enabled: true},
		{ID: "contracts", Type: "contracts", Profile: "contracts", Repository: mono, Scope: "contracts", Enabled: true},
		{ID: "iac", Type: "iac", Profile: "infrastructure", Repository: standalone, Scope: "iac", Enabled: true},
		{ID: "config", Type: "configuration", Profile: "configuration", Repository: standalone, Scope: "config", Enabled: true},
		{ID: "generic", Type: "generic", Profile: "generic", Repository: standalone, Scope: "misc", Enabled: true},
	}
	for i := range components {
		workdir := components[i].WorkDir()
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workdir, "README.md"), []byte("# "+components[i].ID+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{mono, standalone} {
		git(t, repo, "add", ".")
		git(t, repo, "commit", "-m", "fixture")
	}

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = components
	cfg.System.Output = filepath.Join(root, "enterprise-wiki")
	cfg.System.FactsPath = filepath.Join(root, "facts")
	cfg.Mermaid.Mode = "basic"
	cfg.Execution.ParallelComponents = 4
	cfg.Execution.MaxRepairRounds = 1

	fr := &fakeRunner{profilesByWorkdir: map[string]string{}, repositoryByWorkdir: map[string]string{}, activeByRepo: map[string]int{}, maxActiveByRepo: map[string]int{}}
	for _, component := range components {
		fr.profilesByWorkdir[filepath.Clean(component.WorkDir())] = component.Profile
		fr.repositoryByWorkdir[filepath.Clean(component.WorkDir())] = filepath.Clean(component.Repository)
	}
	fr.repositoryByWorkdir[filepath.Clean(cfg.System.Output)] = filepath.Clean(cfg.System.Output)

	o := New(cfg, fr, io.Discard)
	res, err := o.Generate(context.Background(), GenerateOptions{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(res.Report.Components) != len(components) {
		t.Fatalf("expected %d component reports, got %d", len(components), len(res.Report.Components))
	}
	for _, component := range components {
		result := res.Report.Components[component.ID]
		if !result.Accepted {
			t.Fatalf("component %s not accepted: %+v", component.ID, result.Findings)
		}
		if _, err := os.Stat(filepath.Join(component.DocumentationRoot(), "quickstart.md")); err != nil {
			t.Fatalf("component %s missing wiki: %v", component.ID, err)
		}
	}
	if res.Report.System == nil || !res.Report.System.Accepted {
		t.Fatalf("system not accepted: %+v", res.Report.System)
	}
	for repo, maxActive := range fr.maxActiveByRepo {
		if repo == "" {
			continue
		}
		if maxActive > 1 {
			t.Fatalf("repository %s had %d concurrent OpenWiki calls; expected serialization", repo, maxActive)
		}
	}
	manifestPath := filepath.Join(cfg.System.Output, "sources", "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Components []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Profile string `json:"profile"`
			Scope   string `json:"scope"`
		} `json:"components"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Components) != len(components) {
		t.Fatalf("manifest components=%d", len(manifest.Components))
	}
	if manifest.Components[0].ID == "" || manifest.Components[0].Type == "" || manifest.Components[0].Profile == "" {
		t.Fatalf("manifest lacks component identity: %+v", manifest.Components[0])
	}
	for _, path := range []string{
		filepath.Join(root, ".wikiforge", "graph", "app", "nodes.jsonl"),
		filepath.Join(root, ".wikiforge", "graph", "system", "edges.jsonl"),
		filepath.Join(cfg.System.Output, "openwiki", "system", "infrastructure-deployment.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}

	fr.mu.Lock()
	callsBeforeUpdate := fr.calls
	fr.mu.Unlock()
	if _, err := o.Generate(context.Background(), GenerateOptions{UpdateOnly: true}); err != nil {
		t.Fatalf("no-op update: %v", err)
	}
	fr.mu.Lock()
	callsAfterUpdate := fr.calls
	fr.mu.Unlock()
	if callsAfterUpdate != callsBeforeUpdate {
		t.Fatalf("unchanged scoped update made %d unexpected model calls", callsAfterUpdate-callsBeforeUpdate)
	}

	appReadme := filepath.Join(components[0].WorkDir(), "README.md")
	if err := os.WriteFile(appReadme, []byte("# app\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Generate(context.Background(), GenerateOptions{UpdateOnly: true, ComponentID: "app", SkipSystem: true}); err != nil {
		t.Fatalf("changed scoped update: %v", err)
	}
	fr.mu.Lock()
	callsAfterChangedUpdate := fr.calls
	fr.mu.Unlock()
	if callsAfterChangedUpdate != callsAfterUpdate+1 {
		t.Fatalf("changed scoped update made %d calls; expected one full incremental update call", callsAfterChangedUpdate-callsAfterUpdate)
	}
}

func writeComponentWikiFixture(repo string, profile prompts.Profile) error {
	return writeWikiFixture(repo, prompts.ComponentPageContracts(profile), prompts.ExpectedFiles(profile), "README.md")
}

func writeSystemWikiFixture(repo string) error {
	return writeWikiFixture(repo, prompts.SystemPageContracts(), prompts.ExpectedSystemFiles(), "README.md")
}

func writeWikiFixture(repo string, contracts map[string]model.PageContract, files []string, sourceRef string) error {
	root := filepath.Join(repo, "openwiki")
	for _, rel := range files {
		p := contracts[rel]
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "---\ntype: Generated Test\ntitle: %s\ndescription: Complete generated fixture for an end-to-end test.\ntags:\n  - generated\n---\n\n# %s\n\n", rel, rel)
		for _, h := range p.RequiredHeadings {
			if h == "Source References" {
				fmt.Fprintf(&b, "## %s\n\n- `%s`\n\n", h, sourceRef)
			} else {
				fmt.Fprintf(&b, "## %s\n\nEvidence-grounded fixture content.\n\n", h)
			}
		}
		if p.RequiredTableHeader != "" {
			b.WriteString(p.RequiredTableHeader + "\n")
			cols := strings.Count(p.RequiredTableHeader, "|") - 1
			if cols < 1 {
				cols = 1
			}
			b.WriteString("|" + strings.Repeat("---|", cols) + "\n")
			b.WriteString("|" + strings.Repeat(" Evidence |", cols) + "\n\n")
		}
		if len(p.RequiredHeadings) == 0 {
			b.WriteString("## Knowledge Gaps\n\nNone.\n\n")
		}
		if p.RequiredDiagram != "" {
			writeTestDiagram(&b, p.RequiredDiagram)
		}
		if rel == "quickstart.md" {
			b.WriteString("## Canonical Links\n\n")
			for _, target := range files {
				if target != rel {
					fmt.Fprintf(&b, "- [%s](%s)\n", target, target)
				}
			}
		}
		if rel == "knowledge/relationships.md" {
			b.WriteString("\n| Subject | Relationship | Object | Evidence | Authority | Confidence |\n|---|---|---|---|---|---|\n| Component | OWNS | Capability | `" + sourceRef + "` | Verified | High |\n")
		}
		if !strings.Contains(b.String(), "## Source References") {
			b.WriteString("\n## Source References\n\n- `" + sourceRef + "`\n")
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeTestDiagram(b *strings.Builder, required string) {
	switch required {
	case "sequenceDiagram":
		b.WriteString("```mermaid\nsequenceDiagram\n    participant A\n    participant B\n    A->>B: call\n```\n\n")
	case "erDiagram":
		b.WriteString("```mermaid\nerDiagram\n    A ||--o{ B : contains\n```\n\n")
	case "classDiagram":
		b.WriteString("```mermaid\nclassDiagram\n    class A\n    class B\n    A --> B\n```\n\n")
	default:
		b.WriteString("```mermaid\nflowchart LR\n    A[Source] --> B[Target]\n```\n\n")
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "test@example.invalid")
	git(t, dir, "config", "user.name", "Test")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestRemoveObsoleteDocumentation(t *testing.T) {
	root := t.TempDir()
	obsolete := []string{
		"runtime/traffic-and-request-flows.md",
		"security/authentication-and-authorization.md",
		"runtime/concurrency-and-asynchronous-processing.md",
	}
	for _, rel := range obsolete {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("obsolete"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(root, "runtime", "context-propagation.md")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeObsoleteDocumentation(root, obsolete)
	for _, rel := range obsolete {
		if fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			t.Fatalf("obsolete documentation not removed: %s", rel)
		}
	}
	if !fileExists(keep) {
		t.Fatal("unrelated canonical documentation was removed")
	}
}
