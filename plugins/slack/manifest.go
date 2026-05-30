package slack

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "slack"
	PluginVersion     = "0.18.2"
	PluginDescription = "Slack token info, messaging, file upload, search, thread, channel member, and reverse lookup operations."

	AuthMethodTokenSet = "token_set"
	AuthPurposeBot     = "bot_token"
	AuthPurposeUser    = "user_token"
	AuthPurposeApp     = "app_token"

	EnvSlackBotToken  = "SLACK_BOT_TOKEN"
	EnvSlackUserToken = "SLACK_USER_TOKEN"
	EnvSlackAppToken  = "SLACK_APP_TOKEN"

	OperationAuthTest       = "slack.auth.test"
	OperationBookmarkAdd    = "slack.bookmark.add"
	OperationBookmarkEdit   = "slack.bookmark.edit"
	OperationBookmarkDelete = "slack.bookmark.delete"
	OperationBookmarkList   = "slack.bookmark.list"
	OperationChannelJoin    = "slack.channel.join"
	OperationChannelList    = "slack.channel.list"
	OperationChannelMark    = "slack.channel.mark-read"
	OperationDownload       = "slack.download"
	OperationEmojiList      = "slack.emoji.list"
	OperationFileDelete     = "slack.file.delete"
	OperationFileDownload   = "slack.file.download"
	OperationFileInfo       = "slack.file.info"
	OperationFileList       = "slack.file.list"
	OperationIndexBuild     = "slack.index.build"
	OperationFileUpload     = "slack.file.upload"
	OperationInfo           = "slack.info"
	OperationMessageEdit    = "slack.message.edit"
	OperationMessageDelete  = "slack.message.delete"
	OperationMentions       = "slack.mentions"
	OperationPresenceGet    = "slack.presence.get"
	OperationPresenceSet    = "slack.presence.set"
	OperationMessageSend    = "slack.message.send"
	OperationReactionAdd    = "slack.reaction.add"
	OperationReactionRemove = "slack.reaction.remove"
	OperationSearch         = "slack.search"
	OperationThread         = "slack.thread"
	OperationUnreads        = "slack.unreads"
	OperationUserList       = "slack.user.list"

	DatasourceChannels       = "slack.channels"
	DatasourceUsers          = "slack.users"
	DatasourceMessages       = "slack.messages"
	DatasourceThreadMessages = "slack.thread_messages"
	DatasourceChannelMembers = "slack.channel_members"

	EntityChannel       = "slack.channel"
	EntityUser          = "slack.user"
	EntityMessage       = "slack.message"
	EntityThreadMessage = "slack.thread_message"
	EntityChannelMember = "slack.channel_member"

	ContextName = "slack.context"
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
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodTokenSet,
			"Slack tokens resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeBot, "Slack bot token", true, true, EnvSlackBotToken),
			pluginbinding.AuthField(AuthPurposeUser, "Slack user token", true, true, EnvSlackUserToken),
			pluginbinding.AuthField(AuthPurposeApp, "Slack app token", false, true, EnvSlackAppToken),
		)},
		Operations: operationSpecs(),
		Datasources: []core.DatasourceSpec{
			slackMessagesDatasourceSpec(),
			slackThreadMessagesDatasourceSpec(),
			slackChannelMembersDatasourceSpec(),
		},
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasourceWithOptions(DatasourceChannels, EntityChannel, "Slack channels.", "Slack channel reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[ChannelRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceUsers, EntityUser, "Slack users.", "Slack user reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[UserRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
		},
		Context: []core.ContextSpec{
			pluginbinding.ContextSpec(ContextName, "Slack context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference),
		},
	}
}

func operationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		authTestSpec(),
		bookmarkAddSpec(),
		bookmarkEditSpec(),
		bookmarkDeleteSpec(),
		bookmarkListSpec(),
		channelJoinSpec(),
		channelListSpec(),
		channelMarkSpec(),
		downloadSpec(),
		emojiListSpec(),
		fileDeleteSpec(),
		fileDownloadSpec(),
		fileInfoSpec(),
		fileListSpec(),
		indexBuildSpec(),
		fileUploadSpec(),
		infoSpec(),
		messageEditSpec(),
		messageDeleteSpec(),
		mentionsSpec(),
		presenceGetSpec(),
		presenceSetSpec(),
		messageSendSpec(),
		reactionAddSpec(),
		reactionRemoveSpec(),
		searchSpec(),
		threadSpec(),
		unreadsSpec(),
		userListSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NoInput, AuthTestResult](
		OperationAuthTest,
		"Test Slack user and bot token authentication.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](
		OperationIndexBuild,
		"Build Slack channel and user indexes.",
		slackReadOptions(core.OperationConditional)...,
	)
}

func bookmarkAddSpec() core.OperationSpec {
	return slackWriteOperation[BookmarkAddInput, BookmarkResult](OperationBookmarkAdd, "Add a Slack channel bookmark.")
}

func bookmarkEditSpec() core.OperationSpec {
	return slackWriteOperation[BookmarkEditInput, BookmarkResult](OperationBookmarkEdit, "Edit a Slack channel bookmark.")
}

func bookmarkDeleteSpec() core.OperationSpec {
	return slackWriteOperation[BookmarkDeleteInput, BookmarkDeleteResult](OperationBookmarkDelete, "Delete a Slack channel bookmark.")
}

func bookmarkListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[BookmarkListInput, BookmarkListResult](
		OperationBookmarkList,
		"List Slack channel bookmarks.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func channelJoinSpec() core.OperationSpec {
	return slackWriteOperation[ChannelJoinInput, ChannelJoinResult](OperationChannelJoin, "Join a Slack channel.")
}

func channelListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ChannelListInput, ChannelListResult](
		OperationChannelList,
		"List Slack channels.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func channelMarkSpec() core.OperationSpec {
	return slackWriteOperation[ChannelMarkInput, ChannelMarkResult](OperationChannelMark, "Mark a Slack channel read through a timestamp.")
}

func downloadSpec() core.OperationSpec {
	return slackFileDownloadOperation(OperationDownload, "Download a Slack file to a host blob.")
}

func emojiListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[EmojiListInput, EmojiListResult](
		OperationEmojiList,
		"List Slack emoji.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func fileDeleteSpec() core.OperationSpec {
	return slackWriteOperation[FileDeleteInput, FileDeleteResult](OperationFileDelete, "Delete a Slack file.")
}

func fileDownloadSpec() core.OperationSpec {
	return slackFileDownloadOperation(OperationFileDownload, "Download a Slack file to a host blob.")
}

func fileInfoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FileInfoInput, FileInfoResult](
		OperationFileInfo,
		"Show Slack file information.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func fileListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FileListInput, FileListResult](
		OperationFileList,
		"List Slack files.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func fileUploadSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FileUploadInput, FileUploadResult](
		OperationFileUpload,
		"Upload a file or image to a Slack channel, DM, or thread.",
		pluginbinding.SecretPurposes(AuthPurposeBot, AuthPurposeUser),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func messageEditSpec() core.OperationSpec {
	return slackWriteOperation[MessageEditInput, MessageEditResult](OperationMessageEdit, "Edit a Slack message.")
}

func messageDeleteSpec() core.OperationSpec {
	return slackWriteOperation[MessageDeleteInput, MessageDeleteResult](OperationMessageDelete, "Delete a Slack message.")
}

func mentionsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[MentionsInput, MentionsResult](
		OperationMentions,
		"Search Slack mentions and classify handling status.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func presenceGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PresenceGetInput, PresenceGetResult](
		OperationPresenceGet,
		"Get Slack user presence.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func presenceSetSpec() core.OperationSpec {
	return slackWriteOperation[PresenceSetInput, PresenceSetResult](OperationPresenceSet, "Set Slack user presence.")
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NoInput, InfoResult](
		OperationInfo,
		"Show Slack token identity and workspace information.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func messageSendSpec() core.OperationSpec {
	return slackWriteOperation[MessageSendInput, MessageSendResult](OperationMessageSend, "Send a Slack message.")
}

func reactionAddSpec() core.OperationSpec {
	return slackWriteOperation[ReactionAddInput, ReactionAddResult](OperationReactionAdd, "Add a reaction to a Slack message.")
}

func reactionRemoveSpec() core.OperationSpec {
	return slackWriteOperation[ReactionAddInput, ReactionAddResult](OperationReactionRemove, "Remove a reaction from a Slack message.")
}

func slackWriteOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](
		name,
		description,
		pluginbinding.SecretPurposes(AuthPurposeBot, AuthPurposeUser),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func slackFileDownloadOperation(name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FileDownloadInput, FileDownloadResult](
		name,
		description,
		pluginbinding.SecretPurposes(AuthPurposeBot, AuthPurposeUser),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func searchSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[SearchInput, SearchResult](OperationSearch, "Search Slack.", slackReadOptions(core.OperationIdempotent)...)
}

func threadSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ThreadInput, ThreadResult](OperationThread, "View a Slack thread.", slackReadOptions(core.OperationIdempotent)...)
}

func unreadsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[UnreadsInput, UnreadsResult](
		OperationUnreads,
		"List Slack channels with unread messages.",
		slackUserReadOptions(core.OperationIdempotent)...,
	)
}

func userListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[UserListInput, UserListResult](
		OperationUserList,
		"List Slack users.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func slackUserReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUser),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func slackReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUser, AuthPurposeBot),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func slackUsersLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceUsers, EntityUser, "Lookup Slack users.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func slackChannelsLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceChannels, EntityChannel, "Lookup Slack channels.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func slackMessagesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[MessageSearchInput, MessageDatasourceResult](
		DatasourceMessages,
		EntityMessage,
		"Search Slack messages.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot),
		pluginbinding.EntitySchemaFor[MessageRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "message_id", TitleField: "title"}),
		pluginbinding.Completion("Slack message fields.", "channel", "user", "text"),
	)
}

func slackThreadMessagesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[ThreadMessagesInput, ThreadMessagesDatasourceResult](
		DatasourceThreadMessages,
		EntityThreadMessage,
		"Read Slack thread messages.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot),
		pluginbinding.EntitySchemaFor[ThreadMessageRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "thread_message_id", TitleField: "title"}),
		pluginbinding.Completion("Slack thread message fields.", "channel", "root_ts", "reply_ts", "user", "text"),
	)
}

func slackChannelMembersDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[ChannelMembersInput, ChannelMembersDatasourceResult](
		DatasourceChannelMembers,
		EntityChannelMember,
		"List Slack channel members.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot),
		pluginbinding.EntitySchemaFor[ChannelMemberRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "channel_member_id", TitleField: "title"}),
		pluginbinding.Completion("Slack channel member fields.", "channel", "user_id", "name", "real_name", "display_name", "email"),
	)
}
