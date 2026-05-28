package jira

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
		pluginbinding.WithHostOwnedIndexStatus("Jira"),
		pluginbinding.RegisterOperation(authTestSpec(), service.AuthTest),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(createMetaSpec(), service.CreateMeta),
		pluginbinding.RegisterOperation(editMetaSpec(), service.EditMeta),
		pluginbinding.RegisterOperation(transitionListSpec(), service.TransitionList),
		pluginbinding.RegisterOperation(transitionRunSpec(), service.TransitionRun),
		pluginbinding.RegisterOperation(commentAddSpec(), service.CommentAdd),
		pluginbinding.RegisterOperation(commentEditSpec(), service.CommentEdit),
		pluginbinding.RegisterOperation(commentDeleteSpec(), service.CommentDelete),
		pluginbinding.RegisterOperation(attachmentAddSpec(), service.AttachmentAdd),
		pluginbinding.RegisterOperation(attachmentListSpec(), service.AttachmentList),
		pluginbinding.RegisterOperation(attachmentGetSpec(), service.AttachmentGet),
		pluginbinding.RegisterOperation(attachmentDeleteSpec(), service.AttachmentDelete),
		pluginbinding.RegisterOperation(issueCreateSpec(), service.IssueCreate),
		pluginbinding.RegisterOperation(issueEditSpec(), service.IssueEdit),
		pluginbinding.RegisterOperation(issueDeleteSpec(), service.IssueDelete),
		pluginbinding.RegisterOperation(issueSearchSpec(), service.IssueSearch),
		pluginbinding.RegisterOperation(issueShowSpec(), service.IssueShow),
		pluginbinding.RegisterOperation(userSearchSpec(), service.UserSearch),
		pluginbinding.RegisterDatasourceSearch(jiraIssuesDatasourceSpec(), service.IssueDatasource),
		pluginbinding.RegisterDatasourceSearch(jiraUsersDatasourceSpec(), service.UserDatasource),
		pluginbinding.RegisterDatasourceLookup(jiraIssuesLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(jiraUsersLookupSpec(), service.Lookup),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
