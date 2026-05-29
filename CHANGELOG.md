# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.14.0] - 2026-05-29

### Added
- Added declared datasource access grants so live datasources can request typed host capabilities such as provider access.

### Changed
- Parallelized fluxplaneplugin bundle materialization with bounded workers while preserving marketplace order, reducing coder and host startup time when many independent dex plugin manifests are probed.

### Fixed
- Fixed Kubernetes inventory datasource searches to receive provider capability grants and typed Kubernetes scope fields, allowing scoped live searches by context and namespace without broadening generic datasource schemas.

## [0.13.4] - 2026-05-29

### Changed
- Cached successful fluxplaneplugin manifest snapshots and the shared dex intent index across adapter contribution hooks, reducing startup latency for hosts that register many dex plugins while keeping install failures retryable.
- Upgraded Go module dependencies across the root module, fluxplaneplugin adapter, and plugin modules, then tidied each module.
- Updated release version references for the root module, plugin manifests, built-in plugin metadata, release docs, and README examples.

### Fixed
- Fixed concurrent first-use races in `dex.Engine` service accessors by eagerly initializing services at engine construction.
- Fixed secret and index state path naming to use reversible base64url encoding instead of lossy sanitized names, preventing collisions between values such as `prod/work` and `prod_work`.
- Bounded plugin stdout/stderr capture and host HTTP/blob reads to avoid unbounded memory growth from oversized plugin or capability responses.
- Fixed Docker exec inspection to respect the caller's execution context.
- Hardened Kubernetes port-forward state reads by validating generated port-forward IDs before opening state files.
- Fixed fluxplaneplugin datasource routing to reject unknown entity names instead of querying and relabeling the fallback datasource.

## [0.13.3] - 2026-05-29

### Added
- Added live Confluence datasource `get` handlers for page and user records.

### Changed
- Upgraded Go module dependencies across the root module, fluxplaneplugin adapter, and plugin modules.
- Updated release version references for the root module, plugin manifests, built-in plugin metadata, and release documentation.

### Fixed
- Fixed Confluence page and user search inputs so `endpoint_ref` is preserved for operation and datasource calls.
- Documented host-system HTTP and Jira Atlassian Cloud live testing checks for future plugin development.

## [0.13.2] - 2026-05-29

### Fixed
- Fixed fluxplaneplugin system-backed HTTP capability requests to preserve dex request paths and query parameters.
- Added Jira `cloud_id` auth handling so host-system HTTP calls target Atlassian API URLs instead of site URLs.
- Fixed Jira datasource `get` for issues and users to fetch live records instead of requiring host-index integration.
- Fixed Jira datasource search inputs to preserve `endpoint_ref` and default datasource calls to the single registered product endpoint when one is available.
- Defaulted Jira operations that accept `endpoint_ref` to the single registered Jira endpoint when one exists, matching datasource behavior.
- Expanded Slack datasource generic filter support for `slack.message` and `slack.channel_member`, so datasource clients can pass query/channel/user constraints through the standard `filters` object.
- Fixed Slack input schema descriptions that contain commas so generated JSON Schema keeps the complete description text.
- Documented compact plugin development rules for input schemas, comma escaping, datasource generic request compatibility, dev-plugin reinstall checks, and live testing with dex and coder.
- Added E2E testing notes for live endpoint, GitLab, datasource, and coder-surface probes.
- Improved host-index datasource search so combined name and ID queries, such as Slack user/channel lookups, match across record fields.
- Fixed GitLab input schema descriptions containing commas so generated JSON Schema keeps complete descriptions.
- Fixed endpoint-routed HTTP requests to preserve escaped path segments so GitLab project and merge request lookups by namespaced path work.

## [0.13.1] - 2026-05-29

### Fixed
- Fixed the Slack `slack.thread_message` datasource so thread reads accept generic datasource `query` and `filters` inputs, including Slack permalinks and `thread_ts` aliases.


## [0.13.0] - 2026-05-29

### Added
- New `plugins/asterisk` marketplace plugin (manifest, handler, operations,
  and `cmd/dex-plugin-asterisk` entrypoint) plus the matching
  `runtime/asterisk_provider.go`, marketplace registration, and runner
  wiring so the plugin is reachable through the dex engine.

### Changed
- Small kubernetes host provider tweaks and a `plugintest` helper update
  landed alongside the asterisk work.

## [0.12.0] - 2026-05-29

### Changed
- `fluxplaneplugin` now aggregates a plugin's per-entity dex datasources into a
  single `coredatasource.Spec` named after the plugin (e.g. `gitlab`) with a
  multi-entity `Entities` list — matching the fluxplane-core convention where
  one plugin contributes one datasource keyed on the plugin name. Per-entity
  metadata (descriptions, capabilities, fields, schemas) still flows through
  `Provider.Entities()` at runtime so agents reason in terms of entity types
  and entity-typed filters, not per-entity datasource names.
- `dexAccessor` holds an entity→source map and routes `Search`, `List`, and
  `Get` to the matching dex datasource based on the request's `Entity`. The
  legacy single-source `Open` shape (matching `spec.Name` against a specific
  dex datasource name like `gitlab.projects`) keeps working so older callers
  do not break.

## [0.11.0] - 2026-05-29

### Added
- Added `dex.Config.Capabilities` so embedders can provide the host capability
  implementation used by plugin HTTP, blob, environment, and provider calls.
- Added `fluxplaneplugin.Config.System`, `Capabilities`, and `HostProviders`
  wiring. When a `fluxplane-core/runtime/system.System` is supplied, dex plugin
  IO is routed through the same host system boundary as native core plugins.
- Added `docs/plugin-dev.md` with IO-free plugin development principles,
  dependency direction rules, endpoint/auth guidance, security expectations,
  test practices, and release hygiene.

### Changed
- Removed root module dependencies on the Docker and Kubernetes plugin modules.
  Runtime provider implementations now use root-owned internal host DTOs, so
  plugin modules remain leaf modules and releases no longer need root-to-plugin
  module replacement churn.
- Kept built-in system provider calls available when a custom capability host
  is installed.
- Updated plugin module requirements, manifest versions, builtin plugin
  versions, README examples, and maintainer release docs for `v0.11.0`.

## [0.10.0] - 2026-05-29

### Added
- Added host-managed plugin capabilities for secrets, endpoint resolution,
  authenticated HTTP, blobs, and provider calls so plugins can request host
  services without reading files, environment variables, sockets, or local
  credentials directly.
- Added plugin-side `pluginbinding` helpers for host HTTP and capability calls,
  plus runtime capability hosts for live operations that must touch local
  systems from the dex host process.
- Added IO-free verification covering plugin code and plugin-owned shared
  helpers.

### Changed
- Moved plugin network, credential, endpoint, filesystem, and provider IO
  behind the host protocol. Plugins now pass endpoint/auth/blob references and
  let the runtime resolve concrete URLs, tokens, passwords, and bytes.
- Migrated GitLab, Slack, Tavily, OpenAI vision, Atlassian attachments, SQL,
  Docker, Kubernetes, and system operations onto host-mediated IO paths.
- Updated `fluxplaneplugin` to use the latest `fluxplane-core` release,
  `v0.20.0`.
- Updated plugin module requirements, manifest versions, builtin plugin
  versions, README examples, and maintainer release docs for `v0.10.0`.

### Removed
- Removed plugin-side credential loaders and direct Atlassian auth helpers.
- Removed direct file path inputs for Atlassian attachment upload/download in
  favor of inline bytes and host blob references.

## [0.9.0] - 2026-05-28

### Added
- Added the `fluxplaneplugin` adapter module that exposes dex marketplace
  plugins to `fluxplane-core` consumers on rails parallel to native plugins.
  `Bundles(ctx, engine)` returns one `resource.ContributionBundle` per dex
  plugin (operation specs, `<plugin>` operation sets grouping every
  `<plugin>.*` op, datasource specs + entities, a `PluginRef`, and a
  dex-tagged `SourceRef`) so activation sets, `surface_prepare`, and the
  resource catalog all see dex contributions exactly like native ones.
  `Register(engine, host)` pushes a `pluginhost.Plugin` per dex plugin so
  operation and datasource bindings resolve when the surface actually fires.
  Plugins whose manifest can't be fetched (typically because the binary is
  not installed) yield a stub bundle carrying just the `PluginRef` and a
  warning `Diagnostic`, so they remain discoverable for activation flows
  while `dex plugin install` / `dex auth connect` run on demand.
- Exported lower-level building blocks `Wrap(engine, name)` for a single
  dex plugin and `All(engine)` for the full marketplace plugin slice.

### Changed
- Updated plugin module requirements, workspace replacement, plugin manifest
  versions, and builtin vision and websearch plugin versions for the `v0.9.0`
  release.

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
