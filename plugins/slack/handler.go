package slack

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
		pluginbinding.WithHostManagedAuthTest("Slack"),
		pluginbinding.WithIndexBuildOperation(OperationIndexBuild),
		pluginbinding.WithHostOwnedIndexStatus("Slack"),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(infoSpec(), service.Info),
		pluginbinding.RegisterOperation(messageSendSpec(), service.SendMessage),
		pluginbinding.RegisterOperation(searchSpec(), service.Search),
		pluginbinding.RegisterOperation(threadSpec(), service.Thread),
		pluginbinding.RegisterDatasourceSearch(slackMessagesDatasourceSpec(), service.SearchMessagesDatasource),
		pluginbinding.RegisterDatasourceSearch(slackThreadMessagesDatasourceSpec(), service.ThreadMessagesDatasource),
		pluginbinding.RegisterDatasourceSearch(slackChannelMembersDatasourceSpec(), service.ChannelMembersDatasource),
		pluginbinding.RegisterDatasourceLookup(slackUsersLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceLookup(slackChannelsLookupSpec(), service.Lookup),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
