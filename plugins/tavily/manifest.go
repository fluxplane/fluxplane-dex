package tavily

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

const (
	PluginName        = "tavily"
	PluginVersion     = "0.13.3"
	PluginDescription = "Tavily web search provider."

	AuthMethodAPIKey  = "api_key"
	AuthPurposeAPIKey = "api_key"
	EnvTavilyAPIKey   = "TAVILY_API_KEY"

	OperationSearch = "tavily.search"

	DatasourceWebSearch = "tavily.web_search"
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
		Aliases:               []string{PluginName},
		Operation:             OperationSearch,
		Datasource:            DatasourceWebSearch,
		OperationDescription:  "Search the web with Tavily.",
		DatasourceDescription: "Tavily web search results.",
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodAPIKey,
			"Tavily API key resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeAPIKey, "Tavily API key", true, true, EnvTavilyAPIKey),
		)},
		SecretPurposes: []string{AuthPurposeAPIKey},
	}
}
