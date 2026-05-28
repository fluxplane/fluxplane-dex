# fluxplane-dex Roadmap

This plan is both a product roadmap and a parity map. It uses the vocabulary in
`docs/concepts.md`, compares the legacy CLI surface in `~/projects/dex`, the
plugin surface currently present in `fluxplane-dex`, and useful reference
concepts from `../fluxplane-core/plugins`.

The guiding decision is that `fluxplane-dex` should become the replacement
surface, not a compatibility wrapper. Parity is achieved by mapping old behavior
into local operation, datasource, context, endpoint, auth, and shortcut
contracts.

Sources checked:
- `~/projects/dex/internal/cli/*.go`
- `~/projects/dex/internal/skills/dex/references/*.md`
- Current plugin manifests in `plugins/*/manifest.go`, `runtime/websearch_builtin.go`, and `plugins/marketplace.json`
- `../fluxplane-core/plugins/**`

Boundary rule: do not import code from legacy `~/projects/dex` or from `../fluxplane-core`. This repository is the replacement target for the legacy dex CLI surface and the old `fluxplane-core/plugins` implementations. Those repositories are source material only: inspect behavior, port concepts deliberately, and implement them in this repository's pluginbinding/runtime shape.

Integration rule: `fluxplane-dex` should eventually become a
`fluxplane-core` plugin contribution provider, but that is the last phase of
this roadmap. Until then, contribution-provider concerns should not drive the
shape of near-term plugin implementations. First build the local contracts,
prove them with real plugins, and only then expose the resulting catalog to
`fluxplane-core`.

Parity gate: do not start fluxplane-core integration until this repository has
functional parity with the public `codewandler/dex` CLI. Parity does not mean
copying the old command tree internally; it means every legacy workflow has a
working local mapping to a dex plugin operation, datasource capability, or
explicit host CLI feature with comparable behavior.

## Current fluxplane-dex Plugin Surface

Implemented or scaffolded now:
- `gitlab`: auth test, index build, project list/show, merge request list/show; indexed datasources for projects, users, groups, issues, merge requests.
- `slack`: index build for users/channels; message send, search, and thread are declared but currently return not implemented pending live Slack client migration.
- `system`: local system info by category.
- `tavily`: web search provider with API key auth and datasource search.
- `duckduckgo`: web search provider without auth and datasource search.
- `websearch`: builtin generic provider discovery, aggregated search, and datasource search across web search providers.
- `kubernetes`: kubeconfig/client-go context listing and endpoint discovery from services and MySQL-like connection secrets.
- `prometheus`: endpoint-aware health, query, query range, labels, targets, and alerts operations.
- `loki`: endpoint-aware health, query, labels, and recent log operations.
- `sql`: endpoint-ref and DSN/URL based read-only SQL queries for MySQL, PostgreSQL, and SQLite.

`system`, `tavily`, `duckduckgo`, and `websearch` are new plugin work and are not legacy dex parity targets.

## Roadmap Synthesis

The vision and concepts documents frame `fluxplane-dex` as an engineering
interface layer, not just a terminal command tree. That changes the order of
work:

1. Build the local host contracts that dex itself needs.
2. Stabilize the plugins that already exist against those contracts.
3. Prove endpoint discovery with a real discoverer, starting with Kubernetes.
4. Port broad integrations by composing datasources, typed operations, context,
   endpoint discovery, and shortcuts.
5. Preserve legacy ergonomics through CLI shortcuts without freezing the legacy
   command tree into the protocol.
6. Expose the final plugin/catalog surface as a `fluxplane-core` contribution
   provider only after the local contracts are proven.

The immediate implementation focus should be:
- Operation metadata: effects, risk, idempotency, access requirements, and
  render hints.
- Datasource model: entity schemas, views, relations, provider fallback, and
  completion hints.
- Endpoint model: discovery providers, registry entries, endpoint refs, and
  secret refs, driven by the first real endpoint discovery use case.
- Runtime model: host-owned network/process/browser/filesystem/environment
  boundaries plus managed process handles where local plugins need them.
- Context model: dynamic context providers first; activation/operation sets once
  operation metadata and context providers are stable and contribution-provider
  needs are clearer.
- Shortcut model: structured bindings from `dex gl ...`, `dex slack ...`, and
  similar commands to operation or datasource calls.

Do not start large Jira/Kubernetes/Loki/GitHub ports before the relevant shared
contract exists. Otherwise each plugin will invent its own version of the same
behavior and parity will become harder, not easier.

## Highlighted Next Steps

These are the next implementation slices implied by the concepts document,
vision, and roadmap. They are intentionally smaller than the broad P0 themes.

Current recommended order:
1. Implement Shortcut Binding v1.
2. Maintain the plugin install/activation model so plugin-specific surfaces are
   driven by state and marketplace bindings, not hardcoded host commands.
3. Stabilize current plugins, especially Slack live behavior, GitLab read
   coverage, and provider-neutral websearch hardening.
4. Done: validate Kubernetes endpoint discovery live against the dev cluster's
   `latest` namespace, including Crossplane-style MySQL/PostgreSQL secrets and
   SQL queries through endpoint refs.
5. Promote Prometheus, Loki, and SQL endpoint-ref flows from proof points to
   polished parity features with live-test docs, rendering, and datasource
   mappings.
6. Continue through the remaining codewandler/dex parity gaps.
7. Keep `fluxplane-core` contribution-provider work as the final integration
   phase, after functional parity.

### 1. Operation Metadata v1 - Implemented

Goal: make operations self-describing enough for humans, agents, and the host to
reason about safety before broad write operations are added.

Scope:
- Extend operation specs with effects, risk, idempotency, access requirements,
  auth scopes, and render hints.
- Add pluginbinding helpers for the common cases.
- Populate metadata for existing GitLab, Slack, system, Tavily, DuckDuckGo, and
  websearch operations.
- Expose the metadata through `dex op ls/show` and JSON output.
- Add manifest/protocol/CLI tests.

Acceptance:
- Existing operations still run unchanged.
- `dex op ls/show -o json` includes the new metadata.
- Missing metadata has conservative defaults.
- No plugin-specific safety logic is hardcoded in the host.

### 2. Datasource Entity/View v1 - Implemented

Goal: make datasource reads a stable shared data model instead of only a list of
named search handlers.

Scope:
- Extend datasource specs with entity schema hints, view hints, relation hints,
  provider fallback behavior, and completion hints.
- Add pluginbinding helpers and focused tests.
- Apply the model first to websearch results, GitLab indexed records, and Slack
  indexed records. Keep system as operation/context only.
- Keep `search`, `lookup`, `get`, and `index` as the current capability base.

Acceptance:
- Existing datasource calls remain compatible.
- `datasources.list -o json` can explain entity and view shape clearly enough
  for an agent to choose the right datasource.
- Generic search can filter by stable entity names.
- Lookup results retain source, score, matched fields, and record identity.

### 3. Shortcut Binding v1

Goal: make CLI shortcuts explicit bindings over operations and datasources,
while keeping legacy ergonomics.

Scope:
- Extend marketplace command shortcut metadata so a shortcut can target an
  operation or datasource capability.
- Keep shortcut handlers as argument normalization and rendering glue.
- Use GitLab project/MR shortcuts and websearch shortcuts as proof points.
- Add tests that compare shortcut behavior with the underlying generic call.

Acceptance:
- A shortcut can be inspected and traced to its underlying operation or
  datasource.
- Shortcut failures surface the same structured errors as generic calls.
- No plugin implementation needs Cobra command knowledge.

### 4. Context Provider v1 - Implemented

Goal: let plugins produce prompt-ready context dynamically instead of only
declaring static context specs.

Scope:
- Define a dynamic context provider request/response shape.
- Support text, data, and reference context blocks.
- Add a host command path to build context from selected plugins/instances.
- Use system, GitLab, and websearch as first lightweight providers.

Acceptance:
- `context.build` can return useful blocks from at least one external plugin and
  one builtin plugin.
- Context blocks include source identity and stable IDs.
- Empty context is a successful empty result, not a protocol error.

### 5. Kubernetes Endpoint Discovery Proof Point - Implemented

Goal: create the shared endpoint model through the first real endpoint
discovery plugin instead of designing a registry without endpoints to discover.

Scope:
- Added a minimal Kubernetes plugin that uses kubeconfig and `client-go` to
  list clusters and discover product endpoints from Kubernetes services.
- Added endpoint refs, registry storage, endpoint candidate normalization, and
  secret refs.
- Kept discovery providers separate from stored endpoint refs.
- Added CLI inspection commands for endpoint candidates and registered
  endpoints.
- Added endpoint candidate import so agents can run a deterministic two-step
  flow: discover JSON, then import one selected candidate into the registry.
- Added opt-in interactive discovery selection for human sessions.
- Added endpoint test reports: SQL endpoints are probed through `sql.query`
  using endpoint refs, and other network endpoints use a bounded TCP connect
  fallback.
- Added Kubernetes cluster endpoints from kubeconfig contexts and a
  `kubernetes.cluster.test` probe so cluster reachability is represented as
  endpoint health, not only as product discovery inside a cluster.
- Persisted last endpoint health on endpoint registry records and surfaced it
  through endpoint list/show.
- Added Kubernetes namespace, service, and pod read operations plus inventory
  datasource search records.
- Added Kubernetes-discovered MySQL connection-secret candidates so SQL can
  consume cluster-intrinsic credentials without copying secret material into
  the host.
- Added Kubernetes-discovered PostgreSQL connection-secret candidates using the
  same endpoint-ref and credential-ref model.

Acceptance:
- Done: plugins can return endpoint candidates through the protocol.
- Done: the host can store and reference endpoint refs by stable ID.
- Done: endpoint refs can carry secret refs without exposing secret material.
- Done: Prometheus, Loki, and SQL can consume registered endpoint refs.
- Done: endpoint refs can be tested with a structured health report.
- Done: Kubernetes cluster endpoint refs can be discovered and probed through
  kubeconfig/client-go.
- Done: Kubernetes cluster inventory can be read through operations and
  datasource search.
- Done: credential-gated live validation against the dev Kubernetes cluster's
  `latest` namespace with Crossplane-managed MySQL/PostgreSQL connection
  secrets and SQL queries through registered endpoint refs.

### 6. Current Plugin Stabilization

Goal: only after the first contracts exist, bring current partial plugins up to
the new baseline.

Scope:
- Finish Slack live client migration for send/search/thread.
- Expand GitLab read datasource coverage before adding more GitLab writes.
- Add websearch bounded concurrency, multi-query behavior, and error
  aggregation while keeping provider-neutral semantics.
- Add manifest quality tests and credential-gated live-test paths.

Acceptance:
- Existing plugins conform to operation metadata and datasource entity/view v1.
- Builtin and external plugin failures are surfaced consistently.
- `operations.call_batch` and datasource search behave consistently across
  builtins and external plugins.

Do not start Jira, Loki, SQL, Homer, or GitHub as large ports before the
relevant contract slice exists. Kubernetes is the exception because it is the
actual endpoint discovery proof point for cluster-local products such as
Prometheus and Loki.

## fluxplane-core Plugin Overlap Review

`../fluxplane-core/plugins` contains several integrations that overlap with
legacy dex. Treat them as reference material for behavior and concepts before
porting legacy command behavior one command at a time.

Overlapping core plugins:
- `integrations/gitlab`: broad GitLab implementation with write operations for MRs, repo files, branches, tags, commits, CI variables, pipelines, and snippets; rich datasource entities for projects, activity, MRs, diffs, notes, discussions, approvals, pipelines, branches, tags, commits, repository tree/files, compare, blame, blob search, languages, contributors, jobs, snippets, users, groups, and membership.
- `integrations/slack`: Slack auth/test, active channel send/reply/progress operations, datasource entities for users, channels, messages, and thread messages, plus channel-with-members views.
- `integrations/jira`: Atlassian auth/test, operations for issue search/create/comment, datasource views for Jira issues and projects.
- `integrations/confluence`: Atlassian auth/test and datasource-first support for pages and spaces.
- `integrations/loki`: test, labels, query, and recent logs operations, datasource entities for log entries, streams, labels, and detected endpoints, with Kubernetes endpoint discovery and optional port-forwarding.
- `integrations/kubernetes`: datasource-first cluster inventory for clusters, namespaces, pods, services, deployments, and containers; port-forward operation; observers, assertion derivation, endpoint discovery, and Kubernetes secret resolution.
- `integrations/mysql`: bounded read-only MySQL query operation using runtime endpoint refs and secret refs, plus endpoint datasource metadata.
- `integrations/web`: bounded HTTP request operation and web search operation backed by Tavily and DuckDuckGo, plus web search datasource.
- `integrations/openapi`: not a legacy dex integration itself, but useful as a generator path for REST-heavy gaps if stable OpenAPI specs are available.
- `native/skills`: skill activation, context provider, and datasource for skills and references.
- `native/browser`: browser open/navigation/read/screenshot/etc. operations that can cover browser-open style host actions.
- `native/identity` and `native/usage`: adjacent to Claude statusline and status/identity display, but not direct replacements for legacy statusline.

No direct overlap found for these legacy dex features in `fluxplane-core/plugins`:
- Prometheus.
- GitHub.
- Homer.
- Todo.
- Claude statusline.
- Doctor, setup, upgrade, and version host commands.

Other core plugins inspected but not considered dex parity overlap: AWS, Docker, Git, OpenAI, filesystem, shell, workspace, project, memory, task, goal, loop, sleep, human, image, text, language plugins, session history, discovery, datasource catalog, and support/eventcatalog.

### Replacement Stance

Use the old dex CLI and `fluxplane-core/plugins` as reference implementations, not as dependencies, adapters, or compatibility layers. The first pass should port useful models, client behavior, protocol concepts, and tests into this repo's pluginbinding style. Do not add a direct module dependency on `github.com/fluxplane/fluxplane-core`, and do not keep wrappers around the old dex CLI.

Reuse categories:
- Adopt: GitLab datasource/entity coverage and action-operation shapes; Atlassian auth/session handling; Slack users/channels/messages/thread datasource logic; Loki query/discovery model; Kubernetes inventory/discovery/port-forward model; MySQL endpoint-ref query model; web search concurrency/error handling.
- Adapt: core operation names such as `gitlab_mr` and `mysql_query` should become dex pluginbinding names such as `gitlab.mr.action` or `mysql.query` where that better matches existing dex naming.
- Extend: Jira transitions/links/update/delete, Slack unreads/mentions/files/bookmarks/presence, Kubernetes logs, Prometheus, GitHub, Homer, Todo, and Claude statusline need work beyond current core coverage.
- Keep host-only: setup, doctor, upgrade, version, and most skills install/search UX should stay host CLI features unless a plugin contract clearly improves them.
- Skip for parity: core admin surfaces that are absent or intentionally not registered, and unrelated core plugins such as AWS, Docker, OpenAI, filesystem, shell, language, memory, task, and image.

Concrete integration guidance:
- Preserve the dex pluginbinding manifest shape, marketplace entries, CLI shortcuts, and current operation names where already established.
- For GitLab, combine current `fluxplane-dex` read operations with the broader
  datasource model and coarse write-operation shapes from the reference
  implementation. Many items listed as missing in the legacy gap list have a
  reference behavior to port locally.
- For Atlassian, port the shared core `internal/atlassian` approach into a repo-local package instead of building separate Jira and Confluence auth flows.
- For web search, keep `fluxplane-dex`'s generic provider discovery and
  standalone `tavily`/`duckduckgo` providers, but port bounded concurrency,
  multi-query behavior, result rendering, and error aggregation semantics into
  this repo.
- For Loki and Kubernetes, port the endpoint discovery and runtime endpoint
  registry pattern locally. This can avoid hand-coding legacy discovery behavior
  separately in each plugin.
- For SQL, treat legacy `sql` as a MySQL-first plugin initially. The reference
  model moved from named config datasources to endpoint refs plus secret refs;
  dex parity should support legacy config migration but not require the old
  config shape internally.
- For GitHub or Prometheus, consider whether OpenAPI-generated operations can cover low-level API access, but keep hand-authored plugins for agent-facing workflows, compact outputs, and datasource entities.
- For skills, treat legacy `skill search/install/show` and core `native/skills` as adjacent but different: core handles activation/context/datasource; legacy handles marketplace/install UX.

### Core Concepts to Port

Current `fluxplane-dex` already has basic manifest fields for operations, auth,
datasources, context, endpoints, and indexes. Full parity needs several
reference concepts ported or re-shaped locally before the larger integrations
can be clean.

Current local support:
- Present: `PluginManifest`, typed operation schemas, operation safety/access metadata, auth methods/fields, datasource specs with capabilities/entities/views/relations/completion metadata, static context specs, dynamic context providers, endpoint specs, index specs, endpoint candidates, context blocks, typed pluginbinding handlers, batch calls, and secret purpose metadata.
- Partial: marketplace command shortcuts, datasource search/get/lookup, host-owned indexes, builtin plugins, and live auth/secret resolution.
- Missing for local parity: shortcut bindings, discovery providers, endpoint registry, structured auth-test reports, runtime system boundary, managed processes, and richer render metadata.
- Deferred to contribution-provider phase: operation sets, activation sets,
  external identity, observers/assertions as contribution facts, usage/events,
  and catalog export shapes for `fluxplane-core`.

Concepts to port into this repo:
- Context providers: providers that can return text/data/reference blocks on demand, not only static manifest `ContextSpec` declarations.
- Datasource entities and views: entity schemas, list/search/get/lookup/relation capabilities, view hints, relation includes, and provider-first/fallback search behavior.
- Endpoint discovery: discovery providers that return endpoint candidates for products such as Loki, Prometheus, Homer, Kubernetes services, SQL databases, and maybe GitLab/Jira instances.
- Endpoint registry: host-managed endpoint refs that plugins can consume instead of storing URLs repeatedly in plugin config.
- Secret resolution: plugin-contributed secret resolvers, secret purpose metadata, auth scopes, and migration from legacy config/env shapes.
- Auth testing: structured auth test reports per plugin, method, instance, check, status, message, and details.
- Operation semantics and access metadata: read/write/network/process/browser/file effects, risk, idempotency, auth-scope requirements, and target access descriptors.
- Runtime system boundary: local abstractions for network, process, browser, filesystem/artifacts, environment, and human clarification so plugins do not call host IO directly.
- Managed process handles: start/ensure/list/status/stop/output/wait primitives for port-forwards, log follows, and other long-running tasks.
- Command/shortcut binding: structured mapping from legacy CLI shortcuts to operation or datasource calls without embedding command parsing inside plugin implementations.
- Render metadata: compact/table/json/yaml hints owned by the host renderer, not ad hoc output formatting inside each plugin.
- Contribution catalog: a final export surface that can present operations,
  datasources, context providers, endpoint discovery, shortcut bindings,
  activation bundles, identity facts, usage facts, observers/assertions, and
  render hints to `fluxplane-core` without importing `fluxplane-core`.

Immediate concept priority:
- P0: maintain operation semantics/access metadata, datasource entity/view model, and context providers; implement shortcut binding.
- P1: Kubernetes-driven endpoint registry/discovery, auth test reports,
  secret resolution/migration, runtime system boundary, and managed process
  handles.
- P2: current plugin stabilization, richer render metadata, and high-value
  daily workflow integrations.
- Final phase: contribution catalog, operation sets, activation sets, external
  identity, observers/assertions, usage/events, and other
  `fluxplane-core`-facing export concerns.

### Relax Legacy Parity Where Core Is Better

- Do not require a separate plugin operation for every legacy subcommand if a single typed action operation plus datasource reads is cleaner. Examples: core GitLab uses `gitlab_mr` with `op=create|edit|comment|...`, and `gitlab_pipeline` with `op=create|retry|cancel`.
- Do not force legacy cache-file semantics like `~/.dex/gitlab/index.json` into plugin internals. Prefer datasources, indexes, views, endpoint registries, and host-managed caching.
- Do not require legacy CLI output schemas to be identical when the plugin protocol has typed outputs and datasource records. Preserve stable fields needed by agents and shortcuts; render compatibility can live in CLI shortcuts.
- Do not embed provider names in generic websearch semantics. Provider names should remain discovery data or instance/plugin names, not protocol-level cases.
- Do not duplicate host capabilities as integration-specific code when core has generic facilities. Examples: browser open for `gl mr open`, runtime process/endpoint handling for port-forwarding, and runtime system/network access for web requests.
- Keep high-risk/admin surfaces opt-in. Core GitLab explicitly leaves admin operations out; dex parity should not add them just to be exhaustive.

### Contract Pattern

Use three levels of surface area:
- Datasources for read/list/search/get over entities.
- Typed action operations for side effects and lifecycle transitions.
- CLI shortcuts for legacy ergonomics and compact presentation.

Default mapping:
- Legacy `ls`, `show`, `search`, `lookup`, `page`, `projects`, `channels`, `users`, `labels`, and similar read commands should become datasource calls first.
- Legacy write commands such as `create`, `edit`, `delete`, `comment`, `transition`, `react`, `merge`, `retry`, `cancel`, `upload`, and `mark-read` should become typed operations.
- Legacy convenience commands such as `my`, `unreads`, `mentions`, `recent logs`, and `activity` can be typed operations if they combine multiple reads or encode a useful domain query.
- Legacy browser/process/file commands should call host capabilities through the runtime instead of embedding direct `os/exec`, browser, or filesystem behavior inside integration plugins.

This gives parity without freezing legacy CLI command granularity into the plugin protocol.

## Legacy Plugin Inventory and Gaps

In this section, "missing" means missing from current `fluxplane-dex`. Some items are already implemented in `fluxplane-core` and should be ported or used as reference rather than rebuilt.

### GitLab

Legacy support:
- `gl activity`
- `gl index`
- `gl proj ls/show`
- `gl commit ls/show`
- `gl mr ls/show/open/comment/diff/react/close/reopen/approve/merge/create/edit`
- `gl pipeline ls/show/jobs/retry/cancel/create/logs`
- `gl snippet ls/show/create/delete`
- `gl file show/meta/blame`
- `gl tree`
- `gl diff`
- `gl search blobs`

Current status:
- Partially implemented.
- Present: auth test, index build, project list/show, merge request list/show, indexed datasources.
- Core overlap: `fluxplane-core` already has most read datasource entities and coarse write operations for MRs, repo files, branches, tags, commits, CI variables, pipelines, and snippets.

Missing operations/features:
- Activity summary.
- Commit list/show.
- Full MR workflow: open-in-browser, comments including replies and inline comments, diff inspection/search/line context, reactions, close/reopen/approve/merge/create/edit.
- Pipeline workflow: list/show/jobs/retry/cancel/create/logs.
- Personal snippets: list/show/create/delete.
- Repository browsing: file content, metadata, blame, tree, ref diff, blob search.
- Legacy compact render behavior and rich JSON/YAML schemas for all commands.
- Legacy local index semantics: cache TTL, force/no-cache behavior, contributor/language metadata, project filtering/sorting parity.
- Datasource coverage for commits, pipelines, snippets, repository files, and blob search results if those should be searchable via the generic datasource path.
- Relaxation: model lifecycle writes as typed action operations where that reduces operation sprawl; keep read/list/show surfaces datasource-first where possible.

### Slack

Legacy support:
- `slack auth/test/info`
- `slack presence`, `slack presence set`
- `slack index`
- `slack channels`, `slack users`
- `slack channel members/join`
- `slack send/edit/delete`
- `slack upload`
- `slack emoji`
- `slack react`
- `slack unreads`
- `slack mark-read`
- `slack mentions`
- `slack search`
- `slack thread`
- `slack download`
- `slack file list/info/download/delete`
- `slack bookmarks`

Current status:
- Partially scaffolded.
- Present: index build for users/channels.
- Declared but not implemented: send, search, thread.
- Core overlap: `fluxplane-core` has Slack auth/test, active channel send/thread reply/progress operations, and richer live datasources for users, channels, messages, thread messages, and channel membership views.

Missing operations/features:
- Complete live client migration for declared send/search/thread operations.
- OAuth auth flow, auth test, and identity info operations.
- Presence read/write.
- Channel/user list, channel member list, and channel join.
- Message edit/delete.
- File upload, file list/info/download/delete, and top-level download shortcut.
- Emoji list and reaction operations.
- Unreads, mark-read, mentions, and pending/acked/replied classification.
- Bookmarks.
- Mention and channel name resolution using indexes.
- Ticket extraction from Slack search results.
- Compact/JSON/YAML render parity and Slack URL/timestamp parsing parity.
- Datasource coverage for messages, threads, mentions, files, bookmarks, channels, and users.
- Relaxation: channel/user/message/thread reads should prefer datasource/list/search/get rather than dedicated command-style operations unless there is a side effect.

### Jira

Legacy support:
- `jira auth`
- `jira view`
- `jira search`
- `jira my`
- `jira lookup`
- `jira create`
- `jira delete`
- `jira link`
- `jira unlink`
- `jira update`
- `jira transition`
- `jira comment`
- `jira comment-delete`
- `jira project`
- `jira projects`

Current status:
- Missing as a plugin.
- Core overlap: `fluxplane-core` already has shared Atlassian auth/test, issue search/create/comment operations, and issue/project datasource views.

Missing operations/features:
- Atlassian OAuth or token auth.
- Issue show/search/my/lookup.
- Issue create/delete/update.
- Issue transitions and transition listing.
- Comment create/delete.
- Issue link/unlink and link type listing.
- Project list/show with workflow/status detail.
- Markdown to Atlassian Document Format conversion.
- Datasources for issues, projects, users, comments, and issue links.
- Statusline segment for assigned/open issues.
- Compact/JSON/YAML render parity.
- Relaxation: start from a local port of the issue/project datasource model and
  add only missing write/read operations that cannot be represented by
  datasource search/get/list.

### Confluence

Legacy support:
- `confluence auth`
- `confluence spaces`
- `confluence search`
- `confluence page`
- alias: `cf`

Current status:
- Missing as a plugin.
- Core overlap: `fluxplane-core` has shared Atlassian auth/test and datasource-first page/space access.

Missing operations/features:
- Atlassian OAuth or token auth.
- Space listing.
- CQL/plain text search.
- Page view with body extraction.
- Datasources for spaces, pages, and search results.
- Compact/JSON/YAML render parity.
- Relaxation: implement `spaces`, `search`, and `page` as CLI shortcuts over datasource list/search/get where practical.

### Prometheus

Legacy support:
- `prom discover`
- `prom query`
- `prom query-range`
- `prom labels`
- `prom targets`
- `prom alerts`
- `prom test`
- alias: `prometheus`

Current status:
- Missing as a plugin.
- Core overlap: none found in `../fluxplane-core/plugins`.

Missing operations/features:
- URL endpoint config/auth model.
- Kubernetes auto-discovery.
- Instant and range PromQL query.
- Label names and values, including match filters and completion metadata.
- Target and alert listing.
- Connection test.
- Time parsing parity for durations, absolute timestamps, local time, and UTC.
- Datasources for metrics/query results, labels, targets, and alerts.

### Loki

Legacy support:
- `loki discover`
- `loki query`
- `loki labels`
- `loki test`

Current status:
- Missing as a plugin.
- Core overlap: `fluxplane-core` already has test, labels, query, recent logs, datasource entities, Kubernetes discovery, endpoint refs, tenant IDs, and port-forward assisted discovery.

Missing operations/features:
- URL endpoint config/auth model.
- Kubernetes auto-discovery.
- LogQL query with namespace scoping.
- Label names and values.
- Connection test.
- Time parsing parity and oldest-first rendering.
- Datasources for log entries, streams, labels, and discovered endpoints.
- Relaxation: port the `recent_logs` operation shape for common
  app/pod/container lookups instead of requiring legacy users to hand-write
  LogQL every time.

### Kubernetes

Legacy support:
- `k8s ctx ls`
- `k8s ns ls`
- `k8s pod ls/show/logs`
- `k8s svc ls/show`
- `k8s forward ls/start/stop/status`
- aliases: `kube`, `kubernetes`

Current status:
- Minimal plugin implemented.
- Core overlap: `fluxplane-core` already has datasource inventory for clusters, namespaces, pods, services, deployments, and containers; port-forwarding; observers; endpoint discovery; and Kubernetes secret resolution.

Missing operations/features:
- Namespace, pod, service, deployment, and container datasource inventory.
- Namespace, pod, and service list/show.
- Pod logs with follow, previous container, include/exclude regex, tail, since, all containers, and container selection.
- Detached port-forward lifecycle and PID/status tracking.
- Smart pod/service discovery for port-forward targets.
- Shell completion metadata for contexts, namespaces, pods, containers, services, and forwards.
- Datasources for contexts, namespaces, pods, services, logs, and forwards.
- Broader live validation docs/matrix across dev/staging/prod contexts for
  service and Crossplane-style SQL secret discovery.
- Relaxation: list/show commands should be datasource shortcuts. Keep explicit operations for side effects and streaming/long-running behavior such as logs and port-forwarding.

### SQL

Legacy support:
- `sql datasources`
- `sql query`

Current status:
- Implemented as generic `sql` plugin with MySQL, PostgreSQL, and SQLite
  drivers.
- Core overlap: `fluxplane-core` has a MySQL query operation using endpoint refs and secret refs.

Missing operations/features:
- Configured SQL datasource discovery.
- Legacy named datasource listing/migration.
- Table result rendering.
- Datasource protocol mapping for named database connections and query results.
- Full live documentation for manual local MySQL/PostgreSQL/SQLite and
  Kubernetes-discovered endpoint refs.
- Relaxation: support legacy named datasource migration, but internally prefer runtime endpoint refs and secret refs over the old `~/.dex/config.json` SQL shape.

Implemented:
- Read-only query operation with bounded result rows.
- Manual connection via URL or DSN.
- Endpoint-ref resolution through the host endpoint registry.
- Kubernetes secret refs for cluster-intrinsic MySQL credentials.
- Real SQLite test and container-backed MySQL/PostgreSQL tests.

### GitHub

Legacy support:
- `gh auth/test`
- `gh clone`
- `gh repo create`
- `gh issue list/view/create/comment/edit/close`
- `gh label list/create/delete`
- `gh release list/view/create/edit`
- alias: `github`

Current status:
- Missing as a plugin.
- Core overlap: none found in `../fluxplane-core/plugins`.

Missing operations/features:
- Decide whether to wrap the `gh` CLI as legacy did or use native GitHub API clients.
- Auth status and login handoff.
- Repo clone and repo create.
- Issue CRUD subset, comments, labels, close reasons, filtering, cursor pagination.
- Label list/create/delete.
- Release list/view/create/edit.
- Cross-repo `owner/repo` handling.
- Datasources for repositories, issues, labels, releases, and comments.
- Statusline segment for assigned issues/PRs.
- Compact/JSON/YAML render parity.

### Homer

Legacy support:
- `homer discover`
- `homer search`
- `homer calls`
- `homer show`
- `homer export`
- `homer qos`
- `homer analyze`
- `homer endpoints`
- `homer aliases`
- alias: `sip`

Current status:
- Missing as a plugin.
- Core overlap: none found in `../fluxplane-core/plugins`.

Missing operations/features:
- Endpoint config with URL-specific credentials.
- Kubernetes discovery for `homer-webapp`.
- SIP search by number, users, user agent, call ID, time, method, and smart query.
- Call grouping and status classification.
- SIP ladder rendering and raw message display.
- PCAP export.
- RTCP QoS metrics and MOS estimation.
- Multi-leg call correlation and header fan-out analysis.
- Endpoint and IP/port alias listing.
- Datasources for calls, messages, endpoints, aliases, QoS streams, and analysis results.

### Todo

Legacy support:
- `todo add`
- `todo update`
- `todo show`
- `todo ls`
- `todo ref add/del`

Current status:
- Missing as a plugin.
- Core overlap: none found in `../fluxplane-core/plugins`.

Missing operations/features:
- Local todo store.
- Todo create/update/list/show.
- Reference add/remove.
- Statusline segment for total/pending/in-progress/on-hold counts.
- Datasources for todos and references.

### Claude Integration

Legacy support:
- `claude statusline`

Current status:
- Missing as a plugin or runtime feature.

Missing operations/features:
- Claude Code statusline command.
- Session JSON parsing from stdin.
- Segment aggregation from k8s, GitLab, GitHub, Jira, Slack, and Todo.
- Per-session cache files and TTL handling.
- Configurable statusline templates.

### Skills

Legacy support:
- `skill install`
- `skill search`
- `skill show`
- top-level `install` to install the dex skill into Claude skills.
- alias: `skills`

Current status:
- Missing as a plugin.
- Core overlap: `fluxplane-core/native/skills` covers skill activation/context/datasource, but not the legacy skills.sh install/search/show workflow.

Missing operations/features:
- skills.sh search/install support.
- Dex skill content rendering/install.
- Config/path handling for Claude skills.
- This may remain a CLI utility rather than a plugin unless plugin distribution itself needs skill integration.

### Setup

Legacy support:
- `setup`

Current status:
- Missing as a plugin or runtime command.

Missing operations/features:
- Interactive integration configuration.
- Migration from `~/.dex/config.json` to fluxplane-dex auth/instance stores.
- Guided setup for GitLab, Slack, Jira, Confluence, Prometheus, Loki, Homer, SQL, GitHub, and statusline segments.
- This should likely be a host CLI feature, not a plugin operation.

### Doctor

Legacy support:
- `doctor`

Current status:
- Missing as a plugin or runtime command.

Missing operations/features:
- Integration health checks.
- Config validation.
- Auth validation.
- External binary checks where relevant (`gh`, `kubectl`, etc.).
- Endpoint connectivity checks.
- Plugin manifest/protocol validation.
- This should likely be a host CLI feature that calls each plugin's auth/test operation where available.

### Upgrade

Legacy support:
- `upgrade`

Current status:
- Missing as a plugin or runtime command.

Missing operations/features:
- Release discovery.
- Binary self-update.
- Version pinning and upgrade target selection.
- This should remain a host CLI concern, not a plugin.

### Version

Legacy support:
- `version`

Current status:
- Host CLI version handling exists separately in fluxplane-dex and is not a plugin parity item.

## Protocol and Platform Work Needed

The current plugin protocol can express operations, auth methods, datasources, contexts, and endpoints. Parity work will need the following cross-cutting additions or decisions before porting the larger integrations:

- Source boundary: keep the no-import rule explicit. Reusable behavior from legacy dex or `fluxplane-core` must be ported into this repo, not consumed through compatibility wrappers or module dependencies.
- Batch behavior: every builtin and external plugin exposed through operation APIs must support batch calls consistently.
- Operation semantics: add or mirror safety metadata for effects, risk, idempotency, and auth-scope requirements before adding broad write operations.
- Render contracts: define compact/table/json/yaml behavior as schema metadata instead of duplicating legacy CLI renderers.
- File outputs: model operations that write files locally, such as Slack download, Homer PCAP export, and GitHub clone.
- Streaming/long-running operations: support pod logs follow, detached port-forward start/stop/status, and possibly Loki tail if added later.
- Browser/open actions: decide whether operations may request a host-side browser open for GitLab MR open and OAuth flows.
- OAuth flows: standardize browser callback auth for Slack, Jira, and Confluence.
- External command adapters: decide per plugin whether wrapping `gh`/`kubectl` is acceptable or whether native clients are required.
- Instance/config migration: map legacy `~/.dex/config.json` entries into fluxplane-dex instances and secret purposes.
- Discovery operations: standardize endpoint discovery outputs for Prometheus, Loki, Homer, and Kubernetes-backed discovery.
- Datasource taxonomy: define common entity names for issues, projects, repositories, messages, logs, metrics, pages, calls, files, todos, and web results.
- Completion metadata: legacy CLI had rich resource completions; plugin manifests need a way to expose completion datasource hints.
- Statusline aggregation: decide whether statusline is host-only orchestration over plugin operations or its own builtin plugin.
- Live-test policy: define credential-gated live tests per plugin and keep fake-client unit tests for protocol behavior.

## Execution Order

### Phase 0: Local Contract Baseline

1. Confirm and keep enforcing the source boundary: no imports from legacy dex or
   `fluxplane-core`; port selected behavior and concepts into this repo.
2. Maintain Operation Metadata v1 and require it for new write operations.
3. Maintain Datasource Entity/View v1 and require metadata for new datasources.
4. Implement Shortcut Binding v1 for GitLab and websearch shortcuts.
5. Extend Context Provider v1 beyond the system, GitLab, and websearch proof
   points as plugins gain richer context.
6. Update protocol/runtime tests so builtins and external plugins behave the
   same for success, failure, batch calls, auth grants, datasource errors, and
   metadata exposure.
7. Defer operation sets, activation sets, identity facts, usage events, and
   contribution catalogs until the final contribution-provider phase.

### Phase 1: Stabilize Current Partial Plugins

1. Finish Slack live client migration for `slack.message.send`, `slack.search`,
   and `slack.thread`.
2. Reconcile `fluxplane-dex` GitLab with the old GitLab reference: port richer
   datasource entities first, then fill CLI shortcuts over those reads.
3. Add GitLab action operations where the typed operation model is already clear:
   MR, pipeline, snippet, repo file, branch, tag, commit, and CI variable.
4. Harden current plugins with manifest quality tests, CLI error behavior tests,
   datasource behavior tests, and credential-gated live tests.
5. Keep websearch provider-neutral while locally adding bounded concurrency,
   multi-query behavior, result rendering, and error aggregation semantics.
6. Do not add broad write operations without Operation Metadata v1 fields for
   their effect/risk/idempotency/access profile.

### Phase 2: Kubernetes and Endpoint Discovery Proof Point

1. Done: Kubernetes plugin uses kubeconfig/client-go for cluster context
   listing and product endpoint discovery from services.
2. Done: Endpoint Registry/Discovery v1 stores endpoint candidates, registered
   endpoint refs, and secret refs.
3. Done: CLI inspection exists for discovered candidates and registered
   endpoints.
4. Done: Prometheus, Loki, and SQL can consume configured or registered endpoint
   URLs/refs.
5. Done: endpoint health/test reports exist for SQL endpoint refs and generic
   TCP-reachable network endpoints.
6. Done: Kubernetes cluster endpoints can be discovered from kubeconfig and
   probed via `kubernetes.cluster.test`.
7. Done: Kubernetes namespace, service, and pod inventory is exposed through
   operations and datasource search.
8. Next: add a documented live-test matrix for Kubernetes service discovery and
   Crossplane-style SQL secret discovery.

### Phase 3: Port High-Value Daily Workflow Integrations

1. Jira plugin: port the Atlassian auth pattern, issue search/create/comment,
   and issue/project datasource; then add view/my/lookup, transitions,
   update/delete, links, and project workflow detail.
2. Slack parity expansion: port the reference datasource behavior for
   users/channels/messages/thread messages and active channel operations; then
   add mentions, unreads, mark-read, reactions, edit/delete, files, bookmarks.
3. GitHub plugin: issue list/view/create/comment/edit/close, label list,
   release list/view, repo create; decide native API versus `gh` wrapper first.
4. GitLab parity expansion: read-only CLI shortcut parity, job logs, blob
   search, full diff rendering, and any legacy-specific helpers not covered by
   the reference implementation.

### Phase 4: Observability and Cluster Plugins

1. Kubernetes plugin: port the reference datasource inventory, endpoint
   discovery, observers, secret resolver, and port-forward operation; add pod
   logs and legacy context/namespace shortcuts.
2. Loki plugin: port the reference test, labels, query, recent logs, datasource
   entities, endpoint discovery, and port-forward discovery behavior.
3. Add shared time parsing and endpoint discovery helpers to avoid each plugin
   reimplementing legacy behavior.

### Phase 5: Specialized and Local Tools

1. SQL plugin: endpoint-ref read-only query model is implemented for MySQL,
   PostgreSQL, and SQLite. Next add legacy named datasource listing/migration
   and datasource result mapping.
2. Confluence plugin: port the Atlassian auth and page/space datasource
   patterns, then add legacy shortcuts.
3. Homer plugin: discovery, search/calls/show first; export/QoS/analyze after
   the read path is stable.
4. Skills: port or align skill activation/context/datasource concepts
   separately from legacy skills.sh install/search/show.
5. Todo plugin: local store, CRUD, refs, statusline segment.

### Phase 6: Host CLI Utilities

1. Doctor: host command that validates plugin manifests, auth state, endpoints,
   external binaries, and known instances.
2. Setup: host command for interactive auth/instance configuration and legacy
   config migration.
3. Upgrade: host self-update command.
4. Skills and Claude statusline: keep as host-level features unless plugin distribution or statusline segment composition benefits from the plugin protocol.

### Phase 7: fluxplane-core Contribution Provider

Goal: expose the mature `fluxplane-dex` plugin surface as a contribution source
for `fluxplane-core` without importing `fluxplane-core` into this repository.

Prerequisite: functional parity with the public `codewandler/dex` CLI must be
complete or explicitly waived for specific legacy workflows before this phase
starts.

Scope:
- Define a local contribution catalog format that can describe plugins,
  operations, datasources, context providers, endpoint discovery providers,
  auth methods, shortcut bindings, render hints, and secret purposes.
- Add operation sets only where a group is needed for activation, shortcut
  discovery, or agent-facing tool selection.
- Add activation sets only where operations, datasources, context providers,
  and endpoint discovery need to be activated together.
- Add external identity, observer/assertion, and usage/event contributions as
  exported facts, not as prerequisites for ordinary plugin execution.
- Provide a host command or builtin datasource that emits the contribution
  catalog for `fluxplane-core` to consume.
- Keep the integration boundary one-way: dex emits contribution data; no
  `fluxplane-core` packages are imported here.

Acceptance:
- `fluxplane-dex` can enumerate its contribution catalog in a stable machine
  readable form.
- Each contribution record points back to local dex plugin manifests,
  operations, datasources, contexts, endpoints, auth, and shortcuts.
- `fluxplane-core` integration can be built as an adapter over that catalog
  instead of a dependency inside this repo.

## Acceptance Criteria

- Every legacy `~/projects/dex` integration is represented either as a plugin, builtin plugin, or explicit host CLI feature.
- Every legacy command is mapped to an operation, datasource capability, or deliberate non-plugin host command.
- `dex op call`, `dex op batch`, plugin-specific shortcuts, and datasource search paths produce consistent success/failure behavior.
- Each plugin has manifest quality tests, operation handler tests, and at least one live-test path gated by credentials/config.
- For read-only list/search/show operations, compact and JSON outputs are stable enough for agent use.
- Legacy config migration is documented before old `~/.dex/config.json` users are expected to switch.
- The final `fluxplane-core` integration consumes a dex contribution catalog;
  it does not require importing legacy dex or `fluxplane-core` packages into
  this repository.
