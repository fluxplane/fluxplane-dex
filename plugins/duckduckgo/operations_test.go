package duckduckgo

import (
	"io"
	"net/http"
	"strings"
	"testing"

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

func TestDatasourceSearchUsesSharedWebsearchWrapper(t *testing.T) {
	body := `
<a class="result__a" href="/l/?kh=-1&uddg=https%3A%2F%2Fexample.com%2Fa">Example <b>A</b></a>
<a class="result__snippet">A snippet &amp; more</a>`
	plugin := NewPluginWithService(Service{
		HTTPClient:       &fakeHTTPDoer{body: body},
		EndpointTemplate: "https://duckduckgo.test/html/?q={query}",
	})

	out := plugintest.DatasourceSearchOK[websearch.DatasourceSearchResult](t, plugin, websearch.SearchInput{Query: "fluxplane dex", Entity: websearch.EntitySearchResult}, plugintest.WithInstance("work"))
	if out.Source != "live" || out.Count != 1 || out.Records[0].ID != "https://example.com/a" {
		t.Fatalf("datasource output = %#v", out)
	}
	if out.Records[0].Source.Plugin != PluginName || out.Records[0].Source.Instance != "work" {
		t.Fatalf("record source = %#v", out.Records[0].Source)
	}
}

type fakeHTTPDoer struct {
	req  *http.Request
	body string
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.req = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}
