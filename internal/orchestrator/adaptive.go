package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/discovery"
	"github.com/fajarnugraha37/wikiforge/internal/evidence"
	"github.com/fajarnugraha37/wikiforge/internal/graph"
	"github.com/fajarnugraha37/wikiforge/internal/model"
	"github.com/fajarnugraha37/wikiforge/internal/planner"
	"github.com/fajarnugraha37/wikiforge/internal/prompts"
	"github.com/fajarnugraha37/wikiforge/internal/validation"
)

func (o *Orchestrator) runAdaptiveComponent(ctx context.Context, st *model.RunState, component config.ComponentConfig, options GenerateOptions) (model.ValidationResult, error) {
	if _, err := os.Stat(component.WorkDir()); err != nil {
		return model.ValidationResult{}, fmt.Errorf("component %s work directory: %w", component.ID, err)
	}
	if !isGitRepo(component.Repository) {
		return model.ValidationResult{}, fmt.Errorf("component %s repository %s is not a Git repository", component.ID, component.Repository)
	}
	artifacts := componentEvidenceArtifacts(o.Config.Workspace, component.ID)
	_, semantic, _, discoveryMetrics, err := o.ensureSemanticDiscovery(ctx, component, artifacts)
	if err != nil {
		return model.ValidationResult{}, err
	}
	o.recordDiscoveryMetrics(discoveryMetrics, semantic)
	plannerForComponent := planner.Planner{Config: o.Config, Semantic: map[string]discovery.SemanticDiscovery{component.ID: semantic}}
	planResult, err := plannerForComponent.Plan(component.ID, false)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if len(planResult.Components) != 1 {
		return model.ValidationResult{}, fmt.Errorf("adaptive plan for component %s is missing or ambiguous", component.ID)
	}
	componentPlan := planResult.Components[0]
	discoveryManifest, err := plannerForComponent.Discover(component.ID)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if err := savePlanArtifacts(componentPlan, discoveryManifest, artifacts); err != nil {
		return model.ValidationResult{}, err
	}
	profile, err := prompts.GetProfile(component.Profile)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if err := writeAdaptiveInstructions(component, profile, componentPlan, o.Config.Documentation.Language); err != nil {
		return model.ValidationResult{}, err
	}
	removeUnplannedDocumentationFiles(component.DocumentationRoot(), pagesFromPlan(componentPlan.Pages))

	pages := plannedPaths(componentPlan.Pages)
	progress := newProgressTracker(o.Out, component.ID, len(pages)+2)
	target := o.getComponentTarget(st, component.ID)
	if target.Phases == nil {
		target.Phases = map[string]model.PhaseStatus{}
	}
	previousSourceHash := target.SourceHash
	previousDocsHash := target.DocsHash
	workdir := component.WorkDir()
	cachePath := evidenceCachePath(o.Config.Documentation.Evidence.CacheDirectory, component.ID)
	maxEvidenceBytes := o.Config.Documentation.Evidence.MaxFileBytes
	include := evidenceRootsForPlan(o.Config.Documentation.Evidence.Include, componentPlan)
	index, impact, err := prepareEvidence(workdir, component.ID, include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, componentPlan, target, artifacts)
	if err != nil {
		return model.ValidationResult{}, err
	}
	o.recordEvidenceMetrics(index)
	currentPlanHash := planFingerprint(componentPlan)
	if target.PlanHash != "" && target.PlanHash != currentPlanHash {
		forceFullImpact(componentPlan, &impact, "documentation plan changed structurally; full-scope scan")
		if err := evidence.SaveJSON(artifacts.Impact, impact); err != nil {
			return model.ValidationResult{}, err
		}
	}
	currentSourceHash := evidenceSourceHash(index)
	currentDocsHash := directoryHash(component.DocumentationRoot())
	target.Status = "running"
	target.DiscoveryMode = semantic.DiscoveryMode
	target.SourceRevision = semantic.SourceRevision
	target.InventoryFingerprint = semantic.InventoryFingerprint
	target.SemanticFingerprint = componentPlan.SemanticFingerprint
	target.DiscoveryInventoryVersion = semantic.InventoryVersion
	target.DiscoveryPromptVersion = semantic.PromptVersion
	target.DiscoveryModelID = semantic.ModelID
	target.SemanticDiscoveryPath = artifacts.Semantic
	target.SemanticIdentitiesPath = artifacts.Identities
	target.DiscoveryCalls = discoveryMetrics.Calls
	target.DiscoveryCacheHit = discoveryMetrics.CacheHit
	target.DiscoveryCounts = discoveryMetrics.Counts
	target.DiscoveryStageMetrics = discoveryMetrics.StageMetrics
	target.GitHead = gitHead(component.Repository)
	target.SourceHash = currentSourceHash
	target.PlanHash = currentPlanHash
	target.EvidenceRevision = index.Revision
	target.EvidenceIndexPath = artifacts.Index
	target.ImpactIndexPath = artifacts.Impact
	target.CoveragePath = artifacts.Coverage
	o.saveComponentTarget(st, component.ID, target)

	if options.UpdateOnly && previousSourceHash != "" && previousSourceHash == currentSourceHash && previousDocsHash != "" && previousDocsHash == currentDocsHash {
		progress.skip("UPD", "No scoped source or documentation changes; model call skipped")
	} else if options.UpdateOnly {
		updatePages := affectedPagePaths(componentPlan, impact)
		if currentDocsHash != previousDocsHash {
			updatePages = pages
		}
		updatePages = ensureAffectedPages(componentPlan, &impact, updatePages)
		if err := evidence.SaveJSON(artifacts.Impact, impact); err != nil {
			return model.ValidationResult{}, err
		}
		prompt, err := prompts.RenderAdaptiveUpdate(profile, component, updatePages, joinDocumentationViews(componentPlan.Views), o.Config.Documentation.Language)
		if err != nil {
			return model.ValidationResult{}, err
		}
		label := progress.start("UPD", "Adaptive documentation update")
		if err := o.runWithRetries(ctx, workdir, "update", prompt, label); err != nil {
			progress.fail("UPD", err.Error())
			return model.ValidationResult{}, err
		}
		progress.complete("UPD", "Adaptive documentation update")
	} else {
		for _, page := range componentPlan.Pages {
			o.recordPageMetric(options.UpdateOnly)
			key := "page:" + filepath.ToSlash(page.Path)
			status := target.Phases[key]
			if options.Resume && status.Status == "completed" {
				progress.skip(key, page.Path+" (already completed)")
				continue
			}
			status.Status = "running"
			status.Attempts++
			status.StartedAt = time.Now().UTC()
			target.Phases[key] = status
			o.saveComponentTarget(st, component.ID, target)
			prompt, err := prompts.RenderAdaptivePage(page.Path, string(page.View), string(page.Kind), page.OwnerUnit, component.Profile, component, adaptiveUnitContext(componentPlan.Units, page.OwnerUnit), joinDocumentationViews(componentPlan.Views), strings.Join(componentPlan.Packs, ", "), o.Config.Documentation.Language)
			if err != nil {
				return model.ValidationResult{}, err
			}
			op := "prompt"
			if page.Path == "quickstart.md" && !fileExists(filepath.Join(component.DocumentationRoot(), "quickstart.md")) {
				op = "init"
			}
			label := progress.start(key, page.Path)
			if err := o.runWithRetries(ctx, workdir, op, prompt, label); err != nil {
				status.Status = "failed"
				status.Error = err.Error()
				target.Phases[key] = status
				o.saveComponentTarget(st, component.ID, target)
				progress.fail(key, err.Error())
				return model.ValidationResult{}, err
			}
			status.Status = "completed"
			status.Error = ""
			status.CompletedAt = time.Now().UTC()
			target.Phases[key] = status
			o.saveComponentTarget(st, component.ID, target)
			progress.complete(key, page.Path)
		}
	}

	progress.start("VAL", "Validate adaptive hierarchy, links, evidence, and Mermaid")
	finalIndex, err := finalizeEvidence(workdir, component.ID, include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, componentPlan, artifacts)
	if err != nil {
		return model.ValidationResult{}, err
	}
	o.recordEvidenceMetrics(finalIndex)
	vr := o.Validator.ValidateAdaptiveComponent(ctx, component, componentPlan)
	vr.Findings = append(vr.Findings, validation.ValidateEvidenceBacked(component.DocumentationRoot(), finalIndex)...)
	vr = validation.Recalculate(vr, o.Config.Documentation.MinimumQualityScore)
	for round := 1; !vr.Accepted && round <= o.Config.Execution.MaxRepairRounds; round++ {
		progress.note("VAL", fmt.Sprintf("Repair round %d/%d for %d findings", round, o.Config.Execution.MaxRepairRounds, len(vr.Findings)))
		repairPrompt, err := prompts.Render("prompts/common/repair.md", o.Config.Documentation.Language, component.ID, map[string]string{
			"FINDINGS":        validation.FindingsText(vr.Findings),
			"PROFILE_NAME":    profile.DisplayName,
			"COMPONENT_TYPE":  component.Type,
			"SCOPE":           printableScope(component.Scope),
			"CANONICAL_FILES": strings.Join(pages, "\n"),
		})
		if err != nil {
			return vr, err
		}
		label := fmt.Sprintf("%s/adaptive-repair-%d", component.ID, round)
		if err := o.runWithRetries(ctx, workdir, "prompt", repairPrompt, label); err != nil {
			return vr, err
		}
		finalIndex, err = finalizeEvidence(workdir, component.ID, include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, componentPlan, artifacts)
		if err != nil {
			return vr, err
		}
		vr = o.Validator.ValidateAdaptiveComponent(ctx, component, componentPlan)
		vr.Findings = append(vr.Findings, validation.ValidateEvidenceBacked(component.DocumentationRoot(), finalIndex)...)
		vr = validation.Recalculate(vr, o.Config.Documentation.MinimumQualityScore)
	}
	progress.complete("VAL", fmt.Sprintf("Adaptive validation score=%d accepted=%t", vr.Score, vr.Accepted))

	progress.start("FIN", "Write validation report, export graph, and checkpoint state")
	_ = validation.WriteResult(filepath.Join(o.Config.Workspace, ".wikiforge", "validation", component.ID+".json"), vr)
	if err := graph.ExportWithEvidence(component.ID, component.DocumentationRoot(), filepath.Join(o.Config.Workspace, ".wikiforge", "graph", component.ID), finalIndex); err != nil {
		return vr, err
	}
	target = o.getComponentTarget(st, component.ID)
	target.GitHead = gitHead(component.Repository)
	target.SourceHash = evidenceSourceHash(finalIndex)
	target.PlanHash = currentPlanHash
	target.EvidenceRevision = finalIndex.Revision
	target.EvidenceIndexPath = artifacts.Index
	target.ImpactIndexPath = artifacts.Impact
	target.CoveragePath = artifacts.Coverage
	target.DocsHash = directoryHash(component.DocumentationRoot())
	target.PageHashes, target.ShardHashes = documentationPageHashes(component.DocumentationRoot(), componentPlan.Pages)
	target.SnapshotHash = directoryHash(component.DocumentationRoot())
	target.Status = "completed"
	if !vr.Accepted {
		target.Status = "completed-with-findings"
	}
	o.saveComponentTarget(st, component.ID, target)
	progress.complete("FIN", "Reports, graph, and state written")
	if !vr.Accepted {
		return vr, fmt.Errorf("adaptive documentation validation failed with score %d", vr.Score)
	}
	return vr, nil
}

func (o *Orchestrator) runAdaptiveSystem(ctx context.Context, st *model.RunState, components []config.ComponentConfig, options GenerateOptions) (model.ValidationResult, error) {
	root := o.Config.System.Output
	if err := o.prepareSystemWorkspace(root, components); err != nil {
		return model.ValidationResult{}, err
	}
	semanticMap := map[string]discovery.SemanticDiscovery{}
	totalDiscovery := discovery.RunMetrics{}
	for _, component := range components {
		artifacts := componentEvidenceArtifacts(o.Config.Workspace, component.ID)
		_, semantic, _, discoveryMetrics, discoveryErr := o.ensureSemanticDiscovery(ctx, component, artifacts)
		if discoveryErr != nil {
			return model.ValidationResult{}, discoveryErr
		}
		semanticMap[component.ID] = semantic
		o.recordDiscoveryMetrics(discoveryMetrics, semantic)
		totalDiscovery.Stages += discoveryMetrics.Stages
		totalDiscovery.Calls += discoveryMetrics.Calls
		totalDiscovery.Retries += discoveryMetrics.Retries
		totalDiscovery.CacheHit = totalDiscovery.CacheHit || discoveryMetrics.CacheHit
		totalDiscovery.StageMetrics = append(totalDiscovery.StageMetrics, discoveryMetrics.StageMetrics...)
		totalDiscovery.Counts.Modules += discoveryMetrics.Counts.Modules
		totalDiscovery.Counts.Domains += discoveryMetrics.Counts.Domains
		totalDiscovery.Counts.Flows += discoveryMetrics.Counts.Flows
		totalDiscovery.Counts.Concerns += discoveryMetrics.Counts.Concerns
		totalDiscovery.Counts.Ownership += discoveryMetrics.Counts.Ownership
		totalDiscovery.Counts.Relationships += discoveryMetrics.Counts.Relationships
	}
	plannerForSystem := planner.Planner{Config: o.Config, Semantic: semanticMap}
	planResult, err := plannerForSystem.Plan("", true)
	if err != nil || planResult.System == nil {
		if err == nil {
			err = fmt.Errorf("adaptive system plan is disabled")
		}
		return model.ValidationResult{}, err
	}
	plan := *planResult.System
	discovery, err := plannerForSystem.Discover("")
	if err != nil {
		return model.ValidationResult{}, err
	}
	evidencePlan := planner.ComponentPlan{ComponentID: o.Config.System.ID, Profile: "system", Pages: plan.Pages, Packs: []string{"system"}, SemanticFingerprint: plan.SemanticFingerprint}
	artifacts := systemEvidenceArtifacts(o.Config.Workspace, o.Config.System.ID)
	if err := savePlanArtifacts(evidencePlan, discovery, artifacts); err != nil {
		return model.ValidationResult{}, err
	}
	if err := writeAdaptiveSystemInstructions(root, plan); err != nil {
		return model.ValidationResult{}, err
	}
	removeUnplannedDocumentationFiles(filepath.Join(root, "openwiki"), pagesFromPlan(plan.Pages))
	if err := ensureGitRepo(root); err != nil {
		return model.ValidationResult{}, err
	}
	pages := plannedPaths(plan.Pages)
	progress := newProgressTracker(o.Out, "system", len(pages)+2)
	target := o.getSystemTarget(st)
	if target.Phases == nil {
		target.Phases = map[string]model.PhaseStatus{}
	}
	previousSourceHash := target.SourceHash
	previousDocsHash := target.DocsHash
	cachePath := evidenceCachePath(o.Config.Documentation.Evidence.CacheDirectory, o.Config.System.ID)
	maxEvidenceBytes := o.Config.Documentation.Evidence.MaxFileBytes
	index, impact, err := prepareEvidence(root, o.Config.System.ID, o.Config.Documentation.Evidence.Include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, evidencePlan, target, artifacts)
	if err != nil {
		return model.ValidationResult{}, err
	}
	o.recordEvidenceMetrics(index)
	currentPlanHash := planFingerprint(evidencePlan)
	if target.PlanHash != "" && target.PlanHash != currentPlanHash {
		forceFullImpact(evidencePlan, &impact, "documentation plan changed structurally; full-scope scan")
		if err := evidence.SaveJSON(artifacts.Impact, impact); err != nil {
			return model.ValidationResult{}, err
		}
	}
	currentSourceHash := evidenceSourceHash(index)
	currentDocsHash := directoryHash(filepath.Join(root, "openwiki"))
	target.Status = "running"
	target.GitHead = gitHead(root)
	target.SourceHash = currentSourceHash
	target.PlanHash = currentPlanHash
	target.EvidenceRevision = index.Revision
	target.EvidenceIndexPath = artifacts.Index
	target.ImpactIndexPath = artifacts.Impact
	target.CoveragePath = artifacts.Coverage
	o.saveSystemTarget(st, target)

	if options.UpdateOnly && previousSourceHash != "" && previousSourceHash == currentSourceHash && previousDocsHash != "" && previousDocsHash == currentDocsHash {
		progress.skip("UPD", "No source or documentation changes; model call skipped")
	} else if options.UpdateOnly {
		updatePages := affectedPagePaths(evidencePlan, impact)
		if currentDocsHash != previousDocsHash {
			updatePages = pages
		}
		updatePages = ensureAffectedPages(evidencePlan, &impact, updatePages)
		if err := evidence.SaveJSON(artifacts.Impact, impact); err != nil {
			return model.ValidationResult{}, err
		}
		prompt, err := prompts.RenderAdaptiveSystemUpdate(updatePages, o.Config.Documentation.Language)
		if err != nil {
			return model.ValidationResult{}, err
		}
		label := progress.start("UPD", "Adaptive system update")
		if err := o.runWithRetries(ctx, root, "update", prompt, label); err != nil {
			return model.ValidationResult{}, err
		}
		progress.complete("UPD", "Adaptive system update")
	} else {
		for _, page := range plan.Pages {
			o.recordPageMetric(options.UpdateOnly)
			key := "page:" + filepath.ToSlash(page.Path)
			status := target.Phases[key]
			if options.Resume && status.Status == "completed" {
				progress.skip(key, page.Path+" (already completed)")
				continue
			}
			status.Status = "running"
			status.Attempts++
			status.StartedAt = time.Now().UTC()
			target.Phases[key] = status
			o.saveSystemTarget(st, target)
			prompt, err := prompts.RenderAdaptiveSystemPage(page.Path, string(page.View), string(page.Kind), strings.Join(pages, "\n"), o.Config.Documentation.Language)
			if err != nil {
				return model.ValidationResult{}, err
			}
			op := "prompt"
			if page.Path == "quickstart.md" && !fileExists(filepath.Join(root, "openwiki", "quickstart.md")) {
				op = "init"
			}
			label := progress.start(key, page.Path)
			if err := o.runWithRetries(ctx, root, op, prompt, label); err != nil {
				status.Status = "failed"
				status.Error = err.Error()
				target.Phases[key] = status
				o.saveSystemTarget(st, target)
				return model.ValidationResult{}, err
			}
			status.Status = "completed"
			status.CompletedAt = time.Now().UTC()
			target.Phases[key] = status
			o.saveSystemTarget(st, target)
			progress.complete(key, page.Path)
		}
	}

	progress.start("VAL", "Validate adaptive system hierarchy and navigation")
	finalIndex, err := finalizeEvidence(root, o.Config.System.ID, o.Config.Documentation.Evidence.Include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, evidencePlan, artifacts)
	if err != nil {
		return model.ValidationResult{}, err
	}
	o.recordEvidenceMetrics(finalIndex)
	vr := o.Validator.ValidateAdaptiveSystem(ctx, filepath.Join(root, "openwiki"), plan)
	vr.Findings = append(vr.Findings, validation.ValidateEvidenceBacked(filepath.Join(root, "openwiki"), finalIndex)...)
	vr = validation.Recalculate(vr, o.Config.Documentation.MinimumQualityScore)
	for round := 1; !vr.Accepted && round <= o.Config.Execution.MaxRepairRounds; round++ {
		progress.note("VAL", fmt.Sprintf("Repair round %d/%d for %d findings", round, o.Config.Execution.MaxRepairRounds, len(vr.Findings)))
		repairPrompt, repairErr := prompts.Render("prompts/common/repair.md", o.Config.Documentation.Language, o.Config.System.ID, map[string]string{
			"FINDINGS":        validation.FindingsText(vr.Findings),
			"PROFILE_NAME":    "Adaptive system",
			"COMPONENT_TYPE":  "whole-system",
			"SCOPE":           "aggregation workspace",
			"CANONICAL_FILES": strings.Join(pages, "\n"),
		})
		if repairErr != nil {
			return vr, repairErr
		}
		if repairErr = o.runWithRetries(ctx, root, "prompt", repairPrompt, fmt.Sprintf("system/adaptive-repair-%d", round)); repairErr != nil {
			return vr, repairErr
		}
		finalIndex, err = finalizeEvidence(root, o.Config.System.ID, o.Config.Documentation.Evidence.Include, o.Config.Documentation.Evidence.Exclude, cachePath, maxEvidenceBytes, evidencePlan, artifacts)
		if err != nil {
			return vr, err
		}
		vr = o.Validator.ValidateAdaptiveSystem(ctx, filepath.Join(root, "openwiki"), plan)
		vr.Findings = append(vr.Findings, validation.ValidateEvidenceBacked(filepath.Join(root, "openwiki"), finalIndex)...)
		vr = validation.Recalculate(vr, o.Config.Documentation.MinimumQualityScore)
	}
	progress.complete("VAL", fmt.Sprintf("Adaptive system validation score=%d accepted=%t", vr.Score, vr.Accepted))
	progress.start("FIN", "Write system report, export graph, and checkpoint state")
	_ = validation.WriteResult(filepath.Join(o.Config.Workspace, ".wikiforge", "validation", "system.json"), vr)
	if err := graph.ExportWithEvidence(o.Config.System.ID, filepath.Join(root, "openwiki"), filepath.Join(o.Config.Workspace, ".wikiforge", "graph", "system"), finalIndex); err != nil {
		return vr, err
	}
	target = o.getSystemTarget(st)
	target.SemanticFingerprint = plan.SemanticFingerprint
	target.DiscoveryMode = "aggregated"
	target.DiscoveryCalls = totalDiscovery.Calls
	target.DiscoveryCacheHit = totalDiscovery.CacheHit
	target.DiscoveryCounts = totalDiscovery.Counts
	target.DiscoveryStageMetrics = totalDiscovery.StageMetrics
	target.GitHead = gitHead(root)
	target.SourceHash = evidenceSourceHash(finalIndex)
	target.PlanHash = currentPlanHash
	target.EvidenceRevision = finalIndex.Revision
	target.EvidenceIndexPath = artifacts.Index
	target.ImpactIndexPath = artifacts.Impact
	target.CoveragePath = artifacts.Coverage
	target.DocsHash = directoryHash(filepath.Join(root, "openwiki"))
	target.PageHashes, target.ShardHashes = documentationPageHashes(filepath.Join(root, "openwiki"), plan.Pages)
	target.SnapshotHash = directoryHash(filepath.Join(root, "sources"))
	target.Status = "completed"
	if !vr.Accepted {
		target.Status = "completed-with-findings"
	}
	o.saveSystemTarget(st, target)
	progress.complete("FIN", "Reports, graph, and state written")
	if !vr.Accepted {
		return vr, fmt.Errorf("adaptive system validation failed with score %d", vr.Score)
	}
	return vr, nil
}

func documentationPageHashes(root string, pages []planner.PlannedPage) (map[string]string, map[string]string) {
	pageHashes := map[string]string{}
	shardHashes := map[string]string{}
	for _, page := range pages {
		path := filepath.Join(root, filepath.FromSlash(page.Path))
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])
		pageHashes[filepath.ToSlash(page.Path)] = hash
		if page.Kind == planner.PageShard {
			shardHashes[filepath.ToSlash(page.Path)] = hash
		}
	}
	return pageHashes, shardHashes
}

func writeAdaptiveInstructions(component config.ComponentConfig, profile prompts.Profile, plan planner.ComponentPlan, language string) error {
	pages := plannedPaths(plan.Pages)
	content, err := prompts.RenderAdaptiveInstructions(profile, component, pages, joinDocumentationViews(plan.Views), language)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(component.DocumentationRoot(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(component.DocumentationRoot(), "INSTRUCTIONS.md"), []byte(content), 0o644)
}

func writeAdaptiveSystemInstructions(root string, plan planner.SystemPlan) error {
	path := filepath.Join(root, "openwiki", "INSTRUCTIONS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := "# WikiForge adaptive system documentation\n\nThe generated system wiki follows hierarchical navigation.\n\nCanonical pages:\n- " + strings.Join(plannedPaths(plan.Pages), "\n- ") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

func plannedPaths(pages []planner.PlannedPage) []string {
	paths := make([]string, 0, len(pages))
	for _, page := range pages {
		paths = append(paths, filepath.ToSlash(page.Path))
	}
	sort.Strings(paths)
	return paths
}

func pagesFromPlan(pages []planner.PlannedPage) []string {
	return plannedPaths(pages)
}

func removeUnplannedDocumentationFiles(root string, planned []string) {
	set := map[string]bool{}
	for _, path := range planned {
		set[filepath.ToSlash(filepath.Clean(path))] = true
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() || entry.Name() == "INSTRUCTIONS.md" || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if !set[filepath.ToSlash(filepath.Clean(rel))] {
			_ = os.Remove(path)
		}
		return nil
	})
}

func adaptiveUnitContext(units []planner.DocumentationUnit, id string) string {
	for _, unit := range units {
		if unit.ID != id {
			continue
		}
		return fmt.Sprintf("Unit %s (%s)\nDomain: %s\nSubdomain: %s\nBounded context: %s\nSource roots: %s\nEvidence roots: %s\nRelated units: %s\nReason: %s", unit.ID, unit.Kind, unit.Domain, unit.Subdomain, unit.BoundedContext, strings.Join(unit.SourceRoots, ", "), strings.Join(unit.EvidenceRoots, ", "), strings.Join(unit.RelatedUnits, ", "), unit.Reason)
	}
	return "This page is a cross-cutting view; use only repository evidence in the configured scope."
}

func evidenceRootsForPlan(include []string, plan planner.ComponentPlan) []string {
	seen := map[string]bool{}
	roots := make([]string, 0, len(include))
	for _, path := range include {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path != "" && !seen[path] {
			seen[path] = true
			roots = append(roots, path)
		}
	}
	for _, unit := range plan.Units {
		for _, path := range unit.EvidenceRoots {
			path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
			if path != "" && !seen[path] {
				seen[path] = true
				roots = append(roots, path+"/**")
			}
		}
	}
	sort.Strings(roots)
	return roots
}

func joinDocumentationViews(views []planner.DocumentationView) string {
	parts := make([]string, 0, len(views))
	for _, view := range views {
		parts = append(parts, string(view))
	}
	return strings.Join(parts, ", ")
}
