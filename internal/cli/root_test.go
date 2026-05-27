package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestGitLabMRListCommandRoutesOperationInput(t *testing.T) {
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
		"gl", "mr", "ls",
		"--project", "group/dex",
		"--state", "merged",
		"--search", "ship",
		"--limit", "7",
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
	build.SetArgs([]string{"--dex-home", home, "--dev-plugin", "slack=" + pluginDir, "slack", "index", "-o", "json"})
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
