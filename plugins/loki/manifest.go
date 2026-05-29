package loki

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "loki"
	PluginVersion     = "0.14.0"
	PluginDescription = "Loki endpoint discovery, health checks, LogQL queries, recent logs, and labels."

	EnvLokiTenantID     = "LOKI_TENANT_ID"
	AuthPurposeTenantID = "tenant_id"

	OperationTest       = "loki.test"
	OperationQuery      = "loki.query"
	OperationLabels     = "loki.labels"
	OperationRecentLogs = "loki.recent_logs"

	DatasourceLogEntries = "loki.log_entries"
	DatasourceLabels     = "loki.labels"

	EntityLogEntry = "loki.log_entry"
	EntityLabel    = "loki.label"

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
		Datasources: []core.DatasourceSpec{
			logEntriesDatasourceSpec(),
			labelsDatasourceSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Optional Loki tenant ID used by host-resolved endpoint refs.",
			Env:         []string{EnvLokiTenantID},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeTenantID, "Loki tenant ID", false, false, EnvLokiTenantID),
			},
		}},
	}
}

func logEntriesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LogEntriesInput, LogEntriesDatasourceResult](
		DatasourceLogEntries,
		EntityLogEntry,
		"Loki log entries.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.EntitySchemaFor[LogEntryRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("Loki log entry fields.", "app", "namespace", "pod", "container", "endpoint_url"),
	)
}

func labelsDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LabelsInput, LabelDatasourceResult](
		DatasourceLabels,
		EntityLabel,
		"Loki label names or values.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.EntitySchemaFor[LabelRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("Loki label fields.", "name", "label", "endpoint_url"),
	)
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
		pluginbinding.SecretPurposes(AuthPurposeTenantID),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
