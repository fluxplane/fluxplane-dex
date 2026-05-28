package openai

import (
	"context"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

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
