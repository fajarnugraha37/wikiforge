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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/discovery"
	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/openwiki"
	"github.com/fajarnugraha37/wikiforge/internal/report"
	"github.com/fajarnugraha37/wikiforge/internal/state"
	"github.com/fajarnugraha37/wikiforge/internal/validation"
)

type Orchestrator struct {
	Config    config.Config
	Runner    openwiki.Runner
	Validator validation.Validator
	Store     *state.Store
	Out       io.Writer
	stateMu   sync.Mutex
	metricsMu sync.Mutex
	metrics   *model.RunMetrics
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

func (o *Orchestrator) Generate(ctx context.Context, options GenerateOptions) (GenerateResult, error) {
	started := time.Now().UTC()
	metrics := &model.RunMetrics{StartedAt: started}
	o.metricsMu.Lock()
	o.metrics = metrics
	o.metricsMu.Unlock()
	defer func() {
		metrics.CompletedAt = time.Now().UTC()
		metrics.DurationMillis = metrics.CompletedAt.Sub(metrics.StartedAt).Milliseconds()
		o.metricsMu.Lock()
		o.metrics = nil
		o.metricsMu.Unlock()
	}()
	if err := os.MkdirAll(filepath.Join(o.Config.Workspace, ".wikiforge"), 0o755); err != nil {
		return GenerateResult{}, err
	}
	st, err := o.Store.Load()
	if err != nil {
		return GenerateResult{}, err
	}
	if st.RunID == "" || (!options.Resume && !options.UpdateOnly) {
		st = model.RunState{
			Version:    3,
			RunID:      time.Now().UTC().Format("20060102T150405Z"),
			Mode:       "generate",
			StartedAt:  time.Now().UTC(),
			Components: map[string]model.TargetState{},
			System:     model.TargetState{Phases: map[string]model.PhaseStatus{}},
		}
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
	groups := repositoryGroups(components, o.Config.Execution.IsolateSameRepository)
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
					var vr model.ValidationResult
					var runErr error
					effective, cleanup, isolationErr := o.isolatedComponent(component)
					if isolationErr != nil {
						runErr = isolationErr
					} else {
						vr, runErr = o.runAdaptiveComponent(ctx, &st, effective, options)
						if cleanupErr := cleanup(); runErr == nil && cleanupErr != nil {
							runErr = cleanupErr
						}
					}
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
		rep.Metrics = *metrics
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
		var vr model.ValidationResult
		var sysErr error
		vr, sysErr = o.runAdaptiveSystem(ctx, &st, completed, options)
		if sysErr != nil {
			rep.Failures["system"] = sysErr.Error()
		} else {
			rep.System = &vr
		}
	}

	metrics.CompletedAt = time.Now().UTC()
	metrics.DurationMillis = metrics.CompletedAt.Sub(metrics.StartedAt).Milliseconds()
	rep.Metrics = *metrics
	dir, err := report.Write(o.Config.Workspace, rep)
	if err != nil {
		return GenerateResult{}, err
	}
	if len(rep.Failures) > 0 {
		return GenerateResult{ReportDir: dir, Report: rep}, fmt.Errorf("generation completed with failures; see %s", dir)
	}
	return GenerateResult{ReportDir: dir, Report: rep}, nil
}

func (o *Orchestrator) runWithRetries(ctx context.Context, workdir, operation, prompt, label string) error {
	attempts := o.Config.Execution.MaxProcessRetries + 1
	var last error
	for i := 1; i <= attempts; i++ {
		o.metricsMu.Lock()
		if o.metrics != nil {
			o.metrics.OpenWikiCalls++
		}
		o.metricsMu.Unlock()
		if attempts > 1 {
			fmt.Fprintf(o.Out, "[%s] process attempt %d/%d\n", label, i, attempts)
		}
		runCtx := openwiki.WithRunLabel(ctx, label)
		output, err := o.Runner.Run(runCtx, workdir, operation, prompt)
		o.recordUsage(output)
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

func (o *Orchestrator) recordUsage(output string) {
	input := metricValue(output, "input_tokens", "input tokens", "prompt_tokens")
	outputTokens := metricValue(output, "output_tokens", "output tokens", "completion_tokens")
	if input == 0 && outputTokens == 0 {
		return
	}
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()
	if o.metrics != nil {
		o.metrics.InputTokens += int64(input)
		o.metrics.OutputTokens += int64(outputTokens)
		o.metrics.UsageReported = true
	}
}

func metricValue(text string, labels ...string) int {
	lower := strings.ToLower(text)
	for _, label := range labels {
		start := strings.Index(lower, label)
		if start < 0 {
			continue
		}
		for _, candidate := range []string{":", "=", " ", "\""} {
			if index := strings.Index(lower[start+len(label):], candidate); index >= 0 {
				value := lower[start+len(label)+index+len(candidate):]
				value = strings.TrimLeft(value, " \t\"'")
				n := 0
				for n < len(value) && value[n] >= '0' && value[n] <= '9' {
					n++
				}
				if n > 0 {
					parsed, _ := strconv.Atoi(value[:n])
					return parsed
				}
			}
		}
	}
	return 0
}

func (o *Orchestrator) recordPageMetric(update bool) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()
	if o.metrics == nil {
		return
	}
	if update {
		o.metrics.PagesUpdated++
	} else {
		o.metrics.PagesGenerated++
	}
}

func (o *Orchestrator) recordEvidenceMetrics(index evidence.Index) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()
	if o.metrics == nil {
		return
	}
	o.metrics.EvidenceFiles += len(index.References)
	o.metrics.EvidenceCacheHits += index.CacheHits
	o.metrics.EvidenceCacheMisses += index.CacheMisses
}

func (o *Orchestrator) recordDiscoveryMetrics(run discovery.RunMetrics, result discovery.SemanticDiscovery) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()
	if o.metrics == nil {
		return
	}
	o.metrics.DiscoveryStages += run.Stages
	o.metrics.DiscoveryCalls += run.Calls
	if run.CacheHit {
		o.metrics.DiscoveryCacheHits++
	} else {
		o.metrics.DiscoveryCacheMisses++
	}
	o.metrics.DiscoveryAccepted += result.Quality.AcceptedCount
	o.metrics.DiscoveryUncertain += result.Quality.UncertainCount
	o.metrics.DiscoveryConflicting += result.Quality.ConflictingCount
	o.metrics.DiscoveryUnknown += result.Quality.UnknownCount
	o.metrics.DiscoveryStageMetrics = append(o.metrics.DiscoveryStageMetrics, run.StageMetrics...)
	o.metrics.DiscoveryCounts.Modules += run.Counts.Modules
	o.metrics.DiscoveryCounts.Domains += run.Counts.Domains
	o.metrics.DiscoveryCounts.Flows += run.Counts.Flows
	o.metrics.DiscoveryCounts.Concerns += run.Counts.Concerns
	o.metrics.DiscoveryCounts.Ownership += run.Counts.Ownership
	o.metrics.DiscoveryCounts.Relationships += run.Counts.Relationships
	o.metrics.DiscoveryInventoryVersions = appendUnique(o.metrics.DiscoveryInventoryVersions, result.InventoryVersion)
	o.metrics.DiscoveryPromptVersions = appendUnique(o.metrics.DiscoveryPromptVersions, result.PromptVersion)
	o.metrics.DiscoveryModelIDs = appendUnique(o.metrics.DiscoveryModelIDs, result.ModelID)
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (o *Orchestrator) prepareSystemWorkspace(root string, components []config.ComponentConfig) error {
	sourcesRoot := filepath.Join(root, "sources")
	componentsRoot := filepath.Join(sourcesRoot, "components")
	if err := os.MkdirAll(componentsRoot, 0o755); err != nil {
		return err
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
		SnapshotHash     string   `json:"snapshotHash"`
		Documentation    string   `json:"documentation"`
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
		snapshotHash := directoryHash(src)
		dst := filepath.Join(componentsRoot, component.ID, snapshotHash, "openwiki")
		if !fileExists(dst) {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		}
		manifest.Components = append(manifest.Components, manifestComponent{
			ID: component.ID, Type: component.Type, Profile: component.Profile,
			Group: component.Group, Tags: component.Tags, DependsOn: component.DependsOn,
			SourceRepository: component.Repository, Scope: component.Scope,
			GitHead:       gitHead(component.Repository),
			SnapshotHash:  snapshotHash,
			Documentation: "sources/components/" + component.ID + "/" + snapshotHash + "/openwiki/quickstart.md",
		})
	}
	sort.Slice(manifest.Components, func(i, j int) bool { return manifest.Components[i].ID < manifest.Components[j].ID })
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(sourcesRoot, "manifest.json"), b, 0o644); err != nil {
		return err
	}

	if o.Config.System.FactsPath != "" && fileExists(o.Config.System.FactsPath) {
		dst := filepath.Join(root, "facts")
		if filepath.Clean(o.Config.System.FactsPath) != filepath.Clean(dst) {
			if err := syncDir(o.Config.System.FactsPath, dst); err != nil {
				return err
			}
		}
	}
	readme := "# WikiForge System Aggregation Workspace\n\nThe `sources/components/` directory contains immutable snapshots of generated component wikis. Components can be applications, modules, libraries, frameworks, contracts, infrastructure, or configuration. OpenWiki must synthesize the whole-system wiki under `openwiki/` and must not modify source snapshots.\n"
	return os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644)
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

func repositoryGroups(components []config.ComponentConfig, isolate bool) [][]config.ComponentConfig {
	byRepo := map[string][]config.ComponentConfig{}
	for _, component := range components {
		key := filepath.Clean(component.Repository)
		if isolate && component.Scope != "" {
			// Different scoped worktrees can run concurrently. Components with the
			// same scope still share one key and remain serialized.
			key += "\x00" + filepath.Clean(component.Scope)
		}
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

func (o *Orchestrator) isolatedComponent(component config.ComponentConfig) (config.ComponentConfig, func() error, error) {
	if !o.Config.Execution.IsolateSameRepository || strings.TrimSpace(component.Scope) == "" {
		return component, func() error { return nil }, nil
	}
	stageParent := filepath.Join(o.Config.Workspace, ".wikiforge", "staging")
	if err := os.MkdirAll(stageParent, 0o755); err != nil {
		return component, func() error { return nil }, fmt.Errorf("create staging root: %w", err)
	}
	stageRoot, err := os.MkdirTemp(stageParent, component.ID+"-")
	if err != nil {
		return component, func() error { return nil }, fmt.Errorf("create isolated staging for %s: %w", component.ID, err)
	}
	if err := copyDir(component.WorkDir(), stageRoot); err != nil {
		_ = os.RemoveAll(stageRoot)
		return component, func() error { return nil }, fmt.Errorf("stage component %s: %w", component.ID, err)
	}
	if err := ensureGitRepo(stageRoot); err != nil {
		_ = os.RemoveAll(stageRoot)
		return component, func() error { return nil }, fmt.Errorf("initialize isolated Git worktree for %s: %w", component.ID, err)
	}
	effective := component
	effective.Repository = stageRoot
	effective.Scope = ""
	return effective, func() error {
		if err := syncDir(filepath.Join(stageRoot, "openwiki"), component.DocumentationRoot()); err != nil {
			_ = os.RemoveAll(stageRoot)
			return fmt.Errorf("sync isolated documentation for %s: %w", component.ID, err)
		}
		return os.RemoveAll(stageRoot)
	}, nil
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

func syncDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	seen := map[string]bool{}
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.Clean(rel)
		seen[rel] = true
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	}); err != nil {
		return err
	}
	var stale []string
	_ = filepath.WalkDir(dst, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || path == dst {
			return nil
		}
		rel, relErr := filepath.Rel(dst, path)
		if relErr == nil && !seen[filepath.Clean(rel)] {
			stale = append(stale, path)
		}
		return nil
	})
	sort.Slice(stale, func(i, j int) bool { return len(stale[i]) > len(stale[j]) })
	for _, path := range stale {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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

func printableScope(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return "."
	}
	return filepath.ToSlash(scope)
}
