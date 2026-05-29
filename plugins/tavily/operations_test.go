package tavily

import (
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

func TestSearchSendsTavilyRequestAndParsesResults(t *testing.T) {
	host := &fakeHostClient{httpBody: `{"query":"fluxplane dex","answer":"ok","results":[{"title":"Dex","url":"https://example.com","content":"Snippet","score":0.75}]}`}
	plugin := NewPluginWithService(Service{
		Endpoint: "https://tavily.test/search",
	})

	out := plugintest.RunOK[websearch.SearchOutput](t, plugin, OperationSearch, websearch.SearchInput{Query: "fluxplane dex", Max: 99}, plugintest.WithHost(host))
	if len(out.Results) != 1 || len(out.Results[0].Results) != 1 {
		t.Fatalf("output = %#v", out)
	}
	if out.Results[0].Provider != PluginName || out.Results[0].Results[0].URL != "https://example.com" {
		t.Fatalf("output = %#v", out)
	}
	if host.httpRequest.Auth == nil || host.httpRequest.Auth.BearerTokenPurpose != AuthPurposeAPIKey {
		t.Fatalf("auth = %#v", host.httpRequest.Auth)
	}
	if host.httpRequest.URL != "https://tavily.test/search" || host.httpRequest.Method != "POST" {
		t.Fatalf("host HTTP request = %#v", host.httpRequest)
	}
	if !strings.Contains(string(host.httpRequest.Body), `"max_results":20`) {
		t.Fatalf("request body = %s", string(host.httpRequest.Body))
	}
}

func TestDatasourceSearchUsesSharedWebsearchWrapper(t *testing.T) {
	host := &fakeHostClient{httpBody: `{"query":"fluxplane dex","results":[{"title":"Dex","url":"https://example.com","content":"Snippet","score":0.75}]}`}
	plugin := NewPluginWithService(Service{
		Endpoint: "https://tavily.test/search",
	})

	out := plugintest.DatasourceSearchOK[websearch.DatasourceSearchResult](t, plugin, websearch.SearchInput{Query: "fluxplane dex", Entity: websearch.EntitySearchResult}, plugintest.WithInstance("work"), plugintest.WithHost(host))
	if out.Source != "live" || out.Count != 1 || out.Records[0].ID != "https://example.com" {
		t.Fatalf("datasource output = %#v", out)
	}
	if out.Records[0].Source.Plugin != PluginName || out.Records[0].Source.Instance != "work" {
		t.Fatalf("record source = %#v", out.Records[0].Source)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationSearch, websearch.SearchInput{})
	if err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

type fakeHostClient struct {
	httpRequest pluginbinding.HTTPRequest
	httpBody    string
	httpStatus  int
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
	status := f.httpStatus
	if status == 0 {
		status = 200
	}
	return pluginbinding.HTTPResponse{StatusCode: status, Status: "200 OK", Body: []byte(f.httpBody)}, nil
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
