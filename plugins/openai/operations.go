package openai

import (
	"context"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
)

const defaultVisionModel = "gpt-4.1-mini"

type Service struct {
	Client       Client
	SecretGetter pluginbinding.SecretGetter
}

func NewService() Service {
	return Service{Client: NewClient()}
}

func (s Service) ImageGenerate(ctx pluginbinding.Context, input ImageGenerateInput) (ImageGenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return ImageGenerateResult{}, pluginbinding.Fail("bad_input", "prompt is required")
	}
	client, err := s.authedClient(ctx)
	if err != nil {
		return ImageGenerateResult{}, err
	}
	body := map[string]any{"prompt": prompt}
	if input.Model != "" {
		body["model"] = input.Model
	}
	if input.N > 0 {
		body["n"] = input.N
	}
	if input.Size != "" {
		body["size"] = input.Size
	}
	if input.Quality != "" {
		body["quality"] = input.Quality
	}
	if input.Style != "" {
		body["style"] = input.Style
	}
	if input.ResponseFormat != "" {
		body["response_format"] = input.ResponseFormat
	}
	if input.OutputFormat != "" {
		body["output_format"] = input.OutputFormat
	}
	if input.Background != "" {
		body["background"] = input.Background
	}
	if input.Moderation != "" {
		body["moderation"] = input.Moderation
	}
	if input.OutputCompression > 0 {
		body["output_compression"] = input.OutputCompression
	}
	if input.User != "" {
		body["user"] = input.User
	}
	var out ImageGenerateResult
	if err := client.post(context.Background(), "/images/generations", body, &out); err != nil {
		return ImageGenerateResult{}, pluginbinding.Errorf("openai", "%s", err)
	}
	return out, nil
}

func (s Service) VisionAnalyze(ctx pluginbinding.Context, input vision.AnalyzeInput) (vision.AnalyzeOutput, error) {
	if err := vision.ValidateImages(input.Images); err != nil {
		return vision.AnalyzeOutput{}, err
	}
	client, err := s.authedClient(ctx)
	if err != nil {
		return vision.AnalyzeOutput{}, err
	}
	body := map[string]any{
		"model": modelOrDefault(input.Model, defaultVisionModel),
		"input": []map[string]any{{
			"role":    "user",
			"content": nil,
		}},
	}
	content, err := openAIVisionContent(input)
	if err != nil {
		return vision.AnalyzeOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	body["input"].([]map[string]any)[0]["content"] = content
	if input.MaxTokens > 0 {
		body["max_output_tokens"] = input.MaxTokens
	}
	if input.Temperature != nil {
		body["temperature"] = *input.Temperature
	}
	var out responsesOutput
	if err := client.post(context.Background(), "/responses", body, &out); err != nil {
		return vision.AnalyzeOutput{}, pluginbinding.Errorf("openai", "%s", err)
	}
	text := responseOutputText(out)
	if strings.TrimSpace(text) == "" {
		return vision.AnalyzeOutput{}, pluginbinding.Fail("vision_failed", "openai returned no analysis text")
	}
	return vision.AnalyzeOutput{Results: []vision.AnalysisResult{{
		Provider: PluginName,
		Model:    firstNonEmpty(out.Model, modelOrDefault(input.Model, defaultVisionModel)),
		Text:     text,
		Usage:    out.Usage,
	}}}, nil
}

func (s Service) ModelList(ctx pluginbinding.Context, _ ModelListInput) (pluginbinding.ListResult[Model], error) {
	client, err := s.authedClient(ctx)
	if err != nil {
		return pluginbinding.ListResult[Model]{}, err
	}
	var resp struct {
		Data []Model `json:"data"`
	}
	if err := client.get(context.Background(), "/models", &resp); err != nil {
		return pluginbinding.ListResult[Model]{}, pluginbinding.Errorf("openai", "%s", err)
	}
	return pluginbinding.NewListResult(resp.Data), nil
}

func (s Service) authedClient(ctx pluginbinding.Context) (Client, error) {
	client := s.Client
	if strings.TrimSpace(client.APIKey) == "" {
		key, err := ctx.RequiredSecret(AuthPurposeAPIKey)
		if err != nil {
			return Client{}, err
		}
		client.APIKey = strings.TrimSpace(key.Value)
	}
	return client, nil
}

func openAIVisionContent(input vision.AnalyzeInput) ([]map[string]any, error) {
	content := []map[string]any{{
		"type": "input_text",
		"text": vision.NormalizePrompt(input.Prompt),
	}}
	for _, image := range input.Images {
		imageURL, err := vision.DataURL(image)
		if err != nil {
			return nil, err
		}
		item := map[string]any{
			"type":      "input_image",
			"image_url": imageURL,
		}
		if detail := strings.TrimSpace(image.Detail); detail != "" {
			item["detail"] = detail
		}
		content = append(content, item)
	}
	return content, nil
}

func responseOutputText(out responsesOutput) string {
	if strings.TrimSpace(out.OutputText) != "" {
		return strings.TrimSpace(out.OutputText)
	}
	var parts []string
	for _, message := range out.Output {
		for _, content := range message.Content {
			if content.Type == "output_text" || content.Type == "text" || content.Type == "" {
				if text := strings.TrimSpace(content.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

func modelOrDefault(model, fallback string) string {
	if strings.TrimSpace(model) != "" {
		return strings.TrimSpace(model)
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
