package core

import "encoding/json"

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
	Kind        string            `json:"kind"`
	Description string            `json:"description,omitempty"`
	Env         []string          `json:"env,omitempty"`
	Fields      []AuthField       `json:"fields,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type AuthField struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Sensitive   bool     `json:"sensitive,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Env         []string `json:"env,omitempty"`
}

type DatasourceSpec struct {
	Name           string                    `json:"name"`
	Entity         string                    `json:"entity"`
	Description    string                    `json:"description,omitempty"`
	Capabilities   []string                  `json:"capabilities,omitempty"`
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

type EndpointSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Products    []string `json:"products,omitempty"`
}

type EndpointRef struct {
	ID            string            `json:"id"`
	URL           string            `json:"url"`
	Product       string            `json:"product,omitempty"`
	Protocol      string            `json:"protocol,omitempty"`
	Source        string            `json:"source,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type IndexSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Entities    []string `json:"entities,omitempty"`
}

type AuthMaterial struct {
	Method string            `json:"method,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type EndpointCandidate struct {
	ID            string            `json:"id"`
	URL           string            `json:"url,omitempty"`
	Product       string            `json:"product,omitempty"`
	Protocol      string            `json:"protocol,omitempty"`
	Source        string            `json:"source,omitempty"`
	Score         float64           `json:"score,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

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
