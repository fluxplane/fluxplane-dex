package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	systemProviderName = "system"

	systemCategoryOS      = "os"
	systemCategoryRuntime = "runtime"
	systemCategoryUser    = "user"
	systemCategoryPaths   = "paths"
	systemCategoryCPU     = "cpu"
	systemCategoryTime    = "time"
	systemCategoryEnv     = "env"
	systemCategoryNetwork = "network"
)

type localSystemProvider struct{}

func (localSystemProvider) Call(_ context.Context, action string, payload json.RawMessage) (json.RawMessage, error) {
	switch strings.TrimSpace(action) {
	case "info":
		var input struct {
			Categories []string `json:"categories"`
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &input); err != nil {
				return nil, err
			}
		}
		out, err := collectSystemInfo(input.Categories)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	default:
		return nil, fmt.Errorf("unsupported system provider action %q", action)
	}
}

func collectSystemInfo(categories []string) (map[string]any, error) {
	if len(categories) == 0 {
		categories = []string{systemCategoryOS, systemCategoryRuntime, systemCategoryUser, systemCategoryPaths, systemCategoryCPU, systemCategoryTime, systemCategoryEnv, systemCategoryNetwork}
	}
	now := time.Now()
	system := map[string]any{}
	for _, category := range categories {
		switch strings.TrimSpace(category) {
		case systemCategoryOS:
			system[category] = collectHostOS()
		case systemCategoryRuntime:
			system[category] = collectHostRuntime()
		case systemCategoryUser:
			system[category] = collectHostUser()
		case systemCategoryPaths:
			system[category] = collectHostPaths()
		case systemCategoryCPU:
			system[category] = map[string]any{"logical_cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0)}
		case systemCategoryTime:
			system[category] = collectHostTime(now)
		case systemCategoryEnv:
			system[category] = collectHostEnv()
		case systemCategoryNetwork:
			system[category] = collectHostNetwork()
		default:
			return nil, fmt.Errorf("unknown system category %q", category)
		}
	}
	return map[string]any{
		"categories":   categories,
		"generated_at": now.UTC().Format(time.RFC3339),
		"system":       system,
	}, nil
}

func collectHostOS() map[string]any {
	info := map[string]any{"goos": runtime.GOOS, "goarch": runtime.GOARCH}
	if hostname, err := os.Hostname(); err != nil {
		info["warnings"] = []string{err.Error()}
	} else {
		info["hostname"] = hostname
	}
	return info
}

func collectHostRuntime() map[string]any {
	info := map[string]any{
		"go_version": runtime.Version(),
		"compiler":   runtime.Compiler,
		"process_id": os.Getpid(),
		"parent_id":  os.Getppid(),
	}
	if executable, err := os.Executable(); err != nil {
		info["warnings"] = []string{err.Error()}
	} else {
		info["executable"] = executable
	}
	return info
}

func collectHostUser() map[string]any {
	info := map[string]any{}
	current, err := user.Current()
	if err != nil {
		info["warnings"] = []string{err.Error()}
		if username := strings.TrimSpace(os.Getenv("USER")); username != "" {
			info["username"] = username
		}
		if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
			info["home_dir"] = home
		}
		return info
	}
	info["username"] = current.Username
	info["name"] = current.Name
	info["uid"] = current.Uid
	info["gid"] = current.Gid
	info["home_dir"] = current.HomeDir
	return info
}

func collectHostPaths() map[string]any {
	info := map[string]any{"temp_dir": os.TempDir()}
	var warnings []string
	if workingDir, err := os.Getwd(); err != nil {
		warnings = append(warnings, "working_dir: "+err.Error())
	} else {
		info["working_dir"] = workingDir
	}
	if homeDir, err := os.UserHomeDir(); err != nil {
		warnings = append(warnings, "home_dir: "+err.Error())
	} else {
		info["home_dir"] = homeDir
	}
	if executable, err := os.Executable(); err != nil {
		warnings = append(warnings, "executable: "+err.Error())
	} else {
		info["executable"] = executable
	}
	if len(warnings) > 0 {
		info["warnings"] = warnings
	}
	return info
}

func collectHostTime(now time.Time) map[string]any {
	name, offset := now.Zone()
	return map[string]any{
		"utc":            now.UTC().Format(time.RFC3339),
		"local":          now.Format(time.RFC3339),
		"unix":           now.Unix(),
		"timezone":       time.Local.String(),
		"zone_name":      name,
		"offset_seconds": offset,
	}
}

func collectHostEnv() map[string]any {
	keys := []string{
		"PATH",
		"HOME",
		"USER",
		"TMPDIR",
		"TEMP",
		"TMP",
		"GOCACHE",
		"GOPATH",
		"GOMODCACHE",
		"GOENV",
		"GOROOT",
		"GOPROXY",
		"GOSUMDB",
		"GONOSUMDB",
		"GOPRIVATE",
		"GONOPROXY",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"NO_PROXY",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
		"XDG_CACHE_HOME",
		"DEX_HOME",
		"DEX_HOST_CMD",
	}
	values := map[string]string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	return map[string]any{"values": values}
}

func collectHostNetwork() map[string]any {
	info := map[string]any{}
	if proxies := hostProxyEnv(); len(proxies) > 0 {
		info["proxies"] = proxies
	}
	var warnings []string
	if hostname, err := os.Hostname(); err != nil {
		warnings = append(warnings, "hostname: "+err.Error())
	} else {
		info["hostname"] = hostname
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		warnings = append(warnings, "interfaces: "+err.Error())
	} else {
		info["interfaces"] = hostNetworkInterfaces(interfaces)
	}
	if dns, ok := hostDNSInfo(); ok {
		info["dns"] = dns
	}
	if len(warnings) > 0 {
		info["warnings"] = warnings
	}
	return info
}

func hostNetworkInterfaces(interfaces []net.Interface) []map[string]any {
	out := make([]map[string]any, 0, len(interfaces))
	for _, iface := range interfaces {
		item := map[string]any{
			"name":          iface.Name,
			"index":         iface.Index,
			"mtu":           iface.MTU,
			"flags":         splitHostFlags(iface.Flags.String()),
			"hardware_addr": iface.HardwareAddr.String(),
		}
		addrs, err := iface.Addrs()
		if err != nil {
			item["warnings"] = []string{err.Error()}
		} else {
			values := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				values = append(values, addr.String())
			}
			sort.Strings(values)
			if len(values) > 0 {
				item["addrs"] = values
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := out[i]["name"].(string)
		right, _ := out[j]["name"].(string)
		return left < right
	})
	return out
}

func splitHostFlags(flags string) []string {
	if strings.TrimSpace(flags) == "" || flags == "0" {
		return nil
	}
	parts := strings.Split(flags, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func hostDNSInfo() (map[string]any, bool) {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, false
	}
	info := map[string]any{"source": "/etc/resolv.conf"}
	var nameservers, search, options []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			nameservers = append(nameservers, fields[1:]...)
		case "search":
			search = append(search, fields[1:]...)
		case "domain":
			search = append(search, fields[1])
		case "options":
			options = append(options, fields[1:]...)
		}
	}
	if len(nameservers) > 0 {
		info["nameservers"] = nameservers
	}
	if len(search) > 0 {
		info["search"] = search
	}
	if len(options) > 0 {
		info["options"] = options
	}
	return info, true
}

func hostProxyEnv() map[string]string {
	keys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"}
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			out[key] = value
		}
	}
	return out
}
