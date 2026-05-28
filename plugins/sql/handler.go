package sql

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
		pluginbinding.RegisterOperation(querySpec(), service.Query),
		pluginbinding.RegisterDatasourceSearch(queryRowsDatasourceSpec(), service.QueryRows),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
