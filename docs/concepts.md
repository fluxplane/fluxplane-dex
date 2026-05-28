# Concepts

This document defines the core domain terms used by `fluxplane-dex`.

The short version: `dex` is a host that discovers plugins, grants scoped access
to credentials and runtime capabilities, and exposes plugin behavior through
operations, datasources, context, endpoints, indexes, and manifest-driven CLI
commands.

## Host

The host is the `dex` runtime and CLI.

It owns plugin discovery, marketplace resolution, auth state, secret grants,
index storage, rendering, generated command routing, and protocol invocation. Plugins
should not read dex state files directly or assume they are being called from a
terminal.

The host is also where safety boundaries belong: credentials, local files,
network/process/browser access, endpoint registries, and long-running process
handles should be mediated by host/runtime contracts.

## Plugin

A plugin is an integration module that exposes capabilities to the host.

Examples include GitLab, Slack, system, Tavily, DuckDuckGo, and websearch. A
plugin can be an external binary or a builtin implementation, but it should
present the same protocol surface either way.

A plugin declares what it can do in a manifest and implements protocol commands
such as operation calls, datasource queries, auth tests, context builds, endpoint
discovery, and index builds.

## Manifest

A manifest is the plugin's declaration of capabilities.

It contains the plugin name, version, aliases, operations, auth methods,
datasources, context specs, endpoint specs, index specs, and metadata.

The manifest is discovery data. It lets the host list capabilities, generate
schemas, route calls, decide which credentials are needed, and eventually expose
agent tools without hardcoding each plugin.

## Marketplace

A marketplace is a host-side catalog of plugins.

It tells the host how to find and run a plugin: name, binary, local path,
install path, and metadata. The marketplace is not the plugin implementation and
does not define executable commands; it is the host's registry of available
plugin entries.

## Instance

An instance is a named configuration of a plugin.

Most plugins have a `default` instance, but the model should support multiple
instances such as different GitLab servers, Slack workspaces, Kubernetes
clusters, or Jira tenants. Auth material, secret grants, endpoint refs, and
runtime calls are scoped to a plugin instance.

## Protocol

The plugin protocol is the JSON request/response contract between the host and a
plugin.

The current protocol version is `dex.plugin.v1`. Requests include a command,
plugin name, instance, optional secret grant, and JSON payload. Responses include
an `ok` flag, result payload, or structured error.

Core command families include:
- `manifest`
- `auth.methods`, `auth.test`, `auth.connect`
- `operations.list`, `operations.call`, `operations.call_batch`
- `datasources.list`, `datasources.search`, `datasources.get`,
  `datasources.lookup`
- `context.build`
- `endpoints.discover`
- `index.build`, `index.status`

## Operation

An operation is a typed callable action exposed by a plugin.

Operations are the right model for side effects and domain actions: create,
update, delete, comment, transition, react, merge, retry, cancel, upload,
mark-read, send, port-forward, or run a bounded query.

Operations have names, descriptions, input schemas, output schemas, read-only
flags, compact-output hints, and secret purposes. The roadmap expands this with
effect, risk, idempotency, access, auth-scope, and render metadata.

## Batch Operation

A batch operation call is a list of operation calls sent through one protocol
request.

Batch calls let agents and automation amortize process startup and collect
multiple structured results in one response. Builtin and external plugins should
behave consistently here: each item gets its own success or failure result.

## Datasource

A datasource is a plugin-exposed read interface over entities.

Datasources are the right model for list, search, lookup, get, relation, and
read-heavy views. Examples include GitLab projects, Slack users, web search
results, system facts, Jira issues, Kubernetes pods, Loki log entries, and
Confluence pages.

Current datasource capabilities include `search`, `lookup`, `get`, and `index`.
Datasource specs also describe entity schemas, views, relations, provider
fallback behavior, and completion hints.

## Entity

An entity is the type of record a datasource returns.

Examples:
- `gitlab.project`
- `gitlab.merge_request`
- `slack.user`
- `websearch.result`
- `jira.issue`
- `kubernetes.pod`

Entity names should be stable enough for agents and generated commands to use. They are
not just display labels; they are part of the shared data model.

## Datasource Record

A datasource record is one returned item from a datasource.

The common record shape includes entity, id, source, title, links, and metadata.
Plugins can return richer typed records when useful, but records should remain
compact and structured enough for agent use.

## Lookup

Lookup resolves text or explicit terms into datasource records.

This is the mechanism for turning loose references such as project names,
Slack channel names, issue keys, URLs, or pasted terminal text into concrete
entities. Lookup results should include source information, matched fields, and a
score so callers can choose confidently or ask for clarification.

## Index

An index is host-managed or plugin-declared cached lookup/search material.

Indexes support fast reverse lookup and offline-ish resolution for data such as
GitLab projects, Slack users/channels, or other frequently referenced entities.
The host should own index storage and freshness decisions; plugins should expose
build/status behavior and datasource records rather than relying on private cache
files as the contract.

## Context

Context is prompt-ready information contributed by plugins.

Plugins can declare static context specs and register dynamic context providers.
A provider returns text, data, and reference blocks on demand for a workspace,
channel, project, incident, endpoint, or agent task.

## Endpoint

An endpoint is a reachable service location that a plugin can use or discover.

Examples include a GitLab base URL, Jira tenant URL, Loki API URL, Prometheus
API URL, MySQL connection target, Kubernetes service, or Homer web endpoint.

Endpoint candidates can include id, URL, product, protocol, source, score,
labels, and annotations. Registered endpoints are host-owned records that
plugins can consume through endpoint refs instead of each storing URLs and
discovery state independently.

Endpoint health is the last known reachability result for a registered endpoint.
It records whether the endpoint was reachable, when it was checked, which probe
method was used, how long it took, any error, and compact method-specific
details. A probe can be protocol-specific, such as `sql.query` for SQL endpoints
or `kubernetes.cluster.test` for Kubernetes cluster endpoints, or a generic TCP
connect fallback.

## Auth Method

An auth method describes how a plugin can be authenticated.

It declares a method name, kind, environment variables, required fields, and
which fields are sensitive secrets. The host uses this declaration for
interactive setup, environment-based setup, auth status, and scoped secret
grants.

## Secret Purpose

A secret purpose names a specific credential use.

Examples include `access_token`, `bot_token`, `user_token`, or `api_key`.
Operations and datasources declare which purposes they need. The host then
creates short-lived grants that allow a plugin instance to fetch only the
credential material required for that call.

## Secret Grant

A secret grant is a short-lived host-issued token for reading secret material.

It scopes access by plugin, instance, purpose, and expiry. External plugins use
the host command to fetch granted material. This prevents plugins from needing
direct access to dex home directories, auth files, or unrelated credentials.

## Manifest-Driven CLI Command

Manifest-driven CLI commands are generated Cobra commands over plugin
operations.

Examples are `dex gl mr list`, `dex gl project show`, `dex web search`, and
`dex kube pod logs`. A plugin's manifest name and aliases become root-level
command names after the plugin is activated. Operation names become nested
commands by removing the plugin prefix: `kubernetes.pod.logs` becomes
`dex kube pod logs`.

Operation flags are generated from the operation input JSON schema. CLI flag
names are kebab-case, while operation payloads keep the manifest's JSON field
names, so `--endpoint-ref` becomes `endpoint_ref`. Positional arguments are only
a convenience for required input fields; complex or ambiguous input should use
flags or a JSON object.

The same capability remains available through generic protocol paths such as
`dex op run`, `dex datasource search`, `dex search`, and `dex lookup`. Plugin
implementations should not embed Cobra command parsing.

## Builtin Plugin

A builtin plugin is implemented inside the host but exposed like a normal
plugin.

The websearch aggregator is a builtin because it coordinates other provider
plugins. Builtins should still obey the same operation, batch, datasource,
success, and failure behavior as external plugins.

## Provider Plugin

A provider plugin is a concrete implementation behind a generic capability.

Tavily and DuckDuckGo are provider plugins for web search. The generic websearch
surface should discover and call providers without encoding provider-specific
branches into its domain contract.

Provider names are discovery and routing data, not a reason to make the generic
protocol provider-specific.

## Activation Set

An activation set is a planned contract for enabling related capabilities
together.

For example, a `gitlab` activation set might include GitLab operations,
datasources, indexes, context providers, and generated commands. A `channel` activation
set might include Slack channel context, message search, thread reads, and send
operations.

Activation sets are not a replacement for manifests. They are a higher-level way
to expose coherent capability bundles to humans and agents.

## Operation Set

An operation set is a planned grouping of operations.

It helps the host and agents select the right subset of operations for a task
without exposing every possible action at once. Operation sets are especially
useful once large plugins like GitLab, Kubernetes, or Jira have many operations.

## Runtime Boundary

The runtime boundary is the host-owned interface for effects outside pure JSON
operation handling.

This includes network access, process execution, browser actions, filesystem and
artifact writes, environment reads, human clarification, and managed process
handles. Plugins should request these capabilities through explicit runtime
contracts rather than directly reaching into host state or local machine
behavior.

## Managed Process

A managed process is a long-running task controlled by the host.

Examples include Kubernetes port-forwards, log follows, background watches, or
other streaming operations. The host should provide start, ensure, list, status,
stop, output, and wait primitives so plugins do not invent PID files and process
lifecycle semantics independently.

## Observer

An observer is a planned contract for collecting environment facts.

Observers can report information such as Kubernetes availability, browser
availability, endpoint reachability, auth status, identity facts, or derived
capability availability. They support doctor/statusline behavior and agent
context without baking those checks into each command.

## Render Metadata

Render metadata is plugin-provided guidance for host output.

The host should own rendering for text, compact, table, JSON, and YAML output.
Plugins should return structured data and optional render hints rather than
hand-crafting every terminal display internally.

## Status Summary

Implemented today:
- plugins and manifests
- marketplace entries
- operations and batch operations
- operation effects/risk/idempotency/access metadata
- auth methods and scoped secret purposes
- datasource search/get/lookup
- datasource entity schemas, views, relations, fallback, and completion hints
- context specs and dynamic context providers
- endpoint specs
- index specs and host-owned indexes
- builtin plugins
- manifest-driven CLI commands

Planned or partial:
- operation sets and activation sets
- structured auth test reports
- plugin-contributed secret resolvers
- runtime boundaries and managed processes
- observers, assertions, usage/events, and richer render metadata
