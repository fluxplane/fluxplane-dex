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
	GoInstall   string    `json:"go_install,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	Managed     bool      `json:"managed,omitempty"`
}

type InstalledRegistry struct {
	Plugins []InstalledPlugin `json:"plugins"`
}

func (s State) InstalledPluginsPath() string {
	return filepath.Join(s.Home, "plugins", "installed.json")
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
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	record := InstalledPlugin{
		Name:        entry.Name,
		Binary:      entry.Binary,
		GoInstall:   entry.GoInstall,
		InstalledAt: time.Now().UTC(),
		Managed:     managed,
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

func (s State) RemoveInstalledPlugin(name string) (bool, error) {
	name = strings.TrimSpace(name)
	registry, err := s.LoadInstalledPlugins()
	if err != nil {
		return false, err
	}
	next := registry.Plugins[:0]
	removed := false
	for _, plugin := range registry.Plugins {
		if plugin.Name == name {
			removed = true
			continue
		}
		next = append(next, plugin)
	}
	registry.Plugins = next
	if err := s.writeInstalledPlugins(registry); err != nil {
		return false, err
	}
	return removed, nil
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
	fields = append(fields, plugin.Aliases...)
	for _, command := range plugin.Commands {
		fields = append(fields, command.Use, command.Description, command.Operation)
	}
	return strings.Contains(strings.ToLower(strings.Join(fields, " ")), query)
}
