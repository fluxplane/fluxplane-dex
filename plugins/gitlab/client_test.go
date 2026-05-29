package gitlab

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestNewLiveClientUsesEndpointRefHostHTTP(t *testing.T) {
	host := &gitLabLiveClientTestHost{}
	input, err := json.Marshal(map[string]any{"endpoint_ref": "gitlab-dev"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := pluginbinding.Context{
		Call: protocol.OperationCall{Name: OperationAuthTest, Input: input},
		Host: host,
	}
	client, err := NewLiveClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	user, err := client.CurrentUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "timo" {
		t.Fatalf("user = %#v", user)
	}
	if host.request.URL != "" || host.request.EndpointRef != "gitlab-dev" {
		t.Fatalf("request endpoint = %#v", host.request)
	}
	if host.request.Path != "/api/v4/user" || host.request.Method != "GET" {
		t.Fatalf("request target = %#v", host.request)
	}
	if host.request.Auth == nil || host.request.Auth.BearerTokenPurpose != AuthPurposeAccessToken {
		t.Fatalf("auth = %#v", host.request.Auth)
	}
}

func TestNewLiveClientRequiresEndpointRef(t *testing.T) {
	_, err := NewLiveClient(pluginbinding.Context{Host: &gitLabLiveClientTestHost{}})
	if err == nil || !strings.Contains(err.Error(), "endpoint_ref is required") {
		t.Fatalf("err = %v", err)
	}
}

type gitLabLiveClientTestHost struct {
	request pluginbinding.HTTPRequest
}

func (h *gitLabLiveClientTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *gitLabLiveClientTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *gitLabLiveClientTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *gitLabLiveClientTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *gitLabLiveClientTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *gitLabLiveClientTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	h.request = input
	return pluginbinding.HTTPResponse{
		Status:     "200 OK",
		StatusCode: 200,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{"id":9,"username":"timo","name":"Timo"}`),
	}, nil
}

func (h *gitLabLiveClientTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *gitLabLiveClientTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *gitLabLiveClientTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *gitLabLiveClientTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *gitLabLiveClientTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*gitLabLiveClientTestHost)(nil)
