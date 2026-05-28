# Agent Instructions

- Do not create git commits unless the user explicitly asks for a commit in the current task.
- It is OK to stage files only when the user explicitly asks for staging or committing.
- Before any requested commit, show or summarize the staged changes so the user can confirm the commit boundary.
- Do not keep compatibility wrappers, bridges, adapters, deprecated paths, or old APIs only for backwards compatibility. This is a green-field project with no external users; prefer removing the old path and updating call sites directly.

## Release Process

When the user asks for a release, do all of this as one release task:

1. Determine the previous root version tag, e.g. `git tag --sort=-version:refname`.
2. Derive the next semantic version from the diff since the previous root tag.
3. Update `CHANGELOG.md` with clean, deduped release notes for the new version based on `git log` and `git diff --stat <previous-tag>..HEAD`.
4. Update release version references together with the changelog:
   - root workspace replacement in `go.work`
   - plugin module requirements in `plugins/*/go.mod`
   - plugin manifest versions in `plugins/*/manifest.go`
   - builtin plugin versions such as `runtime/websearch_builtin.go`
   - release docs and README examples
5. Run release verification before committing:
   - `go test ./...`
   - `go test ./plugins/<name>/...` for each plugin module, or an equivalent loop over `plugins/*`
   - `git diff --check`
6. Stage the release-ready tree, summarize the staged boundary, and get confirmation unless the current user message already explicitly confirms the commit/release.
7. Commit with a release message such as `Prepare vX.Y.Z release`.
8. Create the root tag and matching plugin module tags:
   - `git tag vX.Y.Z`
   - `git tag plugins/<plugin>/vX.Y.Z` for every installable plugin module
9. Push the branch and all release tags together.
10. Create the GitHub release from the `CHANGELOG.md` section for the new version, for example with `gh release create vX.Y.Z --title vX.Y.Z --notes-file <notes-file>`.
11. Final-check that the working tree is clean, the root tag points at `HEAD`, and the GitHub release exists.
