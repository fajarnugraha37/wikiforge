package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/config"
	"github.com/fajarnugraha37/wikiforge/internal/model"
)

type DocumentationView string

const (
	ViewSystem      DocumentationView = "system"
	ViewDomain      DocumentationView = "domain"
	ViewComponent   DocumentationView = "component"
	ViewFlow        DocumentationView = "flow"
	ViewCatalog     DocumentationView = "catalog"
	ViewPlatform    DocumentationView = "platform"
	ViewEngineering DocumentationView = "engineering"
	ViewOperations  DocumentationView = "operations"
)

type DocumentationUnitKind string

const (
	UnitComponent   DocumentationUnitKind = "component"
	UnitDomain      DocumentationUnitKind = "domain"
	UnitFlow        DocumentationUnitKind = "flow"
	UnitCatalog     DocumentationUnitKind = "catalog"
	UnitPlatform    DocumentationUnitKind = "platform"
	UnitEngineering DocumentationUnitKind = "engineering"
	UnitOperations  DocumentationUnitKind = "operations"
)

type PageKind = model.PageKind

const (
	PageSingle     = model.PageSingle
	PageIndex      = model.PageIndex
	PageCollection = model.PageCollection
	PageShard      = model.PageShard
)

type DocumentationUnit struct {
	ID             string                `json:"id"`
	ComponentID    string                `json:"componentId"`
	Kind           DocumentationUnitKind `json:"kind"`
	SourceRoots    []string              `json:"sourceRoots,omitempty"`
	RelatedUnits   []string              `json:"relatedUnits,omitempty"`
	OutputPath     string                `json:"output"`
	Owners         []string              `json:"owners,omitempty"`
	Capabilities   []string              `json:"capabilities,omitempty"`
	Criticality    string                `json:"criticality,omitempty"`
	Domain         string                `json:"domain,omitempty"`
	Subdomain      string                `json:"subdomain,omitempty"`
	BoundedContext string                `json:"boundedContext,omitempty"`
	View           string                `json:"view,omitempty"`
	EvidenceRoots  []string              `json:"evidenceRoots,omitempty"`
	ShardBy        []string              `json:"shardBy,omitempty"`
	MaximumRows    int                   `json:"maximumRows,omitempty"`
	Explicit       bool                  `json:"explicit"`
	Reason         string                `json:"reason"`
}

type DiscoveredComponent struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
	Profile            string   `json:"profile"`
	Repository         string   `json:"repository"`
	Scope              string   `json:"scope,omitempty"`
	Owners             []string `json:"owners,omitempty"`
	Capabilities       []string `json:"capabilities,omitempty"`
	Packs              []string `json:"packs,omitempty"`
	DocumentationUnits []string `json:"documentationUnits,omitempty"`
}

type DiscoveryManifest struct {
	Components                  []DiscoveredComponent `json:"components"`
	CandidateDocumentationUnits []DocumentationUnit   `json:"candidateDocumentationUnits"`
	Unknowns                    []string              `json:"unknowns,omitempty"`
}

type PlannedPage struct {
	Path      string             `json:"path"`
	View      DocumentationView  `json:"view"`
	Kind      PageKind           `json:"kind"`
	OwnerUnit string             `json:"ownerUnit,omitempty"`
	Reason    string             `json:"reason"`
	Contract  model.PageContract `json:"contract"`
}

type PlanDecision struct {
	Target string `json:"target"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type ComponentPlan struct {
	ComponentID string              `json:"componentId"`
	Profile     string              `json:"profile"`
	Views       []DocumentationView `json:"views"`
	Packs       []string            `json:"packs"`
	Units       []DocumentationUnit `json:"units"`
	Pages       []PlannedPage       `json:"pages"`
	Decisions   []PlanDecision      `json:"decisions"`
}

type SystemPlan struct {
	Views     []DocumentationView `json:"views"`
	Pages     []PlannedPage       `json:"pages"`
	Decisions []PlanDecision      `json:"decisions"`
}

type PlanResult struct {
	Components []ComponentPlan `json:"components"`
	System     *SystemPlan     `json:"system,omitempty"`
}

type Planner struct {
	Config config.Config
}

func (p Planner) Discover(componentID string) (DiscoveryManifest, error) {
	components, err := p.selectedComponents(componentID)
	if err != nil {
		return DiscoveryManifest{}, err
	}
	explicitByComponent := map[string][]config.DocumentationUnitConfig{}
	for _, unit := range p.Config.DocumentationUnits {
		explicitByComponent[unit.Component] = append(explicitByComponent[unit.Component], unit)
	}
	manifest := DiscoveryManifest{}
	for _, component := range components {
		units, unknowns := discoverUnits(component, explicitByComponent[component.ID])
		unitIDs := make([]string, 0, len(units))
		for _, unit := range units {
			manifest.CandidateDocumentationUnits = append(manifest.CandidateDocumentationUnits, unit)
			unitIDs = append(unitIDs, unit.ID)
		}
		sort.Strings(unitIDs)
		manifest.Components = append(manifest.Components, DiscoveredComponent{
			ID:                 component.ID,
			Type:               component.Type,
			Profile:            component.Profile,
			Repository:         component.Repository,
			Scope:              component.Scope,
			Owners:             append([]string(nil), component.Owners...),
			Capabilities:       append([]string(nil), component.Capabilities...),
			Packs:              packsForComponent(component),
			DocumentationUnits: unitIDs,
		})
		manifest.Unknowns = append(manifest.Unknowns, unknowns...)
	}
	sort.Slice(manifest.Components, func(i, j int) bool { return manifest.Components[i].ID < manifest.Components[j].ID })
	sort.Slice(manifest.CandidateDocumentationUnits, func(i, j int) bool {
		if manifest.CandidateDocumentationUnits[i].ComponentID == manifest.CandidateDocumentationUnits[j].ComponentID {
			if manifest.CandidateDocumentationUnits[i].Kind == manifest.CandidateDocumentationUnits[j].Kind {
				return manifest.CandidateDocumentationUnits[i].ID < manifest.CandidateDocumentationUnits[j].ID
			}
			return manifest.CandidateDocumentationUnits[i].Kind < manifest.CandidateDocumentationUnits[j].Kind
		}
		return manifest.CandidateDocumentationUnits[i].ComponentID < manifest.CandidateDocumentationUnits[j].ComponentID
	})
	sort.Strings(manifest.Unknowns)
	return manifest, nil
}

func (p Planner) Plan(componentID string, includeSystem bool) (PlanResult, error) {
	manifest, err := p.Discover(componentID)
	if err != nil {
		return PlanResult{}, err
	}
	componentByID := map[string]config.ComponentConfig{}
	for _, component := range p.Config.EnabledComponents() {
		componentByID[component.ID] = component
	}
	result := PlanResult{}
	for _, discovered := range manifest.Components {
		component := componentByID[discovered.ID]
		units := filterUnits(manifest.CandidateDocumentationUnits, component.ID)
		result.Components = append(result.Components, buildComponentPlan(p.Config, component, units))
	}
	sort.Slice(result.Components, func(i, j int) bool { return result.Components[i].ComponentID < result.Components[j].ComponentID })
	if includeSystem && p.Config.System.Enabled {
		result.System = buildSystemPlan(p.Config, manifest.CandidateDocumentationUnits)
	}
	return result, nil
}

func (p Planner) selectedComponents(componentID string) ([]config.ComponentConfig, error) {
	components := p.Config.EnabledComponents()
	if componentID == "" {
		return components, nil
	}
	for _, component := range components {
		if component.ID == componentID {
			return []config.ComponentConfig{component}, nil
		}
	}
	return nil, fmt.Errorf("no enabled component matched %q", componentID)
}

func discoverUnits(component config.ComponentConfig, explicit []config.DocumentationUnitConfig) ([]DocumentationUnit, []string) {
	unitByID := map[string]DocumentationUnit{}
	for _, unit := range explicit {
		unitByID[unit.ID] = DocumentationUnit{
			ID:             unit.ID,
			ComponentID:    component.ID,
			Kind:           DocumentationUnitKind(unit.Kind),
			SourceRoots:    append([]string(nil), unit.SourceRoots...),
			RelatedUnits:   append([]string(nil), unit.RelatedUnits...),
			OutputPath:     unit.Output,
			Owners:         append([]string(nil), unit.Owners...),
			Capabilities:   append([]string(nil), unit.Capabilities...),
			Criticality:    unit.Criticality,
			Domain:         unit.Domain,
			Subdomain:      unit.Subdomain,
			BoundedContext: unit.BoundedContext,
			View:           unit.View,
			EvidenceRoots:  append([]string(nil), unit.EvidenceRoots...),
			ShardBy:        append([]string(nil), unit.ShardBy...),
			MaximumRows:    unit.MaximumRows,
			Explicit:       true,
			Reason:         "explicit documentation unit from configuration",
		}
	}
	hasComponentUnit := false
	hasDomainUnit := false
	for _, unit := range unitByID {
		if unit.Kind == UnitComponent {
			hasComponentUnit = true
		}
		if unit.Kind == UnitDomain {
			hasDomainUnit = true
		}
	}
	if !hasComponentUnit {
		unitByID[component.ID] = DocumentationUnit{
			ID:           component.ID,
			ComponentID:  component.ID,
			Kind:         UnitComponent,
			SourceRoots:  []string{"."},
			OutputPath:   filepath.ToSlash(filepath.Join("components", component.ID)),
			Owners:       append([]string(nil), component.Owners...),
			Capabilities: append([]string(nil), component.Capabilities...),
			Domain:       "",
			Explicit:     false,
			Reason:       "enabled component always gets a component documentation unit",
		}
	}
	if !hasDomainUnit {
		for _, capability := range component.Capabilities {
			id := slug(capability)
			if id == "" {
				continue
			}
			unitByID[id] = DocumentationUnit{
				ID:           id,
				ComponentID:  component.ID,
				Kind:         UnitDomain,
				SourceRoots:  []string{"."},
				OutputPath:   filepath.ToSlash(filepath.Join("domains", id)),
				Owners:       append([]string(nil), component.Owners...),
				Capabilities: []string{capability},
				Domain:       capability,
				Explicit:     false,
				Reason:       "derived from component capability",
			}
			hasDomainUnit = true
		}
	}
	if !hasUnitKind(unitByID, UnitFlow) {
		for _, unit := range deriveFlowUnitsFromRepository(component) {
			if _, exists := unitByID[unit.ID]; !exists {
				unitByID[unit.ID] = unit
			}
		}
	}
	var unknowns []string
	if !hasDomainUnit && component.Profile == "modular-application" {
		derived := deriveDomainUnitsFromRepository(component)
		if len(derived) == 0 {
			unknowns = append(unknowns, fmt.Sprintf("component %s has modular-application profile but no domain candidates were discovered from capabilities or module roots", component.ID))
		}
		for _, unit := range derived {
			if _, exists := unitByID[unit.ID]; exists {
				continue
			}
			unitByID[unit.ID] = unit
		}
	}
	units := make([]DocumentationUnit, 0, len(unitByID))
	for _, unit := range unitByID {
		units = append(units, unit)
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].Kind == units[j].Kind {
			return units[i].ID < units[j].ID
		}
		return units[i].Kind < units[j].Kind
	})
	return units, unknowns
}

func deriveDomainUnitsFromRepository(component config.ComponentConfig) []DocumentationUnit {
	var units []DocumentationUnit
	for _, root := range []string{"modules", "domains", "bounded-contexts", "contexts"} {
		path := filepath.Join(component.WorkDir(), root)
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := slug(entry.Name())
			if id == "" {
				continue
			}
			units = append(units, DocumentationUnit{
				ID:           id,
				ComponentID:  component.ID,
				Kind:         UnitDomain,
				SourceRoots:  []string{filepath.ToSlash(filepath.Join(root, entry.Name()))},
				OutputPath:   filepath.ToSlash(filepath.Join("domains", id)),
				Owners:       append([]string(nil), component.Owners...),
				Capabilities: []string{entry.Name()},
				Domain:       entry.Name(),
				Explicit:     false,
				Reason:       fmt.Sprintf("derived from %s/%s repository root", root, entry.Name()),
			})
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	return units
}

func deriveFlowUnitsFromRepository(component config.ComponentConfig) []DocumentationUnit {
	var units []DocumentationUnit
	for _, root := range []string{"workflows", "bpmn", "processes"} {
		path := filepath.Join(component.WorkDir(), root)
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			id := slug(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
			if id == "" {
				continue
			}
			units = append(units, DocumentationUnit{
				ID: "flow-" + id, ComponentID: component.ID, Kind: UnitFlow,
				SourceRoots:  []string{filepath.ToSlash(filepath.Join(root, entry.Name()))},
				OutputPath:   filepath.ToSlash(filepath.Join("flows", id+".md")),
				Capabilities: []string{"workflow"}, Explicit: false,
				Reason: fmt.Sprintf("derived from %s/%s workflow source", root, entry.Name()),
			})
		}
	}
	return units
}

func hasUnitKind(units map[string]DocumentationUnit, kind DocumentationUnitKind) bool {
	for _, unit := range units {
		if unit.Kind == kind {
			return true
		}
	}
	return false
}

func buildComponentPlan(cfg config.Config, component config.ComponentConfig, units []DocumentationUnit) ComponentPlan {
	plan := ComponentPlan{
		ComponentID: component.ID,
		Profile:     component.Profile,
		Views:       componentViews(cfg.Documentation.Views, component.Profile),
		Packs:       packsForComponent(component),
		Units:       append([]DocumentationUnit(nil), units...),
	}
	plan.Pages = append(plan.Pages, PlannedPage{
		Path:   "quickstart.md",
		View:   ViewComponent,
		Kind:   PageIndex,
		Reason: "root quickstart is kept as the entrypoint into the adaptive hierarchy",
	})
	plan.Decisions = append(plan.Decisions, PlanDecision{
		Target: "quickstart.md",
		Action: "create",
		Reason: "adaptive planning keeps one root entrypoint while allowing deeper navigation through view indexes",
	})
	addPage := func(page PlannedPage) {
		for _, existing := range plan.Pages {
			if existing.Path == page.Path {
				return
			}
		}
		plan.Pages = append(plan.Pages, page)
	}

	componentUnit := firstUnit(units, UnitComponent)
	if componentUnit != nil {
		addPage(PlannedPage{Path: "components/index.md", View: ViewComponent, Kind: PageIndex, Reason: "component view index links component unit indexes without flattening leaf pages"})
		base := componentUnit.OutputPath
		for _, rel := range []string{"index.md", "architecture.md", "contracts.md", "data-and-consistency.md", "runtime-and-operations.md"} {
			plan.Pages = append(plan.Pages, PlannedPage{
				Path:      filepath.ToSlash(filepath.Join(base, rel)),
				View:      ViewComponent,
				Kind:      pageKindForRelativePath(rel, PageIndex),
				OwnerUnit: componentUnit.ID,
				Reason:    "component view is always created for the deployable or reusable boundary",
			})
		}
	}

	domainUnits := unitsOfKind(units, UnitDomain)
	if containsView(plan.Views, ViewDomain) {
		if len(domainUnits) == 0 {
			plan.Decisions = append(plan.Decisions, PlanDecision{
				Target: "domains/",
				Action: "skip",
				Reason: "no domain documentation units were discovered for this component",
			})
		}
		if len(domainUnits) > 0 {
			addPage(PlannedPage{Path: "domains/index.md", View: ViewDomain, Kind: PageIndex, Reason: "domain view index links discovered domain units"})
		}
		for _, unit := range domainUnits {
			for _, rel := range []string{"index.md", "concepts-and-data.md", "rules-and-invariants.md", "workflows-and-state.md", "interfaces-and-events.md", "component-mapping.md"} {
				plan.Pages = append(plan.Pages, PlannedPage{
					Path:      filepath.ToSlash(filepath.Join(unit.OutputPath, rel)),
					View:      ViewDomain,
					Kind:      pageKindForRelativePath(rel, PageIndex),
					OwnerUnit: unit.ID,
					Reason:    unit.Reason,
				})
			}
		}
	}

	flowUnits := unitsOfKind(units, UnitFlow)
	if containsView(plan.Views, ViewFlow) {
		if len(flowUnits) == 0 {
			plan.Decisions = append(plan.Decisions, PlanDecision{
				Target: "flows/",
				Action: "skip",
				Reason: "flow pages are omitted until explicit flow units exist",
			})
		}
		if len(flowUnits) > 0 {
			addPage(PlannedPage{Path: "flows/index.md", View: ViewFlow, Kind: PageIndex, Reason: "flow view index links explicit flow units"})
		}
		for _, unit := range flowUnits {
			flowPath := unit.OutputPath
			if flowPath == "" {
				flowPath = filepath.ToSlash(filepath.Join("flows", unit.ID))
			}
			if filepath.Ext(flowPath) == "" {
				flowPath += ".md"
			}
			plan.Pages = append(plan.Pages, PlannedPage{
				Path:      filepath.ToSlash(flowPath),
				View:      ViewFlow,
				Kind:      PageSingle,
				OwnerUnit: unit.ID,
				Reason:    unit.Reason,
			})
		}
	}

	if containsView(plan.Views, ViewCatalog) {
		planCatalogPages(cfg, &plan, domainUnits)
		if hasViewPages(plan.Pages, ViewCatalog) {
			addPage(PlannedPage{Path: "catalogs/index.md", View: ViewCatalog, Kind: PageIndex, Reason: "catalog view index links typed collections and shards"})
		}
	}
	if containsView(plan.Views, ViewPlatform) {
		planPlatformPages(&plan)
		if hasViewPages(plan.Pages, ViewPlatform) {
			addPage(PlannedPage{Path: "platform/index.md", View: ViewPlatform, Kind: PageIndex, Reason: "platform view index links applicable cross-cutting concerns"})
		}
	}
	if containsView(plan.Views, ViewEngineering) {
		plan.Pages = append(plan.Pages, PlannedPage{Path: "engineering/index.md", View: ViewEngineering, Kind: PageIndex, Reason: "engineering view is enabled for this profile"})
	}
	if containsView(plan.Views, ViewOperations) {
		plan.Pages = append(plan.Pages, PlannedPage{Path: "operations/index.md", View: ViewOperations, Kind: PageIndex, Reason: "operations view is enabled for this profile"})
	}

	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].Path < plan.Pages[j].Path })
	for i := range plan.Pages {
		plan.Pages[i].Contract = pageContract(cfg, plan.Pages[i], plan.Packs)
		for _, unit := range plan.Units {
			if plan.Pages[i].OwnerUnit == unit.ID && unit.MaximumRows > 0 {
				plan.Pages[i].Contract.MaximumRowsPerShard = unit.MaximumRows
			}
		}
	}
	sort.Slice(plan.Decisions, func(i, j int) bool {
		if plan.Decisions[i].Target == plan.Decisions[j].Target {
			return plan.Decisions[i].Action < plan.Decisions[j].Action
		}
		return plan.Decisions[i].Target < plan.Decisions[j].Target
	})
	return plan
}

func planCatalogPages(cfg config.Config, plan *ComponentPlan, domainUnits []DocumentationUnit) {
	catalogs := []struct {
		ID     string
		Packs  []string
		Reason string
	}{
		{ID: "interfaces", Packs: []string{"api", "interface"}, Reason: "interface pack keeps inbound and outbound contracts separate from component narrative"},
		{ID: "service-interactions", Packs: []string{"api", "integration"}, Reason: "integration pack keeps service interactions and dependency direction explicit"},
		{ID: "events", Packs: []string{"messaging"}, Reason: "messaging pack enables event and message catalogs"},
		{ID: "workflows", Packs: []string{"workflow", "bpmn"}, Reason: "workflow and BPMN packs enable process catalogs"},
		{ID: "jobs", Packs: []string{"jobs", "scheduler"}, Reason: "job and scheduler packs enable worker and schedule catalogs"},
		{ID: "files", Packs: []string{"file-processing"}, Reason: "file processing pack enables file flow and format catalogs"},
		{ID: "data", Packs: []string{"data", "persistence"}, Reason: "data and persistence packs enable data-store catalogs"},
		{ID: "database-objects", Packs: []string{"database-programmability"}, Reason: "database programmability pack enables database object catalogs"},
		{ID: "migrations-and-seeds", Packs: []string{"migration", "migration-and-seeding"}, Reason: "migration and seeding packs enable schema lifecycle catalogs"},
		{ID: "repositories", Packs: []string{"repository", "data-access"}, Reason: "repository and data-access packs enable mapping catalogs"},
		{ID: "caches", Packs: []string{"cache"}, Reason: "cache pack enables cache role and invalidation catalogs"},
		{ID: "rate-limits", Packs: []string{"rate-limit"}, Reason: "rate-limit pack enables quota and enforcement catalogs"},
		{ID: "distributed-coordination", Packs: []string{"distributed-coordination"}, Reason: "distributed coordination pack enables lock and lease catalogs"},
		{ID: "permissions", Packs: []string{"security", "identity", "authentication", "authorization", "acl"}, Reason: "identity, authentication, authorization, and ACL packs enable permission catalogs"},
		{ID: "cryptography", Packs: []string{"cryptography"}, Reason: "cryptography pack enables key and protocol catalogs without exposing values"},
		{ID: "concurrency", Packs: []string{"concurrency", "context-propagation"}, Reason: "concurrency and context packs enable execution and propagation catalogs"},
		{ID: "configuration", Packs: []string{"configuration"}, Reason: "configuration pack enables precedence and source catalogs"},
		{ID: "telemetry", Packs: []string{"telemetry"}, Reason: "telemetry pack enables signal and correlation catalogs"},
		{ID: "deployment", Packs: []string{"deployment", "container-runtime", "container-and-deployment"}, Reason: "container and deployment packs enable deployment resource catalogs"},
	}
	packs := map[string]bool{}
	for _, pack := range plan.Packs {
		packs[pack] = true
	}
	for _, catalog := range catalogs {
		requiredPack := strings.Join(catalog.Packs, " or ")
		applicable := false
		for _, candidate := range catalog.Packs {
			if packs[candidate] {
				applicable = true
				break
			}
		}
		if !applicable {
			plan.Decisions = append(plan.Decisions, PlanDecision{
				Target: filepath.ToSlash(filepath.Join("catalogs", catalog.ID)),
				Action: "skip",
				Reason: fmt.Sprintf("%s is not enabled for this component", requiredPack),
			})
			continue
		}
		root := filepath.ToSlash(filepath.Join("catalogs", catalog.ID, "index.md"))
		plan.Pages = append(plan.Pages, PlannedPage{Path: root, View: ViewCatalog, Kind: PageCollection, Reason: catalog.Reason})
		shardDimension := catalogShardDimension(cfg, domainUnits)
		if shardDimension == "domain" {
			plan.Decisions = append(plan.Decisions, PlanDecision{
				Target: filepath.ToSlash(filepath.Join("catalogs", catalog.ID)),
				Action: "shard",
				Reason: fmt.Sprintf("sharded by domain because documentation.catalogs.shardBy includes domain and %d domain units were discovered", len(domainUnits)),
			})
			for _, unit := range domainUnits {
				plan.Pages = append(plan.Pages, PlannedPage{
					Path:      filepath.ToSlash(filepath.Join("catalogs", catalog.ID, unit.ID+".md")),
					View:      ViewCatalog,
					Kind:      PageShard,
					OwnerUnit: unit.ID,
					Reason:    "domain-based sharding keeps large catalogs bounded and navigable",
				})
			}
			continue
		}
		if shardDimension == "owner" {
			owners := distinctUnitOwners(domainUnits)
			plan.Decisions = append(plan.Decisions, PlanDecision{
				Target: filepath.ToSlash(filepath.Join("catalogs", catalog.ID)),
				Action: "shard",
				Reason: fmt.Sprintf("sharded by owner because documentation.catalogs.shardBy includes owner and %d owners were discovered", len(owners)),
			})
			for _, owner := range owners {
				plan.Pages = append(plan.Pages, PlannedPage{
					Path:      filepath.ToSlash(filepath.Join("catalogs", catalog.ID, slug(owner)+".md")),
					View:      ViewCatalog,
					Kind:      PageShard,
					OwnerUnit: "owner:" + owner,
					Reason:    "owner-based sharding keeps catalog responsibility boundaries explicit",
				})
			}
			continue
		}
		plan.Decisions = append(plan.Decisions, PlanDecision{
			Target: filepath.ToSlash(filepath.Join("catalogs", catalog.ID)),
			Action: "merge",
			Reason: "single collection page is sufficient because no domain shard dimension was applicable",
		})
	}
}

func planPlatformPages(plan *ComponentPlan) {
	packs := map[string]bool{}
	for _, pack := range plan.Packs {
		packs[pack] = true
	}
	optional := []struct {
		Pack string
		Path string
	}{
		{Pack: "messaging", Path: "platform/messaging.md"},
		{Pack: "security", Path: "platform/security-and-identity.md"},
		{Pack: "telemetry", Path: "platform/telemetry.md"},
		{Pack: "container-runtime", Path: "platform/containerization.md"},
		{Pack: "deployment", Path: "platform/containerization.md"},
	}
	seen := map[string]bool{}
	for _, candidate := range optional {
		if !packs[candidate.Pack] || seen[candidate.Path] {
			continue
		}
		seen[candidate.Path] = true
		plan.Pages = append(plan.Pages, PlannedPage{
			Path:   candidate.Path,
			View:   ViewPlatform,
			Kind:   PageSingle,
			Reason: fmt.Sprintf("%s pack maps to a platform view page", candidate.Pack),
		})
	}
}

func buildSystemPlan(cfg config.Config, units []DocumentationUnit) *SystemPlan {
	views := []DocumentationView{ViewSystem}
	plan := &SystemPlan{Views: views}
	plan.Pages = append(plan.Pages,
		PlannedPage{Path: "quickstart.md", View: ViewSystem, Kind: PageIndex, Reason: "system quickstart is the root entrypoint into the system hierarchy"},
		PlannedPage{Path: "system/index.md", View: ViewSystem, Kind: PageIndex, Reason: "system view is enabled because whole-system aggregation is configured"},
		PlannedPage{Path: "system/overview.md", View: ViewSystem, Kind: PageSingle, Reason: "system overview summarizes the discovered component landscape"},
		PlannedPage{Path: "system/capability-map.md", View: ViewSystem, Kind: PageSingle, Reason: "capability map connects business capabilities to documentation units"},
		PlannedPage{Path: "system/component-landscape.md", View: ViewSystem, Kind: PageSingle, Reason: "component landscape tracks runtime and repository boundaries"},
		PlannedPage{Path: "system/runtime-topology.md", View: ViewSystem, Kind: PageSingle, Reason: "runtime topology maps cross-component execution shape"},
	)
	if len(unitsOfKind(units, UnitDomain)) > 0 {
		plan.Pages = append(plan.Pages, PlannedPage{Path: "system/domain-map.md", View: ViewSystem, Kind: PageSingle, Reason: "domain map becomes applicable once domain units exist"})
	}
	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].Path < plan.Pages[j].Path })
	for i := range plan.Pages {
		plan.Pages[i].Contract = pageContract(cfg, plan.Pages[i], []string{"system"})
	}
	return plan
}

func componentViews(configured []string, profile string) []DocumentationView {
	if len(configured) > 0 {
		var out []DocumentationView
		for _, view := range configured {
			if view == string(ViewSystem) {
				continue
			}
			out = append(out, DocumentationView(view))
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	defaults := map[string][]DocumentationView{
		"application":         {ViewCatalog, ViewComponent, ViewFlow, ViewOperations},
		"modular-application": {ViewCatalog, ViewComponent, ViewDomain, ViewEngineering, ViewFlow, ViewOperations},
		"reusable":            {ViewCatalog, ViewComponent, ViewEngineering},
		"infrastructure":      {ViewCatalog, ViewComponent, ViewOperations, ViewPlatform},
		"configuration":       {ViewCatalog, ViewComponent, ViewOperations, ViewPlatform},
		"contracts":           {ViewCatalog, ViewComponent, ViewEngineering},
		"generic":             {ViewCatalog, ViewComponent, ViewOperations},
	}
	return append([]DocumentationView(nil), defaults[profile]...)
}

func packsForComponent(component config.ComponentConfig) []string {
	base := map[string][]string{
		"application":         {"api", "configuration", "data", "jobs", "messaging", "security"},
		"modular-application": {"api", "configuration", "data", "domain", "jobs", "messaging", "security"},
		"reusable":            {"api", "configuration", "engineering", "integration"},
		"infrastructure":      {"configuration", "deployment", "security", "telemetry"},
		"configuration":       {"configuration", "security"},
		"contracts":           {"api", "compatibility", "messaging"},
		"generic":             {"api", "configuration"},
	}[component.Profile]
	set := map[string]bool{}
	for _, pack := range base {
		set[pack] = true
	}
	for _, pack := range component.Packs {
		pack = strings.TrimSpace(strings.ToLower(pack))
		if pack != "" {
			set[pack] = true
		}
	}
	if hasWorkflowSources(component.WorkDir()) {
		set["workflow"] = true
		set["bpmn"] = true
	}
	out := make([]string, 0, len(set))
	for pack := range set {
		out = append(out, pack)
	}
	sort.Strings(out)
	return out
}

func hasWorkflowSources(root string) bool {
	for _, directory := range []string{"workflows", "bpmn", "processes"} {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err == nil && len(entries) > 0 {
			return true
		}
	}
	return false
}

func catalogShardDimension(cfg config.Config, domains []DocumentationUnit) string {
	for _, shard := range cfg.Documentation.Catalogs.ShardBy {
		if strings.EqualFold(shard, "domain") && len(domains) >= 2 {
			return "domain"
		}
		if strings.EqualFold(shard, "owner") && len(distinctUnitOwners(domains)) >= 2 {
			return "owner"
		}
	}
	return ""
}

func distinctUnitOwners(units []DocumentationUnit) []string {
	set := map[string]bool{}
	for _, unit := range units {
		for _, owner := range unit.Owners {
			owner = strings.TrimSpace(owner)
			if owner != "" {
				set[owner] = true
			}
		}
	}
	owners := make([]string, 0, len(set))
	for owner := range set {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	return owners
}

func pageContract(cfg config.Config, page PlannedPage, packs []string) model.PageContract {
	contract := model.PageContract{
		Path:                filepath.ToSlash(page.Path),
		Kind:                page.Kind,
		PathTemplate:        filepath.ToSlash(page.Path),
		IndexPathTemplate:   indexPathFor(page.Path, page.Kind),
		Applicability:       model.ApplicabilityRule{Views: []string{string(page.View)}, Packs: append([]string(nil), packs...)},
		MaximumRowsPerShard: cfg.Documentation.Catalogs.MaximumRowsPerPage,
		MaximumBytes:        cfg.Documentation.Catalogs.MaximumBytesPerPage,
	}
	if page.Kind == PageShard {
		if strings.HasPrefix(page.OwnerUnit, "owner:") {
			contract.ShardDimensions = []model.ShardDimension{model.ShardOwner}
			contract.ShardKey = strings.TrimPrefix(page.OwnerUnit, "owner:")
			contract.OwnershipPartition = true
		} else {
			contract.ShardDimensions = []model.ShardDimension{model.ShardDomain}
			contract.ShardKey = page.OwnerUnit
		}
	}
	return contract
}

func indexPathFor(path string, kind PageKind) string {
	path = filepath.ToSlash(path)
	if kind == PageIndex || kind == PageCollection {
		return path
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(path), "index.md"))
}

func unitsOfKind(units []DocumentationUnit, kind DocumentationUnitKind) []DocumentationUnit {
	var out []DocumentationUnit
	for _, unit := range units {
		if unit.Kind == kind {
			out = append(out, unit)
		}
	}
	return out
}

func firstUnit(units []DocumentationUnit, kind DocumentationUnitKind) *DocumentationUnit {
	for i := range units {
		if units[i].Kind == kind {
			return &units[i]
		}
	}
	return nil
}

func filterUnits(units []DocumentationUnit, componentID string) []DocumentationUnit {
	var out []DocumentationUnit
	for _, unit := range units {
		if unit.ComponentID == componentID {
			out = append(out, unit)
		}
	}
	return out
}

func containsView(views []DocumentationView, target DocumentationView) bool {
	for _, view := range views {
		if view == target {
			return true
		}
	}
	return false
}

func hasViewPages(pages []PlannedPage, view DocumentationView) bool {
	for _, page := range pages {
		if page.View == view && page.Kind != PageIndex {
			return true
		}
	}
	return false
}

func pageKindForRelativePath(path string, index PageKind) PageKind {
	if filepath.Base(filepath.ToSlash(path)) == "index.md" {
		return index
	}
	return PageSingle
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", "-", " ", "-", "/", "-", "\\", "-", ".", "-")
	value = replacer.Replace(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
