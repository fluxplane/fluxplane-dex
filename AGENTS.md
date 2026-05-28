# Agent Instructions

- Do not create git commits unless the user explicitly asks for a commit in the current task.
- It is OK to stage files only when the user explicitly asks for staging or committing.
- Before any requested commit, show or summarize the staged changes so the user can confirm the commit boundary.
- Do not keep compatibility wrappers, bridges, adapters, deprecated paths, or old APIs only for backwards compatibility. This is a green-field project with no external users; prefer removing the old path and updating call sites directly.
