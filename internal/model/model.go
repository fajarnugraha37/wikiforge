package model

import "time"

type Component struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Profile      string   `json:"profile"`
	Repository   string   `json:"repository"`
	Scope        string   `json:"scope,omitempty"`
	WorkDir      string   `json:"workDir"`
	Group        string   `json:"group,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	DependsOn    []string `json:"dependsOn,omitempty"`
	Owners       []string `json:"owners,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Packs        []string `json:"packs,omitempty"`
}

type PageContract struct {
	Path                string
	Objective           string
	RequiredHeadings    []string
	RequiredDiagram     string
	RequiredTableHeader string
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
	Root          string    `json:"root"`
	Profile       string    `json:"profile,omitempty"`
	Score         int       `json:"score"`
	Accepted      bool      `json:"accepted"`
	MarkdownFiles int       `json:"markdownFiles"`
	MermaidBlocks int       `json:"mermaidBlocks"`
	Findings      []Finding `json:"findings"`
}

type PhaseStatus struct {
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type TargetState struct {
	GitHead       string                 `json:"gitHead,omitempty"`
	DocsHash      string                 `json:"docsHash,omitempty"`
	SourceHash    string                 `json:"sourceHash,omitempty"`
	DiscoveryHash string                 `json:"discoveryHash,omitempty"`
	PlanHash      string                 `json:"planHash,omitempty"`
	Status        string                 `json:"status"`
	Phases        map[string]PhaseStatus `json:"phases"`
}

type RunState struct {
	Version    int                    `json:"version"`
	RunID      string                 `json:"runId"`
	Mode       string                 `json:"mode"`
	StartedAt  time.Time              `json:"startedAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	Components map[string]TargetState `json:"components"`
	// Services is retained to migrate v1 state files.
	Services map[string]TargetState `json:"services,omitempty"`
	System   TargetState            `json:"system"`
}

type DocumentationUnit struct {
	ID           string   `json:"id"`
	ComponentID  string   `json:"componentId"`
	Kind         string   `json:"kind"`
	SourceRoots  []string `json:"sourceRoots,omitempty"`
	RelatedUnits []string `json:"relatedUnits,omitempty"`
	OutputPath   string   `json:"outputPath,omitempty"`
	Owners       []string `json:"owners,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Criticality  string   `json:"criticality,omitempty"`
	Origin       string   `json:"origin"`
}

type EvidenceMatch struct {
	Pack  string   `json:"pack"`
	Paths []string `json:"paths"`
	Count int      `json:"count"`
}

type DiscoveryManifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Component     Component           `json:"component"`
	SourceHash    string              `json:"sourceHash"`
	FilesScanned  int                 `json:"filesScanned"`
	BytesScanned  int64               `json:"bytesScanned"`
	Packs         []string            `json:"packs"`
	Evidence      []EvidenceMatch     `json:"evidence"`
	Units         []DocumentationUnit `json:"units"`
	Unknowns      []string            `json:"unknowns,omitempty"`
}

type PlanPage struct {
	ID                 string   `json:"id"`
	Path               string   `json:"path"`
	View               string   `json:"view"`
	Pack               string   `json:"pack,omitempty"`
	UnitID             string   `json:"unitId,omitempty"`
	Kind               string   `json:"kind"`
	Reason             string   `json:"reason"`
	ShardBy            []string `json:"shardBy,omitempty"`
	MaximumRowsPerPage int      `json:"maximumRowsPerPage,omitempty"`
}

type PlanDecision struct {
	Subject string `json:"subject"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

type DocumentationPlan struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	ComponentID        string              `json:"componentId"`
	Profile            string              `json:"profile"`
	Views              []string            `json:"views"`
	SelectedPacks      []string            `json:"selectedPacks"`
	Units              []DocumentationUnit `json:"units"`
	Pages              []PlanPage          `json:"pages"`
	Decisions          []PlanDecision      `json:"decisions"`
	ShardBy            []string            `json:"shardBy,omitempty"`
	MaximumRowsPerPage int                 `json:"maximumRowsPerPage,omitempty"`
}
