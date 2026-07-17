package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
)

func TestDiscoverDeterministicCapabilitiesUnitsAndEvidenceFilters(t *testing.T) {
	root := t.TempDir()
	write(t, root, "modules/order/OrderResource.java", `@Path("/orders") class OrderResource { KafkaProducer producer; RedisCache cache; }`)
	write(t, root, "workflows/order/submit-order.bpmn", `<definitions><process id="submit-order"/></definitions>`)
	write(t, root, "db/migration/V1__orders.sql", `create table orders(id bigint primary key);`)
	write(t, root, "deploy/deployment.yaml", `kind: Deployment`)
	write(t, root, "generated/ignored.sql", `create table should_not_be_seen(id int);`)
	write(t, root, "blob.bin", "must be excluded by root-level double-star glob")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("opentelemetry should not be followed"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(outside, filepath.Join(root, "external-link.txt")) // unsupported platforms may reject; discovery must skip it when present.

	cfg := config.Defaults()
	cfg.Components = []config.ComponentConfig{{ID: "commerce", Type: "modular-monolith", Profile: "modular-application", Repository: root, Enabled: true, Packs: []string{"telemetry"}, Capabilities: []string{"pricing"}}}
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{{ID: "order-management", Component: "commerce", Kind: "domain", SourceRoots: []string{"modules/order", "workflows/order"}, Output: "domains/order-management"}}
	component := cfg.Components[0]

	first, err := Discover(cfg, component)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Discover(cfg, component)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("discovery is not deterministic:\n%+v\n%+v", first, second)
	}
	for _, pack := range []string{"api", "cache", "container-runtime", "database", "messaging", "migrations", "telemetry", "workflow"} {
		if !contains(first.Packs, pack) {
			t.Errorf("missing pack %s in %v", pack, first.Packs)
		}
	}
	if first.FilesScanned != 4 {
		t.Fatalf("files scanned=%d, want 4", first.FilesScanned)
	}
	if !hasUnit(first, "order-management", "configured") || !hasUnit(first, "pricing", "configured-capability") {
		t.Fatalf("unexpected units: %+v", first.Units)
	}
	if hasUnit(first, "submit-order", "discovered") {
		t.Fatal("BPMN covered by configured source root must not create a duplicate flow unit")
	}

	oldHash := first.SourceHash
	write(t, root, "modules/order/OrderResource.java", `@Path("/orders") class OrderResource { KafkaProducer producer; RedisCache cache; Meter meter; }`)
	changed, err := Discover(cfg, component)
	if err != nil {
		t.Fatal(err)
	}
	if changed.SourceHash == oldHash {
		t.Fatal("source hash did not change")
	}
}

func TestDiscoverInfersModuleAndFlowUnits(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/modules/catalog/README.md", "domain module")
	write(t, root, "processes/reconcile.bpmn", "<process />")
	cfg := config.Defaults()
	cfg.Components = []config.ComponentConfig{{ID: "app", Type: "microservice", Profile: "application", Repository: root, Enabled: true}}
	manifest, err := Discover(cfg, cfg.Components[0])
	if err != nil {
		t.Fatal(err)
	}
	if !hasUnit(manifest, "catalog", "discovered") || !hasUnit(manifest, "reconcile", "discovered") {
		t.Fatalf("units=%+v", manifest.Units)
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
func hasUnit(manifest model.DiscoveryManifest, id, origin string) bool {
	for _, unit := range manifest.Units {
		if unit.ID == id && unit.Origin == origin {
			return true
		}
	}
	return false
}

func TestConfiguredUnitsSuppressCaseEquivalentCapabilityAndInferredUnits(t *testing.T) {
	root := t.TempDir()
	write(t, root, "modules/orders/README.md", "domain")
	cfg := config.Defaults()
	component := config.ComponentConfig{ID: "app", Type: "modular-monolith", Profile: "modular-application", Repository: root, Enabled: true, Capabilities: []string{"orders"}}
	cfg.Components = []config.ComponentConfig{component}
	cfg.DocumentationUnits = []config.DocumentationUnitConfig{{ID: "Orders", Component: "app", Kind: "domain", SourceRoots: []string{"modules/orders"}}}
	manifest, err := Discover(cfg, component)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, unit := range manifest.Units {
		if strings.EqualFold(unit.ID, "orders") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("case-equivalent configured/capability/inferred units were not deduplicated: %+v", manifest.Units)
	}
}

func TestDiscoverFailsWhenComponentRootDoesNotExist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	cfg := config.Defaults()
	component := config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: root, Enabled: true}
	cfg.Components = []config.ComponentConfig{component}
	if _, err := Discover(cfg, component); err == nil {
		t.Fatal("expected missing component root error")
	}
}
