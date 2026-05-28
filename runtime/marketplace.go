package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
)

type Marketplace struct {
	data   core.Marketplace
	byName map[string]core.PluginEntry
}

func LoadMarketplace(path string) (Marketplace, error) {
	if strings.TrimSpace(path) == "" {
		return NewMarketplace(core.Marketplace{Version: "1"}), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Marketplace{}, fmt.Errorf("read marketplace: %w", err)
	}
	return LoadMarketplaceData(data)
}

func LoadMarketplaceData(data []byte) (Marketplace, error) {
	var raw core.Marketplace
	if err := json.Unmarshal(data, &raw); err != nil {
		return Marketplace{}, fmt.Errorf("parse marketplace: %w", err)
	}
	return NewMarketplace(raw), nil
}

func NewMarketplace(raw core.Marketplace) Marketplace {
	m := Marketplace{data: raw, byName: map[string]core.PluginEntry{}}
	for _, plugin := range raw.Plugins {
		name := strings.TrimSpace(plugin.Name)
		if name == "" {
			continue
		}
		m.byName[name] = plugin
	}
	return m
}

func (m Marketplace) Data() core.Marketplace {
	return m.data
}

func (m Marketplace) Resolve(nameOrAlias string) (core.PluginEntry, bool) {
	name := strings.TrimSpace(nameOrAlias)
	plugin, ok := m.byName[name]
	return plugin, ok
}

func (m Marketplace) Plugins() []core.PluginEntry {
	out := make([]core.PluginEntry, 0, len(m.data.Plugins))
	out = append(out, m.data.Plugins...)
	return out
}
