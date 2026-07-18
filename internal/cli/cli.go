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

	"github.com/fajarnugraha37/wikiforge/internal/assets"
	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/graph"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/openwiki"
	"github.com/fajarnugraha37/wikiforge/internal/orchestrator"
	"github.com/fajarnugraha37/wikiforge/internal/pathutil"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
	"github.com/fajarnugraha37/wikiforge/internal/prompts"
	"github.com/fajarnugraha37/wikiforge/internal/validation"
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
	case "discover":
		return c.discoverCommand(args[1:])
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
	case "coverage":
		return c.coverageCommand(args[1:])
	case "impact":
		return c.impactCommand(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown command %q\n", args[0])
		c.usage()
		return 2
	}
}

func (c CLI) usage() {
	fmt.Fprint(c.Out, `WikiForge - component-centric, adaptive, validated OpenWiki orchestration

Usage:
  wikiforge init [--config wikiforge.yaml] [--force]
  wikiforge doctor [--config wikiforge.yaml]
  wikiforge discover [--config wikiforge.yaml] [--component ID]
  wikiforge profiles
  wikiforge plan [--config wikiforge.yaml] [--component ID] [--skip-system] [--explain]
  wikiforge generate [--config wikiforge.yaml] [--component ID] [--skip-system] [--resume]
  wikiforge update [--config wikiforge.yaml] [--component ID] [--skip-system]
  wikiforge resume [--config wikiforge.yaml]
  wikiforge validate [--config wikiforge.yaml] [--component ID] [--system] [--strict]
  wikiforge coverage [--config wikiforge.yaml] [--component ID] [--system]
  wikiforge impact [--config wikiforge.yaml] [--component ID] [--system]
  wikiforge graph [--config wikiforge.yaml] [--component ID] [--system]
  wikiforge version
`)
}

func commonFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfg := fs.String("config", "wikiforge.yaml", "configuration file")
	return fs, cfg
}

func componentFlag(fs *flag.FlagSet) *string {
	return fs.String("component", "", "component ID")
}

func (c CLI) profilesCommand() int {
	fmt.Fprintln(c.Out, "Supported component types and documentation profiles:")
	for _, componentType := range config.SupportedTypes() {
		fmt.Fprintf(c.Out, "  %-20s -> %s\n", componentType, config.ProfileForType(componentType))
	}
	fmt.Fprintln(c.Out, "\nProfiles:")
	for _, profileID := range prompts.ProfileIDs() {
		profile, _ := prompts.GetProfile(profileID)
		fmt.Fprintf(c.Out, "  %-20s %s\n", profile.ID, profile.Description)
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
	component := componentFlag(fs)
	skip := fs.Bool("skip-system", false, "skip whole-system plan")
	explain := fs.Bool("explain", false, "show adaptive planning decisions and reasons")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	result, err := (planner.Planner{Config: cfg}).Plan(selected, !*skip)
	if err != nil {
		return c.printErr(err)
	}
	printAdaptivePlan(c.Out, result, *explain)
	return 0
}

func (c CLI) discoverCommand(args []string) int {
	fs, cfgPath := commonFlags("discover")
	component := componentFlag(fs)
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	manifest, err := (planner.Planner{Config: cfg}).Discover(selected)
	if err != nil {
		return c.printErr(err)
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return c.printErr(err)
	}
	fmt.Fprintln(c.Out, string(b))
	return 0
}

func (c CLI) coverageCommand(args []string) int {
	fs, cfgPath := commonFlags("coverage")
	component := componentFlag(fs)
	system := fs.Bool("system", false, "show whole-system coverage only")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	if *system {
		if err := printArtifact(c.Out, filepath.Join(cfg.Workspace, ".wikiforge", "system", cfg.System.ID, "coverage.json"), evidence.LoadCoverage); err != nil {
			return c.printErr(err)
		}
		return 0
	}
	matched := false
	for _, comp := range cfg.EnabledComponents() {
		if selected != "" && comp.ID != selected {
			continue
		}
		matched = true
		if err := printArtifact(c.Out, filepath.Join(cfg.Workspace, ".wikiforge", "components", comp.ID, "coverage.json"), evidence.LoadCoverage); err != nil {
			return c.printErr(err)
		}
	}
	if !matched {
		return c.printErr(fmt.Errorf("no component matched %q", selected))
	}
	return 0
}

func (c CLI) impactCommand(args []string) int {
	fs, cfgPath := commonFlags("impact")
	component := componentFlag(fs)
	system := fs.Bool("system", false, "show whole-system impact only")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	if *system {
		if err := printArtifact(c.Out, filepath.Join(cfg.Workspace, ".wikiforge", "system", cfg.System.ID, "impact-index.json"), evidence.LoadImpact); err != nil {
			return c.printErr(err)
		}
		return 0
	}
	matched := false
	for _, comp := range cfg.EnabledComponents() {
		if selected != "" && comp.ID != selected {
			continue
		}
		matched = true
		if err := printArtifact(c.Out, filepath.Join(cfg.Workspace, ".wikiforge", "components", comp.ID, "impact-index.json"), evidence.LoadImpact); err != nil {
			return c.printErr(err)
		}
	}
	if !matched {
		return c.printErr(fmt.Errorf("no component matched %q", selected))
	}
	return 0
}

func printArtifact[T any](out io.Writer, path string, load func(string) (T, error)) error {
	value, err := load(path)
	if err != nil {
		return fmt.Errorf("load artifact %s: %w", path, err)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}

func printAdaptivePlan(out io.Writer, result planner.PlanResult, explain bool) {
	for _, componentPlan := range result.Components {
		fmt.Fprintf(out, "component %s profile=%s views=%s packs=%s\n", componentPlan.ComponentID, componentPlan.Profile, joinViews(componentPlan.Views), strings.Join(componentPlan.Packs, ","))
		for _, unit := range componentPlan.Units {
			fmt.Fprintf(out, "  unit %-18s kind=%s output=%s\n", unit.ID, unit.Kind, unit.OutputPath)
			if explain {
				fmt.Fprintf(out, "    reason: %s\n", unit.Reason)
			}
		}
		for _, page := range componentPlan.Pages {
			fmt.Fprintf(out, "  page %-11s %s\n", page.Kind, page.Path)
			if explain {
				fmt.Fprintf(out, "    reason: %s\n", page.Reason)
			}
		}
		if explain {
			for _, decision := range componentPlan.Decisions {
				fmt.Fprintf(out, "  decision %-6s %s\n", decision.Action, decision.Target)
				fmt.Fprintf(out, "    reason: %s\n", decision.Reason)
			}
		}
	}
	if result.System != nil {
		fmt.Fprintf(out, "system views=%s\n", joinViews(result.System.Views))
		for _, page := range result.System.Pages {
			fmt.Fprintf(out, "  page %-11s %s\n", page.Kind, page.Path)
			if explain {
				fmt.Fprintf(out, "    reason: %s\n", page.Reason)
			}
		}
	}
}

func joinViews(views []planner.DocumentationView) string {
	if len(views) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(views))
	for _, view := range views {
		parts = append(parts, string(view))
	}
	return strings.Join(parts, ",")
}

func (c CLI) generateCommand(ctx context.Context, args []string, update bool) int {
	name := "generate"
	if update {
		name = "update"
	}
	fs, cfgPath := commonFlags(name)
	component := componentFlag(fs)
	skip := fs.Bool("skip-system", false, "skip whole-system wiki")
	resume := fs.Bool("resume", false, "resume completed phases from current state")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
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
	component := componentFlag(fs)
	system := fs.Bool("system", false, "validate whole-system wiki only")
	strict := fs.Bool("strict", false, "treat warnings as errors")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return c.printErr(err)
	}
	v := validation.Validator{Config: cfg}
	accepted := true
	matched := false
	if *system {
		var r interface{}
		plan, planErr := (planner.Planner{Config: cfg}).Plan("", true)
		if planErr != nil || plan.System == nil {
			if planErr == nil {
				planErr = errors.New("system plan is disabled")
			}
			return c.printErr(planErr)
		}
		r = v.ValidateAdaptiveSystem(ctx, filepath.Join(cfg.System.Output, "openwiki"), *plan.System)
		r = addEvidenceValidation(r, filepath.Join(cfg.System.Output, "openwiki"), filepath.Join(cfg.Workspace, ".wikiforge", "system", cfg.System.ID, "evidence-index.json"), cfg.Documentation.MinimumQualityScore)
		r = strictValidation(r, cfg.Documentation.MinimumQualityScore, *strict)
		printValidation(c.Out, "system", r)
		accepted = validationAccepted(r)
		matched = true
	} else {
		for _, comp := range cfg.EnabledComponents() {
			if selected != "" && comp.ID != selected {
				continue
			}
			matched = true
			var r interface{}
			plan, planErr := (planner.Planner{Config: cfg}).Plan(comp.ID, false)
			if planErr != nil || len(plan.Components) != 1 {
				if planErr == nil {
					planErr = fmt.Errorf("plan for component %s is missing or ambiguous", comp.ID)
				}
				return c.printErr(planErr)
			}
			r = v.ValidateAdaptiveComponent(ctx, comp, plan.Components[0])
			r = addEvidenceValidation(r, comp.DocumentationRoot(), filepath.Join(cfg.Workspace, ".wikiforge", "components", comp.ID, "evidence-index.json"), cfg.Documentation.MinimumQualityScore)
			r = strictValidation(r, cfg.Documentation.MinimumQualityScore, *strict)
			printValidation(c.Out, comp.ID, r)
			accepted = accepted && validationAccepted(r)
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

func addEvidenceValidation(value interface{}, root, indexPath string, minimumScore int) interface{} {
	result, ok := value.(model.ValidationResult)
	if !ok {
		return value
	}
	index, err := evidence.LoadIndex(indexPath)
	if err != nil {
		return value
	}
	result.Findings = append(result.Findings, validation.ValidateEvidenceBacked(root, index)...)
	return validation.Recalculate(result, minimumScore)
}

func strictValidation(value interface{}, minimumScore int, strict bool) interface{} {
	if !strict {
		return value
	}
	result, ok := value.(model.ValidationResult)
	if !ok {
		return value
	}
	return validation.Strict(result, minimumScore)
}

func validationAccepted(value interface{}) bool {
	switch result := value.(type) {
	case model.ValidationResult:
		return result.Accepted
	default:
		return false
	}
}

func (c CLI) graphCommand(args []string) int {
	fs, cfgPath := commonFlags("graph")
	component := componentFlag(fs)
	system := fs.Bool("system", false, "export whole-system graph only")
	if err := fs.Parse(args); err != nil {
		return c.flagError(err)
	}
	selected := *component
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
