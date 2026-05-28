package ollama

import (
	"encoding/json"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Version struct {
	Version string `json:"version,omitempty"`
}

type ModelDetails struct {
	Format            string   `json:"format,omitempty"`
	Family            string   `json:"family,omitempty"`
	Families          []string `json:"families,omitempty"`
	ParameterSize     string   `json:"parameter_size,omitempty"`
	QuantizationLevel string   `json:"quantization_level,omitempty"`
	ParentModel       string   `json:"parent_model,omitempty"`
}

type Model struct {
	Name       string       `json:"name"`
	Model      string       `json:"model,omitempty"`
	ModifiedAt string       `json:"modified_at,omitempty"`
	Size       int64        `json:"size,omitempty"`
	Digest     string       `json:"digest,omitempty"`
	Details    ModelDetails `json:"details,omitempty"`
}

type RunningModel struct {
	Name      string       `json:"name"`
	Model     string       `json:"model,omitempty"`
	Size      int64        `json:"size,omitempty"`
	SizeVRAM  int64        `json:"size_vram,omitempty"`
	Digest    string       `json:"digest,omitempty"`
	ExpiresAt string       `json:"expires_at,omitempty"`
	Details   ModelDetails `json:"details,omitempty"`
}

type ModelInfo struct {
	Name         string         `json:"name,omitempty"`
	Modelfile    string         `json:"modelfile,omitempty"`
	Parameters   string         `json:"parameters,omitempty"`
	Template     string         `json:"template,omitempty"`
	System       string         `json:"system,omitempty"`
	License      string         `json:"license,omitempty"`
	Details      ModelDetails   `json:"details,omitempty"`
	ModelInfo    map[string]any `json:"model_info,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
}

type InfoInput struct{}

type ModelListInput struct{}

type ModelShowInput struct {
	Name    string `json:"name,omitempty" jsonschema:"required,description=Model name (e.g. llama3:8b)."`
	Verbose bool   `json:"verbose,omitempty" jsonschema:"description=Include verbose model info."`
}

type PsInput struct{}

type GenerateInput struct {
	Model     string             `json:"model,omitempty" jsonschema:"required,description=Model name (e.g. llama3:8b)."`
	Prompt    string             `json:"prompt,omitempty" jsonschema:"required,description=User prompt."`
	System    string             `json:"system,omitempty" jsonschema:"description=Optional system instruction."`
	Template  string             `json:"template,omitempty" jsonschema:"description=Override the prompt template."`
	Format    string             `json:"format,omitempty" jsonschema:"description=Response format. Use 'json' to force a JSON object."`
	Suffix    string             `json:"suffix,omitempty" jsonschema:"description=Text appended after the response (for fill-in-the-middle models)."`
	Raw       bool               `json:"raw,omitempty" jsonschema:"description=Skip prompt formatting and send the prompt verbatim."`
	KeepAlive string             `json:"keep_alive,omitempty" jsonschema:"description=How long to keep the model loaded (e.g. '5m', '0' to unload)."`
	Options   *GenerationOptions `json:"options,omitempty" jsonschema:"description=Model sampling and loading options."`
}

type GenerateResult struct {
	Model                string `json:"model"`
	CreatedAt            string `json:"created_at,omitempty"`
	Response             string `json:"response"`
	Done                 bool   `json:"done"`
	DoneReason           string `json:"done_reason,omitempty"`
	TotalDurationNs      int64  `json:"total_duration,omitempty"`
	LoadDurationNs       int64  `json:"load_duration,omitempty"`
	PromptEvalCount      int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDurationNs int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount            int    `json:"eval_count,omitempty"`
	EvalDurationNs       int64  `json:"eval_duration,omitempty"`
}

type ChatMessage struct {
	Role    string   `json:"role" jsonschema:"required,description=Message role: system, user, assistant, or tool."`
	Content string   `json:"content" jsonschema:"required,description=Message content."`
	Images  []string `json:"images,omitempty" jsonschema:"description=Base64-encoded images for multimodal models."`
}

type ChatInput struct {
	Model     string             `json:"model,omitempty" jsonschema:"required,description=Model name."`
	Messages  []ChatMessage      `json:"messages,omitempty" jsonschema:"required,description=Chat message history."`
	Format    string             `json:"format,omitempty" jsonschema:"description=Response format. Use 'json' to force a JSON object."`
	KeepAlive string             `json:"keep_alive,omitempty" jsonschema:"description=How long to keep the model loaded."`
	Options   *GenerationOptions `json:"options,omitempty" jsonschema:"description=Model sampling and loading options."`
}

type ChatResult struct {
	Model                string      `json:"model"`
	CreatedAt            string      `json:"created_at,omitempty"`
	Message              ChatMessage `json:"message"`
	Done                 bool        `json:"done"`
	DoneReason           string      `json:"done_reason,omitempty"`
	TotalDurationNs      int64       `json:"total_duration,omitempty"`
	LoadDurationNs       int64       `json:"load_duration,omitempty"`
	PromptEvalCount      int         `json:"prompt_eval_count,omitempty"`
	PromptEvalDurationNs int64       `json:"prompt_eval_duration,omitempty"`
	EvalCount            int         `json:"eval_count,omitempty"`
	EvalDurationNs       int64       `json:"eval_duration,omitempty"`
}

type EmbedInput struct {
	Model     string             `json:"model,omitempty" jsonschema:"required,description=Embedding model name."`
	Input     []string           `json:"input,omitempty" jsonschema:"required,description=Input text(s) to embed."`
	Truncate  *bool              `json:"truncate,omitempty" jsonschema:"description=Truncate inputs to the model's context length. Default true."`
	KeepAlive string             `json:"keep_alive,omitempty" jsonschema:"description=How long to keep the model loaded."`
	Options   *GenerationOptions `json:"options,omitempty" jsonschema:"description=Model loading options."`
}

type EmbedResult struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	TotalDurationNs int64       `json:"total_duration,omitempty"`
	LoadDurationNs  int64       `json:"load_duration,omitempty"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}

type ModelRecord struct {
	pluginbinding.DatasourceRecord
	Title         string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ModelName     string `json:"model_name" datasource:"id,completion,view=compact|lookup|table"`
	Family        string `json:"family,omitempty" datasource:"completion,view=compact|lookup|table"`
	ParameterSize string `json:"parameter_size,omitempty" datasource:"view=compact|lookup|table"`
	Quantization  string `json:"quantization,omitempty" datasource:"view=compact|lookup|table"`
	SizeBytes     int64  `json:"size_bytes,omitempty" datasource:"view=compact|table"`
	Digest        string `json:"digest,omitempty" datasource:"completion"`
	ModifiedAt    string `json:"modified_at,omitempty" datasource:"view=table"`
}

func normalizeModelRecord(source pluginbinding.DatasourceSource, model Model) (ModelRecord, bool) {
	name := strings.TrimSpace(model.Name)
	if name == "" {
		name = strings.TrimSpace(model.Model)
	}
	if name == "" {
		return ModelRecord{}, false
	}
	return ModelRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityModel, name,
			pluginbinding.RecordTitle(name),
			pluginbinding.RecordLink("self", "ollama://model/"+name),
		),
		Title:         name,
		ModelName:     name,
		Family:        model.Details.Family,
		ParameterSize: model.Details.ParameterSize,
		Quantization:  model.Details.QuantizationLevel,
		SizeBytes:     model.Size,
		Digest:        model.Digest,
		ModifiedAt:    model.ModifiedAt,
	}, true
}

// GenerationOptions holds the most common Ollama sampling and loading options.
// Less-common knobs can be passed verbatim through Extra; typed fields take precedence
// when a key appears in both.
type GenerationOptions struct {
	Temperature      *float64       `json:"temperature,omitempty" jsonschema:"description=Sampling temperature. 0 = deterministic."`
	TopK             int            `json:"top_k,omitempty" jsonschema:"description=Top-K sampling. 0 disables."`
	TopP             float64        `json:"top_p,omitempty" jsonschema:"description=Top-P (nucleus) sampling."`
	MinP             float64        `json:"min_p,omitempty" jsonschema:"description=Min-P sampling."`
	Seed             int            `json:"seed,omitempty" jsonschema:"description=Random seed."`
	NumPredict       int            `json:"num_predict,omitempty" jsonschema:"description=Max tokens to predict. -1 = infinite, -2 = fill context."`
	NumCtx           int            `json:"num_ctx,omitempty" jsonschema:"description=Context window size in tokens."`
	RepeatLastN      int            `json:"repeat_last_n,omitempty" jsonschema:"description=Window of recent tokens to apply repeat penalty over."`
	RepeatPenalty    float64        `json:"repeat_penalty,omitempty" jsonschema:"description=Repetition penalty."`
	PresencePenalty  float64        `json:"presence_penalty,omitempty" jsonschema:"description=Presence penalty."`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty" jsonschema:"description=Frequency penalty."`
	Stop             []string       `json:"stop,omitempty" jsonschema:"description=Stop sequences."`
	Mirostat         *int           `json:"mirostat,omitempty" jsonschema:"description=Mirostat sampling: 0 (off), 1, or 2."`
	MirostatTau      float64        `json:"mirostat_tau,omitempty" jsonschema:"description=Mirostat target entropy."`
	MirostatEta      float64        `json:"mirostat_eta,omitempty" jsonschema:"description=Mirostat learning rate."`
	NumGPU           int            `json:"num_gpu,omitempty" jsonschema:"description=Number of model layers to offload to GPU."`
	NumThread        int            `json:"num_thread,omitempty" jsonschema:"description=Number of CPU threads."`
	Extra            map[string]any `json:"-" jsonschema:"-"`
}

func (o GenerationOptions) MarshalJSON() ([]byte, error) {
	type alias GenerationOptions
	raw, err := json.Marshal(alias(o))
	if err != nil {
		return nil, err
	}
	if len(o.Extra) == 0 {
		return raw, nil
	}
	var merged map[string]any
	if err := json.Unmarshal(raw, &merged); err != nil {
		return nil, err
	}
	if merged == nil {
		merged = map[string]any{}
	}
	for key, value := range o.Extra {
		if _, present := merged[key]; present {
			continue
		}
		merged[key] = value
	}
	return json.Marshal(merged)
}

func modelLookupValues(record ModelRecord) map[string]string {
	return map[string]string{
		"id":                    record.ID,
		"title":                 record.Title,
		"links.self":            record.Links["self"],
		"record.model_name":     record.ModelName,
		"record.family":         record.Family,
		"record.parameter_size": record.ParameterSize,
		"record.quantization":   record.Quantization,
		"record.digest":         record.Digest,
	}
}
