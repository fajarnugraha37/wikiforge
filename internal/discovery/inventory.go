package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/evidence"
)

type Inventory struct {
	SchemaVersion int                  `json:"schemaVersion"`
	RepositoryID  string               `json:"repositoryId"`
	Revision      string               `json:"revision"`
	ComponentRoot string               `json:"componentRoot"`
	Files         []string             `json:"files"`
	Manifests     []string             `json:"manifests,omitempty"`
	Evidence      []EvidenceReference  `json:"evidence"`
	Signals       []Signal             `json:"signals,omitempty"`
	ManifestsData []ManifestEvidence   `json:"manifestEvidence,omitempty"`
	Projects      []ProjectDeclaration `json:"projects,omitempty"`
	Diagnostics   []string             `json:"diagnostics,omitempty"`
	Skipped       []string             `json:"skipped,omitempty"`
	Fingerprint   string               `json:"fingerprint"`
}

type Signal struct {
	Kind       string `json:"kind"`
	EvidenceID string `json:"evidenceId"`
	Path       string `json:"path"`
}

type ManifestEvidence struct {
	EvidenceID   string   `json:"evidenceId"`
	Path         string   `json:"path"`
	Kind         string   `json:"kind"`
	Declarations []string `json:"declarations,omitempty"`
}

func BuildInventory(root, repositoryID, revision, cachePath string, include, exclude []string, maxFileBytes int64) (Inventory, error) {
	canonical, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, fmt.Errorf("resolve component root: %w", err)
	}
	index, err := evidence.BuildIndexCached(canonical, repositoryID, revision, include, exclude, cachePath, maxFileBytes)
	if err != nil {
		return Inventory{}, err
	}
	inv := Inventory{SchemaVersion: SchemaVersion, RepositoryID: repositoryID, Revision: index.Revision, ComponentRoot: "."}
	for _, ref := range index.References {
		path := filepath.ToSlash(filepath.Clean(ref.Path))
		if path == "." || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
			return Inventory{}, fmt.Errorf("evidence registry returned unsafe path %q", ref.Path)
		}
		inv.Files = append(inv.Files, path)
		inv.Evidence = append(inv.Evidence, EvidenceReference{
			ID: ref.ID, Path: path, ContentHash: ref.ContentHash, Kind: string(ref.EvidenceType),
			Locator: EvidenceLocator{LineStart: ref.LineStart, LineEnd: ref.LineEnd},
		})
		content, _ := os.ReadFile(filepath.Join(canonical, filepath.FromSlash(path)))
		for _, kind := range genericSignalKinds(path, string(content)) {
			inv.Signals = append(inv.Signals, Signal{Kind: kind, EvidenceID: ref.ID, Path: path})
		}
		if isGenericManifest(path) || string(ref.EvidenceType) == "configuration" {
			inv.Manifests = append(inv.Manifests, path)
			inv.ManifestsData = append(inv.ManifestsData, ManifestEvidence{
				EvidenceID: ref.ID, Path: path, Kind: string(ref.EvidenceType),
				Declarations: manifestDeclarations(string(content)),
			})
			projects, diagnostics := parseManifest(path, string(content))
			inv.Projects = append(inv.Projects, projects...)
			inv.Diagnostics = append(inv.Diagnostics, diagnostics...)
		}
	}
	for _, skipped := range index.Skipped {
		inv.Skipped = append(inv.Skipped, filepath.ToSlash(skipped.Path)+": "+skipped.Reason)
	}
	sort.Strings(inv.Files)
	sort.Strings(inv.Manifests)
	sort.Slice(inv.ManifestsData, func(i, j int) bool { return inv.ManifestsData[i].Path < inv.ManifestsData[j].Path })
	sort.Strings(inv.Skipped)
	sort.Strings(inv.Diagnostics)
	sort.Slice(inv.Projects, func(i, j int) bool {
		if inv.Projects[i].ManifestPath == inv.Projects[j].ManifestPath {
			return inv.Projects[i].Path < inv.Projects[j].Path
		}
		return inv.Projects[i].ManifestPath < inv.Projects[j].ManifestPath
	})
	sort.Slice(inv.Signals, func(i, j int) bool {
		if inv.Signals[i].Kind == inv.Signals[j].Kind {
			return inv.Signals[i].Path < inv.Signals[j].Path
		}
		return inv.Signals[i].Kind < inv.Signals[j].Kind
	})
	inv.Fingerprint = fingerprint(inv)
	return inv, nil
}

func genericSignalKinds(path, content string) []string {
	value := strings.ToLower(path + "\n" + content)
	checks := []struct {
		kind  string
		terms []string
	}{
		{"endpoint", []string{"controller", "resource", "route", "endpoint", "openapi", "graphql"}},
		{"event", []string{"event", "topic", "queue", "consumer", "producer", "message"}},
		{"data", []string{"entity", "model", "repository", "migration", "schema", "dao", "store"}},
		{"job", []string{"job", "scheduler", "worker", "batch", "cron"}},
		{"security", []string{"security", "auth", "acl", "permission", "policy", "identity"}},
		{"configuration", []string{"config", "configuration", "application", "environment", "feature"}},
		{"deployment", []string{"docker", "compose", "kubernetes", "helm", "terraform", "deploy", "infra"}},
		{"telemetry", []string{"log", "metric", "trace", "health", "monitor", "dashboard"}},
		{"ownership", []string{"codeowners", "owners", "team"}},
	}
	var result []string
	for _, check := range checks {
		for _, term := range check.terms {
			if strings.Contains(value, term) {
				result = append(result, check.kind)
				break
			}
		}
	}
	return result
}

func manifestDeclarations(content string) []string {
	lines := strings.Split(redactSensitiveText(content), "\n")
	declarations := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.Contains(line, "dependencies") || strings.Contains(line, "require") || strings.Contains(line, "project") || strings.Contains(line, "module") || strings.Contains(line, "version") || strings.Contains(line, "scripts") {
			declarations = append(declarations, line)
		}
	}
	return declarations
}

func redactSensitiveText(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "private_key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "api_key") {
			if idx := strings.IndexAny(line, ":="); idx >= 0 {
				lines[i] = line[:idx+1] + " [REDACTED]"
			} else {
				lines[i] = "[REDACTED]"
			}
		}
	}
	return strings.Join(lines, "\n")
}

// Manifest discovery is intentionally generic. It identifies files worth
// presenting to the model without assigning semantics to a language or tool.
func isGenericManifest(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.Contains(name, "manifest") || strings.Contains(name, "lock") || strings.Contains(name, "workspace") {
		return true
	}
	for _, suffix := range []string{".json", ".yaml", ".yml", ".toml", ".xml", ".gradle", ".sbt", ".csproj", ".fsproj", ".mod", ".sum", ".cfg", ".ini"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return name == "makefile" || name == "dockerfile" || name == "pom.xml" || name == "package.json" || name == "pyproject.toml" || name == "go.work"
}

func fingerprint(inv Inventory) string {
	h := sha256.New()
	h.Write([]byte(InventoryVersion))
	for _, ref := range inv.Evidence {
		h.Write([]byte(ref.ID))
		h.Write([]byte{0})
		h.Write([]byte(ref.Path))
		h.Write([]byte{0})
		h.Write([]byte(ref.ContentHash))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
