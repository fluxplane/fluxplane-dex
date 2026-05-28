package runtime

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
)

func TestLoadMarketplaceResolvesCanonicalNames(t *testing.T) {
	marketplace, err := LoadMarketplaceData([]byte(`{"version":"1","plugins":[{"name":"example","aliases":["ex"],"binary":"dex-plugin-example","go_install":"example.com/plugin@latest"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin, ok := marketplace.Resolve("example")
	if !ok {
		t.Fatal("plugin did not resolve")
	}
	if plugin.Name != "example" {
		t.Fatalf("plugin resolved to %q", plugin.Name)
	}
	if plugin.Binary == "" || plugin.GoInstall == "" {
		t.Fatalf("plugin install metadata incomplete: %#v", plugin)
	}
	if _, ok := marketplace.Resolve("ex"); ok {
		t.Fatal("marketplace aliases should not resolve")
	}
}

func TestLoadMarketplaceListsPlugins(t *testing.T) {
	marketplace, err := LoadMarketplaceData([]byte(`{"version":"1","plugins":[{"name":"one","binary":"dex-plugin-one"},{"name":"two","binary":"dex-plugin-two"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plugins := marketplace.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(plugins))
	}
}

func TestGoModuleCandidatesPreferOwningNestedModule(t *testing.T) {
	got := goModuleCandidates("github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd/dex-plugin-gitlab")
	want := []string{
		"github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd/dex-plugin-gitlab",
		"github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd",
		"github.com/fluxplane/fluxplane-dex/plugins/gitlab",
		"github.com/fluxplane/fluxplane-dex/plugins",
		"github.com/fluxplane/fluxplane-dex",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %#v, want %#v", got, want)
	}
}

func TestInstalledPluginRecordsManagedPath(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab", GoInstall: "example.com/gitlab@latest"}
	path := state.PluginBinaryPath(entry.Binary)
	if err := state.SaveInstalledPluginVersion(entry, true, path, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	status, err := state.PluginStatus(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || !status.Activated || !status.Managed || status.Path != path || status.Version != "v1.2.3" {
		t.Fatalf("status = %#v", status)
	}
	registry, err := state.LoadInstalledPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Plugins) != 1 || registry.Plugins[0].Path != path || registry.Plugins[0].Version != "v1.2.3" {
		t.Fatalf("registry = %#v", registry)
	}
}

func TestSaveInstalledPluginVersionActivatedPreservesDeactivatedState(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab", GoInstall: "example.com/gitlab@latest"}
	if err := state.SaveInstalledPluginVersionActivated(entry, true, state.PluginBinaryPath(entry.Binary), "v1.2.3", false); err != nil {
		t.Fatal(err)
	}
	status, err := state.PluginStatus(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.Activated {
		t.Fatalf("status = %#v", status)
	}
}

func TestUninstallManagedPluginRemovesBinary(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{Name: "gitlab", Binary: "dex-plugin-gitlab", GoInstall: "example.com/gitlab@latest"}
	path := state.PluginBinaryPath(entry.Binary)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveInstalledPluginVersion(entry, true, path, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	result, err := state.UninstallPlugin(entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed || !result.BinaryRemoved || result.Path != path {
		t.Fatalf("uninstall result = %#v", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("binary still exists or unexpected stat error: %v", err)
	}
	installed, err := state.IsPluginInstalled(entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("plugin state still installed")
	}
}

func TestRunnerUsesManagedPluginBinaryBeforeLocalPath(t *testing.T) {
	home := t.TempDir()
	state, err := NewState(home)
	if err != nil {
		t.Fatal(err)
	}
	entry := core.PluginEntry{
		Name:      "gitlab",
		Binary:    "dex-plugin-gitlab",
		LocalPath: "plugins/gitlab",
	}
	binaryPath := state.PluginBinaryPath(entry.Binary)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveInstalledPluginAt(entry, true, binaryPath); err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, "plugins", "gitlab"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state, WorkDir: workDir}
	cmd, err := runner.command(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != binaryPath {
		t.Fatalf("command path = %q, want %q", cmd.Path, binaryPath)
	}
}
