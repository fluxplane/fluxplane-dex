package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

func TestRootHelp(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("plugin")) {
		t.Fatalf("help output missing plugin command:\n%s", out.String())
	}
}

func TestVersionCommand(t *testing.T) {
	oldVersion := Version
	oldRevision := Revision
	Version = "v1.2.3"
	Revision = "0123456789abcdef"
	t.Cleanup(func() {
		Version = oldVersion
		Revision = oldRevision
	})
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Name    string `json:"name"`
		Module  string `json:"module"`
		Version string `json:"version"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "fluxplane-dex" || result.Module != dexModulePath || result.Version != "v1.2.3" {
		t.Fatalf("version result = %#v", result)
	}
	if result.Text != "fluxplane-dex v1.2.3 (0123456789ab)" {
		t.Fatalf("version text = %q", result.Text)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"revision": "0123456789abcdef"`)) {
		t.Fatalf("version output missing revision:\n%s", out.String())
	}
}

func TestUnavailableActivatedPluginDoesNotBlockBuiltins(t *testing.T) {
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab"}
	if err := state.ActivatePlugin(entry); err != nil {
		t.Fatal(err)
	}
	marketplacePath := filepath.Join(t.TempDir(), "marketplace.json")
	if err := os.WriteFile(marketplacePath, []byte(`{"version":"1","plugins":[{"name":"gitlab","binary":"dex-plugin-gitlab"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", home, "--marketplace", marketplacePath, "version", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "fluxplane-dex"`) {
		t.Fatalf("version output = %s", out)
	}
}

func TestRootHelpGroupsGeneratedIntegrationCommands(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Essentials", "Plugin Management", "Data and Context", "Configuration", "Maintenance", "Integration Commands", "kubernetes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("root help missing %q:\n%s", want, out)
		}
	}
}

func TestPluginMarketplaceJSON(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"plugin", "marketplace", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"name": "gitlab"`)) {
		t.Fatalf("marketplace output missing gitlab:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"name": "system"`)) {
		t.Fatalf("marketplace output missing system:\n%s", out.String())
	}
}

func TestSystemInfoCommandFiltersCategories(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "system=../../plugins/system", "op", "run", "system.info", `{"categories":["os","time"]}`, "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Categories []string       `json:"categories"`
		System     map[string]any `json:"system"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Categories) != 2 || result.Categories[0] != "os" || result.Categories[1] != "time" {
		t.Fatalf("categories = %#v", result.Categories)
	}
	if _, ok := result.System["cpu"]; ok {
		t.Fatalf("unexpected cpu category in %#v", result.System)
	}
}

func TestPluginStatusActivateDeactivate(t *testing.T) {
	home := t.TempDir()
	activate := NewRootCommand()
	var activateOut bytes.Buffer
	activate.SetOut(&activateOut)
	activate.SetErr(&activateOut)
	activate.SetArgs([]string{"--dex-home", home, "plugin", "activate", "websearch", "-o", "json"})
	if err := activate.Execute(); err != nil {
		t.Fatal(err)
	}
	var active runtime.PluginStatus
	if err := json.Unmarshal(activateOut.Bytes(), &active); err != nil {
		t.Fatal(err)
	}
	if active.Name != "websearch" || !active.Installed || !active.Activated {
		t.Fatalf("active status = %#v", active)
	}

	deactivate := NewRootCommand()
	var deactivateOut bytes.Buffer
	deactivate.SetOut(&deactivateOut)
	deactivate.SetErr(&deactivateOut)
	deactivate.SetArgs([]string{"--dex-home", home, "plugin", "deactivate", "websearch", "-o", "json"})
	if err := deactivate.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Changed bool                 `json:"changed"`
		Status  runtime.PluginStatus `json:"status"`
	}
	if err := json.Unmarshal(deactivateOut.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Status.Activated {
		t.Fatalf("deactivate result = %#v", result)
	}
}

func TestEndpointAddListShow(t *testing.T) {
	home := t.TempDir()
	add := NewRootCommand()
	var addOut bytes.Buffer
	add.SetOut(&addOut)
	add.SetErr(&addOut)
	add.SetArgs([]string{
		"--dex-home", home,
		"endpoint", "add", "mysql://127.0.0.1:3306/dev",
		"--id", "local-mysql",
		"--product", "mysql",
		"--protocol", "mysql",
		"--label", "env=dev",
		"-o", "json",
	})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	var added runtime.EndpointRecord
	if err := json.Unmarshal(addOut.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	if added.ID != "local-mysql" || added.URL != "mysql://127.0.0.1:3306/dev" || added.Labels["env"] != "dev" {
		t.Fatalf("added = %#v", added)
	}

	show := NewRootCommand()
	var showOut bytes.Buffer
	show.SetOut(&showOut)
	show.SetErr(&showOut)
	show.SetArgs([]string{"--dex-home", home, "endpoint", "show", "local-mysql", "-o", "json"})
	if err := show.Execute(); err != nil {
		t.Fatal(err)
	}
	var shown runtime.EndpointRecord
	if err := json.Unmarshal(showOut.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.ID != "local-mysql" {
		t.Fatalf("shown = %#v", shown)
	}
}

func TestEndpointTestUsesSQLPluginWithResolvedEndpoint(t *testing.T) {
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "local-mysql", URL: "mysql://db.example.com:3306/app", Product: "mysql", Protocol: "mysql"}); err != nil {
		t.Fatal(err)
	}
	pluginDir := writeFakeSQLPlugin(t)

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "sql=" + pluginDir, "endpoint", "test", "local-mysql", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result endpointTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Method != "sql.query" {
		t.Fatalf("result = %#v", result)
	}
	if result.Details["endpoint_ref"] != "local-mysql" || result.Details["endpoint_url"] != "mysql://db.example.com:3306/app" {
		t.Fatalf("details = %#v", result.Details)
	}
}

func TestEndpointTestUsesTCPFallback(t *testing.T) {
	home := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "local-tcp", URL: "tcp://" + listener.Addr().String(), Product: "custom", Protocol: "tcp"}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "endpoint", "test", "local-tcp", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result endpointTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Method != "tcp_connect" {
		t.Fatalf("result = %#v", result)
	}
	record, ok, err := state.GetEndpoint("local-tcp")
	if err != nil || !ok {
		t.Fatalf("get endpoint ok=%v err=%v", ok, err)
	}
	if record.LastHealth == nil || !record.LastHealth.OK || record.LastHealth.Method != "tcp_connect" {
		t.Fatalf("last health = %#v", record.LastHealth)
	}
}

func TestDoctorEndpointsTestsAndStoresHealth(t *testing.T) {
	home := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "local-tcp", URL: "tcp://" + listener.Addr().String(), Product: "custom", Protocol: "tcp"}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "doctor", "endpoints", "custom", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result endpointDoctorResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || result.OK != 1 || result.Failed != 0 || result.Endpoints[0].Method != "tcp_connect" {
		t.Fatalf("result = %#v", result)
	}
	record, ok, err := state.GetEndpoint("local-tcp")
	if err != nil || !ok {
		t.Fatalf("get endpoint ok=%v err=%v", ok, err)
	}
	if record.LastHealth == nil || !record.LastHealth.OK {
		t.Fatalf("last health = %#v", record.LastHealth)
	}
}

func TestEndpointTestUsesKubernetesPlugin(t *testing.T) {
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev-cluster", URL: "kubernetes://context/dev", Product: "kubernetes", Protocol: "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	pluginDir := writeFakeKubernetesTestPlugin(t)

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "endpoint", "test", "dev-cluster", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result endpointTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Method != "kubernetes.cluster.test" {
		t.Fatalf("result = %#v", result)
	}
	if result.Details["context"] != "dev" || result.Details["server_version"] != "v1.30.0" {
		t.Fatalf("details = %#v", result.Details)
	}
}

func TestEndpointImportCandidateFromDiscoveryJSON(t *testing.T) {
	home := t.TempDir()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(`{"candidates":[{"index":2,"id":"mysql:abc","url":"mysql://db.example.com:3306/app","product":"mysql","protocol":"mysql","source":"kubernetes_secret","credential_ref":"kubernetes://latest/secrets/mysql","labels":{"namespace":"latest"}}]}`))
	cmd.SetArgs([]string{"--dex-home", home, "endpoint", "import", "--candidate", "2", "--id", "latest-mysql", "--label", "role=read", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var record runtime.EndpointRecord
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.ID != "latest-mysql" || record.URL != "mysql://db.example.com:3306/app" || record.CredentialRef != "kubernetes://latest/secrets/mysql" {
		t.Fatalf("record = %#v", record)
	}
	if record.Labels["namespace"] != "latest" || record.Labels["role"] != "read" {
		t.Fatalf("labels = %#v", record.Labels)
	}
}

func TestEndpointDiscoverPluginPassesInputAndIndexesCandidates(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakekubernetes\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-kubernetes")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	var req struct {
		Command string          ` + "`json:\"command\"`" + `
		Payload json.RawMessage ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "kubernetes",
				"endpoints": []map[string]any{{"name": "kubernetes.endpoint.discover", "products": []string{"mysql"}}},
			},
		})
		return
	}
	var input map[string]any
	_ = json.Unmarshal(req.Payload, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"candidates": []map[string]any{{
			"id": "mysql:abc",
			"url": "mysql://db.example.com:3306/app",
			"product": input["product"],
			"protocol": "mysql",
			"source": "kubernetes_secret",
			"credential_ref": "kubernetes://latest/secrets/mysql",
			"labels": map[string]any{"context": input["context"], "namespace": input["namespace"], "limit": fmt.Sprint(input["limit"])},
			"annotations": map[string]any{"raw": "hidden"},
		}}},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "kubernetes=" + pluginDir, "endpoint", "discover", "mysql", "--plugin", "kubernetes", "--context", "dev", "--namespace", "latest", "--limit", "3", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result endpointDiscoveryView
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plugin != "kubernetes" || len(result.Candidates) != 1 || result.Candidates[0].Index != 1 {
		t.Fatalf("result = %#v", result)
	}
	labels := result.Candidates[0].Labels
	if labels["context"] != "dev" || labels["namespace"] != "latest" || labels["limit"] != "3" {
		t.Fatalf("labels = %#v", labels)
	}
	if result.Candidates[0].Annotations != nil {
		t.Fatalf("default discovery should omit annotations: %#v", result.Candidates[0].Annotations)
	}
	rawCmd := NewRootCommand()
	var rawOut bytes.Buffer
	rawCmd.SetOut(&rawOut)
	rawCmd.SetErr(&rawOut)
	rawCmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "kubernetes=" + pluginDir, "endpoint", "discover", "mysql", "--plugin", "kubernetes", "--limit", "3", "--raw", "-o", "json"})
	if err := rawCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rawResult endpointDiscoveryView
	if err := json.Unmarshal(rawOut.Bytes(), &rawResult); err != nil {
		t.Fatal(err)
	}
	if rawResult.Candidates[0].Annotations["raw"] != "hidden" {
		t.Fatalf("raw discovery annotations = %#v", rawResult.Candidates[0].Annotations)
	}
	defaultLimitCmd := NewRootCommand()
	var defaultLimitOut bytes.Buffer
	defaultLimitCmd.SetOut(&defaultLimitOut)
	defaultLimitCmd.SetErr(&defaultLimitOut)
	defaultLimitCmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "kubernetes=" + pluginDir, "endpoint", "discover", "mysql", "--plugin", "kubernetes", "-o", "json"})
	if err := defaultLimitCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var defaultLimitResult endpointDiscoveryView
	if err := json.Unmarshal(defaultLimitOut.Bytes(), &defaultLimitResult); err != nil {
		t.Fatal(err)
	}
	if defaultLimitResult.Candidates[0].Labels["limit"] != "20" {
		t.Fatalf("default discovery limit = %#v", defaultLimitResult.Candidates[0].Labels)
	}
}

func TestEndpointDiscoverPluginReturnsSinglePluginError(t *testing.T) {
	pluginDir := writeFailingEndpointDiscoverPlugin(t)
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "kubernetes=" + pluginDir, "endpoint", "discover", "mysql", "--plugin", "kubernetes", "-o", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected discovery error")
	}
	if !strings.Contains(err.Error(), "kubeconfig missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseEndpointSelection(t *testing.T) {
	selected, err := parseEndpointSelection("1,3-5,3")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected, []int{1, 3, 4, 5}) {
		t.Fatalf("selected = %#v", selected)
	}
}

func writeFakeSQLPlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakesql\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-sql")
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
		Command string
		Payload json.RawMessage
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "sql",
				"operations": []map[string]any{{"name": "sql.query", "read_only": true}},
			},
		})
		return
	}
	var call struct {
		Name string
		Input json.RawMessage
	}
	_ = json.Unmarshal(req.Payload, &call)
	var input map[string]any
	_ = json.Unmarshal(call.Input, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{
			"endpoint_ref": input["endpoint_ref"],
			"endpoint_url": input["url"],
			"driver": input["endpoint_product"],
			"columns": []string{"ok"},
			"row_count": 1,
			"rows": []map[string]any{{"ok": 1}},
		},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func writeFakeKubernetesTestPlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakekubernetestest\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-kubernetes")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
)

func main() {
	var req struct {
		Command string
		Payload json.RawMessage
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "kubernetes",
				"operations": []map[string]any{{"name": "kubernetes.cluster.test", "read_only": true}},
			},
		})
		return
	}
	var call struct {
		Name string
		Input json.RawMessage
	}
	_ = json.Unmarshal(req.Payload, &call)
	var input map[string]any
	_ = json.Unmarshal(call.Input, &input)
	rawURL, _ := input["url"].(string)
	parsed, _ := url.Parse(rawURL)
	contextName := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Host != "context" {
		contextName = parsed.Host
	}
	contextName, _ = url.PathUnescape(contextName)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{
			"context": contextName,
			"ok": true,
			"server_version": "v1.30.0",
		},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func writeFakeKubernetesAliasPlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakekubernetesalias\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-kubernetes")
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
		Command string
		Payload json.RawMessage
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "kubernetes",
				"aliases": []string{"kube", "k8s", "kubernetes"},
				"operations": []map[string]any{
					{"name": "kubernetes.service.show", "read_only": true, "input_schema": map[string]any{"required": []string{"name"}, "properties": map[string]any{"name": map[string]any{"type": "string"}, "context": map[string]any{"type": "string"}}}},
					{"name": "kubernetes.pod.logs", "read_only": true, "input_schema": map[string]any{"required": []string{"name"}, "properties": map[string]any{"name": map[string]any{"type": "string"}, "container": map[string]any{"type": "string"}, "tail_lines": map[string]any{"type": "integer"}, "timestamps": map[string]any{"type": "boolean"}, "endpoint_ref": map[string]any{"type": "string"}}}},
				},
			},
		})
		return
	}
	var call struct {
		Name string
		Input json.RawMessage
	}
	_ = json.Unmarshal(req.Payload, &call)
	var input map[string]any
	_ = json.Unmarshal(call.Input, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"name": call.Name, "input": input},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func writeFakeOperationPlugin(t *testing.T, name string, aliases []string, operations []map[string]any) string {
	t.Helper()
	return writeFakeManifestPlugin(t, name, map[string]any{"name": name, "aliases": aliases, "operations": operations})
}

func writeFakeManifestPlugin(t *testing.T, name string, manifest map[string]any) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fake"+name+"\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-"+name)
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"os"
)

const manifestJSON = ` + strconv.Quote(string(manifestJSON)) + `

func main() {
	var req struct {
		Command string
		Payload json.RawMessage
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		var manifest map[string]any
		_ = json.Unmarshal([]byte(manifestJSON), &manifest)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": "dex.plugin.v1", "ok": true, "result": manifest})
		return
	}
	if req.Command == "auth.methods" {
		var manifest map[string]any
		_ = json.Unmarshal([]byte(manifestJSON), &manifest)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": "dex.plugin.v1", "ok": true, "result": manifest["auth"]})
		return
	}
	var call struct {
		Name string
		Input json.RawMessage
	}
	_ = json.Unmarshal(req.Payload, &call)
	var input map[string]any
	_ = json.Unmarshal(call.Input, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"name": call.Name, "input": input},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func writeFakeKubernetesDatasourcePlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakekubernetesdatasource\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-kubernetes")
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
		Payload json.RawMessage ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "kubernetes",
				"datasources": []map[string]any{{
					"name": "kubernetes.inventory",
					"entity": "kubernetes.resource",
					"capabilities": []string{"search"},
				}},
			},
		})
		return
	}
	var input map[string]any
	_ = json.Unmarshal(req.Payload, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"source": "kubernetes", "count": 1, "records": []map[string]any{{"entity": "kubernetes.resource", "id": "latest/api"}}, "input": input},
	})
}

`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func executeGeneratedRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	opts := newOptions()
	opts.home = t.TempDir()
	if err := parseStartupFlags(args, opts); err != nil {
		return "", err
	}
	cmd := newRootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := attachGeneratedPluginCommands(context.Background(), cmd, opts); err != nil {
		return out.String(), err
	}
	err := cmd.Execute()
	return out.String(), err
}

func writeFakeSlackDatasourcePlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakeslackdatasource\n\ngo 1.26\n"), 0o600); err != nil {
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

func main() {
	var req struct {
		Command string ` + "`json:\"command\"`" + `
		Payload json.RawMessage ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "slack",
				"datasources": []map[string]any{{
					"name": "slack.messages",
					"entity": "slack.message",
					"capabilities": []string{"search"},
				}, {
					"name": "slack.thread_messages",
					"entity": "slack.thread_message",
					"capabilities": []string{"search"},
				}, {
					"name": "slack.channel_members",
					"entity": "slack.channel_member",
					"capabilities": []string{"search"},
					"entity_schema": map[string]any{"fields": []map[string]any{
						{"name": "title"},
						{"name": "channel"},
						{"name": "user_id"},
						{"name": "name"},
						{"name": "real_name"},
						{"name": "display_name"},
						{"name": "email"},
					}},
					"relations": []map[string]any{{"field": "user_id", "entity": "slack.user", "type": "reference"}},
				}},
			},
		})
		return
	}
	var input map[string]any
	_ = json.Unmarshal(req.Payload, &input)
	source := "slack.messages"
	records := []map[string]any{{"entity": "slack.message", "id": "C1:1710000000.123456"}}
	if input["datasource"] == "slack.channel_members" {
		source = "slack.channel_members"
		records = []map[string]any{{"entity": "slack.channel_member", "id": "C1:U123", "title": "U123", "channel": "C1", "user_id": "U123"}}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"source": source, "count": len(records), "records": records, "input": input},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func writeFailingEndpointDiscoverPlugin(t *testing.T) string {
	t.Helper()
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module failingkubernetes\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-kubernetes")
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
		Command string
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "kubernetes",
				"endpoints": []map[string]any{{"name": "kubernetes.endpoint.discover", "products": []string{"mysql"}}},
			},
		})
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": false,
		"error": map[string]any{"code": "kubernetes", "message": "kubeconfig missing"},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func TestOperationShowIncludesMetadata(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "system=../../plugins/system", "op", "show", "system.info", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result core.OperationSpec
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "system.info" || result.Risk != core.OperationRiskLow || result.Idempotency != core.OperationIdempotent {
		t.Fatalf("operation = %#v", result)
	}
	if len(result.Effects) == 0 || result.Effects[0] != core.OperationEffectRead {
		t.Fatalf("effects = %#v", result.Effects)
	}
}

func TestGeneratedCommandExecutesActivePluginOperation(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	home := state.Home
	out, err := executeGeneratedRoot(t, "--dex-home", home, "--dev-plugin", "kubernetes="+pluginDir, "kube", "service", "show", "latest/api", "--context", "dev", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "kubernetes.service.show" {
		t.Fatalf("result = %#v", result)
	}
	input, _ := result["input"].(map[string]any)
	if input["name"] != "latest/api" || input["context"] != "dev" {
		t.Fatalf("input = %#v", input)
	}
}

func TestRootDispatchesActivePluginAlias(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	home := t.TempDir()
	activate := NewRootCommand()
	var activateOut bytes.Buffer
	activate.SetOut(&activateOut)
	activate.SetErr(&activateOut)
	activate.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "plugin", "activate", "kubernetes"})
	if err := activate.Execute(); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", home, "--dev-plugin", "kubernetes="+pluginDir, "kube", "service", "show", "latest/api", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	input, _ := result["input"].(map[string]any)
	if result["name"] != "kubernetes.service.show" || input["name"] != "latest/api" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGeneratedCommandExecutesOperationWithFlags(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev-kubernetes", URL: "kubernetes://context/dev", Product: "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "kube", "pod", "logs", "latest/api-123", "--container", "api", "--tail-lines", "25", "--timestamps", "--endpoint-ref", "dev-kubernetes", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "kubernetes.pod.logs" {
		t.Fatalf("result = %#v", result)
	}
	input, _ := result["input"].(map[string]any)
	if input["name"] != "latest/api-123" || input["container"] != "api" || input["tail_lines"] != float64(25) || input["timestamps"] != true || input["endpoint_ref"] != "dev-kubernetes" {
		t.Fatalf("input = %#v", input)
	}
}

func TestGeneratedCommandAcceptsJSONObjectInput(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "k8s", "pod", "logs", `{"name":"latest/api-123","container":"api"}`, "--tail-lines", "10", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	input, _ := result["input"].(map[string]any)
	if input["name"] != "latest/api-123" || input["container"] != "api" || input["tail_lines"] != float64(10) {
		t.Fatalf("input = %#v", input)
	}
}

func TestGeneratedCommandHelpShowsSchemaFlags(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "kube", "pod", "logs", "-h")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--container") || !strings.Contains(out, "--tail-lines") || !strings.Contains(out, "--endpoint-ref") {
		t.Fatalf("help missing schema flags:\n%s", out)
	}
	if strings.Contains(out, "--context") {
		t.Fatalf("help includes unrelated operation flag:\n%s", out)
	}
}

func TestSkillRendersActivatedPluginCommands(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "skill")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Dex Skill", "## kubernetes", "`dex kube pod logs`", "`--endpoint-ref`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("skill output missing %q:\n%s", want, out)
		}
	}
}

func TestSkillSkipsPluginUntilRequiredAuthConfigured(t *testing.T) {
	pluginDir := writeFakeManifestPlugin(t, "gitlab", map[string]any{
		"name":        "gitlab",
		"description": "GitLab test plugin.",
		"auth": []map[string]any{{
			"name": "token",
			"kind": "bearer_token",
			"fields": []map[string]any{{
				"name":     "access_token",
				"required": true,
				"env":      []string{"GITLAB_TOKEN"},
			}},
		}},
		"operations": []map[string]any{{
			"name":        "gitlab.project.list",
			"description": "List GitLab projects.",
			"input_schema": map[string]any{
				"properties": map[string]any{"limit": map[string]any{"type": "integer"}},
			},
		}},
	})
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "gitlab="+pluginDir, "skill")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## gitlab") {
		t.Fatalf("skill output included plugin without required auth:\n%s", out)
	}
	if err := state.SaveSecret("gitlab", runtime.DefaultInstance, "access_token", runtime.StoredSecret{Value: "token"}); err != nil {
		t.Fatal(err)
	}
	out, err = executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "gitlab="+pluginDir, "skill")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## gitlab", "`access_token`: stored", "`dex gitlab project list`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("skill output missing %q:\n%s", want, out)
		}
	}
}

func TestSkillInstallWritesDexHomeSkillAndReferences(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{Name: "kubernetes", Description: "Kubernetes test plugin.", Binary: "dex-plugin-kubernetes", LocalPath: pluginDir}
	if err := state.ActivatePlugin(entry); err != nil {
		t.Fatal(err)
	}
	marketplacePath := filepath.Join(t.TempDir(), "marketplace.json")
	marketplaceData, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
		entry,
		{Name: "gitlab", Description: "GitLab test plugin.", Binary: "dex-plugin-gitlab", GoInstall: "example.com/gitlab@latest"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplacePath, marketplaceData, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--marketplace", marketplacePath, "--dev-plugin", "kubernetes="+pluginDir, "skill", "install", "--no-claude-link", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Dir        string   `json:"dir"`
		Main       string   `json:"main"`
		References []string `json:"references"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(state.Home, "skills", "dex")
	if result.Dir != wantDir || result.Main != filepath.Join(wantDir, "SKILL.md") {
		t.Fatalf("install result = %#v", result)
	}
	main, err := os.ReadFile(filepath.Join(wantDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(main, []byte("[kubernetes](references/kubernetes.md)")) || !bytes.Contains(main, []byte("[gitlab](references/gitlab.md)")) {
		t.Fatalf("main skill missing references:\n%s", string(main))
	}
	if !bytes.Contains(main, []byte("[kubernetes](references/kubernetes.md) - installed, activated")) {
		t.Fatalf("main skill missing installed label:\n%s", string(main))
	}
	if !bytes.Contains(main, []byte("## Installed and Active Integration References")) || !bytes.Contains(main, []byte("## Marketplace References")) || !bytes.Contains(main, []byte("available to install")) {
		t.Fatalf("main skill missing active/marketplace split:\n%s", string(main))
	}
	for _, want := range [][]byte{
		[]byte("## Patterns"),
		[]byte("Discover -> import -> use"),
		[]byte("`--endpoint-ref <friendly-id>`"),
		[]byte("Top-level subcommands: plugin, op, datasource"),
		[]byte("Integration commands: kube"),
		[]byte("Empty `null`, map, or list responses can be valid"),
		[]byte("`xxxxx`"),
	} {
		if !bytes.Contains(main, want) {
			t.Fatalf("main skill missing %q:\n%s", string(want), string(main))
		}
	}
	if bytes.Contains(main, []byte("Usage:\n  dex")) {
		t.Fatalf("main skill still embeds full dex help:\n%s", string(main))
	}
	kubeRef, err := os.ReadFile(filepath.Join(wantDir, "references", "kubernetes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(kubeRef, []byte("`dex kube pod logs`")) {
		t.Fatalf("kubernetes reference missing dynamic command:\n%s", string(kubeRef))
	}
	if bytes.Contains(kubeRef, []byte("dex kubernetes, dex kube, dex kubernetes")) || strings.Count(string(kubeRef), "`dex kubernetes`") != 1 {
		t.Fatalf("kubernetes reference has duplicate aliases:\n%s", string(kubeRef))
	}
	gitlabRef, err := os.ReadFile(filepath.Join(wantDir, "references", "gitlab.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(gitlabRef, []byte("dex plugin install gitlab")) || !bytes.Contains(gitlabRef, []byte("Dynamic plugin metadata was unavailable")) {
		t.Fatalf("gitlab reference missing install guidance:\n%s", string(gitlabRef))
	}
}

func TestSkillInstallUsesAbsoluteClaudeSymlinkTarget(t *testing.T) {
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	opts := newOptions()
	opts.home = state.Home
	runner, err := opts.runner()
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	t.Chdir(cwd)
	relativeDir := filepath.Join("relative-skill-root", "dex")
	result, err := writeDexSkillInstall(context.Background(), opts, runner, relativeDir, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(home, ".claude", "skills", "dex"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cwd, relativeDir)
	if target != want || result.ClaudeLink == "" || !result.Linked {
		t.Fatalf("target = %q, want %q; result = %#v", target, want, result)
	}
}

func TestGeneratedSlackInfoCommand(t *testing.T) {
	pluginDir := writeFakeOperationPlugin(t, "slack", nil, []map[string]any{
		{"name": "slack.info", "read_only": true, "input_schema": map[string]any{"properties": map[string]any{}}},
	})
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "slack", Binary: "dex-plugin-slack", LocalPath: pluginDir}); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "slack="+pluginDir, "slack", "info", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "slack.info" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGeneratedDuplicateAliasesFail(t *testing.T) {
	kubeDir := writeFakeKubernetesAliasPlugin(t)
	otherDir := writeFakeOperationPlugin(t, "other", []string{"kube"}, []map[string]any{
		{"name": "other.info", "read_only": true, "input_schema": map[string]any{"properties": map[string]any{}}},
	})
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: kubeDir}); err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "other", Binary: "dex-plugin-other", LocalPath: otherDir}); err != nil {
		t.Fatal(err)
	}
	marketplaceData, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{
		{Name: "kubernetes", Binary: "dex-plugin-kubernetes", LocalPath: kubeDir},
		{Name: "other", Binary: "dex-plugin-other", LocalPath: otherDir},
	}})
	if err != nil {
		t.Fatal(err)
	}
	marketplacePath := filepath.Join(t.TempDir(), "marketplace.json")
	if err := os.WriteFile(marketplacePath, marketplaceData, 0o600); err != nil {
		t.Fatal(err)
	}
	opts := newOptions()
	opts.home = home
	opts.marketplacePath = marketplacePath
	opts.devPlugins = []string{"kubernetes=" + kubeDir, "other=" + otherDir}
	cmd := newRootCommand(opts)
	err = attachGeneratedPluginCommands(context.Background(), cmd, opts)
	if err == nil || !strings.Contains(err.Error(), "duplicate plugin command alias") {
		t.Fatalf("duplicate alias err = %#v", err)
	}
}

func TestGeneratedCommandRequiresActivePlugin(t *testing.T) {
	pluginDir := writeFakeKubernetesAliasPlugin(t)
	state, err := runtime.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev-cluster", URL: "kubernetes://context/dev", Product: "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	_, err = executeGeneratedRoot(t, "--dex-home", state.Home, "--dev-plugin", "kubernetes="+pluginDir, "kube", "pod", "logs", "latest/api-123")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("inactive alias err = %#v", err)
	}
}

func TestUnavailableMarketplacePluginCommandReturnsInstallHint(t *testing.T) {
	marketplacePath := filepath.Join(t.TempDir(), "marketplace.json")
	marketplaceData, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
		Name:        "gitlab",
		Description: "GitLab test plugin.",
		Binary:      "dex-plugin-gitlab",
		GoInstall:   "example.com/gitlab@latest",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplacePath, marketplaceData, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := executeGeneratedRoot(t, "--marketplace", marketplacePath, "gitlab", "project", "list")
	if err == nil {
		t.Fatalf("expected unavailable plugin command to fail, output=%s", out)
	}
	if !strings.Contains(err.Error(), `plugin "gitlab" is not installed`) || !strings.Contains(err.Error(), "dex plugin install gitlab") {
		t.Fatalf("unavailable plugin err = %v, output=%s", err, out)
	}
}

func TestSearchPassesEndpointToDatasource(t *testing.T) {
	pluginDir := writeFakeKubernetesDatasourcePlugin(t)
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev-kubernetes", URL: "kubernetes://context/dev", Product: "kubernetes", Protocol: "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "search", "--plugin", "kubernetes", "api", "--endpoint", "dev-kubernetes", "--limit", "5", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results struct {
			Available map[string]struct {
				Input map[string]any `json:"input"`
			} `json:"available"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	input := result.Results.Available["kubernetes"].Input
	if input["endpoint_ref"] != "dev-kubernetes" || input["url"] != "kubernetes://context/dev" || input["query"] != "api" || input["limit"] != float64(5) {
		t.Fatalf("input = %#v", input)
	}
}

func TestDatasourceCommandsUseExactDatasourceName(t *testing.T) {
	pluginDir := writeFakeKubernetesDatasourcePlugin(t)
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev-kubernetes", URL: "kubernetes://context/dev", Product: "kubernetes", Protocol: "kubernetes"}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "datasource", "show", "kubernetes.inventory", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"name": "kubernetes.inventory"`)) {
		t.Fatalf("show output = %s", out.String())
	}

	out.Reset()
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "datasource", "search", "kubernetes.inventory", `{"query":"api","endpoint_ref":"dev-kubernetes"}`, "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Input["datasource"] != "kubernetes.inventory" || result.Input["entity"] != "kubernetes.resource" || result.Input["url"] != "kubernetes://context/dev" {
		t.Fatalf("input = %#v", result.Input)
	}

	out.Reset()
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "kubernetes=" + pluginDir, "datasource", "search", "kubernetes.inventory", "api", "--endpoint-ref", "dev-kubernetes", "--limit", "5", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	result = struct {
		Input map[string]any `json:"input"`
	}{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Input["query"] != "api" || result.Input["limit"] != float64(5) || result.Input["endpoint_ref"] != "dev-kubernetes" {
		t.Fatalf("flag input = %#v", result.Input)
	}
}

func TestSlackDatasourceCommandsUseExactDatasourceName(t *testing.T) {
	pluginDir := writeFakeSlackDatasourcePlugin(t)
	home := t.TempDir()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "datasource", "show", "slack.messages", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"name": "slack.messages"`)) || !bytes.Contains(out.Bytes(), []byte(`"entity": "slack.message"`)) {
		t.Fatalf("show output = %s", out.String())
	}

	out.Reset()
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "datasource", "search", "slack.messages", `{"query":"incident"}`, "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Input["datasource"] != "slack.messages" || result.Input["entity"] != "slack.message" || result.Input["query"] != "incident" {
		t.Fatalf("input = %#v", result.Input)
	}

	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "datasource", "search", "slack.messages", `{"entity":"slack.thread_message"}`})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `datasource "slack.messages" exposes entity "slack.message"`) {
		t.Fatalf("err = %v, output = %s", err, out.String())
	}
}

func TestSlackChannelMembersDatasourceEnrichesFromUserIndex(t *testing.T) {
	pluginDir := writeFakeSlackDatasourcePlugin(t)
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("slack", "default", "slack.users", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.user","id":"U123","title":"Timo Friedl","user_id":"U123","name":"timo","real_name":"Timo Friedl","display_name":"Timo","email":"timo@example.com","web_url":"slack://user/U123"}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "datasource", "search", "slack.channel_members", `{"channel":"C1","limit":1}`, "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Source  string `json:"source"`
		Records []struct {
			Entity      string `json:"entity"`
			ID          string `json:"id"`
			Title       string `json:"title"`
			UserID      string `json:"user_id"`
			Name        string `json:"name"`
			RealName    string `json:"real_name"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"records"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Source != "slack.channel_members" || len(result.Records) != 1 {
		t.Fatalf("result = %#v", result)
	}
	record := result.Records[0]
	if record.Entity != "slack.channel_member" || record.ID != "C1:U123" || record.UserID != "U123" {
		t.Fatalf("record identity = %#v", record)
	}
	if record.Title != "Timo Friedl" || record.Name != "timo" || record.RealName != "Timo Friedl" || record.DisplayName != "Timo" || record.Email != "timo@example.com" {
		t.Fatalf("record was not enriched from slack.users index: %#v", record)
	}
}

func TestDatasourceSearchRejectsUnknownAndConflictingEntity(t *testing.T) {
	pluginDir := writeFakeKubernetesDatasourcePlugin(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown", args: []string{"--dev-plugin", "kubernetes=" + pluginDir, "datasource", "search", "kubernetes.missing", `{}`}, want: `unknown datasource "kubernetes.missing"`},
		{name: "entity", args: []string{"--dev-plugin", "kubernetes=" + pluginDir, "datasource", "search", "kubernetes.inventory", `{"entity":"other.resource"}`}, want: `exposes entity "kubernetes.resource"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append([]string{"--dex-home", t.TempDir()}, tc.args...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, output = %s", err, out.String())
			}
		})
	}
}

func TestSearchRejectsProviderSpecificScopeFlags(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "search", "--plugin", "kubernetes", "api", "--namespace", "latest", "-o", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected provider-specific search flag to fail")
	}
	if !strings.Contains(err.Error(), "unknown flag: --namespace") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestContextCommandReturnsBlocks(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "system=../../plugins/system", "context", "debug host", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"blocks"`)) || !bytes.Contains(out.Bytes(), []byte(`"plugin": "system"`)) {
		t.Fatalf("context output missing block/source:\n%s", out.String())
	}
}

func TestPluginShowIncludesDatasourceMetadata(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "gitlab=../../plugins/gitlab", "plugin", "show", "gitlab", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"entity_schema"`)) || !bytes.Contains(out.Bytes(), []byte(`"fallback": "host_index_first"`)) {
		t.Fatalf("plugin show missing datasource metadata:\n%s", out.String())
	}
}

func TestWebsearchCommandSurfacesOperationFailure(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "op", "run", "websearch.search", `{"query":"fluxplane dex","providers":["missing-provider"]}`, "-o", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected websearch command to fail, output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), `web search provider "missing-provider" is not available`) {
		t.Fatalf("error = %q", err.Error())
	}
	if strings.TrimSpace(out.String()) == "{}" {
		t.Fatalf("failure rendered empty object")
	}
}

func TestSecretGetRejectsMissingGrant(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "secret", "get", "gitlab", "--purpose", "access_token", "--grant", "missing"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing grant to fail")
	}
}

func TestAuthConnectAutoGitLabStoresEnvWithoutPrintingValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GITLAB_PERSONAL_TOKEN", "super-secret-token")
	t.Setenv("GITLAB_ACCESS_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=../../plugins/gitlab", "--instance", "work", "auth", "connect", "auto", "gitlab", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("super-secret-token")) || bytes.Contains(out.Bytes(), []byte("https://gitlab.example.com")) {
		t.Fatalf("auto-connect output printed secret material:\n%s", out.String())
	}
	var result struct {
		Plugin   string   `json:"plugin"`
		Instance string   `json:"instance"`
		Saved    []string `json:"saved"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plugin != "gitlab" || result.Instance != "work" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !containsString(result.Saved, "access_token") {
		t.Fatalf("saved fields = %#v", result.Saved)
	}

	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	status := state.SecretStatus("gitlab", "work", []runtime.SecretPurpose{{Name: "access_token"}})
	if status["access_token"] != "stored" {
		t.Fatalf("work status = %#v", status)
	}
	defaultStatus := state.SecretStatus("gitlab", "default", []runtime.SecretPurpose{{Name: "access_token"}})
	if defaultStatus["access_token"] == "stored" {
		t.Fatalf("default instance should not receive work secrets: %#v", defaultStatus)
	}
}

func TestAuthConnectAutoGitLabReportsMissingFields(t *testing.T) {
	t.Setenv("GITLAB_PERSONAL_TOKEN", "")
	t.Setenv("GITLAB_ACCESS_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "gitlab=../../plugins/gitlab", "auth", "connect", "auto", "gitlab", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Saved   []string `json:"saved"`
		Missing []string `json:"missing"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Saved) != 0 {
		t.Fatalf("saved fields = %#v", result.Saved)
	}
	if !containsString(result.Missing, "access_token") {
		t.Fatalf("missing fields = %#v", result.Missing)
	}
}

func TestAuthConnectPartialDoesNotMarkPluginAvailable(t *testing.T) {
	home := t.TempDir()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=../../plugins/slack", "auth", "connect", "slack", "--field", "bot_token=xoxb-test", "-o", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected partial required auth to fail")
	}
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := state.IsPluginInstalled("slack")
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("partial auth should not mark plugin installed/available")
	}
}

func TestOperationRunRoutesInput(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakegitlab\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-gitlab")
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
		Payload struct {
			Name  string          ` + "`json:\"name\"`" + `
			Input json.RawMessage ` + "`json:\"input\"`" + `
		} ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	var input any
	_ = json.Unmarshal(req.Payload.Input, &input)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{"name": req.Payload.Name, "input": input},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--dex-home", t.TempDir(),
		"--dev-plugin", "gitlab=" + pluginDir,
		"op", "run", "gitlab.mr.list",
		`{"project":"group/dex","state":"merged","search":"ship","limit":7}`,
		"-o", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Name  string `json:"name"`
		Input struct {
			Project string  `json:"project"`
			State   string  `json:"state"`
			Search  string  `json:"search"`
			Limit   float64 `json:"limit"`
		} `json:"input"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "gitlab.mr.list" {
		t.Fatalf("operation name = %q", result.Name)
	}
	if result.Input.Project != "group/dex" || result.Input.State != "merged" || result.Input.Search != "ship" || result.Input.Limit != 7 {
		t.Fatalf("operation input = %#v", result.Input)
	}
}

func TestPluginEnvDoesNotLeakHostSecretsAndDecodedErrorsReturn(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakeenv\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-gitlab")
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
			Name string ` + "`json:\"name\"`" + `
		} ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	if req.Command == "manifest" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": true,
			"result": map[string]any{
				"name": "gitlab",
				"operations": []map[string]any{{"name": "gitlab.check"}},
			},
		})
		return
	}
	if req.Payload.Name == "gitlab.fail" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"protocol": "dex.plugin.v1",
			"ok": false,
			"error": map[string]any{"code": "bad_input", "message": "specific plugin failure"},
		})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": map[string]any{
			"gitlab": os.Getenv("GITLAB_PERSONAL_TOKEN"),
			"slack": os.Getenv("SLACK_BOT_TOKEN"),
			"host": os.Getenv("DEX_HOST_CMD") != "",
		},
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITLAB_PERSONAL_TOKEN", "gitlab-secret")
	t.Setenv("SLACK_BOT_TOKEN", "slack-secret")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "gitlab=" + pluginDir, "op", "run", "gitlab.check", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		GitLab string `json:"gitlab"`
		Slack  string `json:"slack"`
		Host   bool   `json:"host"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.GitLab != "" || result.Slack != "" || result.Host {
		t.Fatalf("unexpected plugin env: %#v", result)
	}

	fail := NewRootCommand()
	var failOut bytes.Buffer
	fail.SetOut(&failOut)
	fail.SetErr(&failOut)
	fail.SetArgs([]string{"--dex-home", t.TempDir(), "--dev-plugin", "gitlab=" + pluginDir, "op", "run", "gitlab.fail", "-o", "json"})
	err := fail.Execute()
	if err == nil {
		t.Fatal("expected plugin failure")
	}
	if err.Error() != "specific plugin failure" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestIndexBuildStoresRecordsForSearch(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakegitlab\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-gitlab")
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
	result := map[string]any{}
	if req.Command == "manifest" {
		result = map[string]any{
			"name": "gitlab",
			"datasources": []map[string]any{{
				"name": "gitlab.projects",
				"entity": "gitlab.project",
				"capabilities": []string{"search", "lookup", "get", "index"},
			}},
		}
	} else if req.Command == "operations.call" {
		result = map[string]any{
			"index": "gitlab.projects",
			"records": []map[string]any{{
				"entity": "gitlab.project",
				"id": "sbf/manager-v2",
				"name": "manager-v2",
				"path_with_namespace": "sbf/manager-v2",
				"web_url": "https://gitlab.example.com/sbf/manager-v2",
			}},
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"protocol": "dex.plugin.v1",
		"ok": true,
		"result": result,
	})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	build := NewRootCommand()
	var buildOut bytes.Buffer
	build.SetOut(&buildOut)
	build.SetErr(&buildOut)
	build.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=" + pluginDir, "index", "build", "gitlab", "-o", "json"})
	if err := build.Execute(); err != nil {
		t.Fatal(err)
	}
	var buildResult struct {
		Stored  bool   `json:"stored"`
		Records int    `json:"records"`
		Index   string `json:"index"`
	}
	if err := json.Unmarshal(buildOut.Bytes(), &buildResult); err != nil {
		t.Fatal(err)
	}
	if !buildResult.Stored || buildResult.Records != 1 || buildResult.Index != "gitlab.projects" {
		t.Fatalf("build result = %#v", buildResult)
	}

	search := NewRootCommand()
	var searchOut bytes.Buffer
	search.SetOut(&searchOut)
	search.SetErr(&searchOut)
	search.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=" + pluginDir, "search", "manager", "-o", "json"})
	if err := search.Execute(); err != nil {
		t.Fatal(err)
	}
	var searchResult struct {
		Results struct {
			Available map[string]struct {
				Source  string `json:"source"`
				Count   int    `json:"count"`
				Records []struct {
					ID            string            `json:"id"`
					URL           string            `json:"url"`
					Links         map[string]string `json:"links"`
					Score         int               `json:"score"`
					MatchedFields []string          `json:"matched_fields"`
					Origin        struct {
						Plugin   string `json:"plugin"`
						Instance string `json:"instance"`
						Index    string `json:"index"`
						Source   string `json:"source"`
					} `json:"origin"`
				} `json:"records"`
			} `json:"available"`
		} `json:"results"`
	}
	if err := json.Unmarshal(searchOut.Bytes(), &searchResult); err != nil {
		t.Fatal(err)
	}
	gitlab := searchResult.Results.Available["gitlab"]
	if gitlab.Source != "host_index" || gitlab.Count != 1 || gitlab.Records[0].ID != "sbf/manager-v2" {
		t.Fatalf("search result = %#v", gitlab)
	}
	if gitlab.Records[0].URL != "https://gitlab.example.com/sbf/manager-v2" || gitlab.Records[0].Links["namespace"] != "https://gitlab.example.com/sbf" {
		t.Fatalf("search links = %#v", gitlab.Records[0])
	}
	if gitlab.Records[0].Score == 0 || len(gitlab.Records[0].MatchedFields) == 0 {
		t.Fatalf("search score = %#v", gitlab.Records[0])
	}
	if gitlab.Records[0].Origin.Plugin != "gitlab" || gitlab.Records[0].Origin.Instance != "default" || gitlab.Records[0].Origin.Index != "gitlab.projects" || gitlab.Records[0].Origin.Source != "host_index" {
		t.Fatalf("search origin = %#v", gitlab.Records[0].Origin)
	}

	lookup := NewRootCommand()
	var lookupOut bytes.Buffer
	lookup.SetOut(&lookupOut)
	lookup.SetErr(&lookupOut)
	lookup.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=" + pluginDir, "lookup", "open https://gitlab.example.com/sbf/manager-v2", "-o", "json"})
	if err := lookup.Execute(); err != nil {
		t.Fatal(err)
	}
	var lookupResult struct {
		Results struct {
			Available map[string]struct {
				Source  string `json:"source"`
				Count   int    `json:"count"`
				Matches []struct {
					Entity string `json:"entity"`
					ID     string `json:"id"`
					Source struct {
						Plugin string `json:"plugin"`
						Index  string `json:"index"`
					} `json:"source"`
					Record struct {
						Entity string            `json:"entity"`
						ID     string            `json:"id"`
						URL    string            `json:"url"`
						Links  map[string]string `json:"links"`
						Origin struct {
							Plugin string `json:"plugin"`
							Index  string `json:"index"`
						} `json:"origin"`
					} `json:"record"`
				} `json:"matches"`
			} `json:"available"`
		} `json:"results"`
	}
	if err := json.Unmarshal(lookupOut.Bytes(), &lookupResult); err != nil {
		t.Fatal(err)
	}
	lookupGitLab := lookupResult.Results.Available["gitlab"]
	if lookupGitLab.Source != "host_index" || lookupGitLab.Count != 1 || lookupGitLab.Matches[0].ID != "sbf/manager-v2" {
		t.Fatalf("lookup result = %#v", lookupGitLab)
	}
	if lookupGitLab.Matches[0].Source.Plugin != "gitlab" || lookupGitLab.Matches[0].Source.Index != "gitlab.projects" {
		t.Fatalf("lookup source = %#v", lookupGitLab.Matches[0].Source)
	}
	if lookupGitLab.Matches[0].Record.Entity != "gitlab.project" || lookupGitLab.Matches[0].Record.ID != "sbf/manager-v2" || lookupGitLab.Matches[0].Record.URL == "" || lookupGitLab.Matches[0].Record.Links["self"] == "" {
		t.Fatalf("lookup standardized record = %#v", lookupGitLab.Matches[0].Record)
	}
	if lookupGitLab.Matches[0].Record.Origin.Plugin != "gitlab" || lookupGitLab.Matches[0].Record.Origin.Index != "gitlab.projects" {
		t.Fatalf("lookup record origin = %#v", lookupGitLab.Matches[0].Record.Origin)
	}
}

func TestSearchAndLookupHonorInstance(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakegitlab\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-gitlab")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct{ Command string ` + "`json:\"command\"`" + ` }
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	result := map[string]any{}
	if req.Command == "manifest" {
		result = map[string]any{
			"name": "gitlab",
			"datasources": []map[string]any{{
				"name": "gitlab.projects",
				"entity": "gitlab.project",
				"capabilities": []string{"search", "lookup", "get", "index"},
			}},
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":result})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.projects", []json.RawMessage{json.RawMessage(`{"entity":"gitlab.project","id":"default/project","name":"manager","web_url":"https://gitlab.example.com/default/project"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "work", "gitlab.projects", []json.RawMessage{json.RawMessage(`{"entity":"gitlab.project","id":"work/project","name":"manager","web_url":"https://gitlab.example.com/work/project"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := state.ActivatePlugin(core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab"}); err != nil {
		t.Fatal(err)
	}

	search := NewRootCommand()
	var searchOut bytes.Buffer
	search.SetOut(&searchOut)
	search.SetErr(&searchOut)
	search.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=" + pluginDir, "--instance", "work", "search", "manager", "-o", "json"})
	if err := search.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(searchOut.Bytes(), []byte("work/project")) || bytes.Contains(searchOut.Bytes(), []byte("default/project")) {
		t.Fatalf("search did not honor instance:\n%s", searchOut.String())
	}

	lookup := NewRootCommand()
	var lookupOut bytes.Buffer
	lookup.SetOut(&lookupOut)
	lookup.SetErr(&lookupOut)
	lookup.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=" + pluginDir, "--instance", "work", "lookup", "https://gitlab.example.com/work/project", "-o", "json"})
	if err := lookup.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(lookupOut.Bytes(), []byte("work/project")) || bytes.Contains(lookupOut.Bytes(), []byte("default/project")) {
		t.Fatalf("lookup did not honor instance:\n%s", lookupOut.String())
	}
}

func TestSlackIndexCommandStoresSeparateIndexes(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakeslack\n\ngo 1.26\n"), 0o600); err != nil {
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

func main() {
	var req struct {
		Command string ` + "`json:\"command\"`" + `
		Payload struct {
			Name string ` + "`json:\"name\"`" + `
		} ` + "`json:\"payload\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	result := map[string]any{}
	if req.Command == "manifest" {
		result = map[string]any{
			"name": "slack",
			"operations": []map[string]any{{
				"name": "slack.index.build",
				"secret_purposes": []string{"user_token", "bot_token"},
			}},
			"auth": []map[string]any{{
				"fields": []map[string]any{
					{"name": "user_token", "env": []string{"SLACK_USER_TOKEN"}},
					{"name": "bot_token", "env": []string{"SLACK_BOT_TOKEN"}},
				},
			}},
			"datasources": []map[string]any{
				{"name": "slack.users", "entity": "slack.user", "capabilities": []string{"search", "lookup", "get", "index"}},
				{"name": "slack.channels", "entity": "slack.channel", "capabilities": []string{"search", "lookup", "get", "index"}},
			},
		}
	} else if req.Command == "operations.call" {
		result = map[string]any{
			"indexes": []map[string]any{
				{"index": "slack.users", "records": []map[string]any{{
					"entity": "slack.user",
					"id": "U123",
					"title": "Timo Friedl",
					"user_id": "U123",
					"name": "timo",
					"display_name": "Timo",
					"real_name": "Timo Friedl",
					"web_url": "slack://user/U123",
				}}},
				{"index": "slack.channels", "records": []map[string]any{{
					"entity": "slack.channel",
					"id": "C123",
					"title": "general",
					"channel_id": "C123",
					"name": "general",
					"web_url": "slack://channel/C123",
				}}},
			},
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":result})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot")
	build := NewRootCommand()
	var buildOut bytes.Buffer
	build.SetOut(&buildOut)
	build.SetErr(&buildOut)
	build.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "index", "build", "slack", "-o", "json"})
	if err := build.Execute(); err != nil {
		t.Fatal(err)
	}
	var buildResult struct {
		Records int      `json:"records"`
		Indexes []string `json:"indexes"`
	}
	if err := json.Unmarshal(buildOut.Bytes(), &buildResult); err != nil {
		t.Fatal(err)
	}
	if buildResult.Records != 2 || !containsString(buildResult.Indexes, "slack.users") || !containsString(buildResult.Indexes, "slack.channels") {
		t.Fatalf("build result = %#v", buildResult)
	}

	lookup := NewRootCommand()
	var lookupOut bytes.Buffer
	lookup.SetOut(&lookupOut)
	lookup.SetErr(&lookupOut)
	lookup.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "lookup", "timo", "--entity", "slack.user", "-o", "json"})
	if err := lookup.Execute(); err != nil {
		t.Fatal(err)
	}
	var lookupResult struct {
		Results struct {
			Available map[string]struct {
				Count   int `json:"count"`
				Matches []struct {
					ID     string `json:"id"`
					Source struct {
						Index string `json:"index"`
					} `json:"source"`
					Record struct {
						Entity string            `json:"entity"`
						ID     string            `json:"id"`
						Links  map[string]string `json:"links"`
						Origin struct {
							Plugin string `json:"plugin"`
						} `json:"origin"`
					} `json:"record"`
				} `json:"matches"`
			} `json:"available"`
		} `json:"results"`
	}
	if err := json.Unmarshal(lookupOut.Bytes(), &lookupResult); err != nil {
		t.Fatal(err)
	}
	slack := lookupResult.Results.Available["slack"]
	if slack.Count != 1 || slack.Matches[0].ID != "U123" || slack.Matches[0].Source.Index != "slack.users" {
		t.Fatalf("lookup result = %#v", slack)
	}
	if slack.Matches[0].Record.Entity != "slack.user" || slack.Matches[0].Record.ID != "U123" || slack.Matches[0].Record.Links["self"] != "slack://user/U123" || slack.Matches[0].Record.Origin.Plugin != "slack" {
		t.Fatalf("lookup standardized record = %#v", slack.Matches[0].Record)
	}
}

func TestSearchSkipsInstalledPluginWithoutSearchDatasource(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte("module fakenosearch\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(pluginDir, "cmd", "dex-plugin-nosrch")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainGo := `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct{ Command string ` + "`json:\"command\"`" + ` }
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	result := map[string]any{}
	if req.Command == "manifest" {
		result = map[string]any{
			"name": "nosrch",
			"datasources": []map[string]any{{
				"name": "nosrch.items",
				"entity": "nosrch.item",
				"capabilities": []string{"get"},
			}},
		}
	} else if req.Command == "datasources.search" {
		result = map[string]any{"unexpected": true}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":"dex.plugin.v1","ok":true,"result":result})
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	marketplace := filepath.Join(t.TempDir(), "marketplace.json")
	marketplaceData, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
		Name:      "nosrch",
		Binary:    "dex-plugin-nosrch",
		LocalPath: pluginDir,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplace, marketplaceData, 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveInstalledPlugin(core.PluginEntry{Name: "nosrch", Binary: "dex-plugin-nosrch"}, false); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", home, "--marketplace", marketplace, "search", "anything", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results struct {
			Available map[string]any `json:"available"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Results.Available["nosrch"]; ok {
		t.Fatalf("non-searchable plugin should not be searched: %s", out.String())
	}
}

func TestPluginListAlias(t *testing.T) {
	out, err := executeGeneratedRoot(t, "--dex-home", t.TempDir(), "plugin", "list", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Plugins []core.PluginEntry `json:"plugins"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Plugins) == 0 {
		t.Fatalf("plugin list output missing plugins:\n%s", out)
	}
}

func TestSearchHelpShowsOnlyCanonicalEndpointRefFlag(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "-h"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "--endpoint-ref string") {
		t.Fatalf("search help missing --endpoint-ref:\n%s", got)
	}
	if strings.Contains(got, "--endpoint string") {
		t.Fatalf("search help still shows deprecated endpoint flag:\n%s", got)
	}
}

func TestFanoutGroupsMissingPluginsAwayFromAvailableData(t *testing.T) {
	marketplace := filepath.Join(t.TempDir(), "marketplace.json")
	data, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
		Name:   "definitely-missing",
		Binary: "dex-plugin-definitely-missing",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplace, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--marketplace", marketplace, "op", "ls", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Available map[string]any `json:"available"`
		Missing   []string       `json:"missing"`
		Errors    map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Missing) == 0 {
		t.Fatalf("expected missing plugins to be grouped:\n%s", out.String())
	}
	for _, name := range result.Missing {
		if _, ok := result.Available[name]; ok {
			t.Fatalf("missing plugin %q also appeared in available:\n%s", name, out.String())
		}
	}
	for name, value := range result.Available {
		if m, ok := value.(map[string]any); ok && m["error"] != nil {
			t.Fatalf("available plugin %q contains mixed error data:\n%s", name, out.String())
		}
	}
}

func TestBackendErrorDoesNotPrintUsage(t *testing.T) {
	marketplace := filepath.Join(t.TempDir(), "marketplace.json")
	data, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
		Name:   "broken",
		Binary: "dex-plugin-broken",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplace, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "--marketplace", marketplace, "plugin", "show", "broken"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected backend error")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Fatalf("backend error printed usage:\n%s", out.String())
	}
}

func TestOverviewCommandsAcceptNoPluginArgument(t *testing.T) {
	pluginDir := writeFakeManifestPlugin(t, "gitlab", map[string]any{
		"name": "gitlab",
		"auth": []map[string]any{{
			"name": "token",
			"kind": "bearer",
			"fields": []map[string]any{{
				"name":     "access_token",
				"required": true,
				"secret":   true,
			}},
		}},
	})
	marketplace := filepath.Join(t.TempDir(), "marketplace.json")
	data, err := json.Marshal(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
		Name:      "gitlab",
		Binary:    "dex-plugin-gitlab",
		LocalPath: pluginDir,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplace, data, 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveInstalledPlugin(core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab", LocalPath: pluginDir}, false); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"plugin", "status"},
		{"auth", "info"},
		{"auth", "status"},
		{"index", "status"},
	} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"--dex-home", home, "--marketplace", marketplace, "--dev-plugin", "gitlab=" + pluginDir, "-o", "json"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out.String())
		}
		var decoded map[string]any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatalf("%v produced non-json output: %v\n%s", args, err, out.String())
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
