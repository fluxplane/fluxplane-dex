package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
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
			"labels": map[string]any{"context": input["context"], "namespace": input["namespace"], "limit": "3"},
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

func TestShortcutShowTracesDatasourceBinding(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "shortcut", "ls", "websearch", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Shortcuts []shortcutView `json:"shortcuts"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var found shortcutView
	for _, shortcut := range result.Shortcuts {
		if shortcut.Target == "datasource" {
			found = shortcut
			break
		}
	}
	if found.Plugin != "websearch" || found.Datasource != "websearch.results" || found.Capability != "search" {
		t.Fatalf("shortcut = %#v", result)
	}
}

func TestShortcutListIncludesKubernetesInventoryBindings(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dex-home", t.TempDir(), "shortcut", "ls", "kubernetes", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Shortcuts []shortcutView `json:"shortcuts"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	foundPodList := false
	foundSearch := false
	for _, shortcut := range result.Shortcuts {
		if shortcut.Use == "kube pod ls" && shortcut.Operation == "kubernetes.pod.list" {
			foundPodList = true
		}
		if shortcut.Use == "search --plugin kubernetes <query>" && shortcut.Datasource == "kubernetes.inventory" {
			foundSearch = true
		}
	}
	if !foundPodList || !foundSearch {
		t.Fatalf("shortcuts = %#v", result.Shortcuts)
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
	t.Setenv("GITLAB_URL", "https://gitlab.example.com")
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
	if !containsString(result.Saved, "access_token") || !containsString(result.Saved, "gitlab_url") {
		t.Fatalf("saved fields = %#v", result.Saved)
	}

	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	status := state.SecretStatus("gitlab", "work", []runtime.SecretPurpose{{Name: "access_token"}, {Name: "gitlab_url"}})
	if status["access_token"] != "stored" || status["gitlab_url"] != "stored" {
		t.Fatalf("work status = %#v", status)
	}
	defaultStatus := state.SecretStatus("gitlab", "default", []runtime.SecretPurpose{{Name: "access_token"}, {Name: "gitlab_url"}})
	if defaultStatus["access_token"] == "stored" || defaultStatus["gitlab_url"] == "stored" {
		t.Fatalf("default instance should not receive work secrets: %#v", defaultStatus)
	}
}

func TestAuthConnectAutoGitLabReportsMissingFields(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://gitlab.example.com")
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
	if !containsString(result.Saved, "gitlab_url") {
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
	cmd.SetArgs([]string{"--dex-home", home, "--dev-plugin", "gitlab=../../plugins/gitlab", "auth", "connect", "gitlab", "--field", "gitlab_url=https://gitlab.example.com", "-o", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected partial required auth to fail")
	}
	state, err := runtime.NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := state.IsPluginInstalled("gitlab")
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
	if result.GitLab != "" || result.Slack != "" || !result.Host {
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
		Results map[string]struct {
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
		} `json:"results"`
	}
	if err := json.Unmarshal(searchOut.Bytes(), &searchResult); err != nil {
		t.Fatal(err)
	}
	gitlab := searchResult.Results["gitlab"]
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
		Results map[string]struct {
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
		} `json:"results"`
	}
	if err := json.Unmarshal(lookupOut.Bytes(), &lookupResult); err != nil {
		t.Fatal(err)
	}
	lookupGitLab := lookupResult.Results["gitlab"]
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
		Results map[string]struct {
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
		} `json:"results"`
	}
	if err := json.Unmarshal(lookupOut.Bytes(), &lookupResult); err != nil {
		t.Fatal(err)
	}
	slack := lookupResult.Results["slack"]
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
		Results map[string]any `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Results["nosrch"]; ok {
		t.Fatalf("non-searchable plugin should not be searched: %s", out.String())
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
