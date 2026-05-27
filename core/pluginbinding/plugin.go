package pluginbinding

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type Plugin struct {
	manifest        core.PluginManifest
	operations      map[string]operation
	commandHandlers map[string]CommandHandler
}

type Context struct {
	Request protocol.Request
	Call    protocol.OperationCall
	Cache   *Cache
}

type Cache struct {
	values map[string]any
}

type CommandHandler func(Context) protocol.Response

type OperationHandler[I any, O any] func(Context, I) (O, error)

type TextResult struct {
	Text    string `json:"text,omitempty"`
	Summary string `json:"summary,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type Error struct {
	Code    string
	Message string
}

type operation interface {
	Spec() core.OperationSpec
	Run(Context) protocol.OperationResult
}

type typedOperation[I any, O any] struct {
	spec    core.OperationSpec
	handler OperationHandler[I, O]
}

func New(manifest core.PluginManifest) *Plugin {
	return &Plugin{
		manifest:        manifest,
		operations:      map[string]operation{},
		commandHandlers: map[string]CommandHandler{},
	}
}

func Serve(plugin *Plugin) {
	protocol.Serve(plugin.Handle)
}

func Operation[I any, O any](plugin *Plugin, spec core.OperationSpec, handler OperationHandler[I, O]) {
	if plugin == nil {
		return
	}
	if strings.TrimSpace(spec.Name) == "" {
		panic("pluginbinding: operation name is required")
	}
	if len(spec.Input) == 0 {
		spec.Input = MustSchemaFor[I]()
	}
	if len(spec.Output) == 0 {
		spec.Output = MustSchemaFor[O]()
	}
	plugin.operations[spec.Name] = typedOperation[I, O]{spec: spec, handler: handler}
	plugin.upsertOperation(spec)
}

func (p *Plugin) Command(command string, handler CommandHandler) {
	if p == nil || strings.TrimSpace(command) == "" || handler == nil {
		return
	}
	p.commandHandlers[command] = handler
}

func (p *Plugin) AuthConnectText(text string) {
	p.Command(protocol.CommandAuthConnect, func(Context) protocol.Response {
		return OKText(text, nil)
	})
}

func (p *Plugin) HostOwnedIndexStatus(product string) {
	p.Command(protocol.CommandIndexStatus, func(Context) protocol.Response {
		return OKText(product+" index is host-owned", map[string]any{"status": "host_owned"})
	})
}

func (p *Plugin) AuthTestOperation(name string) {
	p.Command(protocol.CommandAuthTest, func(ctx Context) protocol.Response {
		if ctx.Request.Grant == "" {
			return OKText(p.manifest.Name+" auth is host-managed; use dex auth status "+p.manifest.Name+" or dex op run "+name, map[string]any{"status": "host_managed"})
		}
		return p.callOperation(ctx.Request, protocol.OperationCall{Name: name}, NewCache(), true)
	})
}

func (p *Plugin) IndexBuildOperation(name string) {
	p.Command(protocol.CommandIndexBuild, func(ctx Context) protocol.Response {
		if ctx.Request.Grant == "" {
			return OKText("Use dex op run "+name+" to build live records", map[string]any{"status": "requires_operation_grant"})
		}
		return p.callOperation(ctx.Request, protocol.OperationCall{Name: name, Input: ctx.Request.Payload}, NewCache(), false)
	})
}

func (p *Plugin) Handle(req protocol.Request) protocol.Response {
	if p == nil {
		return protocol.Fail("plugin_error", "plugin is nil")
	}
	cache := NewCache()
	ctx := Context{Request: req, Cache: cache}
	if handler := p.commandHandlers[req.Command]; handler != nil {
		return handler(ctx)
	}
	switch req.Command {
	case protocol.CommandManifest:
		return protocol.OK(p.Manifest())
	case protocol.CommandAuthMethods:
		return protocol.OK(p.Manifest().Auth)
	case protocol.CommandOperationsList:
		return protocol.OK(p.Manifest().Operations)
	case protocol.CommandOperationsCall:
		call, err := protocol.DecodePayload[protocol.OperationCall](req.Payload)
		if err != nil {
			return protocol.Fail("bad_payload", err.Error())
		}
		return p.callOperation(req, call, cache, true)
	case protocol.CommandOperationsBatch:
		return p.callBatch(req, cache)
	case protocol.CommandDatasourcesList:
		return protocol.OK(p.Manifest().Datasources)
	case protocol.CommandDatasourcesSearch:
		return protocol.Fail("not_implemented", p.manifest.Name+" datasource live search requires host index integration")
	case protocol.CommandDatasourcesGet:
		return protocol.Fail("not_implemented", p.manifest.Name+" datasource get requires host index integration")
	case protocol.CommandDatasourcesLookup:
		return protocol.Fail("not_implemented", p.manifest.Name+" datasource lookup requires host index integration")
	case protocol.CommandContextBuild:
		return OKData(map[string]any{"blocks": []core.ContextBlock{}})
	case protocol.CommandEndpointsDiscover:
		return OKData(map[string]any{"candidates": []core.EndpointCandidate{}})
	default:
		return protocol.Fail("unknown_command", p.manifest.Name+" plugin does not implement "+req.Command)
	}
}

func (p *Plugin) Manifest() core.PluginManifest {
	manifest := p.manifest
	manifest.Operations = append([]core.OperationSpec(nil), manifest.Operations...)
	return manifest
}

func (p *Plugin) callBatch(req protocol.Request, cache *Cache) protocol.Response {
	batch, err := protocol.DecodePayload[protocol.OperationBatch](req.Payload)
	if err != nil {
		return protocol.Fail("bad_payload", err.Error())
	}
	results := make([]protocol.OperationResult, 0, len(batch.Calls))
	for _, call := range batch.Calls {
		results = append(results, p.runOperation(req, call, cache))
	}
	return protocol.OK(protocol.OperationBatchResult{Results: results})
}

func (p *Plugin) callOperation(req protocol.Request, call protocol.OperationCall, cache *Cache, unwrap bool) protocol.Response {
	result := p.runOperation(req, call, cache)
	if !result.OK {
		return protocol.Response{Protocol: protocol.Version, OK: false, Error: result.Error}
	}
	if unwrap {
		return protocol.Response{Protocol: protocol.Version, OK: true, Result: result.Result}
	}
	var value any
	if len(result.Result) > 0 {
		if err := json.Unmarshal(result.Result, &value); err != nil {
			return protocol.Fail("marshal_result", err.Error())
		}
	}
	return OKData(value)
}

func (p *Plugin) runOperation(req protocol.Request, call protocol.OperationCall, cache *Cache) protocol.OperationResult {
	if call.ID == "" {
		call.ID = call.Name
	}
	op := p.operations[call.Name]
	if op == nil {
		return OperationError(call, "unknown_operation", "unknown operation "+call.Name)
	}
	return op.Run(Context{Request: req, Call: call, Cache: cache})
}

func (p *Plugin) RunOperation(req protocol.Request, call protocol.OperationCall, cache *Cache) protocol.OperationResult {
	if cache == nil {
		cache = NewCache()
	}
	return p.runOperation(req, call, cache)
}

func (p *Plugin) upsertOperation(spec core.OperationSpec) {
	for i := range p.manifest.Operations {
		if p.manifest.Operations[i].Name == spec.Name {
			p.manifest.Operations[i] = mergeOperationSpec(p.manifest.Operations[i], spec)
			return
		}
	}
	p.manifest.Operations = append(p.manifest.Operations, spec)
}

func (op typedOperation[I, O]) Spec() core.OperationSpec {
	return op.spec
}

func (op typedOperation[I, O]) Run(ctx Context) protocol.OperationResult {
	input, err := DecodeCallInput[I](ctx.Call)
	if err != nil {
		return OperationError(ctx.Call, "bad_input", err.Error())
	}
	out, err := op.handler(ctx, input)
	if err != nil {
		var pluginErr Error
		if errors.As(err, &pluginErr) {
			return OperationError(ctx.Call, pluginErr.Code, pluginErr.Message)
		}
		return OperationError(ctx.Call, "plugin_error", err.Error())
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return OperationError(ctx.Call, "marshal_result", err.Error())
	}
	return protocol.OperationResult{ID: ctx.Call.ID, Name: ctx.Call.Name, OK: true, Result: raw}
}

func DecodeCallInput[T any](call protocol.OperationCall) (T, error) {
	var input T
	if len(call.Input) == 0 {
		return input, nil
	}
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return input, fmt.Errorf("decode operation input: %w", err)
	}
	return input, nil
}

func NewCache() *Cache {
	return &Cache{values: map[string]any{}}
}

func (c *Cache) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.values[key]
	return value, ok
}

func (c *Cache) Set(key string, value any) {
	if c == nil {
		return
	}
	c.values[key] = value
}

func Fail(code, message string) error {
	return Error{Code: code, Message: message}
}

func Errorf(code, format string, args ...any) error {
	return Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e Error) Error() string {
	return e.Message
}

func OperationError(call protocol.OperationCall, code, message string) protocol.OperationResult {
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: false, Error: &protocol.Error{Code: code, Message: message}}
}

func OKText(text string, data any) protocol.Response {
	return protocol.OK(TextResult{Text: text, Summary: firstLine(text), Data: data})
}

func OKData(data any) protocol.Response {
	return protocol.OK(TextResult{Data: data})
}

func MustSchemaFor[T any]() json.RawMessage {
	raw, err := SchemaFor[T]()
	if err != nil {
		panic(err)
	}
	return raw
}

func SchemaFor[T any]() (json.RawMessage, error) {
	var zero T
	schema := schemaForType(reflect.TypeOf(zero))
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func schemaForType(typ reflect.Type) map[string]any {
	if typ == nil {
		return map[string]any{}
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		return structSchema(typ)
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaForType(typ.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object"}
	default:
		return map[string]any{}
	}
}

func structSchema(typ reflect.Type) map[string]any {
	properties := map[string]any{}
	var required []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, optional, ok := jsonFieldName(field)
		if !ok {
			continue
		}
		fieldSchema := schemaForType(field.Type)
		tags := parseSchemaTag(field.Tag.Get("jsonschema"))
		if description := tags["description"]; description != "" {
			fieldSchema["description"] = description
		}
		if enum := tags["enum"]; enum != "" {
			fieldSchema["enum"] = strings.Split(enum, "|")
		}
		properties[name] = fieldSchema
		if _, forced := tags["required"]; forced || !optional {
			required = append(required, name)
		}
	}
	out := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		sort.Strings(required)
		out["required"] = required
	}
	return out
}

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}
	if tag == "" {
		return field.Name, false, true
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	optional := false
	for _, part := range parts[1:] {
		if part == "omitempty" {
			optional = true
		}
	}
	return name, optional, true
}

func parseSchemaTag(tag string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			out[part] = "true"
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func mergeOperationSpec(base, generated core.OperationSpec) core.OperationSpec {
	base.Name = firstNonEmpty(base.Name, generated.Name)
	base.Description = firstNonEmpty(base.Description, generated.Description)
	if len(base.Input) == 0 {
		base.Input = generated.Input
	}
	if len(base.Output) == 0 {
		base.Output = generated.Output
	}
	if !base.ReadOnly {
		base.ReadOnly = generated.ReadOnly
	}
	if !base.Compact {
		base.Compact = generated.Compact
	}
	if len(base.SecretPurposes) == 0 {
		base.SecretPurposes = generated.SecretPurposes
	}
	return base
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}
