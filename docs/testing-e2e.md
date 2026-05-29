# E2E Testing Notes

This page is a field guide for future maintainers running live end-to-end checks.
It focuses on practical commands, privacy hygiene, and failure modes that unit
and fixture tests do not catch.

## Principles

- Prefer small live probes over broad dumps. Use low limits (`1` to `3`) and
  validate shape/status rather than printing full records.
- Do not paste or commit tokens, bearer headers, secret material, or full record
  bodies from private systems.
- Reinstall the `dex` CLI after runtime or plugin changes before doing CLI E2E
  checks. Otherwise you may be testing an old binary with a dirty worktree.
- Use `--dev-plugin NAME=./plugins/NAME` when testing plugin changes from the
  working tree.
- Run targeted package tests first, then broader tests once the failure is
  understood.

## Basic local verification

```bash
go test ./core/pluginbinding ./runtime ./plugins/gitlab
```

After changing runtime/plugin surfaces, reinstall the CLI used by E2E commands:

```bash
task install
```

A full suite is still useful before release work:

```bash
go test ./...
```

If the full suite fails in a large CLI golden/manifest test, inspect whether the
failure is caused by your change or by local plugin availability/output drift.
Do not hide unrelated failures; record them in the handoff.

## Endpoint checks

Live operations that route through host HTTP need a registered endpoint. Example
for GitLab:

```bash
dex endpoint add https://gitlab.example.com \
  --id gitlab-example \
  --product gitlab \
  --protocol https \
  --source manual

dex endpoint test gitlab-example -o compact
```

Use an existing endpoint if one is already registered. Endpoint records are not
secrets, but credential refs can reveal infrastructure details, so avoid copying
large endpoint listings into issues or chat unless needed.

## GitLab live probes

Use compact/pass-fail style probes. The goal is to verify capabilities without
leaking private records.

```bash
dex op run gitlab.auth.test \
  '{"endpoint_ref":"gitlab-example"}' \
  -o json \
  --dev-plugin gitlab=./plugins/gitlab

# List one project and one merge request. Inspect only enough JSON to extract
# an ID/path/ref for follow-up show calls.
dex op run gitlab.project.list \
  '{"endpoint_ref":"gitlab-example","limit":1}' \
  -o json \
  --dev-plugin gitlab=./plugins/gitlab

dex op run gitlab.mr.list \
  '{"endpoint_ref":"gitlab-example","limit":1,"state":"all"}' \
  -o json \
  --dev-plugin gitlab=./plugins/gitlab
```

Important regression to keep checking: GitLab project paths contain slashes and
must be sent to the API as escaped path segments (`group%2Frepo`). Endpoint-routed
HTTP must preserve the escaped path. If `gitlab.project.show` works by numeric ID
but fails by `path_with_namespace`, suspect path escaping in host HTTP routing.
The same issue can break `gitlab.mr.show` when using refs like `group/repo!123`.

## GitLab index and datasource probes

Build a bounded live index when validating datasource behavior. Use low limits
unless intentionally refreshing a full local index.

```bash
dex op run gitlab.index.build \
  '{"endpoint_ref":"gitlab-example","limit":10,"user_limit":10,"group_limit":10,"issue_limit":10,"mr_limit":10}' \
  -o json \
  --dev-plugin gitlab=./plugins/gitlab

dex index status gitlab -o json
```

Then verify every indexed datasource can search:

```bash
for ds in \
  gitlab.projects \
  gitlab.users \
  gitlab.groups \
  gitlab.issues \
  gitlab.merge_requests; do
  dex datasource search "$ds" --query a --limit 2 -o compact \
    --dev-plugin gitlab=./plugins/gitlab
done
```

Also verify canonical lookup for each entity. Use known low-sensitivity IDs from
the local index, not full record bodies:

```bash
dex lookup 'group/repo' --plugin gitlab --entity gitlab.project --limit 2 -o compact \
  --dev-plugin gitlab=./plugins/gitlab

dex lookup 'username' --plugin gitlab --entity gitlab.user --limit 2 -o compact \
  --dev-plugin gitlab=./plugins/gitlab

dex lookup 'group' --plugin gitlab --entity gitlab.group --limit 2 -o compact \
  --dev-plugin gitlab=./plugins/gitlab

dex lookup 'group/repo#1' --plugin gitlab --entity gitlab.issue --limit 2 -o compact \
  --dev-plugin gitlab=./plugins/gitlab

dex lookup 'group/repo!1' --plugin gitlab --entity gitlab.merge_request --limit 2 -o compact \
  --dev-plugin gitlab=./plugins/gitlab
```

## Coder datasource surface probe

When validating what an agent sees, run a separate coder process and explicitly
ask for pass/fail output only:

```bash
coder --yolo --disallow-shell --model=codex --input \
  "GitLab datasource API E2E, no record bodies. For each entity gitlab.project, gitlab.user, gitlab.group, gitlab.issue, gitlab.merge_request call datasource_search(query='a', limit=2), datasource_list(limit=2), and datasource_get on first search result id if any. Return only compact PASS/FAIL/EMPTY per method/entity and error messages."
```

This catches integration issues that direct `dex` commands may miss, including
surface preparation, entity naming, list/get/search wiring, and host index access.

## Debugging checklist

- Operation says `endpoint_ref is required`: pass an endpoint ref in the JSON
  payload or make sure the host injected one for the capability being tested.
- Operation works by numeric ID but fails by namespaced path: check escaped path
  handling (`%2F`) through `HostHTTPClient` and runtime `httpRequestURL`.
- `dex lookup` prints the input text with no matches in compact mode: rerun with
  `-o json` and inspect `results.available.<plugin>.count` rather than assuming
  command failure.
- Datasource search passes but coder cannot see results: verify the coder surface
  prepared the plugin/datasource and that entity names match (`gitlab.project`,
  not `gitlab.projects`).
- Large record output appears in stderr/stdout: stop and rerun with smaller
  limits or a stricter prompt. Summaries should be PASS/FAIL/EMPTY only.
