package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/example/wikiforge/internal/assets"
	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/graph"
	"github.com/example/wikiforge/internal/openwiki"
	"github.com/example/wikiforge/internal/orchestrator"
	"github.com/example/wikiforge/internal/pathutil"
	"github.com/example/wikiforge/internal/prompts"
	"github.com/example/wikiforge/internal/validation"
)

const Version = "1.3.0"

type CLI struct {
	Out io.Writer
	Err io.Writer
}

func (c CLI) Run(ctx context.Context, args []string) int {
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	if len(args) == 0 {
		c.usage()
		return 2
	}
	switch args[0] {
	case "help", "--help", "-h":
		c.usage()
		return 0
	case "version", "--version":
		fmt.Fprintln(c.Out, Version)
		return 0
	case "profiles", "types":
		return c.profilesCommand()
	case "config":
		return c.configCommand(args[1:])
	case "discover":
		return c.discoverCommand(args[1:])
	case "init":
		return c.initCommand(args[1:])
	case "doctor":
		return c.doctorCommand(ctx, args[1:])
	case "plan":
		return c.planCommand(args[1:])
	case "generate":
		return c.generateCommand(ctx, args[1:], false)
	case "update":
		return c.generateCommand(ctx, args[1:], true)
	case "resume":
		return c.resumeCommand(ctx, args[1:])
	case "validate":
		return c.validateCommand(ctx, args[1:])
	case "graph":
		return c.graphCommand(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown command %q\n", args[0])
		c.usage()
		return 2
	}
}

func (c CLI) usage() {
	fmt.Fprint(c.Out, `WikiForge - component-centric, phased, validated OpenWiki orchestration

Usage:
  wikiforge init [--config wikiforge.yaml] [--force]
  wikiforge doctor [--config wikiforge.yaml]
  wikiforge profiles
  wikiforge config migrate [--config wikiforge.yaml] [--output wikiforge.v3.json] [--force]
  wikiforge discover [--config wikiforge.yaml] [--component ID]
  wikiforge plan [--config wikiforge.yaml] [--component ID] [--skip-system] [--explain]
  wikiforge generate [--config wikiforge.yaml] [--component ID] [--skip-system] [--resume]
  wikiforge update [--config wikiforge.yaml] [--component ID] [--skip-system]
  wikiforge resume [--config wikiforge.yaml]
  wikiforge validate [--config wikiforge.yaml] [--component ID] [--system]
  wikiforge graph [--config wikiforge.yaml] [--component ID] [--system]
  wikiforge version

The legacy --service flag is accepted as an alias for --component.
`)
}

func commonFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfg := fs.String("config", "wikiforge.yaml", "configuration file")
	return fs, cfg
}

func componentFlag(fs *flag.FlagSet) (*string, *string) {
	component := fs.String("component", "", "component ID")
	legacy := fs.String("service", "", "legacy alias for --component")
	return component, legacy
}

func resolveComponentFlag(component, legacy string) (string, error) {
	if component != "" && legacy != "" && component != legacy {
		return "", errors.New("--component and --service select different IDs")
	}
	if component != "" {
		return component, nil
	}
	return legacy, nil
}

func (c CLI) profilesCommand() int {
	fmt.Fprintln(c.Out, "Supported component types and documentation profiles:")
	for _, componentType := range config.SupportedTypes() {
		fmt.Fprintf(c.Out, "  %-20s -> %s\n", componentType, config.ProfileForType(componentType))
	}
	fmt.Fprintln(c.Out, "\nProfiles:")
	for _, profileID := range prompts.ProfileIDs() {
		profile, _ := prompts.GetProfile(profileID)
		fmt.Fprintf(c.Out, "  %-20s pages=%-2d phases=%-2d %s\n", profile.ID, len(prompts.ExpectedFiles(profile)), len(profile.Phases), profile.Description)
	}
	fmt.Fprintln(c.Out, "\nComposable capability packs:")
	for _, pack := range config.SupportedCapabilityPacks() {
		fmt.Fprintln(c.Out, "  "+pack)
	}
	fmt.Fprintln(c.Out, "\nAdaptive documentation views:")
	for _, view := range config.SupportedViews() {
		fmt.Fprintln(c.Out, "  "+view)
	}
	return 0
}

func (c CLI) configCommand(args []string) int {
	if len(args) == 0 || args[0] != "migrate" {
		fmt.Fprintln(c.Err, "usage: wikiforge config migrate [--config PATH] [--output PATH] [--force]")
		return 2
	}
	fs, cfgPath := commonFlags("config migrate")
	output := fs.String("output", "wikiforge.v3.json", "normalized version 3 output")
	force := fs.Bool("force", false, "overwrite output")
	if err := fs.Parse(args[1:]); err != nil {
		return c.flagError(err)
	}
	if _, err := os.Stat(*output); err == nil && !*force {
		return c.printErr(fmt.Errorf("%s already exists; use --force to replace it", *output))
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	data, err := cfg.NormalizedJSONRelativeTo(filepath.Dir(*output))
	if err != nil {
		return c.printErr(err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return c.printErr(err)
	}
	tmp := *output + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return c.printErr(err)
	}
	if err := replaceFile(tmp, *output, *force); err != nil {
		return c.printErr(err)
	}
	fmt.Fprintf(c.Out, "migrated source version %d to version %d: %s\n", cfg.SourceVersion, config.CurrentVersion, *output)
	return 0
}

func replaceFile(tmp, destination string, replace bool) error {
	if !replace {
		if err := os.Rename(tmp, destination); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	}
	if err := os.Rename(tmp, destination); err == nil {
		return nil
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (c CLI) discoverCommand(args []string) int {
	fs, cfgPath := commonFlags("discover")
	component, legacy := componentFlag(fs)
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected, err := resolveComponentFlag(*component, *legacy)
	if err != nil {
		return c.printErr(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	o := orchestrator.New(cfg, openwiki.ExecRunner{Config: cfg.OpenWiki, Out: c.Out}, c.Out)
	manifests, err := o.Discover(selected, true)
	if err != nil {
		return c.printErr(err)
	}
	for _, manifest := range manifests {
		data, _ := json.MarshalIndent(manifest, "", "  ")
		fmt.Fprintf(c.Out, "[%s]\n%s\n", manifest.Component.ID, data)
	}
	return 0
}

func (c CLI) initCommand(args []string) int {
	fs, cfgPath := commonFlags("init")
	force := fs.Bool("force", false, "overwrite existing config")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	if _, err := os.Stat(*cfgPath); err == nil && !*force {
		fmt.Fprintf(c.Err, "%s already exists; use --force to replace it\n", *cfgPath)
		return 1
	}
	b, err := assets.FS.ReadFile("templates/wikiforge.yaml")
	if err != nil {
		return c.printErr(err)
	}
	if err := os.WriteFile(*cfgPath, b, 0o644); err != nil {
		return c.printErr(err)
	}
	_ = os.MkdirAll("repositories", 0o755)
	_ = os.MkdirAll("facts", 0o755)
	fmt.Fprintf(c.Out, "created %s\nEdit component repositories, types, and scopes, then run: wikiforge doctor\n", *cfgPath)
	return 0
}

func (c CLI) doctorCommand(ctx context.Context, args []string) int {
	fs, cfgPath := commonFlags("doctor")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	failed := false
	check := func(name string, err error) {
		if err != nil {
			failed = true
			fmt.Fprintf(c.Out, "[FAIL] %s: %v\n", name, err)
		} else {
			fmt.Fprintf(c.Out, "[ OK ] %s\n", name)
		}
	}
	_, err = exec.LookPath("git")
	check("git", err)
	if cfg.OpenWiki.Command == "npx" || cfg.OpenWiki.Command == "npm" || cfg.OpenWiki.Command == "node" || cfg.Mermaid.Command == "npx" {
		check("Node.js >= 22", checkNodeVersion(ctx, 22))
	}
	runner := openwiki.ExecRunner{Config: cfg.OpenWiki}
	check("OpenWiki command", runner.Check(ctx))
	if cfg.Mermaid.Mode == "render" {
		_, err = exec.LookPath(cfg.Mermaid.Command)
		check("Mermaid command", err)
	}
	check("workspace path portability", checkExternalPath(cfg.Workspace))
	if cfg.System.Enabled {
		check("system output path portability", checkExternalPath(cfg.System.Output))
		if cfg.System.FactsPath != "" {
			check("system facts path portability", checkExternalPath(cfg.System.FactsPath))
		}
	}
	for _, component := range cfg.EnabledComponents() {
		check(fmt.Sprintf("component %s (%s/%s)", component.ID, component.Type, component.Profile), checkComponentEnvironment(component, cfg.UnitsForComponent(component.ID)))
	}
	if cfg.System.Enabled && cfg.System.FactsPath != "" {
		if _, err := os.Stat(cfg.System.FactsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			check("system facts", err)
		}
	}
	if failed {
		return 1
	}
	return 0
}

func checkComponentEnvironment(component config.ComponentConfig, units []config.DocumentationUnitConfig) error {
	if _, err := os.Stat(component.Repository); err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	if !isGitRepo(component.Repository) {
		return errors.New("repository is not a Git work tree")
	}
	if _, err := os.Stat(component.WorkDir()); err != nil {
		return fmt.Errorf("scope %q: %w", component.Scope, err)
	}
	if err := checkExternalPath(component.Repository); err != nil {
		return fmt.Errorf("repository path portability: %w", err)
	}
	if err := checkExternalPath(component.WorkDir()); err != nil {
		return fmt.Errorf("scope path portability: %w", err)
	}
	for _, unit := range units {
		for _, root := range unit.SourceRoots {
			if _, err := os.Stat(filepath.Join(component.WorkDir(), filepath.FromSlash(root))); err != nil {
				return fmt.Errorf("documentation unit %s source root %q: %w", unit.ID, root, err)
			}
		}
	}
	if err := openwiki.CheckPromptTransport(component.WorkDir()); err != nil {
		return fmt.Errorf("prompt-file transport: %w", err)
	}
	return nil
}

func (c CLI) planCommand(args []string) int {
	fs, cfgPath := commonFlags("plan")
	component, legacy := componentFlag(fs)
	skip := fs.Bool("skip-system", false, "skip whole-system plan")
	explain := fs.Bool("explain", false, "include skipped and deferred planning decisions")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected, err := resolveComponentFlag(*component, *legacy)
	if err != nil {
		return c.printErr(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	o := orchestrator.New(cfg, openwiki.ExecRunner{Config: cfg.OpenWiki, Out: c.Out, LiveOutput: true}, c.Out)
	lines, err := o.PlanWithExplain(selected, !*skip, *explain)
	if err != nil {
		return c.printErr(err)
	}
	for _, line := range lines {
		fmt.Fprintln(c.Out, line)
	}
	return 0
}

func (c CLI) generateCommand(ctx context.Context, args []string, update bool) int {
	name := "generate"
	if update {
		name = "update"
	}
	fs, cfgPath := commonFlags(name)
	component, legacy := componentFlag(fs)
	skip := fs.Bool("skip-system", false, "skip whole-system wiki")
	resume := fs.Bool("resume", false, "resume completed phases from current state")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected, err := resolveComponentFlag(*component, *legacy)
	if err != nil {
		return c.printErr(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	o := orchestrator.New(cfg, openwiki.ExecRunner{Config: cfg.OpenWiki, Out: c.Out, LiveOutput: true}, c.Out)
	res, err := o.Generate(ctx, orchestrator.GenerateOptions{ComponentID: selected, SkipSystem: *skip, Resume: *resume, UpdateOnly: update})
	if res.ReportDir != "" {
		fmt.Fprintln(c.Out, "report:", res.ReportDir)
	}
	if err != nil {
		return c.printErr(err)
	}
	return 0
}

func (c CLI) resumeCommand(ctx context.Context, args []string) int {
	fs, cfgPath := commonFlags("resume")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	o := orchestrator.New(cfg, openwiki.ExecRunner{Config: cfg.OpenWiki, Out: c.Out, LiveOutput: true}, c.Out)
	res, err := o.Generate(ctx, orchestrator.GenerateOptions{Resume: true})
	if res.ReportDir != "" {
		fmt.Fprintln(c.Out, "report:", res.ReportDir)
	}
	if err != nil {
		return c.printErr(err)
	}
	return 0
}

func (c CLI) validateCommand(ctx context.Context, args []string) int {
	fs, cfgPath := commonFlags("validate")
	component, legacy := componentFlag(fs)
	system := fs.Bool("system", false, "validate whole-system wiki only")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected, err := resolveComponentFlag(*component, *legacy)
	if err != nil {
		return c.printErr(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	v := validation.Validator{Config: cfg}
	accepted := true
	matched := false
	if *system {
		r := v.ValidateSystem(ctx)
		printValidation(c.Out, "system", r)
		accepted = r.Accepted
		matched = true
	} else {
		for _, comp := range cfg.EnabledComponents() {
			if selected != "" && comp.ID != selected {
				continue
			}
			matched = true
			r := v.ValidateComponent(ctx, comp)
			printValidation(c.Out, comp.ID, r)
			accepted = accepted && r.Accepted
		}
	}
	if !matched {
		return c.printErr(fmt.Errorf("no component matched %q", selected))
	}
	if !accepted {
		return 1
	}
	return 0
}

func (c CLI) graphCommand(args []string) int {
	fs, cfgPath := commonFlags("graph")
	component, legacy := componentFlag(fs)
	system := fs.Bool("system", false, "export whole-system graph only")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected, err := resolveComponentFlag(*component, *legacy)
	if err != nil {
		return c.printErr(err)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	if *system {
		err = graph.Export(cfg.System.ID, filepath.Join(cfg.System.Output, "openwiki"), filepath.Join(cfg.Workspace, ".wikiforge", "graph", "system"))
		if err != nil {
			return c.printErr(err)
		}
		fmt.Fprintln(c.Out, "exported system graph")
		return 0
	}
	matched := false
	for _, comp := range cfg.EnabledComponents() {
		if selected != "" && comp.ID != selected {
			continue
		}
		matched = true
		if err := graph.Export(comp.ID, comp.DocumentationRoot(), filepath.Join(cfg.Workspace, ".wikiforge", "graph", comp.ID)); err != nil {
			return c.printErr(err)
		}
		fmt.Fprintln(c.Out, "exported graph for", comp.ID)
	}
	if !matched {
		return c.printErr(fmt.Errorf("no component matched %q", selected))
	}
	return 0
}

func printValidation(out io.Writer, id string, r interface{}) {
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Fprintf(out, "[%s]\n%s\n", id, string(b))
}

func (c CLI) flagError(err error) int {
	fmt.Fprintln(c.Err, err)
	return 2
}

func (c CLI) printErr(err error) int {
	fmt.Fprintln(c.Err, "error:", err)
	return 1
}

func checkExternalPath(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("path is empty")
	}
	_, err := pathutil.ExternalToolPath(value)
	return err
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func checkNodeVersion(ctx context.Context, minimumMajor int) error {
	path, err := exec.LookPath("node")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, "--version")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(b), "v"))
	majorText := strings.SplitN(version, ".", 2)[0]
	major := 0
	if _, err := fmt.Sscanf(majorText, "%d", &major); err != nil {
		return fmt.Errorf("cannot parse Node.js version %q", version)
	}
	if major < minimumMajor {
		return fmt.Errorf("found Node.js %s; version %d or newer is required", version, minimumMajor)
	}
	return nil
}
