package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type IndexSnapshot struct {
	Plugin    string          `json:"plugin"`
	Instance  string          `json:"instance"`
	Index     string          `json:"index"`
	Records   []IndexRecord   `json:"records"`
	UpdatedAt time.Time       `json:"updated_at"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type IndexRecord struct {
	Entity        string            `json:"entity,omitempty"`
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	URL           string            `json:"url,omitempty"`
	Links         map[string]string `json:"links,omitempty"`
	Origin        IndexOrigin       `json:"origin,omitempty"`
	Score         int               `json:"score,omitempty"`
	MatchedFields []string          `json:"matched_fields,omitempty"`
	Record        json.RawMessage   `json:"record"`
}

type IndexOrigin = pluginbinding.LookupSource

type IndexStatus struct {
	Plugin    string             `json:"plugin"`
	Instance  string             `json:"instance"`
	Indexes   []string           `json:"indexes"`
	Records   int                `json:"records"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
	Details   []IndexStatusEntry `json:"details,omitempty"`
}

type IndexStatusEntry struct {
	Index     string          `json:"index"`
	Records   int             `json:"records"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type SearchOptions struct {
	Datasource string
	Query      string
	Limit      int
	Entity     string
}

type LookupOptions struct {
	Datasource string
	Text       string
	Terms      []string
	Limit      int
	Entity     string
}

type LookupMatch = pluginbinding.LookupMatch[IndexRecord]

func (s State) IndexDir() string {
	return filepath.Join(s.Home, "indexes")
}

func (s State) SaveIndexRecords(plugin, instance, index string, records []json.RawMessage) (IndexSnapshot, error) {
	return s.SaveIndexRecordsWithMetadata(plugin, instance, index, records, nil)
}

func (s State) SaveIndexRecordsWithMetadata(plugin, instance, index string, records []json.RawMessage, metadata json.RawMessage) (IndexSnapshot, error) {
	plugin = strings.TrimSpace(plugin)
	instance = NormalizeInstance(instance)
	index = strings.TrimSpace(index)
	if plugin == "" || index == "" {
		return IndexSnapshot{}, fmt.Errorf("plugin and index are required")
	}
	snapshot := IndexSnapshot{
		Plugin:    plugin,
		Instance:  instance,
		Index:     index,
		UpdatedAt: time.Now().UTC(),
	}
	for _, raw := range records {
		record, ok := normalizeIndexRecord(raw)
		if !ok {
			continue
		}
		snapshot.Records = append(snapshot.Records, record)
	}
	snapshot.Metadata = normalizeIndexMetadata(metadata, snapshot)
	path := s.indexPath(plugin, instance, index)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return IndexSnapshot{}, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return IndexSnapshot{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return IndexSnapshot{}, err
	}
	return snapshot, nil
}

func (s State) SearchIndex(plugin, instance, query string, limit int) ([]IndexRecord, error) {
	return s.SearchIndexWithOptions(plugin, instance, SearchOptions{Query: query, Limit: limit})
}

func (s State) SearchIndexWithOptions(plugin, instance string, options SearchOptions) ([]IndexRecord, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(options.Query))
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	entity := strings.TrimSpace(options.Entity)
	var out []IndexRecord
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			record = enrichIndexRecord(snapshot, record)
			if entity != "" && record.Entity != entity {
				continue
			}
			score, fields := indexRecordScore(record, query)
			if query == "" || score > 0 {
				record.Score = score
				record.MatchedFields = fields
				out = append(out, record)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Entity != out[j].Entity {
			return out[i].Entity < out[j].Entity
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s State) LookupIndexWithOptions(plugin, instance string, options LookupOptions) ([]LookupMatch, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return nil, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	entity := strings.TrimSpace(options.Entity)
	terms := lookupTerms(options)
	text := strings.ToLower(strings.TrimSpace(options.Text))
	var matches []LookupMatch
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			record = enrichIndexRecord(snapshot, record)
			if entity != "" && record.Entity != entity {
				continue
			}
			score, fields := lookupRecordScore(record, text, terms)
			if score == 0 {
				continue
			}
			record.Score = score
			record.MatchedFields = fields
			matches = append(matches, LookupMatch{
				Source:        record.Origin,
				Entity:        record.Entity,
				ID:            record.ID,
				Score:         score,
				MatchedFields: fields,
				Record:        record,
			})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Entity != matches[j].Entity {
			return matches[i].Entity < matches[j].Entity
		}
		return matches[i].ID < matches[j].ID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func (s State) GetIndexRecord(plugin, instance, id string) (IndexRecord, bool, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return IndexRecord{}, false, err
	}
	id = strings.TrimSpace(id)
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			if record.ID == id {
				return enrichIndexRecord(snapshot, record), true, nil
			}
		}
	}
	return IndexRecord{}, false, nil
}

func (s State) GetIndexRecordByEntity(plugin, instance, entity, id string) (IndexRecord, bool, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return IndexRecord{}, false, err
	}
	entity = strings.TrimSpace(entity)
	id = strings.TrimSpace(id)
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			if record.ID != id {
				continue
			}
			record = enrichIndexRecord(snapshot, record)
			if entity != "" && record.Entity != entity {
				continue
			}
			return record, true, nil
		}
	}
	return IndexRecord{}, false, nil
}

func (s State) HasIndex(plugin, instance, index string) (bool, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return false, err
	}
	index = strings.TrimSpace(index)
	for _, snapshot := range snapshots {
		if snapshot.Index == index {
			return true, nil
		}
	}
	return false, nil
}

func (s State) HasIndexedEntity(plugin, instance, entity string) (bool, error) {
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return false, err
	}
	entity = strings.TrimSpace(entity)
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			if record.Entity == entity {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s State) HasIndexRecords(plugin, instance string) (bool, error) {
	status, err := s.IndexStatus(plugin, instance)
	if err != nil {
		return false, err
	}
	return status.Records > 0, nil
}

func (s State) IndexStatus(plugin, instance string) (IndexStatus, error) {
	instance = NormalizeInstance(instance)
	snapshots, err := s.loadIndexSnapshots(plugin, instance)
	if err != nil {
		return IndexStatus{}, err
	}
	status := IndexStatus{Plugin: strings.TrimSpace(plugin), Instance: instance}
	for _, snapshot := range snapshots {
		status.Indexes = append(status.Indexes, snapshot.Index)
		status.Records += len(snapshot.Records)
		if snapshot.UpdatedAt.After(status.UpdatedAt) {
			status.UpdatedAt = snapshot.UpdatedAt
		}
		status.Details = append(status.Details, IndexStatusEntry{
			Index:     snapshot.Index,
			Records:   len(snapshot.Records),
			UpdatedAt: snapshot.UpdatedAt,
			Metadata:  snapshot.Metadata,
		})
	}
	sort.Strings(status.Indexes)
	sort.Slice(status.Details, func(i, j int) bool { return status.Details[i].Index < status.Details[j].Index })
	return status, nil
}

func (s State) loadIndexSnapshots(plugin, instance string) ([]IndexSnapshot, error) {
	dir := filepath.Join(s.IndexDir(), safeName(plugin), safeName(NormalizeInstance(instance)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snapshots []IndexSnapshot
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var snapshot IndexSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Index < snapshots[j].Index })
	return snapshots, nil
}

func (s State) indexPath(plugin, instance, index string) string {
	return filepath.Join(s.IndexDir(), safeName(plugin), safeName(instance), safeName(index)+".json")
}

func normalizeIndexMetadata(raw json.RawMessage, snapshot IndexSnapshot) json.RawMessage {
	metadata := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &metadata)
	}
	if _, ok := metadata["built_at"]; !ok {
		metadata["built_at"] = snapshot.UpdatedAt.Format(time.RFC3339)
	}
	if _, ok := metadata["plugin"]; !ok {
		metadata["plugin"] = snapshot.Plugin
	}
	if _, ok := metadata["instance"]; !ok {
		metadata["instance"] = snapshot.Instance
	}
	if _, ok := metadata["index"]; !ok {
		metadata["index"] = snapshot.Index
	}
	if _, ok := metadata["records"]; !ok {
		metadata["records"] = len(snapshot.Records)
	}
	out, _ := json.Marshal(metadata)
	return out
}

func normalizeIndexRecord(raw json.RawMessage) (IndexRecord, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return IndexRecord{}, false
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return IndexRecord{}, false
	}
	id := firstRecordString(object, "id", "key", "ref", "path_with_namespace", "web_url")
	if id == "" {
		id = firstRecordString(object, "project_id")
	}
	if id == "" {
		return IndexRecord{}, false
	}
	record := IndexRecord{
		Entity: firstRecordString(object, "entity"),
		ID:     id,
	}
	record = applyStandardIndexFields(record, object)
	record.Record = cleanIndexRecordRaw(object)
	return record, true
}

func enrichIndexRecord(snapshot IndexSnapshot, record IndexRecord) IndexRecord {
	var object map[string]any
	_ = json.Unmarshal(record.Record, &object)
	if record.Title == "" || record.URL == "" || len(record.Links) == 0 {
		record = applyStandardIndexFields(record, object)
	}
	record.Record = cleanIndexRecordRaw(object)
	record.Origin = IndexOrigin{
		Source:   "host_index",
		Plugin:   snapshot.Plugin,
		Instance: snapshot.Instance,
		Index:    snapshot.Index,
	}
	return record
}

func cleanIndexRecordRaw(object map[string]any) json.RawMessage {
	if object == nil {
		return nil
	}
	clean := map[string]any{}
	for key, value := range object {
		switch key {
		case "entity", "id", "title", "url", "links", "origin", "score", "matched_fields":
			continue
		default:
			clean[key] = value
		}
	}
	raw, _ := json.Marshal(clean)
	return raw
}

func applyStandardIndexFields(record IndexRecord, object map[string]any) IndexRecord {
	if record.Title == "" {
		record.Title = firstRecordString(object, "title", "name", "name_with_namespace")
	}
	if record.URL == "" {
		record.URL = firstRecordString(object, "url", "web_url", "html_url")
	}
	calculatedLinks := indexRecordLinks(record, object)
	if len(record.Links) == 0 {
		record.Links = calculatedLinks
	} else {
		for key, value := range calculatedLinks {
			if _, ok := record.Links[key]; !ok {
				record.Links[key] = value
			}
		}
	}
	return record
}

func indexRecordLinks(record IndexRecord, object map[string]any) map[string]string {
	links := map[string]string{}
	if record.URL != "" {
		links["self"] = record.URL
	}
	if namespaceURL := namespaceLink(record, object); namespaceURL != "" {
		links["namespace"] = namespaceURL
	}
	for key, value := range relationshipLinks(record, object) {
		links[key] = value
	}
	if len(links) == 0 {
		return nil
	}
	return links
}

func namespaceLink(record IndexRecord, object map[string]any) string {
	if record.Entity != "gitlab.project" || record.URL == "" {
		return ""
	}
	path := firstRecordString(object, "path_with_namespace")
	if path == "" || !strings.Contains(path, "/") || !strings.HasSuffix(record.URL, path) {
		return ""
	}
	namespace := path[:strings.LastIndex(path, "/")]
	base := strings.TrimSuffix(record.URL, path)
	return strings.TrimRight(base, "/") + "/" + namespace
}

func relationshipLinks(record IndexRecord, object map[string]any) map[string]string {
	links := map[string]string{}
	switch record.Entity {
	case "gitlab.project":
		path := firstRecordString(object, "path_with_namespace")
		if namespace := namespaceFromProjectPath(path); namespace != "" {
			links["namespace_entity"] = entityRef("gitlab.group", namespace)
		}
	case "gitlab.issue", "gitlab.merge_request":
		if project := projectFromWorkItemRecord(record, object); project != "" {
			links["project_entity"] = entityRef("gitlab.project", project)
			if projectURL := projectURLFromWorkItemURL(record.URL); projectURL != "" {
				links["project"] = projectURL
			}
			if namespace := namespaceFromProjectPath(project); namespace != "" {
				links["namespace_entity"] = entityRef("gitlab.group", namespace)
			}
		}
		if author := firstRecordString(object, "author_username"); author != "" {
			links["author_entity"] = entityRef("gitlab.user", author)
		}
	case "slack.channel":
		if user := firstRecordString(object, "user"); user != "" {
			links["user_entity"] = entityRef("slack.user", user)
		}
	}
	return links
}

func namespaceFromProjectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.Contains(path, "/") {
		return ""
	}
	return path[:strings.LastIndex(path, "/")]
}

func projectFromWorkItemRecord(record IndexRecord, object map[string]any) string {
	ref := firstRecordString(object, "reference")
	if ref == "" {
		ref = record.ID
	}
	for _, sep := range []string{"!", "#"} {
		if project, _, ok := strings.Cut(ref, sep); ok {
			return strings.TrimSpace(project)
		}
	}
	return ""
}

func projectURLFromWorkItemURL(url string) string {
	for _, marker := range []string{"/-/merge_requests/", "/-/issues/", "/-/work_items/"} {
		if idx := strings.Index(url, marker); idx > 0 {
			return url[:idx]
		}
	}
	return ""
}

func entityRef(entity, id string) string {
	return entity + ":" + id
}

func firstRecordString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := object[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return strconv.FormatInt(int64(value), 10)
		}
	}
	return ""
}

func indexRecordScore(record IndexRecord, query string) (int, []string) {
	if query == "" {
		return 1, nil
	}
	var score int
	var fields []string
	add := func(field, value string, exact, prefix, contains int) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		switch {
		case value == query:
			if exact > score {
				score = exact
			}
			fields = appendMatchedField(fields, field)
		case strings.HasPrefix(value, query):
			if prefix > score {
				score = prefix
			}
			fields = appendMatchedField(fields, field)
		case strings.Contains(value, query):
			if contains > score {
				score = contains
			}
			fields = appendMatchedField(fields, field)
		}
	}
	add("id", record.ID, 1000, 850, 650)
	add("title", record.Title, 950, 800, 600)
	add("entity", record.Entity, 500, 400, 300)
	for key, value := range record.Links {
		add("links."+key, value, 450, 350, 250)
	}
	var object map[string]any
	_ = json.Unmarshal(record.Record, &object)
	for _, key := range []string{"username", "name", "display_name", "real_name", "name_with_namespace", "path_with_namespace", "full_name", "full_path", "reference", "author_username", "web_url", "email", "state", "user_id", "channel_id"} {
		add("record."+key, firstRecordString(object, key), 900, 750, 550)
	}
	if strings.Contains(strings.ToLower(string(record.Record)), query) {
		if score < 100 {
			score = 100
		}
		fields = appendMatchedField(fields, "record")
	}
	if score == 0 {
		termScore, termFields := indexRecordTokenScore(record, query)
		if termScore > 0 {
			score = termScore
			fields = appendMatchedFieldValues(fields, termFields...)
		}
	}
	return score, fields
}

func indexRecordTokenScore(record IndexRecord, query string) (int, []string) {
	terms := searchTerms(query)
	if len(terms) < 2 {
		return 0, nil
	}
	score := 0
	var fields []string
	for _, term := range terms {
		termScore, termFields := indexRecordScore(record, term)
		if termScore == 0 {
			return 0, nil
		}
		score += termScore
		fields = appendMatchedFieldValues(fields, termFields...)
	}
	return score / len(terms), fields
}

func searchTerms(query string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, token := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		token = strings.Trim(token, " \t\n\r\"'()[]{}<>.,;:#!")
		if len(token) < 2 || lookupStopword(token) || seen[token] {
			continue
		}
		seen[token] = true
		terms = append(terms, token)
	}
	return terms
}

func lookupTerms(options LookupOptions) []string {
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
	for _, term := range options.Terms {
		add(term)
	}
	text := strings.TrimSpace(options.Text)
	if text != "" {
		add(text)
		hasURL := strings.Contains(text, "://")
		for _, token := range strings.Fields(text) {
			token = strings.Trim(token, " \t\n\r\"'()[]{}<>.,;:#!")
			if hasURL && !strings.Contains(token, "://") {
				continue
			}
			if len(token) >= 3 && !lookupStopword(token) {
				add(token)
			}
		}
	}
	return terms
}

func lookupStopword(token string) bool {
	switch token {
	case "look", "lookup", "find", "open", "see", "the", "for", "from", "this", "that", "please", "at", "in", "to", "and", "with":
		return true
	default:
		return false
	}
}

func lookupRecordScore(record IndexRecord, text string, terms []string) (int, []string) {
	score := 0
	var fields []string
	add := func(field string, value string, valueScore int) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if text != "" && containsLookupValue(text, value) {
			if valueScore > score {
				score = valueScore
			}
			fields = appendMatchedField(fields, field)
			return
		}
		for _, term := range terms {
			if term == "" {
				continue
			}
			switch {
			case value == term:
				if valueScore-50 > score {
					score = valueScore - 50
				}
				fields = appendMatchedField(fields, field)
			case strings.Contains(value, term):
				if valueScore-250 > score {
					score = valueScore - 250
				}
				fields = appendMatchedField(fields, field)
			}
		}
	}
	add("id", record.ID, 1200)
	add("url", record.URL, 1150)
	add("title", record.Title, 900)
	add("entity", record.Entity, 500)
	for key, value := range record.Links {
		add("links."+key, value, 1050)
	}
	var object map[string]any
	_ = json.Unmarshal(record.Record, &object)
	for _, key := range []string{"username", "name", "display_name", "name_with_namespace", "path_with_namespace", "full_name", "full_path", "reference", "author_username", "web_url", "email", "state", "user_id", "channel_id"} {
		add("record."+key, firstRecordString(object, key), 950)
	}
	return score, fields
}

func containsLookupValue(text, value string) bool {
	start := 0
	for {
		idx := strings.Index(text[start:], value)
		if idx < 0 {
			return false
		}
		idx += start
		before := idx - 1
		after := idx + len(value)
		if lookupBoundary(text, before) && lookupBoundary(text, after) {
			return true
		}
		start = idx + 1
	}
}

func lookupBoundary(text string, idx int) bool {
	if idx < 0 || idx >= len(text) {
		return true
	}
	ch := text[idx]
	return !(ch >= 'a' && ch <= 'z') &&
		!(ch >= '0' && ch <= '9') &&
		ch != '/' &&
		ch != '-' &&
		ch != '_' &&
		ch != '.' &&
		ch != '!' &&
		ch != '#'
}

func appendMatchedField(fields []string, field string) []string {
	for _, existing := range fields {
		if existing == field {
			return fields
		}
	}
	return append(fields, field)
}

func appendMatchedFieldValues(fields []string, candidates ...string) []string {
	for _, candidate := range candidates {
		fields = appendMatchedField(fields, candidate)
	}
	return fields
}
