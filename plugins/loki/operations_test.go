package loki

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
	"github.com/fluxplane/fluxplane-dex/protocol"
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
	plugin := NewPluginWithService(NewService())
	host := newLokiTestHost(server.URL, "")

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "loki-dev", "query": `{app="api"}`, "since": "1m"}, plugintest.WithHost(host))
	if out.URL != "loki-dev" || out.Count != 1 || out.Entries[0].Line != "hello" {
		t.Fatalf("query output = %#v", out)
	}
}

func TestQueryCapsLimitValidatesDirectionAndSortsForward(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "1000" {
			t.Fatalf("limit = %q, want 1000", got)
		}
		if got := r.URL.Query().Get("direction"); got != "forward" {
			t.Fatalf("direction = %q, want forward", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "streams",
				"result": []map[string]any{{
					"stream": map[string]string{"app": "api"},
					"values": [][]string{{"1710000002000000000", "new"}, {"1710000001000000000", "old"}},
				}},
			},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newLokiTestHost(server.URL, "")

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "loki-dev", "query": `{app="api"}`, "limit": 5000, "direction": "forward"}, plugintest.WithHost(host))
	if out.Limit != 1000 || len(out.Entries) != 2 || out.Entries[0].Line != "old" || out.Entries[1].Line != "new" {
		t.Fatalf("query output = %#v", out)
	}

	err := plugintest.RunError(t, plugin, OperationQuery, map[string]any{"endpoint_ref": "loki-dev", "query": `{app="api"}`, "direction": "sideways"}, plugintest.WithHost(host))
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestManifestQuality(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	plugintest.AssertManifestQuality(t, manifest)
}

func TestDatasourceHandlersUseLokiAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loki/api/v1/query_range":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "streams",
					"result": []map[string]any{{
						"stream": map[string]string{"app": "api", "namespace": "prod", "pod": "api-123", "container": "api"},
						"values": [][]string{{"1710000000123456000", "hello"}},
					}},
				},
			})
		case "/loki/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"app", "namespace"}})
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newLokiTestHost(server.URL, "")

	logs := plugintest.DatasourceSearchOK[LogEntriesDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceLogEntries, "endpoint_ref": "loki-dev", "query": `{app="api"}`, "since": "1m"}, plugintest.WithHost(host))
	if logs.Count != 1 || logs.Records[0].Line != "hello" || logs.Records[0].Pod != "api-123" {
		t.Fatalf("log datasource = %#v", logs)
	}
	labels := plugintest.DatasourceSearchOK[LabelDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceLabels, "endpoint_ref": "loki-dev"}, plugintest.WithHost(host))
	if labels.Count != 2 || labels.Records[0].Name != "app" {
		t.Fatalf("labels datasource = %#v", labels)
	}
}

func TestRecentLogsBuildsSelector(t *testing.T) {
	query := recentLogsQuery(RecentLogsInput{App: "api", Namespace: "prod", Contains: "error"})
	if query != `{app="api",namespace="prod"} |= "error"` {
		t.Fatalf("query = %q", query)
	}
	escaped := recentLogsQuery(RecentLogsInput{App: `api"blue`, Contains: `err\or "x"`})
	if escaped != `{app="api\"blue"} |= "err\\or \"x\""` {
		t.Fatalf("escaped query = %q", escaped)
	}
}

func TestLabelsRejectInvalidLabelAndFailedStatus(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationLabels, map[string]any{"endpoint_ref": "loki-dev", "label": `bad/name`}, plugintest.WithHost(newLokiTestHost("http://127.0.0.1:1", "")))
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "data": []string{}})
	}))
	defer server.Close()
	err = plugintest.RunError(t, plugin, OperationLabels, map[string]any{"endpoint_ref": "loki-dev"}, plugintest.WithHost(newLokiTestHost(server.URL, "")))
	if err.Code != "loki" {
		t.Fatalf("err = %#v", err)
	}
}

func TestDatasourceDeclaresNetworkAccessAndTenantSecret(t *testing.T) {
	for _, spec := range []core.DatasourceSpec{logEntriesDatasourceSpec(), labelsDatasourceSpec()} {
		if !hasLokiDatasourceAccess(spec, core.OperationAccessNetwork) {
			t.Fatalf("%s access = %v, want network", spec.Name, spec.Access)
		}
		if len(spec.SecretPurposes) != 1 || spec.SecretPurposes[0] != AuthPurposeTenantID {
			t.Fatalf("%s secret purposes = %v, want tenant_id", spec.Name, spec.SecretPurposes)
		}
	}
}

func hasLokiDatasourceAccess(spec core.DatasourceSpec, want core.OperationAccess) bool {
	for _, access := range spec.Access {
		if access == want {
			return true
		}
	}
	return false
}

type lokiTestHost struct {
	baseURL  string
	tenantID string
}

func newLokiTestHost(baseURL, tenantID string) *lokiTestHost {
	return &lokiTestHost{baseURL: strings.TrimRight(baseURL, "/"), tenantID: tenantID}
}

func (h *lokiTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	if purpose == AuthPurposeTenantID && h.tenantID != "" {
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.tenantID}, nil
	}
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func (h *lokiTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *lokiTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *lokiTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *lokiTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *lokiTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, h.baseURL+"/"+strings.TrimLeft(input.Path, "/"), bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	if input.Auth != nil && input.Auth.HeaderPurposes["X-Scope-OrgID"] == AuthPurposeTenantID && h.tenantID != "" {
		req.Header.Set("X-Scope-OrgID", h.tenantID)
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

func (h *lokiTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *lokiTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *lokiTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *lokiTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *lokiTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*lokiTestHost)(nil)
