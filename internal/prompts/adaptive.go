package prompts

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
)

const openWikiRepositoryRoot = "repository root (OpenWiki virtual filesystem path: /)"

// AdaptivePageContract supplies a small, typed contract for pages selected by
// the adaptive planner. It deliberately avoids forcing a diagram onto index
// pages while keeping narrative and catalog pages evidence-oriented.
func AdaptivePageContract(path, kind string) model.PageContract {
	contract := model.PageContract{
		Path:             filepath.ToSlash(path),
		Kind:             model.PageKind(kind),
		RequiredHeadings: []string{"Purpose", "Knowledge Gaps", "Source References"},
		RequiredDiagram:  "flowchart",
	}
	if kind == string(model.PageIndex) || kind == string(model.PageCollection) {
		contract.RequiredHeadings = []string{"Navigation", "Knowledge Gaps", "Source References"}
		contract.RequiredDiagram = ""
	}
	if strings.HasPrefix(filepath.ToSlash(path), "catalogs/") && kind != string(model.PageIndex) {
		contract.RequiredTableHeader = "| ID | Name | Direction | Owner | Evidence |"
		contract.RequiredHeadings = []string{"Catalog Scope", "Catalog Entries", "Knowledge Gaps", "Source References"}
	}
	return contract
}

func RenderAdaptivePage(path, view, kind, owner, profile string, component config.ComponentConfig, unitContext, views, packs, language string) (string, error) {
	objective, headings, diagram := adaptivePageShape(path, view, kind)
	profileInfo, err := GetProfile(profile)
	if err != nil {
		return "", err
	}
	return Render("prompts/component/phase.md", language, component.ID, map[string]string{
		"PROFILE_ID":        profileInfo.ID,
		"PROFILE_NAME":      profileInfo.DisplayName,
		"COMPONENT_TYPE":    component.Type,
		"REPOSITORY":        openWikiRepositoryRoot,
		"SCOPE":             displayScope(component.Scope),
		"OUTPUT_FILE":       filepath.ToSlash(path),
		"OBJECTIVE":         objective,
		"REQUIRED_HEADINGS": headingsText(headings),
		"DIAGRAM_CONTRACT":  diagramContract(diagram),
		"GUIDANCE":          adaptiveGuidance(view),
		"UNIT_CONTEXT":      unitContext,
		"ADAPTIVE_VIEWS":    views,
		"ADAPTIVE_PACKS":    packs,
		"ADAPTIVE_OWNER":    owner,
		"NAVIGATION_RULE":   "Link only to direct children of this page. The root quickstart links view indexes; indexes link unit or collection indexes; collections link shards.",
	})
}

func RenderAdaptiveInstructions(profile Profile, component config.ComponentConfig, pages []string, views, language string) (string, error) {
	return RenderTemplateValues("templates/instructions.md", language, component.ID, map[string]string{
		"PROFILE_ID":          profile.ID,
		"PROFILE_NAME":        profile.DisplayName,
		"PROFILE_DESCRIPTION": profile.Description,
		"COMPONENT_TYPE":      component.Type,
		"REPOSITORY":          openWikiRepositoryRoot,
		"SCOPE":               displayScope(component.Scope),
		"CANONICAL_FILES":     adaptiveFilesText(pages),
		"GUIDANCE":            adaptiveGuidance("component"),
		"ADAPTIVE_VIEWS":      views,
	})
}

func RenderAdaptiveUpdate(profile Profile, component config.ComponentConfig, pages []string, views, language string) (string, error) {
	return Render("prompts/component/update.md", language, component.ID, map[string]string{
		"PROFILE_ID":          profile.ID,
		"PROFILE_NAME":        profile.DisplayName,
		"PROFILE_DESCRIPTION": profile.Description,
		"TARGET_NOUN":         profile.TargetNoun,
		"COMPONENT_TYPE":      component.Type,
		"REPOSITORY":          openWikiRepositoryRoot,
		"SCOPE":               displayScope(component.Scope),
		"CANONICAL_FILES":     adaptiveFilesText(pages),
		"GUIDANCE":            adaptiveGuidance("component"),
		"ADAPTIVE_VIEWS":      views,
	})
}

func RenderAdaptiveSystemPage(path, view, kind, canonicalPages, language string) (string, error) {
	contract := AdaptivePageContract(path, kind)
	objective, headings, diagram := adaptivePageShape(path, view, kind)
	contract.Objective = objective
	contract.RequiredHeadings = headings
	contract.RequiredDiagram = diagram
	values := map[string]string{
		"OUTPUT_FILE":       filepath.ToSlash(path),
		"OBJECTIVE":         objective,
		"REQUIRED_HEADINGS": headingsText(headings),
		"DIAGRAM_CONTRACT":  diagramContract(diagram),
		"CANONICAL_FILES":   adaptiveFilesText(strings.Split(canonicalPages, "\n")),
		"COMPONENT_TYPE":    "whole-system",
		"PROFILE_NAME":      "Adaptive system",
		"PROFILE_ID":        "adaptive-system",
		"REPOSITORY":        "system aggregation workspace",
		"SCOPE":             "sources/",
		"GUIDANCE":          "Use only facts supported by the aggregated component documentation and system facts. Do not duplicate component-level canonical detail.",
		"UNIT_CONTEXT":      "System-level view",
		"ADAPTIVE_VIEWS":    "system",
		"ADAPTIVE_PACKS":    "system aggregation",
		"ADAPTIVE_OWNER":    "system",
		"NAVIGATION_RULE":   "Link only to direct children of this page; keep root navigation shallow.",
	}
	return Render("prompts/system/adaptive-page.md", language, "system", values)
}

func RenderAdaptiveSystemUpdate(pages []string, language string) (string, error) {
	return Render("prompts/system/adaptive-page.md", language, "system", map[string]string{
		"OUTPUT_FILE":       "adaptive system wiki",
		"OBJECTIVE":         "Refresh the adaptive system wiki while preserving hierarchical navigation and accurate existing content.",
		"REQUIRED_HEADINGS": "",
		"DIAGRAM_CONTRACT":  "Use diagrams only when supported by aggregated evidence.",
		"CANONICAL_FILES":   adaptiveFilesText(pages),
		"COMPONENT_TYPE":    "whole-system",
		"PROFILE_NAME":      "Adaptive system",
		"PROFILE_ID":        "adaptive-system",
		"REPOSITORY":        "system aggregation workspace",
		"SCOPE":             "sources/",
		"GUIDANCE":          "Refresh only evidence-backed content and preserve the hierarchy.",
		"UNIT_CONTEXT":      "System-level view",
		"ADAPTIVE_VIEWS":    "system",
		"ADAPTIVE_PACKS":    "system aggregation",
		"ADAPTIVE_OWNER":    "system",
		"NAVIGATION_RULE":   "Keep root navigation limited to view indexes.",
	})
}

func adaptivePageShape(path, view, kind string) (string, []string, string) {
	base := filepath.ToSlash(path)
	if kind == string(model.PageIndex) || kind == string(model.PageCollection) {
		return "Provide a navigable index or typed collection for the applicable adaptive documentation area.", []string{"Navigation", "Knowledge Gaps", "Source References"}, ""
	}
	switch {
	case view == "flow":
		return "Document the selected flow from trigger through completion and failure handling.", []string{"Trigger", "Actor", "Preconditions", "Synchronous Steps", "Asynchronous Steps", "State Changes", "Transaction Boundaries", "Events", "Identity Propagation", "Timeouts and Retries", "Failure Branch", "Compensation", "Telemetry Correlation", "Knowledge Gaps", "Source References"}, "sequenceDiagram"
	case view == "domain":
		return "Document the domain unit, its concepts, rules, interfaces, events, and participating flows.", []string{"Purpose", "Business Capability", "Concepts and Data", "Rules and Invariants", "Workflows and State", "Interfaces and Events", "Component Mapping", "Participating Flows", "Knowledge Gaps", "Source References"}, "flowchart"
	case strings.HasPrefix(base, "components/"):
		switch filepath.Base(base) {
		case "architecture.md":
			return "Document component architecture, boundaries, dependencies, entry points, and runtime flow.", []string{"Purpose", "Boundaries", "Major Parts", "Dependency Direction", "Entry Points", "Runtime Flow", "Failure Boundaries", "Knowledge Gaps", "Source References"}, "sequenceDiagram"
		case "contracts.md":
			return "Document the component's externally meaningful contracts and compatibility behaviour.", []string{"Purpose", "Inbound Contracts", "Outbound Contracts", "Validation and Errors", "Identity and Authorization", "Idempotency and Retry", "Compatibility", "Knowledge Gaps", "Source References"}, "sequenceDiagram"
		case "data-and-consistency.md":
			return "Document owned data, persistence boundaries, consistency, transactions, and repair behaviour.", []string{"Purpose", "Data Ownership", "Persistence Boundaries", "Transaction Boundaries", "Consistency", "Concurrency and Deduplication", "Migration and Recovery", "Knowledge Gaps", "Source References"}, "erDiagram"
		case "runtime-and-operations.md":
			return "Document runtime, deployment, observability, failure, and recovery concerns supported by evidence.", []string{"Purpose", "Runtime Shape", "Configuration", "Operational Signals", "Failure Modes", "Timeouts and Retries", "Recovery", "Knowledge Gaps", "Source References"}, "flowchart"
		default:
			return "Document the component purpose, ownership, boundaries, and safe change context.", []string{"Purpose", "Ownership and Boundaries", "Responsibilities", "Documentation Map", "Knowledge Gaps", "Source References"}, "flowchart"
		}
	default:
		return fmt.Sprintf("Document the evidence-backed %s concern represented by this adaptive page.", view), []string{"Purpose", "Evidence Scope", "Observed Behaviour", "Failure and Change Risks", "Knowledge Gaps", "Source References"}, "flowchart"
	}
}

func adaptiveGuidance(view string) string {
	return fmt.Sprintf("This is the %s view in a hierarchical adaptive documentation plan. Do not duplicate canonical detail owned by another view. Report unknowns as knowledge gaps rather than inventing content.", view)
}

func adaptiveFilesText(pages []string) string {
	var b strings.Builder
	for _, page := range pages {
		fmt.Fprintf(&b, "- openwiki/%s\n", filepath.ToSlash(page))
	}
	return strings.TrimRight(b.String(), "\n")
}
