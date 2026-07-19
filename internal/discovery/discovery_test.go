package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
)

func TestInventoryUsesRelativePOSIXEvidenceAndStableFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "order"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "order", "service.go"), []byte("package order\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := BuildInventory(root, "component", "revision", "", nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildInventory(root, "component", "revision", "", nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprint changed: %s != %s", first.Fingerprint, second.Fingerprint)
	}
	for _, ref := range first.Evidence {
		if filepath.IsAbs(ref.Path) || strings.Contains(ref.Path, "\\") || strings.HasPrefix(ref.Path, "../") {
			t.Fatalf("unsafe evidence path: %+v", ref)
		}
	}
}

func TestInventoryRedactsManifestSecretsAndUsesContentSignals(t *testing.T) {
	root := t.TempDir()
	content := "dependencies:\n  - orders\npassword: super-secret\n// controller route\n"
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := BuildInventory(root, "component", "revision", "", nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.ManifestsData) != 1 || strings.Contains(strings.Join(inv.ManifestsData[0].Declarations, "\n"), "super-secret") {
		t.Fatalf("manifest secret leaked: %+v", inv.ManifestsData)
	}
	foundEndpoint := false
	for _, signal := range inv.Signals {
		if signal.Kind == "endpoint" {
			foundEndpoint = true
		}
	}
	if !foundEndpoint {
		t.Fatalf("content signal was not discovered: %+v", inv.Signals)
	}
}

func TestInventoryParsesGenericProjectDeclarationsAndMalformedManifests(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"pom.xml":         "<project><artifactId>root</artifactId><modules><module>api</module><module>persistence</module></modules></project>",
		"settings.gradle": "include ':api', ':persistence'\nincludeBuild('../shared-build')",
		"package.json":    `{"name":"root","workspaces":{"packages":["packages/*"]}}`,
		"go.work":         "go 1.23\nuse (\n ./services/orders\n ./services/catalog\n)",
		"pyproject.toml":  "[project]\nname = 'fixture'",
		"build.gradle":    "implementation 'com.example:orders:1.0'",
		"broken-pom.xml":  "<project>",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := BuildInventory(root, "component", "revision", "", nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, project := range inv.Projects {
		seen[project.Kind+":"+project.Path] = true
	}
	for _, expected := range []string{"maven-module:api", "maven-module:persistence", "gradle-project:api", "gradle-included-build:../shared-build", "node-workspace:packages/*", "go-workspace:./services/orders", "python-project:.", "gradle-dependency:."} {
		if !seen[expected] {
			t.Fatalf("missing project declaration %q: %+v", expected, inv.Projects)
		}
	}
	if len(inv.Diagnostics) == 0 {
		t.Fatalf("malformed manifest was not recorded: %+v", inv)
	}
}

func TestTrackedWorkingTreeChangeInvalidatesEvidenceCache(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("init")
	git("config", "user.email", "wikiforge@test.invalid")
	git("config", "user.name", "WikiForge Test")
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\nconst Value = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "initial")
	cache := filepath.Join(root, ".wikiforge", "cache.json")
	first, err := BuildInventory(root, "component", "", cache, nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\nconst Value = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := BuildInventory(root, "component", "", cache, nil, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatalf("working-tree change reused stale inventory fingerprint: %s", first.Fingerprint)
	}
}

func TestValidationRejectsOverlappingDomainRootsAndMalformedRelationships(t *testing.T) {
	inv := Inventory{SchemaVersion: SchemaVersion, RepositoryID: "x", Fingerprint: "fp", Files: []string{"src/a.go", "src/b.go"}}
	result := SemanticDiscovery{SchemaVersion: SchemaVersion, ComponentID: "x", RepositoryID: "x", DiscoveryMode: "hybrid", InventoryFingerprint: "fp", InventoryVersion: InventoryVersion, PromptVersion: PromptVersion, Repository: RepositoryFinding{Profile: "generic", Status: StatusObserved, Confidence: "high"}, Domains: []DomainFinding{
		{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "a", CandidateKey: "a"}, Status: StatusObserved, Confidence: "high", EvidenceIDs: []string{"ev"}}, SourceRoots: []string{"src"}},
		{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "b", CandidateKey: "b"}, Status: StatusObserved, Confidence: "high", EvidenceIDs: []string{"ev"}}, SourceRoots: []string{"src/b.go"}},
	}}
	inv.Evidence = []EvidenceReference{{ID: "ev", Path: "src/a.go", ContentHash: "hash", Kind: "source-file"}}
	if err := Validate(inv, result); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
}

func TestValidationRetainsOverlappingUncertainDomainCandidates(t *testing.T) {
	inv := Inventory{SchemaVersion: SchemaVersion, RepositoryID: "x", Fingerprint: "fp", Files: []string{"src/a.go"}, Evidence: []EvidenceReference{{ID: "ev", Path: "src/a.go", ContentHash: "hash", Kind: "source-file"}}}
	result := SemanticDiscovery{SchemaVersion: SchemaVersion, ComponentID: "x", RepositoryID: "x", DiscoveryMode: "hybrid", InventoryFingerprint: "fp", InventoryVersion: InventoryVersion, PromptVersion: PromptVersion, Repository: RepositoryFinding{Profile: "generic", Status: StatusObserved, Confidence: "high"}, Domains: []DomainFinding{
		{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "a", CandidateKey: "a"}, Status: StatusUncertain, Confidence: "medium", EvidenceIDs: []string{"ev"}}, SourceRoots: []string{"src"}},
		{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "b", CandidateKey: "b"}, Status: StatusConflicting, Confidence: "low", EvidenceIDs: []string{"ev"}}, SourceRoots: []string{"src"}},
	}}
	if err := Validate(inv, result); err != nil {
		t.Fatalf("uncertain overlap should remain diagnostic: %v", err)
	}
}

func TestStrictSemanticValidationRejectsInventedEvidence(t *testing.T) {
	inv := Inventory{SchemaVersion: SchemaVersion, RepositoryID: "x", Fingerprint: "fp"}
	result := SemanticDiscovery{
		SchemaVersion: SchemaVersion, ComponentID: "x", RepositoryID: "x", DiscoveryMode: "hybrid", InventoryFingerprint: "fp", InventoryVersion: InventoryVersion, PromptVersion: PromptVersion,
		Repository: RepositoryFinding{Profile: "generic", Status: StatusObserved, Confidence: "high"},
		Domains:    []DomainFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "orders"}, Status: StatusObserved, Confidence: "high", EvidenceIDs: []string{"missing"}}}},
	}
	if err := Validate(inv, result); err == nil || !strings.Contains(err.Error(), "unknown evidence") {
		t.Fatalf("expected invented evidence rejection, got %v", err)
	}
}

func TestIdentityResolutionPreservesEvidenceOverlapAndSuffixesCollisions(t *testing.T) {
	previous := IdentityManifest{SchemaVersion: SchemaVersion, ComponentID: "x", Mappings: []IdentityMapping{{ID: "orders", Name: "Orders", CandidateKey: "old", EvidenceIDs: []string{"ev1"}}}}
	result := SemanticDiscovery{
		Domains: []DomainFinding{
			{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "Order Management"}, EvidenceIDs: []string{"ev1"}}},
			{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "Orders"}, EvidenceIDs: []string{"ev2"}}},
		},
	}
	ResolveIdentities("x", previous, &result)
	if result.Domains[0].ID != "orders" || result.Domains[1].ID != "orders-2" {
		t.Fatalf("unexpected identities: %+v", result.Domains)
	}
}

func TestIdentityResolutionNormalizesAllCrossFindingReferences(t *testing.T) {
	result := SemanticDiscovery{
		Modules:       []ModuleFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "module-api", Name: "API"}}}},
		Domains:       []DomainFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "domain-orders", Name: "Orders"}}, ModuleIDs: []string{"module-api"}}},
		Ownership:     []OwnershipFinding{{SubjectID: "module-api", FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "ownership-api", Name: "API ownership"}}}},
		Relationships: []RelationshipFinding{{FromID: "domain-orders", ToID: "module-api", Kind: "depends-on", FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "relationship-orders-api", Name: "Orders API"}}}},
	}
	ResolveIdentities("x", IdentityManifest{}, &result)
	if result.Domains[0].ID != "orders" || result.Modules[0].ID != "api" || result.Domains[0].ModuleIDs[0] != "api" || result.Ownership[0].SubjectID != "api" || result.Relationships[0].FromID != "orders" || result.Relationships[0].ToID != "api" {
		t.Fatalf("cross-finding references were not normalized: %+v", result)
	}
}

func TestIdentityResolutionDoesNotGuessAmbiguousNameReferences(t *testing.T) {
	result := SemanticDiscovery{
		Domains: []DomainFinding{
			{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "orders-a", Name: "Orders"}}},
			{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "orders-b", Name: "Orders"}}},
		},
		Relationships: []RelationshipFinding{{FromID: "orders", ToID: "orders-a", Kind: "related-to", FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "relation", Name: "Orders relation"}}}},
	}
	ResolveIdentities("x", IdentityManifest{}, &result)
	if !strings.HasPrefix(result.Relationships[0].FromID, "ambiguous:") {
		t.Fatalf("ambiguous name reference was guessed: %+v", result.Relationships[0])
	}
}

func TestValidationRejectsDuplicateCandidateKeys(t *testing.T) {
	inv := Inventory{SchemaVersion: SchemaVersion, RepositoryID: "x", Fingerprint: "fp", Files: []string{"a.go"}, Evidence: []EvidenceReference{{ID: "ev", Path: "a.go", ContentHash: "hash", Kind: "source-file"}}}
	result := SemanticDiscovery{
		SchemaVersion: SchemaVersion, ComponentID: "x", RepositoryID: "x", DiscoveryMode: "hybrid", InventoryFingerprint: "fp", InventoryVersion: InventoryVersion, PromptVersion: PromptVersion,
		Repository: RepositoryFinding{Profile: "generic", Status: StatusObserved, Confidence: "high"},
		Domains:    []DomainFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "duplicate", Name: "Orders"}, Status: StatusObserved, Confidence: "high", EvidenceIDs: []string{"ev"}}}},
		Flows:      []FlowFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{CandidateKey: "duplicate", Name: "Submit orders"}, Status: StatusUncertain, Confidence: "medium", EvidenceIDs: []string{"ev"}}}},
	}
	if err := Validate(inv, result); err == nil || !strings.Contains(err.Error(), "candidate key") {
		t.Fatalf("duplicate candidate key was accepted: %v", err)
	}
}

func TestModelCannotChooseStableIdentity(t *testing.T) {
	result := SemanticDiscovery{Domains: []DomainFinding{{FindingBase: FindingBase{ID: "model-chosen", Candidate: SemanticCandidate{Name: "Orders"}, Status: StatusObserved, Confidence: "high"}}}}
	ResolveIdentities("x", IdentityManifest{}, &result)
	if result.Domains[0].ID != "orders" {
		t.Fatalf("model stable ID was accepted: %+v", result.Domains[0])
	}
}

func TestExplicitUnitIDHasIdentityPrecedence(t *testing.T) {
	result := SemanticDiscovery{Domains: []DomainFinding{{FindingBase: FindingBase{Candidate: SemanticCandidate{Name: "Orders"}, Status: StatusObserved, Confidence: "high"}}}}
	applyExplicitIdentities([]config.DocumentationUnitConfig{{Component: "x", ID: "sales-domain", Kind: "domain", Domain: "Orders"}}, "x", &result)
	if result.Domains[0].ID != "sales-domain" || result.Domains[0].Provenance != "explicit" || result.Domains[0].Status != StatusExplicitEnabled {
		t.Fatalf("explicit identity was not applied: %+v", result.Domains[0])
	}
	mapping := ResolveIdentities("x", IdentityManifest{Mappings: []IdentityMapping{{ID: "sales-domain", Accepted: true}}}, &result)
	if len(mapping.Mappings) != 1 || mapping.Mappings[0].ID != "sales-domain" || mapping.Mappings[0].Precedence != "explicit-unit-id" {
		t.Fatalf("unexpected mapping: %+v", mapping)
	}
}

func TestDecodeStageOutputAcceptsOneValidObjectInsideModelReasoning(t *testing.T) {
	output := `The inventory was large, so I inspected the relevant files first.
Here is the required result:
{"schemaVersion":1,"stage":"module-classification","source":"model evidence synthesis","repository":{"profile":"application","description":"application repository","confidence":"high","status":"observed","evidenceIds":[]},"modules":[{"candidate":{"candidateKey":"api","name":"api","description":"REST adapter","evidenceIds":[]},"description":"inbound module","status":"observed","confidence":"high","source":"module manifest","role":"technical"}],"domains":[],"flows":[],"concerns":[],"ownership":[],"relationships":[],"conflicts":[],"unknowns":[{"candidate":{"candidateKey":"missing-domain","name":"missing domain","evidenceIds":[]},"status":"not-observed","confidence":"low"}]}`
	result, err := decodeStageOutput(output, "module-classification")
	if err != nil {
		t.Fatalf("wrapped valid JSON should decode: %v", err)
	}
	if len(result.Unknowns) != 1 || result.Unknowns[0].Candidate == nil || result.Unknowns[0].Candidate.Name != "missing domain" {
		t.Fatalf("unexpected decoded unknown: %+v", result.Unknowns)
	}
	if result.Source != "model evidence synthesis" || result.Modules[0].Source != "module manifest" {
		t.Fatalf("source provenance was not decoded: %+v", result)
	}
	if result.Repository.Description != "application repository" || result.Modules[0].Description != "inbound module" || result.Modules[0].Candidate.Description != "REST adapter" {
		t.Fatalf("description metadata was not decoded: %+v", result)
	}
}

func TestDecodeStageOutputStillRejectsUnsupportedFields(t *testing.T) {
	object := `{"schemaVersion":1,"stage":"module-classification","repository":{"profile":"application","confidence":"high","status":"observed","evidenceIds":[],"unsupported":true},"modules":[],"domains":[],"flows":[],"concerns":[],"ownership":[],"relationships":[],"conflicts":[],"unknowns":[]}`
	if _, err := decodeStageOutput(object, "module-classification"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported discovery field was accepted: %v", err)
	}
}

func TestDecodeStageOutputRejectsMultipleValidObjects(t *testing.T) {
	object := `{"schemaVersion":1,"stage":"module-classification","repository":{"profile":"application","confidence":"high","status":"observed","evidenceIds":[]},"modules":[],"domains":[],"flows":[],"concerns":[],"ownership":[],"relationships":[],"conflicts":[],"unknowns":[]}`
	if _, err := decodeStageOutput(object+object, "module-classification"); err == nil {
		t.Fatal("multiple valid discovery objects must be rejected")
	}
}

type discoveryFixtureRunner struct{}

func (discoveryFixtureRunner) Check(context.Context) error { return nil }
func (discoveryFixtureRunner) Run(_ context.Context, _ string, _ string, prompt string) (string, error) {
	stage := "module-classification"
	for _, candidate := range []string{"module-classification", "concern-flow-extraction", "semantic-synthesis"} {
		if strings.Contains(prompt, "Stage: "+candidate) {
			stage = candidate
		}
	}
	b, err := json.Marshal(model.StageOutput{SchemaVersion: SchemaVersion, Stage: stage, Repository: model.RepositoryFinding{Profile: "modular-application", Status: StatusObserved, Confidence: "high"}, Unknowns: []model.UnknownFinding{{Dimension: "domain", Subject: "fixture", Status: StatusUncertain, Reason: "not enough evidence"}}})
	return string(b), err
}

func TestEngineMakesBoundedStrictStagesAndAcceptsUnknownInsteadOfGuessing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = []config.ComponentConfig{{ID: "x", Profile: "modular-application", Repository: root, Enabled: true}}
	inv, result, _, metrics, err := (Engine{Config: cfg, Runner: discoveryFixtureRunner{}}).Discover(context.Background(), cfg.Components[0], IdentityManifest{})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Fingerprint == "" || metrics.Calls != 3 || !result.Quality.Accepted || len(result.Domains) != 0 {
		t.Fatalf("unexpected discovery: inv=%+v result=%+v metrics=%+v", inv, result, metrics)
	}
}

func TestEngineUsesDeterministicEvidenceBatchesForLargeInventories(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 300; i++ {
		path := filepath.Join(root, "src", "file-"+strings.Repeat("x", 3)+"-"+strings.TrimSpace(string(rune('a'+i%26)))+"-"+fmt.Sprint(i)+".go")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	cfg.Workspace = root
	cfg.Components = []config.ComponentConfig{{ID: "large", Profile: "modular-application", Repository: root, Enabled: true}}
	_, _, _, metrics, err := (Engine{Config: cfg, Runner: discoveryFixtureRunner{}}).Discover(context.Background(), cfg.Components[0], IdentityManifest{})
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Calls != 5 {
		t.Fatalf("expected two batches for each extraction stage plus synthesis, calls=%d", metrics.Calls)
	}
}
