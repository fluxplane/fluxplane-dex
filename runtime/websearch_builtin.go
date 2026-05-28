package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	websearchPluginName        = "websearch"
	websearchOperationSearch   = "websearch.search"
	websearchOperationProvider = "websearch.provider.list"
	websearchDatasource        = "websearch.results"
)

type websearchNoInput struct{}

func isBuiltinPlugin(entry core.PluginEntry) bool {
	return strings.TrimSpace(entry.Metadata["kind"]) == "builtin" || strings.TrimSpace(entry.Metadata["builtin"]) == "true"
}

func (r Runner) invokeBuiltin(ctx context.Context, entry core.PluginEntry, req protocol.Request) (protocol.Response, error) {
	switch entry.Name {
	case websearchPluginName:
		return r.websearchPlugin(ctx).Handle(req), nil
	default:
		return protocol.Response{}, fmt.Errorf("unknown builtin plugin %q", entry.Name)
	}
}

func websearchManifest() core.PluginManifest {
	return pluginbinding.Manifest(websearchManifestSpec())
}

func websearchManifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        websearchPluginName,
		Version:     "0.1.0",
		Description: "Generic web search aggregator over provider plugins.",
		Aliases:     []string{"web", "websearch"},
		Operations: []core.OperationSpec{
			websearchProviderSpec(),
			websearchSearchSpec(),
		},
		Datasources: []core.DatasourceSpec{
			websearchDatasourceSpec(),
		},
		Metadata: map[string]string{"kind": "builtin"},
	}
}

type websearchBuiltinService struct {
	runner Runner
	ctx    context.Context
}

func (r Runner) websearchPlugin(ctx context.Context) *pluginbinding.Plugin {
	if ctx == nil {
		ctx = context.Background()
	}
	service := websearchBuiltinService{runner: r, ctx: ctx}
	return pluginbinding.Define(websearchManifestSpec(),
		pluginbinding.RegisterOperation(websearchProviderSpec(), service.Providers),
		pluginbinding.RegisterOperation(websearchSearchSpec(), service.Search),
		pluginbinding.RegisterDatasourceSearch(websearchDatasourceSpec(), service.DatasourceSearch),
	)
}

func websearchProviderSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[websearchNoInput, websearch.ProviderListResult](websearchOperationProvider, "List available web search provider plugins.", pluginbinding.ReadOnly(), pluginbinding.Compact())
}

func websearchSearchSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[websearch.SearchInput, websearch.SearchOutput](websearchOperationSearch, "Search the web through provider plugins.", pluginbinding.ReadOnly(), pluginbinding.Compact())
}

func websearchDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[websearch.SearchInput, websearch.DatasourceSearchResult](
		websearchDatasource,
		websearch.EntitySearchResult,
		"Aggregated web search results.",
		[]string{pluginbinding.CapabilitySearch},
	)
}

func (s websearchBuiltinService) Providers(pluginbinding.Context, websearchNoInput) (websearch.ProviderListResult, error) {
	providers := s.runner.websearchProviders(s.ctx)
	return websearch.ProviderListResult{Providers: providers, Count: len(providers)}, nil
}

func (s websearchBuiltinService) Search(ctx pluginbinding.Context, input websearch.SearchInput) (websearch.SearchOutput, error) {
	output := s.runner.runWebsearch(s.ctx, ctx.Request.Instance, input)
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("web_search_failed", firstWebsearchError(output, "web search returned no results"))
	}
	return output, nil
}

func (s websearchBuiltinService) DatasourceSearch(ctx pluginbinding.Context, input websearch.SearchInput) (websearch.DatasourceSearchResult, error) {
	output := s.runner.runWebsearch(s.ctx, ctx.Request.Instance, input)
	result := websearch.ToDatasourceSearchResult(ctx.DatasourceSource(), input, output)
	if len(result.Records) == 0 {
		return result, pluginbinding.Fail("web_search_failed", firstWebsearchError(output, "web search returned no results"))
	}
	return result, nil
}

func (r Runner) websearchProviders(ctx context.Context) []websearch.Provider {
	var providers []websearch.Provider
	for _, entry := range r.Marketplace.Plugins() {
		if entry.Name == websearchPluginName || isBuiltinPlugin(entry) {
			continue
		}
		manifest, err := r.manifest(ctx, entry.Name)
		if err != nil {
			continue
		}
		provider, ok := websearch.ProviderFromManifest(entry, manifest)
		if ok {
			providers = append(providers, provider)
		}
	}
	return providers
}

func (r Runner) runWebsearch(ctx context.Context, instance string, input websearch.SearchInput) websearch.SearchOutput {
	queries := websearch.NormalizeQueries(input)
	if len(queries) == 0 {
		return websearch.SearchOutput{Errors: []websearch.SearchError{{Message: "at least one query is required"}}}
	}
	providers, errors := websearch.SelectProviders(r.websearchProviders(ctx), input.Providers)
	output := websearch.SearchOutput{Errors: errors}
	if len(providers) == 0 {
		if len(output.Errors) == 0 {
			output.Errors = append(output.Errors, websearch.SearchError{Message: "no web search provider is available"})
		}
		return output
	}
	max := websearch.NormalizeMax(input)
	for _, query := range queries {
		for _, provider := range providers {
			set, err := r.runProviderWebsearch(ctx, instance, provider, query, max)
			if err != nil {
				output.Errors = append(output.Errors, websearch.SearchError{Provider: provider.Name, Query: query, Message: err.Error()})
				continue
			}
			output.Results = append(output.Results, set)
		}
	}
	return output
}

func (r Runner) runProviderWebsearch(ctx context.Context, instance string, provider websearch.Provider, query string, max int) (websearch.ResultSet, error) {
	if strings.TrimSpace(provider.Operation) != "" {
		inputRaw, err := json.Marshal(websearch.SearchInput{Query: query, Max: max})
		if err != nil {
			return websearch.ResultSet{}, err
		}
		resp, err := r.InvokeInstance(ctx, provider.Plugin, instance, protocol.CommandOperationsCall, protocol.OperationCall{Name: provider.Operation, Input: inputRaw})
		if err != nil {
			return websearch.ResultSet{}, err
		}
		if !resp.OK {
			if resp.Error != nil {
				return websearch.ResultSet{}, fmt.Errorf("%s", resp.Error.Message)
			}
			return websearch.ResultSet{}, fmt.Errorf("provider operation failed")
		}
		var output websearch.SearchOutput
		if err := json.Unmarshal(resp.Result, &output); err != nil {
			return websearch.ResultSet{}, err
		}
		if len(output.Results) == 0 {
			return websearch.ResultSet{}, fmt.Errorf("%s", firstWebsearchError(output, "provider returned no results"))
		}
		set := output.Results[0]
		if strings.TrimSpace(set.Provider) == "" {
			set.Provider = provider.Name
		}
		return set, nil
	}
	resp, err := r.InvokeInstance(ctx, provider.Plugin, instance, protocol.CommandDatasourcesSearch, map[string]any{"query": query, "entity": websearch.EntitySearchResult, "limit": max})
	if err != nil {
		return websearch.ResultSet{}, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return websearch.ResultSet{}, fmt.Errorf("%s", resp.Error.Message)
		}
		return websearch.ResultSet{}, fmt.Errorf("provider datasource search failed")
	}
	var out struct {
		Records []struct {
			ID       string            `json:"id"`
			Title    string            `json:"title"`
			Links    map[string]string `json:"links"`
			Metadata map[string]any    `json:"metadata"`
			URL      string            `json:"url"`
			Snippet  string            `json:"snippet"`
			Provider string            `json:"provider"`
			Score    float64           `json:"score"`
		} `json:"records"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return websearch.ResultSet{}, err
	}
	set := websearch.ResultSet{Provider: provider.Name, Query: query}
	for _, record := range out.Records {
		url := firstNonEmptyString(record.URL, record.Links["self"], record.ID)
		source := firstNonEmptyString(record.Provider, metadataString(record.Metadata, "provider"), provider.Name)
		set.Results = append(set.Results, websearch.Result{URL: url, Title: record.Title, Snippet: record.Snippet, Source: source, Score: record.Score})
	}
	if len(set.Results) == 0 {
		return websearch.ResultSet{}, fmt.Errorf("provider returned no results")
	}
	return set, nil
}

func decodeOperationInput[T any](call protocol.OperationCall) (T, error) {
	var input T
	if len(call.Input) == 0 {
		return input, nil
	}
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return input, err
	}
	return input, nil
}

func websearchDatasourceSource(req protocol.Request) pluginbinding.DatasourceSource {
	return pluginbinding.DatasourceSource{Plugin: websearchPluginName, Instance: NormalizeInstance(req.Instance)}
}

func firstWebsearchError(output websearch.SearchOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func metadataString(metadata map[string]any, key string) string {
	if value, ok := metadata[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
