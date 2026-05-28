package websearch

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestDefineProviderWiresManifestOperationDatasourceAndSecrets(t *testing.T) {
	spec := ProviderSpec{
		Name:                  "example",
		Version:               "0.1.0",
		Description:           "Example web search provider.",
		Aliases:               []string{"ex"},
		Operation:             "example.search",
		Datasource:            "example.web_search",
		OperationDescription:  "Search with Example.",
		DatasourceDescription: "Example web search results.",
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			"api_key",
			"Example API key.",
			pluginbinding.AuthField("api_key", "Example API key", true, true, "EXAMPLE_API_KEY"),
		)},
		SecretPurposes: []string{"api_key"},
	}
	plugin := DefineProvider(spec, func(_ pluginbinding.Context, input SearchInput) (SearchOutput, error) {
		return SearchOutput{Results: []ResultSet{{
			Provider: spec.Name,
			Query:    input.Query,
			Results:  []Result{{URL: "https://example.com", Title: "Example"}},
		}}}, nil
	})
	manifest := plugin.Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if manifest.Metadata[MetadataProvider] != spec.Name || manifest.Metadata[MetadataOperation] != spec.Operation {
		t.Fatalf("metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != spec.Operation || manifest.Operations[0].SecretPurposes[0] != "api_key" {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	if len(manifest.Datasources) != 1 || manifest.Datasources[0].Name != spec.Datasource || manifest.Datasources[0].SecretPurposes[0] != "api_key" {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	out := plugintest.DatasourceSearchOK[DatasourceSearchResult](t, plugin, SearchInput{Query: "dex", Entity: EntitySearchResult})
	if out.Count != 1 || out.Records[0].ID != "https://example.com" {
		t.Fatalf("datasource output = %#v", out)
	}
}

func TestProviderDiscoveryFromManifestAndSelection(t *testing.T) {
	entry := core.PluginEntry{Name: "example", Aliases: []string{"ex"}}
	manifest := ProviderManifestSpec(ProviderSpec{
		Name:       "example-provider",
		Aliases:    []string{"provider-alias"},
		Operation:  "example.search",
		Datasource: "example.web_search",
	})
	coreManifest := pluginbinding.Manifest(manifest)
	provider, ok := ProviderFromManifest(entry, coreManifest)
	if !ok {
		t.Fatalf("provider not discovered")
	}
	if provider.Name != "example-provider" || provider.Plugin != "example" || provider.Operation != "example.search" || provider.Datasource != "example.web_search" {
		t.Fatalf("provider = %#v", provider)
	}
	selected, errors := SelectProviders([]Provider{provider}, []string{"provider-alias"})
	if len(errors) != 0 || len(selected) != 1 || selected[0].Name != provider.Name {
		t.Fatalf("selected = %#v errors=%#v", selected, errors)
	}
	_, errors = SelectProviders([]Provider{provider}, []string{"missing"})
	if len(errors) != 1 || errors[0].Provider != "missing" {
		t.Fatalf("errors = %#v", errors)
	}
}
