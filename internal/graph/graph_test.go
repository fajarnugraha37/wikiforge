package graph

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/evidence"
)

func TestExportUsesCanonicalPathAndFrontMatterType(t *testing.T) {
	root := t.TempDir()
	writeGraphFile(t, root, "domains/order/index.md", "---\ntype: Domain Index\ntitle: Order\n---\n\n# Order\n\n[Workflow](/domains/order/workflow.md)\n")
	writeGraphFile(t, root, "domains/order/workflow.md", "---\ntype: Workflow\ntitle: Workflow\n---\n\n# Workflow\n")
	output := filepath.Join(t.TempDir(), "graph")
	if err := Export("app", root, output); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(filepath.Join(output, "nodes.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	found := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var node Node
		if err := json.Unmarshal(scanner.Bytes(), &node); err != nil {
			t.Fatal(err)
		}
		if node.ID == "app:path:domains/order/workflow" {
			found = node.Type == "Workflow"
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("canonical workflow node with frontmatter type was not exported")
	}

	edges, err := os.ReadFile(filepath.Join(output, "edges.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(edges), "app:path:domains/order/index") || !strings.Contains(string(edges), "app:path:domains/order/workflow") {
		t.Fatalf("absolute document link was not exported: %s", edges)
	}
}

func TestExportUsesStableCrossComponentConceptsAndEvidenceEdges(t *testing.T) {
	root := t.TempDir()
	writeGraphFile(t, root, "a.md", "# A\n\n| Subject | Relationship | Object | Evidence | Authority | Confidence |\n|---|---|---|---|---|---|\n| id: order.service | CALLS | id: order.repository | src/order.go | tracked | high |\n")
	output := filepath.Join(t.TempDir(), "graph")
	index := evidence.Index{References: []evidence.EvidenceReference{{ID: "evidence:1", Path: "src/order.go"}}, Dependencies: []evidence.EvidenceDependency{{EvidenceID: "evidence:1", PageID: "a", SectionID: "Source References"}}}
	if err := ExportWithEvidence("component-a", root, output, index); err != nil {
		t.Fatal(err)
	}
	edges, err := os.ReadFile(filepath.Join(output, "edges.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(edges)
	if !strings.Contains(text, `"from":"concept:order-service"`) || !strings.Contains(text, `"relationship":"SUPPORTED_BY"`) {
		t.Fatalf("stable concept or evidence edge missing: %s", text)
	}
}

func writeGraphFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
