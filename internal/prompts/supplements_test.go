package prompts

import (
	"strings"
	"testing"

	"github.com/example/wikiforge/internal/config"
)

func TestApplicationSupplementalCoverage(t *testing.T) {
	profile, err := GetProfile("application")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, f := range ExpectedFiles(profile) {
		if files[f] {
			t.Fatalf("duplicate file %s", f)
		}
		files[f] = true
	}
	for _, required := range []string{
		"configuration/runtime-configuration.md",
		"integrations/service-to-service.md",
		"integrations/external-services.md",
		"integrations/cloud-services.md",
		"integrations/dependency-matrix.md",
		"interfaces/endpoint-catalog.md",
		"messaging/event-catalog.md",
		"processing/job-catalog.md",
		"business/business-flows.md",
		"business/rules-and-validation.md",
		"business/business-data.md",
		"runtime/traffic-flows.md",
		"runtime/request-flows.md",
		"security/authentication.md",
		"security/authorization.md",
		"runtime/concurrency.md",
		"runtime/asynchronous-processing.md",
		"runtime/context-propagation.md",
		"data/database-structure.md",
		"data/database-programmability.md",
		"security/cryptography.md",
		"files/file-handling-and-formats.md",
	} {
		if !files[required] {
			t.Errorf("missing required specialized page %s", required)
		}
	}
	for _, obsolete := range []string{"runtime/traffic-and-request-flows.md", "security/authentication-and-authorization.md", "runtime/concurrency-and-asynchronous-processing.md"} {
		if files[obsolete] {
			t.Errorf("obsolete merged page still canonical: %s", obsolete)
		}
	}
}

func TestSystemSupplementalCoverage(t *testing.T) {
	files := map[string]bool{}
	for _, f := range ExpectedSystemFiles() {
		files[f] = true
	}
	for _, required := range []string{
		"system/dependency-matrix.md",
		"system/endpoint-catalog.md",
		"system/event-catalog.md",
		"system/job-catalog.md",
		"system/business-flow-rules-and-data.md",
		"system/traffic-flows.md",
		"system/request-flows.md",
		"system/authentication.md",
		"system/authorization.md",
		"system/concurrency.md",
		"system/asynchronous-processing.md",
		"system/context-propagation.md",
		"system/database-structures-and-programmability.md",
		"system/configuration-secrets-and-external-sources.md",
		"system/cloud-service-dependencies.md",
		"system/cryptography-and-key-management.md",
		"system/file-handling-and-formats.md",
	} {
		if !files[required] {
			t.Errorf("missing required system page %s", required)
		}
	}
	for _, obsolete := range []string{"system/traffic-and-request-flows.md", "system/authentication-and-authorization.md", "system/concurrency-async-and-context.md"} {
		if files[obsolete] {
			t.Errorf("obsolete merged system page still canonical: %s", obsolete)
		}
	}
}

func TestSpecializedPromptsRenderWithoutPlaceholders(t *testing.T) {
	profile, err := GetProfile("application")
	if err != nil {
		t.Fatal(err)
	}
	var supplementalPhaseFound bool
	var componentText strings.Builder
	for _, phase := range profile.Phases {
		if phase.PromptAsset != "prompts/component/supplemental.md" {
			continue
		}
		supplementalPhaseFound = true
		text, err := RenderComponentPhase(phase, profile, testComponent(), "English", nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, "{{") {
			t.Fatalf("component specialized prompt contains unresolved placeholder")
		}
		componentText.WriteString(text)
	}
	if !supplementalPhaseFound || !strings.Contains(componentText.String(), "configuration/runtime-configuration.md") {
		t.Fatal("component supplemental batches missing expected contract")
	}
	var systemPhaseFound bool
	var systemText strings.Builder
	for _, phase := range SystemPhases {
		if phase.PromptAsset != "prompts/system/55-specialized-catalogs.md" {
			continue
		}
		systemPhaseFound = true
		text, err := RenderSystemPhase(phase, "English", "system")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, "{{") {
			t.Fatalf("system specialized prompt contains unresolved placeholder")
		}
		systemText.WriteString(text)
	}
	if !systemPhaseFound || !strings.Contains(systemText.String(), "system/dependency-matrix.md") {
		t.Fatal("system supplemental batches missing expected contract")
	}
}

func testComponent() config.ComponentConfig {
	return config.ComponentConfig{ID: "app", Type: "microservice", Profile: "application", Repository: ".", Enabled: true}
}

func TestSplitContractUsesFreshResumePhaseIDs(t *testing.T) {
	profile, err := GetProfile("application")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, phase := range profile.Phases {
		ids[phase.ID] = true
	}
	if !ids["AS01"] || !ids["AC90"] {
		t.Fatalf("new component contract phase IDs missing: %v", ids)
	}
	for _, obsolete := range []string{"A80", "A81", "A82", "A83", "A84", "A90"} {
		if ids[obsolete] {
			t.Fatalf("obsolete resume phase ID remains active: %s", obsolete)
		}
	}

	systemIDs := map[string]bool{}
	for _, phase := range SystemPhases {
		systemIDs[phase.ID] = true
	}
	if !systemIDs["WS01"] || !systemIDs["WO60"] || !systemIDs["WC90"] {
		t.Fatalf("new system contract phase IDs missing: %v", systemIDs)
	}
	for _, obsolete := range []string{"W55", "W56", "W57", "W58", "W60", "W90"} {
		if systemIDs[obsolete] {
			t.Fatalf("obsolete system resume phase ID remains active: %s", obsolete)
		}
	}
}
