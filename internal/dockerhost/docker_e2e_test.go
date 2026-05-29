package dockerhost

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/fluxplane/fluxplane-dex/plugins/docker"
)

func TestDockerE2EContainerImageNetworkVolumeAndCopy(t *testing.T) {
	if os.Getenv("DEX_DOCKER_E2E") != "1" {
		t.Skip("set DEX_DOCKER_E2E=1 to run Docker daemon integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	suffix := strings.ToLower(time.Now().Format("20060102150405"))
	imageRef := "fluxplane-dex-e2e:" + suffix
	taggedRef := "fluxplane-dex-e2e-tagged:" + suffix
	containerName := "fluxplane-dex-e2e-" + suffix
	networkName := "fluxplane-dex-e2e-" + suffix
	volumeName := "fluxplane-dex-e2e-" + suffix
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = client.ContainerRemove(cleanupCtx, ContainerRemoveInput{ID: containerName, Force: true, Volumes: true})
		_, _ = client.ImageRemove(cleanupCtx, ImageRemoveInput{ID: imageRef, Force: true})
		_, _ = client.ImageRemove(cleanupCtx, ImageRemoveInput{ID: taggedRef, Force: true})
		_, _ = client.NetworkRemove(cleanupCtx, NetworkRemoveInput{ID: networkName})
		_, _ = client.VolumeRemove(cleanupCtx, VolumeRemoveInput{ID: volumeName, Force: true})
	})

	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.20\nRUN echo built > /built.txt\nCMD [\"sh\", \"-c\", \"sleep 60\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildResult, err := client.ImageBuild(ctx, ImageBuildInput{ContextPath: contextDir, Tags: []string{imageRef}, Pull: true, Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !buildResult.OK || buildResult.Count == 0 {
		t.Fatalf("build = %#v", buildResult)
	}
	if _, err := client.ImageTag(ctx, ImageTagInput{Source: imageRef, Target: taggedRef}); err != nil {
		t.Fatal(err)
	}
	networkResult, err := client.NetworkCreate(ctx, NetworkCreateInput{Name: networkName, Driver: "bridge"})
	if err != nil {
		t.Fatal(err)
	}
	if !networkResult.OK || networkResult.ID == "" {
		t.Fatalf("network = %#v", networkResult)
	}
	volumeResult, err := client.VolumeCreate(ctx, VolumeCreateInput{Name: volumeName, Driver: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if volumeResult.Name != volumeName {
		t.Fatalf("volume = %#v", volumeResult)
	}
	runResult, err := client.ContainerRun(ctx, ContainerCreateInput{
		Image:   imageRef,
		Name:    containerName,
		Network: networkName,
		Mounts:  []MountInput{{Type: "volume", Source: volumeName, Target: "/data"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !runResult.OK || !runResult.Started {
		t.Fatalf("run = %#v", runResult)
	}
	execResult, err := client.ContainerExec(ctx, ContainerExecInput{ID: containerName, Cmd: []string{"cat", "/built.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(execResult.Stdout) != "built" {
		t.Fatalf("exec = %#v", execResult)
	}
	localFile := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(localFile, []byte("copied\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	copyTo, err := client.ContainerCopyTo(ctx, ContainerCopyToInput{ID: containerName, SourcePath: localFile, DestinationPath: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if !copyTo.OK || copyTo.Bytes == 0 {
		t.Fatalf("copy to = %#v", copyTo)
	}
	copiedExec, err := client.ContainerExec(ctx, ContainerExecInput{ID: containerName, Cmd: []string{"cat", "/tmp/input.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(copiedExec.Stdout) != "copied" {
		t.Fatalf("copied exec = %#v", copiedExec)
	}
	copyOutDir := t.TempDir()
	copyFrom, err := client.ContainerCopyFrom(ctx, ContainerCopyFromInput{ID: containerName, SourcePath: "/built.txt", DestinationPath: copyOutDir})
	if err != nil {
		t.Fatal(err)
	}
	if !copyFrom.OK || copyFrom.Bytes == 0 {
		t.Fatalf("copy from = %#v", copyFrom)
	}
	data, err := os.ReadFile(filepath.Join(copyOutDir, "built.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "built" {
		t.Fatalf("copied file = %q", data)
	}
}
