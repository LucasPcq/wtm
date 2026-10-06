# Watching changes: `wtm events`

Use `wtm events --output json` when you need to **react** to worktrees changing (a sibling agent created one, the user removed one) or to their jobs starting, crashing and stopping, instead of polling `wtm list` or `wtm run ps`. For a one-off answer, `wtm list --output json` is still the right call.

## Running it

- It **never exits on its own**: it streams until interrupted. Run it in the background, or read a bounded number of lines (`wtm events --output json | head -n 2` gives the current state and returns). Never run it in the foreground of a step that must finish.
- `--repo <path>` watches another repository than the current directory's. `--all` follows **every** repository wtm was used in, whatever the current directory or an inherited `GIT_DIR`: one `snapshot` per repository, then one `ready`; `repo.added` (followed by that repository's `snapshot`) and `repo.removed` as repositories come and go. To watch everything, always pass `--all` rather than running from outside a repository (which does the same implicitly). `--all` with `--repo` exits `2`.
- It needs no `--yes`: it changes nothing and asks nothing.
- A daemon that is down never makes it exit: it waits and reconnects. Interrupted, or once its reader is gone, it exits `0`.
- These exits are final, do not retry them: `2` bad usage (including a `--repo` that is not a directory, `--all` with `--repo`, and `--all` on a wtm older than 0.29.2), `12` the repository is not initialized with wtm, `20` an event of a newer schema arrived (wtm must be upgraded: ask the user), `21` `--repo` is not in a git repository. Any other non-zero exit is worth retrying with a backoff. The message is on stderr; stdout carries only JSON Lines.

## Before you rely on it

`wtm version --output json` gives `{"version", "events"}`: `events` is the schema version of this stream (the `v` of its events). Exit `2` on `wtm version` (unknown command), or a missing `events` key, means this wtm is too old for the stream: ask the user to upgrade (`wtm upgrade`). Ignore keys you do not know.

## The sequence

One JSON object per line:

1. `snapshot` — `repo` (`root`, `common_dir`) and `worktrees`: every worktree as it is now, each with `jobs`: `[{name, kind, state, url?, exit_code?}]`, its jobs as they are now (`null` when the daemon could not be asked). `state` is `starting`, `running`, `crashed` or `stopped`.
2. `ready` — the snapshot is complete.
3. Then one event per change, from any source (another shell, an agent, the dashboard):

| `type` | Meaning | Extra field |
|---|---|---|
| `worktree.created` | a worktree exists now (sent before its `on_create` hooks run) | — |
| `worktree.provisioned` | its `on_create` hooks ran (also sent when there are none): wait for this one, not `created`, before using a worktree | `ok`; on `false`, `hook` and `exit_code` |
| `worktree.updated` | its identity changed | `changed`: subset of `isolation`, `ordinal`, `parent`, `created_at` |
| `worktree.relocated` | it moved on disk; a worktree adopted by `relocate` first appears this way, not as `created` | `from_path` |
| `worktree.reparented` | its parent branch changed | `from_parent` |
| `worktree.deprovisioned` | its `on_clean` hooks ran (also sent when there are none); `ok: false` means the removal was aborted and the worktree is still there, and no `removed` follows | `ok`; on `false`, `hook` and `exit_code` |
| `worktree.removed` | it is gone; `worktree` is its last state | — |
| `repo.added` | (global stream only) a repository wtm now follows; its `snapshot` comes next | — |
| `repo.removed` | (global stream only) a repository deleted or de-initialized; drop its worktrees | — |
| `job.started` | a job's process is up (a detached launcher: it exited 0) | — |
| `job.crashed` | a job ended on its own: a service that exited, a task or a launcher that failed | `exit_code` (`-1`: killed by a signal), `last_lines`: the last lines it printed |
| `job.exited` | a task finished with `0` | `exit_code` |
| `job.stopped` | `run stop`, `run down` or a stop command took it down | — |

Every `worktree.*` event carries `worktree`: `branch`, `path`, `parent`, `ordinal` (`null` until allocated), `isolation`, `is_main`, `created_at`. Nothing volatile (dirty, ahead, PR, services): read those from `wtm list --output json`.

Every `job.*` event carries `worktree` with only `branch` and `path` (it names the worktree, never upsert an identity from it) and `job`: `name`, `kind` (`service` or `task`), `url` when it publishes one. A shared service's events and snapshot entry belong to the worktree it runs in (the main checkout). A detached stack (`docker compose up -d`) gets `started` and `stopped`, but its crash after the launcher exited is not seen: check `wtm run ps --output json` when it matters.

## Catching a job that crashes after `run up -d`

`run up -d` checks the ports once and returns; a crash after that only shows on the stream. Subscribe first, start on `ready`, and watch for your own job events:

```sh
wtm events --output json | jq --unbuffered -c 'select(.correlation_id == "agent-1" and (.type | startswith("job.")))'
# once ready has been read, from another step:
WTM_CORRELATION_ID=agent-1 wtm run up feat/login -d --output json
```

A `job.crashed` names the job, its `exit_code` and its `last_lines`; read the whole log with `wtm run logs feat/login --job <name> --output json`.

## Recognising your own command

Start a command with `WTM_CORRELATION_ID=<any id>` (≤ 256 bytes, no control character, else exit `2`) and every event it publishes carries `correlation_id` with that value, the children of a `clean` included. The jobs a `run up` / `run start` started carry it on every later `job.*` event, a crash an hour later included; a `job.stopped` carries the id of the `run stop` / `run down` that stopped it. Wait for the event carrying your id rather than the first one of its type: another agent may be creating worktrees at the same time. Events without an id have no `correlation_id` field.

## Rules for reading it

- Key a worktree by `(repo.common_dir, branch)`; apply each `worktree.*` event as an upsert; a `snapshot` **replaces** your state for that repo, jobs included. Key a job by `(repo.common_dir, worktree.path, job.name)`.
- A new `snapshot` + `ready` can arrive at any time: the background daemon restarted and the stream reconnected. Missed events are not replayed; the snapshot holds their result.
- Skip unknown fields and unknown types; they are not errors.
- Events are only published while the daemon runs; with nobody listening, nothing is lost that a snapshot does not restore.
