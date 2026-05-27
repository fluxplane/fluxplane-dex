package slack

import "github.com/fluxplane/fluxplane-dex/core"

const PluginName = "slack"

func Manifest() core.PluginManifest {
	return core.PluginManifest{
		Name:        PluginName,
		Version:     "0.1.0",
		Description: "Slack messaging, search, thread, unread, and reverse lookup operations.",
		Aliases:     []string{"slack"},
		Auth: []core.AuthMethod{{
			Name:        "bot_token",
			Kind:        "bearer_token",
			Description: "Slack tokens resolved by dex secret broker.",
			Env:         []string{"SLACK_BOT_TOKEN", "SLACK_USER_TOKEN", "SLACK_APP_TOKEN"},
			Fields: []core.AuthField{
				{Name: "bot_token", Required: true, Sensitive: true, Secret: true, Description: "Slack bot token", Env: []string{"SLACK_BOT_TOKEN"}},
				{Name: "user_token", Required: true, Sensitive: true, Secret: true, Description: "Slack user token", Env: []string{"SLACK_USER_TOKEN"}},
				{Name: "app_token", Required: false, Sensitive: true, Secret: true, Description: "Slack app token", Env: []string{"SLACK_APP_TOKEN"}},
			},
		}},
		Operations: []core.OperationSpec{
			{Name: "slack.index.build", Description: "Build Slack channel and user indexes.", ReadOnly: true, SecretPurposes: []string{"user_token", "bot_token"}},
			{Name: "slack.message.send", Description: "Send a Slack message.", SecretPurposes: []string{"bot_token"}},
			{Name: "slack.search", Description: "Search Slack.", ReadOnly: true, SecretPurposes: []string{"user_token", "bot_token"}},
			{Name: "slack.thread", Description: "View a Slack thread.", ReadOnly: true, SecretPurposes: []string{"user_token", "bot_token"}},
		},
		Datasources: []core.DatasourceSpec{
			{Name: "slack.channels", Entity: "slack.channel", Description: "Slack channels.", Capabilities: []string{"search", "lookup", "get", "index"}},
			{Name: "slack.users", Entity: "slack.user", Description: "Slack users.", Capabilities: []string{"search", "lookup", "get", "index"}},
		},
		Context: []core.ContextSpec{{Name: "slack.context", Description: "Slack context blocks.", Kinds: []string{"text", "reference"}}},
		Indexes: []core.IndexSpec{
			{Name: "slack.users", Description: "Slack user reverse lookup index.", Entities: []string{"slack.user"}},
			{Name: "slack.channels", Description: "Slack channel reverse lookup index.", Entities: []string{"slack.channel"}},
		},
	}
}
