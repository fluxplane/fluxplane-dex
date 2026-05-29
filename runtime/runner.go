package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const maxPluginOutputBytes = 4 * 1024 * 1024

type Runner struct {
	Marketplace  Marketplace
	State        State
	DevPlugins   map[string]string
	WorkDir      string
	Timeout      time.Duration
	HostCommand  string
	EventSink    func(context.Context, PluginEvent)
	Capabilities CapabilityHost
	Providers    map[string]HostProvider
}

type HostProvider interface {
	Call(ctx context.Context, action string, payload json.RawMessage) (json.RawMessage, error)
}

type CapabilityHost interface {
	HTTP(context.Context, pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error)
	BlobRead(context.Context, pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error)
	BlobWrite(context.Context, pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error)
	BlobInfo(context.Context, pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error)
	EnvLookup(context.Context, pluginbinding.EnvLookupRequest) (pluginbinding.EnvLookupResponse, error)
	ProviderCall(context.Context, pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error)
}

type PluginEvent struct {
	Plugin   string          `json:"plugin"`
	Instance string          `json:"instance"`
	Command  string          `json:"command,omitempty"`
	Event    string          `json:"event"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

func (r Runner) Invoke(ctx context.Context, pluginName, command string, payload any) (protocol.Response, error) {
	return r.InvokeInstance(ctx, pluginName, DefaultInstance, command, payload)
}

func (r Runner) InvokeInstance(ctx context.Context, pluginName, instance, command string, payload any) (protocol.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	entry, ok := r.Marketplace.Resolve(pluginName)
	if !ok {
		return protocol.Response{}, fmt.Errorf("unknown plugin %q", pluginName)
	}
	req, err := protocol.NewRequest(command, entry.Name, payload)
	if err != nil {
		return protocol.Response{}, err
	}
	req.Instance = NormalizeInstance(instance)
	if command == protocol.CommandOperationsCall || command == protocol.CommandOperationsBatch {
		if err := r.resolveEndpointRefs(ctx, entry.Name, &req); err != nil {
			return protocol.Response{}, err
		}
	}
	if isDatasourceCommand(command) {
		if err := r.resolveDatasourceEndpointRef(ctx, entry.Name, &req); err != nil {
			return protocol.Response{}, err
		}
	}
	if isDatasourceCommand(command) {
		resp, ok, err := (hostIndexDatasource{state: r.State, plugin: entry.Name, instance: req.Instance}).Response(command, payload)
		if err != nil {
			return protocol.Response{}, err
		}
		if ok {
			return resp, nil
		}
	}
	if isBuiltinPlugin(entry) {
		resp, err := r.invokeBuiltin(ctx, entry, req)
		if err != nil {
			return protocol.Response{}, err
		}
		return r.enrichDatasourceResponse(ctx, entry.Name, req.Instance, command, payload, resp)
	}
	if command == protocol.CommandOperationsCall || command == protocol.CommandOperationsBatch {
		operations, purposes, capabilities := r.operationGrantScope(ctx, entry.Name, payload)
		grant, err := r.State.CreateGrantWithCapabilities(entry.Name, req.Instance, operations, purposes, capabilities, 5*time.Minute)
		if err != nil {
			return protocol.Response{}, err
		}
		req.Grant = grant.Token
	}
	if command == protocol.CommandEndpointsDiscover {
		grant, err := r.State.CreateGrantWithCapabilities(entry.Name, req.Instance, []string{command}, nil, []CapabilityGrant{{Name: pluginbinding.CapabilityProvider, Provider: "kubernetes", Action: "*"}}, 5*time.Minute)
		if err != nil {
			return protocol.Response{}, err
		}
		req.Grant = grant.Token
	}
	if command == protocol.CommandDatasourcesSearch || command == protocol.CommandDatasourcesGet || command == protocol.CommandDatasourcesLookup {
		operations, purposes, capabilities := r.datasourceGrantScope(ctx, entry.Name, command, payload)
		if len(purposes) > 0 || len(capabilities) > 0 {
			grant, err := r.State.CreateGrantWithCapabilities(entry.Name, req.Instance, operations, purposes, capabilities, 5*time.Minute)
			if err != nil {
				return protocol.Response{}, err
			}
			req.Grant = grant.Token
		}
	}
	resp, err := r.invokeRequest(ctx, entry, req)
	if err != nil {
		return protocol.Response{}, err
	}
	return r.enrichDatasourceResponse(ctx, entry.Name, req.Instance, command, payload, resp)
}

func (r Runner) resolveEndpointRefs(ctx context.Context, plugin string, req *protocol.Request) error {
	if req == nil || len(req.Payload) == 0 {
		return nil
	}
	operationInputs := r.operationInputSchemas(ctx, plugin)
	switch req.Command {
	case protocol.CommandOperationsCall:
		var call protocol.OperationCall
		if err := json.Unmarshal(req.Payload, &call); err != nil {
			return nil
		}
		changed, err := r.resolveOperationEndpointRefWithMode(plugin, &call, schemaHasProperty(operationInputs[call.Name], "url") || schemaHasProperty(operationInputs[call.Name], "credential_ref") || schemaHasProperty(operationInputs[call.Name], "endpoint_ref"))
		if err != nil {
			return err
		}
		if changed {
			raw, err := json.Marshal(call)
			if err != nil {
				return err
			}
			req.Payload = raw
		}
	case protocol.CommandOperationsBatch:
		var batch protocol.OperationBatch
		if err := json.Unmarshal(req.Payload, &batch); err != nil {
			return nil
		}
		changed := false
		for i := range batch.Calls {
			call := &batch.Calls[i]
			callChanged, err := r.resolveOperationEndpointRefWithMode(plugin, call, schemaHasProperty(operationInputs[call.Name], "url") || schemaHasProperty(operationInputs[call.Name], "credential_ref") || schemaHasProperty(operationInputs[call.Name], "endpoint_ref"))
			if err != nil {
				return err
			}
			changed = changed || callChanged
		}
		if changed {
			raw, err := json.Marshal(batch)
			if err != nil {
				return err
			}
			req.Payload = raw
		}
	}
	return nil
}

func (r Runner) resolveOperationEndpointRef(call *protocol.OperationCall) (bool, error) {
	return r.resolveOperationEndpointRefWithMode("", call, true)
}

func (r Runner) resolveOperationEndpointRefWithMode(plugin string, call *protocol.OperationCall, inject bool) (bool, error) {
	if call == nil || len(call.Input) == 0 {
		return false, nil
	}
	var input map[string]any
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return false, nil
	}
	defaultChanged := false
	var err error
	if inject {
		defaultChanged, err = r.injectDefaultEndpointRef(plugin, input)
		if err != nil {
			return false, err
		}
	}
	changed, err := r.resolveEndpointRefInput(input, inject)
	if err != nil {
		return false, err
	}
	changed = changed || defaultChanged
	if !changed {
		return false, nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	call.Input = raw
	return true, nil
}

func (r Runner) resolveDatasourceEndpointRef(ctx context.Context, plugin string, req *protocol.Request) error {
	_ = ctx
	if req == nil || len(req.Payload) == 0 {
		return nil
	}
	var input map[string]any
	if err := json.Unmarshal(req.Payload, &input); err != nil {
		return nil
	}
	defaultChanged, err := r.injectDefaultEndpointRef(plugin, input)
	if err != nil {
		return err
	}
	changed, err := r.resolveEndpointRefInput(input, true)
	if err != nil {
		return err
	}
	changed = changed || defaultChanged
	if !changed {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req.Payload = raw
	return nil
}

func (r Runner) injectDefaultEndpointRef(plugin string, input map[string]any) (bool, error) {
	if input == nil || strings.TrimSpace(plugin) == "" {
		return false, nil
	}
	if ref, _ := input["endpoint_ref"].(string); strings.TrimSpace(ref) != "" {
		return false, nil
	}
	endpoints, err := r.State.ListEndpoints(plugin)
	if err != nil || len(endpoints) != 1 {
		return false, err
	}
	input["endpoint_ref"] = endpoints[0].ID
	return true, nil
}

func (r Runner) resolveEndpointRefInput(input map[string]any, inject bool) (bool, error) {
	if len(input) == 0 {
		return false, nil
	}
	ref, _ := input["endpoint_ref"].(string)
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false, nil
	}
	endpoint, ok, err := r.State.GetEndpoint(ref)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("unknown endpoint_ref %q", ref)
	}
	if !inject {
		return false, nil
	}
	if value, _ := input["url"].(string); strings.TrimSpace(value) != "" {
		return false, nil
	}
	input["url"] = endpoint.URL
	if endpoint.CredentialRef != "" {
		input["credential_ref"] = endpoint.CredentialRef
	}
	if _, ok := input["endpoint_product"]; !ok && endpoint.Product != "" {
		input["endpoint_product"] = endpoint.Product
	}
	return true, nil
}

func (r Runner) operationInputSchemas(ctx context.Context, plugin string) map[string]json.RawMessage {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(manifest.Operations))
	for _, operation := range manifest.Operations {
		out[operation.Name] = operation.Input
	}
	return out
}

func schemaHasProperty(raw json.RawMessage, name string) bool {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &schema) != nil {
		return true
	}
	if len(schema.Properties) == 0 {
		return true
	}
	_, ok := schema.Properties[name]
	return ok
}

type IndexBuildResult struct {
	Plugin    string    `json:"plugin"`
	Instance  string    `json:"instance"`
	Index     string    `json:"index,omitempty"`
	Indexes   []string  `json:"indexes,omitempty"`
	Records   int       `json:"records"`
	Stored    bool      `json:"stored"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r Runner) BuildIndex(ctx context.Context, pluginName, instance string, input any) (IndexBuildResult, error) {
	entry, ok := r.Marketplace.Resolve(pluginName)
	if !ok {
		return IndexBuildResult{}, fmt.Errorf("unknown plugin %q", pluginName)
	}
	operation := entry.Name + ".index.build"
	inputRaw, err := json.Marshal(input)
	if err != nil {
		return IndexBuildResult{}, err
	}
	resp, err := r.InvokeInstance(ctx, entry.Name, instance, protocol.CommandOperationsCall, protocol.OperationCall{Name: operation, Input: inputRaw})
	if err != nil {
		return IndexBuildResult{}, err
	}
	var result struct {
		Index   string `json:"index"`
		Records []json.RawMessage
		Indexes []struct {
			Index    string            `json:"index"`
			Records  []json.RawMessage `json:"records"`
			Metadata json.RawMessage   `json:"metadata,omitempty"`
		} `json:"indexes"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return IndexBuildResult{}, err
	}
	indexes := result.Indexes
	if len(indexes) == 0 {
		indexName := result.Index
		if strings.TrimSpace(indexName) == "" {
			indexName = defaultIndexName(ctx, r, entry.Name)
		}
		indexes = append(indexes, struct {
			Index    string            `json:"index"`
			Records  []json.RawMessage `json:"records"`
			Metadata json.RawMessage   `json:"metadata,omitempty"`
		}{Index: indexName, Records: result.Records})
	}
	out := IndexBuildResult{Plugin: entry.Name, Instance: NormalizeInstance(instance), Stored: true}
	for _, index := range indexes {
		if strings.TrimSpace(index.Index) == "" {
			return IndexBuildResult{}, fmt.Errorf("index build result did not include index name")
		}
		snapshot, err := r.State.SaveIndexRecordsWithMetadata(entry.Name, instance, index.Index, index.Records, index.Metadata)
		if err != nil {
			return IndexBuildResult{}, err
		}
		out.Indexes = append(out.Indexes, snapshot.Index)
		out.Records += len(snapshot.Records)
		if snapshot.UpdatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = snapshot.UpdatedAt
		}
	}
	if len(out.Indexes) == 1 {
		out.Index = out.Indexes[0]
	}
	if err := r.State.ActivatePlugin(entry); err != nil {
		return IndexBuildResult{}, err
	}
	return out, nil
}

func (r Runner) invokeRequest(ctx context.Context, entry core.PluginEntry, req protocol.Request) (protocol.Response, error) {
	if req.Command == protocol.CommandManifest || !r.pluginUsesFramedProtocol(ctx, entry) {
		return r.invokeRequestV1(ctx, entry, req)
	}
	return r.invokeRequestV2(ctx, entry, req)
}

func (r Runner) pluginUsesFramedProtocol(ctx context.Context, entry core.PluginEntry) bool {
	if isBuiltinPlugin(entry) {
		return false
	}
	manifest, err := r.manifest(ctx, entry.Name)
	if err != nil {
		return false
	}
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		return false
	}
	return r.probeFramedProtocol(ctx, entry)
}

func (r Runner) probeFramedProtocol(ctx context.Context, entry core.PluginEntry) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	cmd, err := r.command(ctx, entry)
	if err != nil {
		return false
	}
	req, err := protocol.NewRequest(protocol.CommandManifest, entry.Name, nil)
	if err != nil {
		return false
	}
	frame, err := protocol.NewRequestFrame("probe", protocol.TargetPlugin, protocol.CommandManifest, req)
	if err != nil {
		return false
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return false
	}
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = r.pluginEnv()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false
	}
	var resp protocol.Frame
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return false
	}
	return resp.Protocol == protocol.Version && resp.Type == protocol.FrameResponse && resp.ID == "probe" && resp.OK
}

func (r Runner) invokeRequestV1(ctx context.Context, entry core.PluginEntry, req protocol.Request) (protocol.Response, error) {
	cmd, err := r.command(ctx, entry)
	if err != nil {
		return protocol.Response{}, err
	}
	req.Protocol = protocol.VersionV1
	data, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, err
	}
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = r.pluginEnv()
	var stdout, stderr limitedBuffer
	stdout.limit = maxPluginOutputBytes
	stderr.limit = maxPluginOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.truncated || stderr.truncated {
			return protocol.Response{}, fmt.Errorf("run plugin %s: plugin output exceeded %d bytes", entry.Name, maxPluginOutputBytes)
		}
		if stdout.Len() > 0 {
			resp, decodeErr := decodeResponse(stdout.Bytes())
			if resp.Protocol == protocol.Version || resp.Protocol == protocol.VersionV1 {
				return resp, decodeErr
			}
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return protocol.Response{}, fmt.Errorf("run plugin %s: %s", entry.Name, msg)
	}
	if stdout.truncated || stderr.truncated {
		return protocol.Response{}, fmt.Errorf("run plugin %s: plugin output exceeded %d bytes", entry.Name, maxPluginOutputBytes)
	}
	return decodeResponse(stdout.Bytes())
}

func (r Runner) invokeRequestV2(ctx context.Context, entry core.PluginEntry, req protocol.Request) (protocol.Response, error) {
	cmd, err := r.command(ctx, entry)
	if err != nil {
		return protocol.Response{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return protocol.Response{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return protocol.Response{}, err
	}
	cmd.Env = r.pluginEnv()
	var stderr limitedBuffer
	stderr.limit = maxPluginOutputBytes
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return protocol.Response{}, err
	}
	enc := json.NewEncoder(stdin)
	stdoutLimit := &limitedReadCloser{ReadCloser: stdout, remaining: maxPluginOutputBytes}
	dec := json.NewDecoder(stdoutLimit)
	frame, err := protocol.NewRequestFrame("root", protocol.TargetPlugin, req.Command, req)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return protocol.Response{}, err
	}
	if err := enc.Encode(frame); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return protocol.Response{}, err
	}
	for {
		var frame protocol.Frame
		if err := dec.Decode(&frame); err != nil {
			_ = stdin.Close()
			if stdoutLimit.exceeded || stderr.truncated {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				_ = cmd.Wait()
				if stdoutLimit.exceeded {
					return protocol.Response{}, fmt.Errorf("run plugin %s: plugin stdout exceeded %d bytes", entry.Name, maxPluginOutputBytes)
				}
				return protocol.Response{}, fmt.Errorf("run plugin %s: plugin stderr exceeded %d bytes", entry.Name, maxPluginOutputBytes)
			}
			waitErr := cmd.Wait()
			msg := strings.TrimSpace(stderr.String())
			if msg == "" && waitErr != nil {
				msg = waitErr.Error()
			}
			if msg == "" {
				msg = err.Error()
			}
			return protocol.Response{}, fmt.Errorf("run plugin %s: %s", entry.Name, msg)
		}
		if frame.Protocol != protocol.Version {
			_ = stdin.Close()
			_ = cmd.Wait()
			return protocol.Response{}, fmt.Errorf("plugin protocol mismatch: %s", frame.Protocol)
		}
		switch frame.Type {
		case protocol.FrameResponse:
			if frame.ID != "root" {
				_ = stdin.Close()
				_ = cmd.Wait()
				return protocol.Response{}, fmt.Errorf("unexpected plugin response frame %q", frame.ID)
			}
			_ = stdin.Close()
			waitErr := cmd.Wait()
			if stderr.truncated {
				return protocol.Response{}, fmt.Errorf("run plugin %s: plugin stderr exceeded %d bytes", entry.Name, maxPluginOutputBytes)
			}
			resp := protocol.Response{Protocol: protocol.Version, OK: frame.OK, Result: frame.Result, Error: frame.Error}
			if waitErr != nil && resp.OK {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = waitErr.Error()
				}
				return protocol.Response{}, fmt.Errorf("run plugin %s: %s", entry.Name, msg)
			}
			if !resp.OK && resp.Error != nil {
				return resp, fmt.Errorf("%s", resp.Error.Message)
			}
			return resp, nil
		case protocol.FrameRequest:
			if frame.Target != protocol.TargetHost {
				_ = enc.Encode(protocol.FrameResponseError(frame.ID, "bad_request", "plugin may only request host target"))
				continue
			}
			resp := r.handleHostRequest(ctx, entry.Name, req.Instance, req.Grant, frame)
			if err := enc.Encode(protocol.NewResponseFrame(frame.ID, resp)); err != nil {
				_ = stdin.Close()
				_ = cmd.Wait()
				return protocol.Response{}, err
			}
		case protocol.FrameEvent:
			if frame.Target != "" && frame.Target != protocol.TargetHost {
				_ = stdin.Close()
				_ = cmd.Wait()
				return protocol.Response{}, fmt.Errorf("plugin event may only target host")
			}
			r.handlePluginEvent(ctx, entry.Name, req.Instance, req.Command, frame)
		default:
			_ = stdin.Close()
			_ = cmd.Wait()
			if frame.Type == "" {
				return protocol.Response{}, fmt.Errorf("plugin %s returned an unframed response on v2 framed protocol", entry.Name)
			}
			return protocol.Response{}, fmt.Errorf("unexpected plugin frame type %q", frame.Type)
		}
	}
}

func (r Runner) handlePluginEvent(ctx context.Context, plugin, instance, command string, frame protocol.Frame) {
	if r.EventSink == nil {
		return
	}
	payload := append(json.RawMessage(nil), frame.Payload...)
	r.EventSink(ctx, PluginEvent{
		Plugin:   plugin,
		Instance: instance,
		Command:  command,
		Event:    strings.TrimSpace(frame.Command),
		Payload:  payload,
	})
}

func (r Runner) handleHostRequest(ctx context.Context, plugin, instance, grant string, frame protocol.Frame) protocol.Response {
	if err := rejectCrossPluginHostCall(plugin, frame.Payload); err != nil {
		return protocol.Fail("forbidden", err.Error())
	}
	switch frame.Command {
	case pluginbinding.HostSecretGet:
		var input struct {
			Purpose string `json:"purpose"`
		}
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		material, err := r.State.ResolveSecret(ctx, plugin, instance, input.Purpose, grant)
		if err != nil {
			return protocol.Fail("secret", err.Error())
		}
		return protocol.OK(material)
	case pluginbinding.HostIndexLookup:
		options := lookupPayload(frame.Payload)
		matches, err := r.State.LookupIndexWithOptions(plugin, instance, options)
		if err != nil {
			return protocol.Fail("host_index", err.Error())
		}
		return protocol.OK(pluginbinding.NewDatasourceLookupResult("host_index", options.Text, options.Terms, matches))
	case pluginbinding.HostIndexSearch:
		options := searchPayload(frame.Payload)
		records, err := r.State.SearchIndexWithOptions(plugin, instance, options)
		if err != nil {
			return protocol.Fail("host_index", err.Error())
		}
		return protocol.OK(pluginbinding.NewDatasourceSearchResult("host_index", options.Query, records))
	case pluginbinding.HostIndexGet:
		id := getPayloadID(frame.Payload)
		entity := getPayloadEntity(frame.Payload)
		if id == "" {
			return protocol.Fail("bad_payload", "host index get requires id")
		}
		record, ok, err := r.State.GetIndexRecordByEntity(plugin, instance, entity, id)
		if err != nil {
			return protocol.Fail("host_index", err.Error())
		}
		if !ok {
			return protocol.Fail("not_found", "indexed record not found")
		}
		return protocol.OK(pluginbinding.NewDatasourceGetResult("host_index", record))
	case pluginbinding.HostEndpointResolve:
		var input struct {
			EndpointRef string `json:"endpoint_ref"`
		}
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		endpoint, ok, err := r.State.GetEndpoint(input.EndpointRef)
		if err != nil {
			return protocol.Fail("endpoint", err.Error())
		}
		if !ok {
			return protocol.Fail("not_found", "unknown endpoint_ref "+input.EndpointRef)
		}
		return protocol.OK(endpoint.EndpointRef)
	case protocol.HostCapabilityHTTPDo:
		var input pluginbinding.HTTPRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, CapabilityGrant{Name: pluginbinding.CapabilityHTTP}); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostHTTP(ctx, plugin, instance, grant, input)
		if err != nil {
			return protocol.Fail("host_http", err.Error())
		}
		return protocol.OK(out)
	case protocol.HostCapabilityBlobRead:
		var input pluginbinding.BlobReadRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, CapabilityGrant{Name: pluginbinding.CapabilityBlobRead}); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostBlobRead(ctx, input)
		if err != nil {
			return protocol.Fail("host_blob", err.Error())
		}
		return protocol.OK(out)
	case protocol.HostCapabilityBlobWrite:
		var input pluginbinding.BlobWriteRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, CapabilityGrant{Name: pluginbinding.CapabilityBlobWrite}); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostBlobWrite(ctx, input)
		if err != nil {
			return protocol.Fail("host_blob", err.Error())
		}
		return protocol.OK(out)
	case protocol.HostCapabilityBlobInfo:
		var input pluginbinding.BlobInfoRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, CapabilityGrant{Name: pluginbinding.CapabilityBlobRead}); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostBlobInfo(ctx, input)
		if err != nil {
			return protocol.Fail("host_blob", err.Error())
		}
		return protocol.OK(out)
	case protocol.HostCapabilityEnvLookup:
		var input pluginbinding.EnvLookupRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, CapabilityGrant{Name: pluginbinding.CapabilityEnvLookup}); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostEnvLookup(ctx, input)
		if err != nil {
			return protocol.Fail("host_env", err.Error())
		}
		return protocol.OK(out)
	case protocol.HostCapabilityProviderCall:
		var input pluginbinding.ProviderCallRequest
		if err := json.Unmarshal(frame.Payload, &input); err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		requested := CapabilityGrant{Name: pluginbinding.CapabilityProvider, Provider: input.Provider, Action: input.Action}
		if err := r.State.ValidateCapabilityGrant(plugin, instance, grant, requested); err != nil {
			return protocol.Fail("forbidden", err.Error())
		}
		out, err := r.hostProviderCall(ctx, plugin, instance, grant, input)
		if err != nil {
			return protocol.Fail("host_provider", err.Error())
		}
		return protocol.OK(out)
	default:
		return protocol.Fail("unknown_host_command", "unknown host command "+frame.Command)
	}
}

func (r Runner) hostHTTP(ctx context.Context, plugin, instance, grant string, input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	resolved, err := r.resolveHTTPRequest(ctx, plugin, instance, grant, input)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	input = resolved
	return r.capabilityHost().HTTP(ctx, input)
}

func (r Runner) hostBlobRead(ctx context.Context, input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return r.capabilityHost().BlobRead(ctx, input)
}

func (r Runner) hostBlobWrite(ctx context.Context, input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return r.capabilityHost().BlobWrite(ctx, input)
}

func (r Runner) hostBlobInfo(ctx context.Context, input pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return r.capabilityHost().BlobInfo(ctx, input)
}

func (r Runner) hostEnvLookup(ctx context.Context, input pluginbinding.EnvLookupRequest) (pluginbinding.EnvLookupResponse, error) {
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return pluginbinding.EnvLookupResponse{}, fmt.Errorf("environment key is empty")
	}
	return r.capabilityHost().EnvLookup(ctx, input)
}

func (r Runner) hostProviderCall(ctx context.Context, plugin, instance, grant string, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	if strings.TrimSpace(input.Provider) == "sql" {
		return r.hostSQLProviderCall(ctx, plugin, instance, grant, input)
	}
	if strings.TrimSpace(input.Provider) == "docker" {
		return r.hostDockerProviderCall(ctx, plugin, instance, grant, input)
	}
	if strings.TrimSpace(input.Provider) == "kubernetes" {
		return r.hostKubernetesProviderCall(ctx, plugin, instance, grant, input)
	}
	if strings.TrimSpace(input.Provider) == "asterisk" {
		return r.hostAsteriskProviderCall(ctx, plugin, instance, grant, input)
	}
	if strings.TrimSpace(input.Provider) == systemProviderName {
		result, err := localSystemProvider{}.Call(ctx, strings.TrimSpace(input.Action), input.Payload)
		if err != nil {
			return pluginbinding.ProviderCallResponse{}, err
		}
		return pluginbinding.ProviderCallResponse{Result: result}, nil
	}
	return r.capabilityHost().ProviderCall(ctx, input)
}

func (r Runner) capabilityHost() CapabilityHost {
	if r.Capabilities != nil {
		return r.Capabilities
	}
	return NewLocalCapabilityHost("", r.Providers)
}

func (r Runner) resolveHTTPRequest(ctx context.Context, plugin, instance, grant string, input pluginbinding.HTTPRequest) (pluginbinding.HTTPRequest, error) {
	if strings.TrimSpace(input.EndpointRef) != "" {
		if strings.TrimSpace(input.URL) != "" {
			return pluginbinding.HTTPRequest{}, fmt.Errorf("host HTTP request cannot include both endpoint_ref and url")
		}
		endpoint, ok, err := r.State.GetEndpoint(input.EndpointRef)
		if err != nil {
			return pluginbinding.HTTPRequest{}, err
		}
		if !ok {
			return pluginbinding.HTTPRequest{}, fmt.Errorf("unknown endpoint_ref %q", input.EndpointRef)
		}
		input.URL = endpoint.URL
	}
	if input.Auth != nil {
		headers, err := r.resolveHTTPAuth(ctx, plugin, instance, grant, input.Headers, *input.Auth)
		if err != nil {
			return pluginbinding.HTTPRequest{}, err
		}
		input.Headers = headers
	}
	return input, nil
}

func (r Runner) resolveHTTPAuth(ctx context.Context, plugin, instance, grant string, headers map[string]string, auth pluginbinding.HTTPAuthRequest) (map[string]string, error) {
	if strings.TrimSpace(headers["Authorization"]) != "" {
		return r.resolveHTTPHeaderPurposes(ctx, plugin, instance, grant, headers, auth.HeaderPurposes), nil
	}
	if tokenPurpose := strings.TrimSpace(auth.BearerTokenPurpose); tokenPurpose != "" {
		material, err := r.State.ResolveSecret(ctx, plugin, instance, tokenPurpose, grant)
		if err == nil && strings.TrimSpace(material.Value) != "" {
			return r.resolveHTTPHeaderPurposes(ctx, plugin, instance, grant, withHTTPHeader(headers, "Authorization", "Bearer "+strings.TrimSpace(material.Value)), auth.HeaderPurposes), nil
		}
	}
	usernamePurpose := strings.TrimSpace(auth.UsernamePurpose)
	passwordPurpose := strings.TrimSpace(auth.PasswordPurpose)
	if usernamePurpose == "" && passwordPurpose == "" {
		return r.resolveHTTPHeaderPurposes(ctx, plugin, instance, grant, headers, auth.HeaderPurposes), nil
	}
	var username, password string
	if usernamePurpose != "" {
		material, err := r.State.ResolveSecret(ctx, plugin, instance, usernamePurpose, grant)
		if err != nil {
			return nil, err
		}
		username = strings.TrimSpace(material.Value)
	}
	if passwordPurpose != "" {
		material, err := r.State.ResolveSecret(ctx, plugin, instance, passwordPurpose, grant)
		if err != nil {
			return nil, err
		}
		password = strings.TrimSpace(material.Value)
	}
	if username == "" && password == "" {
		return r.resolveHTTPHeaderPurposes(ctx, plugin, instance, grant, headers, auth.HeaderPurposes), nil
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return r.resolveHTTPHeaderPurposes(ctx, plugin, instance, grant, withHTTPHeader(headers, "Authorization", "Basic "+encoded), auth.HeaderPurposes), nil
}

func (r Runner) resolveHTTPHeaderPurposes(ctx context.Context, plugin, instance, grant string, headers map[string]string, headerPurposes map[string]string) map[string]string {
	for header, purpose := range headerPurposes {
		header = strings.TrimSpace(header)
		purpose = strings.TrimSpace(purpose)
		if header == "" || purpose == "" || strings.TrimSpace(headers[header]) != "" {
			continue
		}
		material, err := r.State.ResolveSecret(ctx, plugin, instance, purpose, grant)
		if err == nil && strings.TrimSpace(material.Value) != "" {
			headers = withHTTPHeader(headers, header, strings.TrimSpace(material.Value))
		}
	}
	return headers
}

func withHTTPHeader(headers map[string]string, key, value string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	headers[key] = value
	return headers
}

func rejectCrossPluginHostCall(plugin string, raw json.RawMessage) error {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if value, _ := payload["plugin"].(string); strings.TrimSpace(value) != "" && strings.TrimSpace(value) != plugin {
		return fmt.Errorf("host call cannot access plugin %q from plugin %q", value, plugin)
	}
	return nil
}

func (r Runner) pluginEnv() []string {
	allowed := []string{
		"PATH",
		"HOME",
		"USER",
		"TMPDIR",
		"TEMP",
		"TMP",
		"GOCACHE",
		"GOPATH",
		"GOMODCACHE",
		"GOENV",
		"GOROOT",
		"GOPROXY",
		"GOSUMDB",
		"GONOSUMDB",
		"GOPRIVATE",
		"GONOPROXY",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"NO_PROXY",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
		"XDG_CACHE_HOME",
	}
	env := make([]string, 0, len(allowed)+2)
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func (r Runner) OperationBatch(ctx context.Context, pluginName, instance string, calls []protocol.OperationCall) (protocol.OperationBatchResult, error) {
	resp, err := r.InvokeInstance(ctx, pluginName, instance, protocol.CommandOperationsBatch, protocol.OperationBatch{Calls: calls})
	if err != nil {
		return protocol.OperationBatchResult{}, err
	}
	var result protocol.OperationBatchResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return protocol.OperationBatchResult{}, err
	}
	return result, nil
}

func (r Runner) Install(ctx context.Context, name string) error {
	_, err := r.InstallPlugin(ctx, name)
	return err
}

func (r Runner) InstallPlugin(ctx context.Context, name string) (InstalledPlugin, error) {
	return r.installPlugin(ctx, name, true)
}

func (r Runner) UpgradePlugin(ctx context.Context, name string) (InstalledPlugin, error) {
	installed, ok, err := r.State.InstalledPlugin(name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	activated := true
	if ok {
		activated = installed.Activated
	}
	return r.installPlugin(ctx, name, activated)
}

func (r Runner) installPlugin(ctx context.Context, name string, activated bool) (InstalledPlugin, error) {
	entry, ok := r.Marketplace.Resolve(name)
	if !ok {
		return InstalledPlugin{}, fmt.Errorf("unknown plugin %q", name)
	}
	if strings.TrimSpace(entry.GoInstall) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no go_install target", entry.Name)
	}
	if strings.TrimSpace(entry.Binary) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no binary name", entry.Name)
	}
	path := r.State.PluginBinaryPath(executableName(entry.Binary))
	if pluginDir, ok := localPluginPath(r.WorkDir, entry.LocalPath); ok {
		if err := InstallLocalGoTarget(ctx, pluginDir, executableName(entry.Binary), r.State.PluginBinDir()); err != nil {
			return InstalledPlugin{}, err
		}
		if err := r.State.SaveInstalledPluginVersionActivated(entry, true, path, "local", activated); err != nil {
			return InstalledPlugin{}, err
		}
		installed, _, err := r.State.InstalledPlugin(entry.Name)
		return installed, err
	}
	if strings.TrimSpace(entry.GoInstall) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no go_install target", entry.Name)
	}
	version := ""
	if info, err := ResolveGoModuleVersion(ctx, entry.GoInstall, "latest"); err == nil {
		version = info.Version
	}
	if err := InstallGoTarget(ctx, entry.GoInstall, r.State.PluginBinDir()); err != nil {
		return InstalledPlugin{}, err
	}
	if err := r.State.SaveInstalledPluginVersionActivated(entry, true, path, version, activated); err != nil {
		return InstalledPlugin{}, err
	}
	installed, _, err := r.State.InstalledPlugin(entry.Name)
	return installed, err
}

func (r Runner) command(ctx context.Context, entry core.PluginEntry) (*exec.Cmd, error) {
	if path := strings.TrimSpace(r.DevPlugins[entry.Name]); path != "" {
		return goRunCommand(ctx, path, entry.Binary), nil
	}
	if path, ok, err := r.installedPluginCommandPath(entry); err != nil {
		return nil, err
	} else if ok {
		return exec.CommandContext(ctx, path), nil
	}
	if path, ok := localPluginPath(r.WorkDir, entry.LocalPath); ok {
		return goRunCommand(ctx, path, entry.Binary), nil
	}
	if binary, err := exec.LookPath(entry.Binary); err == nil {
		return exec.CommandContext(ctx, binary), nil
	}
	return nil, fmt.Errorf("plugin %q is not installed; run dex plugin install %s", entry.Name, entry.Name)
}

func (r Runner) installedPluginCommandPath(entry core.PluginEntry) (string, bool, error) {
	installed, ok, err := r.State.InstalledPlugin(entry.Name)
	if err != nil || !ok {
		return "", false, err
	}
	for _, candidate := range installedPluginCommandCandidates(r.State, installed, entry) {
		if isExecutableFile(candidate) {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

func installedPluginCommandCandidates(state State, installed InstalledPlugin, entry core.PluginEntry) []string {
	var candidates []string
	if path := strings.TrimSpace(installed.Path); path != "" {
		candidates = append(candidates, path)
	}
	for _, binary := range []string{installed.Binary, entry.Binary} {
		binary = strings.TrimSpace(binary)
		if binary == "" {
			continue
		}
		if filepath.IsAbs(binary) {
			candidates = append(candidates, binary)
			continue
		}
		candidates = append(candidates, state.PluginBinaryPath(executableName(binary)))
	}
	return dedupeStrings(candidates)
}

func isExecutableFile(path string) bool {
	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() {
		return false
	}
	if goruntime.GOOS == "windows" {
		return true
	}
	return stat.Mode()&0o111 != 0
}

func executableName(binary string) string {
	binary = strings.TrimSpace(binary)
	if goruntime.GOOS == "windows" && strings.TrimSpace(filepath.Ext(binary)) == "" {
		return binary + ".exe"
	}
	return binary
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (r Runner) operationGrantScope(ctx context.Context, plugin string, payload any) ([]string, []SecretPurpose, []CapabilityGrant) {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return nil, nil, nil
	}
	byName := map[string]core.OperationSpec{}
	for _, op := range manifest.Operations {
		byName[op.Name] = op
	}
	authEnv := manifestAuthEnv(manifest)
	var operations []string
	purposeByName := map[string]SecretPurpose{}
	var capabilities []CapabilityGrant
	for _, call := range operationCalls(payload) {
		operations = append(operations, call.Name)
		if spec, ok := byName[call.Name]; ok {
			capabilities = append(capabilities, operationCapabilityGrants(spec)...)
			for _, purpose := range spec.SecretPurposes {
				if purpose == "" {
					continue
				}
				purposeByName[purpose] = SecretPurpose{Name: purpose, Env: authEnv[purpose]}
			}
		}
	}
	var purposes []SecretPurpose
	for _, purpose := range purposeByName {
		purposes = append(purposes, purpose)
	}
	return operations, purposes, normalizeCapabilityGrants(capabilities)
}

func (r Runner) datasourceGrantScope(ctx context.Context, plugin, command string, payload any) ([]string, []SecretPurpose, []CapabilityGrant) {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return nil, nil, nil
	}
	authEnv := manifestAuthEnv(manifest)
	capability := datasourceCommandCapability(command)
	datasourceName := ""
	entity := ""
	switch command {
	case protocol.CommandDatasourcesSearch:
		options := searchPayload(payload)
		datasourceName = options.Datasource
		entity = options.Entity
	case protocol.CommandDatasourcesLookup:
		options := lookupPayload(payload)
		datasourceName = options.Datasource
		entity = options.Entity
	case protocol.CommandDatasourcesGet:
		datasourceName = getPayloadDatasource(payload)
	}
	purposeByName := map[string]SecretPurpose{}
	var capabilities []CapabilityGrant
	for _, datasource := range manifest.Datasources {
		if datasourceName != "" && datasource.Name != datasourceName {
			continue
		}
		if entity != "" && datasource.Entity != entity {
			continue
		}
		if capability != "" && !containsString(datasource.Capabilities, capability) {
			continue
		}
		capabilities = append(capabilities, datasourceCapabilityGrants(datasource)...)
		for _, purpose := range datasource.SecretPurposes {
			if strings.TrimSpace(purpose) == "" {
				continue
			}
			purposeByName[purpose] = SecretPurpose{Name: purpose, Env: authEnv[purpose]}
		}
	}
	var purposes []SecretPurpose
	for _, purpose := range purposeByName {
		purposes = append(purposes, purpose)
	}
	return []string{command}, purposes, normalizeCapabilityGrants(capabilities)
}

func operationCapabilityGrants(spec core.OperationSpec) []CapabilityGrant {
	var out []CapabilityGrant
	hasProviderAccess := operationHasAccess(spec, core.OperationAccessProvider)
	for _, access := range spec.Access {
		switch access {
		case core.OperationAccessNetwork:
			if !hasProviderAccess {
				out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityHTTP})
			}
		case core.OperationAccessProvider:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityProvider, Provider: operationProvider(spec.Name), Action: "*"})
		case core.OperationAccessFilesystem:
			if spec.ReadOnly || operationHasEffect(spec, core.OperationEffectRead) || !operationHasEffect(spec, core.OperationEffectWrite) {
				out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityBlobRead})
			}
			if !spec.ReadOnly && operationHasEffect(spec, core.OperationEffectWrite) {
				out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityBlobWrite})
			}
		case core.OperationAccessProcess, core.OperationAccessBrowser:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityProvider, Provider: "*", Action: "*"})
		}
	}
	return out
}

func operationHasAccess(spec core.OperationSpec, access core.OperationAccess) bool {
	for _, candidate := range spec.Access {
		if candidate == access {
			return true
		}
	}
	return false
}

func operationProvider(name string) string {
	provider, _, ok := strings.Cut(strings.TrimSpace(name), ".")
	if !ok || strings.TrimSpace(provider) == "" {
		return "*"
	}
	return strings.TrimSpace(provider)
}

func operationHasEffect(spec core.OperationSpec, effect core.OperationEffect) bool {
	for _, candidate := range spec.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}

func datasourceCapabilityGrants(spec core.DatasourceSpec) []CapabilityGrant {
	var out []CapabilityGrant
	for _, access := range spec.Access {
		switch access {
		case core.OperationAccessNetwork:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityHTTP})
		case core.OperationAccessProvider:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityProvider, Provider: operationProvider(spec.Name), Action: "*"})
		case core.OperationAccessFilesystem:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityBlobRead})
		case core.OperationAccessProcess, core.OperationAccessBrowser:
			out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityProvider, Provider: "*", Action: "*"})
		}
	}
	if len(spec.SecretPurposes) > 0 {
		out = append(out, CapabilityGrant{Name: pluginbinding.CapabilityHTTP})
	}
	return out
}

func manifestAuthEnv(manifest core.PluginManifest) map[string][]string {
	authEnv := map[string][]string{}
	for _, method := range manifest.Auth {
		for _, field := range method.Fields {
			if field.Name == "" {
				continue
			}
			if len(field.Env) > 0 {
				authEnv[field.Name] = append(authEnv[field.Name], field.Env...)
			} else {
				authEnv[field.Name] = append(authEnv[field.Name], method.Env...)
			}
		}
	}
	return authEnv
}

func datasourceCommandCapability(command string) string {
	switch command {
	case protocol.CommandDatasourcesSearch:
		return "search"
	case protocol.CommandDatasourcesGet:
		return "get"
	case protocol.CommandDatasourcesLookup:
		return "lookup"
	default:
		return ""
	}
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (r Runner) manifest(ctx context.Context, plugin string) (core.PluginManifest, error) {
	entry, ok := r.Marketplace.Resolve(plugin)
	if !ok {
		return core.PluginManifest{}, fmt.Errorf("unknown plugin %q", plugin)
	}
	req, err := protocol.NewRequest(protocol.CommandManifest, entry.Name, nil)
	if err != nil {
		return core.PluginManifest{}, err
	}
	req.Instance = DefaultInstance
	var resp protocol.Response
	if isBuiltinPlugin(entry) {
		resp, err = r.invokeBuiltin(ctx, entry, req)
	} else {
		resp, err = r.invokeRequest(ctx, entry, req)
	}
	if err != nil {
		return core.PluginManifest{}, err
	}
	var manifest core.PluginManifest
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return core.PluginManifest{}, err
	}
	return manifest, nil
}

func defaultIndexName(ctx context.Context, r Runner, plugin string) string {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil || len(manifest.Indexes) == 0 {
		return ""
	}
	return manifest.Indexes[0].Name
}

func searchPayload(payload any) SearchOptions {
	data, err := json.Marshal(payload)
	if err != nil {
		return SearchOptions{Limit: 20}
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return SearchOptions{Limit: 20}
	}
	limit := 20
	switch value := input["limit"].(type) {
	case float64:
		limit = int(value)
	case int:
		limit = value
	}
	query, _ := input["query"].(string)
	datasource, _ := input["datasource"].(string)
	entity, _ := input["entity"].(string)
	return SearchOptions{Datasource: strings.TrimSpace(datasource), Query: strings.TrimSpace(query), Limit: limit, Entity: strings.TrimSpace(entity)}
}

func lookupPayload(payload any) LookupOptions {
	data, err := json.Marshal(payload)
	if err != nil {
		return LookupOptions{Limit: 20}
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return LookupOptions{Limit: 20}
	}
	limit := 20
	switch value := input["limit"].(type) {
	case float64:
		limit = int(value)
	case int:
		limit = value
	}
	text := firstPayloadString(input, "text", "query", "q")
	datasource := firstPayloadString(input, "datasource")
	entity := firstPayloadString(input, "entity")
	var terms []string
	if rawTerms, ok := input["terms"].([]any); ok {
		for _, term := range rawTerms {
			if text, ok := term.(string); ok && strings.TrimSpace(text) != "" {
				terms = append(terms, strings.TrimSpace(text))
			}
		}
	}
	if term := firstPayloadString(input, "term", "id", "ref", "url"); term != "" {
		terms = append(terms, term)
	}
	return LookupOptions{Datasource: datasource, Text: strings.TrimSpace(text), Terms: terms, Limit: limit, Entity: strings.TrimSpace(entity)}
}

func getPayloadDatasource(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}
	return firstPayloadString(input, "datasource")
}

func getPayloadEntity(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}
	return firstPayloadString(input, "entity")
}

func getPayloadID(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}
	for _, key := range []string{"id", "ref", "key"} {
		switch value := input[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return fmt.Sprintf("%.0f", value)
		}
	}
	return ""
}

func firstPayloadString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func operationCalls(payload any) []protocol.OperationCall {
	switch value := payload.(type) {
	case protocol.OperationCall:
		return []protocol.OperationCall{value}
	case protocol.OperationBatch:
		return value.Calls
	default:
		data, err := json.Marshal(payload)
		if err != nil {
			return nil
		}
		var call protocol.OperationCall
		if err := json.Unmarshal(data, &call); err == nil && call.Name != "" {
			return []protocol.OperationCall{call}
		}
		var batch protocol.OperationBatch
		if err := json.Unmarshal(data, &batch); err == nil {
			return batch.Calls
		}
		return nil
	}
}

func (r Runner) hostCommand() string {
	if strings.TrimSpace(r.HostCommand) != "" {
		return r.HostCommand
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	return "dex"
}

func goRunCommand(ctx context.Context, dir, binary string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/"+binary)
	cmd.Dir = dir
	return cmd
}

func localPluginPath(workDir, localPath string) (string, bool) {
	if strings.TrimSpace(localPath) == "" {
		return "", false
	}
	if filepath.IsAbs(localPath) {
		if stat, err := os.Stat(localPath); err == nil && stat.IsDir() {
			return localPath, true
		}
		return "", false
	}
	candidates := []string{}
	if strings.TrimSpace(workDir) != "" {
		candidates = append(candidates, filepath.Join(workDir, localPath))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, localPath))
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

type limitedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return b.Buffer.Write(p)
	}
	remaining := b.limit - b.Buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.Buffer.Write(p)
	return len(p), nil
}

type limitedReadCloser struct {
	io.ReadCloser
	remaining int
	exceeded  bool
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		r.exceeded = true
		return 0, fmt.Errorf("plugin output exceeded %d bytes", maxPluginOutputBytes)
	}
	if len(p) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= n
	return n, err
}

func decodeResponse(data []byte) (protocol.Response, error) {
	var resp protocol.Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("decode plugin response: %w", err)
	}
	if resp.Protocol != protocol.Version && resp.Protocol != protocol.VersionV1 {
		return resp, fmt.Errorf("plugin protocol mismatch: %s", resp.Protocol)
	}
	if !resp.OK && resp.Error != nil {
		return resp, fmt.Errorf("%s", resp.Error.Message)
	}
	return resp, nil
}
