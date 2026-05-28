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
```

Run CLI smokes from the repository root with a temporary home:

```bash
DEX_HOME="$(mktemp -d)"
go run ./cmd/dex --dex-home "$DEX_HOME" version
go run ./cmd/dex --dex-home "$DEX_HOME" plugin ls -o json
go run ./cmd/dex --dex-home "$DEX_HOME" plugin marketplace -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --dev-plugin system=plugins/system plugin show system -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --dev-plugin system=plugins/system sys info --category os --category time -o json
go run ./cmd/dex --dex-home "$DEX_HOME" websearch providers -o json
```

Search, lookup, and index build are covered by CLI tests with fake plugins. For
manual live smokes, use real GitLab/Slack credentials in a throwaway instance:

```bash
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke auth connect auto gitlab
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke index build gitlab -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke search acd -o json
go run ./cmd/dex --dex-home "$DEX_HOME" --instance release-smoke lookup timo -o json
```

Do not commit, tag, or push from an agent session unless explicitly asked.

## Module Packaging

The repository uses one root module and separate installable plugin modules
under `plugins/*`.

The checked-in `go.work` is the development bridge. It lets local commands use
the root checkout while plugin module `go.mod` files stay release-ready. The
workspace contains the only local replacement:

```go
replace github.com/fluxplane/fluxplane-dex v0.1.0 => .
```

Plugin modules should require the root release version and must not carry local
`replace` directives:

```go
require github.com/fluxplane/fluxplane-dex v0.1.0
```

Release packaging should use this sequence:

1. Commit the release-ready tree.
2. Tag the root module version, for example `v0.1.0`.
3. Tag plugin modules at matching versions so marketplace `go_install` targets
   resolve through the public module proxy.
4. Push the branch and all release tags.
5. Verify `go install` for the root CLI and each plugin command from outside
   the repository.

Use module-scoped tags for plugin modules:

```bash
git tag v0.1.0
git tag plugins/gitlab/v0.1.0
git tag plugins/slack/v0.1.0
git tag plugins/system/v0.1.0
git tag plugins/tavily/v0.1.0
git tag plugins/duckduckgo/v0.1.0
```

The corresponding install checks are:

```bash
go install github.com/fluxplane/fluxplane-dex/cmd/dex@v0.1.0
go install github.com/fluxplane/fluxplane-dex/plugins/gitlab/cmd/dex-plugin-gitlab@v0.1.0
go install github.com/fluxplane/fluxplane-dex/plugins/slack/cmd/dex-plugin-slack@v0.1.0
go install github.com/fluxplane/fluxplane-dex/plugins/system/cmd/dex-plugin-system@v0.1.0
go install github.com/fluxplane/fluxplane-dex/plugins/tavily/cmd/dex-plugin-tavily@v0.1.0
go install github.com/fluxplane/fluxplane-dex/plugins/duckduckgo/cmd/dex-plugin-duckduckgo@v0.1.0
```
