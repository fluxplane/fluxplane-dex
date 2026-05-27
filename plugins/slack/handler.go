package slack

import (
	"encoding/json"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type Handler struct {
	Runner OperationRunner
}

func NewHandler() Handler {
	return Handler{Runner: NewOperationRunner()}
}

func Handle(req protocol.Request) protocol.Response {
	return NewHandler().Handle(req)
}

func (h Handler) Handle(req protocol.Request) protocol.Response {
	switch req.Command {
	case protocol.CommandManifest:
		return protocol.OK(Manifest())
	case protocol.CommandAuthMethods:
		return protocol.OK(Manifest().Auth)
	case protocol.CommandAuthTest:
		return pluginutil.OKText("Slack auth is host-managed; use dex auth status slack", map[string]any{"status": "host_managed"})
	case protocol.CommandAuthConnect:
		return pluginutil.OKText("Use dex auth connect slack --field user_token=<token> --field bot_token=<token>", nil)
	case protocol.CommandOperationsList:
		return protocol.OK(Manifest().Operations)
	case protocol.CommandOperationsCall:
		return h.callOne(req)
	case protocol.CommandOperationsBatch:
		return h.callBatch(req)
	case protocol.CommandDatasourcesList:
		return protocol.OK(Manifest().Datasources)
	case protocol.CommandDatasourcesSearch:
		return protocol.Fail("not_implemented", "slack datasource live search requires host index integration")
	case protocol.CommandDatasourcesGet:
		return protocol.Fail("not_implemented", "slack datasource get requires host index integration")
	case protocol.CommandDatasourcesLookup:
		return protocol.Fail("not_implemented", "slack datasource lookup requires host index integration")
	case protocol.CommandContextBuild:
		return pluginutil.OKData(map[string]any{"blocks": []core.ContextBlock{}})
	case protocol.CommandEndpointsDiscover:
		return pluginutil.OKData(map[string]any{"candidates": []core.EndpointCandidate{}})
	case protocol.CommandIndexBuild:
		return h.indexBuild(req)
	case protocol.CommandIndexStatus:
		return pluginutil.OKText("Slack index is host-owned", map[string]any{"status": "host_owned"})
	default:
		return protocol.Fail("unknown_command", "slack plugin does not implement "+req.Command)
	}
}

func (h Handler) callOne(req protocol.Request) protocol.Response {
	call, err := protocol.DecodePayload[protocol.OperationCall](req.Payload)
	if err != nil {
		return protocol.Fail("bad_payload", err.Error())
	}
	result := h.runner().Run(req, call, nil)
	if !result.OK {
		return protocol.Response{Protocol: protocol.Version, OK: false, Error: result.Error}
	}
	return protocol.Response{Protocol: protocol.Version, OK: true, Result: result.Result}
}

func (h Handler) callBatch(req protocol.Request) protocol.Response {
	batch, err := protocol.DecodePayload[protocol.OperationBatch](req.Payload)
	if err != nil {
		return protocol.Fail("bad_payload", err.Error())
	}
	secrets := map[string]pluginutil.SecretMaterial{}
	results := make([]protocol.OperationResult, 0, len(batch.Calls))
	for _, call := range batch.Calls {
		results = append(results, h.runner().Run(req, call, secrets))
	}
	return protocol.OK(protocol.OperationBatchResult{Results: results})
}

func (h Handler) indexBuild(req protocol.Request) protocol.Response {
	if req.Grant == "" {
		return pluginutil.OKText("Use dex op run slack.index.build to build live Slack directory records", map[string]any{"status": "requires_operation_grant"})
	}
	result := h.runner().Run(req, protocol.OperationCall{Name: "slack.index.build", Input: req.Payload}, nil)
	if !result.OK {
		return protocol.Response{Protocol: protocol.Version, OK: false, Error: result.Error}
	}
	var value any
	if err := json.Unmarshal(result.Result, &value); err != nil {
		return protocol.Fail("marshal_result", err.Error())
	}
	return pluginutil.OKData(value)
}

func (h Handler) runner() OperationRunner {
	if h.Runner.SecretGetter == nil && h.Runner.ClientFactory == nil {
		return NewOperationRunner()
	}
	return h.Runner
}
