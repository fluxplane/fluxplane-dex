<p align="center">
  <img src="docs/logo.png" alt="fluxplane-dex" width="160" />
</p>

# fluxplane-dex

`fluxplane-dex` is a plugin-backed engineering CLI for humans and AI agents.

It gives engineers and agents one small, scriptable interface to everyday
engineering systems such as GitLab, Slack, Jira, Kubernetes, Loki, Prometheus,
SQL databases, Confluence, GitHub, web search, and local system context.

## Getting Started

Install `dex`:

```bash
go install github.com/fluxplane/fluxplane-dex@latest
```

Then inspect the available plugin surface:

```bash
dex version
dex plugin ls
dex plugin show gitlab
```

Plugin aliases such as `gl`, `slack`, `sys`, `web`, and `kube` are generated
from activated plugin manifests. Activate the plugins you want to expose as
top-level commands:

```bash
dex plugin activate gitlab
dex plugin activate slack
dex plugin activate system
dex plugin activate websearch
```

Connect configured integrations from environment variables:

```bash
dex auth connect auto
dex auth status gitlab
```

Or connect one integration explicitly:

```bash
dex auth connect gitlab \
  --field gitlab_url=https://gitlab.example.com \
  --field access_token=...
```

List operations and run one:

```bash
dex op ls gitlab
dex op run gitlab.project.list '{"limit":10}'
```

Use manifest-driven plugin aliases:

```bash
dex gl index build
dex gl project list
dex gl mr list
dex gl mr show '{"project":"group/project","iid":123}'
```

Aliases are resolved from activated plugin manifests. Operation names map to
words after the plugin alias, so `gitlab.project.list` can be called as
`dex gl project list`.

Build indexes and use datasource search/lookup:

```bash
dex index build gitlab
dex slack info
dex search manager
dex datasource ls
dex datasource show kubernetes.inventory
dex datasource search slack.messages '{"query":"incident"}'
dex datasource search slack.thread_messages '{"channel":"C123","ts":"1710000000.123456"}'
dex datasource search slack.channel_members '{"channel":"C123","query":"timo"}'
dex datasource search sql.query_rows '{"driver":"sqlite","dsn":"./app.db","query":"select 1 as ok"}'
dex lookup "https://gitlab.example.com/group/project/-/merge_requests/123"
dex lookup timo
```

Search the web:

```bash
dex web provider list
dex web search "fluxplane dex"
dex search --plugin websearch "fluxplane dex"
```

Inspect local system context:

```bash
dex sys info
dex sys info --categories '["os","network"]' -o json
```

Check registered endpoint health:

```bash
dex doctor endpoints
```

Use Kubernetes operation aliases and datasource search:

```bash
dex kube service list --endpoint-ref dev-kubernetes --namespace latest
dex kube pod list --endpoint-ref dev-kubernetes --namespace latest --query api
dex kube pod logs latest/api-123 --endpoint-ref dev-kubernetes --since 2h
dex kube portforward start --endpoint-ref dev-kubernetes --namespace monitoring --resource service/loki --remote-port 3100
dex kube container list --endpoint-ref dev-kubernetes --namespace latest --query api
dex kube container show latest/api-123/api --endpoint-ref dev-kubernetes
dex kube deployment list --endpoint-ref dev-kubernetes --namespace latest
dex search --plugin kubernetes --endpoint dev-kubernetes api
```

## Current Status

This project is actively being built. The current surface includes:

- `gitlab`: auth test, index build, project list/show, merge request list/show,
  indexed datasources, and live/index-backed lookup for projects, users,
  groups, issues, and merge requests.
- `slack`: auth test, token identity info, channel/user index build,
  live/index-backed lookup for users and channels, channel/user list, channel
  join, unreads and mark-read including `latest`, presence get/set, message
  send/edit/delete with text, markdown, and Block Kit blocks, reaction
  add/remove, bookmark list/add/edit/delete, custom and built-in emoji list, file
  list/info/upload/download/delete, search with ticket extraction, mentions
  with pending/acked/replied classification, thread operations, and live
  datasource records for messages, thread messages, and channel members.
- `system`: local system information by category.
- `tavily`: authenticated web search provider.
- `duckduckgo`: web search provider without auth.
- `websearch`: builtin generic search aggregator over web search providers.
- `kubernetes`: kubeconfig context discovery, cluster endpoint health probes,
  namespace/service/pod/deployment/container inventory, bounded pod logs, and
  in-cluster endpoint discovery.
- `docker`: local Docker Engine inspection and lifecycle operations for
  containers, images, networks, volumes, contexts, build cache, daemon events,
  and disk usage.
- `prometheus`: endpoint-aware health, query, query range, labels, targets,
  alerts, and datasource records for query results, labels, targets, and alerts.
- `loki`: endpoint-aware health, query, recent logs, labels, and datasource
  records for log entries and labels.
- `sql`: read-only MySQL, PostgreSQL, and SQLite queries through URLs, DSNs, or
  registered endpoint refs, plus datasource records for query rows.
- `ollama`: local Ollama model list/show, running models, generation, chat, and
  embeddings.
- `openai`: OpenAI image generation and model listing.

The broader roadmap is tracked in [.agents/plans/roadmap.md](.agents/plans/roadmap.md).
The endpoint workflow is documented in [docs/endpoints.md](docs/endpoints.md).

## Datasource Records, Search, and Lookup

Datasource records use a common shape:

```json
{
  "entity": "gitlab.project",
  "id": "group/project",
  "title": "Group / Project",
  "url": "https://gitlab.example.com/group/project",
  "links": {
    "self": "https://gitlab.example.com/group/project",
    "namespace_entity": "gitlab.group:group"
  },
  "origin": {
    "source": "host_index",
    "plugin": "gitlab",
    "instance": "default",
    "index": "gitlab.projects"
  },
  "record": {}
}
```

Search returns standardized records. Lookup returns standardized matches with
top-level `entity` and `id`, plus the matched record and source details.
When a plugin operation naturally returns a typed list such as `pods`,
`services`, `contexts`, or `blocks`, dex also adds a generic `records` alias and
`records_source` so agents can normalize list outputs once without knowing each
plugin's plural field name.

Search and lookup fan out only to installed or connected plugins that expose the
requested datasource capability. Host-owned index lookup is used when an index
exists; plugins can also expose live lookup for provider-specific resolution.
Fanout responses separate successful plugin data from unavailable or failed
plugins with `available`, `missing`, and `errors` fields.

## Output Formats

Most read paths support `-o` / `--output`:

```bash
dex plugin ls -o json
dex gl mr ls -o compact
```

Supported formats are currently `text`, `compact`, `json`, and `yaml`.

## How It Works

`dex` is plugin-backed. Plugins expose:

- manifests
- auth methods
- operations
- datasources
- context specs
- endpoint specs
- indexes

The host owns routing, auth state, scoped secret grants, indexes, rendering,
marketplace lookup, installation, and manifest-driven alias dispatch. Plugins
own integration behavior.

The `core/pluginbinding` package owns plugin-facing boilerplate:

- manifest and JSON schema generation
- operation and datasource registration
- secret resolution helpers
- index job helpers
- common datasource record, search, lookup, and get result shapes
- lookup candidate scoring, dedupe, sorting, limiting, and result assembly

This means the same capability can be reached through a manifest-derived alias,
a generic operation call, a datasource search, or an agent workflow.

## Secrets

Plugin auth is host-managed. Plugins declare secret purposes in their manifests;
the host stores material per plugin instance and grants short-lived scoped access
when invoking a plugin.

External plugins fetch secret material through:

```bash
dex secret get <plugin> --instance <name> --grant <token> --purpose <purpose>
```

The grant limits plugin, instance, purpose, and expiry. Plugins should not read
dex state files directly.

`--instance` is a free-form label, not a predeclared registry entry. Use it to
separate auth material, indexes, and runtime calls for different workspaces,
clusters, tenants, or environments. The empty value normalizes to `default`.

## Development and Release Notes

This repository is currently developed as one root module plus separate
installable plugin modules under `plugins/*`. The checked-in `go.work` keeps
local development and tests wired to the local checkout with a workspace-level
replacement for the root release version.

Plugin modules intentionally require the release root module version:

```go
require github.com/fluxplane/fluxplane-dex v0.15.0
```

Do not add local `replace` directives to plugin modules for release. The release
tags must include the root tag `v0.15.0` and matching plugin module tags such as
`plugins/gitlab/v0.15.0`.

Release checks live in [Maintainer Notes](docs/maintainer.md).

## Documentation

- [Vision](docs/vision.md)
- [Concepts](docs/concepts.md)
- [Maintainer Notes](docs/maintainer.md)
- [Roadmap](.agents/plans/roadmap.md)

## History

`fluxplane-dex` builds on lessons from the earlier
[codewandler/dex](https://github.com/codewandler/dex) CLI.
