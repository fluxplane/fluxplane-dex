package grafana

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "grafana"
	PluginVersion     = "0.11.0"
	PluginDescription = "Grafana datasource catalog and proxy operations for Loki, Prometheus, Alertmanager, and Tempo."

	EnvGrafanaAPIToken = "GRAFANA_API_TOKEN"
	EnvGrafanaUsername = "GRAFANA_USERNAME"
	EnvGrafanaPassword = "GRAFANA_PASSWORD"

	AuthPurposeAPIToken = "api_token"
	AuthPurposeUsername = "username"
	AuthPurposePassword = "password"

	OperationDatasourceList     = "grafana.datasource.list"
	OperationDatasourceHealth   = "grafana.datasource.health"
	OperationFolderList         = "grafana.folder.list"
	OperationDashboardList      = "grafana.dashboard.list"
	OperationDashboardGet       = "grafana.dashboard.get"
	OperationAnnotationList     = "grafana.annotation.list"
	OperationAnnotationAdd      = "grafana.annotation.add"
	OperationLokiLabels         = "grafana.loki.labels"
	OperationLokiQuery          = "grafana.loki.query"
	OperationLokiRecentLogs     = "grafana.loki.recent_logs"
	OperationPrometheusQuery    = "grafana.prometheus.query"
	OperationPrometheusRange    = "grafana.prometheus.range"
	OperationPrometheusRules    = "grafana.prometheus.rules"
	OperationAlertsActive       = "grafana.alerts.active"
	OperationAlertSilencesList  = "grafana.alerts.silences.list"
	OperationAlertSilenceCreate = "grafana.alerts.silences.create"
	OperationAlertSilenceDelete = "grafana.alerts.silences.delete"
	OperationTempoSearch        = "grafana.tempo.search"
	OperationTempoTraceGet      = "grafana.tempo.trace.get"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"graf", PluginName},
		Operations: []core.OperationSpec{
			datasourceListSpec(),
			datasourceHealthSpec(),
			folderListSpec(),
			dashboardListSpec(),
			dashboardGetSpec(),
			annotationListSpec(),
			annotationAddSpec(),
			lokiLabelsSpec(),
			lokiQuerySpec(),
			lokiRecentLogsSpec(),
			prometheusQuerySpec(),
			prometheusRangeSpec(),
			prometheusRulesSpec(),
			alertsActiveSpec(),
			alertSilencesListSpec(),
			alertSilenceCreateSpec(),
			alertSilenceDeleteSpec(),
			tempoSearchSpec(),
			tempoTraceGetSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Grafana bearer token or basic auth used by host-resolved endpoint refs.",
			Env:         []string{EnvGrafanaAPIToken, EnvGrafanaUsername, EnvGrafanaPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIToken, "Grafana service account token", true, false, EnvGrafanaAPIToken),
				pluginbinding.AuthField(AuthPurposeUsername, "Grafana basic auth username", false, false, EnvGrafanaUsername),
				pluginbinding.AuthField(AuthPurposePassword, "Grafana basic auth password", true, false, EnvGrafanaPassword),
			},
		}},
	}
}

func datasourceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DatasourceListInput, DatasourceListResult](OperationDatasourceList, "List Grafana datasources and derived cluster aliases.", readOptions(core.OperationIdempotent)...)
}

func datasourceHealthSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DatasourceHealthInput, ProxyQueryResult](OperationDatasourceHealth, "Check Grafana datasource health.", readOptions(core.OperationIdempotent)...)
}

func folderListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FolderListInput, FolderListResult](OperationFolderList, "List Grafana folders.", readOptions(core.OperationIdempotent)...)
}

func dashboardListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DashboardListInput, DashboardListResult](OperationDashboardList, "Search Grafana dashboards.", readOptions(core.OperationIdempotent)...)
}

func dashboardGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DashboardGetInput, DashboardGetResult](OperationDashboardGet, "Get a Grafana dashboard and extract panel queries.", readOptions(core.OperationIdempotent)...)
}

func annotationListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AnnotationListInput, ProxyQueryResult](OperationAnnotationList, "List Grafana annotations.", readOptions(core.OperationIdempotent)...)
}

func annotationAddSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AnnotationAddInput, ProxyQueryResult](OperationAnnotationAdd, "Create a Grafana annotation.", writeOptions(core.OperationNonIdempotent)...)
}

func lokiLabelsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LokiLabelsInput, LabelsResult](OperationLokiLabels, "List Loki labels through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func lokiQuerySpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LokiQueryInput, LokiQueryResult](OperationLokiQuery, "Run a Loki range query through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func lokiRecentLogsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LokiRecentLogsInput, LokiQueryResult](OperationLokiRecentLogs, "Query recent Loki logs by cluster, app, and namespace.", readOptions(core.OperationIdempotent)...)
}

func prometheusQuerySpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PrometheusQueryInput, ProxyQueryResult](OperationPrometheusQuery, "Run an instant Prometheus query through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func prometheusRangeSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PrometheusRangeInput, ProxyQueryResult](OperationPrometheusRange, "Run a Prometheus range query through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func prometheusRulesSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PrometheusRulesInput, ProxyQueryResult](OperationPrometheusRules, "List Prometheus alerting and recording rules through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func alertsActiveSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertsActiveInput, AlertsActiveResult](OperationAlertsActive, "List active Alertmanager alerts through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func alertSilencesListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertSilencesListInput, ProxyQueryResult](OperationAlertSilencesList, "List Alertmanager silences through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func alertSilenceCreateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertSilenceCreateInput, ProxyQueryResult](OperationAlertSilenceCreate, "Create an Alertmanager silence through Grafana datasource proxy.", writeOptions(core.OperationNonIdempotent)...)
}

func alertSilenceDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertSilenceDeleteInput, ProxyQueryResult](OperationAlertSilenceDelete, "Delete an Alertmanager silence through Grafana datasource proxy.", writeOptions(core.OperationNonIdempotent)...)
}

func tempoSearchSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TempoSearchInput, ProxyQueryResult](OperationTempoSearch, "Search Tempo traces through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func tempoTraceGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TempoTraceGetInput, ProxyQueryResult](OperationTempoTraceGet, "Fetch a Tempo trace by trace ID through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(grafanaSecretPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func writeOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(grafanaSecretPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func grafanaSecretPurposes() []string {
	return []string{AuthPurposeAPIToken, AuthPurposeUsername, AuthPurposePassword}
}
