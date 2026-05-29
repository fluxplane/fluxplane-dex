package pluginbinding

import (
	"encoding/json"

	"github.com/fluxplane/fluxplane-dex/core"
)

const (
	CapabilitySearch = "search"
	CapabilityList   = "list"
	CapabilityLookup = "lookup"
	CapabilityGet    = "get"
	CapabilityIndex  = "index"

	ContextKindText      = "text"
	ContextKindReference = "reference"
	ContextKindData      = "data"
)

type ManifestSpec struct {
	Name               string
	Version            string
	Description        string
	Aliases            []string
	Operations         []core.OperationSpec
	Auth               []core.AuthMethod
	Datasources        []core.DatasourceSpec
	IndexedDatasources []IndexedDatasourceSpec
	Context            []core.ContextSpec
	Endpoints          []core.EndpointSpec
	Indexes            []core.IndexSpec
	Metadata           map[string]string
}

type OperationSpecOption func(*core.OperationSpec)
type DatasourceSpecOption func(*core.DatasourceSpec)

type IndexedDatasourceSpec struct {
	Name             string
	Entity           string
	Description      string
	IndexDescription string
	Capabilities     []string
	Options          []DatasourceSpecOption
}

func Manifest(spec ManifestSpec) core.PluginManifest {
	datasources := normalizeDatasourceSpecs(spec.Datasources)
	indexes := append([]core.IndexSpec(nil), spec.Indexes...)
	for _, indexed := range spec.IndexedDatasources {
		datasource := Datasource(indexed.Name, indexed.Entity, indexed.Description, indexed.Capabilities...)
		for _, option := range indexed.Options {
			if option != nil {
				option(&datasource)
			}
		}
		datasources = append(datasources, NormalizeDatasourceSpec(datasource))
		indexDescription := indexed.IndexDescription
		if indexDescription == "" {
			indexDescription = indexed.Description
		}
		indexes = append(indexes, Index(indexed.Name, indexDescription, indexed.Entity))
	}
	return core.PluginManifest{
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		Aliases:     append([]string(nil), spec.Aliases...),
		Operations:  normalizeOperationSpecs(spec.Operations),
		Auth:        append([]core.AuthMethod(nil), spec.Auth...),
		Datasources: datasources,
		Context:     append([]core.ContextSpec(nil), spec.Context...),
		Endpoints:   append([]core.EndpointSpec(nil), spec.Endpoints...),
		Indexes:     indexes,
		Metadata:    cloneStringMap(spec.Metadata),
	}
}

func OperationSpec(name, description string, options ...OperationSpecOption) core.OperationSpec {
	spec := core.OperationSpec{Name: name, Description: description}
	for _, option := range options {
		option(&spec)
	}
	return spec
}

func ReadOnly() OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.ReadOnly = true
	}
}

func Compact() OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Compact = true
		if spec.Render == nil {
			spec.Render = &core.OperationRenderSpec{Preferred: "compact", Formats: []string{"text", "compact", "json", "yaml"}}
		}
	}
}

func SecretPurposes(purposes ...string) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.SecretPurposes = append([]string(nil), purposes...)
	}
}

func Effects(effects ...core.OperationEffect) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Effects = append([]core.OperationEffect(nil), effects...)
	}
}

func Risk(risk core.OperationRisk) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Risk = risk
	}
}

func Idempotency(idempotency core.OperationIdempotency) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Idempotency = idempotency
	}
}

func Access(access ...core.OperationAccess) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Access = append([]core.OperationAccess(nil), access...)
	}
}

func AuthScopes(scopes ...string) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.AuthScopes = append([]string(nil), scopes...)
	}
}

func Render(preferred string, formats ...string) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.Render = &core.OperationRenderSpec{Preferred: preferred, Formats: append([]string(nil), formats...)}
	}
}

func DatasourceSecretPurposes(purposes ...string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.SecretPurposes = append([]string(nil), purposes...)
	}
}

func BearerAuth(name, description string, fields ...core.AuthField) core.AuthMethod {
	return core.AuthMethod{
		Name:        name,
		Kind:        "bearer_token",
		Description: description,
		Env:         authEnv(fields),
		Fields:      append([]core.AuthField(nil), fields...),
	}
}

func AuthField(name, description string, required, secret bool, env ...string) core.AuthField {
	return core.AuthField{
		Name:        name,
		Description: description,
		Required:    required,
		Sensitive:   secret,
		Secret:      secret,
		Env:         append([]string(nil), env...),
	}
}

func Datasource(name, entity, description string, capabilities ...string) core.DatasourceSpec {
	return core.DatasourceSpec{
		Name:         name,
		Entity:       entity,
		Description:  description,
		Capabilities: append([]string(nil), capabilities...),
	}
}

func ContextSpec(name, description string, kinds ...string) core.ContextSpec {
	return core.ContextSpec{
		Name:        name,
		Description: description,
		Kinds:       append([]string(nil), kinds...),
	}
}

func Endpoint(name, description string, products ...string) core.EndpointSpec {
	return core.EndpointSpec{
		Name:        name,
		Description: description,
		Products:    append([]string(nil), products...),
	}
}

func Index(name, description string, entities ...string) core.IndexSpec {
	return core.IndexSpec{
		Name:        name,
		Description: description,
		Entities:    append([]string(nil), entities...),
	}
}

func IndexedDatasource(name, entity, description, indexDescription string, capabilities ...string) IndexedDatasourceSpec {
	return IndexedDatasourceSpec{
		Name:             name,
		Entity:           entity,
		Description:      description,
		IndexDescription: indexDescription,
		Capabilities:     append([]string(nil), capabilities...),
	}
}

func IndexedDatasourceWithOptions(name, entity, description, indexDescription string, capabilities []string, options ...DatasourceSpecOption) IndexedDatasourceSpec {
	return IndexedDatasourceSpec{
		Name:             name,
		Entity:           entity,
		Description:      description,
		IndexDescription: indexDescription,
		Capabilities:     append([]string(nil), capabilities...),
		Options:          append([]DatasourceSpecOption(nil), options...),
	}
}

func SearchableIndexCapabilities() []string {
	return []string{CapabilitySearch, CapabilityList, CapabilityLookup, CapabilityGet, CapabilityIndex}
}

func authEnv(fields []core.AuthField) []string {
	seen := map[string]bool{}
	var out []string
	for _, field := range fields {
		for _, env := range field.Env {
			if env == "" || seen[env] {
				continue
			}
			seen[env] = true
			out = append(out, env)
		}
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func normalizeOperationSpecs(specs []core.OperationSpec) []core.OperationSpec {
	out := make([]core.OperationSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, NormalizeOperationSpec(spec))
	}
	return out
}

func NormalizeOperationSpec(spec core.OperationSpec) core.OperationSpec {
	spec.Input = normalizeOperationInputSchema(spec.Input)
	spec.Effects = uniqueOperationEffects(spec.Effects)
	spec.Access = uniqueOperationAccess(spec.Access)
	spec.AuthScopes = uniqueStringValues(spec.AuthScopes)
	spec.SecretPurposes = uniqueStringValues(spec.SecretPurposes)
	if len(spec.Effects) == 0 {
		if spec.ReadOnly {
			spec.Effects = []core.OperationEffect{core.OperationEffectRead}
		} else {
			spec.Effects = []core.OperationEffect{core.OperationEffectWrite}
		}
	}
	if spec.Risk == "" {
		if spec.ReadOnly {
			spec.Risk = core.OperationRiskLow
		} else {
			spec.Risk = core.OperationRiskMedium
		}
	}
	if spec.Idempotency == "" {
		if spec.ReadOnly {
			spec.Idempotency = core.OperationIdempotent
		} else {
			spec.Idempotency = core.OperationUnknown
		}
	}
	for _, effect := range spec.Effects {
		switch effect {
		case core.OperationEffectNetwork:
			spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessNetwork)
		case core.OperationEffectProcess:
			spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessProcess)
		case core.OperationEffectBrowser:
			spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessBrowser)
		case core.OperationEffectFilesystem:
			spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessFilesystem)
		case core.OperationEffectLocalSystem:
			spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessLocalSystem)
		}
	}
	if len(spec.SecretPurposes) > 0 {
		spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessSecret)
		spec.Access = ensureOperationAccess(spec.Access, core.OperationAccessAuth)
	}
	if len(spec.Access) == 0 {
		spec.Access = []core.OperationAccess{core.OperationAccessNone}
	} else if len(spec.Access) > 1 {
		spec.Access = removeOperationAccess(spec.Access, core.OperationAccessNone)
	}
	if spec.Render == nil && spec.Compact {
		spec.Render = &core.OperationRenderSpec{Preferred: "compact", Formats: []string{"text", "compact", "json", "yaml"}}
	}
	if spec.Render != nil {
		spec.Render.Formats = uniqueStringValues(spec.Render.Formats)
	}
	return spec
}

func normalizeOperationInputSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		raw = json.RawMessage(`{"properties":{},"type":"object"}`)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return raw
	}
	if schema == nil {
		schema = map[string]any{}
	}
	if schemaType, _ := schema["type"].(string); schemaType != "" && schemaType != "object" {
		return raw
	}
	schema["type"] = "object"
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
		schema["properties"] = properties
	}
	if _, ok := properties["endpoint_ref"]; !ok {
		properties["endpoint_ref"] = map[string]any{
			"type":        "string",
			"description": "Registered endpoint ref resolved by the host before invoking the operation.",
		}
	}
	normalized, err := json.Marshal(normalizeSchemaValue(schema))
	if err != nil {
		return raw
	}
	return normalized
}

func uniqueStringValues(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uniqueOperationEffects(values []core.OperationEffect) []core.OperationEffect {
	seen := map[core.OperationEffect]bool{}
	var out []core.OperationEffect
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uniqueOperationAccess(values []core.OperationAccess) []core.OperationAccess {
	seen := map[core.OperationAccess]bool{}
	var out []core.OperationAccess
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func ensureOperationAccess(values []core.OperationAccess, candidate core.OperationAccess) []core.OperationAccess {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func removeOperationAccess(values []core.OperationAccess, candidate core.OperationAccess) []core.OperationAccess {
	out := values[:0]
	for _, value := range values {
		if value != candidate {
			out = append(out, value)
		}
	}
	return out
}
