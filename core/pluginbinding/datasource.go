package pluginbinding

import (
	"sort"
	"strings"
)

type DatasourceSource struct {
	Plugin   string `json:"plugin,omitempty"`
	Instance string `json:"instance,omitempty"`
}

type DatasourceRecord struct {
	Entity   string            `json:"entity"`
	ID       string            `json:"id"`
	Source   DatasourceSource  `json:"source,omitempty"`
	Title    string            `json:"title,omitempty"`
	Links    map[string]string `json:"links,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

type LookupSource struct {
	Source   string `json:"source,omitempty"`
	Plugin   string `json:"plugin,omitempty"`
	Instance string `json:"instance,omitempty"`
	Index    string `json:"index,omitempty"`
}

type LookupMatch[R any] struct {
	Source        LookupSource `json:"source"`
	Entity        string       `json:"entity,omitempty"`
	ID            string       `json:"id"`
	Score         int          `json:"score,omitempty"`
	MatchedFields []string     `json:"matched_fields,omitempty"`
	Record        R            `json:"record"`
}

type LookupCandidate struct {
	Source        LookupSource
	Entity        string
	ID            string
	Score         int
	MatchedFields []string
	Record        any
	Values        map[string]string
}

type DatasourceSearchInput struct {
	Datasource  string `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Query       string `json:"query,omitempty" jsonschema:"description=Search query."`
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Maximum records to return."`
	Entity      string `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Resolved endpoint URL."`
}

type DatasourceLookupInput struct {
	Datasource  string   `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Text        string   `json:"text,omitempty" jsonschema:"description=Text to resolve into datasource references."`
	Terms       []string `json:"terms,omitempty" jsonschema:"description=Explicit lookup terms."`
	Limit       int      `json:"limit,omitempty" jsonschema:"description=Maximum matches to return."`
	Entity      string   `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
	EndpointRef string   `json:"endpoint_ref,omitempty" jsonschema:"description=Registered endpoint ref resolved by the host."`
}

type DatasourceGetInput struct {
	Datasource  string `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	ID          string `json:"id,omitempty" jsonschema:"description=Record ID."`
	Entity      string `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered endpoint ref resolved by the host."`
}

type DatasourceSearchResult[T any] struct {
	Source  string            `json:"source"`
	Query   string            `json:"query,omitempty"`
	Count   int               `json:"count"`
	Records []T               `json:"records"`
	Errors  []DatasourceError `json:"errors,omitempty"`
}

type DatasourceError struct {
	Source  string `json:"source,omitempty"`
	Query   string `json:"query,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type DatasourceLookupResult[T any] struct {
	Source  string   `json:"source"`
	Text    string   `json:"text,omitempty"`
	Terms   []string `json:"terms,omitempty"`
	Count   int      `json:"count"`
	Matches []T      `json:"matches"`
}

type DatasourceGetResult[T any] struct {
	Source string `json:"source"`
	Record T      `json:"record"`
}

type DatasourceRecordOption func(*DatasourceRecord)

func (ctx Context) DatasourceSource() DatasourceSource {
	plugin := strings.TrimSpace(ctx.Request.Plugin)
	if plugin == "" && ctx.plugin != nil {
		plugin = ctx.plugin.manifest.Name
	}
	return DatasourceSource{Plugin: plugin, Instance: strings.TrimSpace(ctx.Request.Instance)}
}

func NewDatasourceRecord(source DatasourceSource, entity, id string, options ...DatasourceRecordOption) DatasourceRecord {
	record := DatasourceRecord{
		Entity: strings.TrimSpace(entity),
		ID:     strings.TrimSpace(id),
		Source: source,
	}
	for _, option := range options {
		if option != nil {
			option(&record)
		}
	}
	return record
}

func NewDatasourceSearchResult[T any](source, query string, records []T) DatasourceSearchResult[T] {
	return DatasourceSearchResult[T]{Source: strings.TrimSpace(source), Query: strings.TrimSpace(query), Count: len(records), Records: records}
}

func NewDatasourceLookupResult[T any](source, text string, terms []string, matches []T) DatasourceLookupResult[T] {
	return DatasourceLookupResult[T]{Source: strings.TrimSpace(source), Text: strings.TrimSpace(text), Terms: append([]string(nil), terms...), Count: len(matches), Matches: matches}
}

func NewDatasourceGetResult[T any](source string, record T) DatasourceGetResult[T] {
	return DatasourceGetResult[T]{Source: strings.TrimSpace(source), Record: record}
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
	return LookupMatch[R]{
		Source:        source,
		Entity:        strings.TrimSpace(entity),
		ID:            strings.TrimSpace(id),
		Score:         score,
		MatchedFields: append([]string(nil), matchedFields...),
		Record:        record,
	}
}

func NewLookupCandidate(source LookupSource, entity, id string, record any, values map[string]string) LookupCandidate {
	return LookupCandidate{
		Source: source,
		Entity: strings.TrimSpace(entity),
		ID:     strings.TrimSpace(id),
		Record: record,
		Values: cloneStringMap(values),
	}
}

func NewExactLookupCandidate(source LookupSource, entity, id string, score int, matchedFields []string, record any, values map[string]string) LookupCandidate {
	candidate := NewLookupCandidate(source, entity, id, record, values)
	candidate.Score = score
	candidate.MatchedFields = append([]string(nil), matchedFields...)
	return candidate
}

func NewDatasourceLookupResultFromCandidates(source string, input DatasourceLookupInput, candidates []LookupCandidate) DatasourceLookupResult[LookupMatch[any]] {
	matches := LookupMatches(input, candidates)
	return NewDatasourceLookupResult(source, input.Text, LookupTerms(input), matches)
}

func LookupMatches(input DatasourceLookupInput, candidates []LookupCandidate) []LookupMatch[any] {
	indexByKey := map[string]int{}
	matches := make([]LookupMatch[any], 0, len(candidates))
	for _, candidate := range candidates {
		entity := strings.TrimSpace(candidate.Entity)
		id := strings.TrimSpace(candidate.ID)
		if entity == "" || id == "" {
			continue
		}
		key := candidate.Source.Plugin + "\x00" + candidate.Source.Instance + "\x00" + candidate.Source.Index + "\x00" + entity + "\x00" + id
		score, fields := ScoreLookupValues(input, candidate.Values, 1100)
		if candidate.Score > score {
			score = candidate.Score
		}
		fields = appendMatchedFields(fields, candidate.MatchedFields...)
		if score <= 0 {
			continue
		}
		match := NewLookupMatch(candidate.Source, entity, id, score, fields, candidate.Record)
		if existing, ok := indexByKey[key]; ok {
			if score > matches[existing].Score {
				matches[existing] = match
			}
			continue
		}
		indexByKey[key] = len(matches)
		matches = append(matches, match)
	}
	SortLookupMatches(matches)
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func SortLookupMatches[R any](matches []LookupMatch[R]) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Entity != matches[j].Entity {
			return matches[i].Entity < matches[j].Entity
		}
		return matches[i].ID < matches[j].ID
	})
}

func LookupTerms(input DatasourceLookupInput) []string {
	return LookupTermsFrom(input.Text, input.Terms)
}

func LookupLimit(input DatasourceLookupInput, fallback, max int) int {
	limit := input.Limit
	if limit <= 0 {
		limit = fallback
	}
	if max > 0 && limit > max {
		limit = max
	}
	return limit
}

func FilterLookupTerms(input DatasourceLookupInput, max int, keep func(string) bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range LookupTerms(input) {
		term = strings.TrimSpace(term)
		if term == "" || seen[term] {
			continue
		}
		if keep != nil && !keep(term) {
			continue
		}
		seen[term] = true
		out = append(out, term)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func LookupTermsFrom(text string, explicit []string) []string {
	seen := map[string]bool{}
	var terms []string
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		terms = append(terms, value)
	}
	for _, term := range explicit {
		add(term)
	}
	text = strings.TrimSpace(text)
	if text != "" {
		add(text)
		for _, token := range strings.Fields(text) {
			token = strings.Trim(token, " \t\n\r\"'()[]{}<>.,;:#!")
			if len(token) >= 2 && !lookupStopword(token) {
				add(token)
			}
		}
	}
	return terms
}

func ScoreLookupValues(input DatasourceLookupInput, values map[string]string, exactScore int) (int, []string) {
	text := strings.ToLower(strings.TrimSpace(input.Text))
	terms := LookupTerms(input)
	score := 0
	var fields []string
	for field, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		matched := false
		switch {
		case text != "" && strings.Contains(text, value):
			score = maxInt(score, exactScore)
			matched = true
		default:
			for _, term := range terms {
				switch {
				case term == value:
					score = maxInt(score, exactScore-50)
					matched = true
				case strings.Contains(value, term):
					score = maxInt(score, exactScore-250)
					matched = true
				}
			}
		}
		if matched {
			fields = appendUnique(fields, field)
		}
	}
	return score, fields
}

func RecordTitle(title string) DatasourceRecordOption {
	return func(record *DatasourceRecord) {
		record.Title = strings.TrimSpace(title)
	}
}

func RecordLink(name, url string) DatasourceRecordOption {
	return func(record *DatasourceRecord) {
		name = strings.TrimSpace(name)
		url = strings.TrimSpace(url)
		if name == "" || url == "" {
			return
		}
		if record.Links == nil {
			record.Links = map[string]string{}
		}
		record.Links[name] = url
	}
}

func RecordMetadata(metadata map[string]any) DatasourceRecordOption {
	return func(record *DatasourceRecord) {
		if len(metadata) == 0 {
			return
		}
		record.Metadata = cloneAnyMap(metadata)
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func lookupStopword(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "look", "lookup", "find", "open", "see", "the", "for", "from", "this", "that", "please", "at", "in", "to", "and", "with":
		return true
	default:
		return false
	}
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func appendMatchedFields(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			values = appendUnique(values, candidate)
		}
	}
	return values
}

func maxInt(left, right int) int {
	if right > left {
		return right
	}
	return left
}
