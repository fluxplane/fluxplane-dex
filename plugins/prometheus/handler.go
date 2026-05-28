package prometheus

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(testSpec(), service.Test),
		pluginbinding.RegisterOperation(querySpec(), service.Query),
		pluginbinding.RegisterOperation(queryRangeSpec(), service.QueryRange),
		pluginbinding.RegisterOperation(labelsSpec(), service.Labels),
		pluginbinding.RegisterOperation(targetsSpec(), service.Targets),
		pluginbinding.RegisterOperation(alertsSpec(), service.Alerts),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
