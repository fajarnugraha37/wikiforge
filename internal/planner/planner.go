package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
)

type packPage struct{ path, view, kind string }

var packPages = map[string]packPage{
	"api":                      {"catalogs/interfaces/index.md", "catalog", "collection"},
	"cache":                    {"platform/caching.md", "platform", "single"},
	"concurrency":              {"platform/concurrency-and-context.md", "platform", "single"},
	"configuration":            {"catalogs/configuration/index.md", "catalog", "collection"},
	"container-runtime":        {"platform/containerization.md", "platform", "single"},
	"cryptography":             {"platform/cryptography.md", "platform", "single"},
	"data-access":              {"catalogs/data-access/index.md", "catalog", "collection"},
	"database":                 {"catalogs/data/index.md", "catalog", "collection"},
	"distributed-coordination": {"platform/distributed-coordination.md", "platform", "single"},
	"domain":                   {"domains/index.md", "domain", "index"},
	"engineering":              {"engineering/standards.md", "engineering", "single"},
	"files":                    {"catalogs/files/index.md", "catalog", "collection"},
	"jobs":                     {"catalogs/jobs/index.md", "catalog", "collection"},
	"messaging":                {"catalogs/events/index.md", "catalog", "collection"},
	"migrations":               {"catalogs/migrations/index.md", "catalog", "collection"},
	"rate-limit":               {"platform/rate-limiting.md", "platform", "single"},
	"runtime":                  {"runtime-and-operations.md", "component", "single"},
	"security":                 {"platform/security-and-identity.md", "platform", "single"},
	"telemetry":                {"platform/telemetry.md", "platform", "single"},
	"workflow":                 {"flows/index.md", "flow", "index"},
}

func Build(cfg config.Config, component config.ComponentConfig, manifest model.DiscoveryManifest) model.DocumentationPlan {
	plan := model.DocumentationPlan{
		SchemaVersion:      1,
		ComponentID:        component.ID,
		Profile:            component.Profile,
		Views:              append([]string(nil), cfg.Documentation.Views...),
		Units:              append([]model.DocumentationUnit(nil), manifest.Units...),
		ShardBy:            append([]string(nil), cfg.Documentation.Catalogs.ShardBy...),
		MaximumRowsPerPage: cfg.Documentation.Catalogs.MaximumRowsPerPage,
	}
	selected := map[string]bool{}
	for _, pack := range config.DefaultPacksForProfile(component.Profile) {
		selected[pack] = true
	}
	for _, pack := range component.Packs {
		selected[pack] = true
	}
	for _, pack := range manifest.Packs {
		selected[pack] = true
	}
	plan.SelectedPacks = mapKeys(selected)

	pagePaths := map[string]model.PlanPage{}
	add := func(page model.PlanPage) bool {
		if page.Path == "" {
			return false
		}
		if existing, found := pagePaths[page.Path]; found {
			if existing.ID != page.ID {
				plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: page.ID, Action: "defer", Reason: fmt.Sprintf("Output path %s conflicts with planned page %s.", page.Path, existing.ID)})
			}
			return false
		}
		pagePaths[page.Path] = page
		plan.Pages = append(plan.Pages, page)
		return true
	}
	add(model.PlanPage{ID: component.ID + ":quickstart", Path: "quickstart.md", View: "component", Kind: "index", Reason: "Every component requires one bounded entry point, independent of optional detailed views."})
	if cfg.ViewEnabled("component") {
		add(model.PlanPage{ID: component.ID + ":overview", Path: "components/" + component.ID + "/index.md", View: "component", Kind: "index", Reason: "Component view is enabled."})
	}
	for _, pack := range plan.SelectedPacks {
		contract, ok := packPages[pack]
		if !ok {
			plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: pack, Action: "defer", Reason: "No Phase 1 canonical page mapping is registered."})
			continue
		}
		if !cfg.ViewEnabled(contract.view) {
			plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: pack, Action: "defer", Reason: fmt.Sprintf("Required %s view is disabled.", contract.view)})
			continue
		}
		path := contract.path
		if pack == "runtime" {
			path = "components/" + component.ID + "/" + path
		}
		page := model.PlanPage{ID: component.ID + ":pack:" + pack, Path: path, View: contract.view, Pack: pack, Kind: contract.kind, Reason: packReason(component, manifest, pack)}
		if contract.kind == "collection" {
			page.ShardBy = append([]string(nil), plan.ShardBy...)
			page.MaximumRowsPerPage = plan.MaximumRowsPerPage
		}
		if add(page) {
			plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: pack, Action: "include", Reason: packReason(component, manifest, pack)})
		}
	}
	for _, unit := range plan.Units {
		view := viewForUnit(unit.Kind)
		if !cfg.ViewEnabled(view) {
			plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: unit.ID, Action: "defer", Reason: fmt.Sprintf("Documentation unit requires disabled %s view.", view)})
			continue
		}
		path := unitPagePath(unit)
		add(model.PlanPage{ID: component.ID + ":unit:" + unit.ID, Path: path, View: view, UnitID: unit.ID, Kind: "unit", Reason: fmt.Sprintf("%s documentation unit (%s).", unit.Origin, unit.Kind)})
	}
	// Materialize hierarchical view indexes after leaf selection so navigation can
	// remain shallow without forcing empty areas.
	viewChildren := map[string]bool{}
	for _, page := range plan.Pages {
		viewChildren[page.View] = true
	}
	if cfg.ViewEnabled("component") && viewChildren["component"] {
		add(model.PlanPage{ID: component.ID + ":view:components", Path: "components/index.md", View: "component", Kind: "index", Reason: "Component view contains planned child pages."})
	}
	if cfg.ViewEnabled("domain") && (viewChildren["domain"] || hasUnitKind(plan.Units, "domain", "subdomain", "bounded-context")) {
		add(model.PlanPage{ID: component.ID + ":view:domains", Path: "domains/index.md", View: "domain", Kind: "index", Reason: "Domain view contains planned units or capability pages."})
	}
	if cfg.ViewEnabled("flow") && (viewChildren["flow"] || hasUnitKind(plan.Units, "flow")) {
		add(model.PlanPage{ID: component.ID + ":view:flows", Path: "flows/index.md", View: "flow", Kind: "index", Reason: "Flow view contains planned units or workflow pages."})
	}
	if cfg.ViewEnabled("catalog") && viewChildren["catalog"] {
		add(model.PlanPage{ID: component.ID + ":view:catalogs", Path: "catalogs/index.md", View: "catalog", Kind: "index", Reason: "Catalog view contains one or more typed collections."})
	}
	if cfg.ViewEnabled("platform") && viewChildren["platform"] {
		add(model.PlanPage{ID: component.ID + ":view:platform", Path: "platform/index.md", View: "platform", Kind: "index", Reason: "Platform view contains shared technical mechanisms."})
	}
	if cfg.ViewEnabled("engineering") && viewChildren["engineering"] {
		add(model.PlanPage{ID: component.ID + ":view:engineering", Path: "engineering/index.md", View: "engineering", Kind: "index", Reason: "Engineering view contains standards or contribution guidance."})
	}
	if cfg.ViewEnabled("operations") && operational(plan.SelectedPacks) {
		add(model.PlanPage{ID: component.ID + ":view:operations", Path: "operations/index.md", View: "operations", Kind: "index", Reason: "Selected runtime capabilities have operational consequences."})
	}

	for _, pack := range config.SupportedCapabilityPacks() {
		if !selected[pack] {
			plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: pack, Action: "skip", Reason: "Neither configured, profile-required, nor discovered from source evidence."})
		}
	}
	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].Path < plan.Pages[j].Path })
	sort.Slice(plan.Decisions, func(i, j int) bool {
		if plan.Decisions[i].Subject == plan.Decisions[j].Subject {
			return plan.Decisions[i].Action < plan.Decisions[j].Action
		}
		return plan.Decisions[i].Subject < plan.Decisions[j].Subject
	})
	return plan
}

func Explain(plan model.DocumentationPlan) []string {
	lines := []string{fmt.Sprintf("adaptive plan component=%s profile=%s packs=%d units=%d pages=%d", plan.ComponentID, plan.Profile, len(plan.SelectedPacks), len(plan.Units), len(plan.Pages))}
	if len(plan.SelectedPacks) > 0 {
		lines = append(lines, "  packs: "+strings.Join(plan.SelectedPacks, ", "))
	}
	for _, page := range plan.Pages {
		policy := ""
		if len(page.ShardBy) > 0 {
			policy = fmt.Sprintf(" shardBy=%s maxRows=%d", strings.Join(page.ShardBy, ","), page.MaximumRowsPerPage)
		}
		lines = append(lines, fmt.Sprintf("  include %-45s view=%-11s kind=%-10s%s reason=%s", page.Path, page.View, page.Kind, policy, page.Reason))
	}
	for _, decision := range plan.Decisions {
		if decision.Action != "include" {
			lines = append(lines, fmt.Sprintf("  %-7s %-24s %s", decision.Action, decision.Subject, decision.Reason))
		}
	}
	return lines
}

func packReason(component config.ComponentConfig, manifest model.DiscoveryManifest, pack string) string {
	for _, explicit := range component.Packs {
		if explicit == pack {
			return "Explicitly configured capability pack."
		}
	}
	for _, evidence := range manifest.Evidence {
		if evidence.Pack == pack {
			return fmt.Sprintf("Discovered from %d evidence file(s).", evidence.Count)
		}
	}
	for _, candidate := range config.DefaultPacksForProfile(component.Profile) {
		if candidate == pack {
			return "Required by the base documentation profile."
		}
	}
	return "Selected by normalized planning inputs."
}

func viewForUnit(kind string) string {
	switch kind {
	case "flow":
		return "flow"
	case "platform":
		return "platform"
	case "catalog":
		return "catalog"
	case "component", "module":
		return "component"
	default:
		return "domain"
	}
}
func unitPagePath(unit model.DocumentationUnit) string {
	path := strings.TrimSuffix(unit.OutputPath, "/")
	if path == "" {
		path = defaultUnitOutput(unit)
	}
	if unit.Kind == "flow" {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		return path
	}
	if strings.HasSuffix(path, ".md") {
		return path
	}
	return path + "/index.md"
}

func defaultUnitOutput(unit model.DocumentationUnit) string {
	switch unit.Kind {
	case "flow":
		return "flows/" + unit.ID
	case "module":
		return "components/" + unit.ComponentID + "/modules/" + unit.ID
	case "component":
		return "components/" + unit.ID
	case "platform":
		return "platform/" + unit.ID
	case "catalog":
		return "catalogs/" + unit.ID
	default:
		return "domains/" + unit.ID
	}
}
func hasUnitKind(units []model.DocumentationUnit, kinds ...string) bool {
	wanted := map[string]bool{}
	for _, kind := range kinds {
		wanted[kind] = true
	}
	for _, unit := range units {
		if wanted[unit.Kind] {
			return true
		}
	}
	return false
}

func operational(packs []string) bool {
	wanted := map[string]bool{"container-runtime": true, "jobs": true, "messaging": true, "runtime": true, "telemetry": true, "workflow": true}
	for _, pack := range packs {
		if wanted[pack] {
			return true
		}
	}
	return false
}

func containsPack(packs []string, wanted string) bool {
	for _, pack := range packs {
		if pack == wanted {
			return true
		}
	}
	return false
}

func anyPackInView(packs []string, view string) bool {
	for _, pack := range packs {
		if page, ok := packPages[pack]; ok && page.view == view {
			return true
		}
	}
	return false
}

func mapKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func BuildSystem(cfg config.Config, componentPlans []model.DocumentationPlan) model.DocumentationPlan {
	plan := model.DocumentationPlan{
		SchemaVersion:      1,
		ComponentID:        cfg.System.ID,
		Profile:            "system",
		Views:              append([]string(nil), cfg.Documentation.Views...),
		ShardBy:            append([]string(nil), cfg.Documentation.Catalogs.ShardBy...),
		MaximumRowsPerPage: cfg.Documentation.Catalogs.MaximumRowsPerPage,
	}
	packs := map[string]bool{}
	units := map[string]model.DocumentationUnit{}
	for _, componentPlan := range componentPlans {
		for _, pack := range componentPlan.SelectedPacks {
			packs[pack] = true
		}
		for _, unit := range componentPlan.Units {
			units[unit.ComponentID+":"+unit.ID] = unit
		}
	}
	plan.SelectedPacks = mapKeys(packs)
	for _, unit := range units {
		plan.Units = append(plan.Units, unit)
	}
	sort.Slice(plan.Units, func(i, j int) bool {
		if plan.Units[i].ComponentID == plan.Units[j].ComponentID {
			return plan.Units[i].ID < plan.Units[j].ID
		}
		return plan.Units[i].ComponentID < plan.Units[j].ComponentID
	})
	if !cfg.ViewEnabled("system") {
		plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: "system", Action: "defer", Reason: "System view is disabled."})
		return plan
	}
	pages := []model.PlanPage{
		{ID: cfg.System.ID + ":quickstart", Path: "quickstart.md", View: "system", Kind: "index", Reason: "Whole-system documentation requires one bounded entry point."},
		{ID: cfg.System.ID + ":index", Path: "system/index.md", View: "system", Kind: "index", Reason: "System view is enabled."},
		{ID: cfg.System.ID + ":overview", Path: "system/overview.md", View: "system", Kind: "single", Reason: "Provides the system boundary and synthesis context."},
		{ID: cfg.System.ID + ":components", Path: "system/component-landscape.md", View: "system", Kind: "single", Reason: "Component plans are available for aggregation."},
		{ID: cfg.System.ID + ":runtime", Path: "system/runtime-topology.md", View: "system", Kind: "single", Reason: "Runtime and dependency packs require a system topology view."},
	}
	if len(plan.Units) > 0 {
		pages = append(pages,
			model.PlanPage{ID: cfg.System.ID + ":capabilities", Path: "system/capability-map.md", View: "system", Kind: "single", Reason: "Documentation units expose capabilities across components."},
			model.PlanPage{ID: cfg.System.ID + ":domains", Path: "system/domain-map.md", View: "system", Kind: "single", Reason: "Domain and flow units require cross-component mapping."},
		)
	}
	if cfg.ViewEnabled("component") {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:components", Path: "components/index.md", View: "component", Kind: "index", Reason: "Aggregated component plans are available."})
	}
	if cfg.ViewEnabled("domain") && hasUnitKind(plan.Units, "domain", "subdomain", "bounded-context") {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:domains", Path: "domains/index.md", View: "domain", Kind: "index", Reason: "Aggregated domain units are available."})
	}
	if cfg.ViewEnabled("flow") && (hasUnitKind(plan.Units, "flow") || containsPack(plan.SelectedPacks, "workflow")) {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:flows", Path: "flows/index.md", View: "flow", Kind: "index", Reason: "Aggregated flow evidence is available."})
	}
	if cfg.ViewEnabled("catalog") && anyPackInView(plan.SelectedPacks, "catalog") {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:catalogs", Path: "catalogs/index.md", View: "catalog", Kind: "index", Reason: "Aggregated typed catalogs are required."})
	}
	if cfg.ViewEnabled("platform") && anyPackInView(plan.SelectedPacks, "platform") {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:platform", Path: "platform/index.md", View: "platform", Kind: "index", Reason: "Aggregated platform mechanisms are required."})
	}
	if cfg.ViewEnabled("engineering") && containsPack(plan.SelectedPacks, "engineering") {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:engineering", Path: "engineering/index.md", View: "engineering", Kind: "index", Reason: "Aggregated engineering guidance is available."})
	}
	if cfg.ViewEnabled("operations") && operational(plan.SelectedPacks) {
		pages = append(pages, model.PlanPage{ID: cfg.System.ID + ":view:operations", Path: "operations/index.md", View: "operations", Kind: "index", Reason: "Aggregated runtime capabilities have operational consequences."})
	}
	plan.Pages = pages
	plan.Decisions = append(plan.Decisions, model.PlanDecision{Subject: "system", Action: "include", Reason: fmt.Sprintf("Aggregates %d component plan(s).", len(componentPlans))})
	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].Path < plan.Pages[j].Path })
	return plan
}
