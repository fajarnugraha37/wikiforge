package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
)

func TestAdaptiveValidationRejectsUnsupportedFrontmatter(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "openwiki")
	writeDocument(t, filepath.Join(docs, "quickstart.md"), "---\ntype: Test\ntitle: Test\ndescription: Test\nowner: team-a\ntags: []\n---\n\n# Test\n\n## Navigation\n\ncontent\n\n## Knowledge Gaps\n\nNone.\n\n## Source References\n\n- `README.md`\n")
	cfg := config.Defaults()
	cfg.Documentation.RequireMermaid = false
	cfg.Documentation.ValidateSourcePaths = false
	pl := planner.ComponentPlan{ComponentID: "app", Profile: "application", Pages: []planner.PlannedPage{{Path: "quickstart.md", Kind: planner.PageSingle}}}
	result := (Validator{Config: cfg}).ValidateAdaptiveComponent(context.Background(), config.ComponentConfig{ID: "app", Repository: root}, pl)
	if result.Accepted || !hasFinding(result, "DOC-FRONTMATTER-UNSUPPORTED") {
		t.Fatalf("expected unsupported frontmatter finding: %+v", result.Findings)
	}
}

func TestRendererArgsPreserveAbsolutePathsWithSpacesAndUnicode(t *testing.T) {
	args := rendererArgs([]string{"-i", "{input}", "-o", "{output}"}, `C:/Project With Spaces/資料/input.mmd`, `/tmp/Output With Spaces/図.svg`)
	want := []string{"-i", `C:/Project With Spaces/資料/input.mmd`, "-o", `/tmp/Output With Spaces/図.svg`}
	if fmt.Sprint(args) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", args, want)
	}
}

func TestMermaidRenderCacheKeyIsStable(t *testing.T) {
	cfg := config.Defaults()
	cfg.Mermaid.CacheDirectory = t.TempDir()
	v := Validator{Config: cfg}
	first := v.mermaidCachePath("flowchart LR\n A --> B")
	second := v.mermaidCachePath("flowchart LR\n A --> B")
	if first == "" || first != second || !strings.HasSuffix(first, ".svg") {
		t.Fatalf("unstable Mermaid cache key: %q %q", first, second)
	}
}

func TestSecretLeakIsRejected(t *testing.T) {
	root := t.TempDir()
	writeDocument(t, filepath.Join(root, "index.md"), "---\ntype: Test\ntitle: Test\ndescription: Test\ntags: []\n---\n\n# Test\n\napi_key: abcdefghijklmnop\n")
	cfg := config.Defaults()
	cfg.Documentation.RequireSourceReferences = false
	cfg.Documentation.ValidateSourcePaths = false
	cfg.Documentation.RequireMermaid = false
	result := (Validator{Config: cfg}).validate(context.Background(), "test", root, root, []string{"index.md"}, nil, "adaptive/test", 0, 0)
	if !hasFinding(result, "SECRET-LEAK") {
		t.Fatalf("expected secret leakage finding: %+v", result.Findings)
	}
}

func TestAbsoluteLinksAndEvidenceIdentityFailuresAreReported(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: Test\ntitle: Test\ndescription: Test\ntags: []\n---\n\n# Test\n\n[Missing](/domains/missing.md)\n\nVerified claim.\n\nConcept ID: shared.order\n"
	writeDocument(t, filepath.Join(root, "index.md"), content)
	writeDocument(t, filepath.Join(root, "other.md"), strings.Replace(content, "Missing", "Other", 1))
	cfg := config.Defaults()
	cfg.Documentation.RequireSourceReferences = false
	cfg.Documentation.ValidateSourcePaths = false
	cfg.Documentation.RequireMermaid = false
	v := Validator{Config: cfg}
	result := v.validate(context.Background(), "test", root, root, []string{"index.md", "other.md"}, nil, "adaptive/test", 0, 0)
	if !hasFinding(result, "DOC-BROKEN-ABSOLUTE-LINK") {
		t.Fatalf("missing absolute link finding: %+v", result.Findings)
	}
	findings := ValidateEvidenceBacked(root, evidence.Index{SchemaVersion: evidence.SchemaVersion})
	if !hasFinding(model.ValidationResult{Findings: findings}, "EVIDENCE-UNRESOLVED") || !hasFinding(model.ValidationResult{Findings: findings}, "GRAPH-DUPLICATE-CONCEPT") {
		t.Fatalf("missing evidence or identity findings: %+v", findings)
	}
}

func TestAdaptiveCatalogRequiresStableIdentityAndEvidence(t *testing.T) {
	root := t.TempDir()
	writeDocument(t, filepath.Join(root, "catalog.md"), "---\ntype: Catalog\ntitle: Catalog\ndescription: Catalog\ntags: [test]\nwikiforge:\n  generated: true\n  unit: orders\n---\n\n# Catalog\n\n## Catalog Scope\n\nOrders.\n\n## Catalog Entries\n\n| ID | Name | Direction | Owner | Evidence |\n|---|---|---|---|---|\n| order.created | Created | outbound | orders | `src/orders.go` |\n| order.created | Duplicate | outbound | orders | `src/orders.go` |\n\n## Knowledge Gaps\n\nNone.\n\n## Source References\n\n- `src/orders.go`\n")
	cfg := config.Defaults()
	cfg.Documentation.RequireMermaid = false
	cfg.Documentation.RequireSourceReferences = false
	result := (Validator{Config: cfg}).validate(context.Background(), "catalog", root, root, []string{"catalog.md"}, map[string]model.PageContract{
		"catalog.md": {RequiredHeadings: []string{"Catalog Scope", "Catalog Entries", "Knowledge Gaps", "Source References"}, RequiredTableHeader: "| ID | Name | Direction | Owner | Evidence |"},
	}, "adaptive/application", 0, 0)
	if !hasFinding(result, "CATALOG-DUPLICATE-ID") {
		t.Fatalf("duplicate catalog identity was not rejected: %+v", result.Findings)
	}
}

func writeDocument(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasFinding(result model.ValidationResult, code string) bool {
	for _, finding := range result.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
