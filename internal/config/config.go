package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/example/wikiforge/internal/pathutil"
)

const CurrentVersion = 3

var defaultViews = []string{"system", "domain", "component", "flow", "catalog", "platform", "engineering", "operations"}

var knownViews = map[string]bool{
	"system": true, "domain": true, "component": true, "flow": true,
	"catalog": true, "platform": true, "engineering": true, "operations": true,
}

var knownUnitKinds = map[string]bool{
	"domain": true, "subdomain": true, "bounded-context": true, "component": true,
	"module": true, "flow": true, "platform": true, "catalog": true,
}

var shardDimensions = []string{"bounded-context", "component", "criticality", "data-store", "domain", "owner", "repository", "runtime", "subdomain", "transport"}

var knownShardDimensions = map[string]bool{
	"domain": true, "subdomain": true, "bounded-context": true, "component": true,
	"owner": true, "repository": true, "runtime": true, "transport": true,
	"data-store": true, "criticality": true,
}

var criticalities = []string{"critical", "high", "low", "medium"}
var knownCriticalities = map[string]bool{"": true, "low": true, "medium": true, "high": true, "critical": true}

var capabilityPacks = []string{
	"api", "cache", "concurrency", "configuration", "container-runtime", "cryptography",
	"data-access", "database", "distributed-coordination", "domain", "engineering",
	"files", "jobs", "messaging", "migrations", "rate-limit", "runtime", "security",
	"telemetry", "workflow",
}

var defaultPacksByProfile = map[string][]string{
	"application":         {"api", "configuration", "domain", "engineering", "runtime", "security", "telemetry"},
	"modular-application": {"api", "configuration", "domain", "engineering", "runtime", "security", "telemetry"},
	"reusable":            {"api", "configuration", "concurrency", "engineering", "security"},
	"infrastructure":      {"configuration", "container-runtime", "engineering", "security", "telemetry"},
	"configuration":       {"configuration", "engineering", "security"},
	"contracts":           {"api", "domain", "engineering"},
	"generic":             {"configuration", "engineering"},
}

type Config struct {
	Version            int                       `json:"version"`
	Workspace          string                    `json:"workspace"`
	OpenWiki           OpenWikiConfig            `json:"openwiki"`
	Execution          ExecutionConfig           `json:"execution"`
	Documentation      DocumentationConfig       `json:"documentation"`
	Mermaid            MermaidConfig             `json:"mermaid"`
	Components         []ComponentConfig         `json:"components"`
	DocumentationUnits []DocumentationUnitConfig `json:"documentationUnits,omitempty"`
	// Services is retained only for backward compatibility with v1 configurations.
	Services      []ServiceConfig `json:"services,omitempty"`
	System        SystemConfig    `json:"system"`
	SourceVersion int             `json:"-"`
}

type OpenWikiConfig struct {
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	ModelID        string            `json:"modelId"`
	TimeoutMinutes int               `json:"timeoutMinutes"`
	Environment    map[string]string `json:"environment"`
}

type ExecutionConfig struct {
	ParallelComponents         int  `json:"parallelComponents"`
	ParallelServices           int  `json:"parallelServices,omitempty"` // legacy alias
	MaxProcessRetries          int  `json:"maxProcessRetries"`
	MaxRepairRounds            int  `json:"maxRepairRounds"`
	ContinueOnComponentFailure bool `json:"continueOnComponentFailure"`
	ContinueOnServiceFailure   bool `json:"continueOnServiceFailure,omitempty"` // legacy alias
}

type DocumentationConfig struct {
	Language                string         `json:"language"`
	MinimumQualityScore     int            `json:"minimumQualityScore"`
	MinimumPages            int            `json:"minimumPages"`
	RequireFrontMatter      bool           `json:"requireFrontMatter"`
	RequireSourceReferences bool           `json:"requireSourceReferences"`
	ValidateSourcePaths     bool           `json:"validateSourcePaths"`
	RequireMermaid          bool           `json:"requireMermaid"`
	MinimumMermaidBlocks    int            `json:"minimumMermaidBlocks"`
	AllowedDiagramTypes     []string       `json:"allowedDiagramTypes"`
	Views                   []string       `json:"views,omitempty"`
	Catalogs                CatalogConfig  `json:"catalogs,omitempty"`
	Evidence                EvidenceConfig `json:"evidence,omitempty"`
}

type CatalogConfig struct {
	ShardBy            []string `json:"shardBy,omitempty"`
	MaximumRowsPerPage int      `json:"maximumRowsPerPage,omitempty"`
}

type EvidenceConfig struct {
	Include          []string `json:"include,omitempty"`
	Exclude          []string `json:"exclude,omitempty"`
	MaxFileSizeBytes int64    `json:"maxFileSizeBytes,omitempty"`
}

type MermaidConfig struct {
	Mode           string   `json:"mode"`
	Command        string   `json:"command"`
	Args           []string `json:"args"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type ComponentConfig struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	Profile         string   `json:"profile"`
	Repository      string   `json:"repository"`
	Scope           string   `json:"scope"`
	Path            string   `json:"path,omitempty"` // accepted legacy alias
	Enabled         bool     `json:"enabled"`
	IncludeInSystem *bool    `json:"includeInSystem"`
	Group           string   `json:"group"`
	Tags            []string `json:"tags"`
	DependsOn       []string `json:"dependsOn"`
	Owners          []string `json:"owners,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	Packs           []string `json:"packs,omitempty"`
}

type DocumentationUnitConfig struct {
	ID           string   `json:"id"`
	Component    string   `json:"component"`
	Kind         string   `json:"kind"`
	SourceRoots  []string `json:"sourceRoots,omitempty"`
	RelatedUnits []string `json:"relatedUnits,omitempty"`
	Output       string   `json:"output,omitempty"`
	Owners       []string `json:"owners,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Criticality  string   `json:"criticality,omitempty"`
}

// ServiceConfig is the legacy v1 shape.
type ServiceConfig struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

type SystemConfig struct {
	Enabled   bool     `json:"enabled"`
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Output    string   `json:"output"`
	FactsPath string   `json:"factsPath"`
	Tags      []string `json:"tags"`
}

func Defaults() Config {
	return Config{
		Version:       CurrentVersion,
		SourceVersion: CurrentVersion,
		Workspace:     ".",
		OpenWiki: OpenWikiConfig{
			Command:        "npx",
			Args:           []string{"--yes", "openwiki@0.2.0", "code"},
			TimeoutMinutes: 60,
			Environment: map[string]string{
				"OPENWIKI_TELEMETRY_DISABLED":      "1",
				"OPENWIKI_PROVIDER_RETRY_ATTEMPTS": "3",
			},
		},
		Execution: ExecutionConfig{
			ParallelComponents:         2,
			MaxProcessRetries:          2,
			MaxRepairRounds:            2,
			ContinueOnComponentFailure: true,
		},
		Documentation: DocumentationConfig{
			Language:                "English",
			MinimumQualityScore:     85,
			MinimumPages:            0,
			RequireFrontMatter:      true,
			RequireSourceReferences: true,
			ValidateSourcePaths:     true,
			RequireMermaid:          true,
			MinimumMermaidBlocks:    0,
			AllowedDiagramTypes:     []string{"flowchart", "sequenceDiagram", "stateDiagram-v2", "erDiagram", "classDiagram", "architecture-beta", "gitGraph", "mindmap"},
			Views:                   append([]string(nil), defaultViews...),
			Catalogs:                CatalogConfig{ShardBy: []string{"domain", "owner"}, MaximumRowsPerPage: 150},
			Evidence: EvidenceConfig{
				Include:          []string{"**"},
				Exclude:          []string{".git/**", ".wikiforge/**", "openwiki/**", "vendor/**", "node_modules/**", "dist/**", "build/**", "target/**", "generated/**", ".wikiforge-prompt-*.md", "**/*.bin"},
				MaxFileSizeBytes: 2 * 1024 * 1024,
			},
		},
		Mermaid: MermaidConfig{
			Mode:           "render",
			Command:        "npx",
			Args:           []string{"--yes", "@mermaid-js/mermaid-cli@11.12.0", "-i", "{input}", "-o", "{output}", "--quiet"},
			TimeoutSeconds: 90,
		},
		System: SystemConfig{Enabled: true, ID: "enterprise-system", Title: "Enterprise System", Output: "./enterprise-wiki", FactsPath: "./facts"},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	var raw any
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		if err := json.Unmarshal(b, &raw); err != nil {
			return cfg, err
		}
	} else {
		raw, err = parseYAML(string(b))
		if err != nil {
			return cfg, fmt.Errorf("parse yaml: %w", err)
		}
	}
	jb, err := json.Marshal(raw)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(jb, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return cfg, err
	}
	if cfg.Workspace == "" {
		cfg.Workspace = "."
	}
	cfg.Workspace, err = pathutil.Resolve(base, cfg.Workspace)
	if err != nil {
		return cfg, fmt.Errorf("workspace path: %w", err)
	}
	if err := normalizeLegacy(&cfg); err != nil {
		return cfg, err
	}
	for i := range cfg.Components {
		if err := normalizeComponent(base, &cfg.Components[i]); err != nil {
			return cfg, fmt.Errorf("component %q paths: %w", cfg.Components[i].ID, err)
		}
	}
	for i := range cfg.DocumentationUnits {
		if err := normalizeDocumentationUnit(&cfg.DocumentationUnits[i]); err != nil {
			return cfg, fmt.Errorf("documentation unit %q: %w", cfg.DocumentationUnits[i].ID, err)
		}
	}
	cfg.System.Output, err = pathutil.Resolve(base, cfg.System.Output)
	if err != nil {
		return cfg, fmt.Errorf("system.output path: %w", err)
	}
	if cfg.System.FactsPath != "" {
		cfg.System.FactsPath, err = pathutil.Resolve(base, cfg.System.FactsPath)
		if err != nil {
			return cfg, fmt.Errorf("system.factsPath: %w", err)
		}
	}
	applyDefaults(&cfg)
	return cfg, Validate(cfg)
}

func normalizeLegacy(c *Config) error {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Version < 1 || c.Version > CurrentVersion {
		return fmt.Errorf("unsupported config version %d; supported versions are 1, 2, and %d", c.Version, CurrentVersion)
	}
	c.SourceVersion = c.Version
	for _, s := range c.Services {
		c.Components = append(c.Components, ComponentConfig{ID: s.ID, Type: "microservice", Repository: s.Path, Enabled: s.Enabled})
	}
	c.Services = nil
	c.Version = CurrentVersion
	return nil
}

func normalizeComponent(base string, c *ComponentConfig) error {
	c.ID = strings.TrimSpace(c.ID)
	c.Type = normalizeType(c.Type)
	c.Profile = strings.TrimSpace(strings.ToLower(c.Profile))
	normalizedScope, err := pathutil.NormalizeRelative(c.Scope)
	if err != nil {
		return fmt.Errorf("scope: %w", err)
	}
	c.Scope = normalizedScope
	if c.Repository == "" {
		c.Repository = c.Path
	}
	c.Repository, err = pathutil.Resolve(base, c.Repository)
	if err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	c.Path = ""
	if c.Type == "" {
		c.Type = "generic"
	}
	if c.Profile == "" {
		c.Profile = ProfileForType(c.Type)
	}
	if c.IncludeInSystem == nil {
		v := true
		c.IncludeInSystem = &v
	}
	c.Tags = sortedUnique(c.Tags)
	c.DependsOn = sortedUnique(c.DependsOn)
	c.Owners = sortedUnique(c.Owners)
	c.Capabilities = sortedUnique(c.Capabilities)
	for i := range c.Packs {
		c.Packs[i] = normalizeID(c.Packs[i])
	}
	c.Packs = sortedUnique(c.Packs)
	return nil
}

func normalizeDocumentationUnit(u *DocumentationUnitConfig) error {
	u.ID = strings.TrimSpace(u.ID)
	u.Component = strings.TrimSpace(u.Component)
	u.Kind = normalizeID(u.Kind)
	u.Output = strings.TrimSpace(u.Output)
	var err error
	if u.Output != "" {
		u.Output, err = pathutil.NormalizeRelative(u.Output)
		if err != nil {
			return fmt.Errorf("output: %w", err)
		}
		// Documentation paths are serialized and consumed as bundle-relative paths,
		// so keep them canonical across Windows, macOS, and Linux.
		u.Output = filepath.ToSlash(u.Output)
	}
	for i, root := range u.SourceRoots {
		u.SourceRoots[i], err = pathutil.NormalizeRelative(root)
		if err != nil {
			return fmt.Errorf("sourceRoots[%d]: %w", i, err)
		}
		u.SourceRoots[i] = filepath.ToSlash(u.SourceRoots[i])
	}
	u.SourceRoots = sortedUnique(u.SourceRoots)
	u.RelatedUnits = sortedUnique(u.RelatedUnits)
	u.Owners = sortedUnique(u.Owners)
	u.Capabilities = sortedUnique(u.Capabilities)
	u.Criticality = normalizeID(u.Criticality)
	return nil
}

func applyDefaults(c *Config) {
	d := Defaults()
	c.Version = CurrentVersion
	if c.SourceVersion == 0 {
		c.SourceVersion = CurrentVersion
	}
	if c.OpenWiki.Command == "" {
		c.OpenWiki.Command = d.OpenWiki.Command
	}
	if len(c.OpenWiki.Args) == 0 {
		c.OpenWiki.Args = d.OpenWiki.Args
	}
	if c.OpenWiki.TimeoutMinutes <= 0 {
		c.OpenWiki.TimeoutMinutes = d.OpenWiki.TimeoutMinutes
	}
	if c.OpenWiki.Environment == nil {
		c.OpenWiki.Environment = d.OpenWiki.Environment
	}
	if c.Execution.ParallelComponents <= 0 {
		if c.Execution.ParallelServices > 0 {
			c.Execution.ParallelComponents = c.Execution.ParallelServices
		} else {
			c.Execution.ParallelComponents = d.Execution.ParallelComponents
		}
	}
	if !c.Execution.ContinueOnComponentFailure && c.Execution.ContinueOnServiceFailure {
		c.Execution.ContinueOnComponentFailure = true
	}
	if c.Execution.MaxProcessRetries < 0 {
		c.Execution.MaxProcessRetries = 0
	}
	if c.Execution.MaxRepairRounds < 0 {
		c.Execution.MaxRepairRounds = 0
	}
	if c.Documentation.Language == "" {
		c.Documentation.Language = d.Documentation.Language
	}
	if c.Documentation.MinimumQualityScore <= 0 {
		c.Documentation.MinimumQualityScore = d.Documentation.MinimumQualityScore
	}
	if len(c.Documentation.AllowedDiagramTypes) == 0 {
		c.Documentation.AllowedDiagramTypes = d.Documentation.AllowedDiagramTypes
	}
	if c.Documentation.Views == nil {
		c.Documentation.Views = append([]string(nil), d.Documentation.Views...)
	}
	for i := range c.Documentation.Views {
		c.Documentation.Views[i] = normalizeID(c.Documentation.Views[i])
	}
	c.Documentation.Views = sortedUnique(c.Documentation.Views)
	if c.Documentation.Catalogs.ShardBy == nil {
		c.Documentation.Catalogs.ShardBy = append([]string(nil), d.Documentation.Catalogs.ShardBy...)
	}
	c.Documentation.Catalogs.ShardBy = sortedUnique(c.Documentation.Catalogs.ShardBy)
	if c.Documentation.Catalogs.MaximumRowsPerPage <= 0 {
		c.Documentation.Catalogs.MaximumRowsPerPage = d.Documentation.Catalogs.MaximumRowsPerPage
	}
	if c.Documentation.Evidence.Include == nil {
		c.Documentation.Evidence.Include = append([]string(nil), d.Documentation.Evidence.Include...)
	}
	if c.Documentation.Evidence.Exclude == nil {
		c.Documentation.Evidence.Exclude = append([]string(nil), d.Documentation.Evidence.Exclude...)
	}
	if c.Documentation.Evidence.MaxFileSizeBytes <= 0 {
		c.Documentation.Evidence.MaxFileSizeBytes = d.Documentation.Evidence.MaxFileSizeBytes
	}
	if c.Mermaid.Mode == "" {
		c.Mermaid.Mode = d.Mermaid.Mode
	}
	if c.Mermaid.Command == "" {
		c.Mermaid.Command = d.Mermaid.Command
	}
	if len(c.Mermaid.Args) == 0 {
		c.Mermaid.Args = d.Mermaid.Args
	}
	if c.Mermaid.TimeoutSeconds <= 0 {
		c.Mermaid.TimeoutSeconds = d.Mermaid.TimeoutSeconds
	}
	if c.System.ID == "" {
		c.System.ID = d.System.ID
	}
	if c.System.Title == "" {
		c.System.Title = d.System.Title
	}
}

func Validate(c Config) error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("configuration was not normalized to version %d", CurrentVersion)
	}
	if c.OpenWiki.Command == "" {
		return errors.New("openwiki.command is required")
	}
	allIDs := map[string]bool{}
	portableComponentIDs := map[string]string{}
	enabledIDs := map[string]bool{}
	workdirs := map[string]string{}
	for _, component := range c.Components {
		if component.ID == "" {
			if component.Enabled {
				return errors.New("every enabled component requires id and repository")
			}
			continue
		}
		if err := pathutil.ValidatePortableSegment(component.ID); err != nil {
			return fmt.Errorf("component id %q is not a portable path segment: %w", component.ID, err)
		}
		if allIDs[component.ID] {
			return fmt.Errorf("duplicate component id %q", component.ID)
		}
		portableID := strings.ToLower(component.ID)
		if other, exists := portableComponentIDs[portableID]; exists {
			return fmt.Errorf("component ids %q and %q differ only by case and are not portable", other, component.ID)
		}
		portableComponentIDs[portableID] = component.ID
		allIDs[component.ID] = true
		for _, pack := range component.Packs {
			if !KnownCapabilityPack(pack) {
				return fmt.Errorf("component %q has unsupported capability pack %q", component.ID, pack)
			}
		}
		if !component.Enabled {
			continue
		}
		if component.Repository == "" {
			return errors.New("every enabled component requires id and repository")
		}
		enabledIDs[component.ID] = true
		if !KnownProfile(component.Profile) {
			return fmt.Errorf("component %q has unsupported profile %q", component.ID, component.Profile)
		}
		if err := validateScope(component.Scope); err != nil {
			return fmt.Errorf("component %q scope: %w", component.ID, err)
		}
		workdir := filepath.Clean(component.WorkDir())
		workdirKey := strings.ToLower(filepath.ToSlash(workdir))
		if other, ok := workdirs[workdirKey]; ok {
			return fmt.Errorf("components %q and %q resolve to the same portable work directory %q", other, component.ID, workdir)
		}
		workdirs[workdirKey] = component.ID
	}
	if len(enabledIDs) == 0 {
		return errors.New("at least one enabled component is required")
	}
	for _, component := range c.Components {
		for _, dep := range component.DependsOn {
			if !allIDs[dep] {
				return fmt.Errorf("component %q dependsOn unknown component %q", component.ID, dep)
			}
		}
	}
	unitIDs := map[string]bool{}
	portableUnitIDs := map[string]string{}
	unitsByID := map[string][]string{}
	outputs := map[string]string{}
	for _, unit := range c.DocumentationUnits {
		if err := pathutil.ValidatePortableSegment(unit.ID); err != nil {
			return fmt.Errorf("documentation unit id %q is not portable: %w", unit.ID, err)
		}
		key := unit.Component + "/" + unit.ID
		if unitIDs[key] {
			return fmt.Errorf("duplicate documentation unit %q in component %q", unit.ID, unit.Component)
		}
		portableKey := strings.ToLower(key)
		if other, exists := portableUnitIDs[portableKey]; exists {
			return fmt.Errorf("documentation units %q and %q differ only by case and are not portable", other, key)
		}
		portableUnitIDs[portableKey] = key
		unitIDs[key] = true
		unitsByID[unit.ID] = append(unitsByID[unit.ID], key)
		if !enabledIDs[unit.Component] {
			return fmt.Errorf("documentation unit %q references unknown or disabled component %q", unit.ID, unit.Component)
		}
		if !knownUnitKinds[unit.Kind] {
			return fmt.Errorf("documentation unit %q has unsupported kind %q", unit.ID, unit.Kind)
		}
		if !knownCriticalities[unit.Criticality] {
			return fmt.Errorf("documentation unit %q has unsupported criticality %q", unit.ID, unit.Criticality)
		}
		if unit.Output != "" {
			outputKey := strings.ToLower(unit.Component + ":" + unit.Output)
			if other, ok := outputs[outputKey]; ok {
				return fmt.Errorf("documentation units %q and %q share output %q", other, unit.ID, unit.Output)
			}
			outputs[outputKey] = unit.ID
		}
	}
	for _, unit := range c.DocumentationUnits {
		for _, related := range unit.RelatedUnits {
			if strings.Contains(related, "/") {
				if !unitIDs[related] {
					return fmt.Errorf("documentation unit %q relates to unknown qualified unit %q", unit.ID, related)
				}
				continue
			}
			if unitIDs[unit.Component+"/"+related] {
				continue
			}
			matches := unitsByID[related]
			if len(matches) == 0 {
				return fmt.Errorf("documentation unit %q relates to unknown unit %q", unit.ID, related)
			}
			if len(matches) > 1 {
				return fmt.Errorf("documentation unit %q has ambiguous relation %q; use component/unit", unit.ID, related)
			}
		}
	}
	for _, view := range c.Documentation.Views {
		if !knownViews[view] {
			return fmt.Errorf("unsupported documentation view %q", view)
		}
	}
	for _, dimension := range c.Documentation.Catalogs.ShardBy {
		if !knownShardDimensions[dimension] {
			return fmt.Errorf("unsupported catalog shard dimension %q", dimension)
		}
	}
	if c.Documentation.Catalogs.MaximumRowsPerPage < 1 {
		return errors.New("documentation.catalogs.maximumRowsPerPage must be positive")
	}
	if c.Documentation.Evidence.MaxFileSizeBytes < 1 {
		return errors.New("documentation.evidence.maxFileSizeBytes must be positive")
	}
	if c.System.Enabled && c.System.Output == "" {
		return errors.New("system.output is required when system.enabled is true")
	}
	if c.Mermaid.Mode != "basic" && c.Mermaid.Mode != "render" && c.Mermaid.Mode != "off" {
		return errors.New("mermaid.mode must be basic, render, or off")
	}
	return nil
}

func validateScope(scope string) error { _, err := pathutil.NormalizeRelative(scope); return err }

func (c ComponentConfig) WorkDir() string {
	if c.Scope == "" {
		return filepath.Clean(c.Repository)
	}
	return filepath.Clean(filepath.Join(c.Repository, c.Scope))
}
func (c ComponentConfig) DocumentationRoot() string { return filepath.Join(c.WorkDir(), "openwiki") }
func (c ComponentConfig) IsIncludedInSystem() bool {
	return c.IncludeInSystem == nil || *c.IncludeInSystem
}

func (c Config) EnabledComponents() []ComponentConfig {
	out := make([]ComponentConfig, 0, len(c.Components))
	for _, component := range c.Components {
		if component.Enabled {
			out = append(out, component)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c Config) UnitsForComponent(componentID string) []DocumentationUnitConfig {
	var out []DocumentationUnitConfig
	for _, unit := range c.DocumentationUnits {
		if unit.Component == componentID {
			out = append(out, unit)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c Config) ViewEnabled(view string) bool {
	view = normalizeID(view)
	for _, candidate := range c.Documentation.Views {
		if candidate == view {
			return true
		}
	}
	return false
}

func (c Config) NormalizedJSON() ([]byte, error) { return c.NormalizedJSONRelativeTo("") }

func (c Config) NormalizedJSONRelativeTo(base string) ([]byte, error) {
	clone := c
	clone.SourceVersion = 0
	clone.Services = nil
	clone.Execution.ParallelServices = 0
	clone.Execution.ContinueOnServiceFailure = false
	if base != "" {
		absBase, err := filepath.Abs(base)
		if err != nil {
			return nil, err
		}
		rel := func(value string) string {
			if value == "" {
				return ""
			}
			candidate, err := filepath.Rel(absBase, value)
			if err != nil {
				return filepath.ToSlash(value)
			}
			if candidate == "" {
				candidate = "."
			}
			return filepath.ToSlash(candidate)
		}
		clone.Workspace = rel(clone.Workspace)
		for i := range clone.Components {
			clone.Components[i].Repository = rel(clone.Components[i].Repository)
			clone.Components[i].Scope = filepath.ToSlash(clone.Components[i].Scope)
		}
		for i := range clone.DocumentationUnits {
			clone.DocumentationUnits[i].Output = filepath.ToSlash(clone.DocumentationUnits[i].Output)
			for j := range clone.DocumentationUnits[i].SourceRoots {
				clone.DocumentationUnits[i].SourceRoots[j] = filepath.ToSlash(clone.DocumentationUnits[i].SourceRoots[j])
			}
		}
		clone.System.Output = rel(clone.System.Output)
		clone.System.FactsPath = rel(clone.System.FactsPath)
	}
	return json.MarshalIndent(clone, "", "  ")
}

var typeToProfile = map[string]string{
	"generic": "generic", "repository": "generic", "application": "application", "monolith": "application",
	"microservice": "application", "service": "application", "worker": "application", "gateway": "application",
	"frontend": "application", "cli": "application", "modular-monolith": "modular-application",
	"library": "reusable", "shared-library": "reusable", "internal-library": "reusable", "framework": "reusable", "sdk": "reusable",
	"iac": "infrastructure", "infrastructure": "infrastructure", "gitops": "infrastructure", "platform": "infrastructure", "deployment": "infrastructure",
	"configuration": "configuration", "shared-config": "configuration", "config": "configuration",
	"contract": "contracts", "contracts": "contracts", "schema": "contracts", "schemas": "contracts",
}

func normalizeType(value string) string { return normalizeID(value) }
func normalizeID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}
func ProfileForType(componentType string) string {
	if p, ok := typeToProfile[normalizeType(componentType)]; ok {
		return p
	}
	return "generic"
}
func KnownType(componentType string) bool {
	_, ok := typeToProfile[normalizeType(componentType)]
	return ok
}
func KnownProfile(profile string) bool {
	switch normalizeID(profile) {
	case "application", "modular-application", "reusable", "infrastructure", "configuration", "contracts", "generic":
		return true
	}
	return false
}
func SupportedTypes() []string {
	out := make([]string, 0, len(typeToProfile))
	for t := range typeToProfile {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
func SupportedCapabilityPacks() []string { return append([]string(nil), capabilityPacks...) }
func KnownCapabilityPack(pack string) bool {
	pack = normalizeID(pack)
	i := sort.SearchStrings(capabilityPacks, pack)
	return i < len(capabilityPacks) && capabilityPacks[i] == pack
}
func DefaultPacksForProfile(profile string) []string {
	return append([]string(nil), defaultPacksByProfile[normalizeID(profile)]...)
}
func SupportedViews() []string           { return append([]string(nil), defaultViews...) }
func SupportedShardDimensions() []string { return append([]string(nil), shardDimensions...) }
func SupportedCriticalities() []string   { return append([]string(nil), criticalities...) }

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
