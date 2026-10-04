# Watching changes: `wtm events`

Use `wtm events --output json` when you need to **react** to worktrees changing — a sibling agent created one, the user removed one — instead of polling `wtm list`. For a one-off answer, `wtm list --output json` is still the right call.

## Running it

- It **never exits on its own**: it streams until interrupted. Run it in the background, or read a bounded number of lines (`wtm events --output json | head -n 2` gives the current state and returns). Never run it in the foreground of a step that must finish.
- `--repo <path>` watches another repository than the current directory's.
- It needs no `--yes`: it changes nothing and asks nothing.
- A daemon that is down never makes it exit: it waits and reconnects. Interrupted, or once its reader is gone, it exits `0`.
- These exits are final, do not retry them: `2` bad usage (including a `--repo` that is not a directory), `12` the repository is not initialized with wtm, `20` an event of a newer schema arrived (wtm must be upgraded: ask the user), `21` not in a git repository. Any other non-zero exit is worth retrying with a backoff. The message is on stderr; stdout carries only JSON Lines.

## Before you rely on it

`wtm version --output json` gives `{"version", "events"}`: `events` is the schema version of this stream (the `v` of its events). Exit `2` on `wtm version` (unknown command), or a missing `events` key, means this wtm is too old for the stream: ask the user to upgrade (`wtm upgrade`). Ignore keys you do not know.

## The sequence

One JSON object per line:

1. `snapshot` — `repo` (`root`, `common_dir`) and `worktrees`: every worktree as it is now.
2. `ready` — the snapshot is complete.
3. Then one event per change, from any source (another shell, an agent, the dashboard):

| `type` | Meaning | Extra field |
|---|---|---|
| `worktree.created` | a worktree exists now (sent before its `on_create` hooks run) | — |
| `worktree.updated` | its identity changed | `changed`: subset of `isolation`, `ordinal`, `parent`, `created_at` |
| `worktree.relocated` | it moved on disk; a worktree adopted by `relocate` first appears this way, not as `created` | `from_path` |
| `worktree.reparented` | its parent branch changed | `from_parent` |
| `worktree.removed` | it is gone; `worktree` is its last state | — |

Every `worktree.*` event carries `worktree`: `branch`, `path`, `parent`, `ordinal` (`null` until allocated), `isolation`, `is_main`, `created_at`. Nothing volatile (dirty, ahead, PR, services): read those from `wtm list --output json`.

## Rules for reading it

- Key a worktree by `(repo.common_dir, branch)`; apply each event as an upsert; a `snapshot` **replaces** your state for that repo.
- A new `snapshot` + `ready` can arrive at any time: the background daemon restarted and the stream reconnected. Missed events are not replayed; the snapshot holds their result.
- Skip unknown fields and unknown types; they are not errors.
- Events are only published while the daemon runs; with nobody listening, nothing is lost that a snapshot does not restore.
