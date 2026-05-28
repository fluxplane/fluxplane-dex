package system

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func Info(_ pluginbinding.Context, input InfoInput) (InfoResult, error) {
	categories, err := selectCategories(input)
	if err != nil {
		return InfoResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	now := time.Now()
	out := InfoResult{
		Categories:  categories,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		System:      map[string]any{},
	}
	for _, category := range categories {
		out.System[category] = collectCategory(category, now)
	}
	return out, nil
}

type StringList []string

func (s *StringList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*s = splitSelectors(values...)
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("expected string or string array")
	}
	*s = splitSelectors(value)
	return nil
}

const (
	categoryOS      = "os"
	categoryRuntime = "runtime"
	categoryUser    = "user"
	categoryPaths   = "paths"
	categoryCPU     = "cpu"
	categoryTime    = "time"
	categoryEnv     = "env"
	categoryNetwork = "network"
)

var allCategories = []string{categoryOS, categoryRuntime, categoryUser, categoryPaths, categoryCPU, categoryTime, categoryEnv, categoryNetwork}

var categoryAliases = map[string]string{
	"arch":         categoryOS,
	"architecture": categoryOS,
	"cpus":         categoryCPU,
	"processor":    categoryCPU,
	"processors":   categoryCPU,
	"tmp":          categoryPaths,
	"temp":         categoryPaths,
	"tempdir":      categoryPaths,
	"timezone":     categoryTime,
}

func selectCategories(input InfoInput) ([]string, error) {
	requested := append([]string{}, input.Categories...)
	requested = append(requested, splitSelectors(input.Category)...)
	requested = append(requested, input.Include...)
	selected := map[string]bool{}
	if len(requested) == 0 {
		for _, category := range allCategories {
			selected[category] = true
		}
	} else {
		for _, value := range requested {
			category, err := normalizeCategory(value)
			if err != nil {
				return nil, err
			}
			selected[category] = true
		}
	}
	for _, value := range input.Exclude {
		category, err := normalizeCategory(value)
		if err != nil {
			return nil, err
		}
		delete(selected, category)
	}
	out := make([]string, 0, len(selected))
	for _, category := range allCategories {
		if selected[category] {
			out = append(out, category)
		}
	}
	return out, nil
}

func normalizeCategory(value string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(value))
	key = strings.ReplaceAll(key, "-", "_")
	if key == "" {
		return "", fmt.Errorf("empty category")
	}
	if alias, ok := categoryAliases[key]; ok {
		key = alias
	}
	for _, category := range allCategories {
		if key == category {
			return category, nil
		}
	}
	return "", fmt.Errorf("unknown category %q", value)
}

func splitSelectors(values ...string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func collectCategory(category string, now time.Time) any {
	switch category {
	case categoryOS:
		return collectOS()
	case categoryRuntime:
		return collectRuntime()
	case categoryUser:
		return collectUser()
	case categoryPaths:
		return collectPaths()
	case categoryCPU:
		return CPUInfo{LogicalCPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0)}
	case categoryTime:
		return collectTime(now)
	case categoryEnv:
		return collectEnv()
	case categoryNetwork:
		return collectNetwork()
	default:
		return nil
	}
}

func collectOS() OSInfo {
	info := OSInfo{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	hostname, err := os.Hostname()
	if err != nil {
		info.Warnings = append(info.Warnings, err.Error())
	} else {
		info.Hostname = hostname
	}
	return info
}

func collectRuntime() RuntimeInfo {
	info := RuntimeInfo{
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		ProcessID: os.Getpid(),
		ParentID:  os.Getppid(),
	}
	executable, err := os.Executable()
	if err != nil {
		info.Warnings = append(info.Warnings, err.Error())
	} else {
		info.Executable = executable
	}
	return info
}

func collectUser() UserInfo {
	info := UserInfo{}
	current, err := user.Current()
	if err != nil {
		info.Warnings = append(info.Warnings, err.Error())
		if username := strings.TrimSpace(os.Getenv("USER")); username != "" {
			info.Username = username
		}
		if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
			info.HomeDir = home
		}
		return info
	}
	info.Username = current.Username
	info.Name = current.Name
	info.UID = current.Uid
	info.GID = current.Gid
	info.HomeDir = current.HomeDir
	return info
}

func collectPaths() PathsInfo {
	info := PathsInfo{TempDir: os.TempDir()}
	workingDir, err := os.Getwd()
	if err != nil {
		info.Warnings = append(info.Warnings, "working_dir: "+err.Error())
	} else {
		info.WorkingDir = workingDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		info.Warnings = append(info.Warnings, "home_dir: "+err.Error())
	} else {
		info.HomeDir = homeDir
	}
	executable, err := os.Executable()
	if err != nil {
		info.Warnings = append(info.Warnings, "executable: "+err.Error())
	} else {
		info.Executable = executable
	}
	return info
}

func collectTime(now time.Time) TimeInfo {
	name, offset := now.Zone()
	return TimeInfo{
		UTC:          now.UTC().Format(time.RFC3339),
		Local:        now.Format(time.RFC3339),
		Unix:         now.Unix(),
		Timezone:     time.Local.String(),
		ZoneName:     name,
		OffsetSecond: offset,
	}
}

func collectEnv() EnvInfo {
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
	return EnvInfo{Values: values}
}

func collectNetwork() NetworkInfo {
	info := NetworkInfo{Proxies: proxyEnv()}
	hostname, err := os.Hostname()
	if err != nil {
		info.Warnings = append(info.Warnings, "hostname: "+err.Error())
	} else {
		info.Hostname = hostname
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		info.Warnings = append(info.Warnings, "interfaces: "+err.Error())
	} else {
		info.Interfaces = networkInterfaces(interfaces)
	}
	if dns, ok := readDNSInfo(); ok {
		info.DNS = &dns
	}
	return info
}

func networkInterfaces(interfaces []net.Interface) []NetworkInterface {
	out := make([]NetworkInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		item := NetworkInterface{
			Name:         iface.Name,
			Index:        iface.Index,
			MTU:          iface.MTU,
			Flags:        splitFlags(iface.Flags.String()),
			HardwareAddr: iface.HardwareAddr.String(),
		}
		addrs, err := iface.Addrs()
		if err != nil {
			item.Warnings = append(item.Warnings, err.Error())
		} else {
			for _, addr := range addrs {
				item.Addrs = append(item.Addrs, addr.String())
			}
			sort.Strings(item.Addrs)
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func splitFlags(flags string) []string {
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

func readDNSInfo() (DNSInfo, bool) {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return DNSInfo{}, false
	}
	info := DNSInfo{Source: "/etc/resolv.conf"}
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
			info.Nameservers = append(info.Nameservers, fields[1:]...)
		case "search":
			info.Search = append(info.Search, fields[1:]...)
		case "domain":
			info.Search = append(info.Search, fields[1])
		case "options":
			info.Options = append(info.Options, fields[1:]...)
		}
	}
	return info, true
}

func proxyEnv() map[string]string {
	keys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"}
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
