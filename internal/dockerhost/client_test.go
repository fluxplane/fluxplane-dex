package dockerhost

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/go-connections/nat"
	. "github.com/fluxplane/fluxplane-dex/plugins/docker"
)

func TestContainerCreateConfigMapsDockerOptions(t *testing.T) {
	config, hostConfig, networkingConfig, platform, err := containerCreateConfig(ContainerCreateInput{
		Image:      "alpine:latest",
		Name:       "api",
		Cmd:        []string{"sleep", "60"},
		Env:        []string{"APP_ENV=test", ""},
		Network:    "app-net",
		Restart:    "unless-stopped",
		Binds:      []string{"/host:/container:ro"},
		Mounts:     []MountInput{{Type: "volume", Source: "cache", Target: "/cache", ReadOnly: true}},
		Ports:      []PortInput{{Container: "8080/tcp", HostIP: "127.0.0.1", HostPort: "18080"}},
		Platform:   "linux/amd64",
		AutoRemove: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Image != "alpine:latest" || len(config.Cmd) != 2 || len(config.Env) != 1 {
		t.Fatalf("config = %#v", config)
	}
	port, ok := firstExposedPort(config.ExposedPorts)
	if !ok || string(port) != "8080/tcp" {
		t.Fatalf("exposed ports = %#v", config.ExposedPorts)
	}
	if hostConfig.NetworkMode != container.NetworkMode("app-net") || hostConfig.RestartPolicy.Name != container.RestartPolicyUnlessStopped {
		t.Fatalf("host config = %#v", hostConfig)
	}
	if !hostConfig.AutoRemove || len(hostConfig.Binds) != 1 || len(hostConfig.Mounts) != 1 || hostConfig.Mounts[0].Type != mount.TypeVolume {
		t.Fatalf("host config mounts = %#v", hostConfig)
	}
	if got := hostConfig.PortBindings[port][0]; got.HostIP != "127.0.0.1" || got.HostPort != "18080" {
		t.Fatalf("port binding = %#v", got)
	}
	if networkingConfig.EndpointsConfig["app-net"] == nil {
		t.Fatalf("networking config = %#v", networkingConfig)
	}
	if platform == nil || platform.OS != "linux" || platform.Architecture != "amd64" {
		t.Fatalf("platform = %#v", platform)
	}
}

func TestTarBuildContextHonorsDockerignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dockerignore"), []byte(".git\nignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := tarBuildContext(root, "")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	reader := tar.NewReader(body)
	seen := map[string]bool{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[header.Name] = true
	}
	if !seen["Dockerfile"] {
		t.Fatalf("tar entries = %#v", seen)
	}
	if seen[".git/"] || seen[".git/config"] || seen["ignored.txt"] {
		t.Fatalf("ignored files were included: %#v", seen)
	}
}

func TestRegistryAuthHelpers(t *testing.T) {
	header, err := registryAuthHeader("", RegistryAuthInput{Username: "user", Password: "secret", ServerAddress: "registry.local"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := registry.DecodeAuthConfig(header)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Username != "user" || decoded.Password != "secret" || decoded.ServerAddress != "registry.local" {
		t.Fatalf("decoded auth = %#v", decoded)
	}
	configs, err := buildAuthConfigs(ImageBuildInput{
		Tags: []string{"registry.local/app:test"},
		Auth: RegistryAuthInput{Username: "builder", Password: "secret"},
		AuthConfigs: map[string]RegistryAuthInput{
			"mirror.local": {Username: "mirror", Password: "secret"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if configs["registry.local"].Username != "builder" || configs["mirror.local"].Username != "mirror" {
		t.Fatalf("configs = %#v", configs)
	}
}

func TestTarLocalPathAndExtractTarToDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "input.txt")
	if err := os.WriteFile(source, []byte("copied"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, files, bytes, err := tarLocalPath(source)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if len(files) != 1 || files[0] != "input.txt" || bytes != 6 {
		t.Fatalf("tar result files=%#v bytes=%d", files, bytes)
	}
	dest := t.TempDir()
	extracted, extractedBytes, err := extractTarToDirectory(body, dest, false)
	if err != nil {
		t.Fatal(err)
	}
	if extractedBytes != 6 || len(extracted) != 1 {
		t.Fatalf("extract result files=%#v bytes=%d", extracted, extractedBytes)
	}
	data, err := os.ReadFile(filepath.Join(dest, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "copied" {
		t.Fatalf("data = %q", data)
	}
}

func TestExtractTarRejectsUnsafePath(t *testing.T) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("bad"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, err := extractTarToDirectory(strings.NewReader(buf.String()), t.TempDir(), false)
	if err == nil {
		t.Fatal("expected unsafe archive path error")
	}
}

func TestExtractTarRejectsExistingSymlinkAncestor(t *testing.T) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: "link/escape", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("bad"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dest, "link")); err != nil {
		t.Fatal(err)
	}
	_, _, err := extractTarToDirectory(bytes.NewReader(buf.Bytes()), dest, true)
	if err == nil {
		t.Fatal("expected symlink ancestor error")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
		t.Fatalf("archive escaped destination, stat err = %v", err)
	}
}

func firstExposedPort(values nat.PortSet) (nat.Port, bool) {
	for value := range values {
		return value, true
	}
	return "", false
}
