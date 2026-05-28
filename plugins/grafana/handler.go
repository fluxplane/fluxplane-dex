package grafana

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(datasourceListSpec(), service.DatasourceList),
		pluginbinding.RegisterOperation(datasourceHealthSpec(), service.DatasourceHealth),
		pluginbinding.RegisterOperation(folderListSpec(), service.FolderList),
		pluginbinding.RegisterOperation(dashboardListSpec(), service.DashboardList),
		pluginbinding.RegisterOperation(dashboardGetSpec(), service.DashboardGet),
		pluginbinding.RegisterOperation(annotationListSpec(), service.AnnotationList),
		pluginbinding.RegisterOperation(annotationAddSpec(), service.AnnotationAdd),
		pluginbinding.RegisterOperation(lokiLabelsSpec(), service.LokiLabels),
		pluginbinding.RegisterOperation(lokiQuerySpec(), service.LokiQuery),
		pluginbinding.RegisterOperation(lokiRecentLogsSpec(), service.LokiRecentLogs),
		pluginbinding.RegisterOperation(prometheusQuerySpec(), service.PrometheusQuery),
		pluginbinding.RegisterOperation(prometheusRangeSpec(), service.PrometheusRange),
		pluginbinding.RegisterOperation(prometheusRulesSpec(), service.PrometheusRules),
		pluginbinding.RegisterOperation(alertsActiveSpec(), service.AlertsActive),
		pluginbinding.RegisterOperation(alertSilencesListSpec(), service.AlertSilencesList),
		pluginbinding.RegisterOperation(alertSilenceCreateSpec(), service.AlertSilenceCreate),
		pluginbinding.RegisterOperation(alertSilenceDeleteSpec(), service.AlertSilenceDelete),
		pluginbinding.RegisterOperation(tempoSearchSpec(), service.TempoSearch),
		pluginbinding.RegisterOperation(tempoTraceGetSpec(), service.TempoTraceGet),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
