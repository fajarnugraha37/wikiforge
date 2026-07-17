package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
)

type rule struct {
	pack   string
	tokens []string
	exts   []string
}

var rules = []rule{
	{pack: "api", tokens: []string{"openapi", "swagger", "graphql", "grpc", "protobuf", "endpoint", "controller", "router", "jakarta.ws.rs", "@path", "http handler"}, exts: []string{".proto", ".graphql", ".gql"}},
	{pack: "messaging", tokens: []string{"kafka", "rabbitmq", "nats", "pubsub", "message broker", "eventbus", "event emitter", "consumer", "producer", "dead letter", "dlq"}},
	{pack: "workflow", tokens: []string{"camunda", "zeebe", "workflow", "business process", "process engine", "bpmn"}, exts: []string{".bpmn", ".bpmn20.xml"}},
	{pack: "jobs", tokens: []string{"cron", "scheduler", "scheduled", "jobrunner", "worker pool", "polling", "batch job"}},
	{pack: "files", tokens: []string{"multipart", "file upload", "file download", "sftp", "object storage", "blob storage", "csv", "xlsx", "pdf generation"}},
	{pack: "database", tokens: []string{"database", "postgres", "mysql", "oracle", "mongodb", "cassandra", "dynamodb", "schema", "transaction"}, exts: []string{".sql"}},
	{pack: "migrations", tokens: []string{"migration", "flyway", "liquibase", "seed data", "seeding", "backfill"}},
	{pack: "data-access", tokens: []string{"repository interface", "repository implementation", "data access", "mybatis", "hibernate", "eclipselink", "jdbc", "entitymanager", "dao", "mapper"}},
	{pack: "cache", tokens: []string{"cache", "caching", "redis", "memcached", "caffeine", "ttl"}},
	{pack: "rate-limit", tokens: []string{"rate limit", "ratelimit", "token bucket", "leaky bucket", "sliding window", "quota"}},
	{pack: "distributed-coordination", tokens: []string{"distributed lock", "leader election", "lease", "fencing token", "semaphore", "coordination", "redlock", "zookeeper", "etcd"}},
	{pack: "security", tokens: []string{"authentication", "authorization", "oauth", "openid", "oidc", "jwt", "acl", "permission", "rbac", "abac", "mtls"}},
	{pack: "cryptography", tokens: []string{"encrypt", "decrypt", "cipher", "signature", "hashing", "keystore", "truststore", "certificate", "kms", "hsm"}},
	{pack: "concurrency", tokens: []string{"thread", "mutex", "semaphore", "executor", "goroutine", "channel", "promise", "future", "coroutine", "reactive", "event loop", "context propagation"}},
	{pack: "configuration", tokens: []string{"environment variable", "configmap", "configuration", "application.properties", "application.yml", "feature flag", "secret reference"}, exts: []string{".properties", ".env"}},
	{pack: "container-runtime", tokens: []string{"dockerfile", "kubernetes", "deployment", "statefulset", "helm", "docker compose", "container"}, exts: []string{".dockerfile"}},
	{pack: "telemetry", tokens: []string{"opentelemetry", "telemetry", "tracing", "metrics", "prometheus", "grafana", "span", "trace id", "structured logging"}},
	{pack: "domain", tokens: []string{"domain", "business rule", "invariant", "aggregate", "bounded context", "use case", "business flow"}},
	{pack: "engineering", tokens: []string{"coding standard", "contributing", "lint", "formatter", "architecture decision", "adr"}},
	{pack: "runtime", tokens: []string{"runtime", "startup", "shutdown", "health check", "readiness", "liveness", "graceful shutdown"}},
}

var unitRootNames = map[string]bool{"domain": true, "domains": true, "module": true, "modules": true, "bounded-context": true, "bounded-contexts": true}

func Discover(cfg config.Config, component config.ComponentConfig) (model.DiscoveryManifest, error) {
	manifest := model.DiscoveryManifest{
		SchemaVersion: 1,
		Component:     model.Component{ID: component.ID, Type: component.Type, Profile: component.Profile, Repository: component.Repository, Scope: component.Scope, WorkDir: component.WorkDir(), Group: component.Group, Tags: component.Tags, DependsOn: component.DependsOn, Owners: component.Owners, Capabilities: component.Capabilities, Packs: component.Packs},
	}
	configured := configuredUnits(cfg, component)
	manifest.Units = append(manifest.Units, configured...)
	seenUnits := map[string]bool{}
	coveredRoots := map[string]bool{}
	for _, unit := range configured {
		seenUnits[strings.ToLower(unit.ID)] = true
		for _, root := range unit.SourceRoots {
			coveredRoots[filepath.ToSlash(root)] = true
		}
	}

	includePatterns, err := compileGlobs(cfg.Documentation.Evidence.Include)
	if err != nil {
		return manifest, fmt.Errorf("compile evidence include patterns: %w", err)
	}
	excludePatterns, err := compileGlobs(cfg.Documentation.Evidence.Exclude)
	if err != nil {
		return manifest, fmt.Errorf("compile evidence exclude patterns: %w", err)
	}
	evidence := map[string]map[string]bool{}
	hash := sha256.New()
	workdir := component.WorkDir()
	err = filepath.WalkDir(workdir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if filepath.Clean(path) == filepath.Clean(workdir) {
				return walkErr
			}
			rel, _ := filepath.Rel(workdir, path)
			manifest.Unknowns = append(manifest.Unknowns, "Unreadable evidence path: "+filepath.ToSlash(rel))
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(workdir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if matchesAny(excludePatterns, rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if matchesAny(excludePatterns, rel) || (len(includePatterns) > 0 && !matchesAny(includePatterns, rel)) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			manifest.Unknowns = append(manifest.Unknowns, "Unreadable evidence metadata: "+rel)
			return nil
		}
		if info.Size() > cfg.Documentation.Evidence.MaxFileSizeBytes {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			manifest.Unknowns = append(manifest.Unknowns, "Unreadable evidence file: "+rel)
			return nil
		}
		if looksBinary(b) {
			return nil
		}
		manifest.FilesScanned++
		manifest.BytesScanned += int64(len(b))
		_, _ = fmt.Fprintf(hash, "%s\x00", rel)
		_, _ = hash.Write(b)
		_, _ = hash.Write([]byte{0})
		text := strings.ToLower(rel + "\n" + string(b))
		ext := strings.ToLower(filepath.Ext(rel))
		for _, candidate := range rules {
			if matchesRule(candidate, text, ext) {
				if evidence[candidate.pack] == nil {
					evidence[candidate.pack] = map[string]bool{}
				}
				evidence[candidate.pack][rel] = true
			}
		}
		inferUnitsFromPath(component, rel, seenUnits, coveredRoots, &manifest.Units)
		return nil
	})
	if err != nil {
		return manifest, err
	}
	manifest.SourceHash = hex.EncodeToString(hash.Sum(nil))
	packSet := map[string]bool{}
	for pack, paths := range evidence {
		packSet[pack] = true
		sorted := keys(paths)
		manifest.Evidence = append(manifest.Evidence, model.EvidenceMatch{Pack: pack, Paths: sorted, Count: len(sorted)})
	}
	for _, pack := range component.Packs {
		packSet[pack] = true
	}
	manifest.Packs = keys(packSet)
	sort.Slice(manifest.Evidence, func(i, j int) bool { return manifest.Evidence[i].Pack < manifest.Evidence[j].Pack })
	sort.Slice(manifest.Units, func(i, j int) bool { return manifest.Units[i].ID < manifest.Units[j].ID })
	if manifest.FilesScanned == 0 {
		manifest.Unknowns = append(manifest.Unknowns, "No eligible source files were found within the component scope.")
	}
	if len(manifest.Packs) == 0 {
		manifest.Unknowns = append(manifest.Unknowns, "No optional capability packs were discovered from source evidence.")
	}
	manifest.Unknowns = uniqueStrings(manifest.Unknowns)
	return manifest, nil
}

func configuredUnits(cfg config.Config, component config.ComponentConfig) []model.DocumentationUnit {
	var units []model.DocumentationUnit
	seen := map[string]bool{}
	for _, unit := range cfg.UnitsForComponent(component.ID) {
		units = append(units, model.DocumentationUnit{ID: unit.ID, ComponentID: unit.Component, Kind: unit.Kind, SourceRoots: unit.SourceRoots, RelatedUnits: unit.RelatedUnits, OutputPath: unit.Output, Owners: unit.Owners, Capabilities: unit.Capabilities, Criticality: unit.Criticality, Origin: "configured"})
		seen[strings.ToLower(unit.ID)] = true
	}
	for _, capability := range component.Capabilities {
		id := portableID(capability)
		if id == "" || seen[strings.ToLower(id)] {
			continue
		}
		units = append(units, model.DocumentationUnit{ID: id, ComponentID: component.ID, Kind: "domain", OutputPath: "domains/" + id, Owners: component.Owners, Capabilities: []string{capability}, Origin: "configured-capability"})
		seen[strings.ToLower(id)] = true
	}
	return units
}

func inferUnitsFromPath(component config.ComponentConfig, rel string, seen, covered map[string]bool, units *[]model.DocumentationUnit) {
	parts := strings.Split(rel, "/")
	for index := 0; index+1 < len(parts); index++ {
		rootKind := strings.ToLower(parts[index])
		if !unitRootNames[rootKind] {
			continue
		}
		root := strings.Join(parts[:index+2], "/")
		if coveredBy(root, covered) {
			break
		}
		id := portableID(parts[index+1])
		if id != "" && !seen[strings.ToLower(id)] {
			kind := "domain"
			output := "domains/" + id
			if strings.Contains(rootKind, "module") {
				kind = "module"
				output = "components/" + component.ID + "/modules/" + id
			}
			*units = append(*units, model.DocumentationUnit{ID: id, ComponentID: component.ID, Kind: kind, SourceRoots: []string{root}, OutputPath: output, Origin: "discovered"})
			seen[strings.ToLower(id)] = true
		}
		break
	}
	lower := strings.ToLower(rel)
	if strings.HasSuffix(lower, ".bpmn") || strings.HasSuffix(lower, ".bpmn20.xml") {
		base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		base = strings.TrimSuffix(base, ".bpmn20")
		id := portableID(base)
		if id == "" || seen[strings.ToLower(id)] {
			return
		}
		if coveredBy(rel, covered) {
			return
		}
		*units = append(*units, model.DocumentationUnit{ID: id, ComponentID: component.ID, Kind: "flow", SourceRoots: []string{rel}, OutputPath: "flows/" + id, Origin: "discovered"})
		seen[strings.ToLower(id)] = true
	}
}

func coveredBy(path string, roots map[string]bool) bool {
	path = strings.ToLower(strings.TrimSuffix(filepath.ToSlash(path), "/"))
	for root := range roots {
		root = strings.ToLower(strings.TrimSuffix(filepath.ToSlash(root), "/"))
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func matchesRule(candidate rule, text, ext string) bool {
	for _, wanted := range candidate.exts {
		if ext == wanted || strings.HasSuffix(text, wanted) {
			return true
		}
	}
	for _, token := range candidate.tokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		expression := globExpression(pattern)
		re, err := regexp.Compile(expression)
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func matchesAny(patterns []*regexp.Regexp, value string) bool {
	value = filepath.ToSlash(value)
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func globExpression(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return b.String()
}

func looksBinary(b []byte) bool {
	limit := len(b)
	if limit > 8192 {
		limit = 8192
	}
	for _, c := range b[:limit] {
		if c == 0 {
			return true
		}
	}
	return false
}

func portableID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = true
		}
	}
	return keys(set)
}

func keys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
