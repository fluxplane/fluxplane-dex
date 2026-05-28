# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
