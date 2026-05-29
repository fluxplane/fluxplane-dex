package loki

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	defaultLokiLimit = 100
	maxLokiLimit     = 1000
)

var lokiLabelNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Service struct {
}

func NewService() Service {
	return Service{}
}

type LokiTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered Loki endpoint ref resolved by the host."`
}

type TestInput struct {
	LokiTargetInput
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type QueryInput struct {
	LokiTargetInput
	Query     string `json:"query,omitempty" jsonschema:"required,description=LogQL stream query for Loki query_range."`
	Since     string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339 unix seconds or duration ago. Defaults to 1h."`
	Until     string `json:"until,omitempty" jsonschema:"description=End time as RFC3339 unix seconds or duration ago. Defaults to now."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum log entries to return. Defaults to 100 and is capped at 1000.,minimum=0,maximum=1000"`
	Direction string `json:"direction,omitempty" jsonschema:"description=Loki query direction. Defaults to backward.,enum=backward,enum=forward"`
}

type LogEntry struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
	Line      string            `json:"line"`
}

type QueryResult struct {
	URL             string     `json:"url"`
	NormalizedQuery string     `json:"normalized_query"`
	Entries         []LogEntry `json:"entries"`
	Count           int        `json:"count"`
	Limit           int        `json:"limit"`
}

type LabelsInput struct {
	LokiTargetInput
	Label string `json:"label,omitempty" jsonschema:"description=Optional Loki label name. When omitted label names are returned."`
	Query string `json:"query,omitempty" jsonschema:"description=Optional LogQL stream selector used to filter label names or values."`
}

type LabelsResult struct {
	URL    string   `json:"url"`
	Label  string   `json:"label,omitempty"`
	Values []string `json:"values"`
}

type RecentLogsInput struct {
	LokiTargetInput
	App       string `json:"app,omitempty" jsonschema:"description=Exact app label filter."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Exact namespace label filter."`
	Pod       string `json:"pod,omitempty" jsonschema:"description=Exact pod label filter."`
	Container string `json:"container,omitempty" jsonschema:"description=Exact container label filter."`
	Contains  string `json:"contains,omitempty" jsonschema:"description=Line substring filter."`
	Since     string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339 unix seconds or duration ago. Defaults to 1h."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum log entries to return. Defaults to 100 and is capped at 1000.,minimum=0,maximum=1000"`
}

type LogEntriesInput struct {
	LokiTargetInput
	Query     string `json:"query,omitempty" jsonschema:"description=LogQL stream query. Plain text is used as a contains filter for recent logs."`
	App       string `json:"app,omitempty" jsonschema:"description=Exact app label filter used when query is plain text or empty."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Exact namespace label filter used when query is plain text or empty."`
	Pod       string `json:"pod,omitempty" jsonschema:"description=Exact pod label filter used when query is plain text or empty."`
	Container string `json:"container,omitempty" jsonschema:"description=Exact container label filter used when query is plain text or empty."`
	Contains  string `json:"contains,omitempty" jsonschema:"description=Line substring filter used when query is plain text or empty."`
	Since     string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339 unix seconds or duration ago. Defaults to 1h."`
	Until     string `json:"until,omitempty" jsonschema:"description=End time as RFC3339 unix seconds or duration ago. Defaults to now for LogQL queries."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum log entries to return. Defaults to 100 and is capped at 1000.,minimum=0,maximum=1000"`
	Direction string `json:"direction,omitempty" jsonschema:"description=Loki query direction for LogQL queries. Defaults to backward.,enum=backward,enum=forward"`
}

type LogEntryRecord struct {
	pluginbinding.DatasourceRecord
	Title       string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Timestamp   string            `json:"timestamp" datasource:"view=compact|lookup|table"`
	Labels      map[string]string `json:"labels,omitempty" datasource:"view=lookup|table"`
	Line        string            `json:"line" datasource:"completion,view=compact|lookup|table"`
	App         string            `json:"app,omitempty" datasource:"completion,view=compact|lookup|table"`
	Namespace   string            `json:"namespace,omitempty" datasource:"completion,view=compact|lookup|table"`
	Pod         string            `json:"pod,omitempty" datasource:"completion,view=compact|lookup|table"`
	Container   string            `json:"container,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string            `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type LabelRecord struct {
	pluginbinding.DatasourceRecord
	Name        string `json:"name" datasource:"id,completion,view=compact|lookup|table"`
	Label       string `json:"label,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type LogEntriesDatasourceResult = pluginbinding.DatasourceSearchResult[LogEntryRecord]
type LabelDatasourceResult = pluginbinding.DatasourceSearchResult[LabelRecord]

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	target, client, err := s.client(ctx, input.LokiTargetInput)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	err = client.ready(context.Background())
	out := TestResult{URL: target, Ready: err == nil, LatencyMS: time.Since(start).Milliseconds()}
	if err != nil {
		out.Error = err.Error()
	}
	return out, nil
}

func (s Service) Query(ctx pluginbinding.Context, input QueryInput) (QueryResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, err := s.client(ctx, input.LokiTargetInput)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return s.query(context.Background(), target, client, query, input.Since, input.Until, input.Limit, input.Direction)
}

func (s Service) RecentLogs(ctx pluginbinding.Context, input RecentLogsInput) (QueryResult, error) {
	query := recentLogsQuery(input)
	target, client, err := s.client(ctx, input.LokiTargetInput)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return s.query(context.Background(), target, client, query, input.Since, "", input.Limit, "backward")
}

func (s Service) Labels(ctx pluginbinding.Context, input LabelsInput) (LabelsResult, error) {
	target, client, err := s.client(ctx, input.LokiTargetInput)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	label := strings.TrimSpace(input.Label)
	if label != "" && !lokiLabelNamePattern.MatchString(label) {
		return LabelsResult{}, pluginbinding.Fail("bad_input", "label must be a valid Loki label name")
	}
	path := "/loki/api/v1/labels"
	if label != "" {
		path = "/loki/api/v1/label/" + url.PathEscape(label) + "/values"
	}
	values := url.Values{}
	if strings.TrimSpace(input.Query) != "" {
		values.Set("query", input.Query)
	}
	var response struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := client.get(context.Background(), path, values, &response); err != nil {
		return LabelsResult{}, pluginbinding.Errorf("loki", "%s", err)
	}
	if response.Status != "success" {
		return LabelsResult{}, pluginbinding.Errorf("loki", "label query failed with status %s", response.Status)
	}
	sort.Strings(response.Data)
	return LabelsResult{URL: target, Label: label, Values: response.Data}, nil
}

func (s Service) LogEntriesDatasource(ctx pluginbinding.Context, input LogEntriesInput) (LogEntriesDatasourceResult, error) {
	query := strings.TrimSpace(input.Query)
	limit := clampLokiLimit(input.Limit)
	var out QueryResult
	var err error
	if strings.HasPrefix(query, "{") || strings.HasPrefix(query, "(") {
		out, err = s.Query(ctx, QueryInput{LokiTargetInput: input.LokiTargetInput, Query: query, Since: input.Since, Until: input.Until, Limit: input.Limit, Direction: input.Direction})
	} else {
		contains := firstNonEmpty(input.Contains, query)
		out, err = s.RecentLogs(ctx, RecentLogsInput{LokiTargetInput: input.LokiTargetInput, App: input.App, Namespace: input.Namespace, Pod: input.Pod, Container: input.Container, Contains: contains, Since: input.Since, Limit: limit})
	}
	if err != nil {
		return LogEntriesDatasourceResult{}, err
	}
	records := make([]LogEntryRecord, 0, len(out.Entries))
	for _, entry := range out.Entries {
		title := firstNonEmpty(entry.Labels["pod"], entry.Labels["app"], entry.Line)
		record := LogEntryRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityLogEntry, entry.ID, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"timestamp": entry.Timestamp, "labels": entry.Labels, "line": entry.Line, "query": out.NormalizedQuery, "endpoint_url": out.URL})),
			Title:            title,
			Timestamp:        entry.Timestamp,
			Labels:           entry.Labels,
			Line:             entry.Line,
			App:              entry.Labels["app"],
			Namespace:        entry.Labels["namespace"],
			Pod:              entry.Labels["pod"],
			Container:        entry.Labels["container"],
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", firstNonEmpty(input.Query, input.Contains), records), nil
}

func (s Service) LabelsDatasource(ctx pluginbinding.Context, input LabelsInput) (LabelDatasourceResult, error) {
	out, err := s.Labels(ctx, input)
	if err != nil {
		return LabelDatasourceResult{}, err
	}
	records := make([]LabelRecord, 0, len(out.Values))
	for _, value := range out.Values {
		id := value
		if out.Label != "" {
			id = out.Label + "=" + value
		}
		record := LabelRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityLabel, id, pluginbinding.RecordTitle(id), pluginbinding.RecordMetadata(map[string]any{"name": value, "label": out.Label, "endpoint_url": out.URL})),
			Name:             value,
			Label:            out.Label,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", firstNonEmpty(input.Label, input.Query), records), nil
}

func (s Service) query(ctx context.Context, target string, client Client, query, since, until string, limit int, direction string) (QueryResult, error) {
	limit = clampLokiLimit(limit)
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(until, "0s"), now)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start, err := parseTimeValue(firstNonEmpty(since, "1h"), now)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	direction = normalizeLokiDirection(direction)
	if direction == "" {
		return QueryResult{}, pluginbinding.Fail("bad_input", "direction must be backward or forward")
	}
	values := url.Values{}
	values.Set("query", query)
	values.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	values.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	values.Set("limit", strconv.Itoa(limit))
	values.Set("direction", direction)
	var response lokiResponse
	if err := client.get(ctx, "/loki/api/v1/query_range", values, &response); err != nil {
		return QueryResult{}, pluginbinding.Errorf("loki", "%s", err)
	}
	if response.Status != "success" {
		return QueryResult{}, pluginbinding.Errorf("loki", "query failed with status %s", response.Status)
	}
	var entries []LogEntry
	for _, stream := range response.Data.Result {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}
			ts := parseLogTimestamp(value[0])
			id := logEntryID(stream.Stream, value[0], value[1])
			entries = append(entries, LogEntry{ID: id, Timestamp: ts.Format(time.RFC3339Nano), Labels: stream.Stream, Line: value[1]})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if direction == "forward" {
			return entries[i].Timestamp < entries[j].Timestamp
		}
		return entries[i].Timestamp > entries[j].Timestamp
	})
	return QueryResult{URL: target, NormalizedQuery: query, Entries: entries, Count: len(entries), Limit: limit}, nil
}

func (s Service) client(ctx pluginbinding.Context, input LokiTargetInput) (string, Client, error) {
	_ = s
	endpointRef := strings.TrimSpace(input.EndpointRef)
	if endpointRef == "" {
		return "", Client{}, fmt.Errorf("endpoint_ref is required")
	}
	return endpointRef, Client{EndpointRef: endpointRef, Host: ctx.Host}, nil
}

func recentLogsQuery(input RecentLogsInput) string {
	labels := map[string]string{}
	if input.App != "" {
		labels["app"] = input.App
	}
	if input.Namespace != "" {
		labels["namespace"] = input.Namespace
	}
	if input.Pod != "" {
		labels["pod"] = input.Pod
	}
	if input.Container != "" {
		labels["container"] = input.Container
	}
	var parts []string
	for key, value := range labels {
		parts = append(parts, key+"="+quoteLogQLString(value))
	}
	sort.Strings(parts)
	query := "{" + strings.Join(parts, ",") + "}"
	if len(parts) == 0 {
		query = `{job=~".+"}`
	}
	if strings.TrimSpace(input.Contains) != "" {
		query += ` |= ` + quoteLogQLString(input.Contains)
	}
	return query
}

func clampLokiLimit(limit int) int {
	if limit <= 0 {
		return defaultLokiLimit
	}
	if limit > maxLokiLimit {
		return maxLokiLimit
	}
	return limit
}

func normalizeLokiDirection(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "", "backward":
		return "backward"
	case "forward":
		return "forward"
	default:
		return ""
	}
}

func quoteLogQLString(value string) string {
	return strconv.Quote(value)
}

func parseTimeValue(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if d, err := time.ParseDuration(value); err == nil {
		return now.Add(-d), nil
	}
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func logEntryID(labels map[string]string, ts, line string) string {
	data, _ := json.Marshal(labels)
	sum := sha1.Sum([]byte(string(data) + "\x00" + ts + "\x00" + line))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
