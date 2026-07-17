package prompts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/example/wikiforge/internal/model"
)

type Profile struct {
	ID             string
	DisplayName    string
	Description    string
	TargetNoun     string
	MinimumPages   int
	MinimumMermaid int
	Phases         []model.Phase
}

var profiles = map[string]Profile{
	"application": {
		ID: "application", DisplayName: "Deployable Application", TargetNoun: "application component",
		Description:  "A deployable application such as a monolith, microservice, worker, gateway, frontend, or CLI.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("A00"),
			phase("A10", "Overview", "quickstart.md", "Orient a new engineer or coding agent before any code change.", "flowchart", []string{"What This Component Does", "Business or Product Context", "Responsibilities", "Non-Responsibilities", "System Context", "Primary Workflows", "Runtime and Deployment Shape", "Where to Start in the Code", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("A20", "Architecture", "architecture/overview.md", "Explain implementation architecture, dependency direction, entry points, and runtime flow.", "sequenceDiagram", []string{"Architectural Role", "Major Components", "Dependency Direction", "Entry Points", "Primary Runtime Flow", "Background and Asynchronous Processing", "Configuration and Runtime Topology", "Extension Points", "Architectural Constraints", "Failure Boundaries", "Change Risks", "Knowledge Gaps", "Source References"}),
			phase("A30", "Domain behaviour", "domain/behavior.md", "Document observable domain concepts, decisions, states, rules, invariants, and invalid operations.", "any", []string{"Domain Vocabulary", "Core Concepts and Identities", "Responsibilities and Boundaries", "Commands and Decisions", "States and Lifecycles", "Business Rules", "Invariants", "Invalid Operations", "Side Effects", "Concurrency and Duplicate Handling", "Examples", "Edge Cases", "Knowledge Gaps", "Source References"}),
			phase("A40", "Interfaces", "interfaces/contracts.md", "Document every externally meaningful inbound and outbound interface and its semantics.", "sequenceDiagram", []string{"Interface Inventory", "Inbound Interfaces", "Outbound Interfaces", "Synchronous Contracts", "Asynchronous Contracts", "Authentication and Authorization at Boundaries", "Validation and Error Semantics", "Idempotency and Retryability", "Versioning and Compatibility", "Ordering and Delivery Behaviour", "External Dependency Failure Behaviour", "Contract Change Checklist", "Knowledge Gaps", "Source References"}),
			phase("A50", "Data and consistency", "data/consistency.md", "Document data ownership, storage, transactions, concurrency, consistency, and repair behaviour.", "any", []string{"Data Ownership", "Data Model", "Identifiers", "Persistence Boundaries", "Transaction Boundaries", "Consistency Guarantees", "Concurrency Control", "Idempotency and Deduplication", "Caching", "Migration and Compatibility", "Reconciliation and Repair", "Retention and Deletion", "Failure Scenarios", "Knowledge Gaps", "Source References"}),
			phase("A60", "Security and reliability", "reliability/security-operations.md", "Document security boundaries, failure modes, resilience, observability, and recovery evidence.", "flowchart", []string{"Security Boundaries", "Authentication", "Authorization", "Sensitive Data Handling", "Dependency Failure Modes", "Timeouts and Retries", "Backpressure and Load Behaviour", "Degraded Behaviour", "Observability", "Operational Signals", "Recovery and Repair", "Dangerous Operations", "Incident Entry Points", "Known Limitations", "Knowledge Gaps", "Source References"}),
			phase("A70", "Development and change", "development/change-guide.md", "Document build, test, debugging, impact analysis, rollout, and safe modification guidance.", "flowchart", []string{"Local Development", "Build and Run", "Test Strategy", "Test Scope by Change Type", "Fixtures and Test Data", "Debugging Entry Points", "Change Impact Matrix", "Contract Changes", "Data and Migration Changes", "Behaviour and Workflow Changes", "Configuration and Deployment Changes", "Safe Rollout and Rollback Evidence", "Review Checklist", "Known Test Gaps", "Knowledge Gaps", "Source References"}),
			consolidatePhase("A90"),
		},
	},
	"modular-application": {
		ID: "modular-application", DisplayName: "Modular Monolith", TargetNoun: "modular application",
		Description:  "A single deployable application with explicit internal modules and dependency boundaries.",
		MinimumPages: 9, MinimumMermaid: 6,
		Phases: []model.Phase{
			initPhase("M00"),
			phase("M10", "Overview", "quickstart.md", "Orient a new engineer to the deployable application and its module structure.", "flowchart", []string{"What This Component Does", "Business or Product Context", "Deployment Boundary", "Module Summary", "Primary Workflows", "Runtime and Deployment Shape", "Where to Start in the Code", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("M20", "Architecture", "architecture/overview.md", "Explain the application shell, runtime entry points, shared mechanisms, and module integration.", "sequenceDiagram", []string{"Architectural Role", "Application Shell", "Shared Runtime Components", "Entry Points", "Primary Runtime Flow", "Configuration and Runtime Topology", "Architectural Constraints", "Failure Boundaries", "Change Risks", "Knowledge Gaps", "Source References"}),
			phase("M25", "Module catalog", "modules/catalog.md", "Catalog each internal module, its responsibility, public surface, owned concepts, and source location.", "flowchart", []string{"Module Inventory", "Module Responsibilities", "Public Module Surfaces", "Owned Domain Concepts", "Owned Data", "Shared Modules", "Module Entry Points", "Module Test Boundaries", "Knowledge Gaps", "Source References"}),
			phase("M27", "Module dependency rules", "modules/dependency-rules.md", "Document allowed and forbidden module dependencies, communication mechanisms, and boundary violations.", "flowchart", []string{"Dependency Principles", "Allowed Dependencies", "Forbidden Dependencies", "Cross-Module Communication", "Shared Code Policy", "Data Access Boundaries", "Observed Boundary Violations", "Refactoring Risks", "Knowledge Gaps", "Source References"}),
			phase("M30", "Domain behaviour", "domain/behavior.md", "Document domain behaviour and show which module owns each rule or lifecycle.", "any", []string{"Domain Vocabulary", "Core Concepts and Identities", "Module Ownership", "Commands and Decisions", "States and Lifecycles", "Business Rules", "Invariants", "Invalid Operations", "Cross-Module Side Effects", "Concurrency and Duplicate Handling", "Examples", "Edge Cases", "Knowledge Gaps", "Source References"}),
			phase("M40", "Interfaces", "interfaces/contracts.md", "Document external interfaces and public module interfaces without exposing internal implementation as contracts.", "sequenceDiagram", []string{"External Interface Inventory", "Public Module Interface Inventory", "Inbound Interfaces", "Outbound Interfaces", "Cross-Module Contracts", "Validation and Error Semantics", "Idempotency and Retryability", "Versioning and Compatibility", "Contract Change Checklist", "Knowledge Gaps", "Source References"}),
			phase("M50", "Data and consistency", "data/consistency.md", "Document module data ownership, shared database risks, transaction boundaries, and consistency.", "erDiagram", []string{"Data Ownership by Module", "Data Model", "Shared Database Boundaries", "Transaction Boundaries", "Cross-Module Consistency", "Concurrency Control", "Migration Ownership", "Reconciliation and Repair", "Failure Scenarios", "Knowledge Gaps", "Source References"}),
			phase("M70", "Development and change", "development/change-guide.md", "Document build, module-level testing, dependency checks, and safe cross-module changes.", "flowchart", []string{"Local Development", "Build and Run", "Module Test Strategy", "Architecture and Dependency Tests", "Debugging Entry Points", "Change Impact Matrix", "Cross-Module Changes", "Data and Migration Changes", "Deployment and Rollback Evidence", "Review Checklist", "Known Test Gaps", "Knowledge Gaps", "Source References"}),
			consolidatePhase("M90"),
		},
	},
	"reusable": {
		ID: "reusable", DisplayName: "Reusable Library or Framework", TargetNoun: "reusable component",
		Description:  "A library, SDK, shared package, internal framework, or reusable foundation consumed by other repositories.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("R00"),
			phase("R10", "Overview", "quickstart.md", "Orient consumers and maintainers to the reusable component, its purpose, supported use cases, and limits.", "flowchart", []string{"What This Component Provides", "Intended Consumers", "Supported Use Cases", "Non-Goals", "Core Abstractions", "Quick Usage Path", "Where to Start in the Code", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("R20", "Design and lifecycle", "architecture/design.md", "Explain internal design, lifecycle, dependency direction, extension model, and runtime behaviour.", "sequenceDiagram", []string{"Design Goals", "Major Components", "Dependency Direction", "Object or Runtime Lifecycle", "Extension Points", "Configuration Flow", "Threading and Concurrency", "Resource Management", "Failure Behaviour", "Design Constraints", "Knowledge Gaps", "Source References"}),
			phase("R30", "Public API", "api/public-api.md", "Document the supported public API and distinguish it from internal implementation details.", "classDiagram", []string{"Public API Inventory", "Core Types and Functions", "Input and Output Semantics", "Error Model", "Lifecycle Requirements", "Thread Safety", "Resource Ownership", "Deprecated API", "Internal API Boundaries", "API Change Checklist", "Knowledge Gaps", "Source References"}),
			phase("R40", "Usage and extension", "integration/usage-and-extension.md", "Document installation, integration, extension points, hooks, adapters, and consumer responsibilities.", "sequenceDiagram", []string{"Installation and Adoption", "Basic Usage", "Integration Patterns", "Extension Points", "Hooks and Callbacks", "Consumer Responsibilities", "Failure Handling", "Examples", "Anti-Patterns", "Knowledge Gaps", "Source References"}),
			phase("R50", "Configuration", "configuration/configuration.md", "Document configuration sources, defaults, precedence, validation, and compatibility.", "flowchart", []string{"Configuration Inventory", "Configuration Sources", "Defaults", "Precedence", "Validation", "Runtime Changes", "Sensitive Configuration", "Compatibility", "Misconfiguration Failure Modes", "Knowledge Gaps", "Source References"}),
			phase("R60", "Compatibility and release", "compatibility/versioning.md", "Document compatibility promises, dependency constraints, versioning, migration, and release evidence.", "flowchart", []string{"Compatibility Policy", "Supported Environments", "Dependency Constraints", "Versioning Model", "Breaking Change Criteria", "Deprecation Process", "Migration Guidance", "Release Process", "Consumer Validation", "Known Compatibility Risks", "Knowledge Gaps", "Source References"}),
			phase("R70", "Contribution guide", "development/contribution-guide.md", "Document build, tests, fixtures, contribution workflow, and safe changes for maintainers.", "flowchart", []string{"Local Development", "Build", "Test Strategy", "Compatibility Tests", "Examples and Fixtures", "Debugging", "Change Impact Matrix", "Documentation and Release Updates", "Review Checklist", "Known Test Gaps", "Knowledge Gaps", "Source References"}),
			consolidatePhase("R90"),
		},
	},
	"infrastructure": {
		ID: "infrastructure", DisplayName: "Infrastructure, IaC, GitOps, or Deployment", TargetNoun: "infrastructure component",
		Description:  "Infrastructure-as-code, GitOps, platform, deployment, environment, or operational automation repository.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("I00"),
			phase("I10", "Overview", "quickstart.md", "Orient operators and engineers to the infrastructure scope, managed environments, and safe entry points.", "flowchart", []string{"What This Repository Manages", "Scope and Non-Scope", "Managed Environments", "Primary Operators and Consumers", "High-Risk Resources", "Execution Entry Points", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("I20", "Topology", "architecture/topology.md", "Document infrastructure topology, boundaries, dependencies, and traffic or control flow.", "flowchart", []string{"Topology Overview", "Environment Boundaries", "Network and Trust Boundaries", "Compute and Runtime Layers", "Data and State Layers", "External Dependencies", "Control Plane Flows", "Data Plane Flows", "Single Points of Failure", "Knowledge Gaps", "Source References"}),
			phase("I30", "Resource inventory", "resources/resource-inventory.md", "Catalog managed resources, ownership evidence, lifecycle, dependencies, and criticality without copying raw declarations.", "flowchart", []string{"Resource Catalog", "Resource Groups and Modules", "Ownership Evidence", "Dependency Relationships", "Stateful Resources", "Shared Resources", "Externally Managed Resources", "Lifecycle and Destruction Risks", "Drift Risks", "Knowledge Gaps", "Source References"}),
			phase("I40", "Environment model", "environments/environment-model.md", "Document environment differences, promotion, variable sources, isolation, and drift controls.", "flowchart", []string{"Environment Inventory", "Environment Differences", "Isolation Boundaries", "Configuration and Variable Sources", "Promotion Model", "State and Locking", "Drift Detection", "Manual Steps", "Environment-Specific Risks", "Knowledge Gaps", "Source References"}),
			phase("I50", "Delivery pipeline", "delivery/change-pipeline.md", "Document planning, validation, approval, apply/deploy, rollback, and change safety.", "sequenceDiagram", []string{"Change Entry Points", "Planning and Preview", "Validation", "Approval Gates", "Apply or Deployment", "Ordering and Dependencies", "Rollback and Roll-Forward", "Emergency Changes", "Change Verification", "Knowledge Gaps", "Source References"}),
			phase("I60", "Security controls", "security/security-controls.md", "Document identity, permissions, secrets, trust boundaries, policy enforcement, and dangerous access.", "flowchart", []string{"Identity and Access Model", "Execution Identities", "Permission Boundaries", "Secrets and Sensitive Values", "Policy Enforcement", "Network Security", "Supply Chain Controls", "Audit Evidence", "Dangerous Permissions", "Knowledge Gaps", "Source References"}),
			phase("I70", "Operations and recovery", "operations/operations-and-recovery.md", "Document observability, failure modes, state recovery, drift repair, disaster recovery evidence, and unsafe actions.", "flowchart", []string{"Operational Signals", "Common Failure Modes", "State and Lock Recovery", "Drift Repair", "Dependency Failures", "Backup and Restore Evidence", "Disaster Recovery Evidence", "Dangerous Operations", "Incident Entry Points", "Validation After Recovery", "Knowledge Gaps", "Source References"}),
			consolidatePhase("I90"),
		},
	},
	"configuration": {
		ID: "configuration", DisplayName: "Configuration Repository", TargetNoun: "configuration component",
		Description:  "A repository primarily containing shared configuration, policy, templates, manifests, feature definitions, or environment values.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("C00"),
			phase("C10", "Overview", "quickstart.md", "Explain what is configured, who consumes it, and how to make safe changes.", "flowchart", []string{"What This Repository Configures", "Consumers", "Scope and Non-Scope", "Configuration Categories", "Change and Promotion Path", "Where to Start", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("C20", "Configuration model", "configuration/model.md", "Document configuration concepts, hierarchy, naming, defaults, and relationships.", "classDiagram", []string{"Configuration Concepts", "Hierarchy and Composition", "Naming and Identity", "Defaults", "Overrides", "Environment Dimensions", "Generated and Handwritten Content", "Configuration Relationships", "Knowledge Gaps", "Source References"}),
			phase("C30", "Sources and precedence", "configuration/sources-and-precedence.md", "Document all configuration sources, merge order, precedence, and runtime resolution.", "flowchart", []string{"Source Inventory", "Resolution Flow", "Precedence Rules", "Merge Behaviour", "Environment Overrides", "Runtime Overrides", "Missing Value Behaviour", "Conflicting Value Behaviour", "Knowledge Gaps", "Source References"}),
			phase("C40", "Schema and validation", "configuration/schema-and-validation.md", "Document schemas, validation, linting, generation, and invalid configuration handling.", "flowchart", []string{"Schema Inventory", "Validation Rules", "Static Validation", "Runtime Validation", "Generation and Transformation", "Invalid Configuration Behaviour", "Backward Compatibility", "Test Coverage", "Knowledge Gaps", "Source References"}),
			phase("C50", "Consumers and compatibility", "configuration/consumers-and-compatibility.md", "Document consumers, coupling, rollout ordering, compatibility, and change impact.", "sequenceDiagram", []string{"Consumer Inventory", "Consumption Mechanisms", "Refresh and Reload Behaviour", "Compatibility Expectations", "Rollout Ordering", "Fallback Behaviour", "Consumer Failure Modes", "Change Impact Matrix", "Knowledge Gaps", "Source References"}),
			phase("C60", "Secrets and sensitive values", "security/secrets-and-sensitive-values.md", "Document references to secrets, redaction, access controls, and forbidden content without exposing values.", "flowchart", []string{"Sensitive Value Categories", "Secret References", "Access Boundaries", "Encryption and Protection Evidence", "Redaction and Logging", "Rotation Evidence", "Forbidden Repository Content", "Leak Response Evidence", "Knowledge Gaps", "Source References"}),
			phase("C70", "Change guide", "development/change-guide.md", "Document editing, validation, testing, promotion, rollback, and review for configuration changes.", "flowchart", []string{"Local Editing", "Validation Commands", "Test Strategy", "Preview and Diff", "Promotion", "Rollback", "Consumer Coordination", "Review Checklist", "Known Validation Gaps", "Knowledge Gaps", "Source References"}),
			consolidatePhase("C90"),
		},
	},
	"contracts": {
		ID: "contracts", DisplayName: "Contract or Schema Repository", TargetNoun: "contract component",
		Description:  "A repository containing API, event, message, data, protocol, or interface contracts shared across components.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("K00"),
			phase("K10", "Overview", "quickstart.md", "Orient producers, consumers, and maintainers to the contract catalog and safe change process.", "flowchart", []string{"What This Repository Defines", "Contract Categories", "Producers and Providers", "Consumers", "Publication and Distribution", "Where to Start", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("K20", "Contract catalog", "contracts/catalog.md", "Catalog contracts, ownership evidence, versions, producers, consumers, and canonical files.", "flowchart", []string{"Contract Inventory", "Canonical Sources", "Ownership Evidence", "Producer or Provider Map", "Consumer Map", "Version Inventory", "Deprecated Contracts", "Unknown Consumers", "Knowledge Gaps", "Source References"}),
			phase("K30", "Semantics", "contracts/semantics.md", "Explain business and technical semantics, identifiers, required fields, errors, ordering, and invariants.", "classDiagram", []string{"Shared Vocabulary", "Identifiers", "Request and Response Semantics", "Event or Message Semantics", "Required and Optional Data", "Validation Rules", "Error Semantics", "Ordering and Correlation", "Invariants", "Knowledge Gaps", "Source References"}),
			phase("K40", "Compatibility", "contracts/compatibility.md", "Document compatibility rules, versioning, evolution, deprecation, and migration.", "flowchart", []string{"Compatibility Policy", "Versioning Model", "Backward-Compatible Changes", "Breaking Changes", "Schema Evolution", "Deprecation", "Migration Process", "Compatibility Risks", "Knowledge Gaps", "Source References"}),
			phase("K50", "Consumers and producers", "contracts/consumers-and-producers.md", "Document interaction direction, lifecycle, generation, distribution, and rollout ordering.", "sequenceDiagram", []string{"Producer and Provider Inventory", "Consumer Inventory", "Generation and Publication", "Consumption and Binding", "Rollout Ordering", "Unknown or Dynamic Consumers", "Failure Behaviour", "Change Coordination", "Knowledge Gaps", "Source References"}),
			phase("K60", "Validation and testing", "contracts/validation-and-testing.md", "Document linting, schema validation, compatibility checks, generated artifacts, and contract tests.", "flowchart", []string{"Validation Toolchain", "Static Checks", "Compatibility Checks", "Contract Tests", "Generated Artifacts", "Fixture Management", "CI Gates", "Known Test Gaps", "Knowledge Gaps", "Source References"}),
			phase("K70", "Change guide", "development/change-guide.md", "Document the evidence-driven workflow for adding, changing, deprecating, or removing a contract.", "flowchart", []string{"Adding a Contract", "Changing a Contract", "Deprecating a Contract", "Removing a Contract", "Consumer Discovery", "Rollout Coordination", "Rollback", "Review Checklist", "Knowledge Gaps", "Source References"}),
			consolidatePhase("K90"),
		},
	},
	"generic": {
		ID: "generic", DisplayName: "Generic Repository", TargetNoun: "repository component",
		Description:  "A language- and technology-neutral fallback for repositories that do not fit another profile.",
		MinimumPages: 8, MinimumMermaid: 5,
		Phases: []model.Phase{
			initPhase("G00"),
			phase("G10", "Overview", "quickstart.md", "Orient a new maintainer to the repository purpose, consumers, outputs, and safe entry points.", "flowchart", []string{"Repository Purpose", "Primary Consumers", "Responsibilities", "Non-Responsibilities", "Key Outputs", "Primary Workflows", "Where to Start", "Safe Change Checklist", "Documentation Map", "Knowledge Gaps", "Source References"}),
			phase("G20", "Architecture and structure", "architecture/overview.md", "Explain structure, dependency direction, execution or generation flow, and boundaries.", "flowchart", []string{"Structural Overview", "Major Areas", "Dependency Direction", "Entry Points", "Primary Flow", "Configuration", "Extension Points", "Failure Boundaries", "Change Risks", "Knowledge Gaps", "Source References"}),
			phase("G30", "Interfaces and outputs", "interfaces/usage.md", "Document public surfaces, inputs, outputs, consumers, and compatibility expectations.", "sequenceDiagram", []string{"Interface Inventory", "Inputs", "Outputs", "Consumers", "Usage Patterns", "Error or Failure Semantics", "Compatibility", "Change Checklist", "Knowledge Gaps", "Source References"}),
			phase("G40", "Behaviour and rules", "behavior/rules.md", "Document observable behaviour, rules, transformations, invariants, and edge cases.", "any", []string{"Core Concepts", "Processing Rules", "Transformations", "Invariants", "Invalid Inputs", "Side Effects", "Concurrency or Ordering", "Examples", "Edge Cases", "Knowledge Gaps", "Source References"}),
			phase("G50", "Configuration", "configuration/configuration.md", "Document configuration sources, defaults, precedence, validation, and sensitive values.", "flowchart", []string{"Configuration Inventory", "Sources", "Defaults", "Precedence", "Validation", "Sensitive Values", "Failure Behaviour", "Compatibility", "Knowledge Gaps", "Source References"}),
			phase("G60", "Risks and operations", "operations/risks.md", "Document failure modes, observability, recovery evidence, dangerous actions, and known limits.", "flowchart", []string{"Failure Modes", "Detection", "Impact", "Containment", "Recovery Evidence", "Dangerous Operations", "Security Considerations", "Known Limitations", "Knowledge Gaps", "Source References"}),
			phase("G70", "Development and change", "development/change-guide.md", "Document build, tests, debugging, impact analysis, and safe change workflow.", "flowchart", []string{"Local Development", "Build or Generation", "Test Strategy", "Debugging", "Change Impact Matrix", "Compatibility Changes", "Release or Distribution", "Review Checklist", "Known Test Gaps", "Knowledge Gaps", "Source References"}),
			consolidatePhase("G90"),
		},
	},
}

func initPhase(id string) model.Phase {
	return model.Phase{ID: id, Name: "Bootstrap OpenWiki and quickstart", PromptAsset: "prompts/component/initialize.md", Initialize: true}
}

func phase(id, name, output, objective, diagram string, headings []string) model.Phase {
	return model.Phase{ID: id, Name: name, PromptAsset: "prompts/component/phase.md", OutputFile: output, Objective: objective, RequiredDiagram: diagram, RequiredHeadings: headings}
}

func supplementalPhase(id string, index, total int, pages []model.PageContract) model.Phase {
	name := "Specialized catalogs"
	if total > 1 {
		name = fmt.Sprintf("Specialized catalogs %d/%d", index, total)
	}
	return model.Phase{
		ID:            id,
		Name:          name,
		PromptAsset:   "prompts/component/supplemental.md",
		Objective:     "Generate an evidence-backed batch of specialized documentation catalogs for this profile.",
		PageContracts: append([]model.PageContract(nil), pages...),
	}
}

func consolidatePhase(id string) model.Phase {
	return model.Phase{ID: id, Name: "Consolidate knowledge", PromptAsset: "prompts/component/consolidate.md", OutputFile: "knowledge/relationships.md", Objective: "Perform a final consistency, navigation, terminology, evidence, diagram, and relationship audit.", RequiredHeadings: []string{"Concept Index", "Relationship Table", "Traceability Paths", "Canonical Terminology", "Contradictions", "Knowledge Gaps", "Source References"}}
}

func GetProfile(id string) (Profile, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	profile, ok := profiles[id]
	if !ok {
		return Profile{}, fmt.Errorf("unknown documentation profile %q", id)
	}
	profile.Phases = append([]model.Phase(nil), profile.Phases...)
	pages := SupplementalPages(profile.ID)
	if len(pages) > 0 {
		insertAt := len(profile.Phases)
		if insertAt > 0 && profile.Phases[insertAt-1].OutputFile == "knowledge/relationships.md" {
			insertAt--
		}
		prefix := map[string]string{
			"application": "A", "modular-application": "M", "reusable": "R",
			"infrastructure": "I", "configuration": "C", "contracts": "K", "generic": "G",
		}[profile.ID]
		batches := batchPageContracts(pages, 4)
		insert := make([]model.Phase, 0, len(batches))
		for i, batch := range batches {
			insert = append(insert, supplementalPhase(fmt.Sprintf("%sS%02d", prefix, i+1), i+1, len(batches), batch))
		}
		if insertAt < len(profile.Phases) && profile.Phases[insertAt].OutputFile == "knowledge/relationships.md" {
			profile.Phases[insertAt].ID = prefix + "C90"
		}
		profile.Phases = append(profile.Phases[:insertAt], append(insert, profile.Phases[insertAt:]...)...)
	}
	return profile, nil
}

func batchPageContracts(pages []model.PageContract, size int) [][]model.PageContract {
	if size < 1 {
		size = 1
	}
	var batches [][]model.PageContract
	for start := 0; start < len(pages); start += size {
		end := start + size
		if end > len(pages) {
			end = len(pages)
		}
		batches = append(batches, append([]model.PageContract(nil), pages[start:end]...))
	}
	return batches
}

func ProfileIDs() []string {
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func ExpectedFiles(profile Profile) []string {
	contracts := ComponentPageContracts(profile)
	return sortedContractPaths(contracts)
}

func CanonicalFilesText(profile Profile) string {
	var b strings.Builder
	for _, path := range ExpectedFiles(profile) {
		fmt.Fprintf(&b, "- openwiki/%s\n", path)
	}
	return strings.TrimRight(b.String(), "\n")
}
