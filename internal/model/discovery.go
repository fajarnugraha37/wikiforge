package model

const (
	DiscoverySchemaVersion = 1
	StatusObserved         = "observed"
	StatusNotObserved      = "not-observed"
	StatusUncertain        = "uncertain"
	StatusConflicting      = "conflicting"
	StatusExplicitEnabled  = "explicitly-enabled"
	StatusExplicitDisabled = "explicitly-disabled"
	StatusNotApplicable    = "not-applicable"
	StatusUnknown          = "unknown"
)

type EvidenceLocator struct {
	Symbol    string `json:"symbol,omitempty"`
	LineStart int    `json:"lineStart,omitempty"`
	LineEnd   int    `json:"lineEnd,omitempty"`
}
type EvidenceReference struct {
	ID          string          `json:"id"`
	Path        string          `json:"path"`
	ContentHash string          `json:"contentHash"`
	Kind        string          `json:"kind"`
	Locator     EvidenceLocator `json:"locator,omitempty"`
}
type SemanticCandidate struct {
	CandidateKey string   `json:"candidateKey"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	EvidenceIDs  []string `json:"evidenceIds"`
}
type FindingBase struct {
	ID          string            `json:"id,omitempty"`
	Candidate   SemanticCandidate `json:"candidate"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status"`
	Confidence  string            `json:"confidence"`
	Source      string            `json:"source,omitempty"`
	EvidenceIDs []string          `json:"evidenceIds,omitempty"`
	Provenance  string            `json:"provenance,omitempty"`
}
type RepositoryFinding struct {
	Profile     string   `json:"profile"`
	Description string   `json:"description,omitempty"`
	Confidence  string   `json:"confidence"`
	Status      string   `json:"status"`
	EvidenceIDs []string `json:"evidenceIds,omitempty"`
}
type ModuleFinding struct {
	FindingBase
	Role        string   `json:"role"`
	SourceRoots []string `json:"sourceRoots,omitempty"`
	Domains     []string `json:"domains,omitempty"`
}
type DomainFinding struct {
	FindingBase
	Subdomain      string   `json:"subdomain,omitempty"`
	BoundedContext string   `json:"boundedContext,omitempty"`
	ModuleIDs      []string `json:"moduleIds,omitempty"`
	SourceRoots    []string `json:"sourceRoots,omitempty"`
	Owners         []string `json:"owners,omitempty"`
	Criticality    string   `json:"criticality,omitempty"`
}
type FlowFinding struct {
	FindingBase
	Triggers    []string `json:"triggers,omitempty"`
	Actors      []string `json:"actors,omitempty"`
	States      []string `json:"states,omitempty"`
	ModuleIDs   []string `json:"moduleIds,omitempty"`
	SourceRoots []string `json:"sourceRoots,omitempty"`
}
type ConcernFinding struct {
	FindingBase
	Concern string `json:"concern"`
}
type OwnershipFinding struct {
	FindingBase
	SubjectID string   `json:"subjectId"`
	Owners    []string `json:"owners,omitempty"`
}
type RelationshipFinding struct {
	FindingBase
	FromID string `json:"fromId"`
	ToID   string `json:"toId"`
	Kind   string `json:"kind"`
}
type ConflictFinding struct {
	ID          string   `json:"id"`
	Dimension   string   `json:"dimension"`
	SubjectIDs  []string `json:"subjectIds"`
	Message     string   `json:"message"`
	Description string   `json:"description,omitempty"`
	EvidenceIDs []string `json:"evidenceIds,omitempty"`
}
type UnknownFinding struct {
	Candidate   *SemanticCandidate `json:"candidate,omitempty"`
	Dimension   string             `json:"dimension"`
	Subject     string             `json:"subject"`
	Status      string             `json:"status"`
	Confidence  string             `json:"confidence,omitempty"`
	Source      string             `json:"source,omitempty"`
	Description string             `json:"description,omitempty"`
	Reason      string             `json:"reason"`
	EvidenceIDs []string           `json:"evidenceIds,omitempty"`
}
type SemanticDiscovery struct {
	SchemaVersion        int                   `json:"schemaVersion"`
	ComponentID          string                `json:"componentId"`
	RepositoryID         string                `json:"repositoryId"`
	SourceRevision       string                `json:"sourceRevision"`
	DiscoveryMode        string                `json:"discoveryMode"`
	InventoryFingerprint string                `json:"inventoryFingerprint"`
	CacheFingerprint     string                `json:"cacheFingerprint"`
	InventoryVersion     string                `json:"inventoryVersion"`
	PromptVersion        string                `json:"promptVersion"`
	ModelID              string                `json:"modelId,omitempty"`
	Repository           RepositoryFinding     `json:"repository"`
	Modules              []ModuleFinding       `json:"modules,omitempty"`
	Domains              []DomainFinding       `json:"domains,omitempty"`
	Flows                []FlowFinding         `json:"flows,omitempty"`
	Concerns             []ConcernFinding      `json:"concerns,omitempty"`
	Ownership            []OwnershipFinding    `json:"ownership,omitempty"`
	Relationships        []RelationshipFinding `json:"relationships,omitempty"`
	Conflicts            []ConflictFinding     `json:"conflicts,omitempty"`
	Unknowns             []UnknownFinding      `json:"unknowns,omitempty"`
	Quality              QualityResult         `json:"quality"`
}
type QualityResult struct {
	Accepted         bool `json:"accepted"`
	AcceptedCount    int  `json:"acceptedCount"`
	UncertainCount   int  `json:"uncertainCount"`
	ConflictingCount int  `json:"conflictingCount"`
	UnknownCount     int  `json:"unknownCount"`
}
type DiscoveryStageMetric struct {
	Name           string `json:"name"`
	Batches        int    `json:"batches"`
	Calls          int    `json:"calls"`
	Retries        int    `json:"retries"`
	DurationMillis int64  `json:"durationMillis"`
	CacheHit       bool   `json:"cacheHit"`
}
type DiscoveryCounts struct {
	Modules       int `json:"modules"`
	Domains       int `json:"domains"`
	Flows         int `json:"flows"`
	Concerns      int `json:"concerns"`
	Ownership     int `json:"ownership"`
	Relationships int `json:"relationships"`
}
type IdentityMapping struct {
	CandidateKey string   `json:"candidateKey"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	EvidenceIDs  []string `json:"evidenceIds"`
	Precedence   string   `json:"precedence"`
	Accepted     bool     `json:"accepted"`
}
type IdentityManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	ComponentID   string            `json:"componentId"`
	Mappings      []IdentityMapping `json:"mappings"`
}
type StageOutput struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Stage         string                `json:"stage"`
	Source        string                `json:"source,omitempty"`
	Repository    RepositoryFinding     `json:"repository"`
	Modules       []ModuleFinding       `json:"modules,omitempty"`
	Domains       []DomainFinding       `json:"domains,omitempty"`
	Flows         []FlowFinding         `json:"flows,omitempty"`
	Concerns      []ConcernFinding      `json:"concerns,omitempty"`
	Ownership     []OwnershipFinding    `json:"ownership,omitempty"`
	Relationships []RelationshipFinding `json:"relationships,omitempty"`
	Conflicts     []ConflictFinding     `json:"conflicts,omitempty"`
	Unknowns      []UnknownFinding      `json:"unknowns,omitempty"`
}
