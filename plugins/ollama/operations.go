package ollama

import (
	"context"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	Client Client
}

func NewService() Service {
	return Service{Client: NewClient()}
}

type ModelSearchResult = pluginbinding.DatasourceSearchResult[ModelRecord]
type ModelGetResult = pluginbinding.DatasourceGetResult[ModelRecord]
type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]
type GetInput = pluginbinding.DatasourceGetInput

func (s Service) Info(_ pluginbinding.Context, _ InfoInput) (Version, error) {
	var out Version
	if err := s.Client.get(context.Background(), "/api/version", &out); err != nil {
		return Version{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	return out, nil
}

func (s Service) ModelList(_ pluginbinding.Context, _ ModelListInput) (pluginbinding.ListResult[Model], error) {
	models, err := s.fetchModels(context.Background())
	if err != nil {
		return pluginbinding.ListResult[Model]{}, err
	}
	return pluginbinding.NewListResult(models), nil
}

func (s Service) ModelShow(_ pluginbinding.Context, input ModelShowInput) (ModelInfo, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ModelInfo{}, pluginbinding.Fail("bad_input", "model name is required")
	}
	body := map[string]any{"name": name}
	if input.Verbose {
		body["verbose"] = true
	}
	var out ModelInfo
	if err := s.Client.post(context.Background(), "/api/show", body, &out); err != nil {
		return ModelInfo{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	if out.Name == "" {
		out.Name = name
	}
	return out, nil
}

func (s Service) Ps(_ pluginbinding.Context, _ PsInput) (pluginbinding.ListResult[RunningModel], error) {
	var resp struct {
		Models []RunningModel `json:"models"`
	}
	if err := s.Client.get(context.Background(), "/api/ps", &resp); err != nil {
		return pluginbinding.ListResult[RunningModel]{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	return pluginbinding.NewListResult(resp.Models), nil
}

func (s Service) Generate(_ pluginbinding.Context, input GenerateInput) (GenerateResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return GenerateResult{}, pluginbinding.Fail("bad_input", "model is required")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return GenerateResult{}, pluginbinding.Fail("bad_input", "prompt is required")
	}
	body := map[string]any{
		"model":  model,
		"prompt": input.Prompt,
		"stream": false,
	}
	if input.System != "" {
		body["system"] = input.System
	}
	if input.Template != "" {
		body["template"] = input.Template
	}
	if input.Format != "" {
		body["format"] = input.Format
	}
	if input.Suffix != "" {
		body["suffix"] = input.Suffix
	}
	if input.Raw {
		body["raw"] = true
	}
	if input.KeepAlive != "" {
		body["keep_alive"] = input.KeepAlive
	}
	if input.Options != nil {
		body["options"] = input.Options
	}
	var out GenerateResult
	if err := s.Client.post(context.Background(), "/api/generate", body, &out); err != nil {
		return GenerateResult{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	return out, nil
}

func (s Service) Chat(_ pluginbinding.Context, input ChatInput) (ChatResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return ChatResult{}, pluginbinding.Fail("bad_input", "model is required")
	}
	if len(input.Messages) == 0 {
		return ChatResult{}, pluginbinding.Fail("bad_input", "messages must not be empty")
	}
	body := map[string]any{
		"model":    model,
		"messages": input.Messages,
		"stream":   false,
	}
	if input.Format != "" {
		body["format"] = input.Format
	}
	if input.KeepAlive != "" {
		body["keep_alive"] = input.KeepAlive
	}
	if input.Options != nil {
		body["options"] = input.Options
	}
	var out ChatResult
	if err := s.Client.post(context.Background(), "/api/chat", body, &out); err != nil {
		return ChatResult{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	return out, nil
}

func (s Service) Embed(_ pluginbinding.Context, input EmbedInput) (EmbedResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return EmbedResult{}, pluginbinding.Fail("bad_input", "model is required")
	}
	if len(input.Input) == 0 {
		return EmbedResult{}, pluginbinding.Fail("bad_input", "input must contain at least one entry")
	}
	body := map[string]any{
		"model": model,
		"input": input.Input,
	}
	if input.Truncate != nil {
		body["truncate"] = *input.Truncate
	}
	if input.KeepAlive != "" {
		body["keep_alive"] = input.KeepAlive
	}
	if input.Options != nil {
		body["options"] = input.Options
	}
	var out EmbedResult
	if err := s.Client.post(context.Background(), "/api/embed", body, &out); err != nil {
		return EmbedResult{}, pluginbinding.Errorf("ollama", "%s", err)
	}
	return out, nil
}

func (s Service) ModelSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (ModelSearchResult, error) {
	records, err := s.modelRecords(ctx)
	if err != nil {
		return ModelSearchResult{}, err
	}
	records = filterModelRecords(records, input.Query)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, limitSlice(records, searchLimit(input.Limit))), nil
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	records, err := s.modelRecords(ctx)
	if err != nil {
		return LookupResult{}, err
	}
	candidates := make([]pluginbinding.LookupCandidate, 0, len(records))
	for _, record := range records {
		candidates = append(candidates, pluginbinding.NewLookupCandidate(
			ctx.LookupSource(PluginName, DatasourceModels),
			record.Entity,
			record.ID,
			record,
			modelLookupValues(record),
		))
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func (s Service) ModelGet(ctx pluginbinding.Context, input GetInput) (ModelGetResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ModelGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	records, err := s.modelRecords(ctx)
	if err != nil {
		return ModelGetResult{}, err
	}
	for _, record := range records {
		if record.ID == id || record.ModelName == id {
			return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
		}
	}
	return ModelGetResult{}, pluginbinding.Fail("not_found", "model "+id+" not found")
}

func (s Service) modelRecords(ctx pluginbinding.Context) ([]ModelRecord, error) {
	models, err := s.fetchModels(context.Background())
	if err != nil {
		return nil, err
	}
	source := ctx.DatasourceSource()
	records := make([]ModelRecord, 0, len(models))
	for _, model := range models {
		if record, ok := normalizeModelRecord(source, model); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s Service) fetchModels(ctx context.Context) ([]Model, error) {
	var resp struct {
		Models []Model `json:"models"`
	}
	if err := s.Client.get(ctx, "/api/tags", &resp); err != nil {
		return nil, pluginbinding.Errorf("ollama", "%s", err)
	}
	return resp.Models, nil
}

func filterModelRecords(records []ModelRecord, query string) []ModelRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	out := make([]ModelRecord, 0, len(records))
	for _, record := range records {
		blob := strings.ToLower(strings.Join([]string{
			record.ModelName,
			record.Family,
			record.ParameterSize,
			record.Quantization,
			record.Digest,
		}, " "))
		if strings.Contains(blob, query) {
			out = append(out, record)
		}
	}
	return out
}

func searchLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

func limitSlice[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
