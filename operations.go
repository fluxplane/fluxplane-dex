package dex

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

// OperationService invokes plugin operations.
type OperationService struct {
	engine *Engine
}

// OperationSpec mirrors core.OperationSpec.
type OperationSpec = core.OperationSpec

// OperationCall describes a single operation invocation in a batch.
type OperationCall = protocol.OperationCall

// OperationBatchResult is the result of a batch invocation.
type OperationBatchResult = protocol.OperationBatchResult

// Response is the raw plugin response (json.RawMessage Result + optional Error).
type Response = protocol.Response

// List returns the operations declared in the plugin manifest.
func (s *OperationService) List(ctx context.Context, plugin string) ([]OperationSpec, error) {
	manifest, err := s.engine.Manifest(ctx, plugin)
	if err != nil {
		return nil, err
	}
	return manifest.Operations, nil
}

// Show returns a single operation spec from a plugin's manifest.
func (s *OperationService) Show(ctx context.Context, plugin, name string) (OperationSpec, error) {
	ops, err := s.List(ctx, plugin)
	if err != nil {
		return OperationSpec{}, err
	}
	for _, op := range ops {
		if op.Name == name {
			return op, nil
		}
	}
	return OperationSpec{}, fmt.Errorf("operation %q not declared by plugin %q", name, plugin)
}

// Run invokes a single operation on the default instance. payload may be a
// Go value (marshaled to JSON), json.RawMessage, or nil.
func (s *OperationService) Run(ctx context.Context, plugin, op string, payload any) (Response, error) {
	return s.RunInstance(ctx, plugin, "", op, payload)
}

// RunInstance invokes a single operation on a named instance. An empty
// instance string is normalized to "default".
func (s *OperationService) RunInstance(ctx context.Context, plugin, instance, op string, payload any) (Response, error) {
	input, err := marshalInput(payload)
	if err != nil {
		return Response{}, err
	}
	call := OperationCall{Name: op, Input: input}
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandOperationsCall, call)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}

// Batch invokes multiple operations against the default instance.
func (s *OperationService) Batch(ctx context.Context, plugin string, calls []OperationCall) (OperationBatchResult, error) {
	return s.engine.runner.OperationBatch(ctx, plugin, "", calls)
}

// BatchInstance invokes multiple operations against a named instance.
func (s *OperationService) BatchInstance(ctx context.Context, plugin, instance string, calls []OperationCall) (OperationBatchResult, error) {
	return s.engine.runner.OperationBatch(ctx, plugin, instance, calls)
}

func marshalInput(payload any) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(payload)
}
