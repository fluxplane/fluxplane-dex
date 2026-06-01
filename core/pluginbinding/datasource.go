package pluginbinding

import (
	"strings"

	datasource "github.com/fluxplane/fluxplane-datasource"
)

type DatasourceSource = datasource.Source
type DatasourceRecord = datasource.RecordBase
type LookupSource = datasource.LookupSource
type LookupMatch[R any] = datasource.LookupMatch[R]
type LookupCandidate = datasource.LookupCandidate
type DatasourceSearchInput = datasource.SearchInput
type DatasourceLookupInput = datasource.LookupInput
type DatasourceGetInput = datasource.GetInput
type DatasourceSearchResult[T any] = datasource.SearchOutput[T]
type DatasourceError = datasource.Error
type DatasourceLookupResult[T any] = datasource.LookupOutput[T]
type DatasourceGetResult[T any] = datasource.GetOutput[T]
type DatasourceRecordOption = datasource.RecordOption

func (ctx Context) DatasourceSource() DatasourceSource {
	plugin := strings.TrimSpace(ctx.Request.Plugin)
	if plugin == "" && ctx.plugin != nil {
		plugin = ctx.plugin.manifest.Name
	}
	return DatasourceSource{Plugin: plugin, Instance: strings.TrimSpace(ctx.Request.Instance)}
}

func NewDatasourceRecord(source DatasourceSource, entity, id string, options ...DatasourceRecordOption) DatasourceRecord {
	return datasource.NewRecord(source, entity, id, options...)
}

func NewDatasourceSearchResult[T any](source, query string, records []T) DatasourceSearchResult[T] {
	return datasource.NewSearchOutput(source, query, records)
}

func NewDatasourceLookupResult[T any](source, text string, terms []string, matches []T) DatasourceLookupResult[T] {
	return datasource.NewLookupOutput(source, text, terms, matches)
}

func NewDatasourceGetResult[T any](source string, record T) DatasourceGetResult[T] {
	return datasource.NewGetOutput(source, record)
}

func (ctx Context) LookupSource(source, index string) LookupSource {
	origin := LookupSource{Source: strings.TrimSpace(source), Instance: strings.TrimSpace(ctx.Request.Instance), Index: strings.TrimSpace(index)}
	origin.Plugin = strings.TrimSpace(ctx.Request.Plugin)
	if origin.Plugin == "" && ctx.plugin != nil {
		origin.Plugin = ctx.plugin.manifest.Name
	}
	return origin
}

func NewLookupMatch[R any](source LookupSource, entity, id string, score int, matchedFields []string, record R) LookupMatch[R] {
	return datasource.NewLookupMatch(source, entity, id, score, matchedFields, record)
}

func NewLookupCandidate(source LookupSource, entity, id string, record any, values map[string]string) LookupCandidate {
	return datasource.NewLookupCandidate(source, entity, id, record, values)
}

func NewExactLookupCandidate(source LookupSource, entity, id string, score int, matchedFields []string, record any, values map[string]string) LookupCandidate {
	return datasource.NewExactLookupCandidate(source, entity, id, score, matchedFields, record, values)
}

func NewDatasourceLookupResultFromCandidates(source string, input DatasourceLookupInput, candidates []LookupCandidate) DatasourceLookupResult[LookupMatch[any]] {
	return datasource.NewLookupOutputFromCandidates(source, input, candidates)
}

func LookupMatches(input DatasourceLookupInput, candidates []LookupCandidate) []LookupMatch[any] {
	return datasource.LookupMatches(input, candidates)
}

func SortLookupMatches[R any](matches []LookupMatch[R]) {
	datasource.SortLookupMatches(matches)
}

func LookupTerms(input DatasourceLookupInput) []string {
	return datasource.LookupTerms(input)
}

func LookupLimit(input DatasourceLookupInput, fallback, max int) int {
	return datasource.LookupLimit(input, fallback, max)
}

func FilterLookupTerms(input DatasourceLookupInput, max int, keep func(string) bool) []string {
	return datasource.FilterLookupTerms(input, max, keep)
}

func LookupTermsFrom(text string, explicit []string) []string {
	return datasource.LookupTermsFrom(text, explicit)
}

func ScoreLookupValues(input DatasourceLookupInput, values map[string]string, exactScore int) (int, []string) {
	return datasource.ScoreLookupValues(input, values, exactScore)
}

func RecordTitle(title string) DatasourceRecordOption {
	return datasource.RecordTitle(title)
}

func RecordLink(name, url string) DatasourceRecordOption {
	return datasource.RecordLink(name, url)
}

func RecordMetadata(metadata map[string]any) DatasourceRecordOption {
	return datasource.RecordMetadata(metadata)
}
