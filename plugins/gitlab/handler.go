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
		pluginbinding.RegisterOperation(mergeRequestCreateSpec(), service.MergeRequestCreate),
		pluginbinding.RegisterOperation(mergeRequestApproveSpec(), service.MergeRequestApprove),
		pluginbinding.RegisterOperation(mergeRequestMergeSpec(), service.MergeRequestMerge),
		pluginbinding.RegisterOperation(repositoryTagCreateSpec(), service.RepositoryTagCreate),
		pluginbinding.RegisterOperation(branchCreateSpec(), service.BranchCreate),
		pluginbinding.RegisterOperation(branchDeleteSpec(), service.BranchDelete),
		pluginbinding.RegisterOperation(branchDeleteMergedSpec(), service.BranchDeleteMerged),
		pluginbinding.RegisterOperation(repoFileCreateSpec(), service.RepoFileCreate),
		pluginbinding.RegisterOperation(repoFileUpdateSpec(), service.RepoFileUpdate),
		pluginbinding.RegisterOperation(repoFileDeleteSpec(), service.RepoFileDelete),
		pluginbinding.RegisterOperation(commitCreateSpec(), service.CommitCreate),
		pluginbinding.RegisterOperation(ciVariableCreateSpec(), service.CIVariableCreate),
		pluginbinding.RegisterOperation(ciVariableUpdateSpec(), service.CIVariableUpdate),
		pluginbinding.RegisterOperation(ciVariableDeleteSpec(), service.CIVariableDelete),
		pluginbinding.RegisterOperation(pipelineCreateSpec(), service.PipelineCreate),
		pluginbinding.RegisterOperation(pipelineRetrySpec(), service.PipelineRetry),
		pluginbinding.RegisterOperation(pipelineCancelSpec(), service.PipelineCancel),
		pluginbinding.RegisterOperation(snippetCreateSpec(), service.SnippetCreate),
		pluginbinding.RegisterOperation(snippetDeleteSpec(), service.SnippetDelete),
		pluginbinding.RegisterContextProvider(pluginbinding.ContextSpec(ContextName, "GitLab context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference), BuildContext),
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
