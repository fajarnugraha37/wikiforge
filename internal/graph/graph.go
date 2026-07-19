package graph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/evidence"
)

type Node struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Source string `json:"source"`
}

type Edge struct {
	From         string `json:"from"`
	Relationship string `json:"relationship"`
	To           string `json:"to"`
	Source       string `json:"source"`
	Evidence     string `json:"evidence,omitempty"`
	Authority    string `json:"authority,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
}

var mdLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+\.md(?:#[^)]+)?)\)`)
var controlledRelationships = map[string]bool{
	"AUTHORIZES": true, "CALLS": true, "CONSUMES": true, "DEPENDS_ON": true,
	"EXPOSES": true, "EXTENDS": true, "IMPLEMENTS": true, "IMPORTS": true,
	"LINKS_TO": true, "OWNS": true, "PART_OF": true, "PERSISTS": true,
	"PRODUCES": true, "REFERENCES": true, "RELATED_TO": true, "VALIDATES": true,
	"SUPPORTED_BY": true,
}

func Export(targetID, docsRoot, outputRoot string) error {
	return ExportWithEvidence(targetID, docsRoot, outputRoot, evidence.Index{})
}

func ExportWithEvidence(targetID, docsRoot, outputRoot string, index evidence.Index) error {
	var files []string
	if err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	var nodes []Node
	var edges []Edge
	idByPath := map[string]string{}
	for _, path := range files {
		rel, _ := filepath.Rel(docsRoot, path)
		rel = filepath.ToSlash(rel)
		id := targetID + ":path:" + canonicalPath(rel)
		idByPath[filepath.Clean(path)] = id
		content, _ := os.ReadFile(path)
		nodes = append(nodes, Node{ID: id, Type: documentType(string(content)), Title: documentTitle(string(content), rel), Source: rel})
	}
	for _, path := range files {
		contentBytes, _ := os.ReadFile(path)
		content := string(contentBytes)
		from := idByPath[filepath.Clean(path)]
		rel, _ := filepath.Rel(docsRoot, path)
		rel = filepath.ToSlash(rel)
		for _, m := range mdLinkRE.FindAllStringSubmatch(content, -1) {
			targetPart := strings.Split(m[2], "#")[0]
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(targetPart)))
			if strings.HasPrefix(targetPart, "/") {
				resolved = filepath.Clean(filepath.Join(docsRoot, filepath.FromSlash(strings.TrimPrefix(targetPart, "/"))))
			}
			to, ok := idByPath[resolved]
			if ok {
				edges = append(edges, Edge{From: from, Relationship: "LINKS_TO", To: to, Source: rel})
			}
		}
		edges = append(edges, relationshipTableEdges(rel, content)...)
	}
	knownEvidence := map[string]evidence.EvidenceReference{}
	for _, ref := range index.References {
		knownEvidence[ref.ID] = ref
		nodes = append(nodes, Node{ID: ref.ID, Type: "Evidence", Title: ref.Path, Source: ref.Path})
	}
	for _, dependency := range index.Dependencies {
		from := targetID + ":path:" + canonicalPath(dependency.PageID)
		if _, ok := idByPath[filepath.Join(docsRoot, filepath.FromSlash(dependency.PageID+".md"))]; !ok {
			from = targetID + ":path:" + canonicalPath(dependency.PageID)
		}
		if _, ok := knownEvidence[dependency.EvidenceID]; ok {
			edges = append(edges, Edge{From: from, Relationship: "SUPPORTED_BY", To: dependency.EvidenceID, Source: dependency.SectionID})
		}
	}
	known := map[string]bool{}
	for _, n := range nodes {
		known[n.ID] = true
	}
	for _, e := range edges {
		for _, id := range []string{e.From, e.To} {
			if !known[id] {
				title := id
				if strings.HasPrefix(id, "concept:") {
					title = strings.ReplaceAll(strings.TrimPrefix(id, "concept:"), "-", " ")
				}
				nodes = append(nodes, Node{ID: id, Type: "Concept", Title: title, Source: e.Source})
				known[id] = true
			}
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outputRoot, "nodes.jsonl"), nodes); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outputRoot, "edges.jsonl"), edges); err != nil {
		return err
	}
	return nil
}

func documentTitle(content, fallback string) string {
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}

func documentType(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) == "---" {
				break
			}
			if strings.HasPrefix(strings.TrimSpace(line), "type:") {
				value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "type:"))
				if value != "" {
					return value
				}
			}
		}
	}
	return "Document"
}

func canonicalPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func relationshipTableEdges(source, content string) []Edge {
	var edges []Edge
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inTable := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "| Subject | Relationship | Object |") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(trim, "|") {
			inTable = false
			continue
		}
		cells := splitTableRow(trim)
		if len(cells) < 3 || isSeparator(cells[0]) {
			continue
		}
		from := "concept:" + conceptIdentity(cells[0])
		to := "concept:" + conceptIdentity(cells[2])
		relationship := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(cells[1]), " ", "_"))
		if !controlledRelationships[relationship] {
			relationship = "RELATED_TO"
		}
		edge := Edge{From: from, Relationship: relationship, To: to, Source: source}
		if len(cells) > 3 {
			edge.Evidence = cells[3]
		}
		if len(cells) > 4 {
			edge.Authority = cells[4]
		}
		if len(cells) > 5 {
			edge.Confidence = cells[5]
		}
		edges = append(edges, edge)
	}
	return edges
}

func conceptIdentity(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, prefix := range []string{"concept id:", "concept_id:", "id:"} {
		if strings.HasPrefix(lower, prefix) {
			return slug(strings.TrimSpace(value[len(prefix):]))
		}
	}
	if strings.HasPrefix(value, "[") {
		if close := strings.Index(value, "]("); close >= 0 {
			value = value[1:close]
		}
	}
	return slug(value)
}

func splitTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparator(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && strings.Trim(s, "-:") == ""
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func writeJSONL[T any](path string, values []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return err
		}
	}
	return w.Flush()
}
