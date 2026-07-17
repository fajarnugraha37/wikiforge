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
	GitHead    string                 `json:"gitHead,omitempty"`
	DocsHash   string                 `json:"docsHash,omitempty"`
	SourceHash string                 `json:"sourceHash,omitempty"`
	Status     string                 `json:"status"`
	Phases     map[string]PhaseStatus `json:"phases"`
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
