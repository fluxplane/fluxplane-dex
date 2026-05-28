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
      "name": "docker",
      "description": "Local Docker Engine inspection for containers, images, networks, volumes, and daemon info.",
      "aliases": ["dock", "docker"],
      "binary": "dex-plugin-docker",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/docker/cmd/dex-plugin-docker@latest",
      "local_path": "plugins/docker",
      "commands": [
        {"use": "docker info", "target": "operation", "operation": "docker.info"},
        {"use": "docker ps", "target": "operation", "operation": "docker.container.list"},
        {"use": "docker container show <id-or-name>", "target": "operation", "operation": "docker.container.show"},
        {"use": "docker logs <id-or-name>", "target": "operation", "operation": "docker.container.logs"},
        {"use": "docker stats <id-or-name>", "target": "operation", "operation": "docker.container.stats"},
        {"use": "docker top <id-or-name>", "target": "operation", "operation": "docker.container.top"},
        {"use": "docker exec <id-or-name> <cmd>", "target": "operation", "operation": "docker.container.exec"},
        {"use": "docker cp <id-or-name>:<path> <local-dir>", "target": "operation", "operation": "docker.container.copy_from"},
        {"use": "docker cp <local-path> <id-or-name>:<path>", "target": "operation", "operation": "docker.container.copy_to"},
        {"use": "docker create <image>", "target": "operation", "operation": "docker.container.create"},
        {"use": "docker run <image>", "target": "operation", "operation": "docker.container.run"},
        {"use": "docker start <id-or-name>", "target": "operation", "operation": "docker.container.start"},
        {"use": "docker stop <id-or-name>", "target": "operation", "operation": "docker.container.stop"},
        {"use": "docker restart <id-or-name>", "target": "operation", "operation": "docker.container.restart"},
        {"use": "docker rm <id-or-name>", "target": "operation", "operation": "docker.container.remove"},
        {"use": "docker container prune", "target": "operation", "operation": "docker.container.prune"},
        {"use": "docker container inspect-raw <id-or-name>", "target": "operation", "operation": "docker.container.inspect.raw"},
        {"use": "docker images", "target": "operation", "operation": "docker.image.list"},
        {"use": "docker image show <id-or-ref>", "target": "operation", "operation": "docker.image.show"},
        {"use": "docker pull <reference>", "target": "operation", "operation": "docker.image.pull"},
        {"use": "docker tag <source> <target>", "target": "operation", "operation": "docker.image.tag"},
        {"use": "docker push <reference>", "target": "operation", "operation": "docker.image.push"},
        {"use": "docker build <context>", "target": "operation", "operation": "docker.image.build"},
        {"use": "docker rmi <id-or-ref>", "target": "operation", "operation": "docker.image.remove"},
        {"use": "docker image prune", "target": "operation", "operation": "docker.image.prune"},
        {"use": "docker image inspect-raw <id-or-ref>", "target": "operation", "operation": "docker.image.inspect.raw"},
        {"use": "docker networks", "target": "operation", "operation": "docker.network.list"},
        {"use": "docker network create <name>", "target": "operation", "operation": "docker.network.create"},
        {"use": "docker network rm <id-or-name>", "target": "operation", "operation": "docker.network.remove"},
        {"use": "docker network prune", "target": "operation", "operation": "docker.network.prune"},
        {"use": "docker network inspect-raw <id-or-name>", "target": "operation", "operation": "docker.network.inspect.raw"},
        {"use": "docker system df", "target": "operation", "operation": "docker.system.df"},
        {"use": "docker system prune", "target": "operation", "operation": "docker.system.prune"},
        {"use": "docker events", "target": "operation", "operation": "docker.events"},
        {"use": "docker volumes", "target": "operation", "operation": "docker.volume.list"},
        {"use": "docker volume create <name>", "target": "operation", "operation": "docker.volume.create"},
        {"use": "docker volume rm <name>", "target": "operation", "operation": "docker.volume.remove"},
        {"use": "docker volume prune", "target": "operation", "operation": "docker.volume.prune"},
        {"use": "docker volume inspect-raw <name>", "target": "operation", "operation": "docker.volume.inspect.raw"},
        {"use": "docker build-cache prune", "target": "operation", "operation": "docker.build_cache.prune"},
        {"use": "docker context ls", "target": "operation", "operation": "docker.context.list"},
        {"use": "docker context show <name>", "target": "operation", "operation": "docker.context.show"},
        {"use": "lookup --plugin docker <text>", "target": "datasource", "datasource": "docker.containers", "capability": "lookup"}
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
        {"use": "kube ns ls", "target": "operation", "operation": "kubernetes.namespace.list"},
        {"use": "kube svc ls", "target": "operation", "operation": "kubernetes.service.list"},
        {"use": "kube svc show <namespace/name>", "target": "operation", "operation": "kubernetes.service.show"},
        {"use": "kube pod ls", "target": "operation", "operation": "kubernetes.pod.list"},
        {"use": "kube pod show <namespace/name>", "target": "operation", "operation": "kubernetes.pod.show"},
        {"use": "kube pod logs <namespace/name>", "target": "operation", "operation": "kubernetes.pod.logs"},
        {"use": "kube container ls", "target": "operation", "operation": "kubernetes.container.list"},
        {"use": "kube container show <namespace/pod/container>", "target": "operation", "operation": "kubernetes.container.show"},
        {"use": "kube deploy ls", "target": "operation", "operation": "kubernetes.deployment.list"},
        {"use": "kube deploy show <namespace/name>", "target": "operation", "operation": "kubernetes.deployment.show"},
        {"use": "search --plugin kubernetes <query>", "target": "datasource", "datasource": "kubernetes.inventory", "capability": "search", "entity": "kubernetes.resource"},
        {"use": "k8s discover <product>", "target": "operation", "operation": "kubernetes.endpoint.discover"},
        {"use": "endpoint discover kubernetes", "target": "endpoint", "capability": "discover", "entity": "kubernetes"},
        {"use": "endpoint discover prometheus", "target": "endpoint", "capability": "discover", "entity": "prometheus"},
        {"use": "endpoint discover loki", "target": "endpoint", "capability": "discover", "entity": "loki"},
        {"use": "endpoint discover mysql", "target": "endpoint", "capability": "discover", "entity": "mysql"},
        {"use": "endpoint discover postgres", "target": "endpoint", "capability": "discover", "entity": "postgres"}
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
    },
    {
      "name": "ollama",
      "description": "Ollama local LLM operations: inspect installed models, generate completions, chat, and embed.",
      "aliases": ["ol", "ollama"],
      "binary": "dex-plugin-ollama",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/ollama/cmd/dex-plugin-ollama@latest",
      "local_path": "plugins/ollama",
      "commands": [
        {"use": "ollama info", "target": "operation", "operation": "ollama.info"},
        {"use": "ollama models", "target": "operation", "operation": "ollama.model.list"},
        {"use": "ollama model show <name>", "target": "operation", "operation": "ollama.model.show"},
        {"use": "ollama ps", "target": "operation", "operation": "ollama.ps"},
        {"use": "ollama generate <model> <prompt>", "target": "operation", "operation": "ollama.generate"},
        {"use": "ollama chat <model>", "target": "operation", "operation": "ollama.chat"},
        {"use": "ollama embed <model> <text>", "target": "operation", "operation": "ollama.embed"},
        {"use": "search --plugin ollama <query>", "target": "datasource", "datasource": "ollama.models", "capability": "search", "entity": "ollama.model"},
        {"use": "lookup --plugin ollama <text>", "target": "datasource", "datasource": "ollama.models", "capability": "lookup"}
      ]
    },
    {
      "name": "openai",
      "description": "OpenAI API plugin. Currently exposes image generation and model listing.",
      "aliases": ["oai", "openai"],
      "binary": "dex-plugin-openai",
      "go_install": "github.com/fluxplane/fluxplane-dex/plugins/openai/cmd/dex-plugin-openai@latest",
      "local_path": "plugins/openai",
      "commands": [
        {"use": "openai image generate <prompt>", "target": "operation", "operation": "openai.image.generate"},
        {"use": "openai models", "target": "operation", "operation": "openai.model.list"}
      ]
    }
  ]
}`
