package ollama

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestInfoReturnsVersion(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /api/version": `{"version":"0.5.7"}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.RunOK[Version](t, plugin, OperationInfo, InfoInput{})
	if out.Version != "0.5.7" {
		t.Fatalf("version = %q", out.Version)
	}
}

func TestModelListParsesTags(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","size":4920753920,"digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}},
			{"name":"mistral:7b","size":4109016704,"digest":"def","details":{"family":"mistral","parameter_size":"7B","quantization_level":"Q4_K_M"}}
		]}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.RunOK[pluginbinding.ListResult[Model]](t, plugin, OperationModelList, ModelListInput{})
	if len(out.Items) != 2 || out.Items[0].Name != "llama3:8b" || out.Items[1].Details.Family != "mistral" {
		t.Fatalf("list = %#v", out)
	}
}

func TestModelShowRequiresName(t *testing.T) {
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: stubDoer{}, Endpoint: "http://ollama.test"}})
	err := plugintest.RunError(t, plugin, OperationModelShow, ModelShowInput{})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestGenerateSendsStreamFalseAndParsesResponse(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /api/generate": `{"model":"llama3:8b","response":"hello world","done":true,"eval_count":7}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	temp := 0.2
	out := plugintest.RunOK[GenerateResult](t, plugin, OperationGenerate, GenerateInput{
		Model:   "llama3:8b",
		Prompt:  "say hi",
		System:  "be terse",
		Format:  "json",
		Options: &GenerationOptions{Temperature: &temp, TopK: 40, Stop: []string{"###"}},
	})
	if out.Response != "hello world" || !out.Done || out.EvalCount != 7 {
		t.Fatalf("result = %#v", out)
	}
	body := doer.lastBody()
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
			"temperature":      999.0, // typed field should win
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
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: stubDoer{}, Endpoint: "http://ollama.test"}})
	err := plugintest.RunError(t, plugin, OperationGenerate, GenerateInput{Model: "llama3"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestChatSendsMessages(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /api/chat": `{"model":"llama3","message":{"role":"assistant","content":"hi back"},"done":true}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.RunOK[ChatResult](t, plugin, OperationChat, ChatInput{
		Model: "llama3",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if out.Message.Content != "hi back" || out.Message.Role != "assistant" {
		t.Fatalf("chat result = %#v", out)
	}
	body := doer.lastBody()
	msgs, ok := body["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages payload = %#v", body["messages"])
	}
}

func TestChatRejectsEmptyMessages(t *testing.T) {
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: stubDoer{}, Endpoint: "http://ollama.test"}})
	err := plugintest.RunError(t, plugin, OperationChat, ChatInput{Model: "llama3"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestEmbedParsesVectors(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"POST /api/embed": `{"model":"all-minilm","embeddings":[[0.1,0.2,0.3]]}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.RunOK[EmbedResult](t, plugin, OperationEmbed, EmbedInput{Model: "all-minilm", Input: []string{"hello"}})
	if len(out.Embeddings) != 1 || len(out.Embeddings[0]) != 3 || out.Embeddings[0][1] != 0.2 {
		t.Fatalf("embed result = %#v", out)
	}
}

func TestDatasourceLookupReturnsModel(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}}
		]}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.DatasourceLookupOK[pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]](
		t, plugin,
		pluginbinding.DatasourceLookupInput{Text: "llama3", Entity: EntityModel},
		plugintest.WithInstance("local"),
	)
	if out.Count != 1 || out.Matches[0].ID != "llama3:8b" {
		t.Fatalf("lookup = %#v", out)
	}
}

func TestEmbedRejectsEmptyInput(t *testing.T) {
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: stubDoer{}, Endpoint: "http://ollama.test"}})
	err := plugintest.RunError(t, plugin, OperationEmbed, EmbedInput{Model: "all-minilm"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelGetReturnsRecord(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama","parameter_size":"8B","quantization_level":"Q4_0"}}
		]}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[ModelRecord]](
		t, plugin,
		pluginbinding.DatasourceGetInput{ID: "llama3:8b", Entity: EntityModel},
	)
	if out.Record.ModelName != "llama3:8b" || out.Record.Family != "llama" {
		t.Fatalf("get = %#v", out)
	}
}

func TestModelGetNotFound(t *testing.T) {
	doer := newRoutedDoer(t, map[string]string{
		"GET /api/tags": `{"models":[
			{"name":"llama3:8b","digest":"abc","details":{"family":"llama"}}
		]}`,
	})
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	err := plugintest.DatasourceError(t, plugin, "datasources.get",
		pluginbinding.DatasourceGetInput{ID: "missing:1b", Entity: EntityModel},
	)
	if err == nil || err.Code != "not_found" {
		t.Fatalf("err = %#v", err)
	}
}

func TestModelShowSurfacesServerError(t *testing.T) {
	doer := &errorDoer{
		status: http.StatusNotFound,
		body:   `{"error":"model 'ghost' not found"}`,
	}
	plugin := NewPluginWithService(Service{Client: Client{HTTPClient: doer, Endpoint: "http://ollama.test"}})

	err := plugintest.RunError(t, plugin, OperationModelShow, ModelShowInput{Name: "ghost"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Message, "model 'ghost' not found") {
		t.Fatalf("error message = %q; want it to include ollama error", err.Message)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	cases := map[string]string{
		"":                   DefaultEndpoint,
		"localhost:11434":    "http://localhost:11434",
		"https://gpu:11434/": "https://gpu:11434",
		"http://gpu:11434":   "http://gpu:11434",
	}
	for input, want := range cases {
		if got := normalizeEndpoint(input); got != want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
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
