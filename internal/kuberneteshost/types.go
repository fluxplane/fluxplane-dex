package kuberneteshost

import "time"

type ClusterListResult struct {
	Contexts []ClusterContext `json:"contexts"`
}

type ClusterContext struct {
	Name    string `json:"name"`
	Current bool   `json:"current,omitempty"`
	Cluster string `json:"cluster,omitempty"`
	User    string `json:"user,omitempty"`
}

type ClusterTestInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty"`
	URL         string `json:"url,omitempty"`
	Context     string `json:"context,omitempty"`
}

type ClusterTestResult struct {
	Context       string `json:"context,omitempty"`
	OK            bool   `json:"ok"`
	ServerVersion string `json:"server_version,omitempty"`
	Platform      string `json:"platform,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

type EndpointDiscoverInput struct {
	Product   string `json:"product,omitempty"`
	Context   string `json:"context,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type InventoryInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty"`
	URL         string `json:"url,omitempty"`
	Context     string `json:"context,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name,omitempty"`
	Query       string `json:"query,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type PodLogsInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty"`
	URL         string `json:"url,omitempty"`
	Context     string `json:"context,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name,omitempty"`
	Container   string `json:"container,omitempty"`
	TailLines   int64  `json:"tail_lines,omitempty"`
	LimitBytes  int64  `json:"limit_bytes,omitempty"`
	Since       string `json:"since,omitempty"`
	Until       string `json:"until,omitempty"`
	Previous    bool   `json:"previous,omitempty"`
	Timestamps  bool   `json:"timestamps,omitempty"`
}

type PodLogsResult struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Container  string   `json:"container,omitempty"`
	Lines      []string `json:"lines"`
	Text       string   `json:"text,omitempty"`
	LineCount  int      `json:"line_count"`
	TailLines  int64    `json:"tail_lines,omitempty"`
	LimitBytes int64    `json:"limit_bytes,omitempty"`
	Since      string   `json:"since,omitempty"`
	Until      string   `json:"until,omitempty"`
	Previous   bool     `json:"previous,omitempty"`
	Timestamps bool     `json:"timestamps,omitempty"`
}

type PortForwardStartInput struct {
	EndpointRef     string `json:"endpoint_ref,omitempty"`
	URL             string `json:"url,omitempty"`
	Context         string `json:"context,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Resource        string `json:"resource,omitempty"`
	ResourceType    string `json:"resource_type,omitempty"`
	Name            string `json:"name,omitempty"`
	RemotePort      int    `json:"remote_port,omitempty"`
	LocalPort       int    `json:"local_port,omitempty"`
	Address         string `json:"address,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type PortForwardResult struct {
	ID              string    `json:"id"`
	EndpointRef     string    `json:"endpoint_ref,omitempty"`
	Context         string    `json:"context,omitempty"`
	Namespace       string    `json:"namespace"`
	Resource        string    `json:"resource"`
	Address         string    `json:"address"`
	LocalPort       int       `json:"local_port"`
	RemotePort      int       `json:"remote_port"`
	LocalURL        string    `json:"local_url"`
	PID             int       `json:"pid"`
	ProcessGroup    int       `json:"process_group,omitempty"`
	DurationSeconds int       `json:"duration_seconds"`
	ExpiresAt       time.Time `json:"expires_at"`
	LogPath         string    `json:"log_path,omitempty"`
	Command         []string  `json:"command,omitempty"`
}

type PortForwardStopInput struct {
	ID           string `json:"id,omitempty"`
	ProcessGroup int    `json:"process_group,omitempty"`
	PID          int    `json:"pid,omitempty"`
}

type PortForwardStopResult struct {
	ID      string `json:"id,omitempty"`
	Stopped bool   `json:"stopped"`
	Signal  string `json:"signal,omitempty"`
	Error   string `json:"error,omitempty"`
}
