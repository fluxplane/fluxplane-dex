package slack

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "slack"
	PluginVersion     = "0.1.0"
	PluginDescription = "Slack messaging, search, thread, unread, and reverse lookup operations."

	AuthMethodTokenSet = "token_set"
	AuthPurposeBot     = "bot_token"
	AuthPurposeUser    = "user_token"
	AuthPurposeApp     = "app_token"

	EnvSlackBotToken  = "SLACK_BOT_TOKEN"
	EnvSlackUserToken = "SLACK_USER_TOKEN"
	EnvSlackAppToken  = "SLACK_APP_TOKEN"

	OperationIndexBuild  = "slack.index.build"
	OperationMessageSend = "slack.message.send"
	OperationSearch      = "slack.search"
	OperationThread      = "slack.thread"

	DatasourceChannels = "slack.channels"
	DatasourceUsers    = "slack.users"

	EntityChannel = "slack.channel"
	EntityUser    = "slack.user"

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
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasource(DatasourceChannels, EntityChannel, "Slack channels.", "Slack channel reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
			pluginbinding.IndexedDatasource(DatasourceUsers, EntityUser, "Slack users.", "Slack user reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
		},
		Context: []core.ContextSpec{
			pluginbinding.ContextSpec(ContextName, "Slack context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference),
		},
	}
}

func operationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		indexBuildSpec(),
		messageSendSpec(),
		searchSpec(),
		threadSpec(),
	}
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](OperationIndexBuild, "Build Slack channel and user indexes.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func messageSendSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[MessageSendInput, MessageSendResult](OperationMessageSend, "Send a Slack message.", pluginbinding.SecretPurposes(AuthPurposeBot))
}

func searchSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[SearchInput, SearchResult](OperationSearch, "Search Slack.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func threadSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ThreadInput, ThreadResult](OperationThread, "View a Slack thread.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func slackUsersLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceUsers, EntityUser, "Lookup Slack users.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot))
}

func slackChannelsLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceChannels, EntityChannel, "Lookup Slack channels.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeUser, AuthPurposeBot))
}
