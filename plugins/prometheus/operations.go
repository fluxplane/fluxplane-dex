package prometheus

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
}

func NewService() Service {
	return Service{}
}

type PrometheusTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Prometheus endpoint ref resolved by the host."`
}

type TestInput struct {
	PrometheusTargetInput
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type QueryInput struct {
	PrometheusTargetInput
	Query string `json:"query,omitempty" jsonschema:"required,description=PromQL query"`
	Time  string `json:"time,omitempty" jsonschema:"description=RFC3339 or unix timestamp"`
}

type QueryResult struct {
	URL        string          `json:"url"`
	Query      string          `json:"query"`
	ResultType string          `json:"result_type"`
	Results    json.RawMessage `json:"results"`
}

type QueryRangeInput struct {
	PrometheusTargetInput
	Query string `json:"query,omitempty" jsonschema:"required,description=PromQL query"`
	Start string `json:"start,omitempty" jsonschema:"description=RFC3339, unix timestamp, or duration ago"`
	End   string `json:"end,omitempty" jsonschema:"description=RFC3339, unix timestamp, or duration ago"`
	Step  string `json:"step,omitempty" jsonschema:"description=Range step duration"`
}

type QueryRangeResult = QueryResult

type LabelsInput struct {
	PrometheusTargetInput
	Label string   `json:"label,omitempty" jsonschema:"description=Optional label name. When set, returns values for that label."`
	Match []string `json:"match,omitempty" jsonschema:"description=Optional PromQL match selectors."`
}

type LabelsResult struct {
	URL    string   `json:"url"`
	Label  string   `json:"label,omitempty"`
	Values []string `json:"values"`
}

type TargetsInput struct {
	PrometheusTargetInput
	State string `json:"state,omitempty" jsonschema:"description=active, dropped, or any"`
}

type TargetsResult struct {
	URL     string          `json:"url"`
	State   string          `json:"state,omitempty"`
	Targets json.RawMessage `json:"targets"`
}

type AlertsResult struct {
	URL    string          `json:"url"`
	Alerts json.RawMessage `json:"alerts"`
}

type QueryRecord struct {
	pluginbinding.DatasourceRecord
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Query       string         `json:"query,omitempty" datasource:"completion,view=compact|lookup|table"`
	ResultType  string         `json:"result_type,omitempty" datasource:"completion,view=compact|lookup|table"`
	Result      map[string]any `json:"result,omitempty" datasource:"view=lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type LabelRecord struct {
	pluginbinding.DatasourceRecord
	Name        string `json:"name" datasource:"id,completion,view=compact|lookup|table"`
	Label       string `json:"label,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type TargetRecord struct {
	pluginbinding.DatasourceRecord
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	State       string         `json:"state,omitempty" datasource:"completion,view=compact|lookup|table"`
	Job         string         `json:"job,omitempty" datasource:"completion,view=compact|lookup|table"`
	Endpoint    string         `json:"endpoint,omitempty" datasource:"completion,view=compact|lookup|table"`
	Target      map[string]any `json:"target,omitempty" datasource:"view=lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type AlertRecord struct {
	pluginbinding.DatasourceRecord
	Title       string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Name        string            `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	State       string            `json:"state,omitempty" datasource:"completion,view=compact|lookup|table"`
	Severity    string            `json:"severity,omitempty" datasource:"completion,view=compact|lookup|table"`
	Labels      map[string]string `json:"labels,omitempty" datasource:"view=lookup|table"`
	Annotations map[string]string `json:"annotations,omitempty" datasource:"view=lookup|table"`
	EndpointURL string            `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type QueryDatasourceResult = pluginbinding.DatasourceSearchResult[QueryRecord]
type LabelDatasourceResult = pluginbinding.DatasourceSearchResult[LabelRecord]
type TargetDatasourceResult = pluginbinding.DatasourceSearchResult[TargetRecord]
type AlertDatasourceResult = pluginbinding.DatasourceSearchResult[AlertRecord]

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
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
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{"query": {query}}
	if strings.TrimSpace(input.Time) != "" {
		t, err := parseTimeValue(input.Time, time.Now())
		if err != nil {
			return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("time", strconv.FormatInt(t.Unix(), 10))
	}
	return s.query(context.Background(), target, client, query, "/api/v1/query", values)
}

func (s Service) QueryRange(ctx pluginbinding.Context, input QueryRangeInput) (QueryRangeResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryRangeResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(input.End, "0s"), now)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start, err := parseTimeValue(firstNonEmpty(input.Start, "1h"), now)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	step := strings.TrimSpace(input.Step)
	if step == "" {
		step = "1m"
	}
	values := url.Values{"query": {query}}
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", step)
	return s.query(context.Background(), target, client, query, "/api/v1/query_range", values)
}

func (s Service) Labels(ctx pluginbinding.Context, input LabelsInput) (LabelsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	for _, match := range input.Match {
		if strings.TrimSpace(match) != "" {
			values.Add("match[]", strings.TrimSpace(match))
		}
	}
	path := "/api/v1/labels"
	label := strings.TrimSpace(input.Label)
	if label != "" {
		path = "/api/v1/label/" + url.PathEscape(label) + "/values"
	}
	data, err := client.get(context.Background(), path, values)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	var valuesOut []string
	if err := json.Unmarshal(data, &valuesOut); err != nil {
		return LabelsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return LabelsResult{URL: target, Label: label, Values: valuesOut}, nil
}

func (s Service) Targets(ctx pluginbinding.Context, input TargetsInput) (TargetsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	if state := strings.TrimSpace(input.State); state != "" {
		values.Set("state", state)
	}
	data, err := client.get(context.Background(), "/api/v1/targets", values)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return TargetsResult{URL: target, State: input.State, Targets: data}, nil
}

func (s Service) Alerts(ctx pluginbinding.Context, input TestInput) (AlertsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	data, err := client.get(context.Background(), "/api/v1/alerts", nil)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return AlertsResult{URL: target, Alerts: data}, nil
}

func (s Service) QueryDatasource(ctx pluginbinding.Context, input QueryInput) (QueryDatasourceResult, error) {
	out, err := s.Query(ctx, input)
	if err != nil {
		return QueryDatasourceResult{}, err
	}
	var raw []map[string]any
	_ = json.Unmarshal(out.Results, &raw)
	if len(raw) == 0 && len(out.Results) > 0 {
		var one map[string]any
		if err := json.Unmarshal(out.Results, &one); err == nil && len(one) > 0 {
			raw = append(raw, one)
		}
	}
	records := make([]QueryRecord, 0, len(raw))
	for i, result := range raw {
		id := stableID(out.URL, out.Query, strconv.Itoa(i), result)
		title := firstNonEmpty(metricTitle(result), fmt.Sprintf("result %d", i+1))
		record := QueryRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityQueryResult, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"query": out.Query, "result_type": out.ResultType, "result": result, "endpoint_url": out.URL})),
			Title:            title,
			Query:            out.Query,
			ResultType:       out.ResultType,
			Result:           result,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
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
	return pluginbinding.NewDatasourceSearchResult("live", firstNonEmpty(input.Label, strings.Join(input.Match, " ")), records), nil
}

func (s Service) TargetsDatasource(ctx pluginbinding.Context, input TargetsInput) (TargetDatasourceResult, error) {
	out, err := s.Targets(ctx, input)
	if err != nil {
		return TargetDatasourceResult{}, err
	}
	targets := targetObjects(out.Targets)
	records := make([]TargetRecord, 0, len(targets))
	for i, target := range targets {
		job := stringFromNested(target, "labels", "job")
		endpoint := firstNonEmpty(stringFromNested(target, "labels", "instance"), stringFromNested(target, "discoveredLabels", "__address__"), stringFromNested(target, "scrapePool"))
		state := firstNonEmpty(stringFromAny(target["health"]), stringFromAny(target["state"]), out.State)
		title := firstNonEmpty(job, endpoint, fmt.Sprintf("target %d", i+1))
		id := stableID(out.URL, title, strconv.Itoa(i), target)
		record := TargetRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityTarget, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"state": state, "job": job, "endpoint": endpoint, "target": target, "endpoint_url": out.URL})),
			Title:            title,
			State:            state,
			Job:              job,
			Endpoint:         endpoint,
			Target:           target,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.State, records), nil
}

func (s Service) AlertsDatasource(ctx pluginbinding.Context, input TestInput) (AlertDatasourceResult, error) {
	out, err := s.Alerts(ctx, input)
	if err != nil {
		return AlertDatasourceResult{}, err
	}
	alerts := alertObjects(out.Alerts)
	records := make([]AlertRecord, 0, len(alerts))
	for i, alert := range alerts {
		labels := stringMapFromAny(alert["labels"])
		annotations := stringMapFromAny(alert["annotations"])
		name := labels["alertname"]
		state := stringFromAny(alert["state"])
		severity := labels["severity"]
		title := firstNonEmpty(name, annotations["summary"], fmt.Sprintf("alert %d", i+1))
		id := stableID(out.URL, title, strconv.Itoa(i), alert)
		record := AlertRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityAlert, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"name": name, "state": state, "severity": severity, "labels": labels, "annotations": annotations, "endpoint_url": out.URL})),
			Title:            title,
			Name:             name,
			State:            state,
			Severity:         severity,
			Labels:           labels,
			Annotations:      annotations,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", "", records), nil
}

func (s Service) query(ctx context.Context, target string, client Client, query, path string, values url.Values) (QueryResult, error) {
	data, err := client.get(ctx, path, values)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	var wrapped struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return QueryResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return QueryResult{URL: target, Query: query, ResultType: wrapped.ResultType, Results: wrapped.Result}, nil
}

func (s Service) client(ctx pluginbinding.Context, input PrometheusTargetInput) (string, Client, error) {
	_ = s
	endpointRef := strings.TrimSpace(input.EndpointRef)
	if endpointRef == "" {
		return "", Client{}, fmt.Errorf("endpoint_ref is required")
	}
	return endpointRef, Client{EndpointRef: endpointRef, Host: ctx.Host}, nil
}

func parseTimeValue(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("time value is empty")
	}
	if d, err := time.ParseDuration(value); err == nil {
		return now.Add(-d), nil
	}
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func targetObjects(raw json.RawMessage) []map[string]any {
	var wrapped struct {
		ActiveTargets  []map[string]any `json:"activeTargets"`
		DroppedTargets []map[string]any `json:"droppedTargets"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		out := append([]map[string]any{}, wrapped.ActiveTargets...)
		out = append(out, wrapped.DroppedTargets...)
		if len(out) > 0 {
			return out
		}
	}
	var direct []map[string]any
	_ = json.Unmarshal(raw, &direct)
	return direct
}

func alertObjects(raw json.RawMessage) []map[string]any {
	var wrapped struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Alerts) > 0 {
		return wrapped.Alerts
	}
	var direct []map[string]any
	_ = json.Unmarshal(raw, &direct)
	return direct
}

func metricTitle(result map[string]any) string {
	metric, _ := result["metric"].(map[string]any)
	if len(metric) == 0 {
		return ""
	}
	var parts []string
	for key, value := range metric {
		if text := stringFromAny(value); text != "" {
			parts = append(parts, key+"="+text)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func stableID(parts ...any) string {
	data, _ := json.Marshal(parts)
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func stringFromNested(object map[string]any, path ...string) string {
	var current any = object
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = next[key]
	}
	return stringFromAny(current)
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func stringMapFromAny(value any) map[string]string {
	out := map[string]string{}
	raw, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for key, value := range raw {
		if text := stringFromAny(value); text != "" {
			out[key] = text
		}
	}
	return out
}
