package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
	"github.com/example/wikiforge/internal/prompts"
)

func TestEveryProfileCanSatisfyItsContract(t *testing.T) {
	for _, profileID := range prompts.ProfileIDs() {
		profileID := profileID
		t.Run(profileID, func(t *testing.T) {
			dir := t.TempDir()
			repo := filepath.Join(dir, "component")
			if err := os.MkdirAll(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("source"), 0o644); err != nil {
				t.Fatal(err)
			}
			profile, _ := prompts.GetProfile(profileID)
			writeFixtureWiki(t, filepath.Join(repo, "openwiki"), prompts.ComponentPageContracts(profile), prompts.ExpectedFiles(profile), "README.md")
			cfg := config.Defaults()
			cfg.Workspace = dir
			cfg.Mermaid.Mode = "basic"
			component := config.ComponentConfig{ID: "component", Type: typeForProfile(profileID), Profile: profileID, Repository: repo, Enabled: true}
			v := Validator{Config: cfg}
			r := v.ValidateComponent(context.Background(), component)
			if !r.Accepted {
				t.Fatalf("expected accepted, score=%d findings=%+v", r.Score, r.Findings)
			}
		})
	}
}

func TestUnsupportedFrontmatterIsRejected(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "component")
	_ = os.MkdirAll(repo, 0o755)
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("source"), 0o644)
	profile, _ := prompts.GetProfile("application")
	writeFixtureWiki(t, filepath.Join(repo, "openwiki"), prompts.ComponentPageContracts(profile), prompts.ExpectedFiles(profile), "README.md")
	quick := filepath.Join(repo, "openwiki", "quickstart.md")
	b, _ := os.ReadFile(quick)
	b = []byte(strings.Replace(string(b), "tags:\n", "owner: team-a\ntags:\n", 1))
	_ = os.WriteFile(quick, b, 0o644)
	cfg := config.Defaults()
	cfg.Mermaid.Mode = "basic"
	r := (Validator{Config: cfg}).ValidateComponent(context.Background(), config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: repo, Enabled: true})
	if r.Accepted || !hasFinding(r, "DOC-FRONTMATTER-UNSUPPORTED") {
		t.Fatalf("expected unsupported frontmatter finding: %+v", r.Findings)
	}
}

func writeFixtureWiki(t *testing.T, root string, contracts map[string]model.PageContract, files []string, sourceRef string) {
	t.Helper()
	for _, rel := range files {
		p := contracts[rel]
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "---\ntype: Test Document\ntitle: %s\ndescription: Generated contract fixture for validation.\ntags:\n  - test\n---\n\n# %s\n\n", rel, titleFromPath(rel))
		for _, h := range p.RequiredHeadings {
			if h == "Source References" {
				fmt.Fprintf(&b, "## %s\n\n- `%s`\n\n", h, sourceRef)
			} else {
				fmt.Fprintf(&b, "## %s\n\nEvidence-backed content for %s.\n\n", h, h)
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
			b.WriteString("## Knowledge Gaps\n\nNone identified in fixture.\n\n")
		}
		if p.RequiredDiagram != "" {
			writeDiagram(&b, p.RequiredDiagram)
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
			t.Fatal(err)
		}
	}
}

func writeDiagram(b *strings.Builder, required string) {
	if required == "sequenceDiagram" {
		b.WriteString("```mermaid\nsequenceDiagram\n    participant A\n    participant B\n    A->>B: verified call\n```\n\n")
		return
	}
	if required == "stateDiagram-v2" {
		b.WriteString("```mermaid\nstateDiagram-v2\n    [*] --> Ready\n```\n\n")
		return
	}
	if required == "erDiagram" {
		b.WriteString("```mermaid\nerDiagram\n    A ||--o{ B : contains\n```\n\n")
		return
	}
	if required == "classDiagram" {
		b.WriteString("```mermaid\nclassDiagram\n    class A\n    class B\n    A --> B\n```\n\n")
		return
	}
	b.WriteString("```mermaid\nflowchart LR\n    A[Source] --> B[Target]\n```\n\n")
}

func titleFromPath(path string) string {
	return strings.Title(strings.ReplaceAll(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), "-", " "))
}

func typeForProfile(profile string) string {
	switch profile {
	case "application":
		return "microservice"
	case "modular-application":
		return "modular-monolith"
	case "reusable":
		return "library"
	case "infrastructure":
		return "iac"
	case "configuration":
		return "configuration"
	case "contracts":
		return "contracts"
	default:
		return "generic"
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

func TestSpecializedCatalogTableIsRequired(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "component")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	profile, _ := prompts.GetProfile("application")
	writeFixtureWiki(t, filepath.Join(repo, "openwiki"), prompts.ComponentPageContracts(profile), prompts.ExpectedFiles(profile), "README.md")
	path := filepath.Join(repo, "openwiki", "interfaces", "endpoint-catalog.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contract := prompts.ComponentPageContracts(profile)["interfaces/endpoint-catalog.md"]
	b = []byte(strings.Replace(string(b), contract.RequiredTableHeader, "| Wrong | Header |", 1))
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Mermaid.Mode = "basic"
	result := (Validator{Config: cfg}).ValidateComponent(context.Background(), config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: repo, Enabled: true})
	if result.Accepted || !hasFinding(result, "DOC-REQUIRED-TABLE") {
		t.Fatalf("expected required table failure: %+v", result.Findings)
	}
}

func TestRendererArgsPreserveAbsolutePathsWithSpacesAndUnicode(t *testing.T) {
	args := rendererArgs([]string{"-i", "{input}", "-o", "{output}"}, `C:/Project With Spaces/資料/input.mmd`, `/tmp/Output With Spaces/図.svg`)
	want := []string{"-i", `C:/Project With Spaces/資料/input.mmd`, "-o", `/tmp/Output With Spaces/図.svg`}
	if fmt.Sprint(args) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", args, want)
	}
}
