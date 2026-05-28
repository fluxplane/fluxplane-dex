package dex

import (
	"context"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

// ContextService invokes plugin-side context.build commands.
type ContextService struct {
	engine *Engine
}

// ContextSpec mirrors core.ContextSpec.
type ContextSpec = core.ContextSpec

// List returns the context kinds declared by the plugin manifest.
func (s *ContextService) List(ctx context.Context, plugin string) ([]ContextSpec, error) {
	manifest, err := s.engine.Manifest(ctx, plugin)
	if err != nil {
		return nil, err
	}
	return manifest.Context, nil
}

// Build invokes the plugin's context.build command.
func (s *ContextService) Build(ctx context.Context, plugin string, payload any) (Response, error) {
	return s.BuildInstance(ctx, plugin, "", payload)
}

func (s *ContextService) BuildInstance(ctx context.Context, plugin, instance string, payload any) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandContextBuild, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}
