package websearch

import (
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
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
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version || manifest.Metadata[MetadataProvider] != spec.Name || manifest.Metadata[MetadataOperation] != spec.Operation {
		t.Fatalf("metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != spec.Operation || manifest.Operations[0].SecretPurposes[0] != "api_key" {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	if len(manifest.Datasources) != 1 || manifest.Datasources[0].Name != spec.Datasource || manifest.Datasources[0].SecretPurposes[0] != "api_key" {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	datasource := manifest.Datasources[0]
	if datasource.EntitySchema == nil || datasource.EntitySchema.IDField != "url" || datasource.Fallback != core.DatasourceFallbackProviderFirst {
		t.Fatalf("datasource metadata = %#v", datasource)
	}
	out := plugintest.DatasourceSearchOK[DatasourceSearchResult](t, plugin, SearchInput{Query: "dex", Entity: EntitySearchResult})
	if out.Count != 1 || out.Records[0].ID != "https://example.com" {
		t.Fatalf("datasource output = %#v", out)
	}
}

func TestProviderDiscoveryFromManifestAndSelection(t *testing.T) {
	entry := core.PluginEntry{Name: "example"}
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

func TestValidateQueriesBoundsInput(t *testing.T) {
	queries, err := ValidateQueries(SearchInput{Query: " dex ", Queries: []string{"dex", "fluxplane"}})
	if err != nil || len(queries) != 2 || queries[0] != "dex" || queries[1] != "fluxplane" {
		t.Fatalf("queries = %#v err=%v", queries, err)
	}
	if _, err := ValidateQueries(SearchInput{Queries: []string{"1", "2", "3", "4", "5", "6"}}); err == nil {
		t.Fatalf("expected too many queries error")
	}
	if _, err := ValidateQueries(SearchInput{Query: strings.Repeat("x", MaxQueryLength+1)}); err == nil {
		t.Fatalf("expected query length error")
	}
}

func TestHasResultsAcceptsAnswerOrResults(t *testing.T) {
	if HasResults(ResultSet{}) {
		t.Fatalf("empty result set should not have results")
	}
	if !HasResults(ResultSet{Answer: "answer"}) || !HasResults(ResultSet{Results: []Result{{URL: "https://example.com"}}}) {
		t.Fatalf("expected answer or results to count")
	}
}
