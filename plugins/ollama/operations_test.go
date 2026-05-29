package ollama

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

const testEndpointRef = "ollama-local"

func TestInfoReturnsVersion(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /api/version": `{"version":"0.5.7"}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[Version](t, plugin, OperationInfo, InfoInput{
		OllamaTargetInput: testTarget(),
	}, plugintest.WithHost(host))
	if out.Version != "0.5.7" {
		t.Fatalf("version = %q", out.Version)
	}
}

func TestInfoRequiresEndpointRef(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationInfo, InfoInput{})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelListParsesTags(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","size":4920753920,"digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}},
			{"name":"mistral:7b","size":4109016704,"digest":"def","details":{"family":"mistral","parameter_size":"7B","quantization_level":"Q4_K_M"}}
		]}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{
		OllamaTargetInput: testTarget(),
	}, plugintest.WithHost(host))
	if len(out.Items) != 2 || out.Items[0].Name != "llama3:8b" || out.Items[1].Details.Family != "mistral" {
		t.Fatalf("list = %#v", out)
	}
}

func TestModelShowRequiresName(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationModelShow, ModelShowInput{OllamaTargetInput: testTarget()})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestGenerateSendsStreamFalseAndParsesResponse(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /api/generate": `{"model":"llama3:8b","response":"hello world","done":true,"eval_count":7}`,
	})
	plugin := NewPluginWithService(NewService())

	temp := 0.2
	out := plugintest.RunOK[GenerateResult](t, plugin, OperationGenerate, GenerateInput{
		OllamaTargetInput: testTarget(),
		Model:             "llama3:8b",
		Prompt:            "say hi",
		System:            "be terse",
		Format:            "json",
		Options:           &GenerationOptions{Temperature: &temp, TopK: 40, Stop: []string{"###"}},
	}, plugintest.WithHost(host))
	if out.Response != "hello world" || !out.Done || out.EvalCount != 7 {
		t.Fatalf("result = %#v", out)
	}
	body := host.lastBody()
	if body["stream"] != false {
		t.Fatalf("stream flag should be false: %#v", body)
	}
	if body["system"] != "be terse" || body["format"] != "json" {
		t.Fatalf("body = %#v", body)
	}
	opts, ok := body["options"].(map[string]any)
	if !ok || opts["temperature"] != 0.2 || opts["top_k"] != float64(40) {
		t.Fatalf("options = %#v", body["options"])
	}
	stops, ok := opts["stop"].([]any)
	if !ok || len(stops) != 1 || stops[0] != "###" {
		t.Fatalf("stop = %#v", opts["stop"])
	}
}

func TestGenerationOptionsMarshalsZeroTemperature(t *testing.T) {
	temp := 0.0
	raw, err := json.Marshal(&GenerationOptions{Temperature: &temp})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"temperature":0}` {
		t.Fatalf("marshal = %s; want temperature:0 to be preserved", raw)
	}
}

func TestGenerationOptionsExtraMerges(t *testing.T) {
	temp := 0.7
	raw, err := json.Marshal(&GenerationOptions{
		Temperature: &temp,
		Extra: map[string]any{
			"tfs_z":            1.5,
			"penalize_newline": true,
			"temperature":      999.0,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["temperature"] != 0.7 {
		t.Fatalf("typed field should win: %#v", decoded["temperature"])
	}
	if decoded["tfs_z"] != 1.5 || decoded["penalize_newline"] != true {
		t.Fatalf("extra fields = %#v", decoded)
	}
}

func TestGenerateRejectsMissingPrompt(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationGenerate, GenerateInput{OllamaTargetInput: testTarget(), Model: "llama3"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestChatSendsMessages(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /api/chat": `{"model":"llama3","message":{"role":"assistant","content":"hi back"},"done":true}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[ChatResult](t, plugin, OperationChat, ChatInput{
		OllamaTargetInput: testTarget(),
		Model:             "llama3",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}, plugintest.WithHost(host))
	if out.Message.Content != "hi back" || out.Message.Role != "assistant" {
		t.Fatalf("chat result = %#v", out)
	}
	body := host.lastBody()
	msgs, ok := body["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages payload = %#v", body["messages"])
	}
}

func TestChatRejectsEmptyMessages(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationChat, ChatInput{OllamaTargetInput: testTarget(), Model: "llama3"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestEmbedParsesVectors(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /api/embed": `{"model":"all-minilm","embeddings":[[0.1,0.2,0.3]]}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[EmbedResult](t, plugin, OperationEmbed, EmbedInput{
		OllamaTargetInput: testTarget(),
		Model:             "all-minilm",
		Input:             []string{"hello"},
	}, plugintest.WithHost(host))
	if len(out.Embeddings) != 1 || len(out.Embeddings[0]) != 3 || out.Embeddings[0][1] != 0.2 {
		t.Fatalf("embed result = %#v", out)
	}
}

func TestDatasourceLookupReturnsModel(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}}
		]}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.DatasourceLookupOK[pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]](
		t, plugin,
		pluginbinding.DatasourceLookupInput{Text: "llama3", Entity: EntityModel, EndpointRef: testEndpointRef},
		plugintest.WithInstance("local"),
		plugintest.WithHost(host),
	)
	if out.Count != 1 || out.Matches[0].ID != "llama3:8b" {
		t.Fatalf("lookup = %#v", out)
	}
}

func TestEmbedRejectsEmptyInput(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationEmbed, EmbedInput{OllamaTargetInput: testTarget(), Model: "all-minilm"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelGetReturnsRecord(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}}
		]}`,
	})
	plugin := NewPluginWithService(NewService())

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[ModelRecord]](
		t, plugin,
		pluginbinding.DatasourceGetInput{ID: "llama3:8b", Entity: EntityModel, EndpointRef: testEndpointRef},
		plugintest.WithHost(host),
	)
	if out.Record.ModelName != "llama3:8b" || out.Record.Family != "llama" {
		t.Fatalf("get = %#v", out)
	}
}

func TestModelGetNotFound(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama"}}
		]}`,
	})
	plugin := NewPluginWithService(NewService())

	err := plugintest.DatasourceError(t, plugin, "datasources.get",
		pluginbinding.DatasourceGetInput{ID: "missing:1b", Entity: EntityModel, EndpointRef: testEndpointRef},
		plugintest.WithHost(host),
	)
	if err == nil || err.Code != "not_found" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelShowSurfacesServerError(t *testing.T) {
	host := newStatusHost(t, map[string]hostRoute{
		"POST /api/show": {status: http.StatusNotFound, body: `{"error":"model 'ghost' not found"}`},
	})
	plugin := NewPluginWithService(NewService())

	err := plugintest.RunError(t, plugin, OperationModelShow, ModelShowInput{
		OllamaTargetInput: testTarget(),
		Name:              "ghost",
	}, plugintest.WithHost(host))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Message, "model 'ghost' not found") {
		t.Fatalf("error message = %q; want it to include ollama error", err.Message)
	}
}

func testTarget() OllamaTargetInput {
	return OllamaTargetInput{EndpointRef: testEndpointRef}
}

type hostRoute struct {
	status int
	body   string
}

type routedHost struct {
	t        *testing.T
	routes   map[string]hostRoute
	lastJSON map[string]any
}

func newRoutedHost(t *testing.T, routes map[string]string) *routedHost {
	mapped := make(map[string]hostRoute, len(routes))
	for key, body := range routes {
		mapped[key] = hostRoute{status: http.StatusOK, body: body}
	}
	return newStatusHost(t, mapped)
}

func newStatusHost(t *testing.T, routes map[string]hostRoute) *routedHost {
	return &routedHost{t: t, routes: routes}
}

func (h *routedHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *routedHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *routedHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *routedHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *routedHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *routedHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	if input.EndpointRef != testEndpointRef {
		h.t.Fatalf("endpoint_ref = %q, want %q", input.EndpointRef, testEndpointRef)
	}
	method := input.Method
	if method == "" {
		method = "GET"
	}
	key := method + " " + input.Path
	route, ok := h.routes[key]
	if !ok {
		h.t.Fatalf("unexpected request: %s", key)
	}
	h.lastJSON = nil
	if len(input.Body) > 0 {
		_ = json.Unmarshal(input.Body, &h.lastJSON)
	}
	status := route.status
	if status == 0 {
		status = http.StatusOK
	}
	return pluginbinding.HTTPResponse{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       []byte(route.body),
	}, nil
}

func (h *routedHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *routedHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *routedHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *routedHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *routedHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

func (h *routedHost) lastBody() map[string]any {
	return h.lastJSON
}

var _ pluginbinding.HostClient = (*routedHost)(nil)
