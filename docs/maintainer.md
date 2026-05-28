# Maintainer Notes

This document is for people developing `fluxplane-dex` itself. End-user usage
belongs in the root [README](../README.md).

## Source Boundary

Do not import code from legacy `dex` or from older plugin implementations in
other repositories. They are reference material only. Port behavior and concepts
into this repository's own pluginbinding/runtime model.

## Run From Source

During development:

```bash
go run ./cmd/dex version
go run ./cmd/dex plugin ls
go run ./cmd/dex plugin show gitlab -o json
go run ./cmd/dex op ls gitlab
```

Use development plugin overrides when testing local plugin paths:

```bash
go run ./cmd/dex --dev-plugin gitlab=plugins/gitlab plugin show gitlab
```

## Tests

Run root tests:

```bash
go test ./...
```

Some plugins are separate Go modules and should be tested from their own
directories when changed:

```bash
cd plugins/gitlab && go test ./...
cd plugins/slack && go test ./...
cd plugins/system && go test ./...
cd plugins/tavily && go test ./...
cd plugins/duckduckgo && go test ./...
```

Before a release candidate, run all of the above plus the smoke checks below.
The root module does not include nested plugin modules in `go test ./...`.

## Local Auth

Connect auth from manifest-declared environment variables:

```bash
go run ./cmd/dex auth connect auto
```

Or save explicit fields:

```bash
go run ./cmd/dex auth connect gitlab \
  --field gitlab_url=https://gitlab.example.com \
  --field access_token=...
```

## Plugin Protocol

The wire protocol is `dex.plugin.v1`: one JSON request on stdin, one JSON
response on stdout.

Supported command families include:

- `manifest`
- `auth.methods`, `auth.test`, `auth.connect`
- `operations.list`, `operations.call`, `operations.call_batch`
- `datasources.list`, `datasources.search`, `datasources.get`,
  `datasources.lookup`
- `context.build`
- `endpoints.discover`
- `index.build`, `index.status`

The host can invoke plugins from marketplace `local_path` entries, installed
plugin binaries, or builtin implementations.

## Release Gate

Run the root suite:

```bash
go test ./...
```

Run every plugin module suite:

```bash
(cd plugins/gitlab && go test ./...)
(cd plugins/slack && go test ./...)
(cd plugins/system && go test ./...)
(cd plugins/tavily && go test ./...)
(cd plugins/duckduckgo && go test ./...)
(cd plugins/docker && go test ./...)
(cd plugins/kubernetes && go test ./...)
(cd plugins/loki && go test ./...)
(cd plugins/ollama && go test ./...)
(cd plugins/openai && go test ./...)
(cd plugins/prometheus && go test ./...)
(cd plugins/sql && go test ./...)
```

Run CLI smokes from the repository root with a temporary home:

```bash
DEX_HOME="$(mktemp -d)"
go run ./cmd/dex --dex-home "$DEX_HOME" version
go run ./cmd/dex --dex-home "$DEX_HOME" plugin ls -o json
go run ./cmd/dex --dex-home "$DEX_HOME" plugin marketplace -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --dev-plugin system=plugins/system plugin show system -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --dev-plugin system=plugins/system plugin activate system -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --dev-plugin system=plugins/system sys info --categories '["os","time"]' -o json
go run ./cmd/dex --dex-home "$DEX_HOME" plugin activate websearch -o json
go run ./cmd/dex --dex-home "$DEX_HOME" web provider list -o json
```

Search, lookup, and index build are covered by CLI tests with fake plugins. For
manual live smokes, use real GitLab/Slack credentials in a throwaway instance:

```bash
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke auth connect auto gitlab
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke index build gitlab -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke search acd -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke lookup timo -o json
```

Slack live smokes require `SLACK_BOT_TOKEN` and/or `SLACK_USER_TOKEN` or stored
Slack auth fields on the throwaway instance. Use a non-critical workspace and a
scratch channel:

```bash
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke auth connect auto slack
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke op run slack.auth.test -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack info -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke index build slack -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack user list --query timo --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack channel list --query general --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack search "incident" --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack search "DEV-" --tickets --ticket-keys '["DEV"]' --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack mentions --user U123 --tickets --ticket-keys '["DEV"]' --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack mentions --user U123 --unhandled --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack unreads --channel C123 --since 1d --limit 10 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack thread --channel C123 --ts 1710000000.123456 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke datasource search slack.channel_members '{"channel":"C123","query":"timo","limit":5}' -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack emoji list --query check --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack emoji list --mode builtin --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack emoji list --mode all --include-aliases --query thumb --limit 10 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack bookmark list --channel C123 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack bookmark add --channel C123 --title "dex smoke bookmark" --link https://example.com --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack bookmark edit --channel C123 --bookmark-id B123 --title "dex smoke bookmark edited" --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack bookmark delete --channel C123 --bookmark-id B123 --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack file list --channel C123 --limit 5 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack file info --file-id F123 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack file download --file-id F123 --output-path /tmp/dex-slack-smoke-download --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack download --file-id F123 --output-path /tmp/dex-slack-smoke-download-top-level --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack file delete --file-id F123 --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack presence get --user U123 -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack presence set --presence auto --role user -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack channel join --channel C123 --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack message send --channel C123 --text "dex live smoke" --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack message send --channel C123 --markdown "*dex* markdown smoke" --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack message send --channel C123 --text "dex block fallback" --blocks '[{"type":"section","text":{"type":"mrkdwn","text":"*dex* block smoke"}}]' --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack message edit --channel C123 --ts 1710000000.123456 --text "dex live smoke edited" --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack reaction add --channel C123 --ts 1710000000.123456 --emoji white_check_mark --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack reaction remove --channel C123 --ts 1710000000.123456 --emoji white_check_mark --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack channel mark-read --channel C123 --ts 1710000000.123456 --role user -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack channel mark-read --channel C123 --ts latest --role user -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack message delete --channel C123 --ts 1710000000.123456 --role bot -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke slack file upload --channel C123 --file-path ./README.md --initial-comment "dex upload smoke" --role bot -o json
```

Do not commit, tag, or push from an agent session unless explicitly asked.

## Module Packaging

The repository uses one root module and separate installable plugin modules
under `plugins/*`.

The checked-in `go.work` is the development bridge. It lets local commands use
the root checkout while plugin module `go.mod` files stay release-ready. The
workspace contains the only local replacement:

```go
replace github.com/fluxplane/fluxplane-dex v0.5.0 => .
```

Plugin modules should require the root release version and must not carry local
`replace` directives:

```go
require github.com/fluxplane/fluxplane-dex v0.5.0
```

Release packaging should use this sequence:

1. Determine the previous root tag with `git describe --tags --match 'v*'`.
2. Derive the next semantic version from the diff since that tag.
3. Update `CHANGELOG.md` with clean, deduped notes based on `git log` and
   `git diff --stat <previous-tag>..HEAD`.
4. Update release version references in `go.work`, plugin module requirements,
   plugin manifest versions, and docs.
5. Commit the release-ready tree.
6. Tag the root module version, for example `v0.5.0`.
7. Tag plugin modules at matching versions so marketplace `go_install` targets
   resolve through the public module proxy.
8. Push the branch and all release tags.
9. Create a GitHub release with release notes derived from the changelog.
10. Verify `go install` for the root CLI and each plugin command from outside
   the repository.

Use module-scoped tags for plugin modules:

```bash
git tag v0.5.0
git tag plugins/gitlab/v0.5.0
git tag plugins/slack/v0.5.0
git tag plugins/system/v0.5.0
git tag plugins/tavily/v0.5.0
git tag plugins/duckduckgo/v0.5.0
git tag plugins/docker/v0.5.0
git tag plugins/grafana/v0.5.0
git tag plugins/kubernetes/v0.5.0
git tag plugins/loki/v0.5.0
git tag plugins/ollama/v0.5.0
git tag plugins/openai/v0.5.0
git tag plugins/prometheus/v0.5.0
git tag plugins/sql/v0.5.0
```

The corresponding install checks are:

```bash
go install github.com/fluxplane/fluxplane-dex/cmd/dex@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd/dex-plugin-gitlab@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/slack/cmd/dex-plugin-slack@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/system/cmd/dex-plugin-system@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/tavily/cmd/dex-plugin-tavily@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/duckduckgo/cmd/dex-plugin-duckduckgo@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/docker/cmd/dex-plugin-docker@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/grafana/cmd/dex-plugin-grafana@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/kubernetes/cmd/dex-plugin-kubernetes@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/loki/cmd/dex-plugin-loki@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/ollama/cmd/dex-plugin-ollama@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/openai/cmd/dex-plugin-openai@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/prometheus/cmd/dex-plugin-prometheus@v0.5.0
go install github.com/fluxplane/fluxplane-dex/plugins/sql/cmd/dex-plugin-sql@v0.5.0
```
