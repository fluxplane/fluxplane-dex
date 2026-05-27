package defaults

const MarketplaceJSON = `{
  "version": "1",
  "plugins": [
    {
      "name": "gitlab",
      "description": "GitLab operations, datasources, indexes, and reverse lookups.",
      "aliases": ["gl", "gitlab"],
      "binary": "dex-plugin-gitlab",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd/dex-plugin-gitlab@latest",
      "local_path": "plugins/gitlab",
      "commands": [
        {"use": "gl index", "operation": "gitlab.index.build"},
        {"use": "gl mr ls", "operation": "gitlab.mr.list"},
        {"use": "gl mr show <project!iid>", "operation": "gitlab.mr.show"},
        {"use": "gl proj ls", "operation": "gitlab.project.list"},
        {"use": "gl proj show <id-or-path>", "operation": "gitlab.project.show"}
      ]
    },
    {
      "name": "slack",
      "description": "Slack messaging, search, thread, unread, and reverse lookup operations.",
      "aliases": ["slack"],
      "binary": "dex-plugin-slack",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/slack/cmd/dex-plugin-slack@latest",
      "local_path": "plugins/slack",
      "commands": [
        {"use": "slack index", "operation": "slack.index.build"},
        {"use": "slack send <channel> <text>", "operation": "slack.message.send"},
        {"use": "slack search <query>", "operation": "slack.search"},
        {"use": "slack thread <channel> <ts>", "operation": "slack.thread"}
      ]
    }
  ]
}`
