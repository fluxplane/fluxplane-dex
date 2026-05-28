package system

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
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
	if manifest.Operations[0].Risk != core.OperationRiskLow || manifest.Operations[0].Idempotency != core.OperationIdempotent {
		t.Fatalf("operation metadata = %#v", manifest.Operations[0])
	}
	if len(manifest.Context) != 1 || manifest.Context[0].Name != ContextName {
		t.Fatalf("context = %#v", manifest.Context)
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

func TestBuildContext(t *testing.T) {
	resp := NewPlugin().Handle(protocol.Request{
		Command: protocol.CommandContextBuild,
		Plugin:  PluginName,
		Payload: []byte(`{"query":"debug"}`),
	})
	if !resp.OK {
		t.Fatalf("context failed: %#v", resp.Error)
	}
	var result struct {
		Blocks []core.ContextBlock `json:"blocks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 1 || result.Blocks[0].Source == nil || result.Blocks[0].Source.Plugin != PluginName {
		t.Fatalf("blocks = %#v", result.Blocks)
	}
}
