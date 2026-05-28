package system

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestManifestDeclaresNoAuthDatasourcesOrIndexes(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if len(manifest.Auth) != 0 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
	if len(manifest.Datasources) != 0 {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	if len(manifest.Indexes) != 0 {
		t.Fatalf("indexes = %#v", manifest.Indexes)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != "system.info" {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	if !manifest.Operations[0].ReadOnly || len(manifest.Operations[0].SecretPurposes) != 0 {
		t.Fatalf("operation spec = %#v", manifest.Operations[0])
	}
}

func TestInfoDefaultsToAllCategories(t *testing.T) {
	out := plugintest.RunOK[InfoResult](t, NewPlugin(), OperationInfo, map[string]any{})
	if len(out.Categories) != len(allCategories) {
		t.Fatalf("categories = %#v", out.Categories)
	}
	for _, category := range allCategories {
		if _, ok := out.System[category]; !ok {
			t.Fatalf("missing category %q in %#v", category, out.System)
		}
	}
}

func TestInfoFiltersCategories(t *testing.T) {
	out := plugintest.RunOK[InfoResult](t, NewPlugin(), OperationInfo, map[string]any{
		"categories": []string{"os", "time"},
	})
	if len(out.Categories) != 2 || out.Categories[0] != "os" || out.Categories[1] != "time" {
		t.Fatalf("categories = %#v", out.Categories)
	}
	if _, ok := out.System["cpu"]; ok {
		t.Fatalf("unexpected cpu category in %#v", out.System)
	}
}

func TestInfoCategoryAliasAndExclude(t *testing.T) {
	out := plugintest.RunOK[InfoResult](t, NewPlugin(), OperationInfo, map[string]any{
		"category": "arch,cpus,timezone",
		"exclude":  []string{"cpu"},
	})
	if len(out.Categories) != 2 || out.Categories[0] != "os" || out.Categories[1] != "time" {
		t.Fatalf("categories = %#v", out.Categories)
	}
}

func TestInfoIncludeStringAndExclude(t *testing.T) {
	out := plugintest.RunOK[InfoResult](t, NewPlugin(), OperationInfo, map[string]any{
		"include": "os,network",
		"exclude": "network",
	})
	if len(out.Categories) != 1 || out.Categories[0] != "os" {
		t.Fatalf("categories = %#v", out.Categories)
	}
}

func TestInfoUnknownCategoryFails(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationInfo, map[string]any{
		"categories": []string{"missing"},
	})
	if err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestInfoNetworkShape(t *testing.T) {
	out := plugintest.RunOK[InfoResult](t, NewPlugin(), OperationInfo, map[string]any{
		"category": "network",
	})
	network, ok := out.System["network"].(map[string]any)
	if !ok {
		t.Fatalf("network = %#v", out.System["network"])
	}
	if _, ok := network["interfaces"]; !ok {
		t.Fatalf("network missing interfaces: %#v", network)
	}
}
