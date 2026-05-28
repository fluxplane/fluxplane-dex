package duckduckgo

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

const (
	PluginName        = "duckduckgo"
	PluginVersion     = "0.4.0"
	PluginDescription = "DuckDuckGo web search provider."

	OperationSearch = "duckduckgo.search"

	DatasourceWebSearch = "duckduckgo.web_search"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return websearch.ProviderManifestSpec(providerSpec())
}

func providerSpec() websearch.ProviderSpec {
	return websearch.ProviderSpec{
		Name:                  PluginName,
		Version:               PluginVersion,
		Description:           PluginDescription,
		Aliases:               []string{"ddg", PluginName},
		Operation:             OperationSearch,
		Datasource:            DatasourceWebSearch,
		OperationDescription:  "Search the web with DuckDuckGo.",
		DatasourceDescription: "DuckDuckGo web search results.",
	}
}
