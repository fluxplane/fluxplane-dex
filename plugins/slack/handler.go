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
		pluginbinding.WithAuthTestOperation(OperationAuthTest),
		pluginbinding.WithIndexBuildOperation(OperationIndexBuild),
		pluginbinding.WithHostOwnedIndexStatus("Slack"),
		pluginbinding.RegisterOperation(authTestSpec(), service.AuthTest),
		pluginbinding.RegisterOperation(bookmarkAddSpec(), service.AddBookmark),
		pluginbinding.RegisterOperation(bookmarkEditSpec(), service.EditBookmark),
		pluginbinding.RegisterOperation(bookmarkDeleteSpec(), service.DeleteBookmark),
		pluginbinding.RegisterOperation(bookmarkListSpec(), service.ListBookmarks),
		pluginbinding.RegisterOperation(channelJoinSpec(), service.JoinChannel),
		pluginbinding.RegisterOperation(channelListSpec(), service.ListChannels),
		pluginbinding.RegisterOperation(channelMarkSpec(), service.MarkRead),
		pluginbinding.RegisterOperation(downloadSpec(), service.DownloadFile),
		pluginbinding.RegisterOperation(emojiListSpec(), service.ListEmojis),
		pluginbinding.RegisterOperation(fileDeleteSpec(), service.DeleteFile),
		pluginbinding.RegisterOperation(fileDownloadSpec(), service.DownloadFile),
		pluginbinding.RegisterOperation(fileInfoSpec(), service.FileInfo),
		pluginbinding.RegisterOperation(fileListSpec(), service.ListFiles),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(fileUploadSpec(), service.UploadFile),
		pluginbinding.RegisterOperation(infoSpec(), service.Info),
		pluginbinding.RegisterOperation(messageEditSpec(), service.EditMessage),
		pluginbinding.RegisterOperation(messageDeleteSpec(), service.DeleteMessage),
		pluginbinding.RegisterOperation(mentionsSpec(), service.Mentions),
		pluginbinding.RegisterOperation(presenceGetSpec(), service.GetPresence),
		pluginbinding.RegisterOperation(presenceSetSpec(), service.SetPresence),
		pluginbinding.RegisterOperation(messageSendSpec(), service.SendMessage),
		pluginbinding.RegisterOperation(reactionAddSpec(), service.AddReaction),
		pluginbinding.RegisterOperation(reactionRemoveSpec(), service.RemoveReaction),
		pluginbinding.RegisterOperation(searchSpec(), service.Search),
		pluginbinding.RegisterOperation(threadSpec(), service.Thread),
		pluginbinding.RegisterOperation(unreadsSpec(), service.Unreads),
		pluginbinding.RegisterOperation(userListSpec(), service.ListUsers),
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
