# Vision

This document is a working product vision for `fluxplane-dex`. It should evolve
as the implementation and the plugin protocol settle.

## What This Product Is

`fluxplane-dex` is an engineering interface layer.

It is a CLI today, but the important product is not the terminal command tree.
The important product is a consistent way to expose engineering systems as
operations, datasources, context, endpoint discovery, indexes, and auth-aware
capabilities that both humans and AI agents can use.

The old `dex` CLI demonstrated the workflow value: one small tool could answer
questions and perform actions across GitLab, Slack, Jira, Kubernetes, Loki,
Prometheus, SQL, Confluence, GitHub, Homer, and local todo/statusline state.

This rewrite keeps that ergonomic goal, but changes the foundation:
- No integration should be trapped inside Cobra command handlers.
- No agent should need to scrape terminal text when a typed operation or
  datasource result is available.
- No plugin should read host auth files directly.
- No old codebase should be imported as a compatibility layer.

`fluxplane-dex` replaces the old CLI and the old plugin implementations by
porting the right behavior into a smaller, explicit pluginbinding/runtime model.

## Who It Is For

Primary users:
- Engineers who need fast command-line access to daily infrastructure.
- AI coding agents that need reliable, compact tools for engineering context and
  actions.

Secondary users:
- Platform teams that want integration behavior to be reusable outside one CLI.
- Plugin authors who want a small manifest/operation/datasource contract.
- Automation authors who need predictable JSON/YAML outputs and scoped auth.

## Problems It Solves

### Engineering Context Is Scattered

A normal engineering question often spans multiple systems:
- Which merge request changed this?
- Is the pipeline still failing?
- What did Slack say about this incident?
- Which Jira issue tracks it?
- What changed in Kubernetes or logs?

Without a common interface, each query becomes a different API, auth method,
pagination model, output shape, and shell incantation.

`fluxplane-dex` gives those systems a shared shape.

### AI Agents Need Tools, Not Terminal Folklore

Agents are much better when they receive compact, structured, scoped data. The
old CLI was useful, but many commands were optimized as human terminal output.

This product should expose:
- typed operation inputs and outputs
- datasource search/get/list/lookup
- compact render hints
- indexes for reverse lookup
- context blocks for agent prompts
- explicit failure objects
- credential-gated live tests

The terminal remains important, but it should be a presentation layer over the
same operations and datasources.

### Auth Needs To Be Useful And Safe

Engineering tools need credentials. Agent tools make that more sensitive, not
less.

The host should own auth state and secret grants. Plugins declare what they
need. Runtime calls receive narrowly scoped access for one plugin, one instance,
one purpose, and one short time window.

That design lets plugins remain useful without handing them the whole dex home
directory or broad secret access.

### Integrations Should Not Be Rewritten For Every Surface

The same GitLab or Slack behavior should work through:
- `dex gl ...` style shortcuts
- generic `dex op run ...`
- datasource search
- batch operation calls
- future context and activation surfaces

The implementation should not care which surface called it.

## Product Principles

### Plugin-First

Integrations are plugins. The host provides the marketplace, runtime, auth,
indexing, rendering, and routing.

### Structured Before Pretty

Terminal output should be good, but typed outputs and datasource records are the
stable contract.

### Read Paths Are Datasources

Lists, search, show, lookup, and relation queries should usually be datasource
capabilities. CLI commands can wrap them for ergonomics.

### Side Effects Are Typed Operations

Create, update, delete, comment, transition, react, merge, retry, cancel, upload,
mark-read, and port-forward should be explicit operations with safety metadata.

### No Compatibility Imports

Legacy `dex` and `fluxplane-core/plugins` are reference material. This repo
ports selected behavior and concepts locally. It should not import those modules
or wrap those binaries.

### Host-Owned Safety

Network, process, browser, filesystem/artifact, environment, auth, and secret
access should flow through host/runtime boundaries, not direct ad hoc plugin IO.

## Capability Model

The current repo has basic manifests, operations, auth methods, datasources,
context specs, endpoint specs, indexes, batch calls, and scoped secret grants.

To replace the old plugin stack cleanly, the next layer is not more CLI command
code. It is a small set of host-owned contracts that plugins can rely on:
- Operation sets and activation sets, so humans and agents can enable a useful
  bundle instead of reasoning about hundreds of raw operations.
- Dynamic context providers, so integrations can contribute prompt-ready context
  on demand.
- Datasource entity schemas, views, relations, and provider fallback behavior,
  so read paths have a shared model.
- Endpoint discovery providers, endpoint registry entries, and endpoint refs, so
  plugins do not each invent URL and service discovery.
- Plugin-contributed secret resolvers, scoped auth purposes, and structured auth
  test reports.
- External identity lookup for `my` queries, statusline segments, and permission
  checks.
- Operation effects, risk, idempotency, access requirements, and render
  metadata.
- Runtime boundaries for network, process, browser, filesystem/artifacts,
  environment, and managed process handles.
- Observers, derived assertions, usage/events, and shortcut bindings.

These are product capabilities, not reasons to import old implementations. They
should be added here in the smallest form that supports the roadmap.

## Execution Thesis

The fastest path is not to port one legacy command at a time. The fastest path
is to stabilize the contracts that let each new plugin contribute operations,
datasources, context, endpoint discovery, auth checks, and shortcut bindings in
the same shape.

That means the near-term work should happen in this order:
- Define operation metadata for effects, risk, idempotency, access, and render
  hints before adding broad write operations.
- Define datasource entities, views, relations, and completion hints before
  expanding read-heavy plugins such as GitLab, Slack, Jira, Confluence, and
  Kubernetes.
- Define endpoint registry/discovery and runtime process handles before Loki,
  Prometheus, SQL, Homer, and Kubernetes port-forwarding.
- Finish current partial plugins only after their behavior fits the shared
  contracts: Slack live calls, GitLab richer read datasources, and consistent
  batch/error behavior.
- Keep websearch generic and provider-neutral; provider names are discovery data
  and instance/plugin selection, not protocol branches.

## Next Steps

1. Add the P0 host contracts: operation metadata, datasource entity/view model,
   endpoint registry/discovery, dynamic context providers, and shortcut binding.
2. Stabilize the current plugin set against those contracts, especially Slack,
   GitLab, system, websearch, Tavily, and DuckDuckGo.
3. Port the highest-value daily workflow plugins: Jira, Slack parity expansion,
   GitLab parity expansion, and GitHub.
4. Add observability and cluster plugins once endpoint discovery and managed
   processes exist: Kubernetes, Loki, and Prometheus.
5. Add specialized/local tools after the common contracts are proven: SQL,
   Confluence, Homer, skills, todo, doctor, setup, upgrade, and statusline.

The roadmap lives in [.agents/plans/roadmap.md](../.agents/plans/roadmap.md).

## Non-Goals For Now

- Importing legacy `dex` or `fluxplane-core`.
- Preserving old internal cache-file layouts as plugin contracts.
- Matching every legacy command with a one-to-one plugin operation.
- Building admin-level integrations by default.
- Treating terminal rendering as the primary API.

## What Good Looks Like

A human can type a short command and get the answer.

An agent can call the same capability through a typed operation or datasource and
get compact, structured context.

A plugin can be tested independently, installed from a marketplace, authenticated
through the host, and invoked without knowing whether the caller is a person, an
agent, or an automation script.

That is the product.
