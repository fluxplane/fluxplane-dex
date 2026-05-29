package dockerhost

import (
	"context"
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

type DockerInfo struct {
	ID                string   `json:"id,omitempty"`
	Name              string   `json:"name,omitempty"`
	ServerVersion     string   `json:"server_version,omitempty"`
	APIVersion        string   `json:"api_version,omitempty"`
	MinAPIVersion     string   `json:"min_api_version,omitempty"`
	OSType            string   `json:"os_type,omitempty"`
	OperatingSystem   string   `json:"operating_system,omitempty"`
	Architecture      string   `json:"architecture,omitempty"`
	KernelVersion     string   `json:"kernel_version,omitempty"`
	Driver            string   `json:"driver,omitempty"`
	CgroupDriver      string   `json:"cgroup_driver,omitempty"`
	CgroupVersion     string   `json:"cgroup_version,omitempty"`
	LoggingDriver     string   `json:"logging_driver,omitempty"`
	Containers        int      `json:"containers"`
	ContainersRunning int      `json:"containers_running"`
	ContainersPaused  int      `json:"containers_paused"`
	ContainersStopped int      `json:"containers_stopped"`
	Images            int      `json:"images"`
	CPUs              int      `json:"cpus,omitempty"`
	MemoryBytes       int64    `json:"memory_bytes,omitempty"`
	DockerRootDir     string   `json:"docker_root_dir,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

type Container struct {
	ID       string            `json:"id"`
	ShortID  string            `json:"short_id,omitempty"`
	Names    []string          `json:"names,omitempty"`
	Name     string            `json:"name,omitempty"`
	Image    string            `json:"image,omitempty"`
	ImageID  string            `json:"image_id,omitempty"`
	Command  string            `json:"command,omitempty"`
	Created  int64             `json:"created,omitempty"`
	State    string            `json:"state,omitempty"`
	Status   string            `json:"status,omitempty"`
	Ports    []string          `json:"ports,omitempty"`
	Networks []string          `json:"networks,omitempty"`
	Mounts   []string          `json:"mounts,omitempty"`
	Health   string            `json:"health,omitempty"`
	Platform string            `json:"platform,omitempty"`
	Restart  string            `json:"restart,omitempty"`
	EnvKeys  []string          `json:"env_keys,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type Image struct {
	ID            string            `json:"id"`
	ShortID       string            `json:"short_id,omitempty"`
	Title         string            `json:"title,omitempty"`
	RepoTags      []string          `json:"repo_tags,omitempty"`
	RepoDigests   []string          `json:"repo_digests,omitempty"`
	Created       int64             `json:"created,omitempty"`
	CreatedAt     string            `json:"created_at,omitempty"`
	Size          int64             `json:"size,omitempty"`
	SharedSize    int64             `json:"shared_size,omitempty"`
	Containers    int64             `json:"containers,omitempty"`
	OS            string            `json:"os,omitempty"`
	Architecture  string            `json:"architecture,omitempty"`
	DockerVersion string            `json:"docker_version,omitempty"`
	Author        string            `json:"author,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

type Network struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"short_id,omitempty"`
	Name       string            `json:"name,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	Attachable bool              `json:"attachable,omitempty"`
	Ingress    bool              `json:"ingress,omitempty"`
	Containers []NetworkEndpoint `json:"containers,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type NetworkEndpoint struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	EndpointID  string `json:"endpoint_id,omitempty"`
	IPv4Address string `json:"ipv4_address,omitempty"`
	IPv6Address string `json:"ipv6_address,omitempty"`
}

type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	Mountpoint string            `json:"mountpoint,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	CreatedAt  string            `json:"created_at,omitempty"`
	Size       int64             `json:"size,omitempty"`
	RefCount   int64             `json:"ref_count,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type ContainerLogsResult struct {
	Container string `json:"container"`
	Tail      string `json:"tail,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Text      string `json:"text,omitempty"`
}

type ContainerStatsResult struct {
	Container        string           `json:"container"`
	Name             string           `json:"name,omitempty"`
	OSType           string           `json:"os_type,omitempty"`
	Read             string           `json:"read,omitempty"`
	CPUPercent       float64          `json:"cpu_percent,omitempty"`
	MemoryUsageBytes uint64           `json:"memory_usage_bytes,omitempty"`
	MemoryLimitBytes uint64           `json:"memory_limit_bytes,omitempty"`
	MemoryPercent    float64          `json:"memory_percent,omitempty"`
	PIDs             uint64           `json:"pids,omitempty"`
	NetworkRxBytes   uint64           `json:"network_rx_bytes,omitempty"`
	NetworkTxBytes   uint64           `json:"network_tx_bytes,omitempty"`
	BlockReadBytes   uint64           `json:"block_read_bytes,omitempty"`
	BlockWriteBytes  uint64           `json:"block_write_bytes,omitempty"`
	Networks         map[string]NetIO `json:"networks,omitempty"`
}

type NetIO struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

type ContainerTopResult struct {
	Container string     `json:"container"`
	Titles    []string   `json:"titles,omitempty"`
	Processes [][]string `json:"processes,omitempty"`
	Count     int        `json:"count"`
}

type ContainerExecResult struct {
	Container string `json:"container"`
	ExecID    string `json:"exec_id,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Running   bool   `json:"running,omitempty"`
	Detached  bool   `json:"detached,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Text      string `json:"text,omitempty"`
	OK        bool   `json:"ok"`
}

type ContainerCreateResult struct {
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Image    string   `json:"image,omitempty"`
	Started  bool     `json:"started,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	OK       bool     `json:"ok"`
}

type ContainerCopyResult struct {
	Container       string   `json:"container"`
	SourcePath      string   `json:"source_path"`
	DestinationPath string   `json:"destination_path"`
	Files           []string `json:"files,omitempty"`
	Bytes           int64    `json:"bytes,omitempty"`
	OK              bool     `json:"ok"`
}

type ContainerActionResult struct {
	Container string `json:"container"`
	Action    string `json:"action"`
	OK        bool   `json:"ok"`
}

type DockerContext struct {
	Name          string                    `json:"name"`
	Current       bool                      `json:"current,omitempty"`
	Host          string                    `json:"host,omitempty"`
	TLS           bool                      `json:"tls,omitempty"`
	SkipTLSVerify bool                      `json:"skip_tls_verify,omitempty"`
	Description   string                    `json:"description,omitempty"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
	Endpoints     map[string]map[string]any `json:"endpoints,omitempty"`
	Path          string                    `json:"path,omitempty"`
}

type ResourceActionResult struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	OK     bool   `json:"ok"`
}

type ImagePullResult struct {
	Reference string           `json:"reference"`
	Platform  string           `json:"platform,omitempty"`
	Events    []map[string]any `json:"events,omitempty"`
	Count     int              `json:"count"`
	OK        bool             `json:"ok"`
}

type ImagePushResult struct {
	Reference string           `json:"reference"`
	Platform  string           `json:"platform,omitempty"`
	Events    []map[string]any `json:"events,omitempty"`
	Count     int              `json:"count"`
	OK        bool             `json:"ok"`
}

type ImageBuildResult struct {
	ContextPath string           `json:"context_path"`
	Tags        []string         `json:"tags,omitempty"`
	ImageID     string           `json:"image_id,omitempty"`
	Events      []map[string]any `json:"events,omitempty"`
	Count       int              `json:"count"`
	OK          bool             `json:"ok"`
}

type ImageRemoveResult struct {
	ID       string   `json:"id"`
	Deleted  []string `json:"deleted,omitempty"`
	Untagged []string `json:"untagged,omitempty"`
	OK       bool     `json:"ok"`
}

type RawInspectResult struct {
	Kind string         `json:"kind"`
	ID   string         `json:"id"`
	Data map[string]any `json:"data"`
}

type PruneResult struct {
	Kind                string   `json:"kind"`
	Deleted             []string `json:"deleted,omitempty"`
	SpaceReclaimedBytes uint64   `json:"space_reclaimed_bytes,omitempty"`
	Count               int      `json:"count"`
	OK                  bool     `json:"ok"`
}

type ImagePruneResult struct {
	Kind                string   `json:"kind"`
	Deleted             []string `json:"deleted,omitempty"`
	Untagged            []string `json:"untagged,omitempty"`
	SpaceReclaimedBytes uint64   `json:"space_reclaimed_bytes,omitempty"`
	Count               int      `json:"count"`
	OK                  bool     `json:"ok"`
}

type SystemPruneResult struct {
	Containers PruneResult      `json:"containers"`
	Networks   PruneResult      `json:"networks"`
	Images     ImagePruneResult `json:"images"`
	BuildCache PruneResult      `json:"build_cache"`
	Volumes    *PruneResult     `json:"volumes,omitempty"`
	TotalCount int              `json:"total_count"`
	TotalBytes uint64           `json:"total_bytes"`
	OK         bool             `json:"ok"`
}

type SystemDFResult struct {
	LayersSizeBytes int64       `json:"layers_size_bytes,omitempty"`
	Images          []Image     `json:"images,omitempty"`
	Containers      []Container `json:"containers,omitempty"`
	Volumes         []Volume    `json:"volumes,omitempty"`
	BuildCacheCount int         `json:"build_cache_count,omitempty"`
	ImageCount      int         `json:"image_count"`
	ContainerCount  int         `json:"container_count"`
	VolumeCount     int         `json:"volume_count"`
}

type Event struct {
	Type       string            `json:"type,omitempty"`
	Action     string            `json:"action,omitempty"`
	ID         string            `json:"id,omitempty"`
	ActorID    string            `json:"actor_id,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	Time       int64             `json:"time,omitempty"`
	TimeNano   int64             `json:"time_nano,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type EventsResult struct {
	Events []Event `json:"events"`
	Count  int     `json:"count"`
}

type InfoInput struct{}

type ShowInput struct {
	ID string `json:"id,omitempty"`
}

type ContainerLogsInput struct {
	ID         string `json:"id,omitempty"`
	Tail       int    `json:"tail,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	Timestamps bool   `json:"timestamps,omitempty"`
}

type ContainerStatsInput struct {
	ID string `json:"id,omitempty"`
}

type ContainerTopInput struct {
	ID   string   `json:"id,omitempty"`
	Args []string `json:"args,omitempty"`
}

type ContainerExecInput struct {
	ID            string   `json:"id,omitempty"`
	Cmd           []string `json:"cmd,omitempty"`
	Env           []string `json:"env,omitempty"`
	User          string   `json:"user,omitempty"`
	Workdir       string   `json:"workdir,omitempty"`
	Privileged    bool     `json:"privileged,omitempty"`
	TTY           bool     `json:"tty,omitempty"`
	Detach        bool     `json:"detach,omitempty"`
	TimeoutSecond int      `json:"timeout_second,omitempty"`
}

type ContainerCopyFromInput struct {
	ID              string `json:"id,omitempty"`
	SourcePath      string `json:"source_path,omitempty"`
	DestinationPath string `json:"destination_path,omitempty"`
	Overwrite       bool   `json:"overwrite,omitempty"`
}

type ContainerCopyToInput struct {
	ID                        string `json:"id,omitempty"`
	SourcePath                string `json:"source_path,omitempty"`
	DestinationPath           string `json:"destination_path,omitempty"`
	AllowOverwriteDirWithFile bool   `json:"allow_overwrite_dir_with_file,omitempty"`
	CopyUIDGID                bool   `json:"copy_uid_gid,omitempty"`
}

type ContainerCreateInput struct {
	Image      string            `json:"image,omitempty"`
	Name       string            `json:"name,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Env        []string          `json:"env,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Workdir    string            `json:"workdir,omitempty"`
	User       string            `json:"user,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	Network    string            `json:"network,omitempty"`
	Restart    string            `json:"restart,omitempty"`
	AutoRemove bool              `json:"auto_remove,omitempty"`
	TTY        bool              `json:"tty,omitempty"`
	OpenStdin  bool              `json:"open_stdin,omitempty"`
	Privileged bool              `json:"privileged,omitempty"`
	Binds      []string          `json:"binds,omitempty"`
	Mounts     []MountInput      `json:"mounts,omitempty"`
	Ports      []PortInput       `json:"ports,omitempty"`
	Platform   string            `json:"platform,omitempty"`
}

type MountInput struct {
	Type     string `json:"type,omitempty"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type PortInput struct {
	Container string `json:"container,omitempty"`
	HostIP    string `json:"host_ip,omitempty"`
	HostPort  string `json:"host_port,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
}

type ContainerStartInput struct {
	ID string `json:"id,omitempty"`
}

type ContainerStopInput struct {
	ID      string `json:"id,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
	Signal  string `json:"signal,omitempty"`
}

type ContainerRestartInput struct {
	ID      string `json:"id,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
	Signal  string `json:"signal,omitempty"`
}

type ContainerRemoveInput struct {
	ID      string `json:"id,omitempty"`
	Force   bool   `json:"force,omitempty"`
	Volumes bool   `json:"volumes,omitempty"`
}

type RawInspectInput struct {
	ID string `json:"id,omitempty"`
}

type PruneInput struct {
	Until string   `json:"until,omitempty"`
	Label []string `json:"label,omitempty"`
}

type ContainerListInput struct {
	All    bool     `json:"all,omitempty"`
	Limit  int      `json:"limit,omitempty"`
	Status []string `json:"status,omitempty"`
	Name   []string `json:"name,omitempty"`
	Label  []string `json:"label,omitempty"`
}

type ImageListInput struct {
	All       bool     `json:"all,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Reference []string `json:"reference,omitempty"`
	Label     []string `json:"label,omitempty"`
}

type ImagePullInput struct {
	Reference    string            `json:"reference,omitempty"`
	Platform     string            `json:"platform,omitempty"`
	RegistryAuth string            `json:"registry_auth,omitempty"`
	Auth         RegistryAuthInput `json:"auth,omitempty"`
	Limit        int               `json:"limit,omitempty"`
}

type RegistryAuthInput struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth,omitempty"`
	Email         string `json:"email,omitempty"`
	ServerAddress string `json:"server_address,omitempty"`
	IdentityToken string `json:"identity_token,omitempty"`
	RegistryToken string `json:"registry_token,omitempty"`
}

type ImageTagInput struct {
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
}

type ImagePushInput struct {
	Reference    string            `json:"reference,omitempty"`
	Platform     string            `json:"platform,omitempty"`
	RegistryAuth string            `json:"registry_auth,omitempty"`
	Auth         RegistryAuthInput `json:"auth,omitempty"`
	Limit        int               `json:"limit,omitempty"`
}

type ImageBuildInput struct {
	ContextPath  string                       `json:"context_path,omitempty"`
	Dockerfile   string                       `json:"dockerfile,omitempty"`
	Tags         []string                     `json:"tags,omitempty"`
	Target       string                       `json:"target,omitempty"`
	BuildArgs    map[string]string            `json:"build_args,omitempty"`
	Labels       map[string]string            `json:"labels,omitempty"`
	Platform     string                       `json:"platform,omitempty"`
	Pull         bool                         `json:"pull,omitempty"`
	NoCache      bool                         `json:"no_cache,omitempty"`
	Network      string                       `json:"network,omitempty"`
	RegistryAuth string                       `json:"registry_auth,omitempty"`
	Auth         RegistryAuthInput            `json:"auth,omitempty"`
	AuthConfigs  map[string]RegistryAuthInput `json:"auth_configs,omitempty"`
	Limit        int                          `json:"limit,omitempty"`
}

type ImageRemoveInput struct {
	ID            string `json:"id,omitempty"`
	Force         bool   `json:"force,omitempty"`
	PruneChildren bool   `json:"prune_children,omitempty"`
}

type ImagePruneInput struct {
	All   bool     `json:"all,omitempty"`
	Until string   `json:"until,omitempty"`
	Label []string `json:"label,omitempty"`
}

type NetworkListInput struct {
	Limit int      `json:"limit,omitempty"`
	Name  []string `json:"name,omitempty"`
	Label []string `json:"label,omitempty"`
}

type NetworkCreateInput struct {
	Name       string            `json:"name,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	Attachable bool              `json:"attachable,omitempty"`
	Ingress    bool              `json:"ingress,omitempty"`
	EnableIPv4 *bool             `json:"enable_ipv4,omitempty"`
	EnableIPv6 *bool             `json:"enable_ipv6,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type NetworkRemoveInput struct {
	ID string `json:"id,omitempty"`
}

type VolumeListInput struct {
	Limit int      `json:"limit,omitempty"`
	Name  []string `json:"name,omitempty"`
	Label []string `json:"label,omitempty"`
}

type VolumeCreateInput struct {
	Name       string            `json:"name,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	DriverOpts map[string]string `json:"driver_opts,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type VolumeRemoveInput struct {
	ID    string `json:"id,omitempty"`
	Force bool   `json:"force,omitempty"`
}

type SystemDFInput struct {
	Type []string `json:"type,omitempty"`
}

type SystemPruneInput struct {
	All     bool     `json:"all,omitempty"`
	Volumes bool     `json:"volumes,omitempty"`
	Until   string   `json:"until,omitempty"`
	Label   []string `json:"label,omitempty"`
}

type BuildCachePruneInput struct {
	All           bool     `json:"all,omitempty"`
	Until         string   `json:"until,omitempty"`
	Label         []string `json:"label,omitempty"`
	KeepStorage   int64    `json:"keep_storage,omitempty"`
	ReservedSpace int64    `json:"reserved_space,omitempty"`
	MaxUsedSpace  int64    `json:"max_used_space,omitempty"`
	MinFreeSpace  int64    `json:"min_free_space,omitempty"`
}

type ContextListInput struct{}

type ContextShowInput struct {
	Name string `json:"name,omitempty"`
}

type EventsInput struct {
	Since     string   `json:"since,omitempty"`
	Until     string   `json:"until,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Type      []string `json:"type,omitempty"`
	Action    []string `json:"action,omitempty"`
	Container []string `json:"container,omitempty"`
	Image     []string `json:"image,omitempty"`
	Label     []string `json:"label,omitempty"`
}
