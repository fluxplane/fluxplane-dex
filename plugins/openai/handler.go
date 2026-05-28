package openai

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.WithSecretGetter(service.SecretGetter),
		pluginbinding.RegisterOperation(imageGenerateSpec(), service.ImageGenerate),
		pluginbinding.RegisterOperation(visionAnalyzeSpec(), service.VisionAnalyze),
		pluginbinding.RegisterOperation(modelListSpec(), service.ModelList),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
