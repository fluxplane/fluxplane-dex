package prometheus

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "prometheus"
	PluginVersion     = "0.1.0"
	PluginDescription = "Prometheus endpoint discovery, health checks, PromQL queries, labels, targets, and alerts."

	EnvPrometheusURL = "PROMETHEUS_URL"

	OperationTest       = "prometheus.test"
	OperationQuery      = "prometheus.query"
	OperationQueryRange = "prometheus.query_range"
	OperationLabels     = "prometheus.labels"
	OperationTargets    = "prometheus.targets"
	OperationAlerts     = "prometheus.alerts"

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
