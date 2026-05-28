package prometheus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestQueryUsesPrometheusAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" || r.URL.Query().Get("query") != "up" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]any{"resultType": "vector", "result": []map[string]any{{"metric": map[string]string{"job": "api"}, "value": []any{1, "1"}}}},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(Service{HTTPClient: server.Client()})

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"url": server.URL, "query": "up"})
	if out.URL != server.URL || out.ResultType != "vector" || len(out.Results) == 0 {
		t.Fatalf("query output = %#v", out)
	}
}
