package fluxplaneplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	coredatasource "github.com/fluxplane/fluxplane-core/core/datasource"
	"github.com/fluxplane/fluxplane-core/orchestration/pluginhost"

	dex "github.com/fluxplane/fluxplane-dex"
)

// DatasourceProviders bridges dex-declared datasources to the core
// datasource.Provider surface. One Provider is returned per dex plugin; it
// exposes every entity declared in the dex manifest. Open() yields an
// Accessor that proxies Search and Get through the dex Datasources service.
//
// Lifecycle: the Provider is constructed from a manifest snapshot taken at
// resolution time. Plugins whose manifest can't be fetched yield a Provider
// that has no entities — callers see an empty set rather than a hard failure
// so listing is robust to transient dex errors.
func (a *adapter) DatasourceProviders(ctx context.Context, _ pluginhost.Context) ([]coredatasource.Provider, error) {
	manifest, err := a.engine.Manifest(ctx, a.name)
	if err != nil {
		return []coredatasource.Provider{&dexDatasourceProvider{plugin: a.name, engine: a.engine}}, nil
	}
	specs := append([]dex.DatasourceSpec(nil), manifest.Datasources...)
	return []coredatasource.Provider{&dexDatasourceProvider{
		plugin:  a.name,
		engine:  a.engine,
		sources: specs,
	}}, nil
}

type dexDatasourceProvider struct {
	plugin  string
	engine  *dex.Engine
	sources []dex.DatasourceSpec
}

// Entities returns the union of entity specs declared across this plugin's
// datasources. EntitySpec.Fields/Capabilities are best-effort mappings from
// the dex DatasourceEntitySchema.
func (p *dexDatasourceProvider) Entities() []coredatasource.EntitySpec {
	seen := map[string]bool{}
	out := make([]coredatasource.EntitySpec, 0, len(p.sources))
	for _, src := range p.sources {
		entity := strings.TrimSpace(src.Entity)
		if entity == "" || seen[entity] {
			continue
		}
		seen[entity] = true
		out = append(out, dexEntitySpec(src))
	}
	return out
}

// Open returns an Accessor for the configured spec. The dex datasource is
// resolved by matching either spec.Name to a dex datasource name, or by
// matching the first entity in spec.Entities to a dex datasource entity.
func (p *dexDatasourceProvider) Open(_ context.Context, spec coredatasource.Spec) (coredatasource.Accessor, error) {
	src, ok := p.resolveSource(spec)
	if !ok {
		return nil, fmt.Errorf("fluxplaneplugin: no dex datasource matches %q (entities=%v)", spec.Name, spec.Entities)
	}
	return &dexAccessor{plugin: p.plugin, engine: p.engine, spec: spec, source: src}, nil
}

func (p *dexDatasourceProvider) resolveSource(spec coredatasource.Spec) (dex.DatasourceSpec, bool) {
	name := strings.TrimSpace(string(spec.Name))
	for _, src := range p.sources {
		if src.Name == name {
			return src, true
		}
	}
	if len(spec.Entities) > 0 {
		want := strings.TrimSpace(string(spec.Entities[0]))
		for _, src := range p.sources {
			if src.Entity == want {
				return src, true
			}
		}
	}
	return dex.DatasourceSpec{}, false
}

func dexEntitySpec(src dex.DatasourceSpec) coredatasource.EntitySpec {
	entity := coredatasource.EntitySpec{
		Type:        coredatasource.EntityType(src.Entity),
		Description: src.Description,
	}
	for _, cap := range src.Capabilities {
		cap = strings.TrimSpace(cap)
		if cap != "" {
			entity.Capabilities = append(entity.Capabilities, coredatasource.EntityCapability(cap))
		}
	}
	if src.EntitySchema != nil {
		for _, f := range src.EntitySchema.Fields {
			entity.Fields = append(entity.Fields, coredatasource.FieldSpec{
				Name:        f.Name,
				Type:        coredatasource.FieldType(f.Type),
				Description: f.Description,
			})
		}
	}
	return entity
}

// dexAccessor implements the base Accessor plus Searcher and Getter. We do
// not implement Lister, BatchGetter, or Relationer because dex's protocol
// does not yet expose stable equivalents — callers should fall back to
// search/get.
type dexAccessor struct {
	plugin string
	engine *dex.Engine
	spec   coredatasource.Spec
	source dex.DatasourceSpec
}

var (
	_ coredatasource.Accessor = (*dexAccessor)(nil)
	_ coredatasource.Searcher = (*dexAccessor)(nil)
	_ coredatasource.Getter   = (*dexAccessor)(nil)
)

func (a *dexAccessor) Spec() coredatasource.Spec { return a.spec }

func (a *dexAccessor) Entities() []coredatasource.EntitySpec {
	return []coredatasource.EntitySpec{dexEntitySpec(a.source)}
}

// Search proxies a search through dex. The request shape sent to dex follows
// the dex datasource search convention: {datasource, entity, query, limit,
// filters}. Dex plugins decode and respond with their own record shape; we
// extract a best-effort list of records by looking for a top-level "records"
// or "results" array, falling back to wrapping the raw payload as a single
// Record with the JSON-encoded content.
func (a *dexAccessor) Search(ctx context.Context, req coredatasource.SearchRequest) (coredatasource.SearchResult, error) {
	entity := strings.TrimSpace(string(req.Entity))
	if entity == "" {
		entity = a.source.Entity
	}
	payload := map[string]any{
		"datasource": a.source.Name,
		"entity":     entity,
	}
	if q := strings.TrimSpace(req.Query); q != "" {
		payload["query"] = q
	}
	if req.Limit > 0 {
		payload["limit"] = req.Limit
	}
	if len(req.Filters) > 0 {
		payload["filters"] = req.Filters
	}
	resp, err := a.engine.Datasources().Search(ctx, a.plugin, payload)
	if err != nil {
		return coredatasource.SearchResult{}, err
	}
	if resp.Error != nil {
		return coredatasource.SearchResult{}, fmt.Errorf("dex %s: %s", a.plugin, resp.Error.Message)
	}
	result := coredatasource.SearchResult{
		Datasource: coredatasource.Name(a.source.Name),
		Entity:     coredatasource.EntityType(entity),
	}
	result.Records = decodeRecords(resp.Result, result.Datasource, result.Entity)
	if total, ok := decodeTotal(resp.Result); ok {
		result.Total = total
	}
	return result, nil
}

func (a *dexAccessor) Get(ctx context.Context, req coredatasource.GetRequest) (coredatasource.Record, error) {
	entity := strings.TrimSpace(string(req.Entity))
	if entity == "" {
		entity = a.source.Entity
	}
	payload := map[string]any{
		"datasource": a.source.Name,
		"entity":     entity,
		"id":         req.ID,
	}
	resp, err := a.engine.Datasources().Get(ctx, a.plugin, payload)
	if err != nil {
		return coredatasource.Record{}, err
	}
	if resp.Error != nil {
		return coredatasource.Record{}, fmt.Errorf("dex %s: %s", a.plugin, resp.Error.Message)
	}
	records := decodeRecords(resp.Result, coredatasource.Name(a.source.Name), coredatasource.EntityType(entity))
	if len(records) > 0 {
		return records[0], nil
	}
	return coredatasource.Record{
		ID:         req.ID,
		Datasource: coredatasource.Name(a.source.Name),
		Entity:     coredatasource.EntityType(entity),
		Raw:        json.RawMessage(resp.Result),
	}, nil
}

// decodeRecords best-effort flattens a dex JSON payload into core Records.
// Supported shapes:
//   - {"records":[…]} or {"results":[…]} — each element is a record-shaped object;
//   - a top-level array of record-shaped objects;
//   - a single record-shaped object — returned as a one-element list.
func decodeRecords(raw json.RawMessage, ds coredatasource.Name, entity coredatasource.EntityType) []coredatasource.Record {
	if len(raw) == 0 {
		return nil
	}
	var asObject map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObject); err == nil {
		for _, key := range []string{"records", "results", "items"} {
			if arr, ok := asObject[key]; ok {
				return recordsFromArray(arr, ds, entity)
			}
		}
		return []coredatasource.Record{recordFromObject(asObject, ds, entity)}
	}
	var asArray []json.RawMessage
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return recordsFromArrayElements(asArray, ds, entity)
	}
	return nil
}

func recordsFromArray(arr json.RawMessage, ds coredatasource.Name, entity coredatasource.EntityType) []coredatasource.Record {
	var elems []json.RawMessage
	if err := json.Unmarshal(arr, &elems); err != nil {
		return nil
	}
	return recordsFromArrayElements(elems, ds, entity)
}

func recordsFromArrayElements(elems []json.RawMessage, ds coredatasource.Name, entity coredatasource.EntityType) []coredatasource.Record {
	out := make([]coredatasource.Record, 0, len(elems))
	for _, elem := range elems {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(elem, &obj); err != nil {
			continue
		}
		out = append(out, recordFromObject(obj, ds, entity))
	}
	return out
}

func recordFromObject(obj map[string]json.RawMessage, ds coredatasource.Name, entity coredatasource.EntityType) coredatasource.Record {
	rec := coredatasource.Record{
		Datasource: ds,
		Entity:     entity,
	}
	if raw, ok := obj["id"]; ok {
		_ = json.Unmarshal(raw, &rec.ID)
	}
	if raw, ok := obj["title"]; ok {
		_ = json.Unmarshal(raw, &rec.Title)
	}
	if raw, ok := obj["url"]; ok {
		_ = json.Unmarshal(raw, &rec.URL)
	}
	if raw, ok := obj["content"]; ok {
		_ = json.Unmarshal(raw, &rec.Content)
	}
	if raw, ok := obj["score"]; ok {
		_ = json.Unmarshal(raw, &rec.Score)
	}
	if raw, ok := obj["metadata"]; ok {
		_ = json.Unmarshal(raw, &rec.Metadata)
	}
	// Preserve full payload for downstream consumers that need fields the
	// adapter doesn't map.
	flat := map[string]json.RawMessage{}
	for k, v := range obj {
		flat[k] = v
	}
	rec.Raw = flat
	return rec
}

func decodeTotal(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var obj struct {
		Total int `json:"total"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0, false
	}
	if obj.Total > 0 {
		return obj.Total, true
	}
	if obj.Count > 0 {
		return obj.Count, true
	}
	return 0, false
}
