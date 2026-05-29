package duckduckgo

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

func TestQueryEscape(t *testing.T) {
	if got := queryEscape("fluxplane dex/web"); got != "fluxplane+dex%2Fweb" {
		t.Fatalf("queryEscape = %q", got)
	}
}

func TestParseResults(t *testing.T) {
	body := `
<a class="result__a" href="/l/?kh=-1&uddg=https%3A%2F%2Fexample.com%2Fa">Example <b>A</b></a>
<a class="result__snippet">A snippet &amp; more</a>
<a class="result__a" href="https://example.com/b">Example B</a>
<div class="result__snippet">B snippet</div>`
	results := parseResults(body, 10)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].URL != "https://example.com/a" || results[0].Title != "Example A" || results[0].Snippet != "A snippet & more" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[1].URL != "https://example.com/b" || results[1].Source != PluginName {
		t.Fatalf("second result = %#v", results[1])
	}
}

func TestParseResultsDropsUnsafeURLs(t *testing.T) {
	body := `
<a class="result__a" href="/l/?kh=-1&uddg=javascript%3Aalert%281%29">Script</a>
<a class="result__a" href="/l/?kh=-1&uddg=https%3A%2F%2Fexample.com%0AX%3A%20y">Control</a>
<a class="result__a" href="https://example.com/safe">Safe</a>`
	results := parseResults(body, 10)
	if len(results) != 1 || results[0].URL != "https://example.com/safe" {
		t.Fatalf("results = %#v", results)
	}
}

func TestDatasourceSearchUsesSharedWebsearchWrapper(t *testing.T) {
	body := `
<a class="result__a" href="/l/?kh=-1&uddg=https%3A%2F%2Fexample.com%2Fa">Example <b>A</b></a>
<a class="result__snippet">A snippet &amp; more</a>`
	host := &fakeHostClient{httpBody: body}
	plugin := NewPluginWithService(Service{
		EndpointTemplate: "https://duckduckgo.test/html/?q={query}",
	})

	out := plugintest.DatasourceSearchOK[websearch.DatasourceSearchResult](t, plugin, websearch.SearchInput{Query: "fluxplane dex", Entity: websearch.EntitySearchResult}, plugintest.WithInstance("work"), plugintest.WithHost(host))
	if out.Source != "live" || out.Count != 1 || out.Records[0].ID != "https://example.com/a" {
		t.Fatalf("datasource output = %#v", out)
	}
	if out.Records[0].Source.Plugin != PluginName || out.Records[0].Source.Instance != "work" {
		t.Fatalf("record source = %#v", out.Records[0].Source)
	}
	if host.httpRequest.URL != "https://duckduckgo.test/html/?q=fluxplane+dex" || host.httpRequest.Method != "GET" {
		t.Fatalf("host HTTP request = %#v", host.httpRequest)
	}
}

func TestSearchRejectsTooManyQueries(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationSearch, websearch.SearchInput{Queries: []string{"1", "2", "3", "4", "5", "6"}})
	if err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestSearchFailsWhenProviderReturnsNoResults(t *testing.T) {
	host := &fakeHostClient{httpBody: `<html><body>No results</body></html>`}
	plugin := NewPluginWithService(Service{EndpointTemplate: "https://duckduckgo.test/html/?q={query}"})

	err := plugintest.RunError(t, plugin, OperationSearch, websearch.SearchInput{Query: "fluxplane dex"}, plugintest.WithHost(host))
	if err.Code != "web_search_failed" {
		t.Fatalf("error = %#v", err)
	}
}

type fakeHostClient struct {
	httpRequest pluginbinding.HTTPRequest
	httpBody    string
}

func (f *fakeHostClient) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (f *fakeHostClient) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (f *fakeHostClient) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (f *fakeHostClient) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (f *fakeHostClient) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (f *fakeHostClient) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	f.httpRequest = input
	return pluginbinding.HTTPResponse{StatusCode: 200, Status: "200 OK", Body: []byte(f.httpBody)}, nil
}

func (f *fakeHostClient) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (f *fakeHostClient) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (f *fakeHostClient) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (f *fakeHostClient) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (f *fakeHostClient) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*fakeHostClient)(nil)
