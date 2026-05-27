package protocol

import (
	"encoding/json"
	"fmt"
)

const Version = "dex.plugin.v1"

const (
	CommandManifest          = "manifest"
	CommandAuthMethods       = "auth.methods"
	CommandAuthTest          = "auth.test"
	CommandAuthConnect       = "auth.connect"
	CommandOperationsList    = "operations.list"
	CommandOperationsCall    = "operations.call"
	CommandOperationsBatch   = "operations.call_batch"
	CommandDatasourcesList   = "datasources.list"
	CommandDatasourcesSearch = "datasources.search"
	CommandDatasourcesGet    = "datasources.get"
	CommandDatasourcesLookup = "datasources.lookup"
	CommandContextBuild      = "context.build"
	CommandEndpointsDiscover = "endpoints.discover"
	CommandIndexBuild        = "index.build"
	CommandIndexStatus       = "index.status"
)

type Request struct {
	Protocol string          `json:"protocol"`
	Command  string          `json:"command"`
	Plugin   string          `json:"plugin,omitempty"`
	Instance string          `json:"instance,omitempty"`
	Grant    string          `json:"secret_grant,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type Response struct {
	Protocol string          `json:"protocol"`
	OK       bool            `json:"ok"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type OperationCall struct {
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

type OperationBatch struct {
	Calls []OperationCall `json:"calls"`
}

type OperationResult struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

type OperationBatchResult struct {
	Results []OperationResult `json:"results"`
}

func NewRequest(command, plugin string, payload any) (Request, error) {
	raw, err := marshalPayload(payload)
	if err != nil {
		return Request{}, err
	}
	return Request{Protocol: Version, Command: command, Plugin: plugin, Payload: raw}, nil
}

func OK(result any) Response {
	raw, err := marshalPayload(result)
	if err != nil {
		return Fail("marshal_result", err.Error())
	}
	return Response{Protocol: Version, OK: true, Result: raw}
}

func Fail(code, message string) Response {
	return Response{Protocol: Version, OK: false, Error: &Error{Code: code, Message: message}}
}

func DecodePayload[T any](raw json.RawMessage) (T, error) {
	var out T
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode payload: %w", err)
	}
	return out, nil
}

func marshalPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		return raw, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return data, nil
}
