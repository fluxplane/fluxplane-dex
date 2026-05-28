package docker

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestContainerListUsesFakeClientAndLimit(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[pluginbinding.ListResult[Container]](t, plugin, OperationContainerList, map[string]any{
		"all":   true,
		"limit": 1,
	})
	if out.Count != 1 || out.Items[0].Name != "api" {
		t.Fatalf("containers = %#v", out)
	}
}

func TestImageShowReturnsRecord(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[pluginbinding.ShowResult[Image]](t, plugin, OperationImageShow, map[string]any{
		"id": "example/api:latest",
	})
	if out.Record.Title != "example/api:latest" || out.Record.ShortID != "aaaaaaaaaaaa" || out.Record.OS != "linux" {
		t.Fatalf("image = %#v", out.Record)
	}
}

func TestContainerLogsDefaultsTail(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[ContainerLogsResult](t, plugin, OperationContainerLogs, map[string]any{
		"id": "api",
	})
	if out.Container != "api" || out.Tail != "200" || out.Stdout != "hello\n" || out.Text != "hello\n" {
		t.Fatalf("logs = %#v", out)
	}
}

func TestContainerStatsReturnsDerivedFields(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[ContainerStatsResult](t, plugin, OperationContainerStats, map[string]any{
		"id": "api",
	})
	if out.Container != "111111111111abcdef" || out.CPUPercent == 0 || out.MemoryPercent == 0 || out.NetworkRxBytes != 10 || out.BlockReadBytes != 30 {
		t.Fatalf("stats = %#v", out)
	}
}

func TestContainerTopReturnsProcesses(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[ContainerTopResult](t, plugin, OperationContainerTop, map[string]any{
		"id":   "api",
		"args": []string{"aux"},
	})
	if out.Count != 1 || len(out.Titles) != 2 || out.Processes[0][1] != "app" {
		t.Fatalf("top = %#v", out)
	}
}

func TestContainerExecReturnsOutput(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[ContainerExecResult](t, plugin, OperationContainerExec, map[string]any{
		"id":  "api",
		"cmd": []string{"echo", "hello"},
	})
	if !out.OK || out.Container != "api" || out.ExitCode != 0 || out.Stdout != "hello\n" {
		t.Fatalf("exec = %#v", out)
	}
}

func TestContainerCopyOperations(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	from := plugintest.RunOK[ContainerCopyResult](t, plugin, OperationContainerCopyFrom, map[string]any{
		"id":               "api",
		"source_path":      "/tmp/out.txt",
		"destination_path": "/tmp/docker-copy",
	})
	if !from.OK || from.Container != "api" || from.SourcePath != "/tmp/out.txt" || from.Bytes != 5 {
		t.Fatalf("copy from = %#v", from)
	}
	to := plugintest.RunOK[ContainerCopyResult](t, plugin, OperationContainerCopyTo, map[string]any{
		"id":               "api",
		"source_path":      "/tmp/in.txt",
		"destination_path": "/tmp",
	})
	if !to.OK || to.Container != "api" || to.DestinationPath != "/tmp" || to.Bytes != 5 {
		t.Fatalf("copy to = %#v", to)
	}
}

func TestContainerCreateAndRun(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	create := plugintest.RunOK[ContainerCreateResult](t, plugin, OperationContainerCreate, map[string]any{
		"image": "alpine:latest",
		"name":  "shell",
		"cmd":   []string{"sleep", "60"},
		"ports": []map[string]any{{"container": "8080/tcp", "host_port": "18080"}},
	})
	if !create.OK || create.ID != "created-shell" || create.Started {
		t.Fatalf("create = %#v", create)
	}
	run := plugintest.RunOK[ContainerCreateResult](t, plugin, OperationContainerRun, map[string]any{
		"image": "alpine:latest",
		"name":  "runner",
	})
	if !run.OK || run.ID != "created-runner" || !run.Started {
		t.Fatalf("run = %#v", run)
	}
}

func TestContainerLifecycleOperations(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	start := plugintest.RunOK[ContainerActionResult](t, plugin, OperationContainerStart, map[string]any{"id": "api"})
	if !start.OK || start.Action != "start" || start.Container != "api" {
		t.Fatalf("start = %#v", start)
	}
	stop := plugintest.RunOK[ContainerActionResult](t, plugin, OperationContainerStop, map[string]any{"id": "api", "timeout": 3, "signal": "SIGTERM"})
	if !stop.OK || stop.Action != "stop" || stop.Container != "api" {
		t.Fatalf("stop = %#v", stop)
	}
	restart := plugintest.RunOK[ContainerActionResult](t, plugin, OperationContainerRestart, map[string]any{"id": "api"})
	if !restart.OK || restart.Action != "restart" || restart.Container != "api" {
		t.Fatalf("restart = %#v", restart)
	}
	remove := plugintest.RunOK[ContainerActionResult](t, plugin, OperationContainerRemove, map[string]any{"id": "api", "force": true})
	if !remove.OK || remove.Action != "remove" || remove.Container != "api" {
		t.Fatalf("remove = %#v", remove)
	}
}

func TestImagePullAndRemove(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	pull := plugintest.RunOK[ImagePullResult](t, plugin, OperationImagePull, map[string]any{"reference": "alpine:latest", "platform": "linux/amd64"})
	if !pull.OK || pull.Reference != "alpine:latest" || pull.Platform != "linux/amd64" || pull.Count != 1 {
		t.Fatalf("pull = %#v", pull)
	}
	remove := plugintest.RunOK[ImageRemoveResult](t, plugin, OperationImageRemove, map[string]any{"id": "example/api:latest"})
	if !remove.OK || len(remove.Untagged) != 1 {
		t.Fatalf("remove = %#v", remove)
	}
}

func TestImageTagPushBuild(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	tag := plugintest.RunOK[ResourceActionResult](t, plugin, OperationImageTag, map[string]any{
		"source": "example/api:latest",
		"target": "registry.local/api:v1",
	})
	if !tag.OK || tag.ID != "example/api:latest" || tag.Action != "tag" {
		t.Fatalf("tag = %#v", tag)
	}
	push := plugintest.RunOK[ImagePushResult](t, plugin, OperationImagePush, map[string]any{
		"reference": "registry.local/api:v1",
		"platform":  "linux/amd64",
	})
	if !push.OK || push.Reference != "registry.local/api:v1" || push.Platform != "linux/amd64" || push.Count != 1 {
		t.Fatalf("push = %#v", push)
	}
	build := plugintest.RunOK[ImageBuildResult](t, plugin, OperationImageBuild, map[string]any{
		"context_path": ".",
		"tags":         []string{"example/api:test"},
		"build_args":   map[string]string{"VERSION": "test"},
	})
	if !build.OK || build.ImageID != "sha256:built" || build.Count != 1 || len(build.Tags) != 1 {
		t.Fatalf("build = %#v", build)
	}
}

func TestNetworkAndVolumeRemove(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	createdNetwork := plugintest.RunOK[ResourceActionResult](t, plugin, OperationNetworkCreate, map[string]any{"name": "app-net", "driver": "bridge"})
	if !createdNetwork.OK || createdNetwork.ID != "network-app-net" || createdNetwork.Action != "create" {
		t.Fatalf("network create = %#v", createdNetwork)
	}
	network := plugintest.RunOK[ResourceActionResult](t, plugin, OperationNetworkRemove, map[string]any{"id": "bridge"})
	if !network.OK || network.ID != "bridge" || network.Action != "remove" {
		t.Fatalf("network = %#v", network)
	}
	createdVolume := plugintest.RunOK[Volume](t, plugin, OperationVolumeCreate, map[string]any{"name": "scratch", "driver": "local"})
	if createdVolume.Name != "scratch" || createdVolume.Driver != "local" {
		t.Fatalf("volume create = %#v", createdVolume)
	}
	volume := plugintest.RunOK[ResourceActionResult](t, plugin, OperationVolumeRemove, map[string]any{"id": "cache-data", "force": true})
	if !volume.OK || volume.ID != "cache-data" || volume.Action != "remove" {
		t.Fatalf("volume = %#v", volume)
	}
}

func TestRawInspectOperations(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	for _, tc := range []struct {
		name string
		op   string
		id   string
		kind string
	}{
		{"container", OperationContainerInspectRaw, "api", "container"},
		{"image", OperationImageInspectRaw, "example/api:latest", "image"},
		{"network", OperationNetworkInspectRaw, "bridge", "network"},
		{"volume", OperationVolumeInspectRaw, "cache-data", "volume"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := plugintest.RunOK[RawInspectResult](t, plugin, tc.op, map[string]any{"id": tc.id})
			if out.Kind != tc.kind || out.ID != tc.id || len(out.Data) == 0 {
				t.Fatalf("raw = %#v", out)
			}
		})
	}
}

func TestSystemDFReturnsCounts(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[SystemDFResult](t, plugin, OperationSystemDF, map[string]any{
		"type": []string{"image", "container", "volume"},
	})
	if out.ImageCount != 1 || out.ContainerCount != 2 || out.VolumeCount != 1 || out.Volumes[0].Size != 4096 {
		t.Fatalf("df = %#v", out)
	}
}

func TestEventsReturnsBoundedResults(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.RunOK[EventsResult](t, plugin, OperationEvents, map[string]any{
		"limit": 1,
		"type":  []string{"container"},
	})
	if out.Count != 1 || len(out.Events) != 1 || out.Events[0].Action != "start" {
		t.Fatalf("events = %#v", out)
	}
}

func TestPruneOperations(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	container := plugintest.RunOK[PruneResult](t, plugin, OperationContainerPrune, map[string]any{"label": []string{"app=test"}})
	if !container.OK || container.Kind != "container" || container.Count != 1 || container.SpaceReclaimedBytes != 100 {
		t.Fatalf("container prune = %#v", container)
	}
	image := plugintest.RunOK[ImagePruneResult](t, plugin, OperationImagePrune, map[string]any{"all": true})
	if !image.OK || image.Kind != "image" || image.Count != 1 || len(image.Deleted) != 1 {
		t.Fatalf("image prune = %#v", image)
	}
	network := plugintest.RunOK[PruneResult](t, plugin, OperationNetworkPrune, map[string]any{})
	if !network.OK || network.Kind != "network" || network.Count != 1 {
		t.Fatalf("network prune = %#v", network)
	}
	volume := plugintest.RunOK[PruneResult](t, plugin, OperationVolumePrune, map[string]any{})
	if !volume.OK || volume.Kind != "volume" || volume.Count != 1 || volume.SpaceReclaimedBytes != 300 {
		t.Fatalf("volume prune = %#v", volume)
	}
	buildCache := plugintest.RunOK[PruneResult](t, plugin, OperationBuildCachePrune, map[string]any{"all": true})
	if !buildCache.OK || buildCache.Kind != "build_cache" || buildCache.Count != 1 || buildCache.SpaceReclaimedBytes != 400 {
		t.Fatalf("build cache prune = %#v", buildCache)
	}
	system := plugintest.RunOK[SystemPruneResult](t, plugin, OperationSystemPrune, map[string]any{"all": true, "volumes": true})
	if !system.OK || system.TotalCount != 5 || system.TotalBytes != 1000 || system.Volumes == nil {
		t.Fatalf("system prune = %#v", system)
	}
}

func TestDockerContexts(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	list := plugintest.RunOK[pluginbinding.ListResult[DockerContext]](t, plugin, OperationContextList, map[string]any{})
	if list.Count != 2 || !list.Items[0].Current || list.Items[0].Name != "default" {
		t.Fatalf("contexts = %#v", list)
	}
	show := plugintest.RunOK[pluginbinding.ShowResult[DockerContext]](t, plugin, OperationContextShow, map[string]any{"name": "remote"})
	if show.Record.Name != "remote" || show.Record.Host != "tcp://remote.example:2376" {
		t.Fatalf("context = %#v", show.Record)
	}
}

func TestContainerSearchFiltersByQuery(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.DatasourceSearchOK[ContainerSearchResult](t, plugin, map[string]any{
		"entity": EntityContainer,
		"query":  "worker",
	})
	if out.Count != 1 || out.Records[0].Name != "worker" {
		t.Fatalf("search = %#v", out)
	}
}

func TestContainerSearchAppliesLimitAfterFiltering(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.DatasourceSearchOK[ContainerSearchResult](t, plugin, map[string]any{
		"entity": EntityContainer,
		"query":  "worker",
		"limit":  1,
	})
	if out.Count != 1 || out.Records[0].Name != "worker" {
		t.Fatalf("search = %#v", out)
	}
}

func TestLookupMatchesImagesAndVolumes(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{
		"text":  "check example/api:latest and cache-data",
		"limit": 5,
	})
	if len(out.Matches) < 2 {
		t.Fatalf("matches = %#v", out.Matches)
	}
	seen := map[string]bool{}
	for _, match := range out.Matches {
		seen[match.Entity] = true
	}
	if !seen[EntityImage] || !seen[EntityVolume] {
		t.Fatalf("entities = %#v matches = %#v", seen, out.Matches)
	}
}

func TestLookupAppliesLimitAfterScoring(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{
		"text":  "cache-data",
		"limit": 1,
	})
	if len(out.Matches) != 1 || out.Matches[0].Entity != EntityVolume || out.Matches[0].ID != "cache-data" {
		t.Fatalf("matches = %#v", out.Matches)
	}
}

func TestDatasourceGetNotFoundUsesPluginError(t *testing.T) {
	plugin := NewPluginWithService(fakeService())
	err := plugintest.DatasourceError(t, plugin, protocol.CommandDatasourcesGet, map[string]any{
		"entity": EntityNetwork,
		"id":     "missing",
	})
	if err.Code != "docker" {
		t.Fatalf("error = %#v", err)
	}
}

func TestNormalizeRecordsRejectEmptyIDs(t *testing.T) {
	if _, ok := normalizeContainerRecord(pluginbinding.DatasourceSource{Plugin: PluginName}, Container{}); ok {
		t.Fatal("empty container normalized")
	}
	if _, ok := normalizeImageRecord(pluginbinding.DatasourceSource{Plugin: PluginName}, Image{}); ok {
		t.Fatal("empty image normalized")
	}
	if _, ok := normalizeNetworkRecord(pluginbinding.DatasourceSource{Plugin: PluginName}, Network{}); ok {
		t.Fatal("empty network normalized")
	}
	if _, ok := normalizeVolumeRecord(pluginbinding.DatasourceSource{Plugin: PluginName}, Volume{}); ok {
		t.Fatal("empty volume normalized")
	}
}

func fakeService() Service {
	client := &fakeClient{
		info: DockerInfo{Name: "local", ServerVersion: "29.0.0", Containers: 2, Images: 1},
		containers: []Container{
			{ID: "111111111111abcdef", ShortID: "111111111111", Name: "api", Names: []string{"api"}, Image: "example/api:latest", State: "running", Status: "Up 1 hour", Health: "healthy", EnvKeys: []string{"APP_ENV"}},
			{ID: "222222222222abcdef", ShortID: "222222222222", Name: "worker", Names: []string{"worker"}, Image: "example/worker:latest", State: "exited", Status: "Exited"},
		},
		images: []Image{
			{ID: "sha256:aaaaaaaaaaaabbbb", ShortID: "aaaaaaaaaaaa", Title: "example/api:latest", RepoTags: []string{"example/api:latest"}, Size: 123, OS: "linux", Architecture: "amd64"},
		},
		networks: []Network{
			{ID: "333333333333abcdef", ShortID: "333333333333", Name: "bridge", Driver: "bridge", Scope: "local"},
		},
		volumes: []Volume{
			{Name: "cache-data", Driver: "local", Mountpoint: "/var/lib/docker/volumes/cache-data/_data", Scope: "local", Size: 4096, RefCount: 1},
		},
		logs: ContainerLogsResult{Container: "api", Tail: "200", Stdout: "hello\n", Text: "hello\n"},
		stats: ContainerStatsResult{
			Container:        "111111111111abcdef",
			Name:             "api",
			CPUPercent:       12.5,
			MemoryUsageBytes: 128,
			MemoryLimitBytes: 1024,
			MemoryPercent:    12.5,
			NetworkRxBytes:   10,
			NetworkTxBytes:   20,
			BlockReadBytes:   30,
			BlockWriteBytes:  40,
		},
		top: ContainerTopResult{Container: "api", Titles: []string{"PID", "CMD"}, Processes: [][]string{{"1", "app"}}, Count: 1},
		contexts: []DockerContext{
			{Name: "default", Current: true, Host: "default"},
			{Name: "remote", Host: "tcp://remote.example:2376", TLS: true},
		},
		events: []Event{
			{Type: "container", Action: "start", ActorID: "111111111111abcdef", Attributes: map[string]string{"name": "api"}},
			{Type: "image", Action: "pull", ActorID: "sha256:aaaaaaaaaaaabbbb"},
		},
	}
	return Service{ClientFactory: func() (Client, error) { return client, nil }}
}

type fakeClient struct {
	info       DockerInfo
	containers []Container
	images     []Image
	networks   []Network
	volumes    []Volume
	logs       ContainerLogsResult
	stats      ContainerStatsResult
	top        ContainerTopResult
	contexts   []DockerContext
	events     []Event
}

func (f *fakeClient) Close() error {
	return nil
}

func (f *fakeClient) Info(context.Context) (DockerInfo, error) {
	return f.info, nil
}

func (f *fakeClient) ListContainers(_ context.Context, input ContainerListInput) ([]Container, error) {
	return limitContainers(append([]Container(nil), f.containers...), input.Limit), nil
}

func (f *fakeClient) InspectContainer(_ context.Context, id string) (Container, error) {
	for _, item := range f.containers {
		if item.ID == id || item.ShortID == id || item.Name == id {
			return item, nil
		}
	}
	return Container{}, errors.New("container not found")
}

func (f *fakeClient) ContainerLogs(_ context.Context, input ContainerLogsInput) (ContainerLogsResult, error) {
	out := f.logs
	out.Container = input.ID
	if input.Tail > 0 {
		out.Tail = strconv.Itoa(input.Tail)
	}
	return out, nil
}

func (f *fakeClient) ContainerStats(context.Context, ContainerStatsInput) (ContainerStatsResult, error) {
	return f.stats, nil
}

func (f *fakeClient) ContainerTop(context.Context, ContainerTopInput) (ContainerTopResult, error) {
	return f.top, nil
}

func (f *fakeClient) ContainerExec(_ context.Context, input ContainerExecInput) (ContainerExecResult, error) {
	if input.ID == "" {
		return ContainerExecResult{}, errors.New("container id or name is required")
	}
	if len(input.Cmd) == 0 {
		return ContainerExecResult{}, errors.New("cmd is required")
	}
	return ContainerExecResult{Container: input.ID, ExecID: "exec-1", ExitCode: 0, Stdout: "hello\n", Text: "hello\n", OK: true}, nil
}

func (f *fakeClient) ContainerCopyFrom(_ context.Context, input ContainerCopyFromInput) (ContainerCopyResult, error) {
	if input.ID == "" {
		return ContainerCopyResult{}, errors.New("container id or name is required")
	}
	if input.SourcePath == "" {
		return ContainerCopyResult{}, errors.New("source_path is required")
	}
	if input.DestinationPath == "" {
		return ContainerCopyResult{}, errors.New("destination_path is required")
	}
	return ContainerCopyResult{Container: input.ID, SourcePath: input.SourcePath, DestinationPath: input.DestinationPath, Files: []string{"out.txt"}, Bytes: 5, OK: true}, nil
}

func (f *fakeClient) ContainerCopyTo(_ context.Context, input ContainerCopyToInput) (ContainerCopyResult, error) {
	if input.ID == "" {
		return ContainerCopyResult{}, errors.New("container id or name is required")
	}
	if input.SourcePath == "" {
		return ContainerCopyResult{}, errors.New("source_path is required")
	}
	if input.DestinationPath == "" {
		return ContainerCopyResult{}, errors.New("destination_path is required")
	}
	return ContainerCopyResult{Container: input.ID, SourcePath: input.SourcePath, DestinationPath: input.DestinationPath, Files: []string{"in.txt"}, Bytes: 5, OK: true}, nil
}

func (f *fakeClient) ContainerCreate(_ context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	if input.Image == "" {
		return ContainerCreateResult{}, errors.New("image is required")
	}
	name := input.Name
	if name == "" {
		name = "container"
	}
	return ContainerCreateResult{ID: "created-" + name, Name: input.Name, Image: input.Image, OK: true}, nil
}

func (f *fakeClient) ContainerRun(ctx context.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	out, err := f.ContainerCreate(ctx, input)
	if err != nil {
		return ContainerCreateResult{}, err
	}
	out.Started = true
	return out, nil
}

func (f *fakeClient) ContainerStart(_ context.Context, input ContainerStartInput) (ContainerActionResult, error) {
	if input.ID == "" {
		return ContainerActionResult{}, errors.New("container id or name is required")
	}
	return ContainerActionResult{Container: input.ID, Action: "start", OK: true}, nil
}

func (f *fakeClient) ContainerStop(_ context.Context, input ContainerStopInput) (ContainerActionResult, error) {
	if input.ID == "" {
		return ContainerActionResult{}, errors.New("container id or name is required")
	}
	return ContainerActionResult{Container: input.ID, Action: "stop", OK: true}, nil
}

func (f *fakeClient) ContainerRestart(_ context.Context, input ContainerRestartInput) (ContainerActionResult, error) {
	if input.ID == "" {
		return ContainerActionResult{}, errors.New("container id or name is required")
	}
	return ContainerActionResult{Container: input.ID, Action: "restart", OK: true}, nil
}

func (f *fakeClient) ContainerRemove(_ context.Context, input ContainerRemoveInput) (ContainerActionResult, error) {
	if input.ID == "" {
		return ContainerActionResult{}, errors.New("container id or name is required")
	}
	return ContainerActionResult{Container: input.ID, Action: "remove", OK: true}, nil
}

func (f *fakeClient) ContainerInspectRaw(_ context.Context, input RawInspectInput) (RawInspectResult, error) {
	return RawInspectResult{Kind: "container", ID: input.ID, Data: map[string]any{"id": input.ID, "name": "api"}}, nil
}

func (f *fakeClient) ContainerPrune(context.Context, PruneInput) (PruneResult, error) {
	return PruneResult{Kind: "container", Deleted: []string{"old-container"}, SpaceReclaimedBytes: 100, Count: 1, OK: true}, nil
}

func (f *fakeClient) ListImages(_ context.Context, input ImageListInput) ([]Image, error) {
	return limitImages(append([]Image(nil), f.images...), input.Limit), nil
}

func (f *fakeClient) InspectImage(_ context.Context, id string) (Image, error) {
	for _, item := range f.images {
		if item.ID == id || item.ShortID == id || item.Title == id || containsString(item.RepoTags, id) || containsString(item.RepoDigests, id) {
			return item, nil
		}
	}
	return Image{}, errors.New("image not found")
}

func (f *fakeClient) ImagePull(_ context.Context, input ImagePullInput) (ImagePullResult, error) {
	if input.Reference == "" {
		return ImagePullResult{}, errors.New("image reference is required")
	}
	return ImagePullResult{Reference: input.Reference, Platform: input.Platform, Events: []map[string]any{{"status": "pulled"}}, Count: 1, OK: true}, nil
}

func (f *fakeClient) ImageTag(_ context.Context, input ImageTagInput) (ResourceActionResult, error) {
	if input.Source == "" {
		return ResourceActionResult{}, errors.New("source image is required")
	}
	if input.Target == "" {
		return ResourceActionResult{}, errors.New("target image reference is required")
	}
	return ResourceActionResult{ID: input.Source, Action: "tag", OK: true}, nil
}

func (f *fakeClient) ImagePush(_ context.Context, input ImagePushInput) (ImagePushResult, error) {
	if input.Reference == "" {
		return ImagePushResult{}, errors.New("image reference is required")
	}
	return ImagePushResult{Reference: input.Reference, Platform: input.Platform, Events: []map[string]any{{"status": "pushed"}}, Count: 1, OK: true}, nil
}

func (f *fakeClient) ImageBuild(_ context.Context, input ImageBuildInput) (ImageBuildResult, error) {
	if input.ContextPath == "" {
		return ImageBuildResult{}, errors.New("context_path is required")
	}
	return ImageBuildResult{ContextPath: input.ContextPath, Tags: append([]string(nil), input.Tags...), ImageID: "sha256:built", Events: []map[string]any{{"aux": map[string]any{"ID": "sha256:built"}}}, Count: 1, OK: true}, nil
}

func (f *fakeClient) ImageRemove(_ context.Context, input ImageRemoveInput) (ImageRemoveResult, error) {
	if input.ID == "" {
		return ImageRemoveResult{}, errors.New("image id, digest, or reference is required")
	}
	return ImageRemoveResult{ID: input.ID, Untagged: []string{input.ID}, OK: true}, nil
}

func (f *fakeClient) ImageInspectRaw(_ context.Context, input RawInspectInput) (RawInspectResult, error) {
	return RawInspectResult{Kind: "image", ID: input.ID, Data: map[string]any{"id": input.ID, "os": "linux"}}, nil
}

func (f *fakeClient) ImagePrune(context.Context, ImagePruneInput) (ImagePruneResult, error) {
	return ImagePruneResult{Kind: "image", Deleted: []string{"sha256:old"}, SpaceReclaimedBytes: 200, Count: 1, OK: true}, nil
}

func (f *fakeClient) ListNetworks(_ context.Context, input NetworkListInput) ([]Network, error) {
	return limitNetworks(append([]Network(nil), f.networks...), input.Limit), nil
}

func (f *fakeClient) InspectNetwork(_ context.Context, id string) (Network, error) {
	for _, item := range f.networks {
		if item.ID == id || item.ShortID == id || item.Name == id {
			return item, nil
		}
	}
	return Network{}, errors.New("network not found")
}

func (f *fakeClient) NetworkCreate(_ context.Context, input NetworkCreateInput) (ResourceActionResult, error) {
	if input.Name == "" {
		return ResourceActionResult{}, errors.New("network name is required")
	}
	return ResourceActionResult{ID: "network-" + input.Name, Action: "create", OK: true}, nil
}

func (f *fakeClient) NetworkRemove(_ context.Context, input NetworkRemoveInput) (ResourceActionResult, error) {
	if input.ID == "" {
		return ResourceActionResult{}, errors.New("network id or name is required")
	}
	return ResourceActionResult{ID: input.ID, Action: "remove", OK: true}, nil
}

func (f *fakeClient) NetworkInspectRaw(_ context.Context, input RawInspectInput) (RawInspectResult, error) {
	return RawInspectResult{Kind: "network", ID: input.ID, Data: map[string]any{"id": input.ID, "name": "bridge"}}, nil
}

func (f *fakeClient) NetworkPrune(context.Context, PruneInput) (PruneResult, error) {
	return PruneResult{Kind: "network", Deleted: []string{"old-network"}, Count: 1, OK: true}, nil
}

func (f *fakeClient) SystemDF(context.Context, SystemDFInput) (SystemDFResult, error) {
	return SystemDFResult{
		Images:         append([]Image(nil), f.images...),
		Containers:     append([]Container(nil), f.containers...),
		Volumes:        append([]Volume(nil), f.volumes...),
		ImageCount:     len(f.images),
		ContainerCount: len(f.containers),
		VolumeCount:    len(f.volumes),
	}, nil
}

func (f *fakeClient) SystemPrune(ctx context.Context, input SystemPruneInput) (SystemPruneResult, error) {
	containers, _ := f.ContainerPrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
	networks, _ := f.NetworkPrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
	images, _ := f.ImagePrune(ctx, ImagePruneInput{All: input.All, Until: input.Until, Label: input.Label})
	buildCache, _ := f.BuildCachePrune(ctx, BuildCachePruneInput{All: input.All, Until: input.Until, Label: input.Label})
	result := SystemPruneResult{Containers: containers, Networks: networks, Images: images, BuildCache: buildCache, TotalCount: 4, TotalBytes: 700, OK: true}
	if input.Volumes {
		volumes, _ := f.VolumePrune(ctx, PruneInput{Until: input.Until, Label: input.Label})
		result.Volumes = &volumes
		result.TotalCount += volumes.Count
		result.TotalBytes += volumes.SpaceReclaimedBytes
	}
	return result, nil
}

func (f *fakeClient) Events(_ context.Context, input EventsInput) (EventsResult, error) {
	limit := input.Limit
	if limit <= 0 || limit > len(f.events) {
		limit = len(f.events)
	}
	events := append([]Event(nil), f.events[:limit]...)
	return EventsResult{Events: events, Count: len(events)}, nil
}

func (f *fakeClient) ListVolumes(_ context.Context, input VolumeListInput) ([]Volume, error) {
	return limitVolumes(append([]Volume(nil), f.volumes...), input.Limit), nil
}

func (f *fakeClient) InspectVolume(_ context.Context, id string) (Volume, error) {
	for _, item := range f.volumes {
		if item.Name == id {
			return item, nil
		}
	}
	return Volume{}, errors.New("volume not found")
}

func (f *fakeClient) VolumeCreate(_ context.Context, input VolumeCreateInput) (Volume, error) {
	name := input.Name
	if name == "" {
		name = "generated-volume"
	}
	driver := input.Driver
	if driver == "" {
		driver = "local"
	}
	return Volume{Name: name, Driver: driver, Labels: cloneStringMap(input.Labels), Options: cloneStringMap(input.DriverOpts)}, nil
}

func (f *fakeClient) VolumeRemove(_ context.Context, input VolumeRemoveInput) (ResourceActionResult, error) {
	if input.ID == "" {
		return ResourceActionResult{}, errors.New("volume name is required")
	}
	return ResourceActionResult{ID: input.ID, Action: "remove", OK: true}, nil
}

func (f *fakeClient) VolumeInspectRaw(_ context.Context, input RawInspectInput) (RawInspectResult, error) {
	return RawInspectResult{Kind: "volume", ID: input.ID, Data: map[string]any{"name": input.ID, "driver": "local"}}, nil
}

func (f *fakeClient) VolumePrune(context.Context, PruneInput) (PruneResult, error) {
	return PruneResult{Kind: "volume", Deleted: []string{"old-volume"}, SpaceReclaimedBytes: 300, Count: 1, OK: true}, nil
}

func (f *fakeClient) BuildCachePrune(context.Context, BuildCachePruneInput) (PruneResult, error) {
	return PruneResult{Kind: "build_cache", Deleted: []string{"old-cache"}, SpaceReclaimedBytes: 400, Count: 1, OK: true}, nil
}

func (f *fakeClient) ContextList(context.Context, ContextListInput) ([]DockerContext, error) {
	return append([]DockerContext(nil), f.contexts...), nil
}

func (f *fakeClient) ContextShow(_ context.Context, input ContextShowInput) (DockerContext, error) {
	name := input.Name
	if name == "" {
		for _, item := range f.contexts {
			if item.Current {
				return item, nil
			}
		}
	}
	for _, item := range f.contexts {
		if item.Name == name {
			return item, nil
		}
	}
	return DockerContext{}, errors.New("context not found")
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
