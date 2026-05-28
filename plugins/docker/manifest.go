package docker

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "docker"
	PluginVersion     = "0.1.0"
	PluginDescription = "Local Docker Engine inspection for containers, images, networks, volumes, and daemon info."

	OperationInfo                = "docker.info"
	OperationContainerList       = "docker.container.list"
	OperationContainerShow       = "docker.container.show"
	OperationContainerLogs       = "docker.container.logs"
	OperationContainerStats      = "docker.container.stats"
	OperationContainerTop        = "docker.container.top"
	OperationContainerExec       = "docker.container.exec"
	OperationContainerCopyFrom   = "docker.container.copy_from"
	OperationContainerCopyTo     = "docker.container.copy_to"
	OperationContainerCreate     = "docker.container.create"
	OperationContainerRun        = "docker.container.run"
	OperationContainerStart      = "docker.container.start"
	OperationContainerStop       = "docker.container.stop"
	OperationContainerRestart    = "docker.container.restart"
	OperationContainerRemove     = "docker.container.remove"
	OperationContainerInspectRaw = "docker.container.inspect.raw"
	OperationContainerPrune      = "docker.container.prune"
	OperationImageList           = "docker.image.list"
	OperationImageShow           = "docker.image.show"
	OperationImagePull           = "docker.image.pull"
	OperationImageTag            = "docker.image.tag"
	OperationImagePush           = "docker.image.push"
	OperationImageBuild          = "docker.image.build"
	OperationImageRemove         = "docker.image.remove"
	OperationImageInspectRaw     = "docker.image.inspect.raw"
	OperationImagePrune          = "docker.image.prune"
	OperationNetworkList         = "docker.network.list"
	OperationNetworkShow         = "docker.network.show"
	OperationNetworkCreate       = "docker.network.create"
	OperationNetworkRemove       = "docker.network.remove"
	OperationNetworkInspectRaw   = "docker.network.inspect.raw"
	OperationNetworkPrune        = "docker.network.prune"
	OperationSystemDF            = "docker.system.df"
	OperationSystemPrune         = "docker.system.prune"
	OperationEvents              = "docker.events"
	OperationVolumeList          = "docker.volume.list"
	OperationVolumeShow          = "docker.volume.show"
	OperationVolumeCreate        = "docker.volume.create"
	OperationVolumeRemove        = "docker.volume.remove"
	OperationVolumeInspectRaw    = "docker.volume.inspect.raw"
	OperationVolumePrune         = "docker.volume.prune"
	OperationBuildCachePrune     = "docker.build_cache.prune"
	OperationContextList         = "docker.context.list"
	OperationContextShow         = "docker.context.show"

	DatasourceContainers = "docker.containers"
	DatasourceImages     = "docker.images"
	DatasourceNetworks   = "docker.networks"
	DatasourceVolumes    = "docker.volumes"

	EntityContainer = "docker.container"
	EntityImage     = "docker.image"
	EntityNetwork   = "docker.network"
	EntityVolume    = "docker.volume"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"dock", PluginName},
		Operations: []core.OperationSpec{
			infoSpec(),
			containerListSpec(),
			containerShowSpec(),
			containerLogsSpec(),
			containerStatsSpec(),
			containerTopSpec(),
			containerExecSpec(),
			containerCopyFromSpec(),
			containerCopyToSpec(),
			containerCreateSpec(),
			containerRunSpec(),
			containerStartSpec(),
			containerStopSpec(),
			containerRestartSpec(),
			containerRemoveSpec(),
			containerInspectRawSpec(),
			containerPruneSpec(),
			imageListSpec(),
			imageShowSpec(),
			imagePullSpec(),
			imageTagSpec(),
			imagePushSpec(),
			imageBuildSpec(),
			imageRemoveSpec(),
			imageInspectRawSpec(),
			imagePruneSpec(),
			networkListSpec(),
			networkShowSpec(),
			networkCreateSpec(),
			networkRemoveSpec(),
			networkInspectRawSpec(),
			networkPruneSpec(),
			systemDFSpec(),
			systemPruneSpec(),
			eventsSpec(),
			volumeListSpec(),
			volumeShowSpec(),
			volumeCreateSpec(),
			volumeRemoveSpec(),
			volumeInspectRawSpec(),
			volumePruneSpec(),
			buildCachePruneSpec(),
			contextListSpec(),
			contextShowSpec(),
		},
		Datasources: []core.DatasourceSpec{
			containerDatasourceSpec(),
			imageDatasourceSpec(),
			networkDatasourceSpec(),
			volumeDatasourceSpec(),
		},
	}
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InfoInput, DockerInfo](OperationInfo, "Show Docker daemon and server information.", dockerReadOptions()...)
}

func containerListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerListInput, pluginbinding.ListResult[Container]](OperationContainerList, "List Docker containers.", dockerCompactReadOptions()...)
}

func containerShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ShowInput, pluginbinding.ShowResult[Container]](OperationContainerShow, "Show one Docker container by ID or name.", dockerReadOptions()...)
}

func containerLogsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerLogsInput, ContainerLogsResult](OperationContainerLogs, "Read recent Docker container logs.", dockerCompactReadOptions()...)
}

func containerStatsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerStatsInput, ContainerStatsResult](OperationContainerStats, "Show one-shot Docker container resource stats.", dockerReadOptions()...)
}

func containerTopSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerTopInput, ContainerTopResult](OperationContainerTop, "Show processes running inside a Docker container.", dockerCompactReadOptions()...)
}

func containerExecSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerExecInput, ContainerExecResult](OperationContainerExec, "Execute a command inside a Docker container.", dockerHighRiskWriteOptions(core.OperationNonIdempotent)...)
}

func containerCopyFromSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerCopyFromInput, ContainerCopyResult](OperationContainerCopyFrom, "Copy files from a Docker container to the local filesystem.", dockerHighRiskFilesystemOptions(core.OperationNonIdempotent)...)
}

func containerCopyToSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerCopyToInput, ContainerCopyResult](OperationContainerCopyTo, "Copy local files into a Docker container.", dockerHighRiskWriteOptions(core.OperationNonIdempotent)...)
}

func containerCreateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerCreateInput, ContainerCreateResult](OperationContainerCreate, "Create a Docker container.", dockerHighRiskWriteOptions(core.OperationNonIdempotent)...)
}

func containerRunSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerCreateInput, ContainerCreateResult](OperationContainerRun, "Create and start a Docker container.", dockerHighRiskWriteOptions(core.OperationNonIdempotent)...)
}

func containerStartSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerStartInput, ContainerActionResult](OperationContainerStart, "Start a Docker container.", dockerWriteOptions(core.OperationConditional)...)
}

func containerStopSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerStopInput, ContainerActionResult](OperationContainerStop, "Stop a Docker container.", dockerWriteOptions(core.OperationConditional)...)
}

func containerRestartSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerRestartInput, ContainerActionResult](OperationContainerRestart, "Restart a Docker container.", dockerWriteOptions(core.OperationNonIdempotent)...)
}

func containerRemoveSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContainerRemoveInput, ContainerActionResult](OperationContainerRemove, "Remove a Docker container.", dockerDestructiveOptions()...)
}

func containerInspectRawSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[RawInspectInput, RawInspectResult](OperationContainerInspectRaw, "Show raw Docker container inspect data.", dockerReadOptions()...)
}

func containerPruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PruneInput, PruneResult](OperationContainerPrune, "Prune stopped Docker containers.", dockerDestructiveOptions()...)
}

func imageListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImageListInput, pluginbinding.ListResult[Image]](OperationImageList, "List local Docker images.", dockerCompactReadOptions()...)
}

func imageShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ShowInput, pluginbinding.ShowResult[Image]](OperationImageShow, "Show one Docker image by ID, digest, or reference.", dockerReadOptions()...)
}

func imagePullSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImagePullInput, ImagePullResult](OperationImagePull, "Pull a Docker image.", dockerWriteOptions(core.OperationConditional)...)
}

func imageTagSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImageTagInput, ResourceActionResult](OperationImageTag, "Tag a Docker image.", dockerWriteOptions(core.OperationConditional)...)
}

func imagePushSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImagePushInput, ImagePushResult](OperationImagePush, "Push a Docker image.", dockerHighRiskWriteOptions(core.OperationNonIdempotent)...)
}

func imageBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImageBuildInput, ImageBuildResult](OperationImageBuild, "Build a Docker image from a local context.", dockerBuildOptions()...)
}

func imageRemoveSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImageRemoveInput, ImageRemoveResult](OperationImageRemove, "Remove a Docker image.", dockerDestructiveOptions()...)
}

func imageInspectRawSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[RawInspectInput, RawInspectResult](OperationImageInspectRaw, "Show raw Docker image inspect data.", dockerReadOptions()...)
}

func imagePruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ImagePruneInput, ImagePruneResult](OperationImagePrune, "Prune unused Docker images.", dockerDestructiveOptions()...)
}

func networkListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NetworkListInput, pluginbinding.ListResult[Network]](OperationNetworkList, "List Docker networks.", dockerCompactReadOptions()...)
}

func networkShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ShowInput, pluginbinding.ShowResult[Network]](OperationNetworkShow, "Show one Docker network by ID or name.", dockerReadOptions()...)
}

func networkCreateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NetworkCreateInput, ResourceActionResult](OperationNetworkCreate, "Create a Docker network.", dockerWriteOptions(core.OperationNonIdempotent)...)
}

func networkRemoveSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NetworkRemoveInput, ResourceActionResult](OperationNetworkRemove, "Remove a Docker network.", dockerDestructiveOptions()...)
}

func networkInspectRawSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[RawInspectInput, RawInspectResult](OperationNetworkInspectRaw, "Show raw Docker network inspect data.", dockerReadOptions()...)
}

func networkPruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PruneInput, PruneResult](OperationNetworkPrune, "Prune unused Docker networks.", dockerDestructiveOptions()...)
}

func systemDFSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[SystemDFInput, SystemDFResult](OperationSystemDF, "Show Docker disk usage by object type.", dockerReadOptions()...)
}

func systemPruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[SystemPruneInput, SystemPruneResult](OperationSystemPrune, "Prune unused Docker containers, networks, images, build cache, and optionally volumes.", dockerDestructiveOptions()...)
}

func eventsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[EventsInput, EventsResult](OperationEvents, "Show recent Docker daemon events.", dockerCompactReadOptions()...)
}

func volumeListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[VolumeListInput, pluginbinding.ListResult[Volume]](OperationVolumeList, "List Docker volumes.", dockerCompactReadOptions()...)
}

func volumeShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ShowInput, pluginbinding.ShowResult[Volume]](OperationVolumeShow, "Show one Docker volume by name.", dockerReadOptions()...)
}

func volumeCreateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[VolumeCreateInput, Volume](OperationVolumeCreate, "Create a Docker volume.", dockerWriteOptions(core.OperationNonIdempotent)...)
}

func volumeRemoveSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[VolumeRemoveInput, ResourceActionResult](OperationVolumeRemove, "Remove a Docker volume.", dockerDestructiveOptions()...)
}

func volumeInspectRawSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[RawInspectInput, RawInspectResult](OperationVolumeInspectRaw, "Show raw Docker volume inspect data.", dockerReadOptions()...)
}

func volumePruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PruneInput, PruneResult](OperationVolumePrune, "Prune unused Docker volumes.", dockerDestructiveOptions()...)
}

func buildCachePruneSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[BuildCachePruneInput, PruneResult](OperationBuildCachePrune, "Prune Docker build cache.", dockerDestructiveOptions()...)
}

func contextListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContextListInput, pluginbinding.ListResult[DockerContext]](OperationContextList, "List local Docker contexts.", dockerReadOptions()...)
}

func contextShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ContextShowInput, pluginbinding.ShowResult[DockerContext]](OperationContextShow, "Show one local Docker context.", dockerReadOptions()...)
}

func containerDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[ContainerRecord]](
		DatasourceContainers,
		EntityContainer,
		"Docker containers.",
		datasourceCapabilities(),
		pluginbinding.EntitySchemaFor[ContainerRecord](),
		pluginbinding.View("compact", "Container summary.", "title", "container_id", "name", "image", "state", "status"),
		pluginbinding.Completion("Container IDs, names, images, and labels.", "container_id", "short_id", "name", "image", "status"),
	)
}

func imageDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[ImageRecord]](
		DatasourceImages,
		EntityImage,
		"Docker images.",
		datasourceCapabilities(),
		pluginbinding.EntitySchemaFor[ImageRecord](),
		pluginbinding.View("compact", "Image summary.", "title", "image_id", "repo_tags", "size"),
		pluginbinding.Completion("Image IDs, tags, digests, and labels.", "image_id", "short_id", "repo_tags", "repo_digests"),
	)
}

func networkDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[NetworkRecord]](
		DatasourceNetworks,
		EntityNetwork,
		"Docker networks.",
		datasourceCapabilities(),
		pluginbinding.EntitySchemaFor[NetworkRecord](),
		pluginbinding.View("compact", "Network summary.", "title", "network_id", "name", "driver", "scope"),
		pluginbinding.Completion("Network IDs, names, drivers, and labels.", "network_id", "short_id", "name", "driver", "scope"),
	)
}

func volumeDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[VolumeRecord]](
		DatasourceVolumes,
		EntityVolume,
		"Docker volumes.",
		datasourceCapabilities(),
		pluginbinding.EntitySchemaFor[VolumeRecord](),
		pluginbinding.View("compact", "Volume summary.", "title", "name", "driver", "scope", "mountpoint"),
		pluginbinding.Completion("Volume names, drivers, mountpoints, and labels.", "name", "driver", "scope", "mountpoint"),
	)
}

func dockerReadOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessNetwork, core.OperationAccessFilesystem, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
}

func dockerCompactReadOptions() []pluginbinding.OperationSpecOption {
	options := dockerReadOptions()
	return append(options, pluginbinding.Compact())
}

func dockerWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectProcess, core.OperationEffectNetwork, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProcess, core.OperationAccessNetwork, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func dockerHighRiskWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectProcess, core.OperationEffectNetwork, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProcess, core.OperationAccessNetwork, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskHigh),
		pluginbinding.Idempotency(idempotency),
	}
}

func dockerBuildOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectFilesystem, core.OperationEffectNetwork, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessFilesystem, core.OperationAccessNetwork, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskHigh),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	}
}

func dockerHighRiskFilesystemOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectFilesystem, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessFilesystem, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskHigh),
		pluginbinding.Idempotency(idempotency),
	}
}

func dockerDestructiveOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectProcess, core.OperationEffectFilesystem, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProcess, core.OperationAccessFilesystem, core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskDestructive),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	}
}

func datasourceCapabilities() []string {
	return []string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup, pluginbinding.CapabilityGet}
}
