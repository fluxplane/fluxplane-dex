# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-05-28

### Added
- Added GitLab branch write operations `gitlab.branch.create`,
  `gitlab.branch.delete`, and `gitlab.branch.delete_merged`.
- Added GitLab repository file write operations
  `gitlab.repository.file.create`, `gitlab.repository.file.update`, and
  `gitlab.repository.file.delete`.
- Added GitLab multi-file commit operation
  `gitlab.repository.commit.create` supporting `create`/`update`/`delete`/
  `move`/`chmod` actions in a single commit.
- Added GitLab project CI/CD variable write operations
  `gitlab.ci.variable.create`, `gitlab.ci.variable.update`, and
  `gitlab.ci.variable.delete` with environment scope, masked/protected/raw
  flags, and `env_var`/`file` variable types.
- Added GitLab CI pipeline write operations `gitlab.pipeline.create`,
  `gitlab.pipeline.retry`, and `gitlab.pipeline.cancel`, including typed
  pipeline variables on create.
- Added GitLab personal snippet write operations `gitlab.snippet.create`
  and `gitlab.snippet.delete` with multi-file snippet bodies and
  private/internal/public visibility.

### Changed
- Extended the GitLab plugin's risk model with high-risk write and
  destructive operation helpers so CI variable mutations and pipeline
  control surface as high risk while delete operations surface as
  destructive.
- Updated plugin module requirements, workspace replacement, plugin manifest
  versions, and builtin vision and websearch plugin versions for the `v0.8.0`
  release.

## [0.7.0] - 2026-05-28

### Added
- Added the public `dex` Go library package at the repo root with a top-level
  `Engine` constructed via `dex.New(dex.Config{...})` and dedicated services
  for `Auth`, `Plugins`, `Operations`, `Datasources`, `Endpoints`, `Secrets`,
  `Index`, and `Contexts`.
- Added a pluggable `dex.Prompter` interface (with a `NoopPrompter` default)
  so embedders can route interactive auth/connect flows through their own UI
  instead of a TTY.
- Added a `dex.EventSink` callback hook on `Config` for surfacing plugin
  progress/status events to embedders.
- Added typed sentinel errors (`ErrPluginNotFound`, `ErrPluginNotInstalled`,
  `ErrAuthRequired`, `ErrInstanceUnknown`, `ErrNoPrompter`,
  `ErrMissingFields`) and a `PluginError` wrapper preserving the underlying
  `protocol.Error` code and message.
- Added `examples/embed/main.go` demonstrating library embedding: marketplace
  iteration, manifest fetch, and operation listing.
- Added `dex_test.go` covering bundled marketplace load, manifest fetch,
  operation listing, prompter wiring, and unknown-plugin error paths.

### Changed
- The `dex` CLI now consumes the public library API through an internal
  `cli.options.engine(cmd)` helper that wires a terminal-backed `Prompter`
  and stderr event sink, in place of the previous direct `runtime.Runner`
  access.
- Extracted the terminal-backed prompter into `internal/cli/prompter.go` and
  removed the inline `promptAuthFields`/`saveAuthValues`/`stdinIsTerminal`
  helpers from `internal/cli/root.go`.
- Auto-connect, plugin install/upgrade/activate/deactivate, endpoint
  discovery, and index build/status flows now route through the library
  services so embedders get the same behavior as the CLI.
- Updated plugin module requirements, workspace replacement, plugin manifest
  versions, and builtin vision and websearch plugin versions for the `v0.7.0`
  release.

## [0.6.0] - 2026-05-28

### Added
- Added the Jira plugin with issue CRUD, Markdown-to-ADF conversion, comments,
  attachments, transitions with optional auto-walk, create/edit metadata, user
  search, indexed datasources, and reverse lookup of `BASE-123` issue keys and
  `/browse/` URLs.
- Added the Confluence plugin with page CRUD on storage-format bodies, page and
  user search, attachments, indexed datasources, and reverse lookup of
  `/wiki/spaces/.../pages/{id}` URLs.
- Added the shared `internal/atlassian` package with the Atlassian Cloud
  gateway URL builder, bearer-token HTTP client, bounded JSON and byte
  response decoding, multipart attachment upload helpers, and Atlassian-shaped
  error decoding.
- Added GitLab merge request write operations `gitlab.mr.create`,
  `gitlab.mr.approve`, and `gitlab.mr.merge`, plus
  `gitlab.repository.tag.create`.
- Added Slack parity operations for bookmarks (add/edit/delete/list), channels
  (join/list/mark-read), files (delete/download/info/list/upload), messages
  (edit/delete), reactions (add/remove), presence (get/set), mentions,
  unreads, thread reads, search, emoji list, user list, info, and download.
- Added the `dex.plugin.v2` framed protocol with request/response/event frames,
  plugin-to-host event emission, host-call dispatch, and `dex.plugin.v1`
  fallback negotiation.
- Added `core/pluginbinding/events.go` and `core/pluginbinding/host.go` for
  plugin-side event publishing and host-side call routing.

### Changed
- The plugin runner now negotiates protocol v2 with v1 fallback and frames
  stdio traffic through `protocol.Frame`.
- pluginbinding secret resolution and plugintest utilities now flow secrets
  through the new host-call surface used by protocol v2.
- Updated plugin module requirements, workspace replacement, plugin manifest
  versions, and builtin vision and websearch plugin versions for the `v0.6.0`
  release.

### Documentation
- Reorganized the roadmap document to reflect protocol v2 and the Atlassian
  plugin additions.
- Updated README and maintainer notes for the new release version examples and
  plugin coverage.

## [0.5.0] - 2026-05-28

### Added
- Added the builtin `vision` aggregator with provider discovery, provider
  listing, fanout image analysis, and context support.
- Added OpenAI vision analysis as a vision provider using the Responses API.
- Added Slack file/image upload with bot-token auth, local file paths,
  base64 inline content, thread uploads, initial comments, and alt text.
- Added Slack thread replies through `slack.message.send` using `thread_ts`
  and optional reply broadcast.
- Added Slack thread image metadata and bounded temporary image downloads for
  thread reads.

### Changed
- OpenAI vision inputs now support local image file paths that are converted to
  data URLs with detected media types.
- Updated marketplace metadata for the new builtin vision provider and the
  expanded OpenAI and Slack plugin capabilities.

### Fixed
- Added an Alertmanager-specific Grafana datasource health fallback using the
  proxied `/api/v2/status` endpoint so Grafana plugin-health errors do not hide
  usable Alertmanager datasources.
- Slack file upload is explicitly bot-token-only instead of falling through the
  read-token preference chain.

### Documentation
- Documented the local dex/plugin install workflow in `AGENTS.md`, including
  the required `--dev-plugin NAME=PATH` syntax.

## [0.4.0] - 2026-05-28

### Added
- Added the installable Grafana plugin with datasource catalog, health checks,
  folders, dashboards, dashboard query extraction, annotations, Loki,
  Prometheus, Alertmanager, and Tempo operations.
- Added Grafana datasource cluster alias resolution from the live Grafana
  catalog so callers can pass cluster aliases instead of datasource UIDs.
- Added Kubernetes Grafana endpoint discovery through Services and Ingresses,
  including path-based ingress URLs and `grafana-admin-creds` credential refs.
- Added local-path plugin installation from marketplace metadata so checkout
  plugins can be installed into dex's plugin bin/state before publication.

### Changed
- Grafana operations now request stored auth secret purposes, so endpoint-ref
  calls can use `dex auth connect auto grafana` credentials without inline
  username/password fields.
- Grafana Loki query and recent-log operations now return normalized log
  entries matching the direct Loki plugin while retaining the raw proxy
  response.

### Fixed
- Avoided resolving Kubernetes `credential_ref` secrets when Grafana bearer
  token auth is already complete.
- Tightened Grafana endpoint discovery so Tempo internals are not classified as
  Grafana endpoints.

## [0.3.1] - 2026-05-28

### Fixed
- Fixed GitLab `project.show` so numeric JSON IDs such as `{"id":231}`
  decode correctly and the operation returns the project fields directly.
- Removed direct-list output bloat from synthetic `records` aliases and reduced
  duplicate Kubernetes inventory metadata.
- Fixed generated skill alias deduplication and installed/activated labels.
- Removed host-side Kubernetes `pod.logs` caps for explicit `tail_lines` and
  `limit_bytes` values.
- Clarified SQL datasource read-only query errors.

### Documentation
- Documented the repository release process in `AGENTS.md`, including
  changelog, version bump, commit, tag, push, and GitHub release steps.

## [0.3.0] - 2026-05-28

### Added
- Added managed dex installation and upgrade commands, including `dex version`,
  `dex upgrade`, `dex setup`, plugin upgrade/uninstall flows, and a `Taskfile`
  install target.
- Added `dex skill install`, which writes a dex-home skill with plugin
  references and links it into Claude when available.
- Added Kubernetes port-forward start/stop operations and richer pod log
  retrieval with `since`, `until`, tail-line, and byte-limit controls.
- Added universal `endpoint_ref` input schema exposure so generated operation
  commands consistently surface `--endpoint-ref`.

### Changed
- Centralized CLI output rendering for text, compact, JSON, and YAML so command
  results are rendered generically instead of through per-result type switches.
- Improved generated integration command grouping and unavailable marketplace
  plugin commands so inactive integrations fail with explicit install or
  activation hints.
- Refined skill content from a full help dump into workflow-oriented guidance,
  compact command summaries, installed-vs-marketplace references, refresh notes,
  and credential-redaction guidance.
- Preserved plugin activation state during managed upgrades and resolved nested
  plugin module versions from the owning module path.

### Documentation
- Updated README, concepts, and endpoint docs for universal endpoint refs,
  Kubernetes pod log windows, and port-forward based endpoint workflows.
- Updated plugin module requirements, workspace replacement, plugin manifest
  versions, and release documentation for the `v0.3.0` release.

## [0.2.0] - 2026-05-28

### Added
- Added manifest-driven operation commands generated from activated plugin
  manifests, including schema-derived flags, positional required-field mapping,
  JSON object input, duplicate-alias validation, and reserved built-in command
  protection.
- Added plugin activation state so installed and activated plugins determine the
  generated CLI command surface.
- Added endpoint registry, endpoint import/show/test/doctor flows, Kubernetes
  cluster probes, endpoint-ref resolution for operations and datasource search,
  and stored endpoint health.
- Added Kubernetes plugin support for kubeconfig contexts, endpoint discovery,
  namespace/service/pod/deployment/container inventory, bounded pod logs, and
  datasource records.
- Added SQL plugin support for read-only MySQL, PostgreSQL, and SQLite queries
  through URLs, DSNs, or registered endpoint refs.
- Added Prometheus and Loki plugins with endpoint-aware health checks, query
  operations, labels, and datasource records; Prometheus also includes range
  queries, targets, and alerts.
- Added Docker plugin support for local Docker Engine inspection and lifecycle
  operations across containers, images, networks, volumes, contexts, build cache,
  daemon events, and disk usage.
- Added Ollama and OpenAI plugins for local model operations, OpenAI model
  listing, and OpenAI image generation.
- Added Slack live message send/search/thread operations, Slack message/thread
  and channel-member datasources, and indexed Slack user/channel enrichment.
- Added datasource entity/view/relation/completion metadata, provider fallback
  behavior, host-owned datasource enrichment, and stricter manifest quality
  tests.
- Added operation effects, risk, idempotency, access, auth-scope, and render
  metadata across plugin manifests.
- Added dynamic context providers for system, GitLab, and websearch.

### Changed
- Kept marketplace data install/discovery-only by removing command shortcuts and
  aliases from marketplace entries; executable command names now come from
  plugin manifests.
- Reworked generic search and websearch provider discovery to remain
  provider-neutral while supporting multi-query fanout and aggregated provider
  errors.
- Expanded endpoint-aware datasource behavior so generic `dex search` can pass
  endpoint refs into plugin datasource calls.
- Updated plugin module requirements, workspace replacement, and plugin manifest
  versions for the `v0.2.0` release.

### Documentation
- Updated README, concepts, endpoint flow, maintainer release notes, and roadmap
  documentation for manifest-generated CLI commands, endpoint refs, current
  plugin coverage, and release procedure.

## [0.1.0] - 2026-05-28

### Added
- Added the initial plugin-backed `dex` CLI with marketplace discovery,
  development plugin overrides, shortcut commands, generic operation calls, and
  JSON/YAML/text rendering.
- Added the `dex.plugin.v1` stdin/stdout plugin protocol for manifests, auth,
  operations, datasources, context, endpoints, and index commands.
- Added host-managed auth setup, environment-based auto-connect, scoped secret
  grants, per-instance auth state, and brokered secret resolution for plugins.
- Added `core/pluginbinding` for typed plugin definition, manifest generation,
  JSON schema generation, operation/datasource registration, secret helpers,
  index helpers, datasource records, and lookup result assembly.
- Added host-owned datasource indexes with standardized records, source
  metadata, canonical links, search, get, and lookup.
- Added GitLab plugin support for auth test, project list/show, merge request
  list/show, index build, indexed datasources, and live/index-backed lookup for
  projects, users, groups, issues, and merge requests.
- Added Slack plugin support for user/channel index build and live/index-backed
  lookup for users and channels.
- Added System plugin support for local system information by category.
- Added Tavily and DuckDuckGo web search provider plugins plus the builtin
  `websearch` aggregator.

### Changed
- Centralized plugin boilerplate in `pluginbinding` so provider plugins mostly
  declare manifests, resolve secrets, fetch provider data, normalize records,
  and register typed handlers.
- Standardized datasource lookup matches around top-level `entity`, `id`,
  source metadata, matched fields, score, links, and normalized records.

### Documentation
- Added README coverage for current usage, auth setup, search/lookup, plugin
  development, and release packaging caveats.
- Added maintainer notes, release gate commands, module packaging notes, and
  concept documentation.
