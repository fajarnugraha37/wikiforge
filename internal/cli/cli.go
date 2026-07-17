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

const Version = "1.2.3"

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
  wikiforge plan [--config wikiforge.yaml] [--component ID] [--skip-system]
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
		statErr := func() error {
			if _, e := os.Stat(component.Repository); e != nil {
				return fmt.Errorf("repository: %w", e)
			}
			if !isGitRepo(component.Repository) {
				return errors.New("repository is not a Git work tree")
			}
			if _, e := os.Stat(component.WorkDir()); e != nil {
				return fmt.Errorf("scope %q: %w", component.Scope, e)
			}
			if e := checkExternalPath(component.Repository); e != nil {
				return fmt.Errorf("repository path portability: %w", e)
			}
			if e := checkExternalPath(component.WorkDir()); e != nil {
				return fmt.Errorf("scope path portability: %w", e)
			}
			if e := openwiki.CheckPromptTransport(component.WorkDir()); e != nil {
				return fmt.Errorf("prompt-file transport: %w", e)
			}
			return nil
		}()
		check(fmt.Sprintf("component %s (%s/%s)", component.ID, component.Type, component.Profile), statErr)
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

func (c CLI) planCommand(args []string) int {
	fs, cfgPath := commonFlags("plan")
	component, legacy := componentFlag(fs)
	skip := fs.Bool("skip-system", false, "skip whole-system plan")
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
	for _, line := range o.Plan(selected, !*skip) {
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
