package kubernetes

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	plugin := pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(clusterListSpec(), service.ClusterList),
		pluginbinding.RegisterOperation(clusterTestSpec(), service.ClusterTest),
		pluginbinding.RegisterOperation(endpointDiscoverSpec(), service.EndpointDiscover),
		pluginbinding.RegisterOperation(namespaceListSpec(), service.NamespaceList),
		pluginbinding.RegisterOperation(serviceListSpec(), service.ServiceList),
		pluginbinding.RegisterOperation(serviceShowSpec(), service.ServiceShow),
		pluginbinding.RegisterOperation(podListSpec(), service.PodList),
		pluginbinding.RegisterOperation(podShowSpec(), service.PodShow),
		pluginbinding.RegisterDatasourceSearch(inventoryDatasourceSpec(), service.InventorySearch),
	)
	plugin.Command(protocol.CommandEndpointsDiscover, service.DiscoverEndpointsCommand)
	return plugin
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
