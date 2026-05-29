package tavily

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return websearch.DefineProvider(providerSpec(), service.Search)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
