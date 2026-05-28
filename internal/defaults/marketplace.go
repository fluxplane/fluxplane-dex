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
        {"use": "gl index", "target": "operation", "operation": "gitlab.index.build"},
        {"use": "gl mr ls", "target": "operation", "operation": "gitlab.mr.list"},
        {"use": "gl mr show <project!iid>", "target": "operation", "operation": "gitlab.mr.show"},
        {"use": "gl proj ls", "target": "operation", "operation": "gitlab.project.list"},
        {"use": "gl proj show <id-or-path>", "target": "operation", "operation": "gitlab.project.show"},
        {"use": "search --plugin gitlab <query>", "target": "datasource", "datasource": "gitlab.projects", "capability": "search", "entity": "gitlab.project"},
        {"use": "lookup --plugin gitlab <text>", "target": "datasource", "datasource": "gitlab.projects", "capability": "lookup"}
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
        {"use": "slack index", "target": "operation", "operation": "slack.index.build"},
        {"use": "slack send <channel> <text>", "target": "operation", "operation": "slack.message.send"},
        {"use": "slack search <query>", "target": "operation", "operation": "slack.search"},
        {"use": "slack thread <channel> <ts>", "target": "operation", "operation": "slack.thread"},
        {"use": "lookup --plugin slack <text>", "target": "datasource", "datasource": "slack.channels", "capability": "lookup"}
      ]
    },
    {
      "name": "system",
      "description": "Local system information across OS, runtime, user, paths, CPU, time, environment, and network categories.",
      "aliases": ["sys", "system"],
      "binary": "dex-plugin-system",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/system/cmd/dex-plugin-system@latest",
      "local_path": "plugins/system",
      "commands": [
        {"use": "sys info", "target": "operation", "operation": "system.info"}
      ]
    },
    {
      "name": "tavily",
      "description": "Tavily web search provider.",
      "aliases": ["tavily"],
      "binary": "dex-plugin-tavily",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/tavily/cmd/dex-plugin-tavily@latest",
      "local_path": "plugins/tavily",
      "commands": [
        {"use": "tavily search <query>", "target": "operation", "operation": "tavily.search"},
        {"use": "search --plugin tavily <query>", "target": "datasource", "datasource": "tavily.web_search", "capability": "search", "entity": "websearch.result"}
      ]
    },
    {
      "name": "duckduckgo",
      "description": "DuckDuckGo web search provider.",
      "aliases": ["ddg", "duckduckgo"],
      "binary": "dex-plugin-duckduckgo",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/duckduckgo/cmd/dex-plugin-duckduckgo@latest",
      "local_path": "plugins/duckduckgo",
      "commands": [
        {"use": "ddg search <query>", "target": "operation", "operation": "duckduckgo.search"},
        {"use": "search --plugin duckduckgo <query>", "target": "datasource", "datasource": "duckduckgo.web_search", "capability": "search", "entity": "websearch.result"}
      ]
    },
    {
      "name": "websearch",
      "description": "Generic web search aggregator over provider plugins.",
      "aliases": ["web", "websearch"],
      "binary": "",
      "commands": [
        {"use": "websearch providers", "target": "operation", "operation": "websearch.provider.list"},
        {"use": "websearch search <query>", "target": "operation", "operation": "websearch.search"},
        {"use": "search --plugin websearch <query>", "target": "datasource", "datasource": "websearch.results", "capability": "search", "entity": "websearch.result"}
      ],
      "metadata": {"kind": "builtin"}
    },
    {
      "name": "prometheus",
      "description": "Prometheus health checks, PromQL queries, labels, targets, and alerts for configured or discovered endpoints.",
      "aliases": ["prom", "prometheus"],
      "binary": "dex-plugin-prometheus",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/prometheus/cmd/dex-plugin-prometheus@latest",
      "local_path": "plugins/prometheus",
      "commands": [
        {"use": "prom test", "target": "operation", "operation": "prometheus.test"},
        {"use": "prom query <promql>", "target": "operation", "operation": "prometheus.query"},
        {"use": "prom query-range <promql>", "target": "operation", "operation": "prometheus.query_range"},
        {"use": "prom labels", "target": "operation", "operation": "prometheus.labels"},
        {"use": "prom targets", "target": "operation", "operation": "prometheus.targets"},
        {"use": "prom alerts", "target": "operation", "operation": "prometheus.alerts"}
      ]
    },
    {
      "name": "loki",
      "description": "Loki health checks, LogQL queries, recent logs, and labels for configured or discovered endpoints.",
      "aliases": ["loki"],
      "binary": "dex-plugin-loki",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/loki/cmd/dex-plugin-loki@latest",
      "local_path": "plugins/loki",
      "commands": [
        {"use": "loki test", "target": "operation", "operation": "loki.test"},
        {"use": "loki query <logql>", "target": "operation", "operation": "loki.query"},
        {"use": "loki labels", "target": "operation", "operation": "loki.labels"},
        {"use": "loki recent", "target": "operation", "operation": "loki.recent_logs"}
      ]
    },
    {
      "name": "kubernetes",
      "description": "Kubernetes cluster discovery using kubeconfig and client-go.",
      "aliases": ["k8s", "kube", "kubernetes"],
      "binary": "dex-plugin-kubernetes",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/kubernetes/cmd/dex-plugin-kubernetes@latest",
      "local_path": "plugins/kubernetes",
      "commands": [
        {"use": "k8s clusters", "target": "operation", "operation": "kubernetes.cluster.list"},
        {"use": "k8s discover <product>", "target": "operation", "operation": "kubernetes.endpoint.discover"},
        {"use": "endpoint discover prometheus", "target": "endpoint", "capability": "discover", "entity": "prometheus"},
        {"use": "endpoint discover loki", "target": "endpoint", "capability": "discover", "entity": "loki"},
        {"use": "endpoint discover mysql", "target": "endpoint", "capability": "discover", "entity": "mysql"}
      ]
    },
    {
      "name": "sql",
      "description": "Read-only SQL query operations for MySQL, PostgreSQL, SQLite, and compatible endpoints.",
      "aliases": ["mysql", "sql"],
      "binary": "dex-plugin-sql",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/sql/cmd/dex-plugin-sql@latest",
      "local_path": "plugins/sql",
      "commands": [
        {"use": "sql query <query>", "target": "operation", "operation": "sql.query"},
        {"use": "mysql query <query>", "target": "operation", "operation": "sql.query"}
      ]
    }
  ]
}`
