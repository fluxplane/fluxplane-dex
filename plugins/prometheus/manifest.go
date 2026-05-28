package prometheus

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "prometheus"
	PluginVersion     = "0.7.0"
	PluginDescription = "Prometheus endpoint discovery, health checks, PromQL queries, labels, targets, and alerts."

	EnvPrometheusURL = "PROMETHEUS_URL"

	OperationTest       = "prometheus.test"
	OperationQuery      = "prometheus.query"
	OperationQueryRange = "prometheus.query_range"
	OperationLabels     = "prometheus.labels"
	OperationTargets    = "prometheus.targets"
	OperationAlerts     = "prometheus.alerts"

	DatasourceQueryResults = "prometheus.query_results"
	DatasourceLabels       = "prometheus.labels"
	DatasourceTargets      = "prometheus.targets"
	DatasourceAlerts       = "prometheus.alerts"

	EntityQueryResult = "prometheus.query_result"
	EntityLabel       = "prometheus.label"
	EntityTarget      = "prometheus.target"
	EntityAlert       = "prometheus.alert"

	EndpointPrometheus = "prometheus.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"prom", PluginName},
		Operations: []core.OperationSpec{
			testSpec(),
			querySpec(),
			queryRangeSpec(),
			labelsSpec(),
			targetsSpec(),
			alertsSpec(),
		},
		Datasources: []core.DatasourceSpec{
			queryResultsDatasourceSpec(),
			labelsDatasourceSpec(),
			targetsDatasourceSpec(),
			alertsDatasourceSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Prometheus endpoint URL.",
			Env:         []string{EnvPrometheusURL},
			Fields: []core.AuthField{
				pluginbinding.AuthField("url", "Prometheus base URL", false, false, EnvPrometheusURL),
			},
		}},
	}
}

func queryResultsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[QueryInput, QueryDatasourceResult, QueryRecord](DatasourceQueryResults, EntityQueryResult, "Prometheus instant query result records.", "query", "result_type", "endpoint_url")
}

func labelsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[LabelsInput, LabelDatasourceResult, LabelRecord](DatasourceLabels, EntityLabel, "Prometheus label names or values.", "name", "label", "endpoint_url")
}

func targetsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[TargetsInput, TargetDatasourceResult, TargetRecord](DatasourceTargets, EntityTarget, "Prometheus scrape targets.", "state", "job", "endpoint", "endpoint_url")
}

func alertsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[TestInput, AlertDatasourceResult, AlertRecord](DatasourceAlerts, EntityAlert, "Prometheus alerts.", "name", "state", "severity", "endpoint_url")
}

func datasourceSpec[I any, O any, R any](name, entity, description string, completions ...string) core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[I, O](
		name,
		entity,
		description,
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.EntitySchemaFor[R](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion(description, completions...),
	)
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest, "Test Prometheus readiness.", readOptions(core.OperationIdempotent)...)
}

func querySpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[QueryInput, QueryResult](OperationQuery, "Run an instant PromQL query.", readOptions(core.OperationIdempotent)...)
}

func queryRangeSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[QueryRangeInput, QueryRangeResult](OperationQueryRange, "Run a range PromQL query.", readOptions(core.OperationIdempotent)...)
}

func labelsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LabelsInput, LabelsResult](OperationLabels, "List Prometheus label names or values.", readOptions(core.OperationIdempotent)...)
}

func targetsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TargetsInput, TargetsResult](OperationTargets, "List Prometheus scrape targets.", readOptions(core.OperationIdempotent)...)
}

func alertsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, AlertsResult](OperationAlerts, "List Prometheus alerts.", readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
