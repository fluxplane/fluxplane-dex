# fluxplane-dex

`fluxplane-dex` is a plugin-backed rewrite of `dex`: a standalone CLI for fast,
token-efficient interaction with engineering systems such as GitLab, Slack,
Kubernetes, Prometheus, Loki, Grafana, Jira, and SQL databases.

The core rule is that plugin implementations are usage-agnostic. A plugin does
not know whether it is reached through `dex gl ...`, `dex op run ...`, a local
development path, an installed binary, an embedded Go reference, or a future
Fluxplane integration. The dex host owns routing, auth state, rendering,
marketplace lookup, installation, indexing, and CLI sugar.

## Shape

- `cmd/dex`: Cobra CLI entrypoint.
- `core`: inert manifest, operation, auth, datasource, context, endpoint, and
  marketplace types.
- `protocol`: JSON stdin/stdout plugin protocol.
- `runtime`: marketplace resolution, local state, plugin installation, and
  invocation.
- `plugins/marketplace.json`: default marketplace index.
- `plugins/gitlab`, `plugins/slack`: first external plugin modules.

## Commands

```bash
go run ./cmd/dex plugin ls -o json
go run ./cmd/dex plugin search git
go run ./cmd/dex plugin show gitlab -o json
go run ./cmd/dex plugin remove gitlab
go run ./cmd/dex op ls slack
go run ./cmd/dex auth status gitlab
go run ./cmd/dex auth connect gitlab --field gitlab_url=https://gitlab.example.com --field access_token=...
GITLAB_PERSONAL_TOKEN=... go run ./cmd/dex op batch gitlab '[{"id":"one","name":"gitlab.index.build"}]'
```

Development plugins can be forced with:

```bash
go run ./cmd/dex --dev-plugin gitlab=/path/to/plugins/gitlab plugin show gitlab
```

## Plugin Protocol

The protocol is `dex.plugin.v1`. The host sends one JSON request to plugin
stdin and expects one JSON response on stdout. Commands include:

- `manifest`
- `auth.methods`, `auth.test`, `auth.connect`
- `operations.list`, `operations.call`
- `operations.call_batch`
- `datasources.list`, `datasources.search`, `datasources.get`
- `context.build`
- `endpoints.discover`
- `index.build`, `index.status`

The first implementation supports local `go run` from marketplace `local_path`
entries and installed binaries from `go_install` metadata. The same protocol is
intended to support embedded Go references later without changing plugin code.

## Secrets

`auth` is setup and readiness. Runtime credential material is brokered through
`secret`.

The host creates a short-lived grant before invoking plugin operations. External
plugins receive the grant token and call:

```bash
dex secret get gitlab --instance default --grant <token> --purpose access_token
```

`secret get` rejects missing, expired, wrong-instance, and wrong-purpose grants.
Plugins do not receive dex home paths or read auth/index files directly.

For agents, `dex auth connect` accepts repeated `--field purpose=value` flags.
For humans, omitting `--field` starts an interactive prompt; sensitive fields are
read without echo when stdin is a terminal.

Plugin-specific auth fields come from plugin manifests. GitLab currently
declares `gitlab_url` and `access_token`; Slack declares `bot_token`,
`user_token`, and `app_token`. Runtime code stays generic and does not hardcode
those plugin names or environment variables.
