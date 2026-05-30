package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestRunnerServesHostLookupDuringPluginOperation(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakehostlookup\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-slack")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"os"
)

func write(id string, ok bool, result any) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v2","id":id,"type":"response","ok":ok,"result":result})
}

func main() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var raw json.RawMessage
	_ = dec.Decode(&raw)
	var frame struct {
		ID      string          ` + "`json:\"id\"`" + `
		Payload json.RawMessage ` + "`json:\"payload\"`" + `
	}
	_ = json.Unmarshal(raw, &frame)
	var req struct{ Command string ` + "`json:\"command\"`" + ` }
	if frame.ID != "" {
		_ = json.Unmarshal(frame.Payload, &req)
	} else {
		_ = json.Unmarshal(raw, &req)
	}
	if req.Command == "manifest" {
		if frame.ID == "" {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":map[string]any{"name":"slack","operations":[]map[string]any{{"name":"slack.lookup-test","read_only":true}},"metadata":map[string]string{"dex.protocol":"dex.plugin.v2"}}})
		} else {
			write(frame.ID, true, map[string]any{"name":"slack","operations":[]map[string]any{{"name":"slack.lookup-test","read_only":true}},"metadata":map[string]string{"dex.protocol":"dex.plugin.v2"}})
		}
		return
	}
	_ = enc.Encode(map[string]any{
		"protocol":"dex.plugin.v2",
		"id":"lookup-1",
		"type":"request",
		"target":"host",
		"command":"host.index.lookup",
		"payload":map[string]any{"entity":"slack.channel","terms":[]string{"engineering"},"limit":1},
	})
	var hostResp struct {
		Result struct {
			Matches []struct{ ID string ` + "`json:\"id\"`" + ` } ` + "`json:\"matches\"`" + `
		} ` + "`json:\"result\"`" + `
	}
	_ = dec.Decode(&hostResp)
	write(frame.ID, true, map[string]any{"channel":hostResp.Result.Matches[0].ID})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("slack", "default", "slack.channels", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.channel","id":"C1","channel_id":"C1","name":"engineering"}`),
	}); err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State: state,
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "slack", Binary: "dex-plugin-slack", LocalPath: pluginDir},
		}}),
	}
	resp, err := runner.InvokeInstance(nil, "slack", "default", protocol.CommandOperationsCall, protocol.OperationCall{Name: "slack.lookup-test"})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.Channel != "C1" {
		t.Fatalf("response = %#v", out)
	}
}

func TestRunnerFallsBackWhenAdvertisedV2PluginCannotFrame(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakelegacyadvertisedv2\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-stale")
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
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	result := map[string]any{"ok": true}
	if req.Command == "manifest" {
		result = map[string]any{
			"name": "stale",
			"operations": []map[string]any{{"name":"stale.ping","read_only":true}},
			"metadata": map[string]string{"dex.protocol":"dex.plugin.v2"},
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":result})
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
		State: state,
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "stale", Binary: "dex-plugin-stale", LocalPath: pluginDir},
		}}),
	}
	resp, err := runner.InvokeInstance(nil, "stale", "default", protocol.CommandOperationsCall, protocol.OperationCall{Name: "stale.ping"})
	if err != nil {
		if strings.Contains(err.Error(), "unexpected plugin frame type") {
			t.Fatalf("mixed protocol should not surface raw frame error: %v", err)
		}
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
}

func TestRunnerServesHostLookupToPluginBindingOverV2(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(cwd)
	pluginDir := t.TempDir()
	rootGoMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goMod := strings.Replace(string(rootGoMod), "module github.com/fluxplane/fluxplane-dex", "module bindinghostlookup", 1)
	endpointRoot := filepath.Join(filepath.Dir(repoRoot), "fluxplane-endpoint")
	goMod += "\nrequire github.com/fluxplane/fluxplane-dex v0.0.0\n\nreplace github.com/fluxplane/fluxplane-dex => " + repoRoot + "\nreplace github.com/fluxplane/fluxplane-endpoint => " + endpointRoot + "\n"
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	goSum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "go.sum"), goSum, 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-lookupbinding")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type lookupInput struct{}

func main() {
	plugin := pluginbinding.New(core.PluginManifest{
		Name: "lookupbinding",
		Metadata: map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
	})
	pluginbinding.Operation(plugin, core.OperationSpec{Name: "lookupbinding.resolve", ReadOnly: true}, func(ctx pluginbinding.Context, _ lookupInput) (map[string]string, error) {
		if err := ctx.Status("resolving channel", map[string]string{"entity": "slack.channel"}); err != nil {
			return nil, err
		}
		result, err := ctx.Host.Lookup(pluginbinding.DatasourceLookupInput{
			Text: "engineering",
			Terms: []string{"engineering"},
			Entity: "slack.channel",
			Limit: 1,
		})
		if err != nil {
			return nil, err
		}
		if len(result.Matches) == 0 {
			return map[string]string{"channel": ""}, nil
		}
		return map[string]string{"channel": result.Matches[0].ID}, nil
	})
	pluginbinding.Serve(plugin)
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("lookupbinding", "default", "slack.channels", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.channel","id":"C1","channel_id":"C1","name":"engineering"}`),
	}); err != nil {
		t.Fatal(err)
	}
	var events []PluginEvent
	runner := Runner{
		State: state,
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
			{Name: "lookupbinding", Binary: "dex-plugin-lookupbinding", LocalPath: pluginDir},
		}}),
		EventSink: func(_ context.Context, event PluginEvent) {
			events = append(events, event)
		},
	}
	resp, err := runner.InvokeInstance(nil, "lookupbinding", "default", protocol.CommandOperationsCall, protocol.OperationCall{Name: "lookupbinding.resolve"})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.Channel != "C1" {
		t.Fatalf("response = %#v", out)
	}
	if len(events) != 1 || events[0].Event != "status" {
		t.Fatalf("events = %#v", events)
	}
	var eventPayload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(events[0].Payload, &eventPayload); err != nil {
		t.Fatal(err)
	}
	if eventPayload.Message != "resolving channel" {
		t.Fatalf("event payload = %#v", eventPayload)
	}
}
