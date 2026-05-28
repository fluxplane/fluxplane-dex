package loki

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestQueryUsesLokiAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" || r.URL.Query().Get("query") != `{app="api"}` {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "streams",
				"result": []map[string]any{{
					"stream": map[string]string{"app": "api"},
					"values": [][]string{{"1710000000123456000", "hello"}},
				}},
			},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(Service{HTTPClient: server.Client()})

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"url": server.URL, "query": `{app="api"}`, "since": "1m"})
	if out.URL != server.URL || out.Count != 1 || out.Entries[0].Line != "hello" {
		t.Fatalf("query output = %#v", out)
	}
}

func TestRecentLogsBuildsSelector(t *testing.T) {
	query := recentLogsQuery(RecentLogsInput{App: "api", Namespace: "prod", Contains: "error"})
	if query != `{app="api",namespace="prod"} |= "error"` {
		t.Fatalf("query = %q", query)
	}
}
