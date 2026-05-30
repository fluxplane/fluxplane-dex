package openai

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
)

const (
	PluginName        = "openai"
	PluginVersion     = "0.18.2"
	PluginDescription = "OpenAI API plugin. Exposes image generation, image understanding, and model listing."

	AuthMethodAPIKey        = "api_key"
	AuthPurposeAPIKey       = "api_key"
	AuthPurposeOrganization = "organization"
	AuthPurposeProject      = "project"
	EnvOpenAIAPIKey         = "OPENAI_API_KEY"
	EnvOpenAIOrganization   = "OPENAI_ORGANIZATION"
	EnvOpenAIProject        = "OPENAI_PROJECT"

	OperationImageGenerate = "openai.image.generate"
	OperationVisionAnalyze = "openai.vision.analyze"
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
			visionAnalyzeSpec(),
			modelListSpec(),
		},
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodAPIKey,
			"OpenAI API key resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeAPIKey, "OpenAI API key", true, true, EnvOpenAIAPIKey),
			pluginbinding.AuthField(AuthPurposeOrganization, "OpenAI organization header", false, false, EnvOpenAIOrganization),
			pluginbinding.AuthField(AuthPurposeProject, "OpenAI project header", false, false, EnvOpenAIProject),
		)},
		Metadata: vision.ProviderMetadata(visionProviderSpec()),
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
		pluginbinding.SecretPurposes(AuthPurposeAPIKey, AuthPurposeOrganization, AuthPurposeProject),
	)
}

func visionProviderSpec() vision.ProviderSpec {
	return vision.ProviderSpec{
		Name:                 PluginName,
		Version:              PluginVersion,
		Description:          PluginDescription,
		Aliases:              []string{"oai", PluginName},
		Operation:            OperationVisionAnalyze,
		OperationDescription: "Analyze one or more images using OpenAI vision-capable models.",
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodAPIKey,
			"OpenAI API key resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeAPIKey, "OpenAI API key", true, true, EnvOpenAIAPIKey),
			pluginbinding.AuthField(AuthPurposeOrganization, "OpenAI organization header", false, false, EnvOpenAIOrganization),
			pluginbinding.AuthField(AuthPurposeProject, "OpenAI project header", false, false, EnvOpenAIProject),
		)},
		SecretPurposes: []string{AuthPurposeAPIKey, AuthPurposeOrganization, AuthPurposeProject},
	}
}

func visionAnalyzeSpec() core.OperationSpec {
	return vision.ProviderOperationSpec(visionProviderSpec())
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
		pluginbinding.SecretPurposes(AuthPurposeAPIKey, AuthPurposeOrganization, AuthPurposeProject),
		pluginbinding.Compact(),
	)
}
