package confluence

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.WithAuthTestOperation(OperationAuthTest),
		pluginbinding.WithIndexBuildOperation(OperationIndexBuild),
		pluginbinding.WithHostOwnedIndexStatus("Confluence"),
		pluginbinding.RegisterOperation(authTestSpec(), service.AuthTest),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(attachmentAddSpec(), service.AttachmentAdd),
		pluginbinding.RegisterOperation(attachmentListSpec(), service.AttachmentList),
		pluginbinding.RegisterOperation(attachmentGetSpec(), service.AttachmentGet),
		pluginbinding.RegisterOperation(attachmentDeleteSpec(), service.AttachmentDelete),
		pluginbinding.RegisterOperation(pageCreateSpec(), service.PageCreate),
		pluginbinding.RegisterOperation(pageDeleteSpec(), service.PageDelete),
		pluginbinding.RegisterOperation(pageSearchSpec(), service.PageSearch),
		pluginbinding.RegisterOperation(pageShowSpec(), service.PageShow),
		pluginbinding.RegisterOperation(userSearchSpec(), service.UserSearch),
		pluginbinding.RegisterDatasourceSearch(confluencePagesDatasourceSpec(), service.PageDatasource),
		pluginbinding.RegisterDatasourceSearch(confluenceUsersDatasourceSpec(), service.UserDatasource),
		pluginbinding.RegisterDatasourceLookup(confluencePagesLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(confluenceUsersLookupSpec(), service.Lookup),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
