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

const CurrentVersion = 2

type Config struct {
	Version       int                 `json:"version"`
	Workspace     string              `json:"workspace"`
	OpenWiki      OpenWikiConfig      `json:"openwiki"`
	Execution     ExecutionConfig     `json:"execution"`
	Documentation DocumentationConfig `json:"documentation"`
	Mermaid       MermaidConfig       `json:"mermaid"`
	Components    []ComponentConfig   `json:"components"`
	// Services is retained only for backward compatibility with v1 configurations.
	// Each legacy service is normalized into a component of type microservice.
	Services []ServiceConfig `json:"services"`
	System   SystemConfig    `json:"system"`
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
	ParallelServices           int  `json:"parallelServices"` // legacy alias
	MaxProcessRetries          int  `json:"maxProcessRetries"`
	MaxRepairRounds            int  `json:"maxRepairRounds"`
	ContinueOnComponentFailure bool `json:"continueOnComponentFailure"`
	ContinueOnServiceFailure   bool `json:"continueOnServiceFailure"` // legacy alias
}

type DocumentationConfig struct {
	Language                string   `json:"language"`
	MinimumQualityScore     int      `json:"minimumQualityScore"`
	MinimumPages            int      `json:"minimumPages"`
	RequireFrontMatter      bool     `json:"requireFrontMatter"`
	RequireSourceReferences bool     `json:"requireSourceReferences"`
	ValidateSourcePaths     bool     `json:"validateSourcePaths"`
	RequireMermaid          bool     `json:"requireMermaid"`
	MinimumMermaidBlocks    int      `json:"minimumMermaidBlocks"`
	AllowedDiagramTypes     []string `json:"allowedDiagramTypes"`
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
	Path            string   `json:"path"` // accepted alias for repository or scoped path
	Enabled         bool     `json:"enabled"`
	IncludeInSystem *bool    `json:"includeInSystem"`
	Group           string   `json:"group"`
	Tags            []string `json:"tags"`
	DependsOn       []string `json:"dependsOn"`
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
		Version:   CurrentVersion,
		Workspace: ".",
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
			MinimumPages:            0, // zero means use the selected profile contract
			RequireFrontMatter:      true,
			RequireSourceReferences: true,
			ValidateSourcePaths:     true,
			RequireMermaid:          true,
			MinimumMermaidBlocks:    0, // zero means use the selected profile contract
			AllowedDiagramTypes:     []string{"flowchart", "sequenceDiagram", "stateDiagram-v2", "erDiagram", "classDiagram", "architecture-beta", "gitGraph", "mindmap"},
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
	normalizeLegacy(&cfg)
	for i := range cfg.Components {
		if err := normalizeComponent(base, &cfg.Components[i]); err != nil {
			return cfg, fmt.Errorf("component %q paths: %w", cfg.Components[i].ID, err)
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

func normalizeLegacy(c *Config) {
	if c.Version == 0 {
		c.Version = 1
	}
	for _, s := range c.Services {
		c.Components = append(c.Components, ComponentConfig{
			ID:         s.ID,
			Type:       "microservice",
			Repository: s.Path,
			Enabled:    s.Enabled,
		})
	}
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
	c.Path = "" // prevent ambiguous use after normalization
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
	sort.Strings(c.Tags)
	sort.Strings(c.DependsOn)
	return nil
}

func applyDefaults(c *Config) {
	d := Defaults()
	if c.Version == 0 {
		c.Version = CurrentVersion
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
	if c.Version != 1 && c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d; supported versions are 1 and %d", c.Version, CurrentVersion)
	}
	if c.OpenWiki.Command == "" {
		return errors.New("openwiki.command is required")
	}
	allIDs := map[string]bool{}
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
		allIDs[component.ID] = true
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
		if other, ok := workdirs[workdir]; ok {
			return fmt.Errorf("components %q and %q resolve to the same work directory %q", other, component.ID, workdir)
		}
		workdirs[workdir] = component.ID
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
	if c.System.Enabled && c.System.Output == "" {
		return errors.New("system.output is required when system.enabled is true")
	}
	if c.Mermaid.Mode != "basic" && c.Mermaid.Mode != "render" && c.Mermaid.Mode != "off" {
		return errors.New("mermaid.mode must be basic, render, or off")
	}
	return nil
}

func validateScope(scope string) error {
	_, err := pathutil.NormalizeRelative(scope)
	return err
}

func (c ComponentConfig) WorkDir() string {
	if c.Scope == "" {
		return filepath.Clean(c.Repository)
	}
	return filepath.Clean(filepath.Join(c.Repository, c.Scope))
}

func (c ComponentConfig) DocumentationRoot() string {
	return filepath.Join(c.WorkDir(), "openwiki")
}

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

var typeToProfile = map[string]string{
	"generic":          "generic",
	"repository":       "generic",
	"application":      "application",
	"monolith":         "application",
	"microservice":     "application",
	"service":          "application",
	"worker":           "application",
	"gateway":          "application",
	"frontend":         "application",
	"cli":              "application",
	"modular-monolith": "modular-application",
	"library":          "reusable",
	"shared-library":   "reusable",
	"internal-library": "reusable",
	"framework":        "reusable",
	"sdk":              "reusable",
	"iac":              "infrastructure",
	"infrastructure":   "infrastructure",
	"gitops":           "infrastructure",
	"platform":         "infrastructure",
	"deployment":       "infrastructure",
	"configuration":    "configuration",
	"shared-config":    "configuration",
	"config":           "configuration",
	"contract":         "contracts",
	"contracts":        "contracts",
	"schema":           "contracts",
	"schemas":          "contracts",
}

func normalizeType(value string) string {
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
	switch strings.TrimSpace(strings.ToLower(profile)) {
	case "application", "modular-application", "reusable", "infrastructure", "configuration", "contracts", "generic":
		return true
	default:
		return false
	}
}

func SupportedTypes() []string {
	out := make([]string, 0, len(typeToProfile))
	for t := range typeToProfile {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
