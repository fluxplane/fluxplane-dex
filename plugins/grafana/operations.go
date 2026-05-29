package grafana

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

type GrafanaTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Grafana endpoint ref resolved by the host."`
}

type DatasourceListInput struct {
	GrafanaTargetInput
}

type DatasourceHealthInput struct {
	GrafanaTargetInput
	UID string `json:"uid,omitempty" jsonschema:"required,description=Grafana datasource UID."`
}

type Datasource struct {
	ID        int               `json:"id,omitempty"`
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	URL       string            `json:"url,omitempty"`
	Access    string            `json:"access,omitempty"`
	IsDefault bool              `json:"is_default,omitempty"`
	JSONData  map[string]any    `json:"json_data,omitempty"`
	Cluster   string            `json:"cluster,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type DatasourceListResult struct {
	URL         string                       `json:"url"`
	Count       int                          `json:"count"`
	Datasources []Datasource                 `json:"datasources"`
	Clusters    map[string]map[string]string `json:"clusters,omitempty"`
	Types       map[string][]string          `json:"types,omitempty"`
}

type FolderListInput struct {
	GrafanaTargetInput
	Limit int `json:"limit,omitempty" jsonschema:"description=Maximum folders."`
}

type Folder struct {
	ID        int    `json:"id,omitempty"`
	UID       string `json:"uid"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`
}

type FolderListResult struct {
	URL     string   `json:"url"`
	Count   int      `json:"count"`
	Folders []Folder `json:"folders"`
}

type DashboardListInput struct {
	GrafanaTargetInput
	FolderUID string   `json:"folder_uid,omitempty" jsonschema:"description=Folder UID filter."`
	Query     string   `json:"query,omitempty" jsonschema:"description=Dashboard search query."`
	Tags      []string `json:"tags,omitempty" jsonschema:"description=Dashboard tag filters."`
	Limit     int      `json:"limit,omitempty" jsonschema:"description=Maximum dashboards."`
}

type DashboardSearchHit struct {
	ID          int      `json:"id,omitempty"`
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	Type        string   `json:"type,omitempty"`
	URI         string   `json:"uri,omitempty"`
	URL         string   `json:"url,omitempty"`
	FolderUID   string   `json:"folder_uid,omitempty"`
	FolderTitle string   `json:"folder_title,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type DashboardListResult struct {
	URL        string               `json:"url"`
	Count      int                  `json:"count"`
	Dashboards []DashboardSearchHit `json:"dashboards"`
}

func (h *DashboardSearchHit) UnmarshalJSON(data []byte) error {
	type alias DashboardSearchHit
	var raw struct {
		alias
		FolderUIDCamel   string `json:"folderUid"`
		FolderTitleCamel string `json:"folderTitle"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*h = DashboardSearchHit(raw.alias)
	if h.FolderUID == "" {
		h.FolderUID = raw.FolderUIDCamel
	}
	if h.FolderTitle == "" {
		h.FolderTitle = raw.FolderTitleCamel
	}
	return nil
}

type DashboardGetInput struct {
	GrafanaTargetInput
	UID string `json:"uid,omitempty" jsonschema:"required,description=Grafana dashboard UID."`
}

type DashboardPanel struct {
	ID         int              `json:"id,omitempty"`
	Title      string           `json:"title,omitempty"`
	Type       string           `json:"type,omitempty"`
	Datasource DashboardTarget  `json:"datasource,omitempty"`
	Targets    []DashboardQuery `json:"targets,omitempty"`
	PanelPath  []string         `json:"panel_path,omitempty"`
}

type DashboardQuery struct {
	PanelID    int             `json:"panel_id,omitempty"`
	PanelTitle string          `json:"panel_title,omitempty"`
	RefID      string          `json:"ref_id,omitempty"`
	Datasource DashboardTarget `json:"datasource,omitempty"`
	Expression string          `json:"expression,omitempty"`
	Query      string          `json:"query,omitempty"`
	QueryType  string          `json:"query_type,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

type DashboardTarget struct {
	Type string `json:"type,omitempty"`
	UID  string `json:"uid,omitempty"`
}

type DashboardGetResult struct {
	URL       string           `json:"url"`
	UID       string           `json:"uid"`
	Title     string           `json:"title,omitempty"`
	Panels    []DashboardPanel `json:"panels,omitempty"`
	Queries   []DashboardQuery `json:"queries,omitempty"`
	Dashboard json.RawMessage  `json:"dashboard,omitempty"`
}

type AnnotationListInput struct {
	GrafanaTargetInput
	Since        string   `json:"since,omitempty" jsonschema:"description=Start time as RFC3339, unix timestamp, or duration ago."`
	Until        string   `json:"until,omitempty" jsonschema:"description=End time as RFC3339, unix timestamp, or duration ago."`
	Tags         []string `json:"tags,omitempty" jsonschema:"description=Annotation tag filters."`
	DashboardUID string   `json:"dashboard_uid,omitempty" jsonschema:"description=Dashboard UID filter."`
	Limit        int      `json:"limit,omitempty" jsonschema:"description=Maximum annotations."`
}

type AnnotationAddInput struct {
	GrafanaTargetInput
	Time         string   `json:"time,omitempty" jsonschema:"description=Annotation time as RFC3339, unix timestamp, or duration ago. Defaults to now."`
	TimeEnd      string   `json:"time_end,omitempty" jsonschema:"description=Optional region end time."`
	Text         string   `json:"text,omitempty" jsonschema:"required,description=Annotation text."`
	Tags         []string `json:"tags,omitempty" jsonschema:"description=Annotation tags."`
	DashboardUID string   `json:"dashboard_uid,omitempty" jsonschema:"description=Optional dashboard UID."`
	PanelID      int      `json:"panel_id,omitempty" jsonschema:"description=Optional panel ID."`
}

type LokiLabelsInput struct {
	GrafanaTargetInput
	Cluster string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Label   string `json:"label,omitempty" jsonschema:"description=Optional label name. When set, returns values for that label."`
	Query   string `json:"query,omitempty" jsonschema:"description=Optional LogQL selector."`
}

type LabelsResult struct {
	URL     string   `json:"url"`
	UID     string   `json:"uid"`
	Cluster string   `json:"cluster,omitempty"`
	Label   string   `json:"label,omitempty"`
	Values  []string `json:"values"`
}

type LogEntry struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
	Line      string            `json:"line"`
}

type LokiQueryResult struct {
	URL             string          `json:"url"`
	UID             string          `json:"uid"`
	Cluster         string          `json:"cluster,omitempty"`
	NormalizedQuery string          `json:"normalized_query"`
	Entries         []LogEntry      `json:"entries"`
	Count           int             `json:"count"`
	Limit           int             `json:"limit"`
	Raw             json.RawMessage `json:"raw,omitempty"`
}

type LokiQueryInput struct {
	GrafanaTargetInput
	Cluster string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Query   string `json:"query,omitempty" jsonschema:"required,description=LogQL query."`
	Since   string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339, unix timestamp, or duration ago."`
	Until   string `json:"until,omitempty" jsonschema:"description=End time as RFC3339, unix timestamp, or duration ago."`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum entries or series."`
}

type LokiRecentLogsInput struct {
	GrafanaTargetInput
	Cluster   string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID       string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	App       string `json:"app,omitempty" jsonschema:"description=Application label filter."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace label filter."`
	Contains  string `json:"contains,omitempty" jsonschema:"description=Line contains filter."`
	Since     string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339, unix timestamp, or duration ago."`
	Until     string `json:"until,omitempty" jsonschema:"description=End time as RFC3339, unix timestamp, or duration ago."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum entries."`
}

type PrometheusQueryInput struct {
	GrafanaTargetInput
	Cluster string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Query   string `json:"query,omitempty" jsonschema:"required,description=PromQL query."`
	Time    string `json:"time,omitempty" jsonschema:"description=Evaluation time as RFC3339, unix timestamp, or duration ago."`
}

type PrometheusRangeInput struct {
	GrafanaTargetInput
	Cluster string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Query   string `json:"query,omitempty" jsonschema:"required,description=PromQL query."`
	Start   string `json:"start,omitempty" jsonschema:"description=Start time as RFC3339, unix timestamp, or duration ago."`
	End     string `json:"end,omitempty" jsonschema:"description=End time as RFC3339, unix timestamp, or duration ago."`
	Step    string `json:"step,omitempty" jsonschema:"description=Range step duration."`
}

type PrometheusRulesInput struct {
	GrafanaTargetInput
	Cluster string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Type    string `json:"type,omitempty" jsonschema:"description=Rule type filter: alert or record."`
}

type AlertsActiveInput struct {
	GrafanaTargetInput
	Cluster   string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID       string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Severity  string `json:"severity,omitempty" jsonschema:"description=Severity label filter."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace label filter."`
}

type AlertsActiveResult struct {
	URL     string          `json:"url"`
	UID     string          `json:"uid"`
	Cluster string          `json:"cluster,omitempty"`
	Count   int             `json:"count"`
	Alerts  json.RawMessage `json:"alerts"`
}

type AlertSilencesListInput struct {
	GrafanaTargetInput
	Cluster string   `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID     string   `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Filter  []string `json:"filter,omitempty" jsonschema:"description=Alertmanager silence filters."`
}

type AlertSilenceMatcher struct {
	Name    string `json:"name" jsonschema:"required,description=Label name."`
	Value   string `json:"value" jsonschema:"required,description=Label value or regular expression."`
	IsRegex bool   `json:"is_regex,omitempty" jsonschema:"description=Treat value as a regular expression."`
	IsEqual *bool  `json:"is_equal,omitempty" jsonschema:"description=Whether matcher is equality. Defaults to true."`
}

type AlertSilenceCreateInput struct {
	GrafanaTargetInput
	Cluster   string                `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID       string                `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	Matchers  []AlertSilenceMatcher `json:"matchers,omitempty" jsonschema:"required,description=Alertmanager matchers."`
	StartsAt  string                `json:"starts_at,omitempty" jsonschema:"description=Silence start time. Defaults to now."`
	EndsAt    string                `json:"ends_at,omitempty" jsonschema:"required,description=Silence end time."`
	CreatedBy string                `json:"created_by,omitempty" jsonschema:"description=Silence creator."`
	Comment   string                `json:"comment,omitempty" jsonschema:"required,description=Silence comment."`
}

type AlertSilenceDeleteInput struct {
	GrafanaTargetInput
	Cluster   string `json:"cluster,omitempty" jsonschema:"required,description=Datasource cluster alias from datasource.list or exact datasource UID suffix."`
	UID       string `json:"uid,omitempty" jsonschema:"description=Grafana datasource UID override."`
	SilenceID string `json:"silence_id,omitempty" jsonschema:"required,description=Alertmanager silence ID."`
}

type TempoSearchInput struct {
	GrafanaTargetInput
	UID   string `json:"uid,omitempty" jsonschema:"description=Grafana Tempo datasource UID override."`
	Query string `json:"query,omitempty" jsonschema:"description=Tempo search query."`
	Start string `json:"start,omitempty" jsonschema:"description=Start time as RFC3339, unix timestamp, or duration ago."`
	End   string `json:"end,omitempty" jsonschema:"description=End time as RFC3339, unix timestamp, or duration ago."`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum traces."`
}

type TempoTraceGetInput struct {
	GrafanaTargetInput
	UID     string `json:"uid,omitempty" jsonschema:"description=Grafana Tempo datasource UID override."`
	TraceID string `json:"trace_id,omitempty" jsonschema:"required,description=Tempo trace ID."`
}

type ProxyQueryResult struct {
	URL     string          `json:"url"`
	UID     string          `json:"uid"`
	Cluster string          `json:"cluster,omitempty"`
	Query   string          `json:"query,omitempty"`
	Data    json.RawMessage `json:"data"`
}

func (s Service) DatasourceList(ctx pluginbinding.Context, input DatasourceListInput) (DatasourceListResult, error) {
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return DatasourceListResult{}, err
	}
	datasources, err := s.datasources(context.Background(), client)
	if err != nil {
		return DatasourceListResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return datasourceListResult(target.URL, datasources), nil
}

func (s Service) DatasourceHealth(ctx pluginbinding.Context, input DatasourceHealthInput) (ProxyQueryResult, error) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "uid is required")
	}
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	raw, err := client.get(context.Background(), "/api/datasources/uid/"+url.PathEscape(uid)+"/health", nil)
	if err != nil {
		if fallback, ok := s.datasourceHealthFallback(context.Background(), target, client, uid); ok {
			return fallback, nil
		}
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return ProxyQueryResult{URL: target.URL, UID: uid, Data: raw}, nil
}

func (s Service) datasourceHealthFallback(ctx context.Context, target target, client Client, uid string) (ProxyQueryResult, bool) {
	datasources, err := s.datasources(ctx, client)
	if err != nil {
		return ProxyQueryResult{}, false
	}
	datasource, ok := datasourceByUID(datasources, uid)
	if !ok {
		return ProxyQueryResult{}, false
	}
	switch normalizeDatasourceType(datasource.Type) {
	case "alertmanager":
		raw, err := client.get(ctx, grafanaProxyPath(uid, "/api/v2/status"), nil)
		if err != nil {
			payload, marshalErr := json.Marshal(map[string]any{
				"status": "error",
				"source": "alertmanager_status",
				"error":  err.Error(),
			})
			if marshalErr != nil {
				return ProxyQueryResult{}, false
			}
			return ProxyQueryResult{URL: target.URL, UID: uid, Data: payload}, true
		}
		payload, err := json.Marshal(map[string]any{
			"status":   "OK",
			"source":   "alertmanager_status",
			"response": json.RawMessage(raw),
		})
		if err != nil {
			return ProxyQueryResult{}, false
		}
		return ProxyQueryResult{URL: target.URL, UID: uid, Data: payload}, true
	default:
		return ProxyQueryResult{}, false
	}
}

func (s Service) FolderList(ctx pluginbinding.Context, input FolderListInput) (FolderListResult, error) {
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return FolderListResult{}, err
	}
	values := url.Values{}
	if input.Limit > 0 {
		values.Set("limit", strconv.Itoa(input.Limit))
	}
	raw, err := client.get(context.Background(), "/api/folders", values)
	if err != nil {
		return FolderListResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	var folders []Folder
	if err := json.Unmarshal(raw, &folders); err != nil {
		return FolderListResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return FolderListResult{URL: target.URL, Count: len(folders), Folders: folders}, nil
}

func (s Service) DashboardList(ctx pluginbinding.Context, input DashboardListInput) (DashboardListResult, error) {
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return DashboardListResult{}, err
	}
	values := url.Values{"type": {"dash-db"}}
	if strings.TrimSpace(input.Query) != "" {
		values.Set("query", strings.TrimSpace(input.Query))
	}
	if strings.TrimSpace(input.FolderUID) != "" {
		values.Add("folderUIDs", strings.TrimSpace(input.FolderUID))
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) != "" {
			values.Add("tag", strings.TrimSpace(tag))
		}
	}
	if input.Limit > 0 {
		values.Set("limit", strconv.Itoa(input.Limit))
	}
	raw, err := client.get(context.Background(), "/api/search", values)
	if err != nil {
		return DashboardListResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	var dashboards []DashboardSearchHit
	if err := json.Unmarshal(raw, &dashboards); err != nil {
		return DashboardListResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return DashboardListResult{URL: target.URL, Count: len(dashboards), Dashboards: dashboards}, nil
}

func (s Service) DashboardGet(ctx pluginbinding.Context, input DashboardGetInput) (DashboardGetResult, error) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return DashboardGetResult{}, pluginbinding.Fail("bad_input", "uid is required")
	}
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return DashboardGetResult{}, err
	}
	raw, err := client.get(context.Background(), "/api/dashboards/uid/"+url.PathEscape(uid), nil)
	if err != nil {
		return DashboardGetResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	result, err := dashboardGetResult(target.URL, uid, raw)
	if err != nil {
		return DashboardGetResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return result, nil
}

func (s Service) AnnotationList(ctx pluginbinding.Context, input AnnotationListInput) (ProxyQueryResult, error) {
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	values, err := annotationListValues(input)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	raw, err := client.get(context.Background(), "/api/annotations", values)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return ProxyQueryResult{URL: target.URL, Data: raw}, nil
}

func (s Service) AnnotationAdd(ctx pluginbinding.Context, input AnnotationAddInput) (ProxyQueryResult, error) {
	if strings.TrimSpace(input.Text) == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "text is required")
	}
	target, client, err := s.client(ctx, input.GrafanaTargetInput)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	payload, err := annotationPayload(input)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	raw, err := client.postJSON(context.Background(), "/api/annotations", nil, payload)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return ProxyQueryResult{URL: target.URL, Data: raw}, nil
}

func (s Service) LokiLabels(ctx pluginbinding.Context, input LokiLabelsInput) (LabelsResult, error) {
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "loki", input.Cluster, input.UID)
	if err != nil {
		return LabelsResult{}, err
	}
	values := url.Values{}
	if strings.TrimSpace(input.Query) != "" {
		values.Set("query", strings.TrimSpace(input.Query))
	}
	path := "/loki/api/v1/labels"
	label := strings.TrimSpace(input.Label)
	if label != "" {
		path = "/loki/api/v1/label/" + url.PathEscape(label) + "/values"
	}
	raw, err := client.get(context.Background(), grafanaProxyPath(uid, path), values)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	data, err := unwrapDatasourceData(raw)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	var labels []string
	if err := json.Unmarshal(data, &labels); err != nil {
		return LabelsResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	sort.Strings(labels)
	return LabelsResult{URL: target.URL, UID: uid, Cluster: input.Cluster, Label: label, Values: labels}, nil
}

func (s Service) LokiQuery(ctx pluginbinding.Context, input LokiQueryInput) (LokiQueryResult, error) {
	if strings.TrimSpace(input.Query) == "" {
		return LokiQueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "loki", input.Cluster, input.UID)
	if err != nil {
		return LokiQueryResult{}, err
	}
	values, err := queryRangeValues(input.Query, input.Since, input.Until, input.Limit)
	if err != nil {
		return LokiQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	raw, err := client.get(context.Background(), grafanaProxyPath(uid, "/loki/api/v1/query_range"), values)
	if err != nil {
		return LokiQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	result, err := lokiQueryResult(target.URL, uid, input.Cluster, input.Query, input.Limit, raw)
	if err != nil {
		return LokiQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return result, nil
}

func (s Service) LokiRecentLogs(ctx pluginbinding.Context, input LokiRecentLogsInput) (LokiQueryResult, error) {
	query := recentLogsQuery(input)
	return s.LokiQuery(ctx, LokiQueryInput{GrafanaTargetInput: input.GrafanaTargetInput, Cluster: input.Cluster, UID: input.UID, Query: query, Since: input.Since, Until: input.Until, Limit: input.Limit})
}

func (s Service) PrometheusQuery(ctx pluginbinding.Context, input PrometheusQueryInput) (ProxyQueryResult, error) {
	if strings.TrimSpace(input.Query) == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "prometheus", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	values := url.Values{"query": {strings.TrimSpace(input.Query)}}
	if strings.TrimSpace(input.Time) != "" {
		t, err := parseTimeValue(input.Time, time.Now())
		if err != nil {
			return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("time", strconv.FormatInt(t.Unix(), 10))
	}
	return proxyResult(context.Background(), client, target.URL, uid, input.Cluster, input.Query, "/api/v1/query", values)
}

func (s Service) PrometheusRange(ctx pluginbinding.Context, input PrometheusRangeInput) (ProxyQueryResult, error) {
	if strings.TrimSpace(input.Query) == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "prometheus", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(input.End, "0s"), now)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start, err := parseTimeValue(firstNonEmpty(input.Start, "1h"), now)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if !start.Before(end) {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "start must be before end")
	}
	step := firstNonEmpty(input.Step, "1m")
	values := url.Values{"query": {strings.TrimSpace(input.Query)}}
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", step)
	return proxyResult(context.Background(), client, target.URL, uid, input.Cluster, input.Query, "/api/v1/query_range", values)
}

func (s Service) PrometheusRules(ctx pluginbinding.Context, input PrometheusRulesInput) (ProxyQueryResult, error) {
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "prometheus", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	values := url.Values{}
	if strings.TrimSpace(input.Type) != "" {
		values.Set("type", strings.TrimSpace(input.Type))
	}
	return proxyResult(context.Background(), client, target.URL, uid, input.Cluster, "", "/api/v1/rules", values)
}

func (s Service) AlertsActive(ctx pluginbinding.Context, input AlertsActiveInput) (AlertsActiveResult, error) {
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "alertmanager", input.Cluster, input.UID)
	if err != nil {
		return AlertsActiveResult{}, err
	}
	values := url.Values{"active": {"true"}, "silenced": {"false"}, "inhibited": {"false"}}
	raw, err := client.get(context.Background(), grafanaProxyPath(uid, "/api/v2/alerts"), values)
	if err != nil {
		return AlertsActiveResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	filtered, count, err := filterAlerts(raw, input.Severity, input.Namespace)
	if err != nil {
		return AlertsActiveResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return AlertsActiveResult{URL: target.URL, UID: uid, Cluster: input.Cluster, Count: count, Alerts: filtered}, nil
}

func (s Service) AlertSilencesList(ctx pluginbinding.Context, input AlertSilencesListInput) (ProxyQueryResult, error) {
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "alertmanager", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	values := url.Values{}
	for _, filter := range input.Filter {
		if strings.TrimSpace(filter) != "" {
			values.Add("filter", strings.TrimSpace(filter))
		}
	}
	return proxyResult(context.Background(), client, target.URL, uid, input.Cluster, "", "/api/v2/silences", values)
}

func (s Service) AlertSilenceCreate(ctx pluginbinding.Context, input AlertSilenceCreateInput) (ProxyQueryResult, error) {
	if len(input.Matchers) == 0 {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "matchers are required")
	}
	if strings.TrimSpace(input.EndsAt) == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "ends_at is required")
	}
	if strings.TrimSpace(input.Comment) == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "comment is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "alertmanager", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	payload, err := silencePayload(input)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	raw, err := client.postJSON(context.Background(), grafanaProxyPath(uid, "/api/v2/silences"), nil, payload)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return ProxyQueryResult{URL: target.URL, UID: uid, Cluster: input.Cluster, Data: raw}, nil
}

func (s Service) AlertSilenceDelete(ctx pluginbinding.Context, input AlertSilenceDeleteInput) (ProxyQueryResult, error) {
	silenceID := strings.TrimSpace(input.SilenceID)
	if silenceID == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "silence_id is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "alertmanager", input.Cluster, input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	raw, err := client.delete(context.Background(), grafanaProxyPath(uid, "/api/v2/silence/"+url.PathEscape(silenceID)), nil)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{"deleted":true}`)
	}
	return ProxyQueryResult{URL: target.URL, UID: uid, Cluster: input.Cluster, Data: raw}, nil
}

func (s Service) TempoSearch(ctx pluginbinding.Context, input TempoSearchInput) (ProxyQueryResult, error) {
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "tempo", "", input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	values := url.Values{}
	if strings.TrimSpace(input.Query) != "" {
		values.Set("q", strings.TrimSpace(input.Query))
	}
	now := time.Now()
	if strings.TrimSpace(input.Start) != "" {
		start, err := parseTimeValue(input.Start, now)
		if err != nil {
			return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("start", strconv.FormatInt(start.Unix(), 10))
	}
	if strings.TrimSpace(input.End) != "" {
		end, err := parseTimeValue(input.End, now)
		if err != nil {
			return ProxyQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("end", strconv.FormatInt(end.Unix(), 10))
	}
	if input.Limit > 0 {
		values.Set("limit", strconv.Itoa(input.Limit))
	}
	return proxyResult(context.Background(), client, target.URL, uid, "", input.Query, "/api/search", values)
}

func (s Service) TempoTraceGet(ctx pluginbinding.Context, input TempoTraceGetInput) (ProxyQueryResult, error) {
	traceID := strings.TrimSpace(input.TraceID)
	if traceID == "" {
		return ProxyQueryResult{}, pluginbinding.Fail("bad_input", "trace_id is required")
	}
	target, client, uid, err := s.resolveDatasource(ctx, input.GrafanaTargetInput, "tempo", "", input.UID)
	if err != nil {
		return ProxyQueryResult{}, err
	}
	return proxyResult(context.Background(), client, target.URL, uid, "", traceID, "/api/traces/"+url.PathEscape(traceID), nil)
}

func (s Service) client(ctx pluginbinding.Context, input GrafanaTargetInput) (target, Client, error) {
	target, err := s.target(ctx, input)
	if err != nil {
		return target, Client{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return target, Client{EndpointRef: target.EndpointRef, Host: ctx.Host}, nil
}

func (s Service) resolveDatasource(ctx pluginbinding.Context, input GrafanaTargetInput, datasourceType, cluster, explicitUID string) (target, Client, string, error) {
	target, client, err := s.client(ctx, input)
	if err != nil {
		return target, Client{}, "", err
	}
	if strings.TrimSpace(explicitUID) != "" {
		return target, client, strings.TrimSpace(explicitUID), nil
	}
	datasources, err := s.datasources(context.Background(), client)
	if err != nil {
		return target, Client{}, "", pluginbinding.Errorf("grafana", "%s", err)
	}
	uid, err := resolveUID(datasources, datasourceType, cluster)
	if err != nil {
		return target, Client{}, "", pluginbinding.Errorf("bad_input", "%s", err)
	}
	return target, client, uid, nil
}

func (s Service) datasources(ctx context.Context, client Client) ([]Datasource, error) {
	raw, err := client.get(ctx, "/api/datasources", nil)
	if err != nil {
		return nil, err
	}
	var datasources []Datasource
	if err := json.Unmarshal(raw, &datasources); err != nil {
		return nil, err
	}
	for i := range datasources {
		datasources[i].Type = normalizeDatasourceType(datasources[i].Type)
		datasources[i].Cluster = clusterFromUID(datasources[i].Type, datasources[i].UID)
		if datasources[i].Labels == nil && datasources[i].Cluster != "" {
			datasources[i].Labels = map[string]string{"cluster": datasources[i].Cluster}
		}
	}
	return datasources, nil
}

func proxyResult(ctx context.Context, client Client, baseURL, uid, cluster, query, path string, values url.Values) (ProxyQueryResult, error) {
	raw, err := client.get(ctx, grafanaProxyPath(uid, path), values)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	data, err := unwrapDatasourceData(raw)
	if err != nil {
		return ProxyQueryResult{}, pluginbinding.Errorf("grafana", "%s", err)
	}
	return ProxyQueryResult{URL: baseURL, UID: uid, Cluster: cluster, Query: strings.TrimSpace(query), Data: data}, nil
}

func lokiQueryResult(baseURL, uid, cluster, query string, limit int, raw json.RawMessage) (LokiQueryResult, error) {
	if limit <= 0 {
		limit = 100
	}
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
		ErrorType string `json:"errorType,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return LokiQueryResult{}, err
	}
	if response.Status != "" && response.Status != "success" {
		return LokiQueryResult{}, fmt.Errorf("datasource error %s: %s", response.ErrorType, response.Error)
	}
	var entries []LogEntry
	for _, stream := range response.Data.Result {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}
			ts := parseLogTimestamp(value[0])
			entries = append(entries, LogEntry{
				ID:        logEntryID(stream.Stream, value[0], value[1]),
				Timestamp: ts.Format(time.RFC3339Nano),
				Labels:    stream.Stream,
				Line:      value[1],
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp > entries[j].Timestamp })
	return LokiQueryResult{URL: baseURL, UID: uid, Cluster: cluster, NormalizedQuery: strings.TrimSpace(query), Entries: entries, Count: len(entries), Limit: limit, Raw: raw}, nil
}

func unwrapDatasourceData(raw json.RawMessage) (json.RawMessage, error) {
	var envelope struct {
		Status    string          `json:"status"`
		Data      json.RawMessage `json:"data"`
		ErrorType string          `json:"errorType,omitempty"`
		Error     string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Data != nil {
		if envelope.Status != "" && envelope.Status != "success" {
			return nil, fmt.Errorf("datasource error %s: %s", envelope.ErrorType, envelope.Error)
		}
		return envelope.Data, nil
	}
	return raw, nil
}

func dashboardGetResult(baseURL, uid string, raw json.RawMessage) (DashboardGetResult, error) {
	var envelope struct {
		Dashboard json.RawMessage `json:"dashboard"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return DashboardGetResult{}, err
	}
	if len(envelope.Dashboard) == 0 {
		envelope.Dashboard = raw
	}
	var dashboard struct {
		UID    string            `json:"uid"`
		Title  string            `json:"title"`
		Panels []json.RawMessage `json:"panels"`
	}
	if err := json.Unmarshal(envelope.Dashboard, &dashboard); err != nil {
		return DashboardGetResult{}, err
	}
	panels, queries := extractDashboardPanels(dashboard.Panels, nil)
	return DashboardGetResult{
		URL:       baseURL,
		UID:       firstNonEmpty(dashboard.UID, uid),
		Title:     dashboard.Title,
		Panels:    panels,
		Queries:   queries,
		Dashboard: envelope.Dashboard,
	}, nil
}

func extractDashboardPanels(rawPanels []json.RawMessage, parentPath []string) ([]DashboardPanel, []DashboardQuery) {
	var panels []DashboardPanel
	var queries []DashboardQuery
	for _, raw := range rawPanels {
		var panel struct {
			ID         int               `json:"id"`
			Title      string            `json:"title"`
			Type       string            `json:"type"`
			Datasource json.RawMessage   `json:"datasource"`
			Targets    []json.RawMessage `json:"targets"`
			Panels     []json.RawMessage `json:"panels"`
		}
		if err := json.Unmarshal(raw, &panel); err != nil {
			continue
		}
		path := append(append([]string(nil), parentPath...), panel.Title)
		datasource := dashboardTarget(panel.Datasource)
		item := DashboardPanel{ID: panel.ID, Title: panel.Title, Type: panel.Type, Datasource: datasource, PanelPath: path}
		for _, rawTarget := range panel.Targets {
			query := dashboardQuery(panel.ID, panel.Title, datasource, rawTarget)
			if query.Expression == "" && query.Query == "" {
				continue
			}
			item.Targets = append(item.Targets, query)
			queries = append(queries, query)
		}
		if len(item.Targets) > 0 || panel.Type != "row" {
			panels = append(panels, item)
		}
		childPanels, childQueries := extractDashboardPanels(panel.Panels, path)
		panels = append(panels, childPanels...)
		queries = append(queries, childQueries...)
	}
	return panels, queries
}

func dashboardQuery(panelID int, panelTitle string, panelDatasource DashboardTarget, raw json.RawMessage) DashboardQuery {
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return DashboardQuery{}
	}
	query := DashboardQuery{
		PanelID:    panelID,
		PanelTitle: panelTitle,
		RefID:      stringField(fields, "refId"),
		Datasource: firstDashboardTarget(dashboardTargetFromAny(fields["datasource"]), panelDatasource),
		Expression: firstNonEmpty(stringField(fields, "expr"), stringField(fields, "expression")),
		Query:      firstNonEmpty(stringField(fields, "query"), stringField(fields, "queryText")),
		QueryType:  stringField(fields, "queryType"),
		Raw:        raw,
	}
	if query.Query == "" && query.Expression != "" && query.Datasource.Type == "loki" {
		query.Query = query.Expression
	}
	return query
}

func dashboardTarget(raw json.RawMessage) DashboardTarget {
	if len(raw) == 0 || string(raw) == "null" {
		return DashboardTarget{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return DashboardTarget{}
	}
	return dashboardTargetFromAny(value)
}

func dashboardTargetFromAny(value any) DashboardTarget {
	switch typed := value.(type) {
	case string:
		return DashboardTarget{UID: typed}
	case map[string]any:
		return DashboardTarget{Type: normalizeDatasourceType(stringField(typed, "type")), UID: stringField(typed, "uid")}
	default:
		return DashboardTarget{}
	}
}

func firstDashboardTarget(values ...DashboardTarget) DashboardTarget {
	for _, value := range values {
		if value.Type != "" || value.UID != "" {
			return value
		}
	}
	return DashboardTarget{}
}

func annotationListValues(input AnnotationListInput) (url.Values, error) {
	values := url.Values{}
	now := time.Now()
	if strings.TrimSpace(input.Since) != "" {
		t, err := parseTimeValue(input.Since, now)
		if err != nil {
			return nil, err
		}
		values.Set("from", strconv.FormatInt(t.UnixMilli(), 10))
	}
	if strings.TrimSpace(input.Until) != "" {
		t, err := parseTimeValue(input.Until, now)
		if err != nil {
			return nil, err
		}
		values.Set("to", strconv.FormatInt(t.UnixMilli(), 10))
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) != "" {
			values.Add("tags", strings.TrimSpace(tag))
		}
	}
	if strings.TrimSpace(input.DashboardUID) != "" {
		values.Set("dashboardUID", strings.TrimSpace(input.DashboardUID))
	}
	if input.Limit > 0 {
		values.Set("limit", strconv.Itoa(input.Limit))
	}
	return values, nil
}

func annotationPayload(input AnnotationAddInput) (map[string]any, error) {
	now := time.Now()
	annotationTime := now
	var err error
	if strings.TrimSpace(input.Time) != "" {
		annotationTime, err = parseTimeValue(input.Time, now)
		if err != nil {
			return nil, err
		}
	}
	payload := map[string]any{
		"time": annotationTime.UnixMilli(),
		"text": strings.TrimSpace(input.Text),
	}
	if len(input.Tags) > 0 {
		payload["tags"] = input.Tags
	}
	if strings.TrimSpace(input.TimeEnd) != "" {
		end, err := parseTimeValue(input.TimeEnd, now)
		if err != nil {
			return nil, err
		}
		if !annotationTime.Before(end) {
			return nil, fmt.Errorf("time_end must be after time")
		}
		payload["timeEnd"] = end.UnixMilli()
		payload["isRegion"] = true
	}
	if strings.TrimSpace(input.DashboardUID) != "" {
		payload["dashboardUID"] = strings.TrimSpace(input.DashboardUID)
	}
	if input.PanelID > 0 {
		payload["panelId"] = input.PanelID
	}
	return payload, nil
}

func silencePayload(input AlertSilenceCreateInput) (map[string]any, error) {
	now := time.Now()
	start := now
	var err error
	if strings.TrimSpace(input.StartsAt) != "" {
		start, err = parseTimeValue(input.StartsAt, now)
		if err != nil {
			return nil, err
		}
	}
	end, err := parseTimeValue(input.EndsAt, now)
	if err != nil {
		return nil, err
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("ends_at must be after starts_at")
	}
	matchers := make([]map[string]any, 0, len(input.Matchers))
	for _, matcher := range input.Matchers {
		if strings.TrimSpace(matcher.Name) == "" || strings.TrimSpace(matcher.Value) == "" {
			return nil, fmt.Errorf("silence matchers require name and value")
		}
		isEqual := true
		if matcher.IsEqual != nil {
			isEqual = *matcher.IsEqual
		}
		matchers = append(matchers, map[string]any{
			"name":    strings.TrimSpace(matcher.Name),
			"value":   strings.TrimSpace(matcher.Value),
			"isRegex": matcher.IsRegex,
			"isEqual": isEqual,
		})
	}
	return map[string]any{
		"matchers":  matchers,
		"startsAt":  start.Format(time.RFC3339Nano),
		"endsAt":    end.Format(time.RFC3339Nano),
		"createdBy": firstNonEmpty(input.CreatedBy, "dex"),
		"comment":   strings.TrimSpace(input.Comment),
	}, nil
}

func filterAlerts(raw json.RawMessage, severity, namespace string) (json.RawMessage, int, error) {
	severity = strings.TrimSpace(severity)
	namespace = strings.TrimSpace(namespace)
	if severity == "" && namespace == "" {
		var values []any
		if err := json.Unmarshal(raw, &values); err == nil {
			return raw, len(values), nil
		}
		return raw, 0, nil
	}
	var alerts []map[string]any
	if err := json.Unmarshal(raw, &alerts); err != nil {
		return nil, 0, err
	}
	filtered := make([]map[string]any, 0, len(alerts))
	for _, alert := range alerts {
		if severity != "" && alertLabel(alert, "severity") != severity {
			continue
		}
		if namespace != "" && alertLabel(alert, "namespace") != namespace {
			continue
		}
		filtered = append(filtered, alert)
	}
	out, err := json.Marshal(filtered)
	return out, len(filtered), err
}

func alertLabel(alert map[string]any, key string) string {
	labels, ok := alert["labels"].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(labels[key]))
}

func datasourceListResult(baseURL string, datasources []Datasource) DatasourceListResult {
	clusters := map[string]map[string]string{}
	types := map[string][]string{}
	for _, datasource := range datasources {
		if datasource.Type != "" && datasource.UID != "" {
			types[datasource.Type] = append(types[datasource.Type], datasource.UID)
		}
		if datasource.Cluster == "" || datasource.Type == "" || datasource.UID == "" {
			continue
		}
		if clusters[datasource.Cluster] == nil {
			clusters[datasource.Cluster] = map[string]string{}
		}
		clusters[datasource.Cluster][datasource.Type] = datasource.UID
	}
	for typ := range types {
		sort.Strings(types[typ])
	}
	return DatasourceListResult{URL: baseURL, Count: len(datasources), Datasources: datasources, Clusters: clusters, Types: types}
}

func datasourceByUID(datasources []Datasource, uid string) (Datasource, bool) {
	uid = strings.TrimSpace(uid)
	for _, datasource := range datasources {
		if strings.EqualFold(strings.TrimSpace(datasource.UID), uid) {
			return datasource, true
		}
	}
	return Datasource{}, false
}

func stringField(fields map[string]any, key string) string {
	value, ok := fields[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func resolveUID(datasources []Datasource, datasourceType, cluster string) (string, error) {
	datasourceType = normalizeDatasourceType(datasourceType)
	cluster, err := resolveClusterAlias(datasources, datasourceType, cluster)
	if err != nil {
		return "", err
	}
	expected := expectedUID(datasourceType, cluster)
	if expected != "" {
		for _, datasource := range datasources {
			if strings.EqualFold(datasource.UID, expected) && datasourceTypeMatches(datasource.Type, datasourceType) {
				return datasource.UID, nil
			}
		}
	}
	var matches []Datasource
	for _, datasource := range datasources {
		if !datasourceTypeMatches(datasource.Type, datasourceType) {
			continue
		}
		if cluster == "" || clusterFromUID(datasource.Type, datasource.UID) == cluster || strings.Contains(strings.ToLower(datasource.Name+" "+datasource.UID), cluster) {
			matches = append(matches, datasource)
		}
	}
	if len(matches) == 1 {
		return matches[0].UID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple %s datasources match cluster %q; pass uid explicitly", datasourceType, cluster)
	}
	if expected != "" {
		return "", fmt.Errorf("no %s datasource found for cluster %q (expected uid %q)", datasourceType, cluster, expected)
	}
	return "", fmt.Errorf("no %s datasource found; pass uid explicitly", datasourceType)
}

func expectedUID(datasourceType, cluster string) string {
	datasourceType = normalizeDatasourceType(datasourceType)
	if datasourceType == "tempo" {
		return "tempo"
	}
	if cluster == "" {
		return ""
	}
	if cluster == "infra" {
		return datasourceType
	}
	return datasourceType + "-" + cluster
}

func clusterFromUID(datasourceType, uid string) string {
	datasourceType = normalizeDatasourceType(datasourceType)
	uid = strings.ToLower(strings.TrimSpace(uid))
	if uid == "" {
		return ""
	}
	if datasourceType != "" && uid == datasourceType {
		return "infra"
	}
	prefix := datasourceType + "-"
	if datasourceType != "" && strings.HasPrefix(uid, prefix) {
		return strings.TrimPrefix(uid, prefix)
	}
	return ""
}

func resolveClusterAlias(datasources []Datasource, datasourceType, requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" || requested == "infra" {
		return requested, nil
	}
	clusters := datasourceClusters(datasources, datasourceType)
	if clusters[requested] {
		return requested, nil
	}
	var matches []string
	for cluster := range clusters {
		env, _, _ := strings.Cut(cluster, "-")
		if env == requested {
			matches = append(matches, cluster)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return requested, nil
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("cluster alias %q is ambiguous; choose one of %s", requested, strings.Join(matches, ", "))
	}
}

func datasourceClusters(datasources []Datasource, datasourceType string) map[string]bool {
	out := map[string]bool{}
	for _, datasource := range datasources {
		if !datasourceTypeMatches(datasource.Type, datasourceType) {
			continue
		}
		cluster := clusterFromUID(datasource.Type, datasource.UID)
		if cluster != "" {
			out[cluster] = true
		}
	}
	return out
}

func normalizeDatasourceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "prom", "prometheus":
		return "prometheus"
	case "loki":
		return "loki"
	case "alertmanager", "alertmanagerng":
		return "alertmanager"
	case "tempo":
		return "tempo"
	case "cloudwatch", "grafana-cloudwatch-datasource":
		return "cloudwatch"
	default:
		return value
	}
}

func datasourceTypeMatches(actual, expected string) bool {
	actual = normalizeDatasourceType(actual)
	expected = normalizeDatasourceType(expected)
	return actual == expected
}

func queryRangeValues(query, since, until string, limit int) (url.Values, error) {
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(until, "0s"), now)
	if err != nil {
		return nil, err
	}
	start, err := parseTimeValue(firstNonEmpty(since, "1h"), now)
	if err != nil {
		return nil, err
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("since must be before until")
	}
	values := url.Values{"query": {strings.TrimSpace(query)}}
	values.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	values.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	return values, nil
}

func recentLogsQuery(input LokiRecentLogsInput) string {
	var selectors []string
	if strings.TrimSpace(input.Namespace) != "" {
		selectors = append(selectors, `namespace="`+escapeLogQLLabel(input.Namespace)+`"`)
	}
	if strings.TrimSpace(input.App) != "" {
		selectors = append(selectors, `app=~"`+escapeLogQLLabel(input.App)+`"`)
	}
	query := "{"
	if len(selectors) > 0 {
		query += strings.Join(selectors, ",")
	}
	query += "}"
	if strings.TrimSpace(input.Contains) != "" {
		query += ` |= "` + escapeLogQLString(input.Contains) + `"`
	}
	return query
}

func parseTimeValue(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return now, nil
	}
	if strings.HasSuffix(value, "ago") {
		value = strings.TrimSpace(strings.TrimSuffix(value, "ago"))
	}
	if d, err := time.ParseDuration(value); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		if i > 1_000_000_000_000 {
			return time.UnixMilli(i), nil
		}
		return time.Unix(i, 0), nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q", value)
}

func parseLogTimestamp(value string) time.Time {
	if ns, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		return time.Unix(0, ns)
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	return time.Time{}
}

func logEntryID(labels map[string]string, ts, line string) string {
	sum := sha1.Sum([]byte(joinLabels(labels) + "\x00" + ts + "\x00" + line))
	return hex.EncodeToString(sum[:8])
}

func joinLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}

func escapeLogQLLabel(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(strings.TrimSpace(value))
}

func escapeLogQLString(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
