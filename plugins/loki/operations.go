package loki

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	HTTPClient *http.Client
	DefaultURL string
}

func NewService() Service {
	return Service{HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

type TestInput struct {
	URL      string `json:"url,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type QueryInput struct {
	URL       string `json:"url,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	Query     string `json:"query,omitempty" jsonschema:"required,description=LogQL query"`
	Since     string `json:"since,omitempty"`
	Until     string `json:"until,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Direction string `json:"direction,omitempty"`
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
	URL      string `json:"url,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Label    string `json:"label,omitempty"`
	Query    string `json:"query,omitempty"`
}

type LabelsResult struct {
	URL    string   `json:"url"`
	Label  string   `json:"label,omitempty"`
	Values []string `json:"values"`
}

type RecentLogsInput struct {
	URL       string `json:"url,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	App       string `json:"app,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
	Contains  string `json:"contains,omitempty"`
	Since     string `json:"since,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type LogEntriesInput struct {
	URL       string `json:"url,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	Query     string `json:"query,omitempty" jsonschema:"description=LogQL query. Plain text is used as a contains filter for recent logs."`
	App       string `json:"app,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
	Contains  string `json:"contains,omitempty"`
	Since     string `json:"since,omitempty"`
	Until     string `json:"until,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Direction string `json:"direction,omitempty"`
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
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	err = s.client(target, input.TenantID).ready(context.Background())
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
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return s.query(context.Background(), target, input.TenantID, query, input.Since, input.Until, input.Limit, input.Direction)
}

func (s Service) RecentLogs(ctx pluginbinding.Context, input RecentLogsInput) (QueryResult, error) {
	query := recentLogsQuery(input)
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return s.query(context.Background(), target, input.TenantID, query, input.Since, "", input.Limit, "backward")
}

func (s Service) Labels(ctx pluginbinding.Context, input LabelsInput) (LabelsResult, error) {
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	path := "/loki/api/v1/labels"
	label := strings.TrimSpace(input.Label)
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
	if err := s.client(target, input.TenantID).get(context.Background(), path, values, &response); err != nil {
		return LabelsResult{}, pluginbinding.Errorf("loki", "%s", err)
	}
	sort.Strings(response.Data)
	return LabelsResult{URL: target, Label: label, Values: response.Data}, nil
}

func (s Service) LogEntriesDatasource(ctx pluginbinding.Context, input LogEntriesInput) (LogEntriesDatasourceResult, error) {
	query := strings.TrimSpace(input.Query)
	var out QueryResult
	var err error
	if strings.HasPrefix(query, "{") || strings.HasPrefix(query, "(") {
		out, err = s.Query(ctx, QueryInput{URL: input.URL, TenantID: input.TenantID, Query: query, Since: input.Since, Until: input.Until, Limit: input.Limit, Direction: input.Direction})
	} else {
		contains := firstNonEmpty(input.Contains, query)
		out, err = s.RecentLogs(ctx, RecentLogsInput{URL: input.URL, TenantID: input.TenantID, App: input.App, Namespace: input.Namespace, Pod: input.Pod, Container: input.Container, Contains: contains, Since: input.Since, Limit: input.Limit})
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

func (s Service) query(ctx context.Context, target, tenantID, query, since, until string, limit int, direction string) (QueryResult, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(until, "0s"), now)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start, err := parseTimeValue(firstNonEmpty(since, "1h"), now)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(direction) == "" {
		direction = "backward"
	}
	values := url.Values{}
	values.Set("query", query)
	values.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	values.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	values.Set("limit", strconv.Itoa(limit))
	values.Set("direction", direction)
	var response lokiResponse
	if err := s.client(target, tenantID).get(ctx, "/loki/api/v1/query_range", values, &response); err != nil {
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
	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp > entries[j].Timestamp })
	return QueryResult{URL: target, NormalizedQuery: query, Entries: entries, Count: len(entries), Limit: limit}, nil
}

func (s Service) client(target, tenantID string) Client {
	if strings.TrimSpace(tenantID) == "" {
		tenantID = strings.TrimSpace(os.Getenv(EnvLokiTenantID))
	}
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return Client{BaseURL: target, TenantID: tenantID, HTTPClient: client}
}

func (s Service) resolveURL(value string) (string, error) {
	target := strings.TrimSpace(value)
	if target == "" {
		target = strings.TrimSpace(s.DefaultURL)
	}
	if target == "" {
		target = strings.TrimSpace(os.Getenv(EnvLokiURL))
	}
	if target == "" {
		return "", fmt.Errorf("loki url is required")
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid loki url %q", target)
	}
	return strings.TrimRight(target, "/"), nil
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
		parts = append(parts, key+`="`+strings.ReplaceAll(value, `"`, `\"`)+`"`)
	}
	sort.Strings(parts)
	query := "{" + strings.Join(parts, ",") + "}"
	if len(parts) == 0 {
		query = `{job=~".+"}`
	}
	if strings.TrimSpace(input.Contains) != "" {
		query += ` |= "` + strings.ReplaceAll(input.Contains, `"`, `\"`) + `"`
	}
	return query
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
