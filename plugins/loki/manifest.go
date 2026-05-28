package loki

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "loki"
	PluginVersion     = "0.1.0"
	PluginDescription = "Loki endpoint discovery, health checks, LogQL queries, recent logs, and labels."

	EnvLokiURL      = "LOKI_URL"
	EnvLokiTenantID = "LOKI_TENANT_ID"

	OperationTest       = "loki.test"
	OperationQuery      = "loki.query"
	OperationLabels     = "loki.labels"
	OperationRecentLogs = "loki.recent_logs"

	EndpointLoki = "loki.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{PluginName},
		Operations: []core.OperationSpec{
			testSpec(),
			querySpec(),
			labelsSpec(),
			recentLogsSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Loki endpoint URL and optional tenant ID.",
			Env:         []string{EnvLokiURL, EnvLokiTenantID},
			Fields: []core.AuthField{
				pluginbinding.AuthField("url", "Loki base URL", false, false, EnvLokiURL),
				pluginbinding.AuthField("tenant_id", "Loki tenant ID", false, false, EnvLokiTenantID),
			},
		}},
	}
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest, "Test Loki readiness.", readOptions(core.OperationIdempotent)...)
}

func querySpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[QueryInput, QueryResult](OperationQuery, "Run a LogQL query.", readOptions(core.OperationIdempotent)...)
}

func labelsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LabelsInput, LabelsResult](OperationLabels, "List Loki label names or values.", readOptions(core.OperationIdempotent)...)
}

func recentLogsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[RecentLogsInput, QueryResult](OperationRecentLogs, "Query recent logs by app, pod, container, or text filter.", readOptions(core.OperationIdempotent)...)
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
