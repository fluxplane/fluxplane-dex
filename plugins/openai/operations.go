package openai

import (
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
)

const defaultVisionModel = "gpt-4.1-mini"

type Service struct{}

func NewService() Service {
	return Service{}
}

func (s Service) ImageGenerate(ctx pluginbinding.Context, input ImageGenerateInput) (ImageGenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return ImageGenerateResult{}, pluginbinding.Fail("bad_input", "prompt is required")
	}
	if err := validateImageGenerateInput(input); err != nil {
		return ImageGenerateResult{}, err
	}
	client, err := s.client(ctx, input.OpenAITargetInput)
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
	if err := client.post("/images/generations", body, &out); err != nil {
		return ImageGenerateResult{}, pluginbinding.Errorf("openai", "%s", err)
	}
	return out, nil
}

func validateImageGenerateInput(input ImageGenerateInput) error {
	if input.N < 0 || input.N > 10 {
		return pluginbinding.Fail("bad_input", "n must be between 1 and 10 when set")
	}
	if input.OutputCompression < 0 || input.OutputCompression > 100 {
		return pluginbinding.Fail("bad_input", "output_compression must be between 0 and 100 when set")
	}
	if len([]rune(input.Prompt)) > 32000 {
		return pluginbinding.Fail("bad_input", "prompt must be at most 32000 characters")
	}
	return nil
}

func (s Service) VisionAnalyze(ctx pluginbinding.Context, input vision.AnalyzeInput) (vision.AnalyzeOutput, error) {
	if err := vision.ValidateImages(input.Images); err != nil {
		return vision.AnalyzeOutput{}, err
	}
	client, err := s.client(ctx, OpenAITargetInput{EndpointRef: input.EndpointRef})
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
	content, err := openAIVisionContent(ctx, input)
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
	if err := client.post("/responses", body, &out); err != nil {
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

func (s Service) ModelList(ctx pluginbinding.Context, input ModelListInput) (pluginbinding.ListResult[Model], error) {
	client, err := s.client(ctx, input.OpenAITargetInput)
	if err != nil {
		return pluginbinding.ListResult[Model]{}, err
	}
	var resp struct {
		Data []Model `json:"data"`
	}
	if err := client.get("/models", &resp); err != nil {
		return pluginbinding.ListResult[Model]{}, pluginbinding.Errorf("openai", "%s", err)
	}
	return pluginbinding.NewListResult(resp.Data), nil
}

func (s Service) client(ctx pluginbinding.Context, target OpenAITargetInput) (Client, error) {
	endpointRef := strings.TrimSpace(target.EndpointRef)
	if endpointRef == "" {
		return Client{}, pluginbinding.Fail("bad_input", "endpoint_ref is required")
	}
	return Client{EndpointRef: endpointRef, Host: ctx.Host}, nil
}

func openAIVisionContent(ctx pluginbinding.Context, input vision.AnalyzeInput) ([]map[string]any, error) {
	content := []map[string]any{{
		"type": "input_text",
		"text": vision.NormalizePrompt(input.Prompt),
	}}
	for _, image := range input.Images {
		imageURL, err := openAIVisionImageURL(ctx, image)
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

func openAIVisionImageURL(ctx pluginbinding.Context, image vision.ImageInput) (string, error) {
	if strings.TrimSpace(image.BlobRef) == "" {
		return vision.DataURL(image)
	}
	blob, err := ctx.Host.BlobRead(pluginbinding.BlobReadRequest{Ref: image.BlobRef})
	if err != nil {
		return "", err
	}
	filename := firstNonEmpty(image.Filename, blob.Blob.Filename, blob.Blob.Path)
	mediaType := firstNonEmpty(image.MediaType, blob.Blob.MediaType)
	return vision.DataURLFromBytes(blob.Content, mediaType, filename), nil
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
