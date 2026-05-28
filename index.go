package dex

import (
	"context"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/protocol"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

// IndexService manages host-side indexes built from plugin data.
type IndexService struct {
	engine *Engine
}

// IndexBuildResult describes the outcome of an index build.
type IndexBuildResult = runtime.IndexBuildResult

// IndexStatus reports current state for a plugin/instance's indexes.
type IndexStatus = runtime.IndexStatus

// IndexSnapshot is a single saved index snapshot.
type IndexSnapshot = runtime.IndexSnapshot

// Build runs the plugin's index.build operation and persists records.
func (s *IndexService) Build(ctx context.Context, plugin, instance string, input any) (IndexBuildResult, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(plugin); !ok {
		return IndexBuildResult{}, fmt.Errorf("%w: %q", ErrPluginNotFound, plugin)
	}
	return s.engine.runner.BuildIndex(ctx, plugin, instance, input)
}

// Status returns the current persisted index status for plugin/instance.
func (s *IndexService) Status(_ context.Context, plugin, instance string) (IndexStatus, error) {
	return s.engine.runner.State.IndexStatus(plugin, instance)
}

// PluginStatus invokes the plugin's own index.status command (live view).
func (s *IndexService) PluginStatus(ctx context.Context, plugin, instance string, payload any) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandIndexStatus, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}
