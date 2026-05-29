package grafana

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

func TestDatasourceListDerivesClusterAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/datasources" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"uid":"loki","name":"Loki Infra","type":"loki"},
			{"uid":"prometheus-alpha-east","name":"Prometheus Alpha East","type":"prometheus"},
			{"uid":"alertmanager-beta-west","name":"Alertmanager Beta West","type":"alertmanager"},
			{"uid":"tempo","name":"Tempo","type":"tempo"}
		]`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DatasourceListResult](t, plugin, OperationDatasourceList, map[string]any{"endpoint_ref": "grafana-dev"}, plugintest.WithHost(host))
	if out.Count != 4 {
		t.Fatalf("count = %d", out.Count)
	}
	if out.Clusters["infra"]["loki"] != "loki" || out.Clusters["alpha-east"]["prometheus"] != "prometheus-alpha-east" {
		t.Fatalf("clusters = %#v", out.Clusters)
	}
	if out.Clusters["beta-west"]["alertmanager"] != "alertmanager-beta-west" {
		t.Fatalf("clusters = %#v", out.Clusters)
	}
}

func TestLokiLabelsUsesClusterUIDAndBearerAuth(t *testing.T) {
	var auth string
	var proxyPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"loki-alpha-east","name":"Loki Alpha East","type":"loki"}]`))
		case "/api/datasources/proxy/uid/loki-alpha-east/loki/api/v1/labels":
			proxyPath = r.URL.Path
			_, _ = w.Write([]byte(`{"status":"success","data":["pod","namespace","app"]}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "glsa_test")

	out := plugintest.RunOK[LabelsResult](t, plugin, OperationLokiLabels, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha"}, plugintest.WithHost(host))
	if out.UID != "loki-alpha-east" || len(out.Values) != 3 {
		t.Fatalf("result = %#v", out)
	}
	if proxyPath == "" {
		t.Fatalf("proxy path was not called")
	}
	if auth != "Bearer glsa_test" {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestResolveUIDRejectsAmbiguousShortAlias(t *testing.T) {
	_, err := resolveUID([]Datasource{
		{UID: "loki-alpha-east", Type: "loki"},
		{UID: "loki-alpha-west", Type: "loki"},
	}, "loki", "alpha")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v", err)
	}
}

func TestDashboardGetExtractsPanelQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dashboards/uid/dash-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"dashboard":{"uid":"dash-1","title":"Runtime","panels":[{"id":7,"title":"Requests","type":"timeseries","datasource":{"type":"prometheus","uid":"prometheus-alpha-east"},"targets":[{"refId":"A","expr":"sum(rate(http_requests_total[5m]))"}]},{"id":8,"title":"Logs","type":"logs","datasource":{"type":"loki","uid":"loki-alpha-east"},"targets":[{"refId":"B","query":"{app=\"api\"}"}]}]}}`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DashboardGetResult](t, plugin, OperationDashboardGet, map[string]any{"endpoint_ref": "grafana-dev", "uid": "dash-1"}, plugintest.WithHost(host))
	if out.Title != "Runtime" || len(out.Queries) != 2 {
		t.Fatalf("result = %#v", out)
	}
	if out.Queries[0].Expression == "" || out.Queries[1].Query == "" {
		t.Fatalf("queries = %#v", out.Queries)
	}
}

func TestLokiQueryNormalizesLogEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"loki-alpha-east","name":"Loki Alpha East","type":"loki"}]`))
		case "/api/datasources/proxy/uid/loki-alpha-east/loki/api/v1/query_range":
			_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"stream":{"namespace":"latest","app":"backend-acd","pod":"backend-acd-1"},"values":[["1710000000123456000","hello"],["1710000001123456000","world"]]}]}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[LokiQueryResult](t, plugin, OperationLokiQuery, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha", "query": `{namespace="latest"}`, "limit": 2}, plugintest.WithHost(host))
	if out.Count != 2 || out.Limit != 2 || out.NormalizedQuery != `{namespace="latest"}` || len(out.Raw) == 0 {
		t.Fatalf("result = %#v", out)
	}
	if out.Entries[0].Line != "world" || out.Entries[0].Labels["pod"] != "backend-acd-1" {
		t.Fatalf("entries = %#v", out.Entries)
	}
}

func TestDatasourceHealthFallsBackForAlertmanager(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources/uid/alertmanager-alpha/health":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Plugin unavailable"}`))
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"Alertmanager Alpha","type":"alertmanager"}]`))
		case "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/status":
			_, _ = w.Write([]byte(`{"cluster":{"status":"ready"}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[ProxyQueryResult](t, plugin, OperationDatasourceHealth, map[string]any{"endpoint_ref": "grafana-dev", "uid": "alertmanager-alpha"}, plugintest.WithHost(host))
	if out.UID != "alertmanager-alpha" || !strings.Contains(string(out.Data), "alertmanager_status") {
		t.Fatalf("result = %#v", out)
	}
}

func TestDatasourceHealthReturnsAlertmanagerProxyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources/uid/alertmanager-alpha/health":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Plugin unavailable"}`))
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"Alertmanager Alpha","type":"alertmanager"}]`))
		case "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/status":
			http.NotFound(w, r)
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[ProxyQueryResult](t, plugin, OperationDatasourceHealth, map[string]any{"endpoint_ref": "grafana-dev", "uid": "alertmanager-alpha"}, plugintest.WithHost(host))
	if out.UID != "alertmanager-alpha" || !strings.Contains(string(out.Data), `"status":"error"`) || !strings.Contains(string(out.Data), "alertmanager_status") {
		t.Fatalf("result = %#v", out)
	}
}

func TestManifestIncludesExpectedOperations(t *testing.T) {
	manifest := Manifest()
	operations := map[string]bool{}
	for _, operation := range manifest.Operations {
		operations[operation.Name] = true
	}
	for _, name := range []string{OperationDatasourceList, OperationDatasourceHealth, OperationDashboardList, OperationDashboardGet, OperationAnnotationList, OperationAnnotationAdd, OperationLokiLabels, OperationLokiQuery, OperationPrometheusQuery, OperationPrometheusRules, OperationAlertsActive, OperationAlertSilencesList, OperationAlertSilenceCreate, OperationAlertSilenceDelete, OperationTempoSearch, OperationTempoTraceGet} {
		if !operations[name] {
			t.Fatalf("manifest missing operation %s", name)
		}
	}
	for _, operation := range manifest.Operations {
		if !containsString(operation.SecretPurposes, AuthPurposeAPIToken) || !containsString(operation.SecretPurposes, AuthPurposeUsername) || !containsString(operation.SecretPurposes, AuthPurposePassword) {
			t.Fatalf("%s secret purposes = %#v", operation.Name, operation.SecretPurposes)
		}
	}
	var raw string
	for _, operation := range manifest.Operations {
		if operation.Name == OperationLokiLabels {
			raw = operationInputSchema(t, operation.Input)
			break
		}
	}
	if !strings.Contains(raw, "endpoint_ref") || !strings.Contains(raw, "cluster") {
		t.Fatalf("input schema = %s", raw)
	}
	if strings.Contains(raw, "token") || strings.Contains(raw, "password") || strings.Contains(raw, "\"url\"") || strings.Contains(raw, "credential_ref") {
		t.Fatalf("input schema exposes endpoint secret fields = %s", raw)
	}
}

type grafanaTestHost struct {
	baseURL string
	token   string
}

func newGrafanaTestHost(baseURL, token string) *grafanaTestHost {
	return &grafanaTestHost{baseURL: strings.TrimRight(baseURL, "/"), token: token}
}

func (h *grafanaTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	if purpose == AuthPurposeAPIToken && h.token != "" {
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.token}, nil
	}
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func (h *grafanaTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *grafanaTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *grafanaTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *grafanaTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *grafanaTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	endpoint := h.baseURL + "/" + strings.TrimLeft(input.Path, "/")
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	if input.Auth != nil && input.Auth.BearerTokenPurpose == AuthPurposeAPIToken && h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	if len(input.Query) > 0 {
		query := req.URL.Query()
		for key, vals := range input.Query {
			for _, value := range vals {
				query.Add(key, value)
			}
		}
		req.URL.RawQuery = query.Encode()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	return pluginbinding.HTTPResponse{URL: endpoint, FinalURL: resp.Request.URL.String(), Status: resp.Status, StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}, nil
}

func (h *grafanaTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *grafanaTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *grafanaTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *grafanaTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *grafanaTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*grafanaTestHost)(nil)

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func operationInputSchema(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		t.Fatal(err)
	}
	return compacted.String()
}
