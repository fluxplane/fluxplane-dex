package websearch

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	EntitySearchResult = "web.search_result"

	DefaultMax = 10
	MaxResults = 20

	MetadataProvider  = "websearch.provider"
	MetadataOperation = "websearch.operation"
)

type SearchInput struct {
	Datasource string   `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Queries    []string `json:"queries,omitempty" jsonschema:"description=Search queries to run."`
	Query      string   `json:"query,omitempty" jsonschema:"description=Single search query convenience field."`
	Providers  []string `json:"providers,omitempty" jsonschema:"description=Optional provider names declared by web search provider plugins."`
	Max        int      `json:"max,omitempty" jsonschema:"description=Maximum results per query/provider. Defaults to 10."`
	Limit      int      `json:"limit,omitempty" jsonschema:"description=Alias for max used by datasource search."`
	Entity     string   `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
}

type SearchOutput struct {
	Results []ResultSet   `json:"results,omitempty"`
	Errors  []SearchError `json:"errors,omitempty"`
}

type ResultSet struct {
	Provider string   `json:"provider"`
	Query    string   `json:"query"`
	Answer   string   `json:"answer,omitempty"`
	Results  []Result `json:"results,omitempty"`
}

type Result struct {
	URL     string  `json:"url"`
	Title   string  `json:"title,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	Source  string  `json:"source,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type SearchError struct {
	Provider string `json:"provider,omitempty"`
	Query    string `json:"query,omitempty"`
	Message  string `json:"message"`
}

type SearchRecord struct {
	pluginbinding.DatasourceRecord
	URL      string  `json:"url,omitempty" datasource:"id,completion,view=compact|lookup|table"`
	Snippet  string  `json:"snippet,omitempty" datasource:"view=compact|lookup"`
	Provider string  `json:"provider,omitempty" datasource:"completion,view=compact|table"`
	Score    float64 `json:"score,omitempty"`
}

type DatasourceSearchResult = pluginbinding.DatasourceSearchResult[SearchRecord]
type SearchHandler func(pluginbinding.Context, SearchInput) (SearchOutput, error)

type Provider struct {
	Name       string   `json:"name"`
	Plugin     string   `json:"plugin"`
	Aliases    []string `json:"aliases,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Datasource string   `json:"datasource,omitempty"`
	Entity     string   `json:"entity,omitempty"`
}

type ProviderListResult struct {
	Providers []Provider `json:"providers"`
	Count     int        `json:"count"`
}

type ProviderSpec struct {
	Name                  string
	Version               string
	Description           string
	Aliases               []string
	Operation             string
	Datasource            string
	OperationDescription  string
	DatasourceDescription string
	Auth                  []core.AuthMethod
	SecretPurposes        []string
}

func ProviderManifestSpec(spec ProviderSpec) pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		Aliases:     append([]string(nil), spec.Aliases...),
		Auth:        append([]core.AuthMethod(nil), spec.Auth...),
		Operations: []core.OperationSpec{
			ProviderOperationSpec(spec),
		},
		Datasources: []core.DatasourceSpec{
			ProviderDatasourceSpec(spec),
		},
		Metadata: map[string]string{
			MetadataProvider:  spec.Name,
			MetadataOperation: spec.Operation,
		},
	}
}

func DefineProvider(spec ProviderSpec, search SearchHandler, options ...pluginbinding.PluginOption) *pluginbinding.Plugin {
	bindings := []pluginbinding.PluginOption{
		pluginbinding.RegisterOperation(ProviderOperationSpec(spec), pluginbinding.OperationHandler[SearchInput, SearchOutput](search)),
		pluginbinding.RegisterDatasourceSearch(ProviderDatasourceSpec(spec), DatasourceSearch(search)),
	}
	return pluginbinding.Define(ProviderManifestSpec(spec), append(options, bindings...)...)
}

func ProviderOperationSpec(spec ProviderSpec) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[SearchInput, SearchOutput](
		spec.Operation,
		firstNonEmpty(spec.OperationDescription, "Search the web with "+spec.Name+"."),
		append([]pluginbinding.OperationSpecOption{
			pluginbinding.ReadOnly(),
			pluginbinding.Compact(),
			pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
			pluginbinding.Access(core.OperationAccessNetwork),
			pluginbinding.Risk(core.OperationRiskLow),
			pluginbinding.Idempotency(core.OperationIdempotent),
		}, secretPurposeOptions(spec.SecretPurposes)...)...,
	)
}

func ProviderDatasourceSpec(spec ProviderSpec) core.DatasourceSpec {
	return DatasourceSpec(spec.Datasource, firstNonEmpty(spec.DatasourceDescription, spec.Description), spec.SecretPurposes...)
}

func ProviderFromManifest(entry core.PluginEntry, manifest core.PluginManifest) (Provider, bool) {
	providerName := strings.TrimSpace(manifest.Metadata[MetadataProvider])
	if providerName == "" {
		providerName = entry.Name
	}
	provider := Provider{
		Name:      providerName,
		Plugin:    entry.Name,
		Aliases:   uniqueStrings(append([]string(nil), manifest.Aliases...)),
		Operation: strings.TrimSpace(manifest.Metadata[MetadataOperation]),
	}
	for _, datasource := range manifest.Datasources {
		if datasource.Entity != EntitySearchResult || !containsString(datasource.Capabilities, pluginbinding.CapabilitySearch) {
			continue
		}
		provider.Datasource = datasource.Name
		provider.Entity = datasource.Entity
		return provider, true
	}
	if provider.Operation != "" && strings.TrimSpace(manifest.Metadata[MetadataProvider]) != "" {
		return provider, true
	}
	return Provider{}, false
}

func SelectProviders(available []Provider, requested []string) ([]Provider, []SearchError) {
	if len(requested) == 0 {
		return available, nil
	}
	var selected []Provider
	var errors []SearchError
	seen := map[string]bool{}
	for _, raw := range requested {
		name := NormalizeProviderName(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		provider, ok := FindProvider(available, name)
		if !ok {
			errors = append(errors, SearchError{Provider: name, Message: fmt.Sprintf("web search provider %q is not available", name)})
			continue
		}
		selected = append(selected, provider)
	}
	return selected, errors
}

func FindProvider(providers []Provider, name string) (Provider, bool) {
	name = NormalizeProviderName(name)
	for _, provider := range providers {
		if NormalizeProviderName(provider.Name) == name || NormalizeProviderName(provider.Plugin) == name {
			return provider, true
		}
		for _, alias := range provider.Aliases {
			if NormalizeProviderName(alias) == name {
				return provider, true
			}
		}
	}
	return Provider{}, false
}

func NormalizeQueries(input SearchInput) []string {
	seen := map[string]bool{}
	var out []string
	appendQuery := func(query string) {
		query = strings.TrimSpace(query)
		if query == "" || seen[query] {
			return
		}
		seen[query] = true
		out = append(out, query)
	}
	appendQuery(input.Query)
	for _, query := range input.Queries {
		appendQuery(query)
	}
	return out
}

func NormalizeMax(input SearchInput) int {
	max := input.Max
	if max <= 0 {
		max = input.Limit
	}
	if max <= 0 {
		return DefaultMax
	}
	if max > MaxResults {
		return MaxResults
	}
	return max
}

func NormalizeProviderName(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func NormalizeProviders(providers []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, provider := range providers {
		name := NormalizeProviderName(provider)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func secretPurposeOptions(purposes []string) []pluginbinding.OperationSpecOption {
	if len(purposes) == 0 {
		return nil
	}
	return []pluginbinding.OperationSpecOption{pluginbinding.SecretPurposes(purposes...)}
}

func DatasourceSpec(name, description string, secretPurposes ...string) core.DatasourceSpec {
	options := []pluginbinding.DatasourceSpecOption{
		pluginbinding.EntitySchemaFor[SearchRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "url", TitleField: "title"}),
		pluginbinding.Fallback(core.DatasourceFallbackProviderFirst),
	}
	if len(secretPurposes) > 0 {
		options = append(options, pluginbinding.DatasourceSecretPurposes(secretPurposes...))
	}
	return pluginbinding.TypedDatasourceSpec[SearchInput, DatasourceSearchResult](
		name,
		EntitySearchResult,
		description,
		[]string{pluginbinding.CapabilitySearch},
		options...,
	)
}

func DatasourceSearch(search SearchHandler) pluginbinding.DatasourceHandler[SearchInput, DatasourceSearchResult] {
	return func(ctx pluginbinding.Context, input SearchInput) (DatasourceSearchResult, error) {
		output, err := search(ctx, input)
		if err != nil {
			return DatasourceSearchResult{}, err
		}
		return ToDatasourceSearchResult(ctx.DatasourceSource(), input, output), nil
	}
}

func Records(source pluginbinding.DatasourceSource, sets []ResultSet) []SearchRecord {
	var records []SearchRecord
	for _, set := range sets {
		for _, result := range set.Results {
			url := strings.TrimSpace(result.URL)
			if url == "" {
				continue
			}
			provider := strings.TrimSpace(result.Source)
			if provider == "" {
				provider = set.Provider
			}
			metadata := map[string]any{"provider": provider, "query": set.Query}
			if result.Score != 0 {
				metadata["score"] = result.Score
			}
			if strings.TrimSpace(result.Snippet) != "" {
				metadata["snippet"] = strings.TrimSpace(result.Snippet)
			}
			records = append(records, SearchRecord{
				DatasourceRecord: pluginbinding.NewDatasourceRecord(
					source,
					EntitySearchResult,
					url,
					pluginbinding.RecordTitle(result.Title),
					pluginbinding.RecordLink("self", url),
					pluginbinding.RecordMetadata(metadata),
				),
				URL:      url,
				Snippet:  strings.TrimSpace(result.Snippet),
				Provider: provider,
				Score:    result.Score,
			})
		}
	}
	return records
}

func ToDatasourceSearchResult(source pluginbinding.DatasourceSource, input SearchInput, output SearchOutput) DatasourceSearchResult {
	records := Records(source, output.Results)
	result := pluginbinding.NewDatasourceSearchResult("live", input.Query, records)
	if len(output.Errors) > 0 {
		result.Errors = make([]pluginbinding.DatasourceError, 0, len(output.Errors))
		for _, err := range output.Errors {
			result.Errors = append(result.Errors, pluginbinding.DatasourceError{
				Source:  err.Provider,
				Query:   err.Query,
				Message: err.Message,
			})
		}
	}
	return result
}

func Render(output SearchOutput) string {
	var b strings.Builder
	b.WriteString("Web search results")
	for _, set := range output.Results {
		fmt.Fprintf(&b, "\n\nQuery: %s\nProvider: %s", set.Query, set.Provider)
		if strings.TrimSpace(set.Answer) != "" {
			fmt.Fprintf(&b, "\nAnswer: %s", strings.TrimSpace(set.Answer))
		}
		for i, result := range set.Results {
			fmt.Fprintf(&b, "\n%d. %s\n   %s", i+1, firstNonEmpty(result.Title, result.URL), result.URL)
			if strings.TrimSpace(result.Snippet) != "" {
				fmt.Fprintf(&b, "\n   %s", strings.TrimSpace(result.Snippet))
			}
		}
	}
	if len(output.Errors) > 0 {
		b.WriteString("\n\nErrors")
		for _, err := range output.Errors {
			label := strings.TrimSpace(strings.TrimSpace(err.Provider) + " " + strings.TrimSpace(err.Query))
			if label == "" {
				label = "search"
			}
			fmt.Fprintf(&b, "\n- %s: %s", label, err.Message)
		}
	}
	return strings.TrimSpace(b.String())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
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
