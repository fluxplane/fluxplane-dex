# Tool Ergonomics Review: SQL Plugin Endpoint and Read-Only Hardening

Date: 2026-05-29
Agent: coder
Topic: Tool use and workflow during SQL plugin endpoint credential, datasource access, JSON Schema, and read-only execution fixes

## Scope

This review covers the current coder session where I investigated and fixed the SQL plugin and runtime SQL provider. The work included:

- Diagnosing a live `sql.query` failure against `dev-backend-acd-mysql`.
- Resolving Kubernetes endpoint `credential_ref` material in the host SQL provider.
- Granting provider access to `sql.query_rows` datasource calls.
- Adding JSON Schema metadata for SQL query inputs.
- Hardening SQL read-only validation and then adding host-level read-only execution controls.
- Running unit tests, live `dex` operations, staging, and committing/amending the changes.

This is a reflection on my actual tool use in this session, not a clean-room or sub-agent review.

## What worked well

- Native project and Go tools were effective. `go_test`, `go_fmt`, `git_diff`, `git_status`, `project_task_run`, and direct `process_run` for `dex` were enough to iterate without broad shell scripts.
- Live E2E checks caught real integration issues that unit tests alone would not have exposed:
  - `dex op run sql.query ...` verified endpoint credential resolution against MySQL.
  - `dex datasource search sql.query_rows ...` exposed the missing provider access grant.
  - Unsafe SQL examples verified plugin-level rejection before execution.
- `dex op show sql.query -o json --dev-plugin sql=./plugins/sql` was a good way to validate that struct `jsonschema` tags emitted the expected JSON Schema, including `required`, `enum`, `minimum`, and `maximum`.
- The final user challenge about read-only enforcement was valuable. It exposed an important gap: I had hardened parsing but had not initially enforced read-only behavior at the connection/transaction layer.
- Using direct `process_run` for known commands was appropriate when no higher-level tool existed, especially for `dex plugin install`, `dex op run`, and `dex op show`.

## What was bad or inefficient

- I accidentally invoked duplicate operations via `multi_tool_use.parallel` several times. This caused repeated installs, repeated live queries, repeated tests, and duplicated output. It wasted time and made the transcript noisier.
- I used `multi_tool_use.parallel` incorrectly for potentially redundant or order-sensitive steps. For example, installing a plugin and immediately querying it was safe enough only because installation was already mostly idempotent, but it would have been cleaner to install once, then run live checks.
- I inserted code in the wrong place twice:
  - The first insertion of SQL credential helper functions landed inside `sqlProviderTargetFromSecret` and required repair.
  - The first insertion of `TestHostSQLProviderUsesReadOnlyTransaction` landed inside another test and caused syntax/test failures.
- I committed before the read-only transaction concern was fully addressed. The later `git commit --amend --no-edit` was acceptable, but I should have considered execution-layer read-only enforcement before committing the first version.
- I initially treated query parsing as sufficient read-only protection. That was a security design mistake. Parsing should be a guardrail, not the only enforcement boundary.
- The SQLite read-only behavior was more subtle than I first assumed. `sql.TxOptions{ReadOnly: true}` alone did not stop a write with the SQLite driver, and only the new test revealed that. I should have known to test the provider boundary directly rather than relying on intent.
- I did not run `git diff --check` before committing. It likely would not have changed the outcome, but it is a cheap hygiene step before a user-requested commit.

## What I would improve

- Use a tighter sequence for live plugin validation:
  1. `project_task_run install`
  2. `dex plugin install <name> --dev-plugin <name>=./plugins/<name>`
  3. `dex op show ... -o json`
  4. `dex op run ...`
  5. `dex datasource search ...`
  6. `go test` focused package
  7. `go test ./...`
- Avoid duplicated parallel calls. Parallelization should only group independent reads/tests, not repeated identical commands.
- Before committing security-sensitive fixes, explicitly ask: what enforces the invariant below the parser or UI layer?
- For provider code, add host-level tests earlier. Plugin tests are not enough when the actual risk boundary is runtime/provider execution.
- Use `git diff --check` before any user-requested commit.
- When editing existing Go functions, inspect surrounding function boundaries more carefully with line-numbered reads before inserting large blocks.

## Candidate activation sets

### Surface: `dex_plugin_dev_loop`

Aliases:

- `plugin-dev`
- `dex-dev-plugin`
- `local-plugin-validation`

Targets:

- `project_task_run` for `task install`
- `process_run` for `dex plugin install <plugin> --dev-plugin <plugin>=./plugins/<plugin>`
- `process_run` for `dex op show <plugin.operation> -o json --dev-plugin ...`
- `process_run` for `dex op run ... --dev-plugin ...`
- `process_run` for `dex datasource search ... --dev-plugin ...`
- `go_test` for plugin module and selected runtime packages

Concrete workflow that would benefit:

- Any session modifying a plugin manifest, operation input/output structs, datasource specs, or host provider implementation. The workflow should install local code, validate generated operation schema, run a live operation/datasource smoke test when credentials/endpoints are available, and run focused tests.

### Surface: `sql_provider_safety_loop`

Aliases:

- `sql-safety`
- `readonly-sql-provider`
- `sql-plugin-hardening`

Targets:

- `file_read` and `go_outline` for `plugins/sql/operations.go` and `runtime/sql_provider.go`
- `go_test` for `plugins/sql` and `./runtime`
- `process_run` for `dex op run sql.query ...`
- `process_run` for `dex datasource search sql.query_rows ...`
- `process_run` for `dex op show sql.query -o json ...`

Concrete workflow that would benefit:

- Any change involving `sql.query`, SQL datasource behavior, endpoint credential resolution, or query safety. It should exercise both plugin-level validation and host-provider-level enforcement, including direct provider tests that bypass plugin parsing.

### Surface: `go_commit_hygiene`

Aliases:

- `precommit-go`
- `commit-boundary-check`

Targets:

- `go_fmt`
- `go_test`
- `git_diff --check` via `process_run` until native support exists
- `git_diff --staged --stat`
- `git_status`
- `git_commit`

Concrete workflow that would benefit:

- User explicitly asks to commit after code changes. It should run formatting, focused tests, broader tests when practical, whitespace checks, show staged boundary, then commit.

## Candidate reaction rules

### Rule: SQL plugin or runtime provider files changed

Evidence assertion:

- A staged or unstaged diff touches any of:
  - `plugins/sql/operations.go`
  - `plugins/sql/manifest.go`
  - `plugins/sql/operations_test.go`
  - `runtime/sql_provider.go`
  - `runtime/sql_provider_test.go`
  - `runtime/runner_capability_test.go`

Subject/target:

- Subject: workspace repository
- Target: SQL plugin and host SQL provider

Activation set to enable:

- `sql_provider_safety_loop`

Rationale:

- This session showed that SQL plugin changes need both schema/operation validation and host-level safety tests. Plugin-only tests missed the read-only transaction gap.

### Rule: User asks to commit code changes

Evidence assertion:

- User says `commit`, `please commit`, `commit your changes`, or equivalent.

Subject/target:

- Subject: current git workspace
- Target: staged or to-be-staged code changes

Activation set to enable:

- `go_commit_hygiene`

Rationale:

- The session involved a user-requested commit, but I did not run `git diff --check`. A commit hygiene surface would make this less likely.

### Rule: Generated JSON Schema is relevant

Evidence assertion:

- A diff changes public plugin input/output structs or `jsonschema` tags, especially under `plugins/*/operations.go`.

Subject/target:

- Subject: plugin operation contract
- Target: generated operation schema

Activation set to enable:

- `dex_plugin_dev_loop`

Rationale:

- `dex op show ... -o json` was the decisive validation that `minimum`, `maximum`, `required`, and descriptions appeared as intended.

## Honest self-critique

I moved too fast after the first successful live query. I treated the parser hardening as the main safety improvement and did not initially ask whether the database connection itself was constrained. The user had to point out the more important security boundary. That is on me.

I also overused parallel tool calls and duplicated several commands. The duplicate installs and live queries were not harmful, but they were sloppy. The tool system made it easy to run many things at once, and I used that convenience without enough discipline.

My code-editing accuracy was mixed. I repaired the mistakes quickly, but large insertions into existing files should have been preceded by line-numbered reads around function boundaries. The broken insertions caused avoidable parse/test failures.

The final result is stronger because it now includes provider-level read-only execution and direct runtime tests, but the workflow got there through user correction rather than my own first-pass design review.

## Bottom line

The tools were capable enough for this work: native Go tests, targeted file edits, live `dex` commands, schema inspection, and git operations covered the full loop. The main weakness was my workflow discipline. I should have avoided duplicate parallel calls, validated execution-layer security earlier, and run a stricter pre-commit hygiene sequence before committing.
