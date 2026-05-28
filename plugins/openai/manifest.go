package openai

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "openai"
	PluginVersion     = "0.3.1"
	PluginDescription = "OpenAI API plugin. Currently exposes image generation and model listing."

	AuthMethodAPIKey  = "api_key"
	AuthPurposeAPIKey = "api_key"
	EnvOpenAIAPIKey   = "OPENAI_API_KEY"

	OperationImageGenerate = "openai.image.generate"
	OperationModelList     = "openai.model.list"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"oai", PluginName},
		Operations: []core.OperationSpec{
			imageGenerateSpec(),
			modelListSpec(),
		},
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodAPIKey,
			"OpenAI API key resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeAPIKey, "OpenAI API key", true, true, EnvOpenAIAPIKey),
		)},
	}
}

func imageGenerateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImageGenerateInput, ImageGenerateResult](
		OperationImageGenerate,
		"Generate one or more images from a text prompt using OpenAI image models.",
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
		pluginbinding.SecretPurposes(AuthPurposeAPIKey),
	)
}

func modelListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ModelListInput, pluginbinding.ListResult[Model]](
		OperationModelList,
		"List available OpenAI models for the caller's API key.",
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
		pluginbinding.SecretPurposes(AuthPurposeAPIKey),
		pluginbinding.Compact(),
	)
}
