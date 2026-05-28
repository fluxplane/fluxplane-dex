package ollama

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(infoSpec(), service.Info),
		pluginbinding.RegisterOperation(modelListSpec(), service.ModelList),
		pluginbinding.RegisterOperation(modelShowSpec(), service.ModelShow),
		pluginbinding.RegisterOperation(psSpec(), service.Ps),
		pluginbinding.RegisterOperation(generateSpec(), service.Generate),
		pluginbinding.RegisterOperation(chatSpec(), service.Chat),
		pluginbinding.RegisterOperation(embedSpec(), service.Embed),
		pluginbinding.RegisterDatasourceSearch(modelDatasourceSpec(), service.ModelSearch),
		pluginbinding.RegisterDatasourceLookup(modelDatasourceSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceGet(modelDatasourceSpec(), service.ModelGet),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
