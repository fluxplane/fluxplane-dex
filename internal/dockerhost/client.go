package dockerhost

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	mobyarchive "github.com/moby/go-archive"
	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type liveClient struct {
	client *dockerclient.Client
}

func NewClient() (Client, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return liveClient{client: client}, nil
}

func (c liveClient) Close() error {
	return c.client.Close()
}

func (c liveClient) Info(ctx context.Context) (DockerInfo, error) {
	info, err := c.client.Info(ctx)
	if err != nil {
		return DockerInfo{}, err
	}
	version, err := c.client.ServerVersion(ctx)
	if err != nil {
		return DockerInfo{}, err
	}
	return normalizeInfo(info, version), nil
}

func (c liveClient) ListContainers(ctx context.Context, input ContainerListInput) ([]Container, error) {
	options := container.ListOptions{All: input.All, Limit: input.Limit, Filters: filters.NewArgs()}
	addFilterValues(options.Filters, "status", input.Status)
	addFilterValues(options.Filters, "name", input.Name)
	addFilterValues(options.Filters, "label", input.Label)
	items, err := c.client.ContainerList(ctx, options)
	if err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeContainerSummary(item))
	}
	return limitContainers(out, input.Limit), nil
}

func (c liveClient) InspectContainer(ctx context.Context, id string) (Container, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Container{}, fmt.Errorf("container id or name is required")
	}
	item, err := c.client.ContainerInspect(ctx, id)
	if err != nil {
		return Container{}, err
	}
	return normalizeContainerInspect(item), nil
}

func (c liveClient) ContainerLogs(ctx context.Context, input ContainerLogsInput) (ContainerLogsResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerLogsResult{}, fmt.Errorf("container id or name is required")
	}
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      strings.TrimSpace(input.Since),
		Until:      strings.TrimSpace(input.Until),
		Timestamps: input.Timestamps,
		Tail:       logsTail(input.Tail),
	}
	reader, err := c.client.ContainerLogs(ctx, id, options)
	if err != nil {
		return ContainerLogsResult{}, err
	}
	defer reader.Close()
	inspect, inspectErr := c.client.ContainerInspect(ctx, id)
	stdout, stderr, text, err := readContainerLogs(reader, inspectErr == nil && inspect.Config != nil && inspect.Config.Tty)
	if err != nil {
		return ContainerLogsResult{}, err
	}
	return ContainerLogsResult{Container: id, Tail: options.Tail, Stdout: stdout, Stderr: stderr, Text: text}, nil
}

func (c liveClient) ContainerStats(ctx context.Context, input ContainerStatsInput) (ContainerStatsResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerStatsResult{}, fmt.Errorf("container id or name is required")
	}
	reader, err := c.client.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return ContainerStatsResult{}, err
	}
	defer reader.Body.Close()
	var stats container.StatsResponse
	if err := json.NewDecoder(reader.Body).Decode(&stats); err != nil {
		return ContainerStatsResult{}, err
	}
	return normalizeContainerStats(id, reader.OSType, stats), nil
}

func (c liveClient) ContainerTop(ctx context.Context, input ContainerTopInput) (ContainerTopResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerTopResult{}, fmt.Errorf("container id or name is required")
	}
	out, err := c.client.ContainerTop(ctx, id, input.Args)
	if err != nil {
		return ContainerTopResult{}, err
	}
	return ContainerTopResult{Container: id, Titles: out.Titles, Processes: out.Processes, Count: len(out.Processes)}, nil
}

func (c liveClient) ContainerExec(ctx context.Context, input ContainerExecInput) (ContainerExecResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerExecResult{}, fmt.Errorf("container id or name is required")
	}
	if len(input.Cmd) == 0 {
		return ContainerExecResult{}, fmt.Errorf("cmd is required")
	}
	execCtx := ctx
	cancel := func() {}
	if input.TimeoutSecond > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutSecond)*time.Second)
	}
	defer cancel()
	create, err := c.client.ContainerExecCreate(execCtx, id, container.ExecOptions{
		User:         strings.TrimSpace(input.User),
		Privileged:   input.Privileged,
		Tty:          input.TTY,
		AttachStdout: !input.Detach,
		AttachStderr: !input.Detach,
		Env:          append([]string(nil), input.Env...),
		WorkingDir:   strings.TrimSpace(input.Workdir),
		Cmd:          append([]string(nil), input.Cmd...),
	})
	if err != nil {
		return ContainerExecResult{}, err
	}
	result := ContainerExecResult{Container: id, ExecID: create.ID, Detached: input.Detach}
	if input.Detach {
		if err := c.client.ContainerExecStart(execCtx, create.ID, container.ExecStartOptions{Detach: true, Tty: input.TTY}); err != nil {
			return ContainerExecResult{}, err
		}
		result.Running = true
		result.OK = true
		return result, nil
	}
	hijacked, err := c.client.ContainerExecAttach(execCtx, create.ID, container.ExecAttachOptions{Tty: input.TTY})
	if err != nil {
		return ContainerExecResult{}, err
	}
	defer hijacked.Close()
	stdout, stderr, text, err := readContainerLogs(hijacked.Reader, input.TTY)
	if err != nil {
		return ContainerExecResult{}, err
	}
	inspect, err := c.client.ContainerExecInspect(context.Background(), create.ID)
	if err != nil {
		return ContainerExecResult{}, err
	}
	result.Stdout = stdout
	result.Stderr = stderr
	result.Text = text
	result.ExitCode = inspect.ExitCode
	result.Running = inspect.Running
	result.OK = inspect.ExitCode == 0
	return result, nil
}

func (c liveClient) ContainerCopyFrom(ctx context.Context, input ContainerCopyFromInput) (ContainerCopyResult, error) {
	id := strings.TrimSpace(input.ID)
	source := strings.TrimSpace(input.SourcePath)
	destination := strings.TrimSpace(input.DestinationPath)
	if id == "" {
		return ContainerCopyResult{}, fmt.Errorf("container id or name is required")
	}
	if source == "" {
		return ContainerCopyResult{}, fmt.Errorf("source_path is required")
	}
	if destination == "" {
		return ContainerCopyResult{}, fmt.Errorf("destination_path is required")
	}
	reader, _, err := c.client.CopyFromContainer(ctx, id, source)
	if err != nil {
		return ContainerCopyResult{}, err
	}
	defer reader.Close()
	files, bytes, err := extractTarToDirectory(reader, destination, input.Overwrite)
	if err != nil {
		return ContainerCopyResult{}, err
	}
	return ContainerCopyResult{Container: id, SourcePath: source, DestinationPath: destination, Files: files, Bytes: bytes, OK: true}, nil
}

func (c liveClient) ContainerCopyTo(ctx context.Context, input ContainerCopyToInput) (ContainerCopyResult, error) {
	id := strings.TrimSpace(input.ID)
	source := strings.TrimSpace(input.SourcePath)
	destination := strings.TrimSpace(input.DestinationPath)
	if id == "" {
		return ContainerCopyResult{}, fmt.Errorf("container id or name is required")
	}
	if source == "" {
		return ContainerCopyResult{}, fmt.Errorf("source_path is required")
	}
	if destination == "" {
		return ContainerCopyResult{}, fmt.Errorf("destination_path is required")
	}
	body, files, bytes, err := tarLocalPath(source)
	if err != nil {
		return ContainerCopyResult{}, err
	}
	defer body.Close()
	err = c.client.CopyToContainer(ctx, id, destination, body, container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: input.AllowOverwriteDirWithFile,
		CopyUIDGID:                input.CopyUIDGID,
	})
	if err != nil {
		return ContainerCopyResult{}, err
	}
	return ContainerCopyResult{Container: id, SourcePath: source, DestinationPath: destination, Files: files, Bytes: bytes, OK: true}, nil
}

func (c liveClient) ContainerCreate(ctx context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	config, hostConfig, networkingConfig, platform, err := containerCreateConfig(input)
	if err != nil {
		return ContainerCreateResult{}, err
	}
	created, err := c.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, strings.TrimSpace(input.Name))
	if err != nil {
		return ContainerCreateResult{}, err
	}
	return ContainerCreateResult{ID: created.ID, Name: strings.TrimSpace(input.Name), Image: strings.TrimSpace(input.Image), Warnings: created.Warnings, OK: true}, nil
}

func (c liveClient) ContainerRun(ctx context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	result, err := c.ContainerCreate(ctx, input)
	if err != nil {
		return ContainerCreateResult{}, err
	}
	if err := c.client.ContainerStart(ctx, result.ID, container.StartOptions{}); err != nil {
		return ContainerCreateResult{}, err
	}
	result.Started = true
	return result, nil
}

func (c liveClient) ContainerStart(ctx context.Context, input ContainerStartInput) (ContainerActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerActionResult{}, fmt.Errorf("container id or name is required")
	}
	if err := c.client.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return ContainerActionResult{}, err
	}
	return ContainerActionResult{Container: id, Action: "start", OK: true}, nil
}

func (c liveClient) ContainerStop(ctx context.Context, input ContainerStopInput) (ContainerActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerActionResult{}, fmt.Errorf("container id or name is required")
	}
	if err := c.client.ContainerStop(ctx, id, stopOptions(input.Timeout, input.Signal)); err != nil {
		return ContainerActionResult{}, err
	}
	return ContainerActionResult{Container: id, Action: "stop", OK: true}, nil
}

func (c liveClient) ContainerRestart(ctx context.Context, input ContainerRestartInput) (ContainerActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerActionResult{}, fmt.Errorf("container id or name is required")
	}
	if err := c.client.ContainerRestart(ctx, id, stopOptions(input.Timeout, input.Signal)); err != nil {
		return ContainerActionResult{}, err
	}
	return ContainerActionResult{Container: id, Action: "restart", OK: true}, nil
}

func (c liveClient) ContainerRemove(ctx context.Context, input ContainerRemoveInput) (ContainerActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ContainerActionResult{}, fmt.Errorf("container id or name is required")
	}
	options := container.RemoveOptions{Force: input.Force, RemoveVolumes: input.Volumes}
	if err := c.client.ContainerRemove(ctx, id, options); err != nil {
		return ContainerActionResult{}, err
	}
	return ContainerActionResult{Container: id, Action: "remove", OK: true}, nil
}

func (c liveClient) ContainerInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return RawInspectResult{}, fmt.Errorf("container id or name is required")
	}
	out, err := c.client.ContainerInspect(ctx, id)
	if err != nil {
		return RawInspectResult{}, err
	}
	return rawInspectResult("container", id, out)
}

func (c liveClient) ContainerPrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	report, err := c.client.ContainersPrune(ctx, pruneFilters(input.Until, input.Label, nil))
	if err != nil {
		return PruneResult{}, err
	}
	return PruneResult{Kind: "container", Deleted: report.ContainersDeleted, SpaceReclaimedBytes: report.SpaceReclaimed, Count: len(report.ContainersDeleted), OK: true}, nil
}

func (c liveClient) ListImages(ctx context.Context, input ImageListInput) ([]Image, error) {
	options := image.ListOptions{All: input.All, Filters: filters.NewArgs()}
	addFilterValues(options.Filters, "reference", input.Reference)
	addFilterValues(options.Filters, "label", input.Label)
	items, err := c.client.ImageList(ctx, options)
	if err != nil {
		return nil, err
	}
	out := make([]Image, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeImageSummary(item))
	}
	return limitImages(out, input.Limit), nil
}

func (c liveClient) InspectImage(ctx context.Context, id string) (Image, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Image{}, fmt.Errorf("image id, digest, or reference is required")
	}
	item, err := c.client.ImageInspect(ctx, id)
	if err != nil {
		return Image{}, err
	}
	return normalizeImageInspect(item), nil
}

func (c liveClient) ImagePull(ctx context.Context, input ImagePullInput) (ImagePullResult, error) {
	ref := strings.TrimSpace(input.Reference)
	if ref == "" {
		return ImagePullResult{}, fmt.Errorf("image reference is required")
	}
	auth, err := registryAuthHeader(input.RegistryAuth, input.Auth)
	if err != nil {
		return ImagePullResult{}, err
	}
	reader, err := c.client.ImagePull(ctx, ref, image.PullOptions{Platform: strings.TrimSpace(input.Platform), RegistryAuth: auth})
	if err != nil {
		return ImagePullResult{}, err
	}
	defer reader.Close()
	events, err := decodeProgressEvents(reader, progressLimit(input.Limit))
	if err != nil {
		return ImagePullResult{}, err
	}
	return ImagePullResult{Reference: ref, Platform: strings.TrimSpace(input.Platform), Events: events, Count: len(events), OK: true}, nil
}

func (c liveClient) ImageTag(ctx context.Context, input ImageTagInput) (ResourceActionResult, error) {
	source := strings.TrimSpace(input.Source)
	target := strings.TrimSpace(input.Target)
	if source == "" {
		return ResourceActionResult{}, fmt.Errorf("source image is required")
	}
	if target == "" {
		return ResourceActionResult{}, fmt.Errorf("target image reference is required")
	}
	if err := c.client.ImageTag(ctx, source, target); err != nil {
		return ResourceActionResult{}, err
	}
	return ResourceActionResult{ID: source, Action: "tag", OK: true}, nil
}

func (c liveClient) ImagePush(ctx context.Context, input ImagePushInput) (ImagePushResult, error) {
	ref := strings.TrimSpace(input.Reference)
	if ref == "" {
		return ImagePushResult{}, fmt.Errorf("image reference is required")
	}
	platform, err := platformSpec(input.Platform)
	if err != nil {
		return ImagePushResult{}, err
	}
	auth, err := registryAuthHeader(input.RegistryAuth, input.Auth)
	if err != nil {
		return ImagePushResult{}, err
	}
	reader, err := c.client.ImagePush(ctx, ref, image.PushOptions{Platform: platform, RegistryAuth: auth})
	if err != nil {
		return ImagePushResult{}, err
	}
	defer reader.Close()
	events, err := decodeProgressEvents(reader, progressLimit(input.Limit))
	if err != nil {
		return ImagePushResult{}, err
	}
	return ImagePushResult{Reference: ref, Platform: strings.TrimSpace(input.Platform), Events: events, Count: len(events), OK: true}, nil
}

func (c liveClient) ImageBuild(ctx context.Context, input ImageBuildInput) (ImageBuildResult, error) {
	contextPath := strings.TrimSpace(input.ContextPath)
	if contextPath == "" {
		return ImageBuildResult{}, fmt.Errorf("context_path is required")
	}
	body, err := tarBuildContext(contextPath, input.Dockerfile)
	if err != nil {
		return ImageBuildResult{}, err
	}
	defer body.Close()
	authConfigs, err := buildAuthConfigs(input)
	if err != nil {
		return ImageBuildResult{}, err
	}
	response, err := c.client.ImageBuild(ctx, body, build.ImageBuildOptions{
		Tags:        append([]string(nil), input.Tags...),
		Dockerfile:  strings.TrimSpace(input.Dockerfile),
		Target:      strings.TrimSpace(input.Target),
		BuildArgs:   buildArgs(input.BuildArgs),
		AuthConfigs: authConfigs,
		Labels:      cloneStringMap(input.Labels),
		Platform:    strings.TrimSpace(input.Platform),
		PullParent:  input.Pull,
		NoCache:     input.NoCache,
		NetworkMode: strings.TrimSpace(input.Network),
		Remove:      true,
	})
	if err != nil {
		return ImageBuildResult{}, err
	}
	defer response.Body.Close()
	events, err := decodeProgressEvents(response.Body, buildProgressLimit(input.Limit))
	if err != nil {
		return ImageBuildResult{}, err
	}
	return ImageBuildResult{ContextPath: contextPath, Tags: append([]string(nil), input.Tags...), ImageID: imageIDFromEvents(events), Events: events, Count: len(events), OK: true}, nil
}

func (c liveClient) ImageRemove(ctx context.Context, input ImageRemoveInput) (ImageRemoveResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ImageRemoveResult{}, fmt.Errorf("image id, digest, or reference is required")
	}
	responses, err := c.client.ImageRemove(ctx, id, image.RemoveOptions{Force: input.Force, PruneChildren: input.PruneChildren})
	if err != nil {
		return ImageRemoveResult{}, err
	}
	result := ImageRemoveResult{ID: id, OK: true}
	for _, response := range responses {
		if strings.TrimSpace(response.Deleted) != "" {
			result.Deleted = append(result.Deleted, response.Deleted)
		}
		if strings.TrimSpace(response.Untagged) != "" {
			result.Untagged = append(result.Untagged, response.Untagged)
		}
	}
	return result, nil
}

func (c liveClient) ImageInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return RawInspectResult{}, fmt.Errorf("image id, digest, or reference is required")
	}
	out, err := c.client.ImageInspect(ctx, id)
	if err != nil {
		return RawInspectResult{}, err
	}
	return rawInspectResult("image", id, out)
}

func (c liveClient) ImagePrune(ctx context.Context, input ImagePruneInput) (ImagePruneResult, error) {
	extra := map[string][]string{}
	if input.All {
		extra["dangling"] = []string{"false"}
	}
	report, err := c.client.ImagesPrune(ctx, pruneFilters(input.Until, input.Label, extra))
	if err != nil {
		return ImagePruneResult{}, err
	}
	result := ImagePruneResult{Kind: "image", SpaceReclaimedBytes: report.SpaceReclaimed, OK: true}
	for _, deleted := range report.ImagesDeleted {
		if strings.TrimSpace(deleted.Deleted) != "" {
			result.Deleted = append(result.Deleted, deleted.Deleted)
		}
		if strings.TrimSpace(deleted.Untagged) != "" {
			result.Untagged = append(result.Untagged, deleted.Untagged)
		}
	}
	result.Count = len(result.Deleted) + len(result.Untagged)
	return result, nil
}

func (c liveClient) ListNetworks(ctx context.Context, input NetworkListInput) ([]Network, error) {
	options := network.ListOptions{Filters: filters.NewArgs()}
	addFilterValues(options.Filters, "name", input.Name)
	addFilterValues(options.Filters, "label", input.Label)
	items, err := c.client.NetworkList(ctx, options)
	if err != nil {
		return nil, err
	}
	out := make([]Network, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeNetwork(item))
	}
	return limitNetworks(out, input.Limit), nil
}

func (c liveClient) InspectNetwork(ctx context.Context, id string) (Network, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Network{}, fmt.Errorf("network id or name is required")
	}
	item, err := c.client.NetworkInspect(ctx, id, network.InspectOptions{})
	if err != nil {
		return Network{}, err
	}
	return normalizeNetwork(item), nil
}

func (c liveClient) NetworkCreate(ctx context.Context, input NetworkCreateInput) (ResourceActionResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ResourceActionResult{}, fmt.Errorf("network name is required")
	}
	created, err := c.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:     strings.TrimSpace(input.Driver),
		Scope:      strings.TrimSpace(input.Scope),
		EnableIPv4: input.EnableIPv4,
		EnableIPv6: input.EnableIPv6,
		Internal:   input.Internal,
		Attachable: input.Attachable,
		Ingress:    input.Ingress,
		Options:    cloneStringMap(input.Options),
		Labels:     cloneStringMap(input.Labels),
	})
	if err != nil {
		return ResourceActionResult{}, err
	}
	return ResourceActionResult{ID: created.ID, Action: "create", OK: true}, nil
}

func (c liveClient) NetworkRemove(ctx context.Context, input NetworkRemoveInput) (ResourceActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ResourceActionResult{}, fmt.Errorf("network id or name is required")
	}
	if err := c.client.NetworkRemove(ctx, id); err != nil {
		return ResourceActionResult{}, err
	}
	return ResourceActionResult{ID: id, Action: "remove", OK: true}, nil
}

func (c liveClient) NetworkInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return RawInspectResult{}, fmt.Errorf("network id or name is required")
	}
	out, err := c.client.NetworkInspect(ctx, id, network.InspectOptions{})
	if err != nil {
		return RawInspectResult{}, err
	}
	return rawInspectResult("network", id, out)
}

func (c liveClient) NetworkPrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	report, err := c.client.NetworksPrune(ctx, pruneFilters(input.Until, input.Label, nil))
	if err != nil {
		return PruneResult{}, err
	}
	return PruneResult{Kind: "network", Deleted: report.NetworksDeleted, Count: len(report.NetworksDeleted), OK: true}, nil
}

func (c liveClient) SystemDF(ctx context.Context, input SystemDFInput) (SystemDFResult, error) {
	options := dockertypes.DiskUsageOptions{Types: diskUsageTypes(input.Type)}
	usage, err := c.client.DiskUsage(ctx, options)
	if err != nil {
		return SystemDFResult{}, err
	}
	return normalizeSystemDF(usage), nil
}

func (c liveClient) SystemPrune(ctx context.Context, input SystemPruneInput) (SystemPruneResult, error) {
	containers, err := c.ContainerPrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
	if err != nil {
		return SystemPruneResult{}, err
	}
	networks, err := c.NetworkPrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
	if err != nil {
		return SystemPruneResult{}, err
	}
	images, err := c.ImagePrune(ctx, ImagePruneInput{All: input.All, Until: input.Until, Label: input.Label})
	if err != nil {
		return SystemPruneResult{}, err
	}
	buildCache, err := c.BuildCachePrune(ctx, BuildCachePruneInput{All: input.All, Until: input.Until, Label: input.Label})
	if err != nil {
		return SystemPruneResult{}, err
	}
	result := SystemPruneResult{Containers: containers, Networks: networks, Images: images, BuildCache: buildCache, OK: true}
	if input.Volumes {
		volumes, err := c.VolumePrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
		if err != nil {
			return SystemPruneResult{}, err
		}
		result.Volumes = &volumes
	}
	result.TotalCount = containers.Count + networks.Count + images.Count + buildCache.Count
	result.TotalBytes = containers.SpaceReclaimedBytes + networks.SpaceReclaimedBytes + images.SpaceReclaimedBytes + buildCache.SpaceReclaimedBytes
	if result.Volumes != nil {
		result.TotalCount += result.Volumes.Count
		result.TotalBytes += result.Volumes.SpaceReclaimedBytes
	}
	return result, nil
}

func (c liveClient) Events(ctx context.Context, input EventsInput) (EventsResult, error) {
	options := events.ListOptions{Since: defaultSince(input.Since), Until: defaultUntil(input.Until), Filters: filters.NewArgs()}
	addFilterValues(options.Filters, "type", input.Type)
	addFilterValues(options.Filters, "event", input.Action)
	addFilterValues(options.Filters, "container", input.Container)
	addFilterValues(options.Filters, "image", input.Image)
	addFilterValues(options.Filters, "label", input.Label)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	messages, errs := c.client.Events(ctx, options)
	limit := eventsLimit(input.Limit)
	out := make([]Event, 0, limit)
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				return EventsResult{Events: out, Count: len(out)}, nil
			}
			out = append(out, normalizeEvent(message))
			if len(out) >= limit {
				return EventsResult{Events: out, Count: len(out)}, nil
			}
		case err, ok := <-errs:
			if !ok || err == nil || err == io.EOF || err == context.Canceled {
				return EventsResult{Events: out, Count: len(out)}, nil
			}
			return EventsResult{}, err
		case <-ctx.Done():
			return EventsResult{Events: out, Count: len(out)}, nil
		}
	}
}

func (c liveClient) ListVolumes(ctx context.Context, input VolumeListInput) ([]Volume, error) {
	options := volume.ListOptions{Filters: filters.NewArgs()}
	addFilterValues(options.Filters, "name", input.Name)
	addFilterValues(options.Filters, "label", input.Label)
	items, err := c.client.VolumeList(ctx, options)
	if err != nil {
		return nil, err
	}
	out := make([]Volume, 0, len(items.Volumes))
	for _, item := range items.Volumes {
		if item != nil {
			out = append(out, normalizeVolume(*item))
		}
	}
	return limitVolumes(out, input.Limit), nil
}

func (c liveClient) InspectVolume(ctx context.Context, name string) (Volume, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Volume{}, fmt.Errorf("volume name is required")
	}
	item, err := c.client.VolumeInspect(ctx, name)
	if err != nil {
		return Volume{}, err
	}
	return normalizeVolume(item), nil
}

func (c liveClient) VolumeCreate(ctx context.Context, input VolumeCreateInput) (Volume, error) {
	item, err := c.client.VolumeCreate(ctx, volume.CreateOptions{
		Name:       strings.TrimSpace(input.Name),
		Driver:     strings.TrimSpace(input.Driver),
		DriverOpts: cloneStringMap(input.DriverOpts),
		Labels:     cloneStringMap(input.Labels),
	})
	if err != nil {
		return Volume{}, err
	}
	return normalizeVolume(item), nil
}

func (c liveClient) VolumeRemove(ctx context.Context, input VolumeRemoveInput) (ResourceActionResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ResourceActionResult{}, fmt.Errorf("volume name is required")
	}
	if err := c.client.VolumeRemove(ctx, id, input.Force); err != nil {
		return ResourceActionResult{}, err
	}
	return ResourceActionResult{ID: id, Action: "remove", OK: true}, nil
}

func (c liveClient) VolumeInspectRaw(ctx context.Context, input RawInspectInput) (RawInspectResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return RawInspectResult{}, fmt.Errorf("volume name is required")
	}
	out, err := c.client.VolumeInspect(ctx, id)
	if err != nil {
		return RawInspectResult{}, err
	}
	return rawInspectResult("volume", id, out)
}

func (c liveClient) VolumePrune(ctx context.Context, input PruneInput) (PruneResult, error) {
	report, err := c.client.VolumesPrune(ctx, pruneFilters(input.Until, input.Label, nil))
	if err != nil {
		return PruneResult{}, err
	}
	return PruneResult{Kind: "volume", Deleted: report.VolumesDeleted, SpaceReclaimedBytes: report.SpaceReclaimed, Count: len(report.VolumesDeleted), OK: true}, nil
}

func (c liveClient) BuildCachePrune(ctx context.Context, input BuildCachePruneInput) (PruneResult, error) {
	options := build.CachePruneOptions{
		All:           input.All,
		KeepStorage:   input.KeepStorage,
		ReservedSpace: input.ReservedSpace,
		MaxUsedSpace:  input.MaxUsedSpace,
		MinFreeSpace:  input.MinFreeSpace,
		Filters:       pruneFilters(input.Until, input.Label, nil),
	}
	report, err := c.client.BuildCachePrune(ctx, options)
	if err != nil {
		return PruneResult{}, err
	}
	if report == nil {
		return PruneResult{Kind: "build_cache", OK: true}, nil
	}
	return PruneResult{Kind: "build_cache", Deleted: report.CachesDeleted, SpaceReclaimedBytes: report.SpaceReclaimed, Count: len(report.CachesDeleted), OK: true}, nil
}

func (c liveClient) ContextList(ctx context.Context, input ContextListInput) ([]DockerContext, error) {
	return listDockerContexts()
}

func (c liveClient) ContextShow(ctx context.Context, input ContextShowInput) (DockerContext, error) {
	name := strings.TrimSpace(input.Name)
	contexts, err := listDockerContexts()
	if err != nil {
		return DockerContext{}, err
	}
	if name == "" {
		for _, item := range contexts {
			if item.Current {
				return item, nil
			}
		}
		name = "default"
	}
	for _, item := range contexts {
		if item.Name == name {
			return item, nil
		}
	}
	return DockerContext{}, fmt.Errorf("context %q not found", name)
}

func normalizeInfo(info system.Info, version dockertypes.Version) DockerInfo {
	return DockerInfo{
		ID:                info.ID,
		Name:              info.Name,
		ServerVersion:     firstNonEmpty(info.ServerVersion, version.Version),
		APIVersion:        version.APIVersion,
		MinAPIVersion:     version.MinAPIVersion,
		OSType:            info.OSType,
		OperatingSystem:   info.OperatingSystem,
		Architecture:      info.Architecture,
		KernelVersion:     info.KernelVersion,
		Driver:            info.Driver,
		CgroupDriver:      info.CgroupDriver,
		CgroupVersion:     info.CgroupVersion,
		LoggingDriver:     info.LoggingDriver,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
		CPUs:              info.NCPU,
		MemoryBytes:       info.MemTotal,
		DockerRootDir:     info.DockerRootDir,
		Warnings:          append([]string(nil), info.Warnings...),
	}
}

func normalizeContainerSummary(item container.Summary) Container {
	var networks []string
	if item.NetworkSettings != nil {
		networks = make([]string, 0, len(item.NetworkSettings.Networks))
		for name := range item.NetworkSettings.Networks {
			networks = append(networks, name)
		}
	}
	sort.Strings(networks)
	mounts := make([]string, 0, len(item.Mounts))
	for _, mount := range item.Mounts {
		mounts = append(mounts, firstNonEmpty(mount.Name, mount.Source, mount.Destination))
	}
	return Container{
		ID:       item.ID,
		ShortID:  shortID(item.ID),
		Names:    cleanContainerNames(item.Names),
		Name:     firstString(cleanContainerNames(item.Names)),
		Image:    item.Image,
		ImageID:  item.ImageID,
		Command:  item.Command,
		Created:  item.Created,
		State:    string(item.State),
		Status:   item.Status,
		Ports:    containerPorts(item.Ports),
		Networks: networks,
		Mounts:   mounts,
		Labels:   cloneStringMap(item.Labels),
	}
}

func normalizeContainerInspect(item container.InspectResponse) Container {
	out := Container{}
	if item.ContainerJSONBase != nil {
		out.ID = item.ID
		out.ShortID = shortID(item.ID)
		out.Name = strings.TrimPrefix(item.Name, "/")
		out.Names = []string{out.Name}
		out.ImageID = item.Image
		out.Command = strings.Join(append([]string{item.Path}, item.Args...), " ")
		out.Platform = item.Platform
		if item.HostConfig != nil {
			out.Restart = string(item.HostConfig.RestartPolicy.Name)
		}
		if item.State != nil {
			out.State = string(item.State.Status)
			out.Status = item.State.Status
			if item.State.Health != nil {
				out.Health = item.State.Health.Status
			}
		}
		if item.Config != nil {
			out.Image = item.Config.Image
			out.Labels = cloneStringMap(item.Config.Labels)
			out.EnvKeys = envKeys(item.Config.Env)
		}
	}
	if item.NetworkSettings != nil {
		for name := range item.NetworkSettings.Networks {
			out.Networks = append(out.Networks, name)
		}
		sort.Strings(out.Networks)
	}
	for _, mount := range item.Mounts {
		out.Mounts = append(out.Mounts, firstNonEmpty(mount.Name, mount.Source, mount.Destination))
	}
	return out
}

func normalizeImageSummary(item image.Summary) Image {
	id := strings.TrimSpace(item.ID)
	return Image{
		ID:          id,
		ShortID:     shortID(id),
		Title:       firstNonEmpty(firstString(item.RepoTags), firstString(item.RepoDigests), shortID(id)),
		RepoTags:    cleanDockerNoneValues(item.RepoTags),
		RepoDigests: cleanDockerNoneValues(item.RepoDigests),
		Created:     item.Created,
		Size:        item.Size,
		SharedSize:  item.SharedSize,
		Containers:  item.Containers,
		Labels:      cloneStringMap(item.Labels),
	}
}

func normalizeImageInspect(item image.InspectResponse) Image {
	id := strings.TrimSpace(item.ID)
	var labels map[string]string
	if item.Config != nil {
		labels = cloneStringMap(item.Config.Labels)
	}
	return Image{
		ID:            id,
		ShortID:       shortID(id),
		Title:         firstNonEmpty(firstString(item.RepoTags), firstString(item.RepoDigests), shortID(id)),
		RepoTags:      cleanDockerNoneValues(item.RepoTags),
		RepoDigests:   cleanDockerNoneValues(item.RepoDigests),
		Size:          item.Size,
		CreatedAt:     item.Created,
		OS:            item.Os,
		Architecture:  item.Architecture,
		DockerVersion: item.DockerVersion,
		Author:        item.Author,
		Labels:        labels,
	}
}

func normalizeContainerStats(id, osType string, stats container.StatsResponse) ContainerStatsResult {
	networks := map[string]NetIO{}
	var rx, tx uint64
	for name, net := range stats.Networks {
		networks[name] = NetIO{RxBytes: net.RxBytes, TxBytes: net.TxBytes}
		rx += net.RxBytes
		tx += net.TxBytes
	}
	read, write := blockIO(stats)
	result := ContainerStatsResult{
		Container:        firstNonEmpty(stats.ID, id),
		Name:             strings.TrimPrefix(stats.Name, "/"),
		OSType:           osType,
		Read:             stats.Read.Format(time.RFC3339Nano),
		CPUPercent:       cpuPercent(stats),
		MemoryUsageBytes: stats.MemoryStats.Usage,
		MemoryLimitBytes: stats.MemoryStats.Limit,
		MemoryPercent:    memoryPercent(stats),
		PIDs:             stats.PidsStats.Current,
		NetworkRxBytes:   rx,
		NetworkTxBytes:   tx,
		BlockReadBytes:   read,
		BlockWriteBytes:  write,
		Networks:         networks,
	}
	if len(result.Networks) == 0 {
		result.Networks = nil
	}
	return result
}

func normalizeSystemDF(usage dockertypes.DiskUsage) SystemDFResult {
	result := SystemDFResult{LayersSizeBytes: usage.LayersSize, BuildCacheCount: len(usage.BuildCache)}
	for _, item := range usage.Images {
		if item != nil {
			result.Images = append(result.Images, normalizeImageSummary(*item))
		}
	}
	for _, item := range usage.Containers {
		if item != nil {
			result.Containers = append(result.Containers, normalizeContainerSummary(*item))
		}
	}
	for _, item := range usage.Volumes {
		if item != nil {
			result.Volumes = append(result.Volumes, normalizeVolume(*item))
		}
	}
	result.ImageCount = len(result.Images)
	result.ContainerCount = len(result.Containers)
	result.VolumeCount = len(result.Volumes)
	return result
}

func normalizeEvent(message events.Message) Event {
	return Event{
		Type:       string(message.Type),
		Action:     string(message.Action),
		ID:         message.ID,
		ActorID:    message.Actor.ID,
		Scope:      message.Scope,
		Time:       message.Time,
		TimeNano:   message.TimeNano,
		Attributes: cloneStringMap(message.Actor.Attributes),
	}
}

func normalizeNetwork(item network.Inspect) Network {
	endpoints := make([]NetworkEndpoint, 0, len(item.Containers))
	for id, endpoint := range item.Containers {
		endpoints = append(endpoints, NetworkEndpoint{ID: id, Name: endpoint.Name, EndpointID: endpoint.EndpointID, IPv4Address: endpoint.IPv4Address, IPv6Address: endpoint.IPv6Address})
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Name < endpoints[j].Name })
	return Network{
		ID:         item.ID,
		ShortID:    shortID(item.ID),
		Name:       item.Name,
		Driver:     item.Driver,
		Scope:      item.Scope,
		Internal:   item.Internal,
		Attachable: item.Attachable,
		Ingress:    item.Ingress,
		Containers: endpoints,
		Labels:     cloneStringMap(item.Labels),
	}
}

func normalizeVolume(item volume.Volume) Volume {
	var size int64
	var refCount int64
	if item.UsageData != nil {
		size = item.UsageData.Size
		refCount = item.UsageData.RefCount
	}
	return Volume{
		Name:       item.Name,
		Driver:     item.Driver,
		Mountpoint: item.Mountpoint,
		Scope:      item.Scope,
		CreatedAt:  item.CreatedAt,
		Size:       size,
		RefCount:   refCount,
		Labels:     cloneStringMap(item.Labels),
		Options:    cloneStringMap(item.Options),
	}
}

func readContainerLogs(reader io.Reader, tty bool) (string, string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if tty {
		if _, err := io.Copy(&stdout, reader); err != nil {
			return "", "", "", err
		}
		text := stdout.String()
		return text, "", text, nil
	}
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", "", "", err
	}
	return stdout.String(), stderr.String(), stdout.String() + stderr.String(), nil
}

func cpuPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta <= 0 || cpuDelta <= 0 || onlineCPUs <= 0 {
		return 0
	}
	return (cpuDelta / systemDelta) * onlineCPUs * 100
}

func memoryPercent(stats container.StatsResponse) float64 {
	if stats.MemoryStats.Limit == 0 || stats.MemoryStats.Usage == 0 {
		return 0
	}
	return float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100
}

func blockIO(stats container.StatsResponse) (uint64, uint64) {
	var read uint64
	var write uint64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			read += entry.Value
		case "write":
			write += entry.Value
		}
	}
	read += stats.StorageStats.ReadSizeBytes
	write += stats.StorageStats.WriteSizeBytes
	return read, write
}

func envKeys(values []string) []string {
	keys := make([]string, 0, len(values))
	for _, value := range values {
		key, _, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if ok && key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func logsTail(tail int) string {
	if tail <= 0 {
		tail = 200
	}
	return strconv.Itoa(tail)
}

func eventsLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func defaultSince(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return time.Now().Add(-time.Hour).Format(time.RFC3339)
}

func defaultUntil(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return time.Now().Format(time.RFC3339)
}

func diskUsageTypes(values []string) []dockertypes.DiskUsageObject {
	out := make([]dockertypes.DiskUsageObject, 0, len(values))
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "container", "containers":
			out = append(out, dockertypes.ContainerObject)
		case "image", "images":
			out = append(out, dockertypes.ImageObject)
		case "volume", "volumes":
			out = append(out, dockertypes.VolumeObject)
		case "build-cache", "build_cache", "cache":
			out = append(out, dockertypes.BuildCacheObject)
		}
	}
	return out
}

func pruneFilters(until string, labels []string, extra map[string][]string) filters.Args {
	args := filters.NewArgs()
	until = strings.TrimSpace(until)
	if until != "" {
		args.Add("until", until)
	}
	addFilterValues(args, "label", labels)
	for key, values := range extra {
		addFilterValues(args, key, values)
	}
	return args
}

func stopOptions(timeout int, signal string) container.StopOptions {
	options := container.StopOptions{Signal: strings.TrimSpace(signal)}
	if timeout != 0 {
		options.Timeout = &timeout
	}
	return options
}

func containerCreateConfig(input ContainerCreateInput) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, error) {
	image := strings.TrimSpace(input.Image)
	if image == "" {
		return nil, nil, nil, nil, fmt.Errorf("image is required")
	}
	exposedPorts, portMap, err := portBindings(input.Ports)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	platform, err := platformSpec(input.Platform)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	config := &container.Config{
		Hostname:     strings.TrimSpace(input.Hostname),
		User:         strings.TrimSpace(input.User),
		Tty:          input.TTY,
		OpenStdin:    input.OpenStdin,
		Env:          cleanStringSlice(input.Env),
		Cmd:          strslice.StrSlice(cleanStringSlice(input.Cmd)),
		Image:        image,
		WorkingDir:   strings.TrimSpace(input.Workdir),
		Entrypoint:   strslice.StrSlice(cleanStringSlice(input.Entrypoint)),
		Labels:       cloneStringMap(input.Labels),
		ExposedPorts: exposedPorts,
	}
	hostConfig := &container.HostConfig{
		Binds:        cleanStringSlice(input.Binds),
		PortBindings: portMap,
		AutoRemove:   input.AutoRemove,
		Privileged:   input.Privileged,
		Mounts:       containerMounts(input.Mounts),
	}
	if restart := restartPolicy(input.Restart); restart.Name != "" {
		hostConfig.RestartPolicy = restart
	}
	networkingConfig := &network.NetworkingConfig{}
	if networkName := strings.TrimSpace(input.Network); networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(networkName)
		if shouldAttachNetworkEndpoint(networkName) {
			networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{networkName: {}}
		}
	}
	return config, hostConfig, networkingConfig, platform, nil
}

func restartPolicy(value string) container.RestartPolicy {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return container.RestartPolicy{}
	case "none", "no":
		return container.RestartPolicy{Name: container.RestartPolicyDisabled}
	case "always":
		return container.RestartPolicy{Name: container.RestartPolicyAlways}
	case "on-failure", "on_failure":
		return container.RestartPolicy{Name: container.RestartPolicyOnFailure}
	case "unless-stopped", "unless_stopped":
		return container.RestartPolicy{Name: container.RestartPolicyUnlessStopped}
	default:
		return container.RestartPolicy{Name: container.RestartPolicyMode(strings.TrimSpace(value))}
	}
}

func shouldAttachNetworkEndpoint(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "default" || value == "bridge" || value == "host" || value == "none" {
		return false
	}
	return !strings.Contains(value, ":")
}

func containerMounts(values []MountInput) []mount.Mount {
	out := make([]mount.Mount, 0, len(values))
	for _, value := range values {
		target := strings.TrimSpace(value.Target)
		if target == "" {
			continue
		}
		mountType := mount.Type(strings.ToLower(strings.TrimSpace(value.Type)))
		if mountType == "" {
			mountType = mount.TypeBind
		}
		out = append(out, mount.Mount{
			Type:     mountType,
			Source:   strings.TrimSpace(value.Source),
			Target:   target,
			ReadOnly: value.ReadOnly,
		})
	}
	return out
}

func portBindings(values []PortInput) (nat.PortSet, nat.PortMap, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, value := range values {
		containerPort, proto := splitContainerPort(value.Container, value.Protocol)
		if containerPort == "" {
			return nil, nil, fmt.Errorf("container port is required")
		}
		port, err := nat.NewPort(proto, containerPort)
		if err != nil {
			return nil, nil, err
		}
		exposed[port] = struct{}{}
		hostPort := strings.TrimSpace(value.HostPort)
		hostIP := strings.TrimSpace(value.HostIP)
		if hostPort != "" || hostIP != "" {
			bindings[port] = append(bindings[port], nat.PortBinding{HostIP: hostIP, HostPort: hostPort})
		}
	}
	if len(exposed) == 0 {
		exposed = nil
	}
	if len(bindings) == 0 {
		bindings = nil
	}
	return exposed, bindings, nil
}

func splitContainerPort(portValue, protocol string) (string, string) {
	portValue = strings.TrimSpace(portValue)
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		protocol = "tcp"
	}
	if port, proto, ok := strings.Cut(portValue, "/"); ok {
		portValue = strings.TrimSpace(port)
		if strings.TrimSpace(proto) != "" {
			protocol = strings.TrimSpace(proto)
		}
	}
	return portValue, protocol
}

func platformSpec(value string) (*ocispec.Platform, error) {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 || len(parts) > 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("platform must be os/architecture[/variant]")
	}
	platform := &ocispec.Platform{
		OS:           strings.TrimSpace(parts[0]),
		Architecture: strings.TrimSpace(parts[1]),
	}
	if len(parts) == 3 {
		platform.Variant = strings.TrimSpace(parts[2])
	}
	return platform, nil
}

func buildArgs(values map[string]string) map[string]*string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]*string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		v := value
		out[key] = &v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func registryAuthHeader(encoded string, input RegistryAuthInput) (string, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded != "" {
		return encoded, nil
	}
	if !hasRegistryAuth(input) {
		return "", nil
	}
	return registry.EncodeAuthConfig(registryAuthConfig(input))
}

func buildAuthConfigs(input ImageBuildInput) (map[string]registry.AuthConfig, error) {
	out := map[string]registry.AuthConfig{}
	if encoded := strings.TrimSpace(input.RegistryAuth); encoded != "" {
		decoded, err := registry.DecodeAuthConfig(encoded)
		if err != nil {
			return nil, err
		}
		server := firstNonEmpty(decoded.ServerAddress, firstRegistryFromReferences(input.Tags), defaultRegistryAddress())
		out[server] = *decoded
	}
	if hasRegistryAuth(input.Auth) {
		config := registryAuthConfig(input.Auth)
		server := firstNonEmpty(config.ServerAddress, firstRegistryFromReferences(input.Tags), defaultRegistryAddress())
		config.ServerAddress = server
		out[server] = config
	}
	for key, value := range input.AuthConfigs {
		if !hasRegistryAuth(value) {
			continue
		}
		config := registryAuthConfig(value)
		server := firstNonEmpty(config.ServerAddress, strings.TrimSpace(key), defaultRegistryAddress())
		config.ServerAddress = server
		out[server] = config
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func hasRegistryAuth(input RegistryAuthInput) bool {
	return strings.TrimSpace(input.Username) != "" ||
		strings.TrimSpace(input.Password) != "" ||
		strings.TrimSpace(input.Auth) != "" ||
		strings.TrimSpace(input.Email) != "" ||
		strings.TrimSpace(input.ServerAddress) != "" ||
		strings.TrimSpace(input.IdentityToken) != "" ||
		strings.TrimSpace(input.RegistryToken) != ""
}

func registryAuthConfig(input RegistryAuthInput) registry.AuthConfig {
	return registry.AuthConfig{
		Username:      strings.TrimSpace(input.Username),
		Password:      input.Password,
		Auth:          strings.TrimSpace(input.Auth),
		Email:         strings.TrimSpace(input.Email),
		ServerAddress: strings.TrimSpace(input.ServerAddress),
		IdentityToken: strings.TrimSpace(input.IdentityToken),
		RegistryToken: strings.TrimSpace(input.RegistryToken),
	}
}

func firstRegistryFromReferences(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		host, _, ok := strings.Cut(value, "/")
		if !ok || host == "" || !strings.ContainsAny(host, ".:") {
			continue
		}
		return host
	}
	return ""
}

func defaultRegistryAddress() string {
	return "https://index.docker.io/v1/"
}

func dockerignoreExcludes(root string, dockerfile string) ([]string, error) {
	file, err := os.Open(filepath.Join(root, ".dockerignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	excludes, err := ignorefile.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading .dockerignore: %w", err)
	}
	return trimBuildFilesFromExcludes(excludes, dockerfile), nil
}

func trimBuildFilesFromExcludes(excludes []string, dockerfile string) []string {
	if keep, _ := patternmatcher.Matches(".dockerignore", excludes); keep {
		excludes = append(excludes, "!.dockerignore")
	}
	dockerfile = strings.TrimSpace(dockerfile)
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	dockerfile = filepath.ToSlash(filepath.Clean(dockerfile))
	if dockerfile != "." {
		if keep, _ := patternmatcher.Matches(dockerfile, excludes); keep {
			excludes = append(excludes, "!"+dockerfile)
		}
	}
	return excludes
}

func tarBuildContext(contextPath string, dockerfile string) (io.ReadCloser, error) {
	contextPath = strings.TrimSpace(contextPath)
	if contextPath == "" {
		return nil, fmt.Errorf("context_path is required")
	}
	root, err := filepath.Abs(contextPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("context_path must be a directory")
	}
	excludes, err := dockerignoreExcludes(root, dockerfile)
	if err != nil {
		return nil, err
	}
	return mobyarchive.TarWithOptions(root, &mobyarchive.TarOptions{ExcludePatterns: excludes})
}

func tarLocalPath(sourcePath string) (io.ReadCloser, []string, int64, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil, nil, 0, fmt.Errorf("source_path is required")
	}
	root, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, nil, 0, err
	}
	sourceInfo, err := mobyarchive.CopyInfoSourcePath(root, false)
	if err != nil {
		return nil, nil, 0, err
	}
	files, total, err := localFileSummary(root)
	if err != nil {
		return nil, nil, 0, err
	}
	body, err := mobyarchive.TarResource(sourceInfo)
	if err != nil {
		return nil, nil, 0, err
	}
	return body, files, total, nil
}

func extractTarToDirectory(reader io.Reader, destination string, overwrite bool) ([]string, int64, error) {
	root, err := filepath.Abs(strings.TrimSpace(destination))
	if err != nil {
		return nil, 0, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, 0, err
	}
	pipeReader, pipeWriter := io.Pipe()
	resultCh := make(chan archiveScanResult, 1)
	go func() {
		files, total, err := validateAndForwardTar(reader, pipeWriter, root, overwrite)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			resultCh <- archiveScanResult{err: err}
			return
		}
		err = pipeWriter.Close()
		resultCh <- archiveScanResult{files: files, bytes: total, err: err}
	}()
	untarErr := mobyarchive.Untar(pipeReader, root, &mobyarchive.TarOptions{NoOverwriteDirNonDir: !overwrite})
	result := <-resultCh
	if untarErr != nil {
		return nil, 0, untarErr
	}
	if result.err != nil {
		return nil, 0, result.err
	}
	return result.files, result.bytes, nil
}

type archiveScanResult struct {
	files []string
	bytes int64
	err   error
}

func validateAndForwardTar(reader io.Reader, writer io.Writer, root string, overwrite bool) ([]string, int64, error) {
	tarReader := tar.NewReader(reader)
	tarWriter := tar.NewWriter(writer)
	var files []string
	var total int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			if err := tarWriter.Close(); err != nil {
				return nil, 0, err
			}
			return files, total, nil
		}
		if err != nil {
			return nil, 0, err
		}
		target, err := safeExtractPath(root, header.Name)
		if err != nil {
			return nil, 0, err
		}
		if err := ensureNoSymlinkAncestor(root, target); err != nil {
			return nil, 0, err
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			if !overwrite {
				if _, err := os.Lstat(target); err == nil {
					return nil, 0, fmt.Errorf("destination file already exists: %s", target)
				} else if !os.IsNotExist(err) {
					return nil, 0, err
				}
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				return nil, 0, err
			}
			written, copyErr := io.Copy(tarWriter, tarReader)
			if copyErr != nil {
				return nil, 0, copyErr
			}
			files = append(files, target)
			total += written
		case tar.TypeSymlink:
			if !overwrite {
				if _, err := os.Lstat(target); err == nil {
					return nil, 0, fmt.Errorf("destination file already exists: %s", target)
				} else if !os.IsNotExist(err) {
					return nil, 0, err
				}
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				return nil, 0, err
			}
			files = append(files, target)
		default:
			if err := tarWriter.WriteHeader(header); err != nil {
				return nil, 0, err
			}
		}
	}
}

func safeExtractPath(root, name string) (string, error) {
	name = filepath.Clean(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	target := filepath.Join(root, name)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	return target, nil
}

func ensureNoSymlinkAncestor(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	for _, part := range parts[:max(0, len(parts)-1)] {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract through symlink ancestor: %s", current)
		}
	}
	return nil
}

func localFileSummary(root string) ([]string, int64, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, 0, err
	}
	base := filepath.Base(root)
	var files []string
	var total int64
	add := func(path string, info os.FileInfo) error {
		if !info.Mode().IsRegular() {
			return nil
		}
		name := base
		if path != root {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name = filepath.Join(base, rel)
		}
		files = append(files, filepath.ToSlash(name))
		total += info.Size()
		return nil
	}
	if !info.IsDir() {
		if err := add(root, info); err != nil {
			return nil, 0, err
		}
		return files, total, nil
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return add(path, info)
	})
	if err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

func buildProgressLimit(limit int) int {
	if limit <= 0 {
		return 500
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}

func imageIDFromEvents(events []map[string]any) string {
	for i := len(events) - 1; i >= 0; i-- {
		if aux, ok := events[i]["aux"].(map[string]any); ok {
			if id, ok := aux["ID"].(string); ok && strings.TrimSpace(id) != "" {
				return id
			}
		}
		stream, ok := events[i]["stream"].(string)
		if !ok {
			continue
		}
		line := strings.TrimSpace(stream)
		for _, prefix := range []string{"Successfully built ", "writing image sha256:"} {
			if rest, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(rest)
			}
		}
	}
	return ""
}

func progressLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func decodeProgressEvents(reader io.Reader, limit int) ([]map[string]any, error) {
	decoder := json.NewDecoder(reader)
	events := make([]map[string]any, 0, limit)
	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return events, nil
			}
			return nil, err
		}
		if len(event) > 0 {
			events = append(events, event)
		}
		if len(events) >= limit {
			break
		}
	}
	return events, nil
}

func rawInspectResult(kind, id string, data any) (RawInspectResult, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return RawInspectResult{}, err
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return RawInspectResult{}, err
	}
	return RawInspectResult{Kind: kind, ID: id, Data: object}, nil
}

type dockerConfigFile struct {
	CurrentContext string `json:"currentContext,omitempty"`
}

type dockerContextMetadataFile struct {
	Name      string                     `json:"Name,omitempty"`
	Metadata  map[string]any             `json:"Metadata,omitempty"`
	Endpoints map[string]json.RawMessage `json:"Endpoints,omitempty"`
}

func listDockerContexts() ([]DockerContext, error) {
	root := dockerConfigDir()
	current := currentDockerContext(root)
	contexts := []DockerContext{{
		Name:    "default",
		Current: current == "" || current == "default",
		Host:    firstNonEmpty(os.Getenv("DOCKER_HOST"), "default"),
	}}
	metaRoot := filepath.Join(root, "contexts", "meta")
	err := filepath.WalkDir(metaRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || entry.Name() != "meta.json" {
			return nil
		}
		ctx, err := readDockerContext(path, current)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(ctx.Name) != "" {
			contexts = append(contexts, ctx)
		}
		return nil
	})
	if err != nil && !errorsIsNotExist(err) {
		return nil, err
	}
	sort.SliceStable(contexts, func(i, j int) bool {
		if contexts[i].Current != contexts[j].Current {
			return contexts[i].Current
		}
		return contexts[i].Name < contexts[j].Name
	})
	return contexts, nil
}

func readDockerContext(path, current string) (DockerContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DockerContext{}, err
	}
	var raw dockerContextMetadataFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return DockerContext{}, err
	}
	ctx := DockerContext{
		Name:      raw.Name,
		Current:   raw.Name == current,
		Metadata:  raw.Metadata,
		Endpoints: map[string]map[string]any{},
		Path:      path,
	}
	for name, payload := range raw.Endpoints {
		var endpoint map[string]any
		if err := json.Unmarshal(payload, &endpoint); err != nil {
			continue
		}
		ctx.Endpoints[name] = endpoint
		if name == "docker" {
			if host, ok := endpoint["Host"].(string); ok {
				ctx.Host = host
			}
			if skip, ok := endpoint["SkipTLSVerify"].(bool); ok {
				ctx.SkipTLSVerify = skip
			}
			if ctx.Host != "" && !strings.HasPrefix(ctx.Host, "unix://") && !strings.HasPrefix(ctx.Host, "npipe://") {
				ctx.TLS = true
			}
		}
	}
	if len(ctx.Endpoints) == 0 {
		ctx.Endpoints = nil
	}
	if desc, ok := raw.Metadata["Description"].(string); ok {
		ctx.Description = desc
	}
	return ctx, nil
}

func dockerConfigDir() string {
	if value := strings.TrimSpace(os.Getenv("DOCKER_CONFIG")); value != "" {
		return value
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".docker")
	}
	return ".docker"
}

func currentDockerContext(root string) string {
	if value := strings.TrimSpace(os.Getenv("DOCKER_CONTEXT")); value != "" {
		return value
	}
	data, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		return "default"
	}
	var config dockerConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return "default"
	}
	return firstNonEmpty(config.CurrentContext, "default")
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func addFilterValues(args filters.Args, key string, values []string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			args.Add(key, value)
		}
	}
}

func cleanContainerNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimPrefix(strings.TrimSpace(value), "/")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func cleanDockerNoneValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<none>:<none>" && value != "<none>@<none>" {
			out = append(out, value)
		}
	}
	return out
}

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func containerPorts(values []container.Port) []string {
	out := make([]string, 0, len(values))
	for _, port := range values {
		value := fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
		if port.PublicPort > 0 {
			value = fmt.Sprintf("%s:%d->%s", port.IP, port.PublicPort, value)
		}
		out = append(out, value)
	}
	return out
}

func limitContainers(items []Container, limit int) []Container {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitImages(items []Image, limit int) []Image {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitNetworks(items []Network, limit int) []Network {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitVolumes(items []Volume, limit int) []Volume {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func shortID(id string) string {
	id = strings.TrimPrefix(strings.TrimSpace(id), "sha256:")
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
