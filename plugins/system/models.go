package system

type InfoInput struct {
	Categories StringList `json:"categories,omitempty" jsonschema:"description=Categories to include"`
	Category   string     `json:"category,omitempty" jsonschema:"description=Comma-separated category selector"`
	Include    StringList `json:"include,omitempty" jsonschema:"description=Alias for categories"`
	Exclude    StringList `json:"exclude,omitempty" jsonschema:"description=Categories to omit"`
}

type InfoResult struct {
	Categories  []string       `json:"categories"`
	GeneratedAt string         `json:"generated_at"`
	System      map[string]any `json:"system"`
}

type OSInfo struct {
	GOOS     string   `json:"goos"`
	GOARCH   string   `json:"goarch"`
	Hostname string   `json:"hostname,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type RuntimeInfo struct {
	GoVersion  string   `json:"go_version"`
	Compiler   string   `json:"compiler"`
	ProcessID  int      `json:"process_id"`
	ParentID   int      `json:"parent_id,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type UserInfo struct {
	Username string   `json:"username,omitempty"`
	Name     string   `json:"name,omitempty"`
	UID      string   `json:"uid,omitempty"`
	GID      string   `json:"gid,omitempty"`
	HomeDir  string   `json:"home_dir,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type PathsInfo struct {
	TempDir    string   `json:"temp_dir"`
	WorkingDir string   `json:"working_dir,omitempty"`
	HomeDir    string   `json:"home_dir,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type CPUInfo struct {
	LogicalCPUs int `json:"logical_cpus"`
	GOMAXPROCS  int `json:"gomaxprocs"`
}

type TimeInfo struct {
	UTC          string `json:"utc"`
	Local        string `json:"local"`
	Unix         int64  `json:"unix"`
	Timezone     string `json:"timezone"`
	ZoneName     string `json:"zone_name"`
	OffsetSecond int    `json:"offset_seconds"`
}

type EnvInfo struct {
	Values map[string]string `json:"values"`
}

type NetworkInfo struct {
	Hostname   string             `json:"hostname,omitempty"`
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`
	DNS        *DNSInfo           `json:"dns,omitempty"`
	Proxies    map[string]string  `json:"proxies,omitempty"`
	Warnings   []string           `json:"warnings,omitempty"`
}

type NetworkInterface struct {
	Name         string   `json:"name"`
	Index        int      `json:"index"`
	MTU          int      `json:"mtu"`
	Flags        []string `json:"flags,omitempty"`
	HardwareAddr string   `json:"hardware_addr,omitempty"`
	Addrs        []string `json:"addrs,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type DNSInfo struct {
	Source      string   `json:"source,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}
