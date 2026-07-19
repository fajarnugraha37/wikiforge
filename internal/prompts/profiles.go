package prompts

import (
	"fmt"
	"sort"
	"strings"
)

// Profile describes the evidence lens used by the adaptive planner and prompts.
// Page selection and execution order belong to planner output, not profiles.
type Profile struct {
	ID          string
	DisplayName string
	Description string
	TargetNoun  string
}

var profiles = map[string]Profile{
	"application": {
		ID: "application", DisplayName: "Deployable Application", TargetNoun: "application component",
		Description: "A deployable application such as a monolith, microservice, worker, gateway, frontend, or CLI.",
	},
	"modular-application": {
		ID: "modular-application", DisplayName: "Modular Monolith", TargetNoun: "modular application",
		Description: "A single deployable application with explicit internal modules and dependency boundaries.",
	},
	"reusable": {
		ID: "reusable", DisplayName: "Reusable Library or Framework", TargetNoun: "reusable component",
		Description: "A library, SDK, shared package, internal framework, or reusable foundation consumed by other repositories.",
	},
	"infrastructure": {
		ID: "infrastructure", DisplayName: "Infrastructure, IaC, GitOps, or Deployment", TargetNoun: "infrastructure component",
		Description: "Infrastructure-as-code, GitOps, platform, deployment, environment, or operational automation repository.",
	},
	"configuration": {
		ID: "configuration", DisplayName: "Configuration Repository", TargetNoun: "configuration component",
		Description: "A repository primarily containing shared configuration, policy, templates, manifests, feature definitions, or environment values.",
	},
	"contracts": {
		ID: "contracts", DisplayName: "Contract or Schema Repository", TargetNoun: "contract component",
		Description: "A repository containing API, event, message, data, protocol, or interface contracts shared across components.",
	},
	"generic": {
		ID: "generic", DisplayName: "Generic Repository", TargetNoun: "repository component",
		Description: "A language- and technology-neutral fallback for repositories that do not fit another profile.",
	},
}

func GetProfile(id string) (Profile, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	profile, ok := profiles[id]
	if !ok {
		return Profile{}, fmt.Errorf("unknown documentation profile %q", id)
	}
	return profile, nil
}

func ProfileIDs() []string {
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
