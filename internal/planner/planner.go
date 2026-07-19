package planner

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	EvidenceIDs    []string              `json:"evidenceIds,omitempty"`
	ShardBy        []string              `json:"shardBy,omitempty"`
	Explicit       bool                  `json:"explicit"`
	Reason         string                `json:"reason"`
	Provenance     string                `json:"provenance,omitempty"`
	Confidence     string                `json:"confidence,omitempty"`
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
	Components                  []DiscoveredComponent              `json:"components"`
	CandidateDocumentationUnits []DocumentationUnit                `json:"candidateDocumentationUnits"`
	Unknowns                    []string                           `json:"unknowns,omitempty"`
	Semantic                    map[string]model.SemanticDiscovery `json:"semantic,omitempty"`
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
	ComponentID         string              `json:"componentId"`
	Profile             string              `json:"profile"`
	Views               []DocumentationView `json:"views"`
	Packs               []string            `json:"packs"`
	Units               []DocumentationUnit `json:"units"`
	Pages               []PlannedPage       `json:"pages"`
	Decisions           []PlanDecision      `json:"decisions"`
	SemanticFingerprint string              `json:"semanticFingerprint"`
}

type SystemPlan struct {
	Views               []DocumentationView `json:"views"`
	Pages               []PlannedPage       `json:"pages"`
	Decisions           []PlanDecision      `json:"decisions"`
	SemanticFingerprint string              `json:"semanticFingerprint"`
}

type PlanResult struct {
	Components []ComponentPlan `json:"components"`
	System     *SystemPlan     `json:"system,omitempty"`
}

type Planner struct {
	Config   config.Config
	Semantic map[string]model.SemanticDiscovery
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
	manifest := DiscoveryManifest{Semantic: map[string]model.SemanticDiscovery{}}
	for _, component := range components {
		semantic, ok := p.Semantic[component.ID]
		if !ok {
			if p.Config.Documentation.Discovery.Mode != "explicit" && p.Config.Documentation.Discovery.Mode != "disabled" {
				return DiscoveryManifest{}, fmt.Errorf("validated semantic discovery is required for component %s; run wikiforge discover first", component.ID)
			}
			semantic = explicitSemantic(component)
			semantic.DiscoveryMode = p.Config.Documentation.Discovery.Mode
		}
		units, unknowns := discoverUnitsFromSemantic(component, explicitByComponent[component.ID], semantic)
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
			Packs:              packsForSemantic(component, semantic),
			DocumentationUnits: unitIDs,
		})
		manifest.Unknowns = append(manifest.Unknowns, unknowns...)
		manifest.Semantic[component.ID] = semantic
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

func explicitSemantic(component config.ComponentConfig) model.SemanticDiscovery {
	return model.SemanticDiscovery{SchemaVersion: model.DiscoverySchemaVersion, ComponentID: component.ID, RepositoryID: component.ID, DiscoveryMode: "explicit", InventoryVersion: "explicit", PromptVersion: "explicit", Repository: model.RepositoryFinding{Profile: component.Profile, Status: model.StatusExplicitEnabled, Confidence: "high"}, Quality: model.QualityResult{Accepted: true}}
}

func discoverUnitsFromSemantic(component config.ComponentConfig, explicit []config.DocumentationUnitConfig, semantic model.SemanticDiscovery) ([]DocumentationUnit, []string) {
	units, unknowns := explicitUnits(component, explicit)
	explicitIDs := map[string]string{}
	for _, unit := range units {
		if unit.Kind != UnitDomain && unit.Kind != UnitFlow {
			continue
		}
		explicitIDs[slug(unit.ID)] = unit.ID
		if unit.Domain != "" {
			explicitIDs[slug(unit.Domain)] = unit.ID
		}
	}
	ownersBySubject := map[string][]string{}
	for _, ownership := range semantic.Ownership {
		ownersBySubject[ownership.SubjectID] = append([]string{}, ownership.Owners...)
	}
	seen := map[string]bool{}
	for _, unit := range units {
		seen[unit.ID] = true
	}
	if !seen[component.ID] {
		units = append(units, DocumentationUnit{ID: component.ID, ComponentID: component.ID, Kind: UnitComponent, SourceRoots: []string{"."}, OutputPath: filepath.ToSlash(filepath.Join("components", component.ID)), Owners: append([]string{}, component.Owners...), Explicit: false, Provenance: "derived", Reason: "validated component boundary"})
	}
	for _, domain := range semantic.Domains {
		if domain.Status != model.StatusObserved && domain.Status != model.StatusExplicitEnabled {
			continue
		}
		if domain.ID == "" {
			continue
		}
		if explicitID := explicitUnitForFinding(explicitIDs, domain.ID, domain.Candidate.CandidateKey, domain.Candidate.Name); explicitID != "" {
			enrichExplicitDomainUnit(units, explicitID, domain)
			seen[explicitID] = true
			continue
		}
		if seen[domain.ID] {
			continue
		}
		roots := append([]string{}, domain.SourceRoots...)
		if len(roots) == 0 {
			roots = []string{"."}
		}
		owners := append([]string{}, domain.Owners...)
		if len(owners) == 0 {
			owners = ownersBySubject[domain.ID]
		}
		related := append([]string{}, domain.ModuleIDs...)
		evidenceIDs := append([]string{}, domain.EvidenceIDs...)
		evidenceIDs = append(evidenceIDs, domain.Candidate.EvidenceIDs...)
		units = append(units, DocumentationUnit{ID: domain.ID, ComponentID: component.ID, Kind: UnitDomain, SourceRoots: roots, RelatedUnits: related, OutputPath: filepath.ToSlash(filepath.Join("domains", domain.ID)), Owners: owners, Criticality: domain.Criticality, Domain: domain.Candidate.Name, Subdomain: domain.Subdomain, BoundedContext: domain.BoundedContext, EvidenceRoots: append([]string{}, roots...), EvidenceIDs: uniqueStrings(evidenceIDs), Confidence: domain.Confidence, Explicit: false, Provenance: "inferred", Reason: "accepted semantic domain finding"})
		seen[domain.ID] = true
	}
	for _, flow := range semantic.Flows {
		if flow.Status != model.StatusObserved && flow.Status != model.StatusExplicitEnabled {
			continue
		}
		if flow.ID == "" || seen[flow.ID] {
			continue
		}
		if explicitID := explicitUnitForFinding(explicitIDs, flow.ID, flow.Candidate.CandidateKey, flow.Candidate.Name); explicitID != "" {
			enrichExplicitFlowUnit(units, explicitID, flow)
			seen[explicitID] = true
			continue
		}
		roots := append([]string{}, flow.SourceRoots...)
		if len(roots) == 0 {
			roots = []string{"."}
		}
		evidenceIDs := append([]string{}, flow.EvidenceIDs...)
		evidenceIDs = append(evidenceIDs, flow.Candidate.EvidenceIDs...)
		units = append(units, DocumentationUnit{ID: flow.ID, ComponentID: component.ID, Kind: UnitFlow, SourceRoots: roots, RelatedUnits: append([]string{}, flow.ModuleIDs...), OutputPath: filepath.ToSlash(filepath.Join("flows", flow.ID+".md")), Capabilities: append([]string{}, flow.Triggers...), EvidenceRoots: append([]string{}, roots...), EvidenceIDs: uniqueStrings(evidenceIDs), Confidence: flow.Confidence, Explicit: false, Provenance: "inferred", Reason: "accepted semantic flow finding"})
		seen[flow.ID] = true
	}
	if len(semantic.Domains) == 0 && component.Profile == "modular-application" {
		unknowns = append(unknowns, "no accepted semantic domain findings")
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].Kind == units[j].Kind {
			return units[i].ID < units[j].ID
		}
		return units[i].Kind < units[j].Kind
	})
	return units, unknowns
}

func explicitUnitForFinding(explicitIDs map[string]string, references ...string) string {
	for _, reference := range references {
		if id := explicitIDs[slug(reference)]; id != "" {
			return id
		}
	}
	return ""
}

func enrichExplicitDomainUnit(units []DocumentationUnit, id string, finding model.DomainFinding) {
	for i := range units {
		if units[i].ID != id {
			continue
		}
		if units[i].Domain == "" {
			units[i].Domain = finding.Candidate.Name
		}
		if len(units[i].SourceRoots) == 0 {
			units[i].SourceRoots = append([]string{}, finding.SourceRoots...)
		}
		if len(units[i].RelatedUnits) == 0 {
			units[i].RelatedUnits = append([]string{}, finding.ModuleIDs...)
		}
		if len(units[i].Owners) == 0 {
			units[i].Owners = append([]string{}, finding.Owners...)
		}
		if units[i].Criticality == "" {
			units[i].Criticality = finding.Criticality
		}
		if units[i].Subdomain == "" {
			units[i].Subdomain = finding.Subdomain
		}
		if units[i].BoundedContext == "" {
			units[i].BoundedContext = finding.BoundedContext
		}
		units[i].EvidenceRoots = uniqueStrings(append(units[i].EvidenceRoots, finding.SourceRoots...))
		units[i].EvidenceIDs = uniqueStrings(append(units[i].EvidenceIDs, finding.EvidenceIDs...))
		return
	}
}

func enrichExplicitFlowUnit(units []DocumentationUnit, id string, finding model.FlowFinding) {
	for i := range units {
		if units[i].ID != id {
			continue
		}
		if len(units[i].SourceRoots) == 0 {
			units[i].SourceRoots = append([]string{}, finding.SourceRoots...)
		}
		if len(units[i].RelatedUnits) == 0 {
			units[i].RelatedUnits = append([]string{}, finding.ModuleIDs...)
		}
		if len(units[i].Capabilities) == 0 {
			units[i].Capabilities = append([]string{}, finding.Triggers...)
		}
		units[i].EvidenceRoots = uniqueStrings(append(units[i].EvidenceRoots, finding.SourceRoots...))
		units[i].EvidenceIDs = uniqueStrings(append(units[i].EvidenceIDs, finding.EvidenceIDs...))
		return
	}
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value != "" && !set[value] {
			set[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func explicitUnits(component config.ComponentConfig, explicit []config.DocumentationUnitConfig) ([]DocumentationUnit, []string) {
	var units []DocumentationUnit
	for _, unit := range explicit {
		units = append(units, DocumentationUnit{ID: unit.ID, ComponentID: component.ID, Kind: DocumentationUnitKind(unit.Kind), SourceRoots: append([]string{}, unit.SourceRoots...), RelatedUnits: append([]string{}, unit.RelatedUnits...), OutputPath: unit.Output, Owners: append([]string{}, unit.Owners...), Capabilities: append([]string{}, unit.Capabilities...), Criticality: unit.Criticality, Domain: unit.Domain, Subdomain: unit.Subdomain, BoundedContext: unit.BoundedContext, View: unit.View, EvidenceRoots: append([]string{}, unit.EvidenceRoots...), ShardBy: append([]string{}, unit.ShardBy...), Explicit: true, Provenance: "explicit", Reason: "explicit documentation unit from configuration"})
	}
	return units, nil
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
		result.Components = append(result.Components, buildComponentPlan(p.Config, component, units, manifest.Semantic[component.ID]))
	}
	sort.Slice(result.Components, func(i, j int) bool { return result.Components[i].ComponentID < result.Components[j].ComponentID })
	if includeSystem && p.Config.System.Enabled {
		result.System = buildSystemPlan(p.Config, manifest.CandidateDocumentationUnits, p.Semantic)
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

func buildComponentPlan(cfg config.Config, component config.ComponentConfig, units []DocumentationUnit, semantic model.SemanticDiscovery) ComponentPlan {
	plan := ComponentPlan{
		ComponentID:         component.ID,
		Profile:             component.Profile,
		Views:               componentViews(cfg.Documentation.Views, component.Profile),
		Packs:               packsForSemantic(component, semantic),
		Units:               append([]DocumentationUnit(nil), units...),
		SemanticFingerprint: semanticFingerprint(semantic),
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
	for _, domain := range semantic.Domains {
		if domain.Status == model.StatusUncertain || domain.Status == model.StatusConflicting {
			plan.Decisions = append(plan.Decisions, PlanDecision{Target: "domain:" + domain.Candidate.Name, Action: "retain-uncertain", Reason: "semantic domain was not promoted because its status is " + domain.Status})
		}
	}
	for _, flow := range semantic.Flows {
		if flow.Status == model.StatusUncertain || flow.Status == model.StatusConflicting {
			plan.Decisions = append(plan.Decisions, PlanDecision{Target: "flow:" + flow.Candidate.Name, Action: "retain-uncertain", Reason: "semantic flow was not promoted because its status is " + flow.Status})
		}
	}
	for _, module := range semantic.Modules {
		if module.ID == "" {
			continue
		}
		reason := "semantic module role=" + module.Role
		if len(module.Domains) > 0 {
			reason += " mapped-to=" + strings.Join(module.Domains, ",")
		}
		plan.Decisions = append(plan.Decisions, PlanDecision{Target: "module:" + module.ID, Action: "classify", Reason: reason})
	}
	for _, concern := range semantic.Concerns {
		if concern.Concern == "" {
			continue
		}
		action := "skip"
		if concern.Status == model.StatusObserved || concern.Status == model.StatusExplicitEnabled {
			action = "enable"
		}
		plan.Decisions = append(plan.Decisions, PlanDecision{Target: "concern:" + concern.Concern, Action: action, Reason: "semantic concern status=" + concern.Status + " confidence=" + concern.Confidence})
	}
	for _, unknown := range semantic.Unknowns {
		plan.Decisions = append(plan.Decisions, PlanDecision{Target: unknown.Dimension + ":" + unknown.Subject, Action: "retain-unknown", Reason: unknown.Reason})
	}

	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].Path < plan.Pages[j].Path })
	for i := range plan.Pages {
		plan.Pages[i].Contract = pageContract(cfg, plan.Pages[i], plan.Packs)
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
					Reason:    "domain-based sharding preserves an accepted semantic boundary",
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

func buildSystemPlan(cfg config.Config, units []DocumentationUnit, semantic map[string]model.SemanticDiscovery) *SystemPlan {
	views := []DocumentationView{ViewSystem}
	plan := &SystemPlan{Views: views, SemanticFingerprint: semanticMapFingerprint(semantic)}
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

func packsForSemantic(component config.ComponentConfig, semantic model.SemanticDiscovery) []string {
	set := map[string]bool{}
	for _, pack := range component.Packs {
		if strings.TrimSpace(pack) != "" {
			set[strings.ToLower(strings.TrimSpace(pack))] = true
		}
	}
	for _, concern := range semantic.Concerns {
		if concern.Status == model.StatusObserved || concern.Status == model.StatusExplicitEnabled {
			if concern.Concern != "" {
				set[normalizeConcernPack(concern.Concern)] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for pack := range set {
		out = append(out, pack)
	}
	sort.Strings(out)
	return out
}

func normalizeConcernPack(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, pair := range [][2]string{{"events", "messaging"}, {"interfaces", "api"}, {"apis", "api"}, {"workflows", "workflow"}, {"persistence", "data"}, {"databases", "data"}, {"authorization", "security"}, {"authentication", "security"}, {"observability", "telemetry"}} {
		if value == pair[0] {
			return pair[1]
		}
	}
	return value
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

func semanticFingerprint(semantic model.SemanticDiscovery) string {
	b, _ := json.Marshal(semantic)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func semanticMapFingerprint(semantic map[string]model.SemanticDiscovery) string {
	b, _ := json.Marshal(semantic)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func pageContract(cfg config.Config, page PlannedPage, packs []string) model.PageContract {
	contract := model.PageContract{
		Path:              filepath.ToSlash(page.Path),
		Kind:              page.Kind,
		PathTemplate:      filepath.ToSlash(page.Path),
		IndexPathTemplate: indexPathFor(page.Path, page.Kind),
		Applicability:     model.ApplicabilityRule{Views: []string{string(page.View)}, Packs: append([]string(nil), packs...)},
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
