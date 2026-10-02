# Migrating to 0.29

## `wtm create --output json` answers with an envelope

`create` can now make several worktrees in one run, so its JSON document is always an envelope, even for a single branch.

Before:

```json
{"branch": "feat/login", "path": "/repo/.worktrees/feat-login", "already_exists": false, ...}
```

After:

```json
{"results": [{"branch": "feat/login", "path": "/repo/.worktrees/feat-login", "already_exists": false, ...}], "failed": []}
```

A script reading `.path` reads `.results[0].path`. A branch that could not be created is in `failed`, with its `error`, its `exit_code`, and its `path` when the worktree exists but an `on_create` hook failed. The process still exits with the first failure's code, so a script that only checks the exit code needs no change.

## The wizard asks for a list of branches

`wtm create` without arguments now asks for one or more branches: type a name, press tab to add another, enter to continue. `wtm create <branch>` with a single argument skips that step as before; with several arguments the list opens pre-filled. The dashboard's create (`wtm ui`) asks the same list; each worktree appears in the list as soon as it exists, and the cursor lands on the first.
