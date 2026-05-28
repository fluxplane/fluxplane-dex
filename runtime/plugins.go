package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
)

type InstalledPlugin struct {
	Name        string    `json:"name"`
	Binary      string    `json:"binary,omitempty"`
	Path        string    `json:"path,omitempty"`
	GoInstall   string    `json:"go_install,omitempty"`
	Version     string    `json:"version,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	Managed     bool      `json:"managed,omitempty"`
	Activated   bool      `json:"activated,omitempty"`
}

type InstalledRegistry struct {
	Plugins []InstalledPlugin `json:"plugins"`
}

type PluginStatus struct {
	Name      string `json:"name"`
	Known     bool   `json:"known"`
	Installed bool   `json:"installed"`
	Activated bool   `json:"activated"`
	Managed   bool   `json:"managed,omitempty"`
	Binary    string `json:"binary,omitempty"`
	Path      string `json:"path,omitempty"`
	GoInstall string `json:"go_install,omitempty"`
	Version   string `json:"version,omitempty"`
}

type PluginUninstallResult struct {
	Plugin        string `json:"plugin"`
	Removed       bool   `json:"removed"`
	BinaryRemoved bool   `json:"binary_removed,omitempty"`
	Path          string `json:"path,omitempty"`
}

func (s State) InstalledPluginsPath() string {
	return filepath.Join(s.Home, "plugins", "installed.json")
}

func (s State) PluginBinDir() string {
	return filepath.Join(s.Home, "plugins", "bin")
}

func (s State) PluginBinaryPath(binary string) string {
	return filepath.Join(s.PluginBinDir(), filepath.Base(strings.TrimSpace(binary)))
}

func (s State) LoadInstalledPlugins() (InstalledRegistry, error) {
	var registry InstalledRegistry
	data, err := os.ReadFile(s.InstalledPluginsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return registry, err
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return registry, err
	}
	sort.Slice(registry.Plugins, func(i, j int) bool { return registry.Plugins[i].Name < registry.Plugins[j].Name })
	return registry, nil
}

func (s State) SaveInstalledPlugin(entry core.PluginEntry, managed bool) error {
	return s.saveInstalledPlugin(entry, managed, "", "")
}

func (s State) SaveInstalledPluginAt(entry core.PluginEntry, managed bool, path string) error {
	return s.saveInstalledPlugin(entry, managed, path, "")
}

func (s State) SaveInstalledPluginVersion(entry core.PluginEntry, managed bool, path, version string) error {
	return s.saveInstalledPlugin(entry, managed, path, version)
}

func (s State) SaveInstalledPluginVersionActivated(entry core.PluginEntry, managed bool, path, version string, activated bool) error {
	return s.saveInstalledPluginActivated(entry, managed, path, version, activated)
}

func (s State) saveInstalledPlugin(entry core.PluginEntry, managed bool, path, version string) error {
	return s.saveInstalledPluginActivated(entry, managed, path, version, true)
}

func (s State) saveInstalledPluginActivated(entry core.PluginEntry, managed bool, path, version string, activated bool) error {
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	record := InstalledPlugin{
		Name:        entry.Name,
		Binary:      entry.Binary,
		Path:        strings.TrimSpace(path),
		GoInstall:   entry.GoInstall,
		Version:     strings.TrimSpace(version),
		InstalledAt: time.Now().UTC(),
		Managed:     managed,
		Activated:   activated,
	}
	replaced := false
	for i := range registry.Plugins {
		if registry.Plugins[i].Name == entry.Name {
			registry.Plugins[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		registry.Plugins = append(registry.Plugins, record)
	}
	return s.writeInstalledPlugins(registry)
}

func (s State) MarkPluginInstalled(entry core.PluginEntry, managed bool) error {
	return s.SaveInstalledPlugin(entry, managed)
}

func (s State) PluginStatus(entry core.PluginEntry) (PluginStatus, error) {
	status := PluginStatus{Name: entry.Name, Known: strings.TrimSpace(entry.Name) != "", Binary: entry.Binary, GoInstall: entry.GoInstall}
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return status, err
	}
	for _, plugin := range registry.Plugins {
		if plugin.Name != entry.Name {
			continue
		}
		status.Installed = true
		status.Activated = plugin.Activated
		status.Managed = plugin.Managed
		if strings.TrimSpace(plugin.Binary) != "" {
			status.Binary = plugin.Binary
		}
		if strings.TrimSpace(plugin.GoInstall) != "" {
			status.GoInstall = plugin.GoInstall
		}
		if strings.TrimSpace(plugin.Path) != "" {
			status.Path = plugin.Path
		}
		if strings.TrimSpace(plugin.Version) != "" {
			status.Version = plugin.Version
		}
		return status, nil
	}
	return status, nil
}

func (s State) InstalledPlugin(name string) (InstalledPlugin, bool, error) {
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return InstalledPlugin{}, false, err
	}
	name = strings.TrimSpace(name)
	for _, plugin := range registry.Plugins {
		if plugin.Name == name {
			return plugin, true, nil
		}
	}
	return InstalledPlugin{}, false, nil
}

func (s State) PluginStatuses(m Marketplace) (map[string]PluginStatus, error) {
	out := map[string]PluginStatus{}
	for _, entry := range m.Plugins() {
		status, err := s.PluginStatus(entry)
		if err != nil {
			return nil, err
		}
		out[entry.Name] = status
	}
	return out, nil
}

func (s State) IsPluginInstalled(name string) (bool, error) {
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return false, err
	}
	name = strings.TrimSpace(name)
	for _, plugin := range registry.Plugins {
		if plugin.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (s State) IsPluginActivated(name string) (bool, error) {
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return false, err
	}
	name = strings.TrimSpace(name)
	for _, plugin := range registry.Plugins {
		if plugin.Name == name {
			return plugin.Activated, nil
		}
	}
	return false, nil
}

func (s State) ActivatePlugin(entry core.PluginEntry) error {
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range registry.Plugins {
		if registry.Plugins[i].Name == entry.Name {
			registry.Plugins[i].Activated = true
			if registry.Plugins[i].InstalledAt.IsZero() {
				registry.Plugins[i].InstalledAt = now
			}
			if registry.Plugins[i].Binary == "" {
				registry.Plugins[i].Binary = entry.Binary
			}
			if registry.Plugins[i].GoInstall == "" {
				registry.Plugins[i].GoInstall = entry.GoInstall
			}
			return s.writeInstalledPlugins(registry)
		}
	}
	registry.Plugins = append(registry.Plugins, InstalledPlugin{
		Name:        entry.Name,
		Binary:      entry.Binary,
		GoInstall:   entry.GoInstall,
		InstalledAt: now,
		Activated:   true,
	})
	return s.writeInstalledPlugins(registry)
}

func (s State) DeactivatePlugin(name string) (bool, error) {
	name = strings.TrimSpace(name)
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return false, err
	}
	for i := range registry.Plugins {
		if registry.Plugins[i].Name == name {
			changed := registry.Plugins[i].Activated
			registry.Plugins[i].Activated = false
			if err := s.writeInstalledPlugins(registry); err != nil {
				return false, err
			}
			return changed, nil
		}
	}
	return false, nil
}

func (s State) UninstallPlugin(name string) (PluginUninstallResult, error) {
	name = strings.TrimSpace(name)
	result := PluginUninstallResult{Plugin: name}
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return result, err
	}
	next := registry.Plugins[:0]
	for _, plugin := range registry.Plugins {
		if plugin.Name == name {
			result.Removed = true
			if plugin.Managed && insideDir(s.PluginBinDir(), plugin.Path) {
				result.Path = plugin.Path
				if err := os.Remove(plugin.Path); err == nil {
					result.BinaryRemoved = true
				} else if !os.IsNotExist(err) {
					return result, err
				}
			}
			continue
		}
		next = append(next, plugin)
	}
	registry.Plugins = next
	if err := s.writeInstalledPlugins(registry); err != nil {
		return result, err
	}
	return result, nil
}

func (s State) writeInstalledPlugins(registry InstalledRegistry) error {
	if err := os.MkdirAll(filepath.Dir(s.InstalledPluginsPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal installed plugins: %w", err)
	}
	return os.WriteFile(s.InstalledPluginsPath(), data, 0o600)
}

func SearchPlugins(m Marketplace, query string) []core.PluginEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []core.PluginEntry
	for _, plugin := range m.Plugins() {
		if query == "" || pluginMatches(plugin, query) {
			out = append(out, plugin)
		}
	}
	return out
}

func pluginMatches(plugin core.PluginEntry, query string) bool {
	fields := []string{plugin.Name, plugin.Description, plugin.Binary, plugin.GoInstall}
	return strings.Contains(strings.ToLower(strings.Join(fields, " ")), query)
}
