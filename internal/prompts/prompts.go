package prompts

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/fajarnugraha37/wikiforge/internal/assets"
)

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
