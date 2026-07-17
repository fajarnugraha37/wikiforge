package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestPublishedSchemaIsValidAndContainsV3PlanningContracts(t *testing.T) {
	path := filepath.Join("..", "..", "schema", "wikiforge-config.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	version := properties["version"].(map[string]any)
	enums := version["enum"].([]any)
	if enums[len(enums)-1].(float64) != float64(CurrentVersion) {
		t.Fatalf("schema versions=%v", enums)
	}
	if _, ok := properties["documentationUnits"]; !ok {
		t.Fatal("schema missing documentationUnits")
	}
	defs := schema["$defs"].(map[string]any)
	component := defs["component"].(map[string]any)["properties"].(map[string]any)
	packs, ok := component["packs"]
	if !ok {
		t.Fatal("schema missing component packs")
	}
	packItems := packs.(map[string]any)["items"].(map[string]any)["enum"].([]any)
	if got, want := stringEnum(packItems), sortedStrings(SupportedCapabilityPacks()); !reflect.DeepEqual(got, want) {
		t.Fatalf("schema pack registry drift: got=%v want=%v", got, want)
	}
	doc := properties["documentation"].(map[string]any)["properties"].(map[string]any)
	viewItems := doc["views"].(map[string]any)["items"].(map[string]any)["enum"].([]any)
	if got, want := stringEnum(viewItems), sortedStrings(SupportedViews()); !reflect.DeepEqual(got, want) {
		t.Fatalf("schema view registry drift: got=%v want=%v", got, want)
	}
	for _, field := range []string{"views", "catalogs", "evidence"} {
		if _, ok := doc[field]; !ok {
			t.Fatalf("schema missing documentation.%s", field)
		}
	}
	shardItems := doc["catalogs"].(map[string]any)["properties"].(map[string]any)["shardBy"].(map[string]any)["items"].(map[string]any)["enum"].([]any)
	if got, want := stringEnum(shardItems), sortedStrings(SupportedShardDimensions()); !reflect.DeepEqual(got, want) {
		t.Fatalf("schema shard registry drift: got=%v want=%v", got, want)
	}
	criticalityItems := defs["documentationUnit"].(map[string]any)["properties"].(map[string]any)["criticality"].(map[string]any)["enum"].([]any)
	if got, want := stringEnum(criticalityItems), sortedStrings(SupportedCriticalities()); !reflect.DeepEqual(got, want) {
		t.Fatalf("schema criticality registry drift: got=%v want=%v", got, want)
	}
}

func stringEnum(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.(string))
	}
	sort.Strings(out)
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
