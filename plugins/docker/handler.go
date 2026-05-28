package docker

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
		pluginbinding.RegisterOperation(containerListSpec(), service.ContainerList),
		pluginbinding.RegisterOperation(containerShowSpec(), service.ContainerShow),
		pluginbinding.RegisterOperation(containerLogsSpec(), service.ContainerLogs),
		pluginbinding.RegisterOperation(containerStatsSpec(), service.ContainerStats),
		pluginbinding.RegisterOperation(containerTopSpec(), service.ContainerTop),
		pluginbinding.RegisterOperation(containerExecSpec(), service.ContainerExec),
		pluginbinding.RegisterOperation(containerCopyFromSpec(), service.ContainerCopyFrom),
		pluginbinding.RegisterOperation(containerCopyToSpec(), service.ContainerCopyTo),
		pluginbinding.RegisterOperation(containerCreateSpec(), service.ContainerCreate),
		pluginbinding.RegisterOperation(containerRunSpec(), service.ContainerRun),
		pluginbinding.RegisterOperation(containerStartSpec(), service.ContainerStart),
		pluginbinding.RegisterOperation(containerStopSpec(), service.ContainerStop),
		pluginbinding.RegisterOperation(containerRestartSpec(), service.ContainerRestart),
		pluginbinding.RegisterOperation(containerRemoveSpec(), service.ContainerRemove),
		pluginbinding.RegisterOperation(containerInspectRawSpec(), service.ContainerInspectRaw),
		pluginbinding.RegisterOperation(containerPruneSpec(), service.ContainerPrune),
		pluginbinding.RegisterOperation(imageListSpec(), service.ImageList),
		pluginbinding.RegisterOperation(imageShowSpec(), service.ImageShow),
		pluginbinding.RegisterOperation(imagePullSpec(), service.ImagePull),
		pluginbinding.RegisterOperation(imageTagSpec(), service.ImageTag),
		pluginbinding.RegisterOperation(imagePushSpec(), service.ImagePush),
		pluginbinding.RegisterOperation(imageBuildSpec(), service.ImageBuild),
		pluginbinding.RegisterOperation(imageRemoveSpec(), service.ImageRemove),
		pluginbinding.RegisterOperation(imageInspectRawSpec(), service.ImageInspectRaw),
		pluginbinding.RegisterOperation(imagePruneSpec(), service.ImagePrune),
		pluginbinding.RegisterOperation(networkListSpec(), service.NetworkList),
		pluginbinding.RegisterOperation(networkShowSpec(), service.NetworkShow),
		pluginbinding.RegisterOperation(networkCreateSpec(), service.NetworkCreate),
		pluginbinding.RegisterOperation(networkRemoveSpec(), service.NetworkRemove),
		pluginbinding.RegisterOperation(networkInspectRawSpec(), service.NetworkInspectRaw),
		pluginbinding.RegisterOperation(networkPruneSpec(), service.NetworkPrune),
		pluginbinding.RegisterOperation(systemDFSpec(), service.SystemDF),
		pluginbinding.RegisterOperation(systemPruneSpec(), service.SystemPrune),
		pluginbinding.RegisterOperation(eventsSpec(), service.Events),
		pluginbinding.RegisterOperation(volumeListSpec(), service.VolumeList),
		pluginbinding.RegisterOperation(volumeShowSpec(), service.VolumeShow),
		pluginbinding.RegisterOperation(volumeCreateSpec(), service.VolumeCreate),
		pluginbinding.RegisterOperation(volumeRemoveSpec(), service.VolumeRemove),
		pluginbinding.RegisterOperation(volumeInspectRawSpec(), service.VolumeInspectRaw),
		pluginbinding.RegisterOperation(volumePruneSpec(), service.VolumePrune),
		pluginbinding.RegisterOperation(buildCachePruneSpec(), service.BuildCachePrune),
		pluginbinding.RegisterOperation(contextListSpec(), service.ContextList),
		pluginbinding.RegisterOperation(contextShowSpec(), service.ContextShow),
		pluginbinding.RegisterDatasourceSearch(containerDatasourceSpec(), service.ContainerSearch),
		pluginbinding.RegisterDatasourceLookup(containerDatasourceSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceGet(containerDatasourceSpec(), service.ContainerGet),
		pluginbinding.RegisterDatasourceSearch(imageDatasourceSpec(), service.ImageSearch),
		pluginbinding.RegisterDatasourceLookup(imageDatasourceSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceGet(imageDatasourceSpec(), service.ImageGet),
		pluginbinding.RegisterDatasourceSearch(networkDatasourceSpec(), service.NetworkSearch),
		pluginbinding.RegisterDatasourceLookup(networkDatasourceSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceGet(networkDatasourceSpec(), service.NetworkGet),
		pluginbinding.RegisterDatasourceSearch(volumeDatasourceSpec(), service.VolumeSearch),
		pluginbinding.RegisterDatasourceLookup(volumeDatasourceSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceGet(volumeDatasourceSpec(), service.VolumeGet),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
