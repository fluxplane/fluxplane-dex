package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
)

func TestImageGenerateSendsRequestAndParsesResponse(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /images/generations": `{"created":1737000000,"data":[{"b64_json":"AAAA","revised_prompt":"a refined prompt"}],"background":"transparent","output_format":"png","size":"1024x1024","quality":"high","usage":{"total_tokens":100,"input_tokens":50,"output_tokens":50,"input_tokens_details":{"text_tokens":10,"image_tokens":40}}}`,
	})
	plugin := newTestPlugin(doer, "test-key")

	out := plugintest.RunOK[ImageGenerateResult](t, plugin, OperationImageGenerate, ImageGenerateInput{
		Prompt:     "a cute baby sea otter",
		Model:      "gpt-image-1",
		N:          1,
		Size:       "1024x1024",
		Quality:    "high",
		Background: "transparent",
	})
	if len(out.Data) != 1 || out.Data[0].B64JSON != "AAAA" || out.Data[0].RevisedPrompt != "a refined prompt" {
		t.Fatalf("result = %#v", out)
	}
	if out.Background != "transparent" || out.Size != "1024x1024" || out.Quality != "high" {
		t.Fatalf("metadata = %#v", out)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 100 || out.Usage.InputTokensDetails == nil || out.Usage.InputTokensDetails.ImageTokens != 40 {
		t.Fatalf("usage = %#v", out.Usage)
	}
	if got := doer.lastReq.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("authorization = %q", got)
	}
	body := doer.lastBody()
	if body["prompt"] != "a cute baby sea otter" || body["model"] != "gpt-image-1" {
		t.Fatalf("body = %#v", body)
	}
	if body["quality"] != "high" || body["background"] != "transparent" {
		t.Fatalf("body = %#v", body)
	}
}

func TestImageGenerateOmitsUnsetOptionalFields(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /images/generations": `{"created":1737000000,"data":[{"url":"https://cdn.example.com/img.png"}]}`,
	})
	plugin := newTestPlugin(doer, "test-key")

	_ = plugintest.RunOK[ImageGenerateResult](t, plugin, OperationImageGenerate, ImageGenerateInput{Prompt: "hello"})
	body := doer.lastBody()
	for _, key := range []string{"model", "n", "size", "quality", "style", "response_format", "output_format", "background", "moderation", "output_compression", "user"} {
		if _, present := body[key]; present {
			t.Errorf("expected %q to be omitted, got %#v", key, body[key])
		}
	}
}

func TestImageGenerateRejectsEmptyPrompt(t *testing.T) {
	plugin := newTestPlugin(stubDoer{}, "test-key")
	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestImageGenerateSurfacesOpenAIError(t *testing.T) {
	doer := &errorDoer{
		status: http.StatusBadRequest,
		body:   `{"error":{"message":"invalid_request_error: prompt too long","type":"invalid_request_error","code":"prompt_too_long"}}`,
	}
	plugin := newTestPlugin(doer, "test-key")

	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{Prompt: "x"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Message, "prompt too long") {
		t.Fatalf("error message = %q; want it to include OpenAI error", err.Message)
	}
}

func TestImageGenerateFailsWhenSecretMissing(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Client: Client{HTTPClient: stubDoer{}, BaseURL: "https://openai.test"},
		SecretGetter: func(_ pluginbinding.Context, _ string) (pluginbinding.SecretMaterial, error) {
			return pluginbinding.SecretMaterial{}, pluginbinding.Fail("missing_secret", "no api key configured")
		},
	})
	err := plugintest.RunError(t, plugin, OperationImageGenerate, ImageGenerateInput{Prompt: "hello"})
	if err == nil || err.Code != "secret" {
		t.Fatalf("err = %#v", err)
	}
}

func TestVisionAnalyzeSendsResponsesRequestAndParsesText(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /responses": `{"id":"resp_123","model":"gpt-4.1-mini","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The image shows a receipt."}]}],"usage":{"input_tokens":42,"output_tokens":9}}`,
	})
	plugin := newTestPlugin(doer, "test-key")

	out := plugintest.RunOK[vision.AnalyzeOutput](t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{
		Prompt:    "Describe it",
		Model:     "gpt-4.1-mini",
		MaxTokens: 128,
		Images: []vision.ImageInput{{
			URL:    "https://example.com/receipt.png",
			Detail: "high",
		}},
	})
	if len(out.Results) != 1 || out.Results[0].Provider != PluginName || out.Results[0].Text != "The image shows a receipt." {
		t.Fatalf("result = %#v", out)
	}
	body := doer.lastBody()
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

func TestVisionAnalyzeAcceptsFilePath(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /responses": `{"output_text":"A diagram."}`,
	})
	plugin := newTestPlugin(doer, "test-key")
	path := filepath.Join(t.TempDir(), "chart.png")
	if err := os.WriteFile(path, []byte("png bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = plugintest.RunOK[vision.AnalyzeOutput](t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{
		Images: []vision.ImageInput{{FilePath: path}},
	})
	content := doer.lastBody()["input"].([]any)[0].(map[string]any)["content"].([]any)
	image := content[1].(map[string]any)
	if image["image_url"] != "data:image/png;base64,cG5nIGJ5dGVz" {
		t.Fatalf("image content = %#v", image)
	}
}

func TestVisionAnalyzeRejectsMissingImage(t *testing.T) {
	plugin := newTestPlugin(stubDoer{}, "test-key")
	err := plugintest.RunError(t, plugin, OperationVisionAnalyze, vision.AnalyzeInput{})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelListParsesData(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /models": `{"object":"list","data":[
			{"id":"gpt-image-1","object":"model","created":1700000000,"owned_by":"openai"},
			{"id":"dall-e-3","object":"model","created":1690000000,"owned_by":"openai"},
			{"id":"gpt-4o","object":"model","created":1715000000,"owned_by":"openai"}
		]}`,
	})
	plugin := newTestPlugin(doer, "test-key")

	out := plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{})
	if len(out.Items) != 3 || out.Items[0].ID != "gpt-image-1" || out.Items[2].OwnedBy != "openai" {
		t.Fatalf("list = %#v", out)
	}
}

func TestModelListSendsAuthorizationAndOptionalHeaders(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /models": `{"object":"list","data":[]}`,
	})
	plugin := NewPluginWithService(Service{
		Client: Client{
			HTTPClient:   doer,
			BaseURL:      "https://openai.test",
			APIKey:       "test-key",
			Organization: "org-123",
			Project:      "proj-abc",
		},
	})

	_ = plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{})
	headers := doer.lastReq.Header
	if headers.Get("Authorization") != "Bearer test-key" {
		t.Fatalf("authorization = %q", headers.Get("Authorization"))
	}
	if headers.Get("OpenAI-Organization") != "org-123" {
		t.Fatalf("org header = %q", headers.Get("OpenAI-Organization"))
	}
	if headers.Get("OpenAI-Project") != "proj-abc" {
		t.Fatalf("project header = %q", headers.Get("OpenAI-Project"))
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"":                              DefaultBaseURL,
		"api.openai.com/v1":             "https://api.openai.com/v1",
		"https://proxy.example.com/v1/": "https://proxy.example.com/v1",
		"http://localhost:8080":         "http://localhost:8080",
	}
	for input, want := range cases {
		if got := normalizeBaseURL(input); got != want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func newTestPlugin(doer HTTPDoer, apiKey string) *pluginbinding.Plugin {
	return NewPluginWithService(Service{
		Client: Client{
			HTTPClient: doer,
			BaseURL:    "https://openai.test",
			APIKey:     apiKey,
		},
	})
}

type stubDoer struct{}

func (stubDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

type errorDoer struct {
	status int
	body   string
}

func (d *errorDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: d.status,
		Status:     http.StatusText(d.status),
		Body:       io.NopCloser(strings.NewReader(d.body)),
	}, nil
}

type routedDoer struct {
	t        *testing.T
	routes   map[string]string
	lastReq  *http.Request
	lastJSON map[string]any
}

func newRoutedDoer(t *testing.T, routes map[string]string) *routedDoer {
	return &routedDoer{t: t, routes: routes}
}

func (d *routedDoer) Do(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Path
	body, ok := d.routes[key]
	if !ok {
		d.t.Fatalf("unexpected request: %s", key)
	}
	d.lastReq = req
	d.lastJSON = nil
	if req.Body != nil {
		raw, err := io.ReadAll(req.Body)
		if err == nil && len(raw) > 0 {
			_ = json.Unmarshal(raw, &d.lastJSON)
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (d *routedDoer) lastBody() map[string]any {
	return d.lastJSON
}
