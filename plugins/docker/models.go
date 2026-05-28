package docker

import (
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

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

type ContainerRecord struct {
	pluginbinding.DatasourceRecord
	Title       string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ContainerID string            `json:"container_id" datasource:"id"`
	ShortID     string            `json:"short_id,omitempty" datasource:"completion,view=compact|lookup|table"`
	Name        string            `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	Image       string            `json:"image,omitempty" datasource:"completion,view=compact|lookup|table"`
	State       string            `json:"state,omitempty" datasource:"view=compact|lookup|table"`
	Status      string            `json:"status,omitempty" datasource:"completion,view=compact|lookup|table"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ImageRecord struct {
	pluginbinding.DatasourceRecord
	Title       string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ImageID     string            `json:"image_id" datasource:"id"`
	ShortID     string            `json:"short_id,omitempty" datasource:"completion,view=compact|lookup|table"`
	RepoTags    []string          `json:"repo_tags,omitempty" datasource:"completion,view=compact|lookup|table"`
	RepoDigests []string          `json:"repo_digests,omitempty" datasource:"completion,view=lookup|table"`
	Size        int64             `json:"size,omitempty" datasource:"view=compact|table"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type NetworkRecord struct {
	pluginbinding.DatasourceRecord
	Title     string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	NetworkID string            `json:"network_id" datasource:"id"`
	ShortID   string            `json:"short_id,omitempty" datasource:"completion,view=compact|lookup|table"`
	Name      string            `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	Driver    string            `json:"driver,omitempty" datasource:"completion,view=compact|lookup|table"`
	Scope     string            `json:"scope,omitempty" datasource:"completion,view=compact|lookup|table"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type VolumeRecord struct {
	pluginbinding.DatasourceRecord
	Title      string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Name       string            `json:"name" datasource:"id,completion,view=compact|lookup|table"`
	Driver     string            `json:"driver,omitempty" datasource:"completion,view=compact|lookup|table"`
	Scope      string            `json:"scope,omitempty" datasource:"completion,view=compact|lookup|table"`
	Mountpoint string            `json:"mountpoint,omitempty" datasource:"completion,view=compact|lookup|table"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func normalizeContainerRecord(source pluginbinding.DatasourceSource, container Container) (ContainerRecord, bool) {
	if strings.TrimSpace(container.ID) == "" {
		return ContainerRecord{}, false
	}
	title := firstNonEmpty(container.Name, container.ShortID, shortID(container.ID), container.ID)
	short := firstNonEmpty(container.ShortID, shortID(container.ID))
	return ContainerRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityContainer, container.ID, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", "docker://container/"+container.ID)),
		Title:            title,
		ContainerID:      container.ID,
		ShortID:          short,
		Name:             container.Name,
		Image:            container.Image,
		State:            container.State,
		Status:           container.Status,
		Labels:           cloneStringMap(container.Labels),
	}, true
}

func normalizeImageRecord(source pluginbinding.DatasourceSource, image Image) (ImageRecord, bool) {
	id := strings.TrimSpace(image.ID)
	if id == "" && len(image.RepoDigests) > 0 {
		id = image.RepoDigests[0]
	}
	if id == "" {
		return ImageRecord{}, false
	}
	title := firstNonEmpty(image.Title, firstString(image.RepoTags), firstString(image.RepoDigests), shortID(id), id)
	short := firstNonEmpty(image.ShortID, shortID(id))
	return ImageRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityImage, id, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", "docker://image/"+id)),
		Title:            title,
		ImageID:          id,
		ShortID:          short,
		RepoTags:         append([]string(nil), image.RepoTags...),
		RepoDigests:      append([]string(nil), image.RepoDigests...),
		Size:             image.Size,
		Labels:           cloneStringMap(image.Labels),
	}, true
}

func normalizeNetworkRecord(source pluginbinding.DatasourceSource, network Network) (NetworkRecord, bool) {
	if strings.TrimSpace(network.ID) == "" {
		return NetworkRecord{}, false
	}
	title := firstNonEmpty(network.Name, network.ShortID, shortID(network.ID), network.ID)
	short := firstNonEmpty(network.ShortID, shortID(network.ID))
	return NetworkRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityNetwork, network.ID, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", "docker://network/"+network.ID)),
		Title:            title,
		NetworkID:        network.ID,
		ShortID:          short,
		Name:             network.Name,
		Driver:           network.Driver,
		Scope:            network.Scope,
		Labels:           cloneStringMap(network.Labels),
	}, true
}

func normalizeVolumeRecord(source pluginbinding.DatasourceSource, volume Volume) (VolumeRecord, bool) {
	if strings.TrimSpace(volume.Name) == "" {
		return VolumeRecord{}, false
	}
	return VolumeRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityVolume, volume.Name, pluginbinding.RecordTitle(volume.Name), pluginbinding.RecordLink("self", "docker://volume/"+volume.Name)),
		Title:            volume.Name,
		Name:             volume.Name,
		Driver:           volume.Driver,
		Scope:            volume.Scope,
		Mountpoint:       volume.Mountpoint,
		Labels:           cloneStringMap(volume.Labels),
	}, true
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
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<none>:<none>" {
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
