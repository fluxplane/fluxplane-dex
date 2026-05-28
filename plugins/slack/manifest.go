package slack

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "slack"
	PluginVersion     = "0.3.1"
	PluginDescription = "Slack token info, messaging, search, thread, channel member, and reverse lookup operations."

	AuthMethodTokenSet = "token_set"
	AuthPurposeBot     = "bot_token"
	AuthPurposeUser    = "user_token"
	AuthPurposeApp     = "app_token"

	EnvSlackBotToken  = "SLACK_BOT_TOKEN"
	EnvSlackUserToken = "SLACK_USER_TOKEN"
	EnvSlackAppToken  = "SLACK_APP_TOKEN"

	OperationIndexBuild  = "slack.index.build"
	OperationInfo        = "slack.info"
	OperationMessageSend = "slack.message.send"
	OperationSearch      = "slack.search"
	OperationThread      = "slack.thread"

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
		indexBuildSpec(),
		infoSpec(),
		messageSendSpec(),
		searchSpec(),
		threadSpec(),
	}
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](
		OperationIndexBuild,
		"Build Slack channel and user indexes.",
		slackReadOptions(core.OperationConditional)...,
	)
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NoInput, InfoResult](
		OperationInfo,
		"Show Slack token identity and workspace information.",
		slackReadOptions(core.OperationIdempotent)...,
	)
}

func messageSendSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[MessageSendInput, MessageSendResult](
		OperationMessageSend,
		"Send a Slack message.",
		pluginbinding.SecretPurposes(AuthPurposeBot),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
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
