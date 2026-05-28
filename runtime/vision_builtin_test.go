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

func TestVisionBuiltinManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, visionManifest())
}

func TestVisionBuiltinDiscoversAndCallsGenericProvider(t *testing.T) {
	pluginDir := writeFakeVisionProvider(t, "fake", false)
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State:   state,
		WorkDir: t.TempDir(),
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "fake", Binary: "dex-plugin-fake", LocalPath: pluginDir},
			{Name: "vision", Metadata: map[string]string{"kind": "builtin"}},
		}}),
	}

	providersResp, err := runner.InvokeInstance(nil, "vision", "default", protocol.CommandOperationsCall, operationCall(t, "vision.provider.list", nil))
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

	analyzeResp, err := runner.InvokeInstance(nil, "vision", "default", protocol.CommandOperationsCall, operationCall(t, "vision.analyze", map[string]any{
		"prompt":    "read it",
		"providers": []string{"fake-provider"},
		"images":    []map[string]any{{"url": "https://example.com/image.png"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Results []struct {
			Provider string `json:"provider"`
			Text     string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(analyzeResp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 1 || out.Results[0].Provider != "fake-provider" || out.Results[0].Text != "analysis: read it" {
		t.Fatalf("analysis = %#v", out)
	}
}

func TestVisionBuiltinFailsWhenAllProvidersFail(t *testing.T) {
	pluginDir := writeFakeVisionProvider(t, "bad", true)
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State:   state,
		WorkDir: t.TempDir(),
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "bad", Binary: "dex-plugin-bad", LocalPath: pluginDir},
			{Name: "vision", Metadata: map[string]string{"kind": "builtin"}},
		}}),
	}

	resp, err := runner.InvokeInstance(nil, "vision", "default", protocol.CommandOperationsCall, operationCall(t, "vision.analyze", map[string]any{
		"images": []map[string]any{{"url": "https://example.com/image.png"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != "vision_failed" {
		t.Fatalf("response = %#v", resp)
	}
}

func writeFakeVisionProvider(t *testing.T, name string, fail bool) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fake"+name+"\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-"+name)
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	failLiteral := "false"
	if fail {
		failLiteral = "true"
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
				"name": "` + name + `",
				"aliases": []string{"` + name + `-alias"},
				"operations": []map[string]any{{"name": "` + name + `.vision.analyze", "read_only": true}},
				"metadata": map[string]string{"vision.provider": "` + name + `-provider", "vision.operation": "` + name + `.vision.analyze"},
			},
		})
		return
	}
	var input struct{ Prompt string ` + "`json:\"prompt\"`" + ` }
	_ = json.Unmarshal(req.Payload.Input, &input)
	if ` + failLiteral + ` {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":map[string]any{"errors":[]map[string]any{{"message":"boom"}}}})
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"results": []map[string]any{{"provider":"` + name + `-provider","text":"analysis: "+input.Prompt}}},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}
