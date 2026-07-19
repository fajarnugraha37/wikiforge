package model

import "time"

type Component struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Profile    string   `json:"profile"`
	Repository string   `json:"repository"`
	Scope      string   `json:"scope,omitempty"`
	WorkDir    string   `json:"workDir"`
	Group      string   `json:"group,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
}

type PageContract struct {
	Path                string            `json:"path"`
	Kind                PageKind          `json:"kind"`
	PathTemplate        string            `json:"pathTemplate,omitempty"`
	IndexPathTemplate   string            `json:"indexPathTemplate,omitempty"`
	Applicability       ApplicabilityRule `json:"applicability,omitempty"`
	ShardDimensions     []ShardDimension  `json:"shardDimensions,omitempty"`
	ShardKey            string            `json:"shardKey,omitempty"`
	OwnershipPartition  bool              `json:"ownershipPartition,omitempty"`
	Objective           string            `json:"objective,omitempty"`
	RequiredHeadings    []string          `json:"requiredHeadings,omitempty"`
	RequiredDiagram     string            `json:"requiredDiagram,omitempty"`
	RequiredTableHeader string            `json:"requiredTableHeader,omitempty"`
}

type PageKind string

const (
	PageSingle     PageKind = "single"
	PageIndex      PageKind = "index"
	PageCollection PageKind = "collection"
	PageShard      PageKind = "shard"
)

type ShardDimension string

const (
	ShardDomain      ShardDimension = "domain"
	ShardSubdomain   ShardDimension = "subdomain"
	ShardBoundedCtx  ShardDimension = "bounded-context"
	ShardComponent   ShardDimension = "component"
	ShardOwner       ShardDimension = "owner"
	ShardRepository  ShardDimension = "repository"
	ShardRuntime     ShardDimension = "runtime"
	ShardTransport   ShardDimension = "transport"
	ShardDataStore   ShardDimension = "data-store"
	ShardCriticality ShardDimension = "criticality"
)

type ApplicabilityRule struct {
	Views        []string `json:"views,omitempty"`
	Packs        []string `json:"packs,omitempty"`
	MinimumUnits int      `json:"minimumUnits,omitempty"`
}

type Phase struct {
	ID               string
	Name             string
	PromptAsset      string
	OutputFile       string
	Objective        string
	Guidance         string
	RequiredHeadings []string
	RequiredDiagram  string
	Initialize       bool
	PageContracts    []PageContract
}

type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type ValidationResult struct {
	Root          string                     `json:"root"`
	Profile       string                     `json:"profile,omitempty"`
	Score         int                        `json:"score"`
	Accepted      bool                       `json:"accepted"`
	MarkdownFiles int                        `json:"markdownFiles"`
	MermaidBlocks int                        `json:"mermaidBlocks"`
	Findings      []Finding                  `json:"findings"`
	Dimensions    map[string]DimensionResult `json:"dimensions,omitempty"`
}

type DimensionResult struct {
	Score        int      `json:"score"`
	FindingCodes []string `json:"findingCodes,omitempty"`
}

type PhaseStatus struct {
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type TargetState struct {
	GitHead                   string                 `json:"gitHead,omitempty"`
	DocsHash                  string                 `json:"docsHash,omitempty"`
	SourceHash                string                 `json:"sourceHash,omitempty"`
	PlanHash                  string                 `json:"planHash,omitempty"`
	EvidenceRevision          string                 `json:"evidenceRevision,omitempty"`
	EvidenceIndexPath         string                 `json:"evidenceIndexPath,omitempty"`
	ImpactIndexPath           string                 `json:"impactIndexPath,omitempty"`
	CoveragePath              string                 `json:"coveragePath,omitempty"`
	SnapshotHash              string                 `json:"snapshotHash,omitempty"`
	DiscoveryMode             string                 `json:"discoveryMode,omitempty"`
	SourceRevision            string                 `json:"sourceRevision,omitempty"`
	InventoryFingerprint      string                 `json:"inventoryFingerprint,omitempty"`
	SemanticFingerprint       string                 `json:"semanticFingerprint,omitempty"`
	SemanticDiscoveryPath     string                 `json:"semanticDiscoveryPath,omitempty"`
	SemanticIdentitiesPath    string                 `json:"semanticIdentitiesPath,omitempty"`
	DiscoveryCalls            int                    `json:"discoveryCalls,omitempty"`
	DiscoveryCacheHit         bool                   `json:"discoveryCacheHit,omitempty"`
	DiscoveryInventoryVersion string                 `json:"discoveryInventoryVersion,omitempty"`
	DiscoveryPromptVersion    string                 `json:"discoveryPromptVersion,omitempty"`
	DiscoveryModelID          string                 `json:"discoveryModelId,omitempty"`
	DiscoveryCounts           DiscoveryCounts        `json:"discoveryCounts"`
	DiscoveryStageMetrics     []DiscoveryStageMetric `json:"discoveryStageMetrics,omitempty"`
	PageHashes                map[string]string      `json:"pageHashes,omitempty"`
	ShardHashes               map[string]string      `json:"shardHashes,omitempty"`
	Status                    string                 `json:"status"`
	Phases                    map[string]PhaseStatus `json:"phases"`
}

type RunMetrics struct {
	StartedAt                  time.Time              `json:"startedAt"`
	CompletedAt                time.Time              `json:"completedAt,omitempty"`
	DurationMillis             int64                  `json:"durationMillis,omitempty"`
	OpenWikiCalls              int                    `json:"openWikiCalls"`
	PagesGenerated             int                    `json:"pagesGenerated"`
	PagesUpdated               int                    `json:"pagesUpdated"`
	EvidenceFiles              int                    `json:"evidenceFiles"`
	EvidenceCacheHits          int                    `json:"evidenceCacheHits"`
	EvidenceCacheMisses        int                    `json:"evidenceCacheMisses"`
	InputTokens                int64                  `json:"inputTokens,omitempty"`
	OutputTokens               int64                  `json:"outputTokens,omitempty"`
	UsageReported              bool                   `json:"usageReported"`
	DiscoveryStages            int                    `json:"discoveryStages"`
	DiscoveryCalls             int                    `json:"discoveryCalls"`
	DiscoveryCacheHits         int                    `json:"discoveryCacheHits"`
	DiscoveryCacheMisses       int                    `json:"discoveryCacheMisses"`
	DiscoveryAccepted          int                    `json:"discoveryAccepted"`
	DiscoveryUncertain         int                    `json:"discoveryUncertain"`
	DiscoveryConflicting       int                    `json:"discoveryConflicting"`
	DiscoveryUnknown           int                    `json:"discoveryUnknown"`
	DiscoveryStageMetrics      []DiscoveryStageMetric `json:"discoveryStageMetrics,omitempty"`
	DiscoveryCounts            DiscoveryCounts        `json:"discoveryCounts"`
	DiscoveryInventoryVersions []string               `json:"discoveryInventoryVersions,omitempty"`
	DiscoveryPromptVersions    []string               `json:"discoveryPromptVersions,omitempty"`
	DiscoveryModelIDs          []string               `json:"discoveryModelIds,omitempty"`
}

type RunState struct {
	Version    int                    `json:"version"`
	RunID      string                 `json:"runId"`
	Mode       string                 `json:"mode"`
	StartedAt  time.Time              `json:"startedAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	Components map[string]TargetState `json:"components"`
	System     TargetState            `json:"system"`
}
