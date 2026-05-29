# Plugin Development

Dex plugins are model-facing capability surfaces. They should describe what an
operation can do, validate the request, and call the dex SDK. They should not
own host IO.

## Principles

- Keep plugins IO-free. Plugin code must not read process environment, open
  workspace files, create raw HTTP clients, open sockets, shell out, or talk
  directly to Docker, Kubernetes, databases, or other host services.
- Keep secrets out of operation params. A model may see operation params, so
  credentials must be requested by purpose or endpoint reference and resolved
  by the host/runtime.
- Keep dependency direction one-way. Plugin modules depend on the dex SDK and
  protocol. The root dex module must not import plugin modules.
- Treat the host protocol as the IO boundary. Plugins ask for HTTP, blobs,
  environment lookup, endpoint resolution, auth material, and provider calls
  through the SDK; the host decides whether and how to satisfy those requests.
- Prefer endpoint refs over raw connection material. Operations should accept
  `endpoint_ref` when possible; the runtime resolves the actual URL, token,
  password, or certificate material outside the model-visible payload.
- Declare the smallest useful capability. Read-only operations get read grants;
  write, mutation, network, or provider access must be explicit.

## Architecture

Plugin modules are leaf modules:

- `plugins/<name>` imports `github.com/fluxplane/fluxplane-dex/core/pluginbinding`
  and any local plugin implementation dependencies.
- Root runtime packages may share protocol DTOs through internal host packages
  such as `internal/<provider>host`.
- Root runtime packages must not import `github.com/fluxplane/fluxplane-dex/plugins/<name>`.

If the runtime needs a host implementation for a plugin-facing provider, put
the host implementation and its wire DTOs in an internal package owned by the
root module. The plugin and the host can then exchange JSON across the host
protocol without a compile-time module edge from root back into the plugin.

## Host Capabilities

Use the SDK capability APIs instead of direct IO:

- HTTP goes through the host HTTP client. Pass `endpoint_ref` and auth purposes
  when applicable; do not place bearer tokens, passwords, cookies, or API keys
  in params.
- Files go through blob refs. Use `blob.read` for inspection and `blob.write`
  only for operations that intentionally create or modify workspace blobs.
- Environment lookup goes through host env lookup and should be used only for
  compatibility or discovery paths. Prefer named secrets, auth purposes, and
  endpoint refs for new operation design.
- Provider calls go through host provider capability grants. Provider names
  should stay generic, for example `container`, `cluster`, `database`,
  `search`, or existing stable names, rather than encoding a deployment detail
  into the capability set.

`fluxplaneplugin.Config.System` wires dex host capabilities to
`fluxplane-core/runtime/system.System`. When an embedding host passes this
system, dex HTTP, workspace blob, and environment calls are resolved through the
same host boundary as native core plugins. Plugins never receive the
`system.System` directly.

## Auth And Endpoints

Design operations so the model supplies intent, not credentials:

- Accept `endpoint_ref` for configured services.
- Request auth by purpose, such as bearer token, username/password, or named
  headers, through the host capability request.
- Let the runtime combine endpoint and auth material before making the host
  HTTP call.
- Never log, echo, serialize, or include resolved secret values in operation
  results, diagnostics, errors, events, or context blocks.

For new integrations, the preferred flow is:

1. The user configures an endpoint and its auth material through dex auth,
   secrets, or host-owned configuration.
2. The operation receives an `endpoint_ref`.
3. The plugin calls the SDK host client with that ref and any required auth
   purpose.
4. The runtime resolves URL and credentials and sends the request through the
   host capability implementation.

## Operation Design

- Keep schemas small and explicit. Avoid generic `map[string]any` params unless
  the upstream API genuinely requires arbitrary structured input.
- Separate read operations from write operations. This lets manifests and grants
  express least privilege.
- Mark destructive operations with clear effects and risk metadata.
- Return normalized, bounded results. Cap large lists, trim logs, and include
  continuation fields instead of dumping unbounded upstream responses.
- Keep context providers concise. Context blocks should summarize useful state,
  not mirror full operation results.
- Prefer stable, integration-level language in operation names and docs. Avoid
  baking a specific credential source, local path, or runtime implementation
  detail into the user-facing contract.

## Security

- Assume operation params and results may be visible to a model, logs, traces,
  or UI. Do not put secrets there.
- Validate all required inputs before calling host capabilities.
- Refuse ambiguous requests that combine mutually exclusive fields, such as
  both `url` and `endpoint_ref`.
- Bound network and file reads with timeouts and max byte limits.
- Do not follow redirects, read files, or access private networks unless the
  host policy and operation grant allow it.
- Do not grant write capabilities to read-only operations.
- Treat host provider payloads as untrusted JSON. Decode into typed structs and
  validate before executing host-side effects.

## Testing

Every plugin should have focused tests for:

- Manifest validity, operation schemas, capability declarations, effects, and
  risk metadata.
- Operation behavior using fake host capabilities or test transports.
- Auth and endpoint flows that verify secrets stay out of params, results, and
  logs.
- IO-free enforcement with `internal/pluginiofree` checks.
- Error handling for missing endpoints, missing auth, invalid params, and denied
  capabilities.

Live upstream tests must be opt-in and clearly gated. Normal `go test ./...`
should not require network access, Docker, Kubernetes, cloud credentials, or
local user configuration.

## Local Development

Install and test local plugin code with the development override:

```sh
task install
dex plugin install <name> --dev-plugin <name>=./plugins/<name>
dex plugin list -o json
dex op show <plugin>.<operation> -o json
dex <plugin> <operation path> -h
```

Do not use committed `replace` directives for release preparation. The root
module should build without plugin-module requirements, and plugin modules
should depend on released dex SDK versions.

## Release Hygiene

- Root dex versions and plugin module versions are released together.
- Root tags use `vX.Y.Z`.
- Plugin module tags use `plugins/<plugin>/vX.Y.Z`.
- Changelog entries should separate SDK/runtime changes from individual plugin
  changes.
- Before release, verify the root module, each plugin module, and `git diff
  --check`.

If a release requires temporary local module overrides to verify unpublished
tags, keep them out of committed files.
