package evidence

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/planner"
)

func TestBuildIndexFiltersFilesAndAttachesDocumentation(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init")
	gitTest(t, root, "config", "user.email", "wikiforge@test.invalid")
	gitTest(t, root, "config", "user.name", "WikiForge Test")
	writeTestFile(t, root, "src/main.go", "package main\n")
	writeTestFile(t, root, "vendor/dependency.go", "package dependency\n")
	writeTestFile(t, root, "generated/client.go", "package generated\n")
	writeTestFile(t, root, "binary.dat", string([]byte{'a', 0, 'b'}))
	writeTestFile(t, root, "openwiki/quickstart.md", "# Quickstart\n\n## Source References\n\n- `../src/main.go`\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "fixture")

	index, err := BuildIndex(root, "repo", gitRevision(root), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasReference(index, "src/main.go") {
		t.Fatalf("source file was not indexed: %+v", index.References)
	}
	if hasReference(index, "vendor/dependency.go") || hasReference(index, "generated/client.go") || hasReference(index, "openwiki/quickstart.md") {
		t.Fatalf("excluded paths were indexed: %+v", index.References)
	}
	if !hasSkipped(index, "binary.dat") {
		t.Fatalf("binary file was not recorded as skipped: %+v", index.Skipped)
	}

	index, err = AttachDocumentation(filepath.Join(root, "openwiki"), index)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Dependencies) != 1 || index.Dependencies[0].PageID != "quickstart" {
		t.Fatalf("unexpected documentation dependency: %+v", index.Dependencies)
	}
}

func TestChangedPathsAndImpactAreScoped(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init")
	gitTest(t, root, "config", "user.email", "wikiforge@test.invalid")
	gitTest(t, root, "config", "user.name", "WikiForge Test")
	writeTestFile(t, root, "src/order.go", "package order\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "initial")
	previous := gitRevision(root)
	writeTestFile(t, root, "src/order.go", "package order\n\n// changed\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "change")
	current := gitRevision(root)
	changed, full, _, err := ChangedPaths(root, previous, current)
	if err != nil || full || len(changed) != 1 || changed[0] != "src/order.go" {
		t.Fatalf("unexpected changed paths: changed=%v full=%t err=%v", changed, full, err)
	}

	index, err := BuildIndex(root, "repo", current, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := planner.PlannedPage{Path: "domains/order/index.md", OwnerUnit: "order"}
	plan := planner.ComponentPlan{ComponentID: "app", Units: []planner.DocumentationUnit{{ID: "order", SourceRoots: []string{"src"}}}, Pages: []planner.PlannedPage{page}}
	impact := BuildImpact(index, plan, previous, current, changed, false, "test")
	if len(impact.AffectedPages[page.Path]) == 0 || len(impact.AffectedUnits["order"]) == 0 {
		t.Fatalf("change did not affect expected page/unit: %+v", impact)
	}
}

func TestBuildIndexCachedUsesGitObjectIdentity(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init")
	gitTest(t, root, "config", "user.email", "wikiforge@test.invalid")
	gitTest(t, root, "config", "user.name", "WikiForge Test")
	writeTestFile(t, root, "src/main.go", "package main\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "fixture")
	cache := filepath.Join(root, ".wikiforge", "evidence-cache.json")
	first, err := BuildIndexCached(root, "repo", gitRevision(root), nil, nil, cache, MaxEvidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	if first.CacheMisses == 0 {
		t.Fatalf("first index should read source files: %+v", first)
	}
	second, err := BuildIndexCached(root, "repo", gitRevision(root), nil, nil, cache, MaxEvidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	if second.CacheHits == 0 || second.CacheMisses != 0 {
		t.Fatalf("second index did not reuse unchanged Git object: %+v", second)
	}
}

func TestBuildImpactWithPreviousInvalidatesDeletedEvidence(t *testing.T) {
	page := planner.PlannedPage{Path: "domains/order/index.md", OwnerUnit: "order"}
	plan := planner.ComponentPlan{
		ComponentID: "app",
		Units:       []planner.DocumentationUnit{{ID: "order", SourceRoots: []string{"src"}}},
		Pages:       []planner.PlannedPage{page},
	}
	previous := Index{
		RepositoryID: "repo",
		References:   []EvidenceReference{{ID: "evidence:deleted", Path: "src/order.go"}},
		Dependencies: []EvidenceDependency{{EvidenceID: "evidence:deleted", PageID: "domains/order/index", SectionID: "Source References"}},
	}
	impact := BuildImpactWithPrevious(Index{RepositoryID: "repo"}, previous, plan, "old", "new", []string{"src/order.go"}, false, "test")
	if len(impact.AffectedPages[page.Path]) == 0 {
		t.Fatalf("deleted evidence did not invalidate former page: %+v", impact)
	}
}

func hasReference(index Index, path string) bool {
	for _, ref := range index.References {
		if ref.Path == path {
			return true
		}
	}
	return false
}

func hasSkipped(index Index, path string) bool {
	for _, skipped := range index.Skipped {
		if skipped.Path == path {
			return true
		}
	}
	return false
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitTest(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
