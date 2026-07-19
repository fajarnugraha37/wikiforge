package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
	"github.com/fajarnugraha37/wikiforge/internal/prompts"
)

var ownedPageRE = regexp.MustCompile("openwiki/([^`\\r\\n]+)")

type adaptiveFixtureRunner struct {
	mu         sync.Mutex
	plan       planner.ComponentPlan
	systemPlan *planner.SystemPlan
	calls      int
}

type deterministicAdaptiveRunner struct {
	plan planner.ComponentPlan
}

func (r deterministicAdaptiveRunner) Check(context.Context) error { return nil }

func (r deterministicAdaptiveRunner) Run(_ context.Context, workdir, operation, prompt string) (string, error) {
	if operation == "discovery" {
		return discoveryFixtureResult(prompt)
	}
	if operation == "update" {
		for _, page := range r.plan.Pages {
			if err := writeAdaptiveFixturePage(workdir, r.plan.Pages, page, "README.md"); err != nil {
				return "", err
			}
		}
		return "updated", nil
	}
	match := ownedPageRE.FindStringSubmatch(prompt)
	if len(match) != 2 {
		return "", fmt.Errorf("adaptive prompt did not identify an owned page")
	}
	path := filepath.ToSlash(strings.TrimSpace(match[1]))
	for _, page := range r.plan.Pages {
		if filepath.ToSlash(page.Path) == path {
			return "generated", writeAdaptiveFixturePage(workdir, r.plan.Pages, page, "README.md")
		}
	}
	return "", fmt.Errorf("adaptive prompt selected unplanned page %s", path)
}

func (r *adaptiveFixtureRunner) Check(context.Context) error { return nil }

func (r *adaptiveFixtureRunner) Run(_ context.Context, workdir, _ string, prompt string) (string, error) {
	if strings.Contains(prompt, "You are performing WikiForge semantic discovery") {
		return discoveryFixtureResult(prompt)
	}
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	match := ownedPageRE.FindStringSubmatch(prompt)
	if len(match) != 2 {
		return "", fmt.Errorf("adaptive prompt did not identify an owned page")
	}
	path := filepath.ToSlash(strings.TrimSpace(match[1]))
	if r.systemPlan != nil && fileExists(filepath.Join(workdir, "sources", "manifest.json")) {
		for _, page := range r.systemPlan.Pages {
			if filepath.ToSlash(page.Path) == path {
				return "ok", writeAdaptiveFixturePage(workdir, r.systemPlan.Pages, page, "sources/manifest.json")
			}
		}
		return "", fmt.Errorf("adaptive prompt selected unplanned system page %s", path)
	}
	for _, page := range r.plan.Pages {
		if filepath.ToSlash(page.Path) != path {
			continue
		}
		return "ok", writeAdaptiveFixturePage(workdir, r.plan.Pages, page, "README.md")
	}
	return "", fmt.Errorf("adaptive prompt selected unplanned page %s", path)
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

func TestAdaptiveGenerationUsesPlannedHierarchy(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repository")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "fixture")

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.System.Enabled = true
	cfg.System.Output = filepath.Join(root, "system-wiki")
	cfg.Mermaid.Mode = "off"
	cfg.Components = []config.ComponentConfig{{ID: "app", Type: "microservice", Profile: "application", Repository: repo, Enabled: true, Packs: []string{"api", "configuration", "data", "jobs", "messaging", "security"}}}
	semantic := testPlannerSemantic(cfg)
	planResult, err := (planner.Planner{Config: cfg, Semantic: semantic}).Plan("app", true)
	if err != nil || len(planResult.Components) != 1 {
		t.Fatalf("adaptive plan: %v", err)
	}
	runner := &adaptiveFixtureRunner{plan: planResult.Components[0], systemPlan: planResult.System}
	o := New(cfg, runner, nil)
	result, err := o.Generate(context.Background(), GenerateOptions{ComponentID: "app"})
	if err != nil {
		validationResult := o.Validator.ValidateAdaptiveComponent(context.Background(), cfg.Components[0], planResult.Components[0])
		t.Fatalf("adaptive generate: %v\nvalidation=%+v", err, validationResult)
	}
	if !result.Report.Components["app"].Accepted {
		t.Fatalf("adaptive validation was not accepted: %+v", result.Report.Components["app"])
	}
	plannedCalls := len(planResult.Components[0].Pages)
	if planResult.System != nil {
		plannedCalls += len(planResult.System.Pages)
	}
	if runner.calls != plannedCalls {
		t.Fatalf("runner calls=%d planned pages=%d", runner.calls, plannedCalls)
	}
	for _, path := range []string{"quickstart.md", "components/index.md", "components/app/index.md", "catalogs/index.md"} {
		if _, err := os.Stat(filepath.Join(repo, "openwiki", filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing adaptive page %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "openwiki", "architecture", "overview.md")); !os.IsNotExist(err) {
		t.Fatalf("adaptive generation unexpectedly created an unplanned architecture page: %v", err)
	}
	for _, path := range []string{"quickstart.md", "system/index.md", "system/capability-map.md", "system/component-landscape.md"} {
		if _, err := os.Stat(filepath.Join(cfg.System.Output, "openwiki", filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing adaptive system page %s: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(root, ".wikiforge", "components", "app", "discovery.json"),
		filepath.Join(root, ".wikiforge", "components", "app", "plan.json"),
		filepath.Join(root, ".wikiforge", "components", "app", "evidence-index.json"),
		filepath.Join(root, ".wikiforge", "components", "app", "impact-index.json"),
		filepath.Join(root, ".wikiforge", "components", "app", "coverage.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing adaptive artifact %s: %v", path, err)
		}
	}
	before := snapshotFiles(t, filepath.Join(repo, "openwiki"))
	beforeSnapshots, _ := filepath.Glob(filepath.Join(cfg.System.Output, "sources", "components", "app", "*", "openwiki"))
	callCount := runner.calls
	if _, err := o.Generate(context.Background(), GenerateOptions{ComponentID: "app", UpdateOnly: true}); err != nil {
		t.Fatalf("no-op adaptive update: %v", err)
	}
	if runner.calls != callCount {
		t.Fatalf("no-op update invoked model runner: before=%d after=%d", callCount, runner.calls)
	}
	if after := snapshotFiles(t, filepath.Join(repo, "openwiki")); fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("no-op update changed documentation files: before=%v after=%v", before, after)
	}
	afterSnapshots, _ := filepath.Glob(filepath.Join(cfg.System.Output, "sources", "components", "app", "*", "openwiki"))
	if len(beforeSnapshots) != len(afterSnapshots) {
		t.Fatalf("no-op system update created a new snapshot: before=%v after=%v", beforeSnapshots, afterSnapshots)
	}
}

func TestAdaptiveIncrementalAndFullRegenerationAreEquivalent(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repository")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "fixture")

	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.System.Enabled = false
	cfg.Mermaid.Mode = "off"
	cfg.Components = []config.ComponentConfig{{ID: "app", Type: "application", Profile: "application", Repository: repo, Enabled: true, Packs: []string{"api", "configuration", "data", "jobs", "messaging", "security"}}}
	planResult, err := (planner.Planner{Config: cfg, Semantic: testPlannerSemantic(cfg)}).Plan("app", false)
	if err != nil || len(planResult.Components) != 1 {
		t.Fatalf("adaptive plan: %v", err)
	}
	runner := deterministicAdaptiveRunner{plan: planResult.Components[0]}
	o := New(cfg, runner, nil)
	if _, err := o.Generate(context.Background(), GenerateOptions{ComponentID: "app"}); err != nil {
		t.Fatalf("full generation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\n\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Generate(context.Background(), GenerateOptions{ComponentID: "app", UpdateOnly: true}); err != nil {
		t.Fatalf("incremental generation: %v", err)
	}
	incremental := snapshotFiles(t, filepath.Join(repo, "openwiki"))
	if _, err := o.Generate(context.Background(), GenerateOptions{ComponentID: "app"}); err != nil {
		t.Fatalf("full regeneration: %v", err)
	}
	full := snapshotFiles(t, filepath.Join(repo, "openwiki"))
	if fmt.Sprint(incremental) != fmt.Sprint(full) {
		t.Fatalf("incremental and full regeneration differ:\nincremental=%v\nfull=%v", incremental, full)
	}
}

func discoveryFixtureResult(prompt string) (string, error) {
	stage := "module-classification"
	for _, candidate := range []string{"module-classification", "concern-flow-extraction", "semantic-synthesis"} {
		if strings.Contains(prompt, "Stage: "+candidate) {
			stage = candidate
		}
	}
	b, err := json.Marshal(model.StageOutput{SchemaVersion: model.DiscoverySchemaVersion, Stage: stage, Repository: model.RepositoryFinding{Profile: "application", Status: model.StatusObserved, Confidence: "high"}, Unknowns: []model.UnknownFinding{{Dimension: "repository", Subject: "fixture", Status: model.StatusUncertain, Reason: "deterministic fixture"}}})
	return string(b), err
}

func testPlannerSemantic(cfg config.Config) map[string]model.SemanticDiscovery {
	result := map[string]model.SemanticDiscovery{}
	for _, component := range cfg.Components {
		result[component.ID] = model.SemanticDiscovery{SchemaVersion: model.DiscoverySchemaVersion, ComponentID: component.ID, RepositoryID: component.ID, DiscoveryMode: "hybrid", Repository: model.RepositoryFinding{Profile: component.Profile, Status: model.StatusObserved, Confidence: "high"}}
	}
	return result
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		b, readErr := os.ReadFile(path)
		if readErr == nil {
			files[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	return files
}

func writeAdaptiveFixturePage(workdir string, pages []planner.PlannedPage, page planner.PlannedPage, sourceRef string) error {
	contract := prompts.AdaptivePageContract(page.Path, string(page.Kind))
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntype: Adaptive Fixture\ntitle: %s\ndescription: Evidence-backed adaptive fixture.\ntags:\n  - fixture\n---\n\n# %s\n\n", page.Path, page.Path)
	for _, heading := range contract.RequiredHeadings {
		if heading == "Source References" {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\nEvidence-backed fixture content.\n\n", heading)
	}
	if contract.RequiredTableHeader != "" {
		b.WriteString(contract.RequiredTableHeader + "\n|---|---|---|---|---|\n| entry | fixture | inbound | fixture | `" + sourceRef + "` |\n\n")
	}
	if contract.RequiredDiagram != "" {
		b.WriteString("```mermaid\nflowchart LR\n    A[Source] --> B[Target]\n```\n\n")
	}
	for _, child := range adaptiveFixtureChildren(pages, page) {
		link, err := filepath.Rel(filepath.Dir(filepath.FromSlash(page.Path)), filepath.FromSlash(child))
		if err != nil {
			return err
		}
		link = filepath.ToSlash(link)
		if !strings.Contains(b.String(), "("+link+")") {
			fmt.Fprintf(&b, "- [%s](%s)\n", child, link)
		}
	}
	b.WriteString("\n## Source References\n\n- `" + sourceRef + "`\n")
	path := filepath.Join(workdir, "openwiki", filepath.FromSlash(page.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func adaptiveFixtureChildren(pages []planner.PlannedPage, page planner.PlannedPage) []string {
	var children []string
	path := filepath.ToSlash(page.Path)
	for _, candidate := range pages {
		candidatePath := filepath.ToSlash(candidate.Path)
		if candidatePath == path {
			continue
		}
		if path == "quickstart.md" {
			if candidate.Kind == planner.PageIndex && strings.Count(candidatePath, "/") == 1 {
				children = append(children, candidatePath)
			}
			continue
		}
		base := strings.TrimSuffix(path, "/index.md")
		if page.Kind == planner.PageCollection {
			if candidate.Kind == planner.PageShard && filepath.ToSlash(filepath.Dir(candidatePath)) == base {
				children = append(children, candidatePath)
			}
			continue
		}
		if filepath.ToSlash(filepath.Dir(candidatePath)) == base || (strings.Count(path, "/") == 1 && (candidate.Kind == planner.PageIndex || candidate.Kind == planner.PageCollection) && strings.HasPrefix(candidatePath, base+"/")) {
			children = append(children, candidatePath)
		}
	}
	sort.Strings(children)
	return children
}
