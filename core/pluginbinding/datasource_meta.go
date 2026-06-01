package pluginbinding

import (
	datasource "github.com/fluxplane/fluxplane-datasource"
	"github.com/fluxplane/fluxplane-dex/core"
)

const (
	DatasourceViewCompact = datasource.DeclarationViewCompact
	DatasourceViewDetail  = datasource.DeclarationViewDetail
	DatasourceViewLookup  = datasource.DeclarationViewLookup
	DatasourceViewTable   = datasource.DeclarationViewTable
)

func EntitySchema(schema core.DatasourceEntitySchema) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		copied := schema
		*spec = datasource.NormalizeDeclaration(datasource.MergeDeclarationSchema(*spec, copied))
	}
}

func EntitySchemaFor[T any]() DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		generated := datasource.EntitySchemaFor[T]()
		if spec.EntitySchema != nil {
			generated = datasource.MergeEntitySchema(generated, *spec.EntitySchema)
		}
		spec.EntitySchema = &generated
	}
}

func View(name, description string, fields ...string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Views = append(spec.Views, core.DatasourceViewSpec{Name: name, Description: description, Fields: append([]string(nil), fields...)})
	}
}

func Relation(name, field, entity, relationType string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Relations = append(spec.Relations, core.DatasourceRelationSpec{Name: name, Field: field, Entity: entity, Type: relationType})
	}
}

func Fallback(fallback core.DatasourceFallback) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Fallback = fallback
	}
}

func Completion(description string, fields ...string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Completion = &core.DatasourceCompletionSpec{Description: description, Fields: append([]string(nil), fields...)}
	}
}

func normalizeDatasourceSpecs(specs []core.DatasourceSpec) []core.DatasourceSpec {
	return datasource.NormalizeDeclarations(specs)
}

func NormalizeDatasourceSpec(spec core.DatasourceSpec) core.DatasourceSpec {
	return datasource.NormalizeDeclaration(spec)
}

func mergeEntitySchema(base, generated core.DatasourceEntitySchema) core.DatasourceEntitySchema {
	return datasource.MergeEntitySchema(base, generated)
}

func normalizeDatasourceViews(views []core.DatasourceViewSpec) []core.DatasourceViewSpec {
	return datasource.NormalizeDeclaration(core.DatasourceSpec{Views: views}).Views
}

func normalizeDatasourceRelations(relations []core.DatasourceRelationSpec) []core.DatasourceRelationSpec {
	return datasource.NormalizeDeclaration(core.DatasourceSpec{Relations: relations}).Relations
}
