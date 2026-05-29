package prometheus

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
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
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "prometheus-dev", "query": "up"}, plugintest.WithHost(host))
	if out.URL != "prometheus-dev" || out.ResultType != "vector" || len(out.Results) == 0 {
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
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	query := plugintest.DatasourceSearchOK[QueryDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceQueryResults, "endpoint_ref": "prometheus-dev", "query": "up"}, plugintest.WithHost(host))
	if query.Count != 1 || query.Records[0].Query != "up" || query.Records[0].EndpointURL != "prometheus-dev" {
		t.Fatalf("query datasource = %#v", query)
	}
	labels := plugintest.DatasourceSearchOK[LabelDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceLabels, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if labels.Count != 2 || labels.Records[0].Name != "job" {
		t.Fatalf("labels datasource = %#v", labels)
	}
	targets := plugintest.DatasourceSearchOK[TargetDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceTargets, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if targets.Count != 1 || targets.Records[0].Job != "api" {
		t.Fatalf("targets datasource = %#v", targets)
	}
	alerts := plugintest.DatasourceSearchOK[AlertDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceAlerts, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if alerts.Count != 1 || alerts.Records[0].Name != "HighErrorRate" || alerts.Records[0].Severity != "page" {
		t.Fatalf("alerts datasource = %#v", alerts)
	}
}

type prometheusTestHost struct {
	baseURL string
}

func newPrometheusTestHost(baseURL string) *prometheusTestHost {
	return &prometheusTestHost{baseURL: strings.TrimRight(baseURL, "/")}
}

func (h *prometheusTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *prometheusTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *prometheusTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *prometheusTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *prometheusTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *prometheusTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, h.baseURL+"/"+strings.TrimLeft(input.Path, "/"), bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	query := req.URL.Query()
	for key, values := range input.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	return pluginbinding.HTTPResponse{URL: req.URL.String(), FinalURL: resp.Request.URL.String(), Status: resp.Status, StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}, nil
}

func (h *prometheusTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *prometheusTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *prometheusTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *prometheusTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *prometheusTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*prometheusTestHost)(nil)
