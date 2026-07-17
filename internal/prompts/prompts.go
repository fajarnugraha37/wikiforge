package prompts

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/example/wikiforge/internal/assets"
	"github.com/example/wikiforge/internal/config"
	"github.com/example/wikiforge/internal/model"
)

var SystemPhases = buildSystemPhases()

func buildSystemPhases() []model.Phase {
	phases := []model.Phase{
		{ID: "W00", Name: "Bootstrap system OpenWiki", PromptAsset: "prompts/system/00-initialize.md", Initialize: true},
		{ID: "W05", Name: "System quickstart", PromptAsset: "prompts/system/05-quickstart.md", OutputFile: "quickstart.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"System at a Glance", "Component Categories", "Critical Journeys", "Reading Paths", "High-Risk Knowledge Gaps", "Documentation Map", "Source References"}},
		{ID: "W10", Name: "System landscape", PromptAsset: "prompts/system/10-system-overview.md", OutputFile: "system/landscape.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"System Purpose and Scope", "Business and Technical Capabilities", "Actors and Boundaries", "Component Categories", "Critical Journeys", "Runtime and Deployment Shape", "Repository and Ownership Evidence", "Knowledge Gaps", "Source References"}},
		{ID: "W20", Name: "Component map", PromptAsset: "prompts/system/20-component-map.md", OutputFile: "system/component-map.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"Component Catalog", "Deployable Components", "Modular Applications", "Libraries and Frameworks", "Infrastructure and Configuration Components", "Contract and Schema Components", "Dependency Relationships", "External Systems", "Shared Foundations", "Dependency Risks", "Contradictions", "Knowledge Gaps", "Source References"}},
		{ID: "W30", Name: "Cross-component flows", PromptAsset: "prompts/system/30-cross-component-flows.md", OutputFile: "system/cross-component-flows.md", RequiredDiagram: "sequenceDiagram", RequiredHeadings: []string{"Journey Inventory", "Primary Journey", "Alternate Paths", "Failure and Compensation Paths", "Correlation and Identity Propagation", "Application and Infrastructure Boundaries", "Transaction and Consistency Boundaries", "Operational Checkpoints", "Change Impact", "Knowledge Gaps", "Source References"}},
		{ID: "W40", Name: "Data, events, and contracts", PromptAsset: "prompts/system/40-data-events-contracts.md", OutputFile: "system/data-events-contracts.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"System-of-Record Map", "Data Ownership", "Shared Identifiers", "Data Movement", "Event and Message Catalog", "API and Contract Catalog", "Ordering and Delivery Boundaries", "Cross-Component Consistency", "Schema and Contract Evolution", "Replay and Reconciliation Impact", "Knowledge Gaps", "Source References"}},
		{ID: "W45", Name: "Infrastructure and deployment", PromptAsset: "prompts/system/45-infrastructure-deployment.md", OutputFile: "system/infrastructure-deployment.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"Infrastructure Component Inventory", "Environment Model", "Runtime Topology", "Network and Trust Boundaries", "Deployment and Promotion", "Shared Configuration", "Stateful Infrastructure", "Operational Dependencies", "Drift and Configuration Risks", "Knowledge Gaps", "Source References"}},
		{ID: "W50", Name: "Failure, security, and operations", PromptAsset: "prompts/system/50-failure-and-security.md", OutputFile: "system/failure-security-operations.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"Trust Boundaries", "Identity and Authorization Flow", "Critical Dependencies", "Failure Propagation", "Degraded Modes", "Detection and Observability", "Recovery Dependencies", "Operational Ownership Evidence", "Systemic Risks", "Dangerous Gaps", "Knowledge Gaps", "Source References"}},
	}
	batches := batchPageContracts(SystemSupplementalPages, 4)
	for i, batch := range batches {
		phases = append(phases, model.Phase{
			ID:            fmt.Sprintf("WS%02d", i+1),
			Name:          fmt.Sprintf("System specialized catalogs %d/%d", i+1, len(batches)),
			PromptAsset:   "prompts/system/55-specialized-catalogs.md",
			Objective:     "Generate a batch of system-level specialized catalogs from component documentation.",
			PageContracts: append([]model.PageContract(nil), batch...),
		})
	}
	phases = append(phases,
		model.Phase{ID: "WO60", Name: "Onboarding and change", PromptAsset: "prompts/system/60-onboarding-change.md", OutputFile: "system/onboarding-change-guide.md", RequiredDiagram: "flowchart", RequiredHeadings: []string{"Recommended Reading Order", "Repository and Component Map", "How to Trace a Requirement", "How to Trace a Runtime Failure", "How to Change an Application Contract", "How to Change a Library or Framework", "How to Change Infrastructure or Configuration", "How to Change an Event, Message, or Schema", "How to Add or Remove a Component", "Cross-Component Test Strategy", "Release and Rollback Evidence", "Review Checklist", "Knowledge Gaps", "Source References"}},
		model.Phase{ID: "WC90", Name: "Consolidate system wiki", PromptAsset: "prompts/system/90-consolidate.md", OutputFile: "knowledge/relationships.md", RequiredHeadings: []string{"Entity Index", "Relationship Table", "Capability-to-Component Traceability", "Component-to-Contract Traceability", "Application-to-Infrastructure Traceability", "Data and Event Traceability", "Failure and Recovery Traceability", "Canonical Terminology", "Contradictions", "Knowledge Gaps", "Source References"}},
	)
	return phases
}

func RenderComponentPhase(phase model.Phase, profile Profile, component config.ComponentConfig, language string, values map[string]string) (string, error) {
	supplemental := SupplementalPages(profile.ID)
	if len(phase.PageContracts) > 0 {
		supplemental = phase.PageContracts
	}
	common := map[string]string{
		"PROFILE_ID":             profile.ID,
		"PROFILE_NAME":           profile.DisplayName,
		"PROFILE_DESCRIPTION":    profile.Description,
		"TARGET_NOUN":            profile.TargetNoun,
		"COMPONENT_TYPE":         component.Type,
		"REPOSITORY":             component.Repository,
		"SCOPE":                  displayScope(component.Scope),
		"CANONICAL_FILES":        CanonicalFilesText(profile),
		"OUTPUT_FILE":            phase.OutputFile,
		"OBJECTIVE":              phase.Objective,
		"REQUIRED_HEADINGS":      headingsText(phase.RequiredHeadings),
		"DIAGRAM_CONTRACT":       diagramContract(phase.RequiredDiagram),
		"GUIDANCE":               profileGuidance(profile.ID, component.Type),
		"SUPPLEMENTAL_CONTRACTS": supplementalContractsText(supplemental),
	}
	for k, v := range values {
		common[k] = v
	}
	return Render(phase.PromptAsset, language, component.ID, common)
}

func RenderSystemPhase(phase model.Phase, language, targetID string) (string, error) {
	supplemental := SystemSupplementalPages
	if len(phase.PageContracts) > 0 {
		supplemental = phase.PageContracts
	}
	return Render(phase.PromptAsset, language, targetID, map[string]string{
		"SYSTEM_CANONICAL_FILES":        systemCanonicalFilesText(),
		"SYSTEM_SUPPLEMENTAL_CONTRACTS": supplementalContractsText(supplemental),
	})
}

func systemCanonicalFilesText() string {
	var b strings.Builder
	for _, path := range ExpectedSystemFiles() {
		fmt.Fprintf(&b, "- openwiki/%s\n", path)
	}
	return strings.TrimRight(b.String(), "\n")
}

func RenderComponentUpdate(profile Profile, component config.ComponentConfig, language string) (string, error) {
	return RenderComponentPhase(model.Phase{PromptAsset: "prompts/component/update.md"}, profile, component, language, nil)
}

func RenderSystemUpdate(language, targetID string) (string, error) {
	return RenderSystemPhase(model.Phase{PromptAsset: "prompts/system/update.md"}, language, targetID)
}

func RenderInstructions(profile Profile, component config.ComponentConfig, language string) (string, error) {
	return RenderTemplateValues("templates/instructions.md", language, component.ID, map[string]string{
		"PROFILE_ID":          profile.ID,
		"PROFILE_NAME":        profile.DisplayName,
		"PROFILE_DESCRIPTION": profile.Description,
		"COMPONENT_TYPE":      component.Type,
		"REPOSITORY":          component.Repository,
		"SCOPE":               displayScope(component.Scope),
		"CANONICAL_FILES":     CanonicalFilesText(profile),
		"GUIDANCE":            profileGuidance(profile.ID, component.Type),
	})
}

func Render(assetPath, language, targetID string, values map[string]string) (string, error) {
	baseBytes, err := fs.ReadFile(assets.FS, "prompts/common/base.md")
	if err != nil {
		return "", err
	}
	bodyBytes, err := fs.ReadFile(assets.FS, assetPath)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", assetPath, err)
	}
	base := replace(string(baseBytes), language, targetID, values)
	bodyValues := map[string]string{"BASE": base}
	for k, v := range values {
		bodyValues[k] = v
	}
	return replace(string(bodyBytes), language, targetID, bodyValues), nil
}

func RenderTemplate(assetPath, language, targetID string) (string, error) {
	return RenderTemplateValues(assetPath, language, targetID, nil)
}

func RenderTemplateValues(assetPath, language, targetID string, values map[string]string) (string, error) {
	b, err := fs.ReadFile(assets.FS, assetPath)
	if err != nil {
		return "", err
	}
	return replace(string(b), language, targetID, values), nil
}

func replace(s, language, targetID string, values map[string]string) string {
	s = strings.ReplaceAll(s, "{{LANGUAGE}}", language)
	s = strings.ReplaceAll(s, "{{TARGET_ID}}", targetID)
	for k, v := range values {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

func headingsText(headings []string) string {
	var b strings.Builder
	for _, heading := range headings {
		fmt.Fprintf(&b, "## %s\n", heading)
	}
	return strings.TrimRight(b.String(), "\n")
}

func diagramContract(required string) string {
	switch required {
	case "":
		return "A Mermaid diagram is optional. Add one only when it materially improves understanding and is supported by evidence."
	case "any":
		return "Include at least one readable evidence-backed Mermaid diagram using the most suitable allowed diagram type. Explain it in prose."
	case "flowchart":
		return "Include at least one readable evidence-backed Mermaid `flowchart` and explain it in prose."
	default:
		return fmt.Sprintf("Include at least one readable evidence-backed Mermaid `%s` diagram and explain it in prose.", required)
	}
}

func displayScope(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return "repository root"
	}
	return scope
}

func profileGuidance(profile, componentType string) string {
	switch profile {
	case "application":
		return "Treat this as a deployable application boundary. Describe only behaviours and operational concerns that repository evidence supports. Stateless components may explicitly state that no owned persistence was found."
	case "modular-application":
		return "Treat the deployment as one runtime boundary while preserving explicit internal module boundaries. Distinguish public module surfaces from internal implementation and surface illegal dependencies or shared-data coupling."
	case "reusable":
		return "Prioritize consumer-facing API, lifecycle, extension points, compatibility, thread safety, dependency constraints, migration, and contribution guidance. Do not fabricate business workflows, production operations, or data ownership."
	case "infrastructure":
		return "Prioritize managed resources, topology, environments, state, drift, permissions, promotion, rollback, recovery, and destructive-change risks. Never expose secret values."
	case "configuration":
		return "Prioritize configuration semantics, consumers, precedence, validation, compatibility, promotion, rollback, and sensitive-value handling. Do not copy secret values."
	case "contracts":
		return "Prioritize canonical contracts, semantics, producers/providers, consumers, compatibility, versioning, validation, generation, distribution, and rollout coordination."
	default:
		return "Use repository evidence to describe its actual purpose and outputs without forcing application-specific concepts when they do not apply."
	}
}

func ExpectedSystemFiles() []string {
	return sortedContractPaths(SystemPageContracts())
}
