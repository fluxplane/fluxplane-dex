package pluginbinding

import (
	"reflect"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
)

const (
	DatasourceViewCompact = "compact"
	DatasourceViewDetail  = "detail"
	DatasourceViewLookup  = "lookup"
	DatasourceViewTable   = "table"
)

func EntitySchema(schema core.DatasourceEntitySchema) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		copied := cloneEntitySchema(schema)
		if spec.EntitySchema != nil {
			copied = mergeEntitySchema(*spec.EntitySchema, copied)
		}
		spec.EntitySchema = &copied
	}
}

func EntitySchemaFor[T any]() DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		schema := deriveEntitySchemaFor[T]()
		if spec.EntitySchema != nil {
			schema = mergeEntitySchema(schema, *spec.EntitySchema)
		}
		spec.EntitySchema = &schema
	}
}

func View(name, description string, fields ...string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Views = mergeDatasourceViews(spec.Views, core.DatasourceViewSpec{
			Name:        strings.TrimSpace(name),
			Description: strings.TrimSpace(description),
			Fields:      uniqueStringValues(fields),
		})
	}
}

func Relation(name, field, entity, relationType string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Relations = mergeDatasourceRelations(spec.Relations, core.DatasourceRelationSpec{
			Name:   strings.TrimSpace(name),
			Field:  strings.TrimSpace(field),
			Entity: strings.TrimSpace(entity),
			Type:   strings.TrimSpace(relationType),
		})
	}
}

func Fallback(fallback core.DatasourceFallback) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Fallback = fallback
	}
}

func Completion(description string, fields ...string) DatasourceSpecOption {
	return func(spec *core.DatasourceSpec) {
		spec.Completion = &core.DatasourceCompletionSpec{
			Description: strings.TrimSpace(description),
			Fields:      uniqueStringValues(fields),
		}
	}
}

func normalizeDatasourceSpecs(specs []core.DatasourceSpec) []core.DatasourceSpec {
	out := make([]core.DatasourceSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, NormalizeDatasourceSpec(spec))
	}
	return out
}

func NormalizeDatasourceSpec(spec core.DatasourceSpec) core.DatasourceSpec {
	spec.Capabilities = uniqueStringValues(spec.Capabilities)
	spec.SecretPurposes = uniqueStringValues(spec.SecretPurposes)
	if spec.EntitySchema != nil {
		schema := cloneEntitySchema(*spec.EntitySchema)
		if strings.TrimSpace(schema.Entity) == "" {
			schema.Entity = strings.TrimSpace(spec.Entity)
		}
		schema.Fields = uniqueDatasourceFields(schema.Fields)
		spec.Views = mergeViewsFromFields(spec.Views, schema.Fields)
		spec.Relations = mergeRelationsFromFields(spec.Relations, schema.Fields)
		spec.Completion = mergeCompletionFromFields(spec.Completion, schema.Fields)
		spec.EntitySchema = &schema
	}
	spec.Views = normalizeDatasourceViews(spec.Views)
	spec.Relations = normalizeDatasourceRelations(spec.Relations)
	if spec.Fallback == "" {
		if containsDatasourceString(spec.Capabilities, CapabilityIndex) {
			spec.Fallback = core.DatasourceFallbackHostIndexFirst
		} else {
			spec.Fallback = core.DatasourceFallbackNone
		}
	}
	if spec.Completion != nil {
		spec.Completion.Fields = uniqueStringValues(spec.Completion.Fields)
	}
	return spec
}

func containsDatasourceString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func deriveEntitySchemaFor[T any]() core.DatasourceEntitySchema {
	var zero T
	typ := reflect.TypeOf(zero)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	schema := core.DatasourceEntitySchema{}
	if typ.Kind() != reflect.Struct {
		return schema
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || field.Anonymous {
			continue
		}
		name := jsonFieldName(field)
		if name == "" {
			continue
		}
		meta := parseDatasourceTag(field.Tag.Get("datasource"))
		fieldSpec := core.DatasourceFieldSpec{
			Name:        name,
			Type:        datasourceFieldType(field.Type),
			Description: jsonschemaDescription(field.Tag.Get("jsonschema")),
			Views:       meta.views,
			Completion:  meta.completion,
		}
		if meta.id {
			schema.IDField = name
		}
		if meta.title {
			schema.TitleField = name
		}
		if meta.relation != nil {
			relation := *meta.relation
			relation.Field = name
			fieldSpec.Relation = &relation
		}
		schema.Fields = append(schema.Fields, fieldSpec)
	}
	return schema
}

type datasourceTagMeta struct {
	id         bool
	title      bool
	completion bool
	views      []string
	relation   *core.DatasourceRelationSpec
}

func parseDatasourceTag(tag string) datasourceTagMeta {
	var meta datasourceTagMeta
	for _, raw := range strings.Split(tag, ",") {
		part := strings.TrimSpace(raw)
		switch {
		case part == "id":
			meta.id = true
		case part == "title":
			meta.title = true
		case part == "completion":
			meta.completion = true
		case strings.HasPrefix(part, "view="):
			meta.views = append(meta.views, splitTagValues(strings.TrimPrefix(part, "view="))...)
		case strings.HasPrefix(part, "relation="):
			relation := parseRelationTag(strings.TrimPrefix(part, "relation="))
			meta.relation = &relation
		}
	}
	meta.views = uniqueStringValues(meta.views)
	return meta
}

func parseRelationTag(value string) core.DatasourceRelationSpec {
	entity, name, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		name = ""
	}
	return core.DatasourceRelationSpec{Name: strings.TrimSpace(name), Entity: strings.TrimSpace(entity), Type: "reference"}
}

func splitTagValues(value string) []string {
	var out []string
	for _, part := range strings.Split(value, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	return name
}

func datasourceFieldType(typ reflect.Type) string {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map:
		return "object"
	case reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

func jsonschemaDescription(tag string) string {
	for _, raw := range strings.Split(tag, ",") {
		part := strings.TrimSpace(raw)
		if strings.HasPrefix(part, "description=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "description="))
		}
	}
	return ""
}

func mergeEntitySchema(base, generated core.DatasourceEntitySchema) core.DatasourceEntitySchema {
	if strings.TrimSpace(generated.Entity) != "" {
		base.Entity = strings.TrimSpace(generated.Entity)
	}
	if strings.TrimSpace(generated.IDField) != "" {
		base.IDField = strings.TrimSpace(generated.IDField)
	}
	if strings.TrimSpace(generated.TitleField) != "" {
		base.TitleField = strings.TrimSpace(generated.TitleField)
	}
	base.Fields = mergeDatasourceFields(base.Fields, generated.Fields)
	return base
}

func cloneEntitySchema(schema core.DatasourceEntitySchema) core.DatasourceEntitySchema {
	schema.Fields = append([]core.DatasourceFieldSpec(nil), schema.Fields...)
	for i := range schema.Fields {
		schema.Fields[i].Views = append([]string(nil), schema.Fields[i].Views...)
		if schema.Fields[i].Relation != nil {
			relation := *schema.Fields[i].Relation
			schema.Fields[i].Relation = &relation
		}
	}
	return schema
}

func mergeDatasourceFields(base, generated []core.DatasourceFieldSpec) []core.DatasourceFieldSpec {
	out := append([]core.DatasourceFieldSpec(nil), base...)
	byName := map[string]int{}
	for i, field := range out {
		byName[field.Name] = i
	}
	for _, field := range generated {
		if field.Name == "" {
			continue
		}
		if idx, ok := byName[field.Name]; ok {
			out[idx] = mergeDatasourceField(out[idx], field)
			continue
		}
		byName[field.Name] = len(out)
		out = append(out, field)
	}
	return out
}

func mergeDatasourceField(base, generated core.DatasourceFieldSpec) core.DatasourceFieldSpec {
	if generated.Type != "" {
		base.Type = generated.Type
	}
	if generated.Description != "" {
		base.Description = generated.Description
	}
	base.Views = uniqueStringValues(append(base.Views, generated.Views...))
	base.Completion = base.Completion || generated.Completion
	if generated.Relation != nil {
		relation := *generated.Relation
		base.Relation = &relation
	}
	return base
}

func uniqueDatasourceFields(fields []core.DatasourceFieldSpec) []core.DatasourceFieldSpec {
	return mergeDatasourceFields(nil, fields)
}

func mergeDatasourceViews(base []core.DatasourceViewSpec, view core.DatasourceViewSpec) []core.DatasourceViewSpec {
	if view.Name == "" {
		return append([]core.DatasourceViewSpec(nil), base...)
	}
	out := append([]core.DatasourceViewSpec(nil), base...)
	for i := range out {
		if out[i].Name == view.Name {
			if view.Description != "" {
				out[i].Description = view.Description
			}
			out[i].Fields = uniqueStringValues(append(out[i].Fields, view.Fields...))
			return out
		}
	}
	return append(out, view)
}

func normalizeDatasourceViews(views []core.DatasourceViewSpec) []core.DatasourceViewSpec {
	var out []core.DatasourceViewSpec
	for _, view := range views {
		view.Name = strings.TrimSpace(view.Name)
		view.Description = strings.TrimSpace(view.Description)
		view.Fields = uniqueStringValues(view.Fields)
		out = mergeDatasourceViews(out, view)
	}
	return out
}

func mergeDatasourceRelations(base []core.DatasourceRelationSpec, relation core.DatasourceRelationSpec) []core.DatasourceRelationSpec {
	if relation.Entity == "" {
		return append([]core.DatasourceRelationSpec(nil), base...)
	}
	out := append([]core.DatasourceRelationSpec(nil), base...)
	for i := range out {
		if out[i].Field == relation.Field && out[i].Entity == relation.Entity && out[i].Name == relation.Name {
			if relation.Type != "" {
				out[i].Type = relation.Type
			}
			return out
		}
	}
	return append(out, relation)
}

func normalizeDatasourceRelations(relations []core.DatasourceRelationSpec) []core.DatasourceRelationSpec {
	var out []core.DatasourceRelationSpec
	for _, relation := range relations {
		relation.Name = strings.TrimSpace(relation.Name)
		relation.Field = strings.TrimSpace(relation.Field)
		relation.Entity = strings.TrimSpace(relation.Entity)
		relation.Type = strings.TrimSpace(relation.Type)
		out = mergeDatasourceRelations(out, relation)
	}
	return out
}

func mergeViewsFromFields(views []core.DatasourceViewSpec, fields []core.DatasourceFieldSpec) []core.DatasourceViewSpec {
	out := append([]core.DatasourceViewSpec(nil), views...)
	for _, field := range fields {
		for _, view := range field.Views {
			out = mergeDatasourceViews(out, core.DatasourceViewSpec{Name: view, Fields: []string{field.Name}})
		}
	}
	return out
}

func mergeRelationsFromFields(relations []core.DatasourceRelationSpec, fields []core.DatasourceFieldSpec) []core.DatasourceRelationSpec {
	out := append([]core.DatasourceRelationSpec(nil), relations...)
	for _, field := range fields {
		if field.Relation == nil {
			continue
		}
		relation := *field.Relation
		if relation.Field == "" {
			relation.Field = field.Name
		}
		out = mergeDatasourceRelations(out, relation)
	}
	return out
}

func mergeCompletionFromFields(completion *core.DatasourceCompletionSpec, fields []core.DatasourceFieldSpec) *core.DatasourceCompletionSpec {
	var out core.DatasourceCompletionSpec
	if completion != nil {
		out = *completion
		out.Fields = append([]string(nil), completion.Fields...)
	}
	for _, field := range fields {
		if field.Completion {
			out.Fields = append(out.Fields, field.Name)
		}
	}
	out.Fields = uniqueStringValues(out.Fields)
	if len(out.Fields) == 0 && out.Description == "" {
		return nil
	}
	return &out
}
