package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/discovery"
	"github.com/example/wikiforge/internal/graph"
	"github.com/example/wikiforge/internal/model"
	"github.com/example/wikiforge/internal/openwiki"
	"github.com/example/wikiforge/internal/planner"
	"github.com/example/wikiforge/internal/prompts"
	"github.com/example/wikiforge/internal/report"
	"github.com/example/wikiforge/internal/state"
	"github.com/example/wikiforge/internal/validation"
)

type Orchestrator struct {
	Config    config.Config
	Runner    openwiki.Runner
	Validator validation.Validator
	Store     *state.Store
	Out       io.Writer
	stateMu   sync.Mutex
}

type GenerateOptions struct {
	ComponentID string
	SkipSystem  bool
	Resume      bool
	UpdateOnly  bool
}

type GenerateResult struct {
	ReportDir string
	Report    report.RunReport
}

func New(cfg config.Config, runner openwiki.Runner, out io.Writer) *Orchestrator {
	if out == nil {
		out = io.Discard
	}
	return &Orchestrator{
		Config:    cfg,
		Runner:    runner,
		Validator: validation.Validator{Config: cfg},
		Store:     &state.Store{Path: filepath.Join(cfg.Workspace, ".wikiforge", "state.json")},
		Out:       out,
	}
}

func (o *Orchestrator) Plan(componentID string, includeSystem bool) []string {
	lines, err := o.PlanWithExplain(componentID, includeSystem, true)
	if err != nil {
		return []string{"plan failed: " + err.Error()}
	}
	return lines
}

func (o *Orchestrator) PlanWithExplain(componentID string, includeSystem, explain bool) ([]string, error) {
	components := o.selectedComponents(componentID)
	if len(components) == 0 {
		return nil, fmt.Errorf("no enabled component matched %q", componentID)
	}
	var lines []string
	var systemComponentPlans []model.DocumentationPlan
	for _, component := range components {
		manifest, plan, err := o.prepareAdaptivePlan(component, true)
		if err != nil {
			return nil, fmt.Errorf("component %s discovery failed: %w", component.ID, err)
		}
		if component.IsIncludedInSystem() {
			systemComponentPlans = append(systemComponentPlans, plan)
		}
		lines = append(lines, fmt.Sprintf("component %s type=%s profile=%s repository=%s scope=%s scanned=%d", component.ID, component.Type, component.Profile, component.Repository, printableScope(component.Scope), manifest.FilesScanned))
		if explain {
			for _, line := range planner.Explain(plan) {
				lines = append(lines, "  "+line)
			}
		} else {
			lines = append(lines, fmt.Sprintf("  adaptive plan packs=%d units=%d pages=%d; use --explain for decisions", len(plan.SelectedPacks), len(plan.Units), len(plan.Pages)))
		}
		profile, _ := prompts.GetProfile(component.Profile)
		lines = append(lines, "  execution compatibility phases:")
		for _, p := range profile.Phases {
			lines = append(lines, "    "+p.ID+"  "+p.Name)
		}
		lines = append(lines, "    validate -> targeted repair -> graph export")
	}
	if includeSystem && o.Config.System.Enabled {
		if len(systemComponentPlans) == 0 {
			_ = os.Remove(filepath.Join(o.Config.Workspace, ".wikiforge", "system", "plan.json"))
			lines = append(lines, fmt.Sprintf("system %s skipped: no selected component is included in system aggregation", o.Config.System.ID))
		} else {
			systemPlan := planner.BuildSystem(o.Config, systemComponentPlans)
			if err := writeJSON(filepath.Join(o.Config.Workspace, ".wikiforge", "system", "plan.json"), systemPlan); err != nil {
				return nil, fmt.Errorf("persist system %s plan: %w", o.Config.System.ID, err)
			}
			lines = append(lines, fmt.Sprintf("system %s (%s)", o.Config.System.ID, o.Config.System.Output))
			if explain {
				for _, line := range planner.Explain(systemPlan) {
					lines = append(lines, "  "+line)
				}
			} else {
				lines = append(lines, fmt.Sprintf("  adaptive system plan packs=%d units=%d pages=%d; use --explain for decisions", len(systemPlan.SelectedPacks), len(systemPlan.Units), len(systemPlan.Pages)))
			}
			lines = append(lines, "  execution compatibility phases:")
			for _, p := range prompts.SystemPhases {
				lines = append(lines, "    "+p.ID+"  "+p.Name)
			}
			lines = append(lines, "    validate -> targeted repair -> graph export")
		}
	}
	return lines, nil
}

func (o *Orchestrator) Discover(componentID string, persist bool) ([]model.DiscoveryManifest, error) {
	components := o.selectedComponents(componentID)
	if len(components) == 0 {
		return nil, fmt.Errorf("no enabled component matched %q", componentID)
	}
	out := make([]model.DiscoveryManifest, 0, len(components))
	for _, component := range components {
		manifest, _, err := o.prepareAdaptivePlan(component, persist)
		if err != nil {
			return nil, fmt.Errorf("component %s: %w", component.ID, err)
		}
		out = append(out, manifest)
	}
	return out, nil
}

func (o *Orchestrator) AdaptivePlans(componentID string, persist bool) ([]model.DocumentationPlan, error) {
	components := o.selectedComponents(componentID)
	if len(components) == 0 {
		return nil, fmt.Errorf("no enabled component matched %q", componentID)
	}
	out := make([]model.DocumentationPlan, 0, len(components))
	for _, component := range components {
		_, plan, err := o.prepareAdaptivePlan(component, persist)
		if err != nil {
			return nil, fmt.Errorf("component %s: %w", component.ID, err)
		}
		out = append(out, plan)
	}
	return out, nil
}

func (o *Orchestrator) prepareAdaptivePlan(component config.ComponentConfig, persist bool) (model.DiscoveryManifest, model.DocumentationPlan, error) {
	manifest, err := discovery.Discover(o.Config, component)
	if err != nil {
		return model.DiscoveryManifest{}, model.DocumentationPlan{}, err
	}
	plan := planner.Build(o.Config, component, manifest)
	if persist {
		root := filepath.Join(o.Config.Workspace, ".wikiforge", "components", component.ID)
		if err := writeJSON(filepath.Join(root, "discovery.json"), manifest); err != nil {
			return manifest, plan, err
		}
		if err := writeJSON(filepath.Join(root, "plan.json"), plan); err != nil {
			return manifest, plan, err
		}
	}
	return manifest, plan, nil
}

func (o *Orchestrator) prepareSystemAdaptivePlan(components []config.ComponentConfig, persist bool) (model.DocumentationPlan, error) {
	componentPlans := make([]model.DocumentationPlan, 0, len(components))
	for _, component := range components {
		_, plan, err := o.prepareAdaptivePlan(component, persist)
		if err != nil {
			return model.DocumentationPlan{}, fmt.Errorf("component %s: %w", component.ID, err)
		}
		componentPlans = append(componentPlans, plan)
	}
	systemPlan := planner.BuildSystem(o.Config, componentPlans)
	if persist {
		if err := writeJSON(filepath.Join(o.Config.Workspace, ".wikiforge", "system", "plan.json"), systemPlan); err != nil {
			return systemPlan, err
		}
	}
	return systemPlan, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err == nil {
		return nil
	}
	// Windows does not replace an existing destination. Preserve atomic rename
	// on hosts that support it, then use a remove-and-retry fallback.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func hashValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (o *Orchestrator) Generate(ctx context.Context, options GenerateOptions) (GenerateResult, error) {
	if err := os.MkdirAll(filepath.Join(o.Config.Workspace, ".wikiforge"), 0o755); err != nil {
		return GenerateResult{}, err
	}
	st, err := o.Store.Load()
	if err != nil {
		return GenerateResult{}, err
	}
	migrateState(&st)
	if st.RunID == "" {
		st = model.RunState{
			Version:    3,
			Components: map[string]model.TargetState{},
			System:     model.TargetState{Phases: map[string]model.PhaseStatus{}},
		}
	}
	if !options.Resume && !options.UpdateOnly {
		// A full generation starts a new reportable run, but the target hashes
		// remain the last-successful checkpoint until each target completes.
		st.RunID = time.Now().UTC().Format("20060102T150405.000000000Z")
		st.Mode = "generate"
		st.StartedAt = time.Now().UTC()
	} else if options.UpdateOnly {
		// Start a new reportable run while preserving the last successful hashes
		// and phase state required for scoped no-op detection.
		st.RunID = time.Now().UTC().Format("20060102T150405.000000000Z")
		st.Mode = "update"
		st.StartedAt = time.Now().UTC()
	}
	if st.Components == nil {
		st.Components = map[string]model.TargetState{}
	}
	if st.System.Phases == nil {
		st.System.Phases = map[string]model.PhaseStatus{}
	}
	_ = o.Store.Save(st)

	components := o.selectedComponents(options.ComponentID)
	if len(components) == 0 {
		return GenerateResult{}, fmt.Errorf("no enabled component matched %q", options.ComponentID)
	}

	rep := report.RunReport{RunID: st.RunID, GeneratedAt: time.Now().UTC(), Components: map[string]model.ValidationResult{}, Failures: map[string]string{}}
	var resultMu sync.Mutex
	groups := repositoryGroups(components)
	workers := o.Config.Execution.ParallelComponents
	if workers < 1 {
		workers = 1
	}
	if workers > len(groups) {
		workers = len(groups)
	}
	jobs := make(chan []config.ComponentConfig)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range jobs {
				// Components sharing a Git repository are deliberately serialized to
				// avoid concurrent OpenWiki writes to repository-level agent files.
				for _, component := range group {
					fmt.Fprintf(o.Out, "[%s] starting type=%s profile=%s scope=%s\n", component.ID, component.Type, component.Profile, printableScope(component.Scope))
					vr, runErr := o.runComponent(ctx, &st, component, options)
					resultMu.Lock()
					if runErr != nil {
						rep.Failures[component.ID] = runErr.Error()
					} else {
						rep.Components[component.ID] = vr
					}
					resultMu.Unlock()
					if runErr != nil {
						fmt.Fprintf(o.Out, "[%s] failed: %v\n", component.ID, runErr)
						if !o.Config.Execution.ContinueOnComponentFailure {
							break
						}
					} else {
						fmt.Fprintf(o.Out, "[%s] complete, score=%d accepted=%t\n", component.ID, vr.Score, vr.Accepted)
					}
				}
			}
		}()
	}
	for _, group := range groups {
		jobs <- group
	}
	close(jobs)
	wg.Wait()

	if len(rep.Failures) > 0 && !o.Config.Execution.ContinueOnComponentFailure {
		dir, _ := report.Write(o.Config.Workspace, rep)
		return GenerateResult{ReportDir: dir, Report: rep}, fmt.Errorf("one or more components failed")
	}

	completed := make([]config.ComponentConfig, 0, len(rep.Components))
	for _, component := range components {
		if _, ok := rep.Components[component.ID]; ok && component.IsIncludedInSystem() {
			completed = append(completed, component)
		}
	}
	if !options.SkipSystem && o.Config.System.Enabled && len(completed) > 0 {
		vr, sysErr := o.runSystem(ctx, &st, completed, options)
		if sysErr != nil {
			rep.Failures["system"] = sysErr.Error()
		} else {
			rep.System = &vr
		}
	}

	dir, err := report.Write(o.Config.Workspace, rep)
	if err != nil {
		return GenerateResult{}, err
	}
	if len(rep.Failures) > 0 {
		return GenerateResult{ReportDir: dir, Report: rep}, fmt.Errorf("generation completed with failures; see %s", dir)
	}
	return GenerateResult{ReportDir: dir, Report: rep}, nil
}

func (o *Orchestrator) runComponent(ctx context.Context, st *model.RunState, component config.ComponentConfig, options GenerateOptions) (model.ValidationResult, error) {
	workdir := component.WorkDir()
	if _, err := os.Stat(workdir); err != nil {
		return model.ValidationResult{}, fmt.Errorf("component %s work directory: %w", component.ID, err)
	}
	if !isGitRepo(component.Repository) {
		return model.ValidationResult{}, fmt.Errorf("component %s repository %s is not a Git repository", component.ID, component.Repository)
	}
	profile, err := prompts.GetProfile(component.Profile)
	if err != nil {
		return model.ValidationResult{}, err
	}
	manifest, adaptivePlan, err := o.prepareAdaptivePlan(component, true)
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("prepare adaptive plan: %w", err)
	}
	adaptiveValues := prompts.AdaptiveValues(manifest, adaptivePlan)
	artifactRoot := filepath.Join(o.Config.Workspace, ".wikiforge", "components", component.ID)
	if rel, relErr := filepath.Rel(component.WorkDir(), filepath.Join(artifactRoot, "discovery.json")); relErr == nil {
		adaptiveValues["DISCOVERY_ARTIFACT"] = filepath.ToSlash(rel)
	}
	if rel, relErr := filepath.Rel(component.WorkDir(), filepath.Join(artifactRoot, "plan.json")); relErr == nil {
		adaptiveValues["PLAN_ARTIFACT"] = filepath.ToSlash(rel)
	}
	discoveryHash := hashValue(manifest)
	planHash := hashValue(adaptivePlan)
	if err := o.writeComponentInstructions(component, profile, manifest, adaptivePlan, adaptiveValues); err != nil {
		return model.ValidationResult{}, err
	}
	removeObsoleteDocumentation(component.DocumentationRoot(), []string{
		"runtime/traffic-and-request-flows.md",
		"security/authentication-and-authorization.md",
		"runtime/concurrency-and-asynchronous-processing.md",
	})

	generationSteps := len(profile.Phases)
	if options.UpdateOnly {
		generationSteps = 1
	}
	progress := newProgressTracker(o.Out, component.ID, generationSteps+2) // generation + validation + report/graph

	target := o.getComponentTarget(st, component.ID)
	if target.Phases == nil {
		target.Phases = map[string]model.PhaseStatus{}
	}
	previousSourceHash := target.SourceHash
	previousDocsHash := target.DocsHash
	previousDiscoveryHash := target.DiscoveryHash
	previousPlanHash := target.PlanHash
	currentSourceHash := componentSourceHash(workdir)
	currentDocsHash := directoryHash(component.DocumentationRoot())
	target.Status = "running"
	target.GitHead = gitHead(component.Repository)
	// Source, discovery, and plan hashes remain the last-successful checkpoint
	// until generation, validation, and finalization complete.
	o.saveComponentTarget(st, component.ID, target)

	if options.UpdateOnly && previousSourceHash != "" && previousSourceHash == currentSourceHash && previousDocsHash != "" && previousDocsHash == currentDocsHash && previousDiscoveryHash == discoveryHash && previousPlanHash == planHash {
		progress.skip("UPD", "No scoped source or documentation changes; model call skipped")
	} else if options.UpdateOnly {
		prompt, err := prompts.RenderComponentUpdateWithValues(profile, component, o.Config.Documentation.Language, adaptiveValues)
		if err != nil {
			return model.ValidationResult{}, err
		}
		label := progress.start("UPD", "Incremental documentation update")
		if err := o.runWithRetries(ctx, workdir, "update", prompt, label); err != nil {
			target.Status = "failed"
			o.saveComponentTarget(st, component.ID, target)
			progress.fail("UPD", err.Error())
			return model.ValidationResult{}, err
		}
		progress.complete("UPD", "Incremental documentation update")
	} else {
		for _, phase := range profile.Phases {
			ps := target.Phases[phase.ID]
			if options.Resume && ps.Status == "completed" {
				progress.skip(phase.ID, phase.Name+" (already completed)")
				continue
			}
			ps.Status = "running"
			ps.Attempts++
			ps.StartedAt = time.Now().UTC()
			target.Phases[phase.ID] = ps
			o.saveComponentTarget(st, component.ID, target)

			prompt, err := prompts.RenderComponentPhase(phase, profile, component, o.Config.Documentation.Language, adaptiveValues)
			if err != nil {
				progress.fail(phase.ID, err.Error())
				return model.ValidationResult{}, err
			}
			op := "prompt"
			if phase.Initialize && !fileExists(filepath.Join(component.DocumentationRoot(), "quickstart.md")) {
				op = "init"
			}
			label := progress.start(phase.ID, phase.Name)
			if err := o.runWithRetries(ctx, workdir, op, prompt, label); err != nil {
				ps.Status = "failed"
				ps.Error = err.Error()
				target.Status = "failed"
				target.Phases[phase.ID] = ps
				o.saveComponentTarget(st, component.ID, target)
				progress.fail(phase.ID, err.Error())
				return model.ValidationResult{}, err
			}
			ps.Status = "completed"
			ps.Error = ""
			ps.CompletedAt = time.Now().UTC()
			target.Phases[phase.ID] = ps
			o.saveComponentTarget(st, component.ID, target)
			progress.complete(phase.ID, phase.Name)
		}
	}

	progress.start("VAL", "Validate generated Markdown, catalogs, links, evidence, and Mermaid")
	vr := o.Validator.ValidateComponent(ctx, component)
	for round := 1; !vr.Accepted && round <= o.Config.Execution.MaxRepairRounds; round++ {
		progress.note("VAL", fmt.Sprintf("Repair round %d/%d for %d findings", round, o.Config.Execution.MaxRepairRounds, len(vr.Findings)))
		repairPrompt, err := prompts.Render("prompts/common/repair.md", o.Config.Documentation.Language, component.ID, map[string]string{
			"FINDINGS":       validation.FindingsText(vr.Findings),
			"PROFILE_NAME":   profile.DisplayName,
			"COMPONENT_TYPE": component.Type,
			"SCOPE":          printableScope(component.Scope),
		})
		if err != nil {
			progress.fail("VAL", err.Error())
			return vr, err
		}
		label := fmt.Sprintf("%s/repair-%d", component.ID, round)
		if err := o.runWithRetries(ctx, workdir, "prompt", repairPrompt, label); err != nil {
			progress.fail("VAL", err.Error())
			return vr, err
		}
		vr = o.Validator.ValidateComponent(ctx, component)
	}
	if vr.Accepted {
		progress.complete("VAL", fmt.Sprintf("Validation accepted with score=%d", vr.Score))
	} else {
		progress.fail("VAL", fmt.Sprintf("Validation rejected with score=%d findings=%d", vr.Score, len(vr.Findings)))
	}

	progress.start("FIN", "Write validation report, export graph, and checkpoint state")
	reportPath := filepath.Join(o.Config.Workspace, ".wikiforge", "validation", component.ID+".json")
	_ = validation.WriteResult(reportPath, vr)
	graphRoot := filepath.Join(o.Config.Workspace, ".wikiforge", "graph", component.ID)
	if err := graph.Export(component.ID, component.DocumentationRoot(), graphRoot); err != nil {
		progress.fail("FIN", err.Error())
		return vr, err
	}

	target = o.getComponentTarget(st, component.ID)
	target.GitHead = gitHead(component.Repository)
	target.SourceHash = componentSourceHash(workdir)
	target.DocsHash = directoryHash(component.DocumentationRoot())
	target.DiscoveryHash = discoveryHash
	target.PlanHash = planHash
	if vr.Accepted {
		target.Status = "completed"
	} else {
		target.Status = "completed-with-findings"
	}
	o.saveComponentTarget(st, component.ID, target)
	progress.complete("FIN", "Reports, graph, and state written")
	if !vr.Accepted {
		return vr, fmt.Errorf("documentation validation failed with score %d", vr.Score)
	}
	return vr, nil
}

func (o *Orchestrator) runSystem(ctx context.Context, st *model.RunState, components []config.ComponentConfig, options GenerateOptions) (model.ValidationResult, error) {
	root := o.Config.System.Output
	systemPlan, err := o.prepareSystemWorkspace(root, components)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if err := o.writeSystemInstructions(root, systemPlan); err != nil {
		return model.ValidationResult{}, err
	}
	removeObsoleteDocumentation(filepath.Join(root, "openwiki"), []string{
		"system/traffic-and-request-flows.md",
		"system/authentication-and-authorization.md",
		"system/concurrency-async-and-context.md",
	})
	if err := ensureGitRepo(root); err != nil {
		return model.ValidationResult{}, err
	}

	generationSteps := len(prompts.SystemPhases)
	if options.UpdateOnly {
		generationSteps = 1
	}
	progress := newProgressTracker(o.Out, "system", generationSteps+2)

	target := o.getSystemTarget(st)
	if target.Phases == nil {
		target.Phases = map[string]model.PhaseStatus{}
	}
	previousSourceHash := target.SourceHash
	previousDocsHash := target.DocsHash
	previousPlanHash := target.PlanHash
	currentPlanHash := hashValue(systemPlan)
	currentSourceHash := directoryHash(filepath.Join(root, "sources")) + directoryHash(filepath.Join(root, "facts"))
	currentDocsHash := directoryHash(filepath.Join(root, "openwiki"))
	target.Status = "running"
	target.GitHead = gitHead(root)
	o.saveSystemTarget(st, target)

	if options.UpdateOnly && previousSourceHash != "" && previousSourceHash == currentSourceHash && previousDocsHash != "" && previousDocsHash == currentDocsHash && previousPlanHash == currentPlanHash {
		progress.skip("UPD", "No source or documentation changes; model call skipped")
	} else if options.UpdateOnly {
		prompt, err := prompts.RenderSystemUpdateWithPlan(o.Config.Documentation.Language, o.Config.System.ID, systemPlan)
		if err != nil {
			return model.ValidationResult{}, err
		}
		label := progress.start("UPD", "Incremental whole-system update")
		if err := o.runWithRetries(ctx, root, "update", prompt, label); err != nil {
			target.Status = "failed"
			o.saveSystemTarget(st, target)
			progress.fail("UPD", err.Error())
			return model.ValidationResult{}, err
		}
		progress.complete("UPD", "Incremental whole-system update")
	} else {
		for _, phase := range prompts.SystemPhases {
			ps := target.Phases[phase.ID]
			if options.Resume && ps.Status == "completed" {
				progress.skip(phase.ID, phase.Name+" (already completed)")
				continue
			}
			ps.Status = "running"
			ps.Attempts++
			ps.StartedAt = time.Now().UTC()
			target.Phases[phase.ID] = ps
			o.saveSystemTarget(st, target)

			prompt, err := prompts.RenderSystemPhaseWithPlan(phase, o.Config.Documentation.Language, o.Config.System.ID, systemPlan)
			if err != nil {
				progress.fail(phase.ID, err.Error())
				return model.ValidationResult{}, err
			}
			op := "prompt"
			if phase.Initialize && !fileExists(filepath.Join(root, "openwiki", "quickstart.md")) {
				op = "init"
			}
			label := progress.start(phase.ID, phase.Name)
			if err := o.runWithRetries(ctx, root, op, prompt, label); err != nil {
				ps.Status = "failed"
				ps.Error = err.Error()
				target.Status = "failed"
				target.Phases[phase.ID] = ps
				o.saveSystemTarget(st, target)
				progress.fail(phase.ID, err.Error())
				return model.ValidationResult{}, err
			}
			ps.Status = "completed"
			ps.Error = ""
			ps.CompletedAt = time.Now().UTC()
			target.Phases[phase.ID] = ps
			o.saveSystemTarget(st, target)
			progress.complete(phase.ID, phase.Name)
		}
	}

	progress.start("VAL", "Validate whole-system Markdown and catalogs")
	vr := o.Validator.ValidateSystem(ctx)
	for round := 1; !vr.Accepted && round <= o.Config.Execution.MaxRepairRounds; round++ {
		progress.note("VAL", fmt.Sprintf("Repair round %d/%d for %d findings", round, o.Config.Execution.MaxRepairRounds, len(vr.Findings)))
		repairPrompt, err := prompts.Render("prompts/common/repair.md", o.Config.Documentation.Language, o.Config.System.ID, map[string]string{
			"FINDINGS":       validation.FindingsText(vr.Findings),
			"PROFILE_NAME":   "Whole-System Landscape",
			"COMPONENT_TYPE": "system",
			"SCOPE":          "aggregation workspace",
		})
		if err != nil {
			progress.fail("VAL", err.Error())
			return vr, err
		}
		label := fmt.Sprintf("system/repair-%d", round)
		if err := o.runWithRetries(ctx, root, "prompt", repairPrompt, label); err != nil {
			progress.fail("VAL", err.Error())
			return vr, err
		}
		vr = o.Validator.ValidateSystem(ctx)
	}
	if vr.Accepted {
		progress.complete("VAL", fmt.Sprintf("Validation accepted with score=%d", vr.Score))
	} else {
		progress.fail("VAL", fmt.Sprintf("Validation rejected with score=%d findings=%d", vr.Score, len(vr.Findings)))
	}

	progress.start("FIN", "Write system report, export graph, and checkpoint state")
	_ = validation.WriteResult(filepath.Join(o.Config.Workspace, ".wikiforge", "validation", "system.json"), vr)
	if err := graph.Export(o.Config.System.ID, filepath.Join(root, "openwiki"), filepath.Join(o.Config.Workspace, ".wikiforge", "graph", "system")); err != nil {
		progress.fail("FIN", err.Error())
		return vr, err
	}
	target.GitHead = gitHead(root)
	target.SourceHash = directoryHash(filepath.Join(root, "sources")) + directoryHash(filepath.Join(root, "facts"))
	target.DocsHash = directoryHash(filepath.Join(root, "openwiki"))
	target.PlanHash = currentPlanHash
	if vr.Accepted {
		target.Status = "completed"
	} else {
		target.Status = "completed-with-findings"
	}
	o.saveSystemTarget(st, target)
	progress.complete("FIN", "Reports, graph, and state written")
	if !vr.Accepted {
		return vr, fmt.Errorf("system documentation validation failed with score %d", vr.Score)
	}
	return vr, nil
}

func (o *Orchestrator) runWithRetries(ctx context.Context, workdir, operation, prompt, label string) error {
	attempts := o.Config.Execution.MaxProcessRetries + 1
	var last error
	for i := 1; i <= attempts; i++ {
		if attempts > 1 {
			fmt.Fprintf(o.Out, "[%s] process attempt %d/%d\n", label, i, attempts)
		}
		runCtx := openwiki.WithRunLabel(ctx, label)
		_, err := o.Runner.Run(runCtx, workdir, operation, prompt)
		if err == nil {
			return nil
		}
		last = err
		fmt.Fprintf(o.Out, "[%s] process attempt %d/%d failed: %v\n", label, i, attempts, err)
		if openwiki.IsNonRetryableError(err) {
			fmt.Fprintf(o.Out, "[%s] deterministic invocation/path failure; retries skipped\n", label)
			return err
		}
		if i < attempts {
			delay := time.Duration(i) * 2 * time.Second
			fmt.Fprintf(o.Out, "[%s] retrying in %s\n", label, delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return last
}

func (o *Orchestrator) writeComponentInstructions(component config.ComponentConfig, profile prompts.Profile, manifest model.DiscoveryManifest, plan model.DocumentationPlan, values map[string]string) error {
	content, err := prompts.RenderInstructionsWithPlanValues(profile, component, o.Config.Documentation.Language, manifest, plan, values)
	if err != nil {
		return err
	}
	return writeMergedInstructions(filepath.Join(component.DocumentationRoot(), "INSTRUCTIONS.md"), content)
}

func (o *Orchestrator) writeSystemInstructions(root string, plan model.DocumentationPlan) error {
	content, err := prompts.RenderSystemInstructions(o.Config.Documentation.Language, o.Config.System.ID, plan)
	if err != nil {
		return err
	}
	return writeMergedInstructions(filepath.Join(root, "openwiki", "INSTRUCTIONS.md"), content)
}

func writeMergedInstructions(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	old, _ := os.ReadFile(path)
	merged := mergeMarker(string(old), content, "<!-- WIKIFORGE:START -->", "<!-- WIKIFORGE:END -->")
	return os.WriteFile(path, []byte(merged), 0o644)
}

func mergeMarker(existing, replacement, start, end string) string {
	replacement = strings.TrimSpace(replacement)
	if strings.TrimSpace(existing) == "" {
		return replacement + "\n"
	}
	a := strings.Index(existing, start)
	b := strings.Index(existing, end)
	if a >= 0 && b >= a {
		prefix := strings.TrimRight(existing[:a], " \t\r\n")
		suffix := strings.TrimLeft(existing[b+len(end):], " \t\r\n")
		var parts []string
		if prefix != "" {
			parts = append(parts, prefix)
		}
		parts = append(parts, replacement)
		if suffix != "" {
			parts = append(parts, suffix)
		}
		return strings.Join(parts, "\n\n") + "\n"
	}
	return strings.TrimRight(existing, " \t\r\n") + "\n\n" + replacement + "\n"
}

func (o *Orchestrator) prepareSystemWorkspace(root string, components []config.ComponentConfig) (model.DocumentationPlan, error) {
	systemPlan, err := o.prepareSystemAdaptivePlan(components, true)
	if err != nil {
		return model.DocumentationPlan{}, err
	}
	sourcesRoot := filepath.Join(root, "sources")
	componentsRoot := filepath.Join(sourcesRoot, "components")
	if err := os.RemoveAll(componentsRoot); err != nil {
		return model.DocumentationPlan{}, err
	}
	if err := os.MkdirAll(componentsRoot, 0o755); err != nil {
		return model.DocumentationPlan{}, err
	}

	type manifestComponent struct {
		ID               string   `json:"id"`
		Type             string   `json:"type"`
		Profile          string   `json:"profile"`
		Group            string   `json:"group,omitempty"`
		Tags             []string `json:"tags,omitempty"`
		DependsOn        []string `json:"dependsOn,omitempty"`
		SourceRepository string   `json:"sourceRepository"`
		Scope            string   `json:"scope,omitempty"`
		GitHead          string   `json:"gitHead,omitempty"`
		Documentation    string   `json:"documentation"`
		Discovery        string   `json:"discovery,omitempty"`
		Plan             string   `json:"plan,omitempty"`
	}
	manifest := struct {
		SchemaVersion int                 `json:"schemaVersion"`
		SystemID      string              `json:"systemId"`
		Title         string              `json:"title"`
		Components    []manifestComponent `json:"components"`
	}{SchemaVersion: 2, SystemID: o.Config.System.ID, Title: o.Config.System.Title}

	for _, component := range components {
		src := component.DocumentationRoot()
		if !fileExists(src) {
			continue
		}
		componentRoot := filepath.Join(componentsRoot, component.ID)
		dst := filepath.Join(componentRoot, "openwiki")
		if err := copyDir(src, dst); err != nil {
			return model.DocumentationPlan{}, err
		}
		adaptiveRoot := filepath.Join(o.Config.Workspace, ".wikiforge", "components", component.ID)
		for _, name := range []string{"discovery.json", "plan.json"} {
			source := filepath.Join(adaptiveRoot, name)
			if fileExists(source) {
				data, err := os.ReadFile(source)
				if err != nil {
					return model.DocumentationPlan{}, err
				}
				if err := os.WriteFile(filepath.Join(componentRoot, name), data, 0o644); err != nil {
					return model.DocumentationPlan{}, err
				}
			}
		}
		manifest.Components = append(manifest.Components, manifestComponent{
			ID: component.ID, Type: component.Type, Profile: component.Profile,
			Group: component.Group, Tags: component.Tags, DependsOn: component.DependsOn,
			SourceRepository: component.Repository, Scope: component.Scope,
			GitHead:       gitHead(component.Repository),
			Documentation: "sources/components/" + component.ID + "/openwiki/quickstart.md",
			Discovery:     "sources/components/" + component.ID + "/discovery.json",
			Plan:          "sources/components/" + component.ID + "/plan.json",
		})
	}
	sort.Slice(manifest.Components, func(i, j int) bool { return manifest.Components[i].ID < manifest.Components[j].ID })
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(sourcesRoot, "manifest.json"), b, 0o644); err != nil {
		return model.DocumentationPlan{}, err
	}
	if err := writeJSON(filepath.Join(sourcesRoot, "system-plan.json"), systemPlan); err != nil {
		return model.DocumentationPlan{}, err
	}

	if o.Config.System.FactsPath != "" && fileExists(o.Config.System.FactsPath) {
		dst := filepath.Join(root, "facts")
		if filepath.Clean(o.Config.System.FactsPath) != filepath.Clean(dst) {
			if err := os.RemoveAll(dst); err != nil {
				return model.DocumentationPlan{}, err
			}
			if err := copyDir(o.Config.System.FactsPath, dst); err != nil {
				return model.DocumentationPlan{}, err
			}
		}
	}
	readme := "# WikiForge System Aggregation Workspace\n\nThe `sources/components/` directory contains immutable snapshots of generated component wikis plus their deterministic discovery manifests and adaptive plans. Components can be applications, modules, libraries, frameworks, contracts, infrastructure, or configuration. OpenWiki must synthesize the whole-system wiki under `openwiki/` and must not modify source snapshots.\n"
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644); err != nil {
		return model.DocumentationPlan{}, err
	}
	return systemPlan, nil
}

func (o *Orchestrator) getComponentTarget(st *model.RunState, id string) model.TargetState {
	o.stateMu.Lock()
	defer o.stateMu.Unlock()
	return cloneTargetState(st.Components[id])
}

func (o *Orchestrator) saveComponentTarget(st *model.RunState, id string, target model.TargetState) {
	o.stateMu.Lock()
	st.Components[id] = cloneTargetState(target)
	_ = o.Store.Save(*st)
	o.stateMu.Unlock()
}

func (o *Orchestrator) getSystemTarget(st *model.RunState) model.TargetState {
	o.stateMu.Lock()
	defer o.stateMu.Unlock()
	return cloneTargetState(st.System)
}

func (o *Orchestrator) saveSystemTarget(st *model.RunState, target model.TargetState) {
	o.stateMu.Lock()
	st.System = cloneTargetState(target)
	_ = o.Store.Save(*st)
	o.stateMu.Unlock()
}

func cloneTargetState(in model.TargetState) model.TargetState {
	out := in
	if in.Phases != nil {
		out.Phases = make(map[string]model.PhaseStatus, len(in.Phases))
		for id, status := range in.Phases {
			out.Phases[id] = status
		}
	}
	return out
}

func (o *Orchestrator) selectedComponents(id string) []config.ComponentConfig {
	var out []config.ComponentConfig
	for _, component := range o.Config.EnabledComponents() {
		if id == "" || component.ID == id {
			out = append(out, component)
		}
	}
	return out
}

func repositoryGroups(components []config.ComponentConfig) [][]config.ComponentConfig {
	byRepo := map[string][]config.ComponentConfig{}
	for _, component := range components {
		key := filepath.Clean(component.Repository)
		byRepo[key] = append(byRepo[key], component)
	}
	keys := make([]string, 0, len(byRepo))
	for key := range byRepo {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	groups := make([][]config.ComponentConfig, 0, len(keys))
	for _, key := range keys {
		group := byRepo[key]
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
		groups = append(groups, group)
	}
	return groups
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func ensureGitRepo(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if isGitRepo(root) {
		return nil
	}
	if err := runGit(root, "init"); err != nil {
		return err
	}
	_ = runGit(root, "config", "user.email", "wikiforge@local.invalid")
	_ = runGit(root, "config", "user.name", "WikiForge")
	_ = runGit(root, "add", ".")
	_ = runGit(root, "commit", "-m", "Initialize WikiForge aggregation workspace")
	return nil
}

func isGitRepo(root string) bool {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func gitHead(root string) string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	b, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func runGit(root string, args ...string) error {
	all := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", all...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return nil
}

func removeObsoleteDocumentation(root string, relativePaths []string) {
	for _, rel := range relativePaths {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func directoryHash(root string) string {
	return hashDirectory(root, nil)
}

func componentSourceHash(root string) string {
	excludedDirs := map[string]bool{
		".git": true, "openwiki": true, ".wikiforge": true, "node_modules": true,
		".venv": true, ".terraform": true, "target": true, "bin": true, "obj": true,
	}
	excludedFiles := map[string]bool{"AGENTS.md": true, "CLAUDE.md": true}
	return hashDirectory(root, func(path string, d os.DirEntry) bool {
		name := d.Name()
		if d.IsDir() && excludedDirs[name] {
			return true
		}
		return !d.IsDir() && excludedFiles[name]
	})
}

func hashDirectory(root string, skip func(string, os.DirEntry) bool) string {
	h := sha256.New()
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if skip != nil && skip(path, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	for _, p := range files {
		rel, _ := filepath.Rel(root, p)
		h.Write([]byte(filepath.ToSlash(rel)))
		b, _ := os.ReadFile(p)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func migrateState(st *model.RunState) {
	if st.Components == nil {
		st.Components = map[string]model.TargetState{}
	}
	for id, target := range st.Services {
		if _, exists := st.Components[id]; !exists {
			st.Components[id] = target
		}
	}
	st.Services = nil
	if st.Version < 3 {
		st.Version = 3
	}
}

func printableScope(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return "."
	}
	return filepath.ToSlash(scope)
}
