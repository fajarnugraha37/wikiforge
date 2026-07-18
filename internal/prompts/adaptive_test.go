package prompts

import (
	"strings"
	"testing"

	"github.com/fajarnugraha37/wikiforge/internal/config"
)

func TestRenderAdaptivePageUsesOpenWikiVirtualRepositoryRoot(t *testing.T) {
	prompt, err := RenderAdaptivePage(
		"components/sentinel/architecture.md",
		"component",
		"page",
		"sentinel",
		"modular-application",
		config.ComponentConfig{
			ID:         "sentinel",
			Type:       "modular-monolith",
			Repository: `C:\Users\nugra\workspace\project`,
		},
		"unit context",
		"component",
		"architecture",
		"English",
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, `C:\Users\nugra`) {
		t.Fatal("adaptive prompt leaked the host repository path")
	}
	if !strings.Contains(prompt, openWikiRepositoryRoot) {
		t.Fatal("adaptive prompt omitted the OpenWiki virtual repository path contract")
	}
}
