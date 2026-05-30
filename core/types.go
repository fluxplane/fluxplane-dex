package core

import (
	"encoding/json"
	"strings"

	auth "github.com/fluxplane/fluxplane-auth"
	endpoint "github.com/fluxplane/fluxplane-endpoint"
	secret "github.com/fluxplane/fluxplane-secret"
)

type Marketplace struct {
	Version string        `json:"version"`
	Plugins []PluginEntry `json:"plugins"`
}

type PluginEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Binary      string            `json:"binary"`
	GoInstall   string            `json:"go_install,omitempty"`
	LocalPath   string            `json:"local_path,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type PluginManifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version,omitempty"`
	Description string            `json:"description,omitempty"`
	Aliases     []string          `json:"aliases,omitempty"`
	Operations  []OperationSpec   `json:"operations,omitempty"`
	Auth        []AuthMethod      `json:"auth,omitempty"`
	Datasources []DatasourceSpec  `json:"datasources,omitempty"`
	Context     []ContextSpec     `json:"context,omitempty"`
	Endpoints   []EndpointSpec    `json:"endpoints,omitempty"`
	Indexes     []IndexSpec       `json:"indexes,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type OperationSpec struct {
	Name           string               `json:"name"`
	Description    string               `json:"description,omitempty"`
	Input          json.RawMessage      `json:"input_schema,omitempty"`
	Output         json.RawMessage      `json:"output_schema,omitempty"`
	ReadOnly       bool                 `json:"read_only,omitempty"`
	Compact        bool                 `json:"compact,omitempty"`
	SecretPurposes []string             `json:"secret_purposes,omitempty"`
	Effects        []OperationEffect    `json:"effects,omitempty"`
	Risk           OperationRisk        `json:"risk,omitempty"`
	Idempotency    OperationIdempotency `json:"idempotency,omitempty"`
	Access         []OperationAccess    `json:"access,omitempty"`
	AuthScopes     []string             `json:"auth_scopes,omitempty"`
	Render         *OperationRenderSpec `json:"render,omitempty"`
}

type OperationEffect string

const (
	OperationEffectRead        OperationEffect = "read"
	OperationEffectWrite       OperationEffect = "write"
	OperationEffectNetwork     OperationEffect = "network"
	OperationEffectProcess     OperationEffect = "process"
	OperationEffectBrowser     OperationEffect = "browser"
	OperationEffectFilesystem  OperationEffect = "filesystem"
	OperationEffectLocalSystem OperationEffect = "local_system"
)

type OperationRisk string

const (
	OperationRiskLow         OperationRisk = "low"
	OperationRiskMedium      OperationRisk = "medium"
	OperationRiskHigh        OperationRisk = "high"
	OperationRiskDestructive OperationRisk = "destructive"
)

type OperationIdempotency string

const (
	OperationIdempotent    OperationIdempotency = "idempotent"
	OperationNonIdempotent OperationIdempotency = "non_idempotent"
	OperationConditional   OperationIdempotency = "conditional"
	OperationUnknown       OperationIdempotency = "unknown"
)

type OperationAccess string

const (
	OperationAccessNone        OperationAccess = "none"
	OperationAccessAuth        OperationAccess = "auth"
	OperationAccessSecret      OperationAccess = "secret"
	OperationAccessNetwork     OperationAccess = "network"
	OperationAccessProvider    OperationAccess = "provider"
	OperationAccessProcess     OperationAccess = "process"
	OperationAccessBrowser     OperationAccess = "browser"
	OperationAccessFilesystem  OperationAccess = "filesystem"
	OperationAccessLocalSystem OperationAccess = "local_system"
)

type OperationRenderSpec struct {
	Preferred string   `json:"preferred,omitempty"`
	Formats   []string `json:"formats,omitempty"`
}

type AuthMethod struct {
	Name        string            `json:"name"`
	Kind        secret.Kind       `json:"kind"`
	Description string            `json:"description,omitempty"`
	Env         []string          `json:"env,omitempty"`
	Fields      []AuthField       `json:"fields,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MethodSpec converts the legacy dex auth declaration to the shared auth contract.
func (m AuthMethod) MethodSpec() auth.MethodSpec {
	fields := make([]auth.FieldSpec, 0, len(m.Fields))
	for _, field := range m.Fields {
		fields = append(fields, field.FieldSpec())
	}
	return auth.MethodSpec{
		Name:        strings.TrimSpace(m.Name),
		Method:      auth.MethodStored,
		Scheme:      auth.SchemeBearerToken,
		Kind:        m.Kind,
		Description: strings.TrimSpace(m.Description),
		Env:         auth.EnvSpec{Aliases: trimStrings(m.Env)},
		SetupFields: fields,
		Annotations: cloneStringMap(m.Metadata),
	}.Normalize()
}

type AuthField struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Sensitive   bool     `json:"sensitive,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Env         []string `json:"env,omitempty"`
}

// FieldSpec converts the legacy dex auth field to the shared auth contract.
func (f AuthField) FieldSpec() auth.FieldSpec {
	kind := auth.FieldString
	if f.Secret || f.Sensitive {
		kind = auth.FieldPassword
	}
	return auth.FieldSpec{
		Slot:        secret.Slot(strings.TrimSpace(f.Name)),
		Kind:        kind,
		Description: strings.TrimSpace(f.Description),
		Required:    f.Required,
		Sensitive:   f.Sensitive || f.Secret,
		Env:         auth.EnvSpec{Aliases: trimStrings(f.Env)},
	}.Normalize()
}

type DatasourceSpec struct {
	Name           string                    `json:"name"`
	Entity         string                    `json:"entity"`
	Description    string                    `json:"description,omitempty"`
	Capabilities   []string                  `json:"capabilities,omitempty"`
	Access         []OperationAccess         `json:"access,omitempty"`
	SecretPurposes []string                  `json:"secret_purposes,omitempty"`
	Input          json.RawMessage           `json:"input_schema,omitempty"`
	Output         json.RawMessage           `json:"output_schema,omitempty"`
	EntitySchema   *DatasourceEntitySchema   `json:"entity_schema,omitempty"`
	Views          []DatasourceViewSpec      `json:"views,omitempty"`
	Relations      []DatasourceRelationSpec  `json:"relations,omitempty"`
	Fallback       DatasourceFallback        `json:"fallback,omitempty"`
	Completion     *DatasourceCompletionSpec `json:"completion,omitempty"`
}

type DatasourceEntitySchema struct {
	Entity     string                `json:"entity,omitempty"`
	IDField    string                `json:"id_field,omitempty"`
	TitleField string                `json:"title_field,omitempty"`
	Fields     []DatasourceFieldSpec `json:"fields,omitempty"`
}

type DatasourceFieldSpec struct {
	Name        string                  `json:"name"`
	Type        string                  `json:"type,omitempty"`
	Description string                  `json:"description,omitempty"`
	Views       []string                `json:"views,omitempty"`
	Completion  bool                    `json:"completion,omitempty"`
	Relation    *DatasourceRelationSpec `json:"relation,omitempty"`
}

type DatasourceViewSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Fields      []string `json:"fields,omitempty"`
}

type DatasourceRelationSpec struct {
	Name   string `json:"name,omitempty"`
	Field  string `json:"field,omitempty"`
	Entity string `json:"entity"`
	Type   string `json:"type,omitempty"`
}

type DatasourceFallback string

const (
	DatasourceFallbackNone           DatasourceFallback = "none"
	DatasourceFallbackHostIndex      DatasourceFallback = "host_index"
	DatasourceFallbackProviderFirst  DatasourceFallback = "provider_first"
	DatasourceFallbackHostIndexFirst DatasourceFallback = "host_index_first"
)

type DatasourceCompletionSpec struct {
	Fields      []string `json:"fields,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ContextSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Kinds       []string `json:"kinds,omitempty"`
}

type EndpointSpec = endpoint.EndpointSpec

type EndpointRef = endpoint.EndpointRef

type IndexSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Entities    []string `json:"entities,omitempty"`
}

type AuthMaterial struct {
	Method string            `json:"method,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type EndpointCandidate = endpoint.Candidate

type ContextBlock struct {
	ID       string            `json:"id,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Title    string            `json:"title,omitempty"`
	Content  string            `json:"content,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Source   *ContextSource    `json:"source,omitempty"`
	Priority int               `json:"priority,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ContextSource struct {
	Plugin   string `json:"plugin,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func trimStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
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

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
