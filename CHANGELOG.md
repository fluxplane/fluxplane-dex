# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
