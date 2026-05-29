# Plugin Development

Dex plugins are model-facing capability surfaces: describe what an operation can
safely do, validate input, call the dex SDK, and leave host IO to the runtime.

## Core Rules

- **IO-free plugin code:** do not read process env directly, open workspace
  files, create raw HTTP clients, open sockets, shell out, or talk directly to
  Docker, Kubernetes, databases, or other host services.
- **Host protocol is the IO boundary:** use SDK capabilities for HTTP, blob
  reads/writes, env lookup, endpoint resolution, auth material, and provider
  calls; the host decides whether and how to satisfy them.
- **No secrets in params/results:** model-visible payloads must contain intent,
  never bearer tokens, passwords, cookies, API keys, certificates, or resolved
  secret values in errors, logs, diagnostics, events, context, or results.
- **Prefer endpoint refs:** accept `endpoint_ref` and auth purposes instead of
  raw connection material. The runtime combines endpoint and auth material
  outside the model-visible request.
- **Least privilege:** declare the smallest useful capability, effects, risk,
  and access metadata. Read-only operations must not receive write grants.
- **One-way dependencies:** `plugins/<name>` may import
  `github.com/fluxplane/fluxplane-dex/core/pluginbinding`; root runtime code
  must not import `github.com/fluxplane/fluxplane-dex/plugins/<name>`.
- **Internal host DTOs:** if runtime-side host support is needed, put wire DTOs
  and host implementation in `internal/<provider>host`; plugin and host exchange
  JSON across the host protocol without a root-to-plugin compile-time edge.

`fluxplaneplugin.Config.System` connects dex host capabilities to
`fluxplane-core/runtime/system.System`; plugins never receive `system.System`
directly.

Endpoint/auth details discovered during auth should be stored as explicit auth
purposes when they affect routing. For Atlassian Cloud, Jira and Confluence may
need a `cloud_id` purpose so host-backed HTTP can call
`https://api.atlassian.com/ex/<product>/<cloud_id>/...` instead of the site URL.
Treat those IDs as routing metadata, not model-visible secrets, and keep them out
of operation inputs unless there is a user-facing reason.

## Auth, Endpoints, And Host Capabilities

Preferred integration flow:

1. User configures endpoint/auth via dex auth, secrets, or host-owned config.
2. Operation receives `endpoint_ref` plus model-provided intent.
3. Plugin calls the SDK host client with endpoint ref and required auth purpose.
4. Runtime resolves URL/credentials and executes through host capabilities.

Capability guidance:

- HTTP: use host HTTP with `endpoint_ref` and auth purposes where applicable.
- Files: use blob refs; `blob.read` for inspection, `blob.write` only for
  intentional blob creation/modification.
- Env lookup: use host env lookup only for compatibility/discovery; prefer
  named secrets, auth purposes, and endpoint refs for new designs.
- Provider calls: use generic provider grants such as `container`, `cluster`,
  `database`, `search`, or existing stable names; avoid deployment-specific
  capability names.
- Preserve dex request `path` and `query` exactly when bridging to host-system
  HTTP. Bugs here can pass direct `dex` tests but fail through coder because the
  plugin sees a system-backed HTTP implementation rather than the direct dex
  transport.
- If exactly one product endpoint is registered, runtime may inject its
  `endpoint_ref` for operation and datasource calls. Tests should cover both
  explicit `endpoint_ref` and single-endpoint defaulting so coder prompts with
  empty input continue to work.

- Authenticated plugins must declare auth fields and secret purposes in the
  manifest, then request those purposes through host capabilities. For example,
  Tavily declares `api_key` and performs HTTP with
  `BearerTokenPurpose: "api_key"`; plugin operations must never call
  `os.Getenv("TAVILY_API_KEY")` or otherwise read env directly. Use
  `dex auth connect` or `dex auth connect auto` to move environment-sourced
  material into host-owned dex auth storage before live tests.

## Input And Operation Design

- Keep schemas small and explicit; avoid `map[string]any` unless the upstream
  API genuinely requires arbitrary structured input.
- Every exported operation/datasource input field must have a useful JSON Schema
  description. Models and UIs rely on it to choose fields.
- Escape commas in `jsonschema` tag values as `\\,` in Go source, e.g.
  `jsonschema:"description=User ID\\, mention\\, or name"`. Unescaped commas
  split tag options; a single backslash makes `reflect.StructTag.Get` fail.
- Prefer typed fields for operations. Datasource compatibility inputs may also
  expose generic `query`, `entity`, `datasource`, and `filters`; document every
  supported `filters` key and map aliases into the same validation path as typed
  fields.
- Datasource `Search`, `List`, and `Get` handlers should accept host/agent
  request shapes, not only a custom plugin shape: e.g. top-level `query`,
  `filters.query`, `filters.channel`, `filters.thread_ts`.
- Separate reads from writes; mark destructive operations with effects/risk.
- Return normalized, bounded results: cap lists, trim logs, include continuation
  fields, and avoid dumping unbounded upstream responses.
- Keep context providers concise; summarize useful state rather than mirroring
  full operation results.
- Use stable integration-level names/docs; do not expose credential source,
  local path, or runtime implementation details in the user-facing contract.

## Security And Validation

- Validate required inputs before host calls.
- Refuse ambiguous mutually exclusive input, such as both `url` and
  `endpoint_ref`.
- Bound network/file reads with timeouts and max-byte limits.
- Do not follow redirects, read files, or access private networks unless host
  policy and operation grants allow it.
- Treat host provider payloads as untrusted JSON: decode into typed structs and
  validate before executing host-side effects.

## Tests

Normal `go test ./...` must not require network, Docker, Kubernetes, cloud
credentials, or local user config. Live upstream tests must be opt-in and gated.

Focused tests should cover:

- Manifest validity, operation schemas, capability declarations, effects, risk.
- Generated input schema quality: every manifest operation/datasource property
  has non-empty `description`; comma-containing descriptions are asserted so
  escaping regressions are caught.
- Datasource generic request compatibility: top-level `query`, `filters`
  aliases, and entity-specific identifiers.
- Operation behavior with fake host capabilities or test transports.
- Auth/endpoint flows proving secrets stay out of params, results, and logs.
- IO-free enforcement via `internal/pluginiofree`.
- Host-system HTTP bridge tests: path/query preservation, endpoint ref
  propagation, auth purpose routing, and direct dex versus embedded/coder
  execution parity.
- Missing endpoint/auth, invalid params, denied capability, and other error
  paths.
- Live provider plugins with auth, such as Tavily, should have tests/probes that
  confirm the manifest advertises auth/secret purposes while runtime calls obtain
  credentials only through the dex secret broker.

## Local And Live Testing

Install local code and the managed development plugin:

```sh
task install
dex plugin install <name> --dev-plugin <name>=./plugins/<name>
dex plugin list -o json
dex op show <plugin>.<operation> -o json
dex <plugin> <operation path> -h
```

After changing plugin code, reinstall the dev plugin before testing through dex,
coder, or datasource tools. A stale binary in `~/.dex/plugins/bin` can make fixed
code appear broken. Confirm `dex plugin list -o json` shows the expected plugin
path/version.

For authenticated plugins, connect secrets through dex before running live
commands. `auto` may read manifest-declared environment variables in the host
process and save them to dex auth storage, but plugin code still consumes the
value only through the runtime secret broker:

```sh
# Explicit value path. Avoid shell history or logs that expose real keys.
dex auth connect tavily -f api_key="$TAVILY_API_KEY"

# Auto path. Requires TAVILY_API_KEY in the dex process environment.
dex auth connect auto tavily

dex auth status tavily -o json
```

Live-test at three levels when possible:

```sh
# 1. Direct dex operation/datasource smoke test.
dex <plugin> <operation path> --json '{"field":"value"}'
dex datasource search <datasource> --query 'example' --limit 3

# 2. Host/agent datasource path. Ask coder to use the datasource by name and
# verify it can find real data through the installed managed plugin.
coder --yolo --model=codex --input \
  'Use datasource_search for <entity> in <datasource> with filters {...}; summarize the result.'

# 3. End-to-end behavior. Ask coder to perform the intended user task, then
# inspect whether it chose the right operation/datasource and returned bounded,
# secret-free output.
coder --yolo --model=codex --input \
  'Using the <plugin> integration, do <task>. Keep the output concise.'
```

For datasource compatibility, live-test both the typed/top-level shape and the
generic `filters` shape that agents use.

For provider-style integrations, test both the concrete provider operation and
its datasource. This catches missing datasource grants even when direct operation
calls pass:

```sh
dex op run tavily.search '{"query":"fluxplane dex","max":2}' -o json \
  --dev-plugin tavily=./plugins/tavily
dex datasource search tavily.web_search '{"query":"fluxplane dex","limit":2}' -o json \
  --dev-plugin tavily=./plugins/tavily
```

For Jira and other Atlassian Cloud plugins, always include a host-backed probe in
addition to direct `dex` CLI checks. Direct dex transport can succeed while coder
fails if `cloud_id`, path/query forwarding, or endpoint default injection is
missing.

Do not commit `replace` directives for release prep. The root module should
build without plugin-module requirements; plugin modules should depend on
released dex SDK versions.

## Release Hygiene

- Root dex and plugin module versions are released together.
- Root tags: `vX.Y.Z`; plugin tags: `plugins/<plugin>/vX.Y.Z`.
- Changelog entries should separate SDK/runtime changes from plugin changes.
- Before release, verify root module, each plugin module, and `git diff --check`.
- Temporary local module overrides for unpublished tags must stay uncommitted.
