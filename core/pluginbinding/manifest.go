package pluginbinding

import "github.com/fluxplane/fluxplane-dex/core"

const (
	CapabilitySearch = "search"
	CapabilityLookup = "lookup"
	CapabilityGet    = "get"
	CapabilityIndex  = "index"

	ContextKindText      = "text"
	ContextKindReference = "reference"
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
}

func Manifest(spec ManifestSpec) core.PluginManifest {
	datasources := append([]core.DatasourceSpec(nil), spec.Datasources...)
	indexes := append([]core.IndexSpec(nil), spec.Indexes...)
	for _, indexed := range spec.IndexedDatasources {
		datasources = append(datasources, Datasource(indexed.Name, indexed.Entity, indexed.Description, indexed.Capabilities...))
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
		Operations:  append([]core.OperationSpec(nil), spec.Operations...),
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
	}
}

func SecretPurposes(purposes ...string) OperationSpecOption {
	return func(spec *core.OperationSpec) {
		spec.SecretPurposes = append([]string(nil), purposes...)
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

func SearchableIndexCapabilities() []string {
	return []string{CapabilitySearch, CapabilityLookup, CapabilityGet, CapabilityIndex}
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
