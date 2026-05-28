package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func (r Runner) enrichDatasourceResponse(ctx context.Context, plugin, instance, command string, payload any, resp protocol.Response) (protocol.Response, error) {
	if command != protocol.CommandDatasourcesSearch || !resp.OK || len(resp.Result) == 0 {
		return resp, nil
	}
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return resp, nil
	}
	return enrichDatasourceSearchResponse(manifest, r.State, plugin, instance, payload, resp)
}

func enrichDatasourceSearchResponse(manifest core.PluginManifest, state State, plugin, instance string, payload any, resp protocol.Response) (protocol.Response, error) {
	spec, ok := datasourceSpecForPayload(manifest, payload)
	if !ok || len(spec.Relations) == 0 || spec.EntitySchema == nil {
		return resp, nil
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp, nil
	}
	rawRecords, ok := result["records"].([]any)
	if !ok || len(rawRecords) == 0 {
		return resp, nil
	}
	allowedFields := datasourceEntityFields(spec)
	if len(allowedFields) == 0 {
		return resp, nil
	}
	changed := false
	for _, rawRecord := range rawRecords {
		record, ok := rawRecord.(map[string]any)
		if !ok {
			continue
		}
		for _, relation := range spec.Relations {
			field := strings.TrimSpace(relation.Field)
			entity := strings.TrimSpace(relation.Entity)
			if field == "" || entity == "" {
				continue
			}
			id := recordString(record, field)
			if id == "" {
				continue
			}
			indexed, found, err := state.GetIndexRecordByEntity(plugin, instance, entity, id)
			if err != nil {
				return resp, err
			}
			if !found {
				continue
			}
			if applyIndexedRecordFields(record, indexed, allowedFields, id) {
				changed = true
			}
		}
	}
	if !changed {
		return resp, nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return resp, err
	}
	resp.Result = raw
	return resp, nil
}

func datasourceSpecForPayload(manifest core.PluginManifest, payload any) (core.DatasourceSpec, bool) {
	options := searchPayload(payload)
	datasource := strings.TrimSpace(options.Datasource)
	entity := strings.TrimSpace(options.Entity)
	for _, spec := range manifest.Datasources {
		if datasource != "" && spec.Name == datasource {
			return spec, true
		}
	}
	if entity != "" {
		for _, spec := range manifest.Datasources {
			if spec.Entity == entity {
				return spec, true
			}
		}
	}
	return core.DatasourceSpec{}, false
}

func datasourceEntityFields(spec core.DatasourceSpec) map[string]bool {
	fields := map[string]bool{}
	if spec.EntitySchema == nil {
		return fields
	}
	for _, field := range spec.EntitySchema.Fields {
		name := strings.TrimSpace(field.Name)
		if name != "" {
			fields[name] = true
		}
	}
	return fields
}

func applyIndexedRecordFields(record map[string]any, indexed IndexRecord, allowedFields map[string]bool, relationID string) bool {
	source := indexedRecordFields(indexed)
	changed := false
	for field := range allowedFields {
		value, ok := source[field]
		if !ok || emptyValue(value) {
			continue
		}
		if field == "title" {
			if emptyValue(record[field]) || recordString(record, field) == relationID {
				record[field] = value
				changed = true
			}
			continue
		}
		if emptyValue(record[field]) {
			record[field] = value
			changed = true
		}
	}
	return changed
}

func indexedRecordFields(record IndexRecord) map[string]any {
	fields := map[string]any{}
	_ = json.Unmarshal(record.Record, &fields)
	fields["id"] = record.ID
	fields["entity"] = record.Entity
	if record.Title != "" {
		fields["title"] = record.Title
	}
	if record.URL != "" {
		fields["url"] = record.URL
		fields["web_url"] = record.URL
	}
	if len(record.Links) > 0 {
		fields["links"] = record.Links
	}
	return fields
}

func recordString(record map[string]any, field string) string {
	value, _ := record[field].(string)
	return strings.TrimSpace(value)
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return false
	}
}
