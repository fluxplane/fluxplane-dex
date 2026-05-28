package tavily

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

func TestSearchSendsTavilyRequestAndParsesResults(t *testing.T) {
	fake := &fakeHTTPDoer{body: `{"query":"fluxplane dex","answer":"ok","results":[{"title":"Dex","url":"https://example.com","content":"Snippet","score":0.75}]}`}
	plugin := NewPluginWithService(Service{
		SecretGetter: func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
			return pluginbinding.SecretMaterial{Purpose: purpose, Value: "test-key"}, nil
		},
		HTTPClient: fake,
		Endpoint:   "https://tavily.test/search",
	})

	out := plugintest.RunOK[websearch.SearchOutput](t, plugin, OperationSearch, websearch.SearchInput{Query: "fluxplane dex", Max: 99})
	if len(out.Results) != 1 || len(out.Results[0].Results) != 1 {
		t.Fatalf("output = %#v", out)
	}
	if out.Results[0].Provider != PluginName || out.Results[0].Results[0].URL != "https://example.com" {
		t.Fatalf("output = %#v", out)
	}
	if fake.req.Header.Get("Authorization") != "Bearer test-key" {
		t.Fatalf("authorization = %q", fake.req.Header.Get("Authorization"))
	}
	if !strings.Contains(fake.requestBody, `"max_results":20`) {
		t.Fatalf("request body = %s", fake.requestBody)
	}
}

func TestDatasourceSearchUsesSharedWebsearchWrapper(t *testing.T) {
	fake := &fakeHTTPDoer{body: `{"query":"fluxplane dex","results":[{"title":"Dex","url":"https://example.com","content":"Snippet","score":0.75}]}`}
	plugin := NewPluginWithService(Service{
		SecretGetter: func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
			return pluginbinding.SecretMaterial{Purpose: purpose, Value: "test-key"}, nil
		},
		HTTPClient: fake,
		Endpoint:   "https://tavily.test/search",
	})

	out := plugintest.DatasourceSearchOK[websearch.DatasourceSearchResult](t, plugin, websearch.SearchInput{Query: "fluxplane dex", Entity: websearch.EntitySearchResult}, plugintest.WithInstance("work"))
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

type fakeHTTPDoer struct {
	req         *http.Request
	requestBody string
	body        string
	status      int
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.req = req
	data, _ := io.ReadAll(req.Body)
	f.requestBody = string(data)
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}
