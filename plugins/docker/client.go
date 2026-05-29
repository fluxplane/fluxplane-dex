package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Client interface {
	Close() error
	Info(context.Context) (DockerInfo, error)
	ListContainers(context.Context, ContainerListInput) ([]Container, error)
	InspectContainer(context.Context, string) (Container, error)
	ContainerLogs(context.Context, ContainerLogsInput) (ContainerLogsResult, error)
	ContainerStats(context.Context, ContainerStatsInput) (ContainerStatsResult, error)
	ContainerTop(context.Context, ContainerTopInput) (ContainerTopResult, error)
	ContainerExec(context.Context, ContainerExecInput) (ContainerExecResult, error)
	ContainerCopyFrom(context.Context, ContainerCopyFromInput) (ContainerCopyResult, error)
	ContainerCopyTo(context.Context, ContainerCopyToInput) (ContainerCopyResult, error)
	ContainerCreate(context.Context, ContainerCreateInput) (ContainerCreateResult, error)
	ContainerRun(context.Context, ContainerCreateInput) (ContainerCreateResult, error)
	ContainerStart(context.Context, ContainerStartInput) (ContainerActionResult, error)
	ContainerStop(context.Context, ContainerStopInput) (ContainerActionResult, error)
	ContainerRestart(context.Context, ContainerRestartInput) (ContainerActionResult, error)
	ContainerRemove(context.Context, ContainerRemoveInput) (ContainerActionResult, error)
	ContainerInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	ContainerPrune(context.Context, PruneInput) (PruneResult, error)
	ListImages(context.Context, ImageListInput) ([]Image, error)
	InspectImage(context.Context, string) (Image, error)
	ImagePull(context.Context, ImagePullInput) (ImagePullResult, error)
	ImageTag(context.Context, ImageTagInput) (ResourceActionResult, error)
	ImagePush(context.Context, ImagePushInput) (ImagePushResult, error)
	ImageBuild(context.Context, ImageBuildInput) (ImageBuildResult, error)
	ImageRemove(context.Context, ImageRemoveInput) (ImageRemoveResult, error)
	ImageInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	ImagePrune(context.Context, ImagePruneInput) (ImagePruneResult, error)
	ListNetworks(context.Context, NetworkListInput) ([]Network, error)
	InspectNetwork(context.Context, string) (Network, error)
	NetworkCreate(context.Context, NetworkCreateInput) (ResourceActionResult, error)
	NetworkRemove(context.Context, NetworkRemoveInput) (ResourceActionResult, error)
	NetworkInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	NetworkPrune(context.Context, PruneInput) (PruneResult, error)
	SystemDF(context.Context, SystemDFInput) (SystemDFResult, error)
	SystemPrune(context.Context, SystemPruneInput) (SystemPruneResult, error)
	Events(context.Context, EventsInput) (EventsResult, error)
	ListVolumes(context.Context, VolumeListInput) ([]Volume, error)
	InspectVolume(context.Context, string) (Volume, error)
	VolumeCreate(context.Context, VolumeCreateInput) (Volume, error)
	VolumeRemove(context.Context, VolumeRemoveInput) (ResourceActionResult, error)
	VolumeInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	VolumePrune(context.Context, PruneInput) (PruneResult, error)
	BuildCachePrune(context.Context, BuildCachePruneInput) (PruneResult, error)
	ContextList(context.Context, ContextListInput) ([]DockerContext, error)
	ContextShow(context.Context, ContextShowInput) (DockerContext, error)
}

type ClientFactory func(pluginbinding.Context) (Client, error)

func NewLiveClient(ctx pluginbinding.Context) (Client, error) {
	if ctx.Host == nil {
		return nil, fmt.Errorf("host client is unavailable")
	}
	return providerClient{host: ctx.Host}, nil
}

type providerClient struct {
	host pluginbinding.HostClient
}

func (c providerClient) Close() error { return nil }

func (c providerClient) call(_ context.Context, action string, input any, out any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	resp, err := c.host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: PluginName,
		Action:   action,
		Payload:  payload,
	})
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func (c providerClient) Info(ctx context.Context) (DockerInfo, error) {
	var out DockerInfo
	err := c.call(ctx, "Info", nil, &out)
	return out, err
}

func (c providerClient) ListContainers(ctx context.Context, input ContainerListInput) ([]Container, error) {
	var out []Container
	err := c.call(ctx, "ListContainers", input, &out)
	return out, err
}

func (c providerClient) InspectContainer(ctx context.Context, id string) (Container, error) {
	var out Container
	err := c.call(ctx, "InspectContainer", id, &out)
	return out, err
}

func (c providerClient) ContainerLogs(ctx context.Context, input ContainerLogsInput) (ContainerLogsResult, error) {
	var out ContainerLogsResult
	err := c.call(ctx, "ContainerLogs", input, &out)
	return out, err
}

func (c providerClient) ContainerStats(ctx context.Context, input ContainerStatsInput) (ContainerStatsResult, error) {
	var out ContainerStatsResult
	err := c.call(ctx, "ContainerStats", input, &out)
	return out, err
}

func (c providerClient) ContainerTop(ctx context.Context, input ContainerTopInput) (ContainerTopResult, error) {
	var out ContainerTopResult
	err := c.call(ctx, "ContainerTop", input, &out)
	return out, err
}

func (c providerClient) ContainerExec(ctx context.Context, input ContainerExecInput) (ContainerExecResult, error) {
	var out ContainerExecResult
	err := c.call(ctx, "ContainerExec", input, &out)
	return out, err
}

func (c providerClient) ContainerCopyFrom(ctx context.Context, input ContainerCopyFromInput) (ContainerCopyResult, error) {
	var out ContainerCopyResult
	err := c.call(ctx, "ContainerCopyFrom", input, &out)
	return out, err
}

func (c providerClient) ContainerCopyTo(ctx context.Context, input ContainerCopyToInput) (ContainerCopyResult, error) {
	var out ContainerCopyResult
	err := c.call(ctx, "ContainerCopyTo", input, &out)
	return out, err
}

func (c providerClient) ContainerCreate(ctx context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	var out ContainerCreateResult
	err := c.call(ctx, "ContainerCreate", input, &out)
	return out, err
}

func (c providerClient) ContainerRun(ctx context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	var out ContainerCreateResult
	err := c.call(ctx, "ContainerRun", input, &out)
	return out, err
}

func (c providerClient) ContainerStart(ctx context.Context, input ContainerStartInput) (ContainerActionResult, error) {
	var out ContainerActionResult
	err := c.call(ctx, "ContainerStart", input, &out)
	return out, err
}

func (c providerClient) ContainerStop(ctx context.Context, input ContainerStopInput) (ContainerActionResult, error) {
	var out ContainerActionResult
	err := c.call(ctx, "ContainerStop", input, &out)
	return out, err
}

func (c providerClient) ContainerRestart(ctx context.Context, input ContainerRestartInput) (ContainerActionResult, error) {
	var out ContainerActionResult
	err := c.call(ctx, "ContainerRestart", input, &out)
	return out, err
}

func (c providerClient) ContainerRemove(ctx context.Context, input ContainerRemoveInput) (ContainerActionResult, error) {
	var out ContainerActionResult
	err := c.call(ctx, "ContainerRemove", input, &out)
	return out, err
}

func (c providerClient) ContainerInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	var out RawInspectResult
	err := c.call(ctx, "ContainerInspectRaw", input, &out)
	return out, err
}

func (c providerClient) ContainerPrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	var out PruneResult
	err := c.call(ctx, "ContainerPrune", input, &out)
	return out, err
}

func (c providerClient) ListImages(ctx context.Context, input ImageListInput) ([]Image, error) {
	var out []Image
	err := c.call(ctx, "ListImages", input, &out)
	return out, err
}

func (c providerClient) InspectImage(ctx context.Context, id string) (Image, error) {
	var out Image
	err := c.call(ctx, "InspectImage", id, &out)
	return out, err
}

func (c providerClient) ImagePull(ctx context.Context, input ImagePullInput) (ImagePullResult, error) {
	var out ImagePullResult
	err := c.call(ctx, "ImagePull", input, &out)
	return out, err
}

func (c providerClient) ImageTag(ctx context.Context, input ImageTagInput) (ResourceActionResult, error) {
	var out ResourceActionResult
	err := c.call(ctx, "ImageTag", input, &out)
	return out, err
}

func (c providerClient) ImagePush(ctx context.Context, input ImagePushInput) (ImagePushResult, error) {
	var out ImagePushResult
	err := c.call(ctx, "ImagePush", input, &out)
	return out, err
}

func (c providerClient) ImageBuild(ctx context.Context, input ImageBuildInput) (ImageBuildResult, error) {
	var out ImageBuildResult
	err := c.call(ctx, "ImageBuild", input, &out)
	return out, err
}

func (c providerClient) ImageRemove(ctx context.Context, input ImageRemoveInput) (ImageRemoveResult, error) {
	var out ImageRemoveResult
	err := c.call(ctx, "ImageRemove", input, &out)
	return out, err
}

func (c providerClient) ImageInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	var out RawInspectResult
	err := c.call(ctx, "ImageInspectRaw", input, &out)
	return out, err
}

func (c providerClient) ImagePrune(ctx context.Context, input ImagePruneInput) (ImagePruneResult, error) {
	var out ImagePruneResult
	err := c.call(ctx, "ImagePrune", input, &out)
	return out, err
}

func (c providerClient) ListNetworks(ctx context.Context, input NetworkListInput) ([]Network, error) {
	var out []Network
	err := c.call(ctx, "ListNetworks", input, &out)
	return out, err
}

func (c providerClient) InspectNetwork(ctx context.Context, id string) (Network, error) {
	var out Network
	err := c.call(ctx, "InspectNetwork", id, &out)
	return out, err
}

func (c providerClient) NetworkCreate(ctx context.Context, input NetworkCreateInput) (ResourceActionResult, error) {
	var out ResourceActionResult
	err := c.call(ctx, "NetworkCreate", input, &out)
	return out, err
}

func (c providerClient) NetworkRemove(ctx context.Context, input NetworkRemoveInput) (ResourceActionResult, error) {
	var out ResourceActionResult
	err := c.call(ctx, "NetworkRemove", input, &out)
	return out, err
}

func (c providerClient) NetworkInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	var out RawInspectResult
	err := c.call(ctx, "NetworkInspectRaw", input, &out)
	return out, err
}

func (c providerClient) NetworkPrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	var out PruneResult
	err := c.call(ctx, "NetworkPrune", input, &out)
	return out, err
}

func (c providerClient) SystemDF(ctx context.Context, input SystemDFInput) (SystemDFResult, error) {
	var out SystemDFResult
	err := c.call(ctx, "SystemDF", input, &out)
	return out, err
}

func (c providerClient) SystemPrune(ctx context.Context, input SystemPruneInput) (SystemPruneResult, error) {
	var out SystemPruneResult
	err := c.call(ctx, "SystemPrune", input, &out)
	return out, err
}

func (c providerClient) Events(ctx context.Context, input EventsInput) (EventsResult, error) {
	var out EventsResult
	err := c.call(ctx, "Events", input, &out)
	return out, err
}

func (c providerClient) ListVolumes(ctx context.Context, input VolumeListInput) ([]Volume, error) {
	var out []Volume
	err := c.call(ctx, "ListVolumes", input, &out)
	return out, err
}

func (c providerClient) InspectVolume(ctx context.Context, id string) (Volume, error) {
	var out Volume
	err := c.call(ctx, "InspectVolume", id, &out)
	return out, err
}

func (c providerClient) VolumeCreate(ctx context.Context, input VolumeCreateInput) (Volume, error) {
	var out Volume
	err := c.call(ctx, "VolumeCreate", input, &out)
	return out, err
}

func (c providerClient) VolumeRemove(ctx context.Context, input VolumeRemoveInput) (ResourceActionResult, error) {
	var out ResourceActionResult
	err := c.call(ctx, "VolumeRemove", input, &out)
	return out, err
}

func (c providerClient) VolumeInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	var out RawInspectResult
	err := c.call(ctx, "VolumeInspectRaw", input, &out)
	return out, err
}

func (c providerClient) VolumePrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	var out PruneResult
	err := c.call(ctx, "VolumePrune", input, &out)
	return out, err
}

func (c providerClient) BuildCachePrune(ctx context.Context, input BuildCachePruneInput) (PruneResult, error) {
	var out PruneResult
	err := c.call(ctx, "BuildCachePrune", input, &out)
	return out, err
}

func (c providerClient) ContextList(ctx context.Context, input ContextListInput) ([]DockerContext, error) {
	var out []DockerContext
	err := c.call(ctx, "ContextList", input, &out)
	return out, err
}

func (c providerClient) ContextShow(ctx context.Context, input ContextShowInput) (DockerContext, error) {
	var out DockerContext
	err := c.call(ctx, "ContextShow", input, &out)
	return out, err
}
