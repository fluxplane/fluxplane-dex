package dex

import (
	"context"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

// PluginService manages plugin lifecycle: install, uninstall, activate,
// list, search.
type PluginService struct {
	engine *Engine
}

// MarketplacePlugin is a marketplace catalog entry. Type alias for
// core.PluginEntry so embedders don't need to import core directly.
type MarketplacePlugin = core.PluginEntry

// InstalledPlugin describes a plugin that has been installed locally.
type InstalledPlugin = runtime.InstalledPlugin

// PluginStatus combines marketplace and installed-registry info for one plugin.
type PluginStatus = runtime.PluginStatus

// UninstallResult describes the outcome of an uninstall.
type UninstallResult = runtime.PluginUninstallResult

// List returns all locally installed plugins (managed and unmanaged).
func (s *PluginService) List(ctx context.Context) ([]InstalledPlugin, error) {
	registry, err := s.engine.runner.State.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}
	return registry.Plugins, nil
}

// Search filters the marketplace catalog by query (matched against name,
// description, binary, and go_install). Empty query returns the full catalog.
func (s *PluginService) Search(_ context.Context, query string) []MarketplacePlugin {
	return runtime.SearchPlugins(s.engine.runner.Marketplace, query)
}

// Status returns the marketplace + installation status for a single plugin.
func (s *PluginService) Status(_ context.Context, name string) (PluginStatus, error) {
	entry, ok := s.engine.runner.Marketplace.Resolve(name)
	if !ok {
		return PluginStatus{Name: name}, fmt.Errorf("%w: %q", ErrPluginNotFound, name)
	}
	return s.engine.runner.State.PluginStatus(entry)
}

// Statuses returns the marketplace + installation status for every catalog plugin.
func (s *PluginService) Statuses(_ context.Context) (map[string]PluginStatus, error) {
	return s.engine.runner.State.PluginStatuses(s.engine.runner.Marketplace)
}

// Install builds and installs the plugin binary, marks it activated.
func (s *PluginService) Install(ctx context.Context, name string) (InstalledPlugin, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(name); !ok {
		return InstalledPlugin{}, fmt.Errorf("%w: %q", ErrPluginNotFound, name)
	}
	return s.engine.runner.InstallPlugin(ctx, name)
}

// Uninstall removes the plugin from the installed registry and (if managed)
// removes its binary.
func (s *PluginService) Uninstall(_ context.Context, name string) (UninstallResult, error) {
	return s.engine.runner.State.UninstallPlugin(name)
}

// Activate marks a plugin as activated. Activation is required before the
// plugin shows up in CLI integration menus.
func (s *PluginService) Activate(_ context.Context, name string) error {
	entry, ok := s.engine.runner.Marketplace.Resolve(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrPluginNotFound, name)
	}
	return s.engine.runner.State.ActivatePlugin(entry)
}

// Deactivate clears the activation flag for an installed plugin. Returns
// (changed, error) — changed is true when the plugin transitioned from
// activated to deactivated.
func (s *PluginService) Deactivate(_ context.Context, name string) (bool, error) {
	return s.engine.runner.State.DeactivatePlugin(name)
}

// Upgrade reinstalls a plugin to its latest go_install version (or
// reuses the current activation state).
func (s *PluginService) Upgrade(ctx context.Context, name string) (InstalledPlugin, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(name); !ok {
		return InstalledPlugin{}, fmt.Errorf("%w: %q", ErrPluginNotFound, name)
	}
	return s.engine.runner.UpgradePlugin(ctx, name)
}

// Resolve looks up a marketplace entry by name or alias.
func (s *PluginService) Resolve(_ context.Context, name string) (MarketplacePlugin, bool) {
	return s.engine.runner.Marketplace.Resolve(name)
}

// All returns the full marketplace catalog.
func (s *PluginService) All(_ context.Context) []MarketplacePlugin {
	return s.engine.runner.Marketplace.Plugins()
}
