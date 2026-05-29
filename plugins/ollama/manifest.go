package ollama

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "ollama"
	PluginVersion     = "0.13.2"
	PluginDescription = "Ollama local LLM operations: inspect installed models, generate completions, chat, and embed."

	OperationInfo      = "ollama.info"
	OperationModelList = "ollama.model.list"
	OperationModelShow = "ollama.model.show"
	OperationPs        = "ollama.ps"
	OperationGenerate  = "ollama.generate"
	OperationChat      = "ollama.chat"
	OperationEmbed     = "ollama.embed"

	DatasourceModels = "ollama.models"
	EntityModel      = "ollama.model"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"ol", PluginName},
		Operations: []core.OperationSpec{
			infoSpec(),
			modelListSpec(),
			modelShowSpec(),
			psSpec(),
			generateSpec(),
			chatSpec(),
			embedSpec(),
		},
		Datasources: []core.DatasourceSpec{
			modelDatasourceSpec(),
		},
	}
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InfoInput, Version](
		OperationInfo,
		"Show the Ollama server version.",
		ollamaReadOptions()...,
	)
}

func modelListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ModelListInput, pluginbinding.ListResult[Model]](
		OperationModelList,
		"List local Ollama models.",
		ollamaCompactReadOptions()...,
	)
}

func modelShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ModelShowInput, ModelInfo](
		OperationModelShow,
		"Show details for one Ollama model.",
		ollamaReadOptions()...,
	)
}

func psSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PsInput, pluginbinding.ListResult[RunningModel]](
		OperationPs,
		"List Ollama models currently loaded in memory.",
		ollamaCompactReadOptions()...,
	)
}

func generateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[GenerateInput, GenerateResult](
		OperationGenerate,
		"Generate a completion from a prompt using a local Ollama model.",
		ollamaGenerationOptions()...,
	)
}

func chatSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ChatInput, ChatResult](
		OperationChat,
		"Chat with a local Ollama model using a message history.",
		ollamaGenerationOptions()...,
	)
}

func embedSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[EmbedInput, EmbedResult](
		OperationEmbed,
		"Compute embeddings for one or more inputs with a local Ollama model.",
		ollamaGenerationOptions()...,
	)
}

func modelDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[ModelRecord]](
		DatasourceModels,
		EntityModel,
		"Locally installed Ollama models.",
		datasourceCapabilities(),
		pluginbinding.EntitySchemaFor[ModelRecord](),
		pluginbinding.View("compact", "Model summary.", "title", "model_name", "family", "parameter_size", "quantization", "size_bytes"),
		pluginbinding.Completion("Model names, families, and digests.", "model_name", "family", "digest"),
	)
}

func ollamaReadOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
}

func ollamaCompactReadOptions() []pluginbinding.OperationSpecOption {
	options := ollamaReadOptions()
	return append(options, pluginbinding.Compact())
}

func ollamaGenerationOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	}
}

func datasourceCapabilities() []string {
	return []string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup, pluginbinding.CapabilityGet}
}
