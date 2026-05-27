package gitlab

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithRunner(NewOperationRunner())
}

func NewPluginWithRunner(runner OperationRunner) *pluginbinding.Plugin {
	plugin := pluginbinding.New(Manifest())
	plugin.AuthConnectText("Use dex auth connect gitlab --field access_token=<token> --field gitlab_url=<url>")
	plugin.AuthTestOperation("gitlab.auth.test")
	plugin.IndexBuildOperation("gitlab.index.build")
	plugin.HostOwnedIndexStatus("GitLab")

	pluginbinding.Operation(plugin, operationSpec("gitlab.auth.test"), runner.AuthTest)
	pluginbinding.Operation(plugin, operationSpec("gitlab.index.build"), runner.IndexBuild)
	pluginbinding.Operation(plugin, operationSpec("gitlab.project.list"), runner.ProjectList)
	pluginbinding.Operation(plugin, operationSpec("gitlab.project.show"), runner.ProjectShow)
	pluginbinding.Operation(plugin, operationSpec("gitlab.mr.list"), runner.MergeRequestList)
	pluginbinding.Operation(plugin, operationSpec("gitlab.mr.show"), runner.MergeRequestShow)

	return plugin
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}

func operationSpec(name string) core.OperationSpec {
	for _, spec := range Manifest().Operations {
		if spec.Name == name {
			return spec
		}
	}
	return core.OperationSpec{Name: name}
}
