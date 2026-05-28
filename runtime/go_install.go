package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GoModuleVersion struct {
	Path    string    `json:"path"`
	Version string    `json:"version,omitempty"`
	Time    time.Time `json:"time,omitempty"`
}

func InstallGoTarget(ctx context.Context, target, binDir string) error {
	return InstallGoTargetWithLdflags(ctx, target, binDir, "")
}

func InstallLocalGoTarget(ctx context.Context, pluginDir, binary, binDir string) error {
	pluginDir = strings.TrimSpace(pluginDir)
	binary = strings.TrimSpace(binary)
	if pluginDir == "" {
		return fmt.Errorf("plugin dir is empty")
	}
	if binary == "" {
		return fmt.Errorf("plugin binary is empty")
	}
	return installGoTargetInDir(ctx, "./cmd/"+binary, binDir, "", pluginDir)
}

func InstallGoTargetWithLdflags(ctx context.Context, target, binDir, ldflags string) error {
	return installGoTargetInDir(ctx, target, binDir, ldflags, "")
}

func installGoTargetInDir(ctx context.Context, target, binDir, ldflags, dir string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("go install target is empty")
	}
	if strings.TrimSpace(binDir) == "" {
		return fmt.Errorf("go install bin dir is empty")
	}
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		return err
	}
	args := []string{"install"}
	if ldflags = strings.TrimSpace(ldflags); ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, target)
	cmd := exec.CommandContext(ctx, "go", args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = strings.TrimSpace(dir)
	}
	cmd.Env = withEnv(withEnv(os.Environ(), "GOBIN", binDir), "GO111MODULE", "on")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func LatestGoModuleVersion(ctx context.Context, target string) (GoModuleVersion, error) {
	return ResolveGoModuleVersion(ctx, target, "latest")
}

func ResolveGoModuleVersion(ctx context.Context, target, fallbackVersion string) (GoModuleVersion, error) {
	pkg, version := splitGoInstallTarget(target)
	if pkg == "" {
		return GoModuleVersion{}, fmt.Errorf("go target is empty")
	}
	if version == "" {
		version = strings.TrimSpace(fallbackVersion)
	}
	if version == "" {
		version = "latest"
	}
	var lastErr error
	for _, module := range goModuleCandidates(pkg) {
		info, err := goListModuleVersion(ctx, module+"@"+version)
		if err == nil {
			return info, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no module candidates for %q", pkg)
	}
	return GoModuleVersion{}, lastErr
}

func splitGoInstallTarget(target string) (string, string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", ""
	}
	pkg, version, ok := strings.Cut(target, "@")
	if !ok {
		return target, ""
	}
	return strings.TrimSpace(pkg), strings.TrimSpace(version)
}

func goModuleCandidates(pkg string) []string {
	pkg = strings.Trim(strings.TrimSpace(pkg), "/")
	if pkg == "" {
		return nil
	}
	parts := strings.Split(pkg, "/")
	var candidates []string
	for i := len(parts); i >= 3; i-- {
		candidates = append(candidates, strings.Join(parts[:i], "/"))
	}
	if len(parts) < 3 {
		candidates = append(candidates, pkg)
	}
	return dedupeStrings(candidates)
}

func goListModuleVersion(ctx context.Context, moduleTarget string) (GoModuleVersion, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", moduleTarget)
	cmd.Env = withEnv(os.Environ(), "GO111MODULE", "on")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return GoModuleVersion{}, fmt.Errorf("go list %s: %s", moduleTarget, msg)
	}
	var info GoModuleVersion
	if err := json.Unmarshal([]byte(stdout.String()), &info); err != nil {
		return GoModuleVersion{}, err
	}
	return info, nil
}

func insideDir(dir, path string) bool {
	dir = strings.TrimSpace(dir)
	path = strings.TrimSpace(path)
	if dir == "" || path == "" {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
