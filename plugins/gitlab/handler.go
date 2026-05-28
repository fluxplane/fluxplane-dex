package gitlab

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.WithSecretGetter(service.SecretGetter),
		pluginbinding.WithAuthTestOperation(OperationAuthTest),
		pluginbinding.WithIndexBuildOperation(OperationIndexBuild),
		pluginbinding.WithHostOwnedIndexStatus("GitLab"),
		pluginbinding.RegisterOperation(authTestSpec(), service.AuthTest),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(projectListSpec(), service.ProjectList),
		pluginbinding.RegisterOperation(projectShowSpec(), service.ProjectShow),
		pluginbinding.RegisterOperation(mergeRequestListSpec(), service.MergeRequestList),
		pluginbinding.RegisterOperation(mergeRequestShowSpec(), service.MergeRequestShow),
		pluginbinding.RegisterDatasourceLookup(gitlabProjectsLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(gitlabUsersLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(gitlabGroupsLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(gitlabIssuesLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(gitlabMergeRequestsLookupSpec(), service.Lookup),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
