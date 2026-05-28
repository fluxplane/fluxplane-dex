package dex

import (
	"context"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

// DatasourceService queries plugin-declared datasources.
type DatasourceService struct {
	engine *Engine
}

// DatasourceSpec mirrors core.DatasourceSpec.
type DatasourceSpec = core.DatasourceSpec

// List returns the datasources declared in the plugin manifest.
func (s *DatasourceService) List(ctx context.Context, plugin string) ([]DatasourceSpec, error) {
	manifest, err := s.engine.Manifest(ctx, plugin)
	if err != nil {
		return nil, err
	}
	return manifest.Datasources, nil
}

// Search invokes the datasource search command. payload should carry
// {datasource, query, limit, entity, ...}.
func (s *DatasourceService) Search(ctx context.Context, plugin string, payload any) (Response, error) {
	return s.SearchInstance(ctx, plugin, "", payload)
}

// SearchInstance is Search bound to a specific instance.
func (s *DatasourceService) SearchInstance(ctx context.Context, plugin, instance string, payload any) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandDatasourcesSearch, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}

// Get fetches a single record from a datasource.
func (s *DatasourceService) Get(ctx context.Context, plugin string, payload any) (Response, error) {
	return s.GetInstance(ctx, plugin, "", payload)
}

func (s *DatasourceService) GetInstance(ctx context.Context, plugin, instance string, payload any) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandDatasourcesGet, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}

// Lookup resolves arbitrary terms (e.g. links, IDs, mentions) to records.
func (s *DatasourceService) Lookup(ctx context.Context, plugin string, payload any) (Response, error) {
	return s.LookupInstance(ctx, plugin, "", payload)
}

func (s *DatasourceService) LookupInstance(ctx context.Context, plugin, instance string, payload any) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandDatasourcesLookup, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}
