package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	URL string `json:"url,omitempty" jsonschema:"description=Prometheus base URL. Defaults to PROMETHEUS_URL."`
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type QueryInput struct {
	URL   string `json:"url,omitempty"`
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
	URL   string `json:"url,omitempty"`
	Query string `json:"query,omitempty" jsonschema:"required,description=PromQL query"`
	Start string `json:"start,omitempty" jsonschema:"description=RFC3339, unix timestamp, or duration ago"`
	End   string `json:"end,omitempty" jsonschema:"description=RFC3339, unix timestamp, or duration ago"`
	Step  string `json:"step,omitempty" jsonschema:"description=Range step duration"`
}

type QueryRangeResult = QueryResult

type LabelsInput struct {
	URL   string   `json:"url,omitempty"`
	Label string   `json:"label,omitempty" jsonschema:"description=Optional label name. When set, returns values for that label."`
	Match []string `json:"match,omitempty" jsonschema:"description=Optional PromQL match selectors."`
}

type LabelsResult struct {
	URL    string   `json:"url"`
	Label  string   `json:"label,omitempty"`
	Values []string `json:"values"`
}

type TargetsInput struct {
	URL   string `json:"url,omitempty"`
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

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	err = s.client(target).ready(context.Background())
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
	values := url.Values{"query": {query}}
	if strings.TrimSpace(input.Time) != "" {
		t, err := parseTimeValue(input.Time, time.Now())
		if err != nil {
			return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("time", strconv.FormatInt(t.Unix(), 10))
	}
	return s.query(context.Background(), target, query, "/api/v1/query", values)
}

func (s Service) QueryRange(ctx pluginbinding.Context, input QueryRangeInput) (QueryRangeResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryRangeResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, err := s.resolveURL(input.URL)
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
	return s.query(context.Background(), target, query, "/api/v1/query_range", values)
}

func (s Service) Labels(ctx pluginbinding.Context, input LabelsInput) (LabelsResult, error) {
	target, err := s.resolveURL(input.URL)
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
	data, err := s.client(target).get(context.Background(), path, values)
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
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	if state := strings.TrimSpace(input.State); state != "" {
		values.Set("state", state)
	}
	data, err := s.client(target).get(context.Background(), "/api/v1/targets", values)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return TargetsResult{URL: target, State: input.State, Targets: data}, nil
}

func (s Service) Alerts(ctx pluginbinding.Context, input TestInput) (AlertsResult, error) {
	target, err := s.resolveURL(input.URL)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	data, err := s.client(target).get(context.Background(), "/api/v1/alerts", nil)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return AlertsResult{URL: target, Alerts: data}, nil
}

func (s Service) query(ctx context.Context, target, query, path string, values url.Values) (QueryResult, error) {
	data, err := s.client(target).get(ctx, path, values)
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

func (s Service) client(target string) Client {
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return Client{BaseURL: target, HTTPClient: client}
}

func (s Service) resolveURL(value string) (string, error) {
	target := strings.TrimSpace(value)
	if target == "" {
		target = strings.TrimSpace(s.DefaultURL)
	}
	if target == "" {
		target = strings.TrimSpace(os.Getenv(EnvPrometheusURL))
	}
	if target == "" {
		return "", fmt.Errorf("prometheus url is required")
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid prometheus url %q", target)
	}
	return strings.TrimRight(target, "/"), nil
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
