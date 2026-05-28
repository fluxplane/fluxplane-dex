package docker

import (
	"context"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

type InfoInput struct{}

type ShowInput struct {
	ID string `json:"id,omitempty" jsonschema:"required,description=Object ID, name, digest, or reference."`
}

type ContainerLogsInput struct {
	ID         string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Tail       int    `json:"tail,omitempty" jsonschema:"description=Number of log lines to return. Defaults to 200."`
	Since      string `json:"since,omitempty" jsonschema:"description=Show logs since timestamp or duration supported by Docker."`
	Until      string `json:"until,omitempty" jsonschema:"description=Show logs until timestamp or duration supported by Docker."`
	Timestamps bool   `json:"timestamps,omitempty" jsonschema:"description=Include log timestamps."`
}

type ContainerStatsInput struct {
	ID string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
}

type ContainerTopInput struct {
	ID   string   `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Args []string `json:"args,omitempty" jsonschema:"description=Optional ps arguments."`
}

type ContainerExecInput struct {
	ID            string   `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Cmd           []string `json:"cmd,omitempty" jsonschema:"required,description=Command argv to execute."`
	Env           []string `json:"env,omitempty" jsonschema:"description=Environment variables in KEY=value form."`
	User          string   `json:"user,omitempty" jsonschema:"description=User to run the command as."`
	Workdir       string   `json:"workdir,omitempty" jsonschema:"description=Working directory inside the container."`
	Privileged    bool     `json:"privileged,omitempty" jsonschema:"description=Run exec in privileged mode."`
	TTY           bool     `json:"tty,omitempty" jsonschema:"description=Allocate a TTY."`
	Detach        bool     `json:"detach,omitempty" jsonschema:"description=Start command and return without waiting for output."`
	TimeoutSecond int      `json:"timeout_second,omitempty" jsonschema:"description=Maximum seconds to wait for attached exec output."`
}

type ContainerCopyFromInput struct {
	ID              string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	SourcePath      string `json:"source_path,omitempty" jsonschema:"required,description=Path inside the container to copy."`
	DestinationPath string `json:"destination_path,omitempty" jsonschema:"required,description=Local destination directory."`
	Overwrite       bool   `json:"overwrite,omitempty" jsonschema:"description=Allow overwriting existing local files."`
}

type ContainerCopyToInput struct {
	ID                        string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	SourcePath                string `json:"source_path,omitempty" jsonschema:"required,description=Local file or directory to copy."`
	DestinationPath           string `json:"destination_path,omitempty" jsonschema:"required,description=Destination path inside the container."`
	AllowOverwriteDirWithFile bool   `json:"allow_overwrite_dir_with_file,omitempty" jsonschema:"description=Allow replacing a container directory with a local file."`
	CopyUIDGID                bool   `json:"copy_uid_gid,omitempty" jsonschema:"description=Copy local UID and GID metadata into the container."`
}

type ContainerCreateInput struct {
	Image      string            `json:"image,omitempty" jsonschema:"required,description=Image to create the container from."`
	Name       string            `json:"name,omitempty" jsonschema:"description=Container name."`
	Cmd        []string          `json:"cmd,omitempty" jsonschema:"description=Container command argv."`
	Entrypoint []string          `json:"entrypoint,omitempty" jsonschema:"description=Container entrypoint argv."`
	Env        []string          `json:"env,omitempty" jsonschema:"description=Environment variables in KEY=value form."`
	Labels     map[string]string `json:"labels,omitempty" jsonschema:"description=Container labels."`
	Workdir    string            `json:"workdir,omitempty" jsonschema:"description=Working directory inside the container."`
	User       string            `json:"user,omitempty" jsonschema:"description=User to run as."`
	Hostname   string            `json:"hostname,omitempty" jsonschema:"description=Container hostname."`
	Network    string            `json:"network,omitempty" jsonschema:"description=Network mode or network name."`
	Restart    string            `json:"restart,omitempty" jsonschema:"description=Restart policy: no, always, on-failure, unless-stopped."`
	AutoRemove bool              `json:"auto_remove,omitempty" jsonschema:"description=Automatically remove the container when it exits."`
	TTY        bool              `json:"tty,omitempty" jsonschema:"description=Allocate a TTY."`
	OpenStdin  bool              `json:"open_stdin,omitempty" jsonschema:"description=Keep stdin open."`
	Privileged bool              `json:"privileged,omitempty" jsonschema:"description=Run container in privileged mode."`
	Binds      []string          `json:"binds,omitempty" jsonschema:"description=Bind mounts in Docker -v syntax."`
	Mounts     []MountInput      `json:"mounts,omitempty" jsonschema:"description=Structured mounts."`
	Ports      []PortInput       `json:"ports,omitempty" jsonschema:"description=Port bindings."`
	Platform   string            `json:"platform,omitempty" jsonschema:"description=Image platform, for example linux/amd64."`
}

type MountInput struct {
	Type     string `json:"type,omitempty" jsonschema:"description=Mount type: bind, volume, tmpfs."`
	Source   string `json:"source,omitempty" jsonschema:"description=Mount source."`
	Target   string `json:"target,omitempty" jsonschema:"description=Mount target path in the container."`
	ReadOnly bool   `json:"read_only,omitempty" jsonschema:"description=Mount read-only."`
}

type PortInput struct {
	Container string `json:"container,omitempty" jsonschema:"required,description=Container port, for example 8080 or 8080/tcp."`
	HostIP    string `json:"host_ip,omitempty" jsonschema:"description=Host IP to bind."`
	HostPort  string `json:"host_port,omitempty" jsonschema:"description=Host port to bind."`
	Protocol  string `json:"protocol,omitempty" jsonschema:"description=Protocol: tcp or udp."`
}

type ContainerStartInput struct {
	ID string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
}

type ContainerStopInput struct {
	ID      string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Timeout int    `json:"timeout,omitempty" jsonschema:"description=Seconds to wait before killing. Zero uses Docker default; -1 waits indefinitely."`
	Signal  string `json:"signal,omitempty" jsonschema:"description=Signal to send before killing, for example SIGTERM."`
}

type ContainerRestartInput struct {
	ID      string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Timeout int    `json:"timeout,omitempty" jsonschema:"description=Seconds to wait before killing during restart. Zero uses Docker default; -1 waits indefinitely."`
	Signal  string `json:"signal,omitempty" jsonschema:"description=Signal to send before killing, for example SIGTERM."`
}

type ContainerRemoveInput struct {
	ID      string `json:"id,omitempty" jsonschema:"required,description=Container ID or name."`
	Force   bool   `json:"force,omitempty" jsonschema:"description=Force removal of a running container."`
	Volumes bool   `json:"volumes,omitempty" jsonschema:"description=Remove anonymous volumes associated with the container."`
}

type RawInspectInput struct {
	ID string `json:"id,omitempty" jsonschema:"required,description=Object ID, name, digest, or reference."`
}

type PruneInput struct {
	Until string   `json:"until,omitempty" jsonschema:"description=Prune objects created before this timestamp or duration supported by Docker."`
	Label []string `json:"label,omitempty" jsonschema:"description=Only prune objects with these labels."`
}

type ContainerListInput struct {
	All    bool     `json:"all,omitempty" jsonschema:"description=Include stopped containers."`
	Limit  int      `json:"limit,omitempty" jsonschema:"description=Maximum containers to return."`
	Status []string `json:"status,omitempty" jsonschema:"description=Container status filters."`
	Name   []string `json:"name,omitempty" jsonschema:"description=Container name filters."`
	Label  []string `json:"label,omitempty" jsonschema:"description=Container label filters."`
}

type ImageListInput struct {
	All       bool     `json:"all,omitempty" jsonschema:"description=Include intermediate images."`
	Limit     int      `json:"limit,omitempty" jsonschema:"description=Maximum images to return."`
	Reference []string `json:"reference,omitempty" jsonschema:"description=Image reference filters."`
	Label     []string `json:"label,omitempty" jsonschema:"description=Image label filters."`
}

type ImagePullInput struct {
	Reference    string            `json:"reference,omitempty" jsonschema:"required,description=Image reference to pull."`
	Platform     string            `json:"platform,omitempty" jsonschema:"description=Optional platform, for example linux/amd64."`
	RegistryAuth string            `json:"registry_auth,omitempty" jsonschema:"description=Base64-encoded Docker registry auth header."`
	Auth         RegistryAuthInput `json:"auth,omitempty" jsonschema:"description=Registry auth fields to encode for this request."`
	Limit        int               `json:"limit,omitempty" jsonschema:"description=Maximum pull progress events to keep. Defaults to 200."`
}

type RegistryAuthInput struct {
	Username      string `json:"username,omitempty" jsonschema:"description=Registry username."`
	Password      string `json:"password,omitempty" jsonschema:"description=Registry password."`
	Auth          string `json:"auth,omitempty" jsonschema:"description=Base64 username:password auth payload."`
	Email         string `json:"email,omitempty" jsonschema:"description=Registry email."`
	ServerAddress string `json:"server_address,omitempty" jsonschema:"description=Registry server address."`
	IdentityToken string `json:"identity_token,omitempty" jsonschema:"description=Registry identity token."`
	RegistryToken string `json:"registry_token,omitempty" jsonschema:"description=Registry bearer token."`
}

type ImageTagInput struct {
	Source string `json:"source,omitempty" jsonschema:"required,description=Source image ID or reference."`
	Target string `json:"target,omitempty" jsonschema:"required,description=Target image reference."`
}

type ImagePushInput struct {
	Reference    string            `json:"reference,omitempty" jsonschema:"required,description=Image reference to push."`
	Platform     string            `json:"platform,omitempty" jsonschema:"description=Optional platform, for example linux/amd64."`
	RegistryAuth string            `json:"registry_auth,omitempty" jsonschema:"description=Base64-encoded Docker registry auth header."`
	Auth         RegistryAuthInput `json:"auth,omitempty" jsonschema:"description=Registry auth fields to encode for this request."`
	Limit        int               `json:"limit,omitempty" jsonschema:"description=Maximum push progress events to keep. Defaults to 200."`
}

type ImageBuildInput struct {
	ContextPath  string                       `json:"context_path,omitempty" jsonschema:"required,description=Local build context directory."`
	Dockerfile   string                       `json:"dockerfile,omitempty" jsonschema:"description=Dockerfile path relative to context. Defaults to Dockerfile."`
	Tags         []string                     `json:"tags,omitempty" jsonschema:"description=Image tags to apply."`
	Target       string                       `json:"target,omitempty" jsonschema:"description=Build target stage."`
	BuildArgs    map[string]string            `json:"build_args,omitempty" jsonschema:"description=Build arguments."`
	Labels       map[string]string            `json:"labels,omitempty" jsonschema:"description=Image labels."`
	Platform     string                       `json:"platform,omitempty" jsonschema:"description=Build platform."`
	Pull         bool                         `json:"pull,omitempty" jsonschema:"description=Always attempt to pull parent images."`
	NoCache      bool                         `json:"no_cache,omitempty" jsonschema:"description=Do not use cache."`
	Network      string                       `json:"network,omitempty" jsonschema:"description=Network mode for RUN instructions."`
	RegistryAuth string                       `json:"registry_auth,omitempty" jsonschema:"description=Base64-encoded Docker registry auth header to decode into auth configs."`
	Auth         RegistryAuthInput            `json:"auth,omitempty" jsonschema:"description=Registry auth fields to include in build auth configs."`
	AuthConfigs  map[string]RegistryAuthInput `json:"auth_configs,omitempty" jsonschema:"description=Registry auth configs keyed by registry host."`
	Limit        int                          `json:"limit,omitempty" jsonschema:"description=Maximum build progress events to keep. Defaults to 500."`
}

type ImageRemoveInput struct {
	ID            string `json:"id,omitempty" jsonschema:"required,description=Image ID, digest, or reference."`
	Force         bool   `json:"force,omitempty" jsonschema:"description=Force image removal."`
	PruneChildren bool   `json:"prune_children,omitempty" jsonschema:"description=Delete untagged parent images."`
}

type ImagePruneInput struct {
	All   bool     `json:"all,omitempty" jsonschema:"description=Prune all unused images, not only dangling images."`
	Until string   `json:"until,omitempty" jsonschema:"description=Prune images created before this timestamp or duration supported by Docker."`
	Label []string `json:"label,omitempty" jsonschema:"description=Only prune images with these labels."`
}

type NetworkListInput struct {
	Limit int      `json:"limit,omitempty" jsonschema:"description=Maximum networks to return."`
	Name  []string `json:"name,omitempty" jsonschema:"description=Network name filters."`
	Label []string `json:"label,omitempty" jsonschema:"description=Network label filters."`
}

type NetworkCreateInput struct {
	Name       string            `json:"name,omitempty" jsonschema:"required,description=Network name."`
	Driver     string            `json:"driver,omitempty" jsonschema:"description=Network driver. Defaults to Docker daemon default."`
	Scope      string            `json:"scope,omitempty" jsonschema:"description=Network scope."`
	Internal   bool              `json:"internal,omitempty" jsonschema:"description=Restrict external access to the network."`
	Attachable bool              `json:"attachable,omitempty" jsonschema:"description=Allow standalone containers to attach to swarm-scoped network."`
	Ingress    bool              `json:"ingress,omitempty" jsonschema:"description=Create ingress routing mesh network."`
	EnableIPv4 *bool             `json:"enable_ipv4,omitempty" jsonschema:"description=Enable IPv4."`
	EnableIPv6 *bool             `json:"enable_ipv6,omitempty" jsonschema:"description=Enable IPv6."`
	Options    map[string]string `json:"options,omitempty" jsonschema:"description=Driver options."`
	Labels     map[string]string `json:"labels,omitempty" jsonschema:"description=Network labels."`
}

type NetworkRemoveInput struct {
	ID string `json:"id,omitempty" jsonschema:"required,description=Network ID or name."`
}

type VolumeListInput struct {
	Limit int      `json:"limit,omitempty" jsonschema:"description=Maximum volumes to return."`
	Name  []string `json:"name,omitempty" jsonschema:"description=Volume name filters."`
	Label []string `json:"label,omitempty" jsonschema:"description=Volume label filters."`
}

type VolumeCreateInput struct {
	Name       string            `json:"name,omitempty" jsonschema:"description=Volume name. Empty lets Docker generate one."`
	Driver     string            `json:"driver,omitempty" jsonschema:"description=Volume driver."`
	DriverOpts map[string]string `json:"driver_opts,omitempty" jsonschema:"description=Volume driver options."`
	Labels     map[string]string `json:"labels,omitempty" jsonschema:"description=Volume labels."`
}

type VolumeRemoveInput struct {
	ID    string `json:"id,omitempty" jsonschema:"required,description=Volume name."`
	Force bool   `json:"force,omitempty" jsonschema:"description=Force volume removal."`
}

type SystemDFInput struct {
	Type []string `json:"type,omitempty" jsonschema:"description=Object types to include: image, container, volume, build-cache."`
}

type SystemPruneInput struct {
	All     bool     `json:"all,omitempty" jsonschema:"description=Prune all unused images, not only dangling images."`
	Volumes bool     `json:"volumes,omitempty" jsonschema:"description=Also prune unused volumes."`
	Until   string   `json:"until,omitempty" jsonschema:"description=Prune objects created before this timestamp or duration supported by Docker."`
	Label   []string `json:"label,omitempty" jsonschema:"description=Only prune objects with these labels."`
}

type BuildCachePruneInput struct {
	All           bool     `json:"all,omitempty" jsonschema:"description=Remove all unused build cache, not only dangling cache."`
	Until         string   `json:"until,omitempty" jsonschema:"description=Prune cache created before this timestamp or duration supported by Docker."`
	Label         []string `json:"label,omitempty" jsonschema:"description=Only prune cache with these labels."`
	KeepStorage   int64    `json:"keep_storage,omitempty" jsonschema:"description=Deprecated Docker keep-storage bytes."`
	ReservedSpace int64    `json:"reserved_space,omitempty" jsonschema:"description=Reserved build cache bytes."`
	MaxUsedSpace  int64    `json:"max_used_space,omitempty" jsonschema:"description=Maximum build cache bytes to keep."`
	MinFreeSpace  int64    `json:"min_free_space,omitempty" jsonschema:"description=Minimum free bytes to leave after prune."`
}

type ContextListInput struct{}

type ContextShowInput struct {
	Name string `json:"name,omitempty" jsonschema:"description=Context name. Empty means current context."`
}

type EventsInput struct {
	Since     string   `json:"since,omitempty" jsonschema:"description=Event start time. Defaults to one hour ago."`
	Until     string   `json:"until,omitempty" jsonschema:"description=Event end time. Defaults to now."`
	Limit     int      `json:"limit,omitempty" jsonschema:"description=Maximum events to return. Defaults to 100."`
	Type      []string `json:"type,omitempty" jsonschema:"description=Event type filters."`
	Action    []string `json:"action,omitempty" jsonschema:"description=Event action filters."`
	Container []string `json:"container,omitempty" jsonschema:"description=Container filters."`
	Image     []string `json:"image,omitempty" jsonschema:"description=Image filters."`
	Label     []string `json:"label,omitempty" jsonschema:"description=Label filters."`
}

type ContainerSearchResult = pluginbinding.DatasourceSearchResult[ContainerRecord]
type ImageSearchResult = pluginbinding.DatasourceSearchResult[ImageRecord]
type NetworkSearchResult = pluginbinding.DatasourceSearchResult[NetworkRecord]
type VolumeSearchResult = pluginbinding.DatasourceSearchResult[VolumeRecord]
type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]
type GetInput = pluginbinding.DatasourceGetInput
type ContainerGetResult = pluginbinding.DatasourceGetResult[ContainerRecord]
type ImageGetResult = pluginbinding.DatasourceGetResult[ImageRecord]
type NetworkGetResult = pluginbinding.DatasourceGetResult[NetworkRecord]
type VolumeGetResult = pluginbinding.DatasourceGetResult[VolumeRecord]

func (s Service) Info(ctx pluginbinding.Context, input InfoInput) (DockerInfo, error) {
	client, err := s.client()
	if err != nil {
		return DockerInfo{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.Info(context.Background())
	if err != nil {
		return DockerInfo{}, pluginbinding.Errorf("docker", "%s", err)
	}
	return out, nil
}

func (s Service) ContainerList(ctx pluginbinding.Context, input ContainerListInput) (pluginbinding.ListResult[Container], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ListResult[Container]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListContainers(context.Background(), input)
	if err != nil {
		return pluginbinding.ListResult[Container]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	return pluginbinding.NewListResult(items), nil
}

func (s Service) ContainerShow(ctx pluginbinding.Context, input ShowInput) (pluginbinding.ShowResult[Container], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ShowResult[Container]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectContainer(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return pluginbinding.ShowResult[Container]{}, dockerError(err)
	}
	return pluginbinding.NewShowResult(item, nil), nil
}

func (s Service) ContainerLogs(ctx pluginbinding.Context, input ContainerLogsInput) (ContainerLogsResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerLogsResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerLogs(context.Background(), input)
	if err != nil {
		return ContainerLogsResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerStats(ctx pluginbinding.Context, input ContainerStatsInput) (ContainerStatsResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerStatsResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerStats(context.Background(), input)
	if err != nil {
		return ContainerStatsResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerTop(ctx pluginbinding.Context, input ContainerTopInput) (ContainerTopResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerTopResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerTop(context.Background(), input)
	if err != nil {
		return ContainerTopResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerExec(ctx pluginbinding.Context, input ContainerExecInput) (ContainerExecResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerExecResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerExec(context.Background(), input)
	if err != nil {
		return ContainerExecResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerCopyFrom(ctx pluginbinding.Context, input ContainerCopyFromInput) (ContainerCopyResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerCopyResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerCopyFrom(context.Background(), input)
	if err != nil {
		return ContainerCopyResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerCopyTo(ctx pluginbinding.Context, input ContainerCopyToInput) (ContainerCopyResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerCopyResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerCopyTo(context.Background(), input)
	if err != nil {
		return ContainerCopyResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerCreate(ctx pluginbinding.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerCreateResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerCreate(context.Background(), input)
	if err != nil {
		return ContainerCreateResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerRun(ctx pluginbinding.Context, input ContainerCreateInput) (ContainerCreateResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerCreateResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerRun(context.Background(), input)
	if err != nil {
		return ContainerCreateResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerStart(ctx pluginbinding.Context, input ContainerStartInput) (ContainerActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerStart(context.Background(), input)
	if err != nil {
		return ContainerActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerStop(ctx pluginbinding.Context, input ContainerStopInput) (ContainerActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerStop(context.Background(), input)
	if err != nil {
		return ContainerActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerRestart(ctx pluginbinding.Context, input ContainerRestartInput) (ContainerActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerRestart(context.Background(), input)
	if err != nil {
		return ContainerActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerRemove(ctx pluginbinding.Context, input ContainerRemoveInput) (ContainerActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerRemove(context.Background(), input)
	if err != nil {
		return ContainerActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerInspectRaw(ctx pluginbinding.Context, input RawInspectInput) (RawInspectResult, error) {
	client, err := s.client()
	if err != nil {
		return RawInspectResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerInspectRaw(context.Background(), input)
	if err != nil {
		return RawInspectResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContainerPrune(ctx pluginbinding.Context, input PruneInput) (PruneResult, error) {
	client, err := s.client()
	if err != nil {
		return PruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContainerPrune(context.Background(), input)
	if err != nil {
		return PruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImageList(ctx pluginbinding.Context, input ImageListInput) (pluginbinding.ListResult[Image], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ListResult[Image]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListImages(context.Background(), input)
	if err != nil {
		return pluginbinding.ListResult[Image]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	return pluginbinding.NewListResult(items), nil
}

func (s Service) ImageShow(ctx pluginbinding.Context, input ShowInput) (pluginbinding.ShowResult[Image], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ShowResult[Image]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectImage(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return pluginbinding.ShowResult[Image]{}, dockerError(err)
	}
	return pluginbinding.NewShowResult(item, nil), nil
}

func (s Service) ImagePull(ctx pluginbinding.Context, input ImagePullInput) (ImagePullResult, error) {
	client, err := s.client()
	if err != nil {
		return ImagePullResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImagePull(context.Background(), input)
	if err != nil {
		return ImagePullResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImageTag(ctx pluginbinding.Context, input ImageTagInput) (ResourceActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ResourceActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImageTag(context.Background(), input)
	if err != nil {
		return ResourceActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImagePush(ctx pluginbinding.Context, input ImagePushInput) (ImagePushResult, error) {
	client, err := s.client()
	if err != nil {
		return ImagePushResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImagePush(context.Background(), input)
	if err != nil {
		return ImagePushResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImageBuild(ctx pluginbinding.Context, input ImageBuildInput) (ImageBuildResult, error) {
	client, err := s.client()
	if err != nil {
		return ImageBuildResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImageBuild(context.Background(), input)
	if err != nil {
		return ImageBuildResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImageRemove(ctx pluginbinding.Context, input ImageRemoveInput) (ImageRemoveResult, error) {
	client, err := s.client()
	if err != nil {
		return ImageRemoveResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImageRemove(context.Background(), input)
	if err != nil {
		return ImageRemoveResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImageInspectRaw(ctx pluginbinding.Context, input RawInspectInput) (RawInspectResult, error) {
	client, err := s.client()
	if err != nil {
		return RawInspectResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImageInspectRaw(context.Background(), input)
	if err != nil {
		return RawInspectResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ImagePrune(ctx pluginbinding.Context, input ImagePruneInput) (ImagePruneResult, error) {
	client, err := s.client()
	if err != nil {
		return ImagePruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ImagePrune(context.Background(), input)
	if err != nil {
		return ImagePruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) NetworkList(ctx pluginbinding.Context, input NetworkListInput) (pluginbinding.ListResult[Network], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ListResult[Network]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListNetworks(context.Background(), input)
	if err != nil {
		return pluginbinding.ListResult[Network]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	return pluginbinding.NewListResult(items), nil
}

func (s Service) NetworkShow(ctx pluginbinding.Context, input ShowInput) (pluginbinding.ShowResult[Network], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ShowResult[Network]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectNetwork(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return pluginbinding.ShowResult[Network]{}, dockerError(err)
	}
	return pluginbinding.NewShowResult(item, nil), nil
}

func (s Service) NetworkCreate(ctx pluginbinding.Context, input NetworkCreateInput) (ResourceActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ResourceActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.NetworkCreate(context.Background(), input)
	if err != nil {
		return ResourceActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) NetworkRemove(ctx pluginbinding.Context, input NetworkRemoveInput) (ResourceActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ResourceActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.NetworkRemove(context.Background(), input)
	if err != nil {
		return ResourceActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) NetworkInspectRaw(ctx pluginbinding.Context, input RawInspectInput) (RawInspectResult, error) {
	client, err := s.client()
	if err != nil {
		return RawInspectResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.NetworkInspectRaw(context.Background(), input)
	if err != nil {
		return RawInspectResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) NetworkPrune(ctx pluginbinding.Context, input PruneInput) (PruneResult, error) {
	client, err := s.client()
	if err != nil {
		return PruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.NetworkPrune(context.Background(), input)
	if err != nil {
		return PruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) SystemDF(ctx pluginbinding.Context, input SystemDFInput) (SystemDFResult, error) {
	client, err := s.client()
	if err != nil {
		return SystemDFResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.SystemDF(context.Background(), input)
	if err != nil {
		return SystemDFResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) SystemPrune(ctx pluginbinding.Context, input SystemPruneInput) (SystemPruneResult, error) {
	client, err := s.client()
	if err != nil {
		return SystemPruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.SystemPrune(context.Background(), input)
	if err != nil {
		return SystemPruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) Events(ctx pluginbinding.Context, input EventsInput) (EventsResult, error) {
	client, err := s.client()
	if err != nil {
		return EventsResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.Events(context.Background(), input)
	if err != nil {
		return EventsResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) VolumeList(ctx pluginbinding.Context, input VolumeListInput) (pluginbinding.ListResult[Volume], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ListResult[Volume]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListVolumes(context.Background(), input)
	if err != nil {
		return pluginbinding.ListResult[Volume]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	return pluginbinding.NewListResult(items), nil
}

func (s Service) VolumeShow(ctx pluginbinding.Context, input ShowInput) (pluginbinding.ShowResult[Volume], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ShowResult[Volume]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectVolume(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return pluginbinding.ShowResult[Volume]{}, dockerError(err)
	}
	return pluginbinding.NewShowResult(item, nil), nil
}

func (s Service) VolumeCreate(ctx pluginbinding.Context, input VolumeCreateInput) (Volume, error) {
	client, err := s.client()
	if err != nil {
		return Volume{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.VolumeCreate(context.Background(), input)
	if err != nil {
		return Volume{}, dockerError(err)
	}
	return out, nil
}

func (s Service) VolumeRemove(ctx pluginbinding.Context, input VolumeRemoveInput) (ResourceActionResult, error) {
	client, err := s.client()
	if err != nil {
		return ResourceActionResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.VolumeRemove(context.Background(), input)
	if err != nil {
		return ResourceActionResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) VolumeInspectRaw(ctx pluginbinding.Context, input RawInspectInput) (RawInspectResult, error) {
	client, err := s.client()
	if err != nil {
		return RawInspectResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.VolumeInspectRaw(context.Background(), input)
	if err != nil {
		return RawInspectResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) VolumePrune(ctx pluginbinding.Context, input PruneInput) (PruneResult, error) {
	client, err := s.client()
	if err != nil {
		return PruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.VolumePrune(context.Background(), input)
	if err != nil {
		return PruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) BuildCachePrune(ctx pluginbinding.Context, input BuildCachePruneInput) (PruneResult, error) {
	client, err := s.client()
	if err != nil {
		return PruneResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.BuildCachePrune(context.Background(), input)
	if err != nil {
		return PruneResult{}, dockerError(err)
	}
	return out, nil
}

func (s Service) ContextList(ctx pluginbinding.Context, input ContextListInput) (pluginbinding.ListResult[DockerContext], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ListResult[DockerContext]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContextList(context.Background(), input)
	if err != nil {
		return pluginbinding.ListResult[DockerContext]{}, dockerError(err)
	}
	return pluginbinding.NewListResult(out), nil
}

func (s Service) ContextShow(ctx pluginbinding.Context, input ContextShowInput) (pluginbinding.ShowResult[DockerContext], error) {
	client, err := s.client()
	if err != nil {
		return pluginbinding.ShowResult[DockerContext]{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	out, err := client.ContextShow(context.Background(), input)
	if err != nil {
		return pluginbinding.ShowResult[DockerContext]{}, dockerError(err)
	}
	return pluginbinding.NewShowResult(out, nil), nil
}

func (s Service) ContainerSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (ContainerSearchResult, error) {
	records, err := s.containerRecords(ctx, 0)
	if err != nil {
		return ContainerSearchResult{}, err
	}
	records = filterContainerRecords(records, input.Query)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, limitSlice(records, searchLimit(input.Limit))), nil
}

func (s Service) ImageSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (ImageSearchResult, error) {
	records, err := s.imageRecords(ctx, 0)
	if err != nil {
		return ImageSearchResult{}, err
	}
	records = filterImageRecords(records, input.Query)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, limitSlice(records, searchLimit(input.Limit))), nil
}

func (s Service) NetworkSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (NetworkSearchResult, error) {
	records, err := s.networkRecords(ctx, 0)
	if err != nil {
		return NetworkSearchResult{}, err
	}
	records = filterNetworkRecords(records, input.Query)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, limitSlice(records, searchLimit(input.Limit))), nil
}

func (s Service) VolumeSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (VolumeSearchResult, error) {
	records, err := s.volumeRecords(ctx, 0)
	if err != nil {
		return VolumeSearchResult{}, err
	}
	records = filterVolumeRecords(records, input.Query)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, limitSlice(records, searchLimit(input.Limit))), nil
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	entity := strings.TrimSpace(input.Entity)
	var candidates []pluginbinding.LookupCandidate
	if entity == "" || entity == EntityContainer {
		records, err := s.containerRecords(ctx, 0)
		if err != nil {
			return LookupResult{}, err
		}
		for _, record := range records {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceContainers), record.Entity, record.ID, record, containerLookupValues(record)))
		}
	}
	if entity == "" || entity == EntityImage {
		records, err := s.imageRecords(ctx, 0)
		if err != nil {
			return LookupResult{}, err
		}
		for _, record := range records {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceImages), record.Entity, record.ID, record, imageLookupValues(record)))
		}
	}
	if entity == "" || entity == EntityNetwork {
		records, err := s.networkRecords(ctx, 0)
		if err != nil {
			return LookupResult{}, err
		}
		for _, record := range records {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceNetworks), record.Entity, record.ID, record, networkLookupValues(record)))
		}
	}
	if entity == "" || entity == EntityVolume {
		records, err := s.volumeRecords(ctx, 0)
		if err != nil {
			return LookupResult{}, err
		}
		for _, record := range records {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceVolumes), record.Entity, record.ID, record, volumeLookupValues(record)))
		}
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func (s Service) ContainerGet(ctx pluginbinding.Context, input GetInput) (ContainerGetResult, error) {
	client, err := s.client()
	if err != nil {
		return ContainerGetResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectContainer(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return ContainerGetResult{}, dockerError(err)
	}
	record, ok := normalizeContainerRecord(ctx.DatasourceSource(), item)
	if !ok {
		return ContainerGetResult{}, pluginbinding.Fail("not_found", "container not found")
	}
	return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
}

func (s Service) ImageGet(ctx pluginbinding.Context, input GetInput) (ImageGetResult, error) {
	client, err := s.client()
	if err != nil {
		return ImageGetResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectImage(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return ImageGetResult{}, dockerError(err)
	}
	record, ok := normalizeImageRecord(ctx.DatasourceSource(), item)
	if !ok {
		return ImageGetResult{}, pluginbinding.Fail("not_found", "image not found")
	}
	return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
}

func (s Service) NetworkGet(ctx pluginbinding.Context, input GetInput) (NetworkGetResult, error) {
	client, err := s.client()
	if err != nil {
		return NetworkGetResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectNetwork(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return NetworkGetResult{}, dockerError(err)
	}
	record, ok := normalizeNetworkRecord(ctx.DatasourceSource(), item)
	if !ok {
		return NetworkGetResult{}, pluginbinding.Fail("not_found", "network not found")
	}
	return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
}

func (s Service) VolumeGet(ctx pluginbinding.Context, input GetInput) (VolumeGetResult, error) {
	client, err := s.client()
	if err != nil {
		return VolumeGetResult{}, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	item, err := client.InspectVolume(context.Background(), strings.TrimSpace(input.ID))
	if err != nil {
		return VolumeGetResult{}, dockerError(err)
	}
	record, ok := normalizeVolumeRecord(ctx.DatasourceSource(), item)
	if !ok {
		return VolumeGetResult{}, pluginbinding.Fail("not_found", "volume not found")
	}
	return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
}

func (s Service) containerRecords(ctx pluginbinding.Context, limit int) ([]ContainerRecord, error) {
	client, err := s.client()
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListContainers(context.Background(), ContainerListInput{All: true, Limit: limit})
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	records := make([]ContainerRecord, 0, len(items))
	for _, item := range items {
		if record, ok := normalizeContainerRecord(ctx.DatasourceSource(), item); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s Service) imageRecords(ctx pluginbinding.Context, limit int) ([]ImageRecord, error) {
	client, err := s.client()
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListImages(context.Background(), ImageListInput{All: true, Limit: limit})
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	records := make([]ImageRecord, 0, len(items))
	for _, item := range items {
		if record, ok := normalizeImageRecord(ctx.DatasourceSource(), item); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s Service) networkRecords(ctx pluginbinding.Context, limit int) ([]NetworkRecord, error) {
	client, err := s.client()
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListNetworks(context.Background(), NetworkListInput{Limit: limit})
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	records := make([]NetworkRecord, 0, len(items))
	for _, item := range items {
		if record, ok := normalizeNetworkRecord(ctx.DatasourceSource(), item); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s Service) volumeRecords(ctx pluginbinding.Context, limit int) ([]VolumeRecord, error) {
	client, err := s.client()
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	defer client.Close()
	items, err := client.ListVolumes(context.Background(), VolumeListInput{Limit: limit})
	if err != nil {
		return nil, pluginbinding.Errorf("docker", "%s", err)
	}
	records := make([]VolumeRecord, 0, len(items))
	for _, item := range items {
		if record, ok := normalizeVolumeRecord(ctx.DatasourceSource(), item); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s Service) client() (Client, error) {
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	return factory()
}

func dockerError(err error) error {
	if err == nil {
		return nil
	}
	return pluginbinding.Errorf("docker", "%s", err)
}

func searchLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

func limitSlice[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func filterContainerRecords(records []ContainerRecord, query string) []ContainerRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	return filterRecords(records, func(record ContainerRecord) string {
		return strings.Join([]string{record.ContainerID, record.ShortID, record.Name, record.Image, record.State, record.Status, joinLabels(record.Labels)}, " ")
	}, query)
}

func filterImageRecords(records []ImageRecord, query string) []ImageRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	return filterRecords(records, func(record ImageRecord) string {
		return strings.Join([]string{record.ImageID, record.ShortID, strings.Join(record.RepoTags, " "), strings.Join(record.RepoDigests, " "), joinLabels(record.Labels)}, " ")
	}, query)
}

func filterNetworkRecords(records []NetworkRecord, query string) []NetworkRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	return filterRecords(records, func(record NetworkRecord) string {
		return strings.Join([]string{record.NetworkID, record.ShortID, record.Name, record.Driver, record.Scope, joinLabels(record.Labels)}, " ")
	}, query)
}

func filterVolumeRecords(records []VolumeRecord, query string) []VolumeRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	return filterRecords(records, func(record VolumeRecord) string {
		return strings.Join([]string{record.Name, record.Driver, record.Scope, record.Mountpoint, joinLabels(record.Labels)}, " ")
	}, query)
}

func filterRecords[T any](records []T, text func(T) string, query string) []T {
	out := make([]T, 0, len(records))
	for _, record := range records {
		if strings.Contains(strings.ToLower(text(record)), query) {
			out = append(out, record)
		}
	}
	return out
}

func containerLookupValues(record ContainerRecord) map[string]string {
	return map[string]string{
		"id":                  record.ID,
		"title":               record.Title,
		"links.self":          record.Links["self"],
		"record.container_id": record.ContainerID,
		"record.short_id":     record.ShortID,
		"record.name":         record.Name,
		"record.image":        record.Image,
		"record.state":        record.State,
		"record.status":       record.Status,
		"record.labels":       joinLabels(record.Labels),
	}
}

func imageLookupValues(record ImageRecord) map[string]string {
	return map[string]string{
		"id":                  record.ID,
		"title":               record.Title,
		"links.self":          record.Links["self"],
		"record.image_id":     record.ImageID,
		"record.short_id":     record.ShortID,
		"record.repo_tags":    strings.Join(record.RepoTags, " "),
		"record.repo_digests": strings.Join(record.RepoDigests, " "),
		"record.labels":       joinLabels(record.Labels),
	}
}

func networkLookupValues(record NetworkRecord) map[string]string {
	return map[string]string{
		"id":                record.ID,
		"title":             record.Title,
		"links.self":        record.Links["self"],
		"record.network_id": record.NetworkID,
		"record.short_id":   record.ShortID,
		"record.name":       record.Name,
		"record.driver":     record.Driver,
		"record.scope":      record.Scope,
		"record.labels":     joinLabels(record.Labels),
	}
}

func volumeLookupValues(record VolumeRecord) map[string]string {
	return map[string]string{
		"id":                record.ID,
		"title":             record.Title,
		"links.self":        record.Links["self"],
		"record.name":       record.Name,
		"record.driver":     record.Driver,
		"record.scope":      record.Scope,
		"record.mountpoint": record.Mountpoint,
		"record.labels":     joinLabels(record.Labels),
	}
}

func joinLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for key, value := range labels {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, " ")
}
