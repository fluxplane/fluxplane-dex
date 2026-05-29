package runtime

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestHTTPRequestURLPreservesEscapedPath(t *testing.T) {
	got, err := httpRequestURL(pluginbinding.HTTPRequest{
		URL:  "https://gitlab.example.com",
		Path: "/api/v4/projects/group%2Frepo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://gitlab.example.com/api/v4/projects/group%2Frepo" {
		t.Fatalf("url = %q", got)
	}
}

func TestHandleHostCapabilityRejectsMissingGrant(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityEnvLookup, pluginbinding.EnvLookupRequest{Key: "HOME"})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "example", "default", "", frame)
	if resp.OK || resp.Error == nil || resp.Error.Code != "forbidden" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestHandleHostBlobReadUsesCapabilityHost(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("example", "default", []string{"example.read"}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityBlobRead}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state, Capabilities: fakeCapabilityHost{
		blobRead: func(_ context.Context, input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
			if input.Path != "input.txt" {
				t.Fatalf("input = %#v", input)
			}
			return pluginbinding.BlobReadResponse{
				Blob:    pluginbinding.BlobRef{Ref: "workspace:input.txt", Path: "input.txt", Size: 5},
				Content: []byte("hello"),
			}, nil
		},
	}}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityBlobRead, pluginbinding.BlobReadRequest{Path: "input.txt"})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "example", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var out pluginbinding.BlobReadResponse
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if string(out.Content) != "hello" || out.Blob.Ref != "workspace:input.txt" {
		t.Fatalf("blob response = %#v", out)
	}
}

func TestHandleHostBlobReadUsesDefaultLocalCapabilityHost(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("example", "default", []string{"example.read"}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityBlobRead}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(root+"/input.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})

	runner := Runner{State: state}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityBlobRead, pluginbinding.BlobReadRequest{Path: "input.txt"})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "example", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var out pluginbinding.BlobReadResponse
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if string(out.Content) != "hello" || out.Blob.Path != "input.txt" {
		t.Fatalf("blob response = %#v", out)
	}
}

func TestHandleHostHTTPResolvesEndpointRefAndSecretAuth(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/datasources" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "grafana-dev", URL: server.URL, Product: "grafana"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSecret("grafana", "default", "api_token", StoredSecret{Value: "glsa_test"}); err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("grafana", "default", []string{"grafana.datasource.list"}, []SecretPurpose{{Name: "api_token"}}, []CapabilityGrant{{Name: pluginbinding.CapabilityHTTP}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityHTTPDo, pluginbinding.HTTPRequest{
		EndpointRef: "grafana-dev",
		Path:        "/api/datasources",
		Method:      "GET",
		Auth:        &pluginbinding.HTTPAuthRequest{BearerTokenPurpose: "api_token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "grafana", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	if auth != "Bearer glsa_test" {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestHandleHostProviderCallUsesGenericProviderGrant(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("example", "default", []string{"example.call"}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityProvider, Provider: "cluster", Action: "list"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State: state,
		Providers: map[string]HostProvider{
			"cluster": providerFunc(func(_ context.Context, action string, payload json.RawMessage) (json.RawMessage, error) {
				if action != "list" || !strings.Contains(string(payload), "pods") {
					t.Fatalf("action=%q payload=%s", action, string(payload))
				}
				return json.RawMessage(`{"ok":true}`), nil
			}),
		},
	}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityProviderCall, pluginbinding.ProviderCallRequest{
		Provider: "cluster",
		Action:   "list",
		Payload:  json.RawMessage(`{"kind":"pods"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "example", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
}

func TestDefaultCapabilityHostServesSystemProvider(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("system", "default", []string{"system.info"}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityProvider, Provider: "system", Action: "info"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityProviderCall, pluginbinding.ProviderCallRequest{
		Provider: "system",
		Action:   "info",
		Payload:  json.RawMessage(`{"categories":["os","time"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "system", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var providerOut pluginbinding.ProviderCallResponse
	if err := json.Unmarshal(resp.Result, &providerOut); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Categories []string       `json:"categories"`
		System     map[string]any `json:"system"`
	}
	if err := json.Unmarshal(providerOut.Result, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Categories) != 2 || out.Categories[0] != "os" || out.Categories[1] != "time" {
		t.Fatalf("categories = %#v", out.Categories)
	}
	if _, ok := out.System["os"]; !ok {
		t.Fatalf("system response missing os: %#v", out.System)
	}
	if _, ok := out.System["time"]; !ok {
		t.Fatalf("system response missing time: %#v", out.System)
	}
}

func TestHostSQLProviderQueriesEndpointRef(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dbPath := t.TempDir() + "/app.db"
	db, err := stdsql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table users (id integer primary key, name text); insert into users (id, name) values (1, 'Ada'), (2, 'Linus')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "sqlite-dev", URL: "sqlite://" + dbPath, Product: "sql"}); err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrantWithCapabilities("sql", "default", []string{"sql.query"}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityProvider, Provider: "sql", Action: "query"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state}
	frame, err := protocol.NewRequestFrame("cap-1", protocol.TargetHost, protocol.HostCapabilityProviderCall, pluginbinding.ProviderCallRequest{
		Provider: "sql",
		Action:   "query",
		Payload:  json.RawMessage(`{"endpoint_ref":"sqlite-dev","query":"select id, name from users order by id","max_rows":10}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := runner.handleHostRequest(context.Background(), "sql", "default", grant.Token, frame)
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var providerOut pluginbinding.ProviderCallResponse
	if err := json.Unmarshal(resp.Result, &providerOut); err != nil {
		t.Fatal(err)
	}
	var out struct {
		EndpointRef string           `json:"endpoint_ref"`
		Driver      string           `json:"driver"`
		Rows        []map[string]any `json:"rows"`
		RowCount    int              `json:"row_count"`
	}
	if err := json.Unmarshal(providerOut.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.EndpointRef != "sqlite-dev" || out.Driver != "sqlite" || out.RowCount != 2 || out.Rows[0]["name"] != "Ada" {
		t.Fatalf("sql output = %#v", out)
	}
}

func TestOperationCapabilityGrantsLimitBlobWriteToWriteOperations(t *testing.T) {
	readOnlySpec := core.OperationSpec{
		ReadOnly: true,
		Access:   []core.OperationAccess{core.OperationAccessFilesystem},
		Effects:  []core.OperationEffect{core.OperationEffectRead, core.OperationEffectFilesystem},
	}
	readOnlyGrants := operationCapabilityGrants(readOnlySpec)
	if !hasCapabilityGrant(readOnlyGrants, pluginbinding.CapabilityBlobRead) {
		t.Fatalf("read-only grants missing blob read: %#v", readOnlyGrants)
	}
	if hasCapabilityGrant(readOnlyGrants, pluginbinding.CapabilityBlobWrite) {
		t.Fatalf("read-only grants include blob write: %#v", readOnlyGrants)
	}

	writeSpec := core.OperationSpec{
		Access:  []core.OperationAccess{core.OperationAccessFilesystem},
		Effects: []core.OperationEffect{core.OperationEffectWrite, core.OperationEffectFilesystem},
	}
	writeGrants := operationCapabilityGrants(writeSpec)
	if !hasCapabilityGrant(writeGrants, pluginbinding.CapabilityBlobWrite) {
		t.Fatalf("write grants missing blob write: %#v", writeGrants)
	}
}

func hasCapabilityGrant(grants []CapabilityGrant, name string) bool {
	for _, grant := range grants {
		if grant.Name == name {
			return true
		}
	}
	return false
}

type providerFunc func(context.Context, string, json.RawMessage) (json.RawMessage, error)

func TestDatasourceCapabilityGrantsIncludeDeclaredProviderAccess(t *testing.T) {
	grants := datasourceCapabilityGrants(core.DatasourceSpec{
		Name:   "kubernetes.inventory",
		Access: []core.OperationAccess{core.OperationAccessProvider},
	})
	if !hasCapabilityGrantWithProvider(grants, pluginbinding.CapabilityProvider, "kubernetes") {
		t.Fatalf("datasource grants missing kubernetes provider access: %#v", grants)
	}
}

func (f providerFunc) Call(ctx context.Context, action string, payload json.RawMessage) (json.RawMessage, error) {
	return f(ctx, action, payload)
}

type fakeCapabilityHost struct {
	blobRead func(context.Context, pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error)
}

func (h fakeCapabilityHost) HTTP(context.Context, pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h fakeCapabilityHost) BlobRead(ctx context.Context, input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return h.blobRead(ctx, input)
}

func (h fakeCapabilityHost) BlobWrite(context.Context, pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h fakeCapabilityHost) BlobInfo(context.Context, pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h fakeCapabilityHost) EnvLookup(context.Context, pluginbinding.EnvLookupRequest) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h fakeCapabilityHost) ProviderCall(context.Context, pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

func hasCapabilityGrantWithProvider(grants []CapabilityGrant, name, provider string) bool {
	for _, grant := range grants {
		if grant.Name == name && grant.Provider == provider {
			return true
		}
	}
	return false
}
