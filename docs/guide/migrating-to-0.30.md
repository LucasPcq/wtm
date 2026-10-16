# Migrating to 0.30

`wtm create` is the first command on wtm's command engine: one contract for the command line, the form and agents. What a script reading its output must change is below; the other commands keep their 0.29 contract until they move, each release saying which.

## `wtm create --output json` writes the v2 envelope

Every run writes one document on stdout, a refusal before anything is created included. Before:

```json
{"results": [{"branch": "feat/login", "path": "/repo/.trees/feat-login", "already_exists": false, "metadata": {...}}], "failed": []}
```

After:

```json
{"v": 2, "command": "create", "ok": true, "op": "3f2a9c01e4b7d6a8",
 "items": [{"subject": "feat/login", "status": "done",
            "detail": {"path": "/repo/.trees/feat-login", "metadata": {...}, "existing_branch": false}}],
 "follow_ups": [], "warnings": []}
```

| 0.29 | 0.30 |
| --- | --- |
| `.results[i].branch` | `.items[i].subject` |
| `.results[i].path`, `metadata`, `existing_branch`, `origin_state`, `upstream`, `env_ports` | `.items[i].detail.…`, same names |
| `.results[i].already_exists: true` | `.items[i].status: "done"` with `reason: "unchanged"` |
| `.results[i].isolation` | `.items[i].detail.metadata.isolation` |
| `.results[i].warnings` (strings) | `.items[i].warnings`, each `{code, params, message}`; warnings about the whole run in `.warnings` |
| `.failed[i]` | an item with `status: "failed"` and its `error`; `reason: "incomplete"` when the worktree exists but `on_create` failed |
| `.skipped[i]` (interrupted) | an item with `status: "skipped"`, `reason: "interrupted"` |
| `.results[i].origins` | gone: pass the flag for a guaranteed answer |
| a refusal on stderr, nothing on stdout | `.error`, with `kind`, `code`, `message` and, for an invalid request, `fields[]` naming each problem (`path`, `code`, `flag`) |

The whole contract is in the [JSON contract](../../internal/surface/cli/agents/assets/using-wtm/references/json.md#the-v2-envelope-create) the agent skill ships.

## Exit codes of `create`

- A request `create` refuses before creating anything exits `2`: a branch another worktree holds, a folder already taken, a derived-name clash (`10` before), an unknown `--from` or base branch (`11` before). The reason is in `.error.fields[].code` (`create.checked_out_elsewhere`, `create.path_exists`, `create.name_clash`, `not_found`).
- A repository with no config still exits `12`.
- Another wtm operation on the same worktree makes it exit `24` (`lock.held`), nothing created.
- A run applied with `--expect-fingerprint` after the repository changed exits `25` (`plan.stale`), nothing created.

## A batch is checked whole before it runs

`wtm create a b c` checks every branch first. A folder already taken for `b` used to fail `b` alone while `a` and `c` were created; it now refuses the call, nothing created, `b` named in `.error.fields`. A hook that fails during the run still fails its branch only.

## `--yes` is not needed in JSON

`wtm create --output json` never asks anything: `--yes` is accepted and changes nothing.

## The fast-forward never asks

`--ff` (or the answer remembered for it) is best effort everywhere: a source that cannot be fast-forwarded, or whose worktree holds uncommitted changes, is left as it is, the worktree starts from the local branch, and a warning says why. The form no longer asks whether to go on from a stale source.

## In the form

`wtm create` without `--yes` on a terminal asks its questions in the form every command on the engine shares, and its recap is the plan `--dry-run` prints. Refreshing the branch list (`r`) reads the repository again but no longer runs `git fetch`: fetch in another terminal first when you need a branch just pushed.

## Gone

- The warning about `--isolation` on a worktree `--if-not-exists` found: the item's `detail.metadata.isolation` is the worktree's own; change it with `wtm env <branch> --isolation …`.
