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

Use a shortcut command:

```bash
dex gl index
dex gl proj ls
dex gl mr ls
dex gl mr show group/project!123
```

Shortcuts are marketplace bindings over operations or datasources. The same
underlying capability remains available through `dex op run`, `dex search`, or
`dex lookup`.

Build indexes and use datasource search/lookup:

```bash
dex index build gitlab
dex search manager
dex lookup "https://gitlab.example.com/group/project/-/merge_requests/123"
dex lookup timo
```

Search the web:

```bash
dex websearch providers
dex websearch search "fluxplane dex"
dex search --plugin websearch "fluxplane dex"
```

Inspect local system context:

```bash
dex sys info
dex sys info --category os --category network -o json
```

Check registered endpoint health:

```bash
dex doctor endpoints
```

Use Kubernetes inventory shortcuts:

```bash
dex kube svc ls --endpoint dev-kubernetes --namespace latest
dex kube pod ls --endpoint dev-kubernetes --namespace latest --query api
dex kube pod logs latest/api-123 --endpoint dev-kubernetes --tail-lines 50
dex kube container ls --endpoint dev-kubernetes --namespace latest --query api
dex kube container show latest/api-123/api --endpoint dev-kubernetes
dex kube deploy ls --endpoint dev-kubernetes --namespace latest
dex search --plugin kubernetes --endpoint dev-kubernetes --namespace latest api
```

## Current Status

This project is actively being built. The current surface includes:

- `gitlab`: auth test, index build, project list/show, merge request list/show,
  indexed datasources, and live/index-backed lookup for projects, users,
  groups, issues, and merge requests.
- `slack`: channel/user index build and live/index-backed lookup for users and
  channels; send/search/thread are declared but still pending live-client
  migration.
- `system`: local system information by category.
- `tavily`: authenticated web search provider.
- `duckduckgo`: web search provider without auth.
- `websearch`: builtin generic search aggregator over web search providers.
- `kubernetes`: kubeconfig context discovery, cluster endpoint health probes,
  namespace/service/pod/deployment/container inventory, bounded pod logs,
  executable inventory shortcuts, and
  in-cluster endpoint discovery.
- `sql`: read-only MySQL, PostgreSQL, and SQLite queries through URLs, DSNs, or
  registered endpoint refs.

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

Search and lookup fan out only to installed or connected plugins that expose the
requested datasource capability. Host-owned index lookup is used when an index
exists; plugins can also expose live lookup for provider-specific resolution.

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
marketplace lookup, installation, and CLI shortcuts. Plugins own integration
behavior.

The `core/pluginbinding` package owns plugin-facing boilerplate:

- manifest and JSON schema generation
- operation and datasource registration
- secret resolution helpers
- index job helpers
- common datasource record, search, lookup, and get result shapes
- lookup candidate scoring, dedupe, sorting, limiting, and result assembly

This means the same capability can be reached through a human-friendly shortcut,
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

## Development and Release Notes

This repository is currently developed as one root module plus separate
installable plugin modules under `plugins/*`. The checked-in `go.work` keeps
local development and tests wired to the local checkout with a workspace-level
replacement for the root release version.

Plugin modules intentionally require the release root module version:

```go
require github.com/fluxplane/fluxplane-dex v0.1.0
```

Do not add local `replace` directives to plugin modules for release. The release
tags must include the root tag `v0.1.0` and matching plugin module tags such as
`plugins/gitlab/v0.1.0`.

Release checks live in [Maintainer Notes](docs/maintainer.md).

## Documentation

- [Vision](docs/vision.md)
- [Concepts](docs/concepts.md)
- [Maintainer Notes](docs/maintainer.md)
- [Roadmap](.agents/plans/roadmap.md)

## History

`fluxplane-dex` builds on lessons from the earlier
[codewandler/dex](https://github.com/codewandler/dex) CLI.
