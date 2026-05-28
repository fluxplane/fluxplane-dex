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

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestDatasourceHandlersUsePrometheusAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"resultType": "vector", "result": []map[string]any{{"metric": map[string]string{"job": "api"}, "value": []any{1, "1"}}}},
			})
		case "/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"job", "instance"}})
		case "/api/v1/targets":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"activeTargets": []map[string]any{{"health": "up", "labels": map[string]string{"job": "api", "instance": "api:9090"}}}}})
		case "/api/v1/alerts":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"alerts": []map[string]any{{"state": "firing", "labels": map[string]string{"alertname": "HighErrorRate", "severity": "page"}}}}})
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(Service{HTTPClient: server.Client()})

	query := plugintest.DatasourceSearchOK[QueryDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceQueryResults, "url": server.URL, "query": "up"})
	if query.Count != 1 || query.Records[0].Query != "up" || query.Records[0].EndpointURL != server.URL {
		t.Fatalf("query datasource = %#v", query)
	}
	labels := plugintest.DatasourceSearchOK[LabelDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceLabels, "url": server.URL})
	if labels.Count != 2 || labels.Records[0].Name != "job" {
		t.Fatalf("labels datasource = %#v", labels)
	}
	targets := plugintest.DatasourceSearchOK[TargetDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceTargets, "url": server.URL})
	if targets.Count != 1 || targets.Records[0].Job != "api" {
		t.Fatalf("targets datasource = %#v", targets)
	}
	alerts := plugintest.DatasourceSearchOK[AlertDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceAlerts, "url": server.URL})
	if alerts.Count != 1 || alerts.Records[0].Name != "HighErrorRate" || alerts.Records[0].Severity != "page" {
		t.Fatalf("alerts datasource = %#v", alerts)
	}
}
