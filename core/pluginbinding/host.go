package pluginbinding

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	ManifestProtocolKey = "dex.protocol"
	HostSecretGet       = "host.secret.get"
	HostIndexLookup     = "host.index.lookup"
	HostIndexSearch     = "host.index.search"
	HostIndexGet        = "host.index.get"
	HostEndpointResolve = "host.endpoint.resolve"
)

type HostClient interface {
	Secret(purpose string) (SecretMaterial, error)
	Lookup(input DatasourceLookupInput) (DatasourceLookupResult[LookupMatch[any]], error)
	Search(input DatasourceSearchInput) (DatasourceSearchResult[any], error)
	Get(input DatasourceGetInput) (DatasourceGetResult[any], error)
	ResolveEndpoint(ref string) (core.EndpointRef, error)
	HTTP(input HTTPRequest) (HTTPResponse, error)
	BlobRead(input BlobReadRequest) (BlobReadResponse, error)
	BlobWrite(input BlobWriteRequest) (BlobRef, error)
	BlobInfo(input BlobInfoRequest) (BlobRef, error)
	EnvLookup(key string) (EnvLookupResponse, error)
	CapabilityCall(input ProviderCallRequest) (ProviderCallResponse, error)
}

type hostClient struct {
	caller protocol.HostCaller
}

type unavailableHostClient struct{}

func newHostClient(caller protocol.HostCaller) HostClient {
	if caller == nil {
		return unavailableHostClient{}
	}
	return hostClient{caller: caller}
}

func (h hostClient) Secret(purpose string) (SecretMaterial, error) {
	var out SecretMaterial
	err := h.call(HostSecretGet, map[string]any{"purpose": strings.TrimSpace(purpose)}, &out)
	if out.Purpose == "" {
		out.Purpose = strings.TrimSpace(purpose)
	}
	return out, err
}

func (h hostClient) Lookup(input DatasourceLookupInput) (DatasourceLookupResult[LookupMatch[any]], error) {
	var out DatasourceLookupResult[LookupMatch[any]]
	err := h.call(HostIndexLookup, input, &out)
	return out, err
}

func (h hostClient) Search(input DatasourceSearchInput) (DatasourceSearchResult[any], error) {
	var out DatasourceSearchResult[any]
	err := h.call(HostIndexSearch, input, &out)
	return out, err
}

func (h hostClient) Get(input DatasourceGetInput) (DatasourceGetResult[any], error) {
	var out DatasourceGetResult[any]
	err := h.call(HostIndexGet, input, &out)
	return out, err
}

func (h hostClient) ResolveEndpoint(ref string) (core.EndpointRef, error) {
	var out core.EndpointRef
	err := h.call(HostEndpointResolve, map[string]any{"endpoint_ref": strings.TrimSpace(ref)}, &out)
	return out, err
}

func (h hostClient) HTTP(input HTTPRequest) (HTTPResponse, error) {
	var out HTTPResponse
	err := h.call(protocol.HostCapabilityHTTPDo, input, &out)
	return out, err
}

func (h hostClient) BlobRead(input BlobReadRequest) (BlobReadResponse, error) {
	var out BlobReadResponse
	err := h.call(protocol.HostCapabilityBlobRead, input, &out)
	return out, err
}

func (h hostClient) BlobWrite(input BlobWriteRequest) (BlobRef, error) {
	var out BlobRef
	err := h.call(protocol.HostCapabilityBlobWrite, input, &out)
	return out, err
}

func (h hostClient) BlobInfo(input BlobInfoRequest) (BlobRef, error) {
	var out BlobRef
	err := h.call(protocol.HostCapabilityBlobInfo, input, &out)
	return out, err
}

func (h hostClient) EnvLookup(key string) (EnvLookupResponse, error) {
	var out EnvLookupResponse
	err := h.call(protocol.HostCapabilityEnvLookup, EnvLookupRequest{Key: strings.TrimSpace(key)}, &out)
	return out, err
}

func (h hostClient) CapabilityCall(input ProviderCallRequest) (ProviderCallResponse, error) {
	var out ProviderCallResponse
	err := h.call(protocol.HostCapabilityProviderCall, input, &out)
	return out, err
}

func (h hostClient) call(command string, input any, out any) error {
	raw, err := h.caller.CallHost(command, input)
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (unavailableHostClient) Secret(string) (SecretMaterial, error) {
	return SecretMaterial{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) Lookup(DatasourceLookupInput) (DatasourceLookupResult[LookupMatch[any]], error) {
	return DatasourceLookupResult[LookupMatch[any]]{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) Search(DatasourceSearchInput) (DatasourceSearchResult[any], error) {
	return DatasourceSearchResult[any]{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) Get(DatasourceGetInput) (DatasourceGetResult[any], error) {
	return DatasourceGetResult[any]{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) HTTP(HTTPRequest) (HTTPResponse, error) {
	return HTTPResponse{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) BlobRead(BlobReadRequest) (BlobReadResponse, error) {
	return BlobReadResponse{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) BlobWrite(BlobWriteRequest) (BlobRef, error) {
	return BlobRef{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) BlobInfo(BlobInfoRequest) (BlobRef, error) {
	return BlobRef{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) EnvLookup(string) (EnvLookupResponse, error) {
	return EnvLookupResponse{}, fmt.Errorf("host client is unavailable")
}

func (unavailableHostClient) CapabilityCall(ProviderCallRequest) (ProviderCallResponse, error) {
	return ProviderCallResponse{}, fmt.Errorf("host client is unavailable")
}
