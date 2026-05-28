package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestWebsearchBuiltinManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, websearchManifest())
}

func TestWebsearchBuiltinDiscoversAndCallsGenericProvider(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakeprovider\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-fake")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct {
		Command string ` + "`json:\"command\"`" + `
		Payload struct {
			Name  string          ` + "`json:\"name\"`" + `
			Input json.RawMessage ` + "`json:\"input\"`" + `
		} ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "fake",
				"aliases": []string{"generic"},
				"operations": []map[string]any{{"name": "fake.search", "read_only": true}},
				"datasources": []map[string]any{{"name": "fake.web", "entity": "web.search_result", "capabilities": []string{"search"}}},
				"metadata": map[string]string{"websearch.provider": "fake-provider", "websearch.operation": "fake.search"},
			},
		})
		return
	}
	var input struct {
		Query string ` + "`json:\"query\"`" + `
	}
	_ = json.Unmarshal(req.Payload.Input, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{
			"results": []map[string]any{{
				"provider": "fake-provider",
				"query": input.Query,
				"results": []map[string]any{{"url": "https://example.com", "title": "Example", "snippet": "Result"}},
			}},
		},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State:   state,
		WorkDir: t.TempDir(),
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "fake", Aliases: []string{"generic"}, Binary: "dex-plugin-fake", LocalPath: pluginDir},
			{Name: "websearch", Aliases: []string{"web"}, Metadata: map[string]string{"kind": "builtin"}},
		}}),
	}

	providersResp, err := runner.InvokeInstance(nil, "websearch", "default", protocol.CommandOperationsCall, operationCall(t, "websearch.provider.list", nil))
	if err != nil {
		t.Fatal(err)
	}
	var providers struct {
		Count     int `json:"count"`
		Providers []struct {
			Name   string `json:"name"`
			Plugin string `json:"plugin"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(providersResp.Result, &providers); err != nil {
		t.Fatal(err)
	}
	if providers.Count != 1 || providers.Providers[0].Name != "fake-provider" || providers.Providers[0].Plugin != "fake" {
		t.Fatalf("providers = %#v", providers)
	}

	searchResp, err := runner.InvokeInstance(nil, "websearch", "default", protocol.CommandOperationsCall, operationCall(t, "websearch.search", map[string]any{"query": "hello", "providers": []string{"generic"}}))
	if err != nil {
		t.Fatal(err)
	}
	var search struct {
		Results []struct {
			Provider string `json:"provider"`
			Query    string `json:"query"`
			Results  []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"results"`
	}
	if err := json.Unmarshal(searchResp.Result, &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Results) != 1 || search.Results[0].Provider != "fake-provider" || search.Results[0].Results[0].URL != "https://example.com" {
		t.Fatalf("search = %#v", search)
	}
}

func TestWebsearchBuiltinSupportsOperationBatch(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State: state,
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "websearch", Aliases: []string{"web"}, Metadata: map[string]string{"kind": "builtin"}},
		}}),
	}

	result, err := runner.OperationBatch(nil, "websearch", "default", []protocol.OperationCall{
		{ID: "providers", Name: "websearch.provider.list"},
		operationCall(t, "websearch.missing", nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("results = %#v", result.Results)
	}
	if !result.Results[0].OK || result.Results[0].ID != "providers" {
		t.Fatalf("first result = %#v", result.Results[0])
	}
	if result.Results[1].OK || result.Results[1].Error == nil || result.Results[1].Error.Code != "unknown_operation" {
		t.Fatalf("second result = %#v", result.Results[1])
	}
}

func operationCall(t *testing.T, name string, input any) protocol.OperationCall {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.OperationCall{Name: name, Input: raw}
}
