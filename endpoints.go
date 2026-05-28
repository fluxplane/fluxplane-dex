package dex

import (
	"context"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

// EndpointService manages the endpoint registry.
type EndpointService struct {
	engine *Engine
}

// EndpointRef is the canonical endpoint reference.
type EndpointRef = core.EndpointRef

// EndpointRecord is an entry in the saved registry, including health info.
type EndpointRecord = runtime.EndpointRecord

// EndpointHealth captures a probe result.
type EndpointHealth = runtime.EndpointHealth

// List returns saved endpoints, optionally filtered by product (e.g. "github").
func (s *EndpointService) List(_ context.Context, product string) ([]EndpointRecord, error) {
	return s.engine.runner.State.ListEndpoints(product)
}

// Get returns a single endpoint by id.
func (s *EndpointService) Get(_ context.Context, id string) (EndpointRecord, bool, error) {
	return s.engine.runner.State.GetEndpoint(id)
}

// Save stores an endpoint ref. Validates url and assigns an id if absent.
func (s *EndpointService) Save(_ context.Context, ref EndpointRef) (EndpointRecord, error) {
	return s.engine.runner.State.SaveEndpoint(ref)
}

// SaveHealth records the result of a health probe against an endpoint.
func (s *EndpointService) SaveHealth(_ context.Context, id string, health EndpointHealth) (EndpointRecord, error) {
	return s.engine.runner.State.SaveEndpointHealth(id, health)
}

// Remove deletes an endpoint by id. Returns true if a record was removed.
func (s *EndpointService) Remove(_ context.Context, id string) (bool, error) {
	return s.engine.runner.State.RemoveEndpoint(id)
}

// Discover asks a plugin for endpoint candidates (manifest-declared discovery flow).
func (s *EndpointService) Discover(ctx context.Context, plugin string, payload any) (Response, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(plugin); !ok {
		return Response{}, fmt.Errorf("%w: %q", ErrPluginNotFound, plugin)
	}
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, "", protocol.CommandEndpointsDiscover, payload)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}
