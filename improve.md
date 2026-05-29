# Improvement Log

## Correctness: make plugin cache concurrency-safe

- [x] Identified a correctness risk in `core/pluginbinding.Cache`: it stored values in a plain map without synchronization.
- [x] Added a `sync.RWMutex` around cache reads and writes to prevent concurrent map access panics when a shared cache is used across operations/providers.
- [x] Ran focused formatting: `go fmt ./core/pluginbinding`.
- [x] Ran focused race-enabled tests: `go test -race ./core/pluginbinding`.
- [x] Ran repository tests: `go test ./...`.
- [x] Summarize final status.

## Docs/Security: clarify secret handling in README

- [x] Reviewed the README `Secrets` section and `docs/plugin-dev.md` guidance.
- [x] Added README guidance that resolved secret values must not appear in model-visible inputs, results, errors, logs, diagnostics, events, or context.
- [x] Documented the preferred pattern: pass intent plus secret purposes and let the host resolve material inside the scoped grant boundary.
- [x] No code changes required; validation was a targeted docs readback.

## Correctness: prompt for missing auth fields after partial prefill

- [x] Identified a correctness bug in `AuthService.Connect`: when any `PrefilledFields` value was supplied, the interactive prompt loop was skipped entirely, leaving remaining required fields missing.
- [x] Changed `Connect` to always walk manifest auth fields, skipping only the fields already supplied by `PrefilledFields`.
- [x] Added `TestConnectPromptsForMissingFieldsWhenPartiallyPrefilled` to cover a partially-prefilled `asterisk` auth connect flow.
- [x] Ran focused test: `go test . -run TestConnectPromptsForMissingFieldsWhenPartiallyPrefilled`.
- [x] Ran repository tests: `go test ./...`.

## Security: harden Docker archive extraction against symlink escapes

- [x] Reviewed `internal/dockerhost` archive extraction path validation for tar entries copied out of containers.
- [x] Tightened regular-file extraction so overwrite mode refuses to follow an existing symlink at the destination path.
- [x] Added symlink target validation so archive symlink entries cannot point outside the extraction root.
- [x] Added focused regression tests for escaping symlink targets and overwriting an existing symlink destination.
- [x] Ran focused tests: `go test ./internal/dockerhost -run TestExtractTar`.
- [x] Ran package tests: `go test ./internal/dockerhost`.
- [x] Ran repository tests: `go test ./...`.

## Security: mark Grafana credentials as sensitive secrets

- [x] Identified that the Grafana manifest declared the service account token and basic-auth password with `secret=false`, so auth connect flows would treat them as non-secret config inputs.
- [x] Marked Grafana token and password auth fields as secret/sensitive while leaving username non-secret.
- [x] Added `TestManifestMarksCredentialsSensitive` as a focused manifest regression test.
- [x] Formatted touched Go files with `gofmt` (native `go fmt` could not run in the nested module because its `go.sum` lacks the root replacement checksum).
- [x] Ran focused Grafana plugin tests: `go test ./plugins/grafana/...`.
- [x] Summarize final status.

## Security: reject unsafe Atlassian attachment filenames

- [x] Identified that attachment upload normalization accepted path-like names and control characters in `filename`, which could leak path context or produce unsafe multipart metadata.
- [x] Hardened `internal/atlassian.BuildAttachmentUploadRequest` to reject dot path segments, forward/backslash path separators, and ASCII control characters after trimming.
- [x] Added a focused regression test for traversal-like, Windows path-like, newline-containing, and dot-segment filenames.
- [x] Ran formatting: `go fmt ./internal/atlassian`.
- [x] Ran focused tests: `go test ./internal/atlassian`.
- [x] Ran dependent Confluence plugin tests: `go test ./plugins/confluence/...`.

## Security: filter unsafe web search result URLs

- [x] Identified that web search provider results could carry non-web or control-character URLs into operation output/datasource records.
- [x] Added shared `internal/websearch.NormalizeResultURL` validation for absolute `http`/`https` URLs with hosts and no control characters.
- [x] Applied shared URL filtering to datasource record creation plus DuckDuckGo and Tavily provider result parsing.
- [x] Added regression tests for `javascript:` URLs, control-character URLs, missing-host URLs, and safe URL retention.
- [x] Formatted touched Go files with `gofmt` (native `go fmt` for plugin modules still fails because their `go.sum` lacks the root replacement checksum).
- [x] Ran focused tests: `go test ./internal/websearch`, `go test .` in `plugins/duckduckgo`, and `go test .` in `plugins/tavily`.
- [x] Ran repository tests: `go test ./...`.

## Security: reject unsafe system HTTP capability URLs

- [x] Identified that `fluxplaneplugin.systemHTTPRequestURL` accepted relative URLs, non-HTTP schemes, missing-host URLs, and embedded userinfo before handing requests to the host network capability.
- [x] Hardened URL validation to require absolute `http`/`https` URLs with a host and to reject userinfo so credentials are not embedded in request URLs.
- [x] Added focused regression coverage for relative, missing-host, `file:`, and `user:password@host` URL inputs.
- [x] Formatted touched files with `gofmt` after native `go fmt` in the nested module failed due an existing missing replacement checksum.
- [x] Ran focused tests: `go test ./fluxplaneplugin -run TestSystemHTTPRequestURL`.
- [x] Ran package tests: `go test ./fluxplaneplugin`.
