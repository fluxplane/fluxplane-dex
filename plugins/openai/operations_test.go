package openai

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
)

const testEndpointRef = "openai-default"

func TestImageGenerateSendsRequestAndParsesResponse(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /images/generations": `{"created":1737000000,"data":[{"b64_json":"AAAA","revised_prompt":"a refined prompt"}],"background":"transparent","output_format":"png","size":"1024x1024","quality":"high","usage":{"total_tokens":100,"input_tokens":50,"output_tokens":50,"input_tokens_details":{"text_tokens":10,"image_tokens":40}}}`,
	})
	plugin := newTestPlugin()

	out := plugintest.RunOK[ImageGenerateResult](t, plugin, OperationImageGenerate, ImageGenerateInput{
		OpenAITargetInput: testTarget(),
		Prompt:            "a cute baby sea otter",
		Model:             "gpt-image-1",
		N:                 1,
		Size:              "1024x1024",
		Quality:           "high",
		Background:        "transparent",
	}, plugintest.WithHost(host))
	if len(out.Data) != 1 || out.Data[0].B64JSON != "AAAA" || out.Data[0].RevisedPrompt != "a refined prompt" {
		t.Fatalf("result = %#v", out)
	}
	if out.Background != "transparent" || out.Size != "1024x1024" || out.Quality != "high" {
		t.Fatalf("metadata = %#v", out)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 100 || out.Usage.InputTokensDetails == nil || out.Usage.InputTokensDetails.ImageTokens != 40 {
		t.Fatalf("usage = %#v", out.Usage)
	}
	req := host.lastRequest
	if req.Auth == nil || req.Auth.BearerTokenPurpose != AuthPurposeAPIKey {
		t.Fatalf("auth request = %#v", req.Auth)
	}
	body := host.lastBody()
	if body["prompt"] != "a cute baby sea otter" || body["model"] != "gpt-image-1" {
		t.Fatalf("body = %#v", body)
	}
	if body["quality"] != "high" || body["background"] != "transparent" {
		t.Fatalf("body = %#v", body)
	}
}

func TestImageGenerateOmitsUnsetOptionalFields(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /images/generations": `{"created":1737000000,"data":[{"url":"https://cdn.example.com/img.png"}]}`,
	})
	plugin := newTestPlugin()

	_ = plugintest.RunOK[ImageGenerateResult](t, plugin, OperationImageGenerate, ImageGenerateInput{
		OpenAITargetInput: testTarget(),
		Prompt:            "hello",
	}, plugintest.WithHost(host))
	body := host.lastBody()
	for _, key := range []string{"model", "n", "size", "quality", "style", "response_format", "output_format", "background", "moderation", "output_compression", "user"} {
		if _, present := body[key]; present {
			t.Errorf("expected %q to be omitted, got %#v", key, body[key])
		}
	}
}

func TestImageGenerateRejectsEmptyPrompt(t *testing.T) {
	plugin := newTestPlugin()
	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{OpenAITargetInput: testTarget()})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestImageGenerateRejectsInvalidBounds(t *testing.T) {
	plugin := newTestPlugin()
	cases := []struct {
		name  string
		input ImageGenerateInput
		want  string
	}{
		{
			name:  "negative n",
			input: ImageGenerateInput{OpenAITargetInput: testTarget(), Prompt: "hello", N: -1},
			want:  "n must be between 1 and 10",
		},
		{
			name:  "too many images",
			input: ImageGenerateInput{OpenAITargetInput: testTarget(), Prompt: "hello", N: 11},
			want:  "n must be between 1 and 10",
		},
		{
			name:  "negative compression",
			input: ImageGenerateInput{OpenAITargetInput: testTarget(), Prompt: "hello", OutputCompression: -1},
			want:  "output_compression must be between 0 and 100",
		},
		{
			name:  "too much compression",
			input: ImageGenerateInput{OpenAITargetInput: testTarget(), Prompt: "hello", OutputCompression: 101},
			want:  "output_compression must be between 0 and 100",
		},
		{
			name:  "prompt too long",
			input: ImageGenerateInput{OpenAITargetInput: testTarget(), Prompt: strings.Repeat("x", 32001)},
			want:  "prompt must be at most 32000 characters",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := plugintest.RunError(t, plugin, OperationImageGenerate, tc.input)
			if err == nil || err.Code != "bad_input" || !strings.Contains(err.Message, tc.want) {
				t.Fatalf("err = %#v, want bad_input containing %q", err, tc.want)
			}
		})
	}
}

func TestImageGenerateRequiresEndpointRef(t *testing.T) {
	plugin := newTestPlugin()
	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{Prompt: "hello"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestImageGenerateSurfacesOpenAIError(t *testing.T) {
	host := newStatusHost(t, map[string]hostRoute{
		"POST /images/generations": {
			status: http.StatusBadRequest,
			body:   `{"error":{"message":"invalid_request_error: prompt too long","type":"invalid_request_error","code":"prompt_too_long"}}`,
		},
	})
	plugin := newTestPlugin()

	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{
		OpenAITargetInput: testTarget(),
		Prompt:            "x",
	}, plugintest.WithHost(host))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Message, "prompt too long") {
		t.Fatalf("error message = %q; want it to include OpenAI error", err.Message)
	}
}

func TestImageGenerateSurfacesHostAuthError(t *testing.T) {
	host := newStatusHost(t, map[string]hostRoute{
		"POST /images/generations": {err: errors.New("api key not configured")},
	})
	plugin := newTestPlugin()

	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{
		OpenAITargetInput: testTarget(),
		Prompt:            "hello",
	}, plugintest.WithHost(host))
	if err == nil || !strings.Contains(err.Message, "api key not configured") {
		t.Fatalf("err = %#v", err)
	}
}

func TestVisionAnalyzeSendsResponsesRequestAndParsesText(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /responses": `{"id":"resp_123","model":"gpt-4.1-mini","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The image shows a receipt."}]}],"usage":{"input_tokens":42,"output_tokens":9}}`,
	})
	plugin := newTestPlugin()

	out := plugintest.RunOK[vision.AnalyzeOutput](t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{
		EndpointRef: testEndpointRef,
		Prompt:      "Describe it",
		Model:       "gpt-4.1-mini",
		MaxTokens:   128,
		Images: []vision.ImageInput{{
			URL:    "https://example.com/receipt.png",
			Detail: "high",
		}},
	}, plugintest.WithHost(host))
	if len(out.Results) != 1 || out.Results[0].Provider != PluginName || out.Results[0].Text != "The image shows a receipt." {
		t.Fatalf("result = %#v", out)
	}
	body := host.lastBody()
	if body["model"] != "gpt-4.1-mini" || body["max_output_tokens"] != float64(128) {
		t.Fatalf("body = %#v", body)
	}
	input := body["input"].([]any)[0].(map[string]any)
	content := input["content"].([]any)
	if content[0].(map[string]any)["text"] != "Describe it" {
		t.Fatalf("content = %#v", content)
	}
	image := content[1].(map[string]any)
	if image["image_url"] != "https://example.com/receipt.png" || image["detail"] != "high" {
		t.Fatalf("image content = %#v", image)
	}
}

func TestVisionAnalyzeAcceptsInlineContentBytes(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"POST /responses": `{"output_text":"A diagram."}`,
	})
	plugin := newTestPlugin()

	_ = plugintest.RunOK[vision.AnalyzeOutput](t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{
		EndpointRef: testEndpointRef,
		Images:      []vision.ImageInput{{ContentBytes: []byte("png bytes"), Filename: "chart.png"}},
	}, plugintest.WithHost(host))
	content := host.lastBody()["input"].([]any)[0].(map[string]any)["content"].([]any)
	image := content[1].(map[string]any)
	if image["image_url"] != "data:image/png;base64,cG5nIGJ5dGVz" {
		t.Fatalf("image content = %#v", image)
	}
}

func TestVisionAnalyzeRejectsMissingImage(t *testing.T) {
	plugin := newTestPlugin()
	err := plugintest.RunError(t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{EndpointRef: testEndpointRef})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelListParsesData(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /models": `{"object":"list","data":[
			{"id":"gpt-image-1","object":"model","created":1700000000,"owned_by":"openai"},
			{"id":"dall-e-3","object":"model","created":1690000000,"owned_by":"openai"},
			{"id":"gpt-4o","object":"model","created":1715000000,"owned_by":"openai"}
		]}`,
	})
	plugin := newTestPlugin()

	out := plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{
		OpenAITargetInput: testTarget(),
	}, plugintest.WithHost(host))
	if len(out.Items) != 3 || out.Items[0].ID != "gpt-image-1" || out.Items[2].OwnedBy != "openai" {
		t.Fatalf("list = %#v", out)
	}
}

func TestModelListRequestsAuthAndOptionalHeadersByPurpose(t *testing.T) {
	host := newRoutedHost(t, map[string]string{
		"GET /models": `{"object":"list","data":[]}`,
	})
	plugin := newTestPlugin()

	_ = plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{
		OpenAITargetInput: testTarget(),
	}, plugintest.WithHost(host))
	auth := host.lastRequest.Auth
	if auth == nil || auth.BearerTokenPurpose != AuthPurposeAPIKey {
		t.Fatalf("auth = %#v", auth)
	}
	if auth.HeaderPurposes["OpenAI-Organization"] != AuthPurposeOrganization {
		t.Fatalf("organization header purpose = %#v", auth.HeaderPurposes)
	}
	if auth.HeaderPurposes["OpenAI-Project"] != AuthPurposeProject {
		t.Fatalf("project header purpose = %#v", auth.HeaderPurposes)
	}
}

func newTestPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func testTarget() OpenAITargetInput {
	return OpenAITargetInput{EndpointRef: testEndpointRef}
}

type hostRoute struct {
	status int
	body   string
	err    error
}

type routedHost struct {
	t           *testing.T
	routes      map[string]hostRoute
	lastRequest pluginbinding.HTTPRequest
	lastJSON    map[string]any
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
	h.lastRequest = input
	h.lastJSON = nil
	if len(input.Body) > 0 {
		_ = json.Unmarshal(input.Body, &h.lastJSON)
	}
	if route.err != nil {
		return pluginbinding.HTTPResponse{}, route.err
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

func (h *routedHost) BlobRead(input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
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
