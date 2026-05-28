package core

import "encoding/json"

type Marketplace struct {
	Version string        `json:"version"`
	Plugins []PluginEntry `json:"plugins"`
}

type PluginEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Aliases     []string          `json:"aliases,omitempty"`
	Binary      string            `json:"binary"`
	GoInstall   string            `json:"go_install,omitempty"`
	LocalPath   string            `json:"local_path,omitempty"`
	Commands    []CommandShortcut `json:"commands,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type CommandShortcut struct {
	Use         string         `json:"use"`
	Description string         `json:"description,omitempty"`
	Operation   string         `json:"operation,omitempty"`
	Defaults    map[string]any `json:"defaults,omitempty"`
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
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Input          json.RawMessage `json:"input_schema,omitempty"`
	Output         json.RawMessage `json:"output_schema,omitempty"`
	ReadOnly       bool            `json:"read_only,omitempty"`
	Compact        bool            `json:"compact,omitempty"`
	SecretPurposes []string        `json:"secret_purposes,omitempty"`
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
	Name           string          `json:"name"`
	Entity         string          `json:"entity"`
	Description    string          `json:"description,omitempty"`
	Capabilities   []string        `json:"capabilities,omitempty"`
	SecretPurposes []string        `json:"secret_purposes,omitempty"`
	Input          json.RawMessage `json:"input_schema,omitempty"`
	Output         json.RawMessage `json:"output_schema,omitempty"`
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
	ID          string            `json:"id"`
	URL         string            `json:"url,omitempty"`
	Product     string            `json:"product,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Source      string            `json:"source,omitempty"`
	Score       float64           `json:"score,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ContextBlock struct {
	ID       string            `json:"id,omitempty"`
	Title    string            `json:"title,omitempty"`
	Content  string            `json:"content,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Priority int               `json:"priority,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
