# The event stream: `wtm events`

`wtm events` tells whatever reads it what wtm does to a repository's worktrees and their jobs, as it happens, whoever does it: a command in another shell, an agent, or the `wtm ui` dashboard (itself one of its readers). A terminal plugin opens a pane for a new worktree, an editor closes the window of a removed one, an agent waits for a sibling to finish provisioning or learns that the server it started an hour ago crashed, all without polling `wtm list` or `wtm run ps`.

```sh
wtm events                     # one line per change, for a person watching
wtm events --output json       # JSON Lines: the contract an integration reads
wtm events --repo ~/code/app   # another repository than the current one
wtm events --all               # every repository wtm knows
```

Report every hook that failed, in any worktree, as it happens:

```console
$ wtm events --output json | jq -c 'select(.type == "worktree.provisioned" and .ok == false) | {branch: .worktree.branch, hook, exit_code}'
{"branch":"feat/login","hook":"pnpm install","exit_code":1}
```

## What the stream carries

A subscription opens on a **snapshot** (one `snapshot` event listing every worktree and its jobs), then a single `ready`, then one event per change until you interrupt it or its reader goes away. `wtm events --output json | head -n 2` prints the current state and returns.

```jsonc
{"v":1,"type":"snapshot","ts":"2026-10-03T09:12:01.512Z","repo":{"root":"/code/app","common_dir":"/code/app/.git"},"worktrees":[{"branch":"main","path":"/code/app","parent":"","ordinal":0,"isolation":"isolated","is_main":true,"created_at":"","jobs":[{"name":"db","kind":"service","state":"running"}]}]}
{"v":1,"type":"ready","ts":"2026-10-03T09:12:01.513Z"}
{"v":1,"type":"worktree.created","ts":"…","repo":{…},"worktree":{"branch":"feat/login","path":"/code/.trees/feat-login","parent":"main","ordinal":null,"isolation":"isolated","is_main":false,"created_at":"2026-10-03T09:13:40Z"}}
{"v":1,"type":"worktree.provisioned","ts":"…","repo":{…},"worktree":{…},"ok":false,"hook":"pnpm install","exit_code":1}
{"v":1,"type":"worktree.updated","ts":"…","repo":{…},"worktree":{…,"ordinal":1},"changed":["ordinal"]}
{"v":1,"type":"worktree.relocated","ts":"…","repo":{…},"worktree":{…},"from_path":"/code/old/feat-login"}
{"v":1,"type":"worktree.reparented","ts":"…","repo":{…},"worktree":{…,"parent":"main"},"from_parent":"feat/auth"}
{"v":1,"type":"worktree.deprovisioned","ts":"…","repo":{…},"worktree":{…},"ok":true}
{"v":1,"type":"worktree.removed","ts":"…","repo":{…},"worktree":{…}}
{"v":1,"type":"job.started","ts":"…","repo":{…},"worktree":{"branch":"feat/login","path":"/code/.trees/feat-login"},"job":{"name":"web","kind":"service","url":"http://web.feat-login.app.localhost"}}
{"v":1,"type":"job.crashed","ts":"…","repo":{…},"worktree":{…},"job":{…},"exit_code":1,"last_lines":["Error: listen EADDRINUSE :::3000"]}
{"v":1,"type":"job.exited","ts":"…","repo":{…},"worktree":{…},"job":{"name":"migrate","kind":"task"},"exit_code":0}
{"v":1,"type":"job.stopped","ts":"…","repo":{…},"worktree":{…},"job":{…}}
```

| Type | Sent when | Extra field |
| --- | --- | --- |
| `snapshot` | the stream opens, and again after every reconnection | `worktrees`: every worktree as it is now, each with its `jobs` (see [Jobs](#jobs)) |
| `ready` | right after the snapshots, each time they are sent | — |
| `worktree.created` | `create`, `checkout` or `extract` brought a worktree into existence, before its `on_create` hooks run | — |
| `worktree.provisioned` | its `on_create` hooks have run; also sent when there are none, so it always follows a `created` | `ok`; when `false`, `hook` (the command that failed) and `exit_code`. `hook` is absent when the phase failed before any hook ran, `exit_code` when the hook was killed by a signal |
| `worktree.updated` | a field of its identity changed: isolation (`wtm env --isolation`), ordinal (the first time something needs its ports), parent and creation date (adopted by `wtm relocate`) | `changed`: the fields that changed |
| `worktree.relocated` | `wtm relocate` moved it; a worktree created outside wtm and adopted by `relocate` first appears this way, never as `created` | `from_path`: where it was |
| `worktree.reparented` | `wtm reparent`, or a `clean` / `prune` that moved its children past a removed parent | `from_parent`: its previous parent |
| `worktree.deprovisioned` | `clean` or `prune` ran its `on_clean` hooks; also sent when there are none | `ok`, then `hook` and `exit_code` as for `provisioned` |
| `worktree.removed` | `clean` or `prune` removed it, after its `deprovisioned` | — |
| `repo.added` | global stream only: a repository joined the registry (see [Every repository at once](#every-repository-at-once)) | — |
| `repo.removed` | global stream only: a repository left it (deleted, or no longer initialized with wtm) | — |
| `job.started` | a job's process is up: a service's, a task's, or a detached launcher's once it exited `0` | — |
| `job.crashed` | a job ended on its own: a service whose process exited, a task or a detached launcher that exited non-zero | `exit_code` (`-1` when a signal killed it), `last_lines` |
| `job.exited` | a task exited `0` | `exit_code` |
| `job.stopped` | `run stop`, `run down` or the job's `stop` command took it down | — |

- Every event carries `v` (the schema version), `type` and `ts` (RFC 3339, UTC); every event but `ready` carries `repo`, every `worktree.*` event the `worktree` it is about, and every `job.*` event the `worktree` and `job` it is about. A `removed` carries the worktree's last state.
- `deprovisioned` with `ok: true` is followed by `removed`; `ok: false` means the removal stopped there and the worktree is still on disk. A worktree whose directory is already gone gets `ok: true` only without `on_clean` hooks: with some, they cannot run there and the removal stops.
- An event published by a command started with `WTM_CORRELATION_ID` carries it as `correlation_id`; see [Recognising your own command](#recognising-your-own-command).

### The worktree identity

| Field | Meaning |
| --- | --- |
| `branch` | the branch it holds; with `repo.common_dir`, the key of the worktree |
| `path` | where it is on disk |
| `parent` | the branch it was created from (empty for the main checkout) |
| `ordinal` | the number its ports are offset by; `null` until something first needs it, `0` for the main checkout |
| `isolation` | `isolated` or `verbatim`, see [Isolation](isolation.md) |
| `is_main` | whether it is the main checkout |
| `created_at` | when wtm created or adopted it, RFC 3339; empty when it never did |

Nothing volatile is in it (dirty, ahead, behind, pull request): those change without any wtm command running, so read them from `wtm list --output json`. Jobs are not part of the identity either: they come as their own events, and in the snapshot.

## Jobs

The background daemon that runs `wtm run` jobs sees each one start and end, and publishes it. A `job.*` event carries:

| Field | Meaning |
| --- | --- |
| `worktree` | `branch` and `path` only: it names the worktree the job runs in, it does not describe it. Never upsert a worktree's identity from it |
| `job` | `name`, `kind` (`service` or `task`), and `url` when the job is published under a name or a port |
| `exit_code` | on `job.crashed` and `job.exited`: what the process exited with, `-1` when a signal killed it |
| `last_lines` | on `job.crashed`: the last lines it printed (at most 10, terminal escapes removed); `wtm run logs` has the rest |

In the snapshot, each worktree carries `jobs`: one `{name, kind, state, url?, exit_code?}` per job (`exit_code` on a crashed one only) the daemon holds for it, sorted by name. `state` is one of `starting` (a detached launcher still running), `running`, `crashed` and `stopped`; a task that exited is no longer held, so `exited` only appears as an event. `jobs` is `[]` when the worktree has none, and `null` when the daemon could not be asked.

- **One job, one sequence.** A service reads `started`, then `crashed` or `stopped`; a task `started`, then `exited`, `crashed` or `stopped`. A job killed by a stop is `stopped`, never `crashed`.
- **A crash after `run up -d` is on the stream.** `run up` checks the ports once and returns; whatever happens next reaches only a reader of `job.crashed`.
- **Correlation.** The jobs a `run up` or `run start` started with `WTM_CORRELATION_ID` set carry it on every later `job.*` event, a crash an hour later included. A `job.stopped` carries the id of the `run stop` or `run down` that stopped it, or none.
- **Shared services** belong to the worktree they run in, the main checkout: their events and their snapshot entry are there. A linked worktree that only holds one gets neither.

### What is not seen

- **A detached stack crashing.** A job with a `stop` command (`docker compose up -d`) is `started` once its launcher exits `0`, and `stopped` by `run stop` or `run down`, but the containers it left running are Docker's: no daemon watches them, so a container that dies later sends nothing. `wtm run ps` and the snapshot report what the daemon last knew. Watching them is planned (LUC-270).
- **The daemon is a job's only witness.** It stays up for as long as it runs a job of its own, and while a `wtm events` reader is connected, so neither goes unwatched. A daemon stopped by hand (`wtm run daemon stop`, an upgrade) takes its foreground jobs down with it, and the next one does not report them: read the state from the snapshot of the reconnected stream.
- **Jobs started by an older wtm**, or by a daemon of a version without job events, publish none. `repo.common_dir` is git's common directory with symlinks resolved, the same from any worktree however its path was spelled; `repo.root` is the main checkout.

## Reading it right

- **Treat every `worktree.*` event as an upsert** keyed by `(repo.common_dir, branch)`, every `job.*` event as one keyed by `(repo.common_dir, worktree.path, job.name)`, and every `snapshot` as a reset of that repository's state, jobs included. Applying an event twice changes nothing.
- **A reconnection is not an error.** The stream rides on wtm's background daemon. If the daemon stops (`wtm run daemon stop`, an upgrade), `wtm events` starts it again (it and `wtm ui` keep it running while open) and reopens on a fresh `snapshot` and `ready`. Events in between are not replayed: the snapshot holds their result. A daemon of another wtm version is used as it is, since it relays events without reading them; only one too old to know `wtm events` is replaced, and only while it runs no job.
- **Ignore what you do not know.** An unknown field or type is skipped, never an error: new ones are added without changing `v`, which moves only on a breaking change. The `job.*` types and the snapshot's `jobs` came that way, in wtm 0.29.2, still `v: 1`.
- **Delivery is opportunistic.** A command publishes only if the daemon is running, and never starts it, so nobody pays for the stream unless something listens. A reader that falls far behind is disconnected, and resynchronises from the snapshot it gets on reconnecting.

## Every repository at once

`wtm events --all` follows every repository wtm was used in: one `snapshot` per repository, a single `ready`, then the changes of all of them, each event's `repo.common_dir` saying which.

- `--all` does so whatever the current directory, and ignores an inherited `GIT_DIR` or `GIT_WORK_TREE`, which would otherwise make every repository read as the one they name. An integration should always pass it rather than rely on where it runs. `--all` with `--repo` is refused with exit `2`.
- Run outside any git repository without `--repo`, `wtm events` does the same, implicitly. Inside a repository, or with `GIT_DIR` set, that same command follows only that repository.
- `--all` exists from wtm 0.29.2: an older wtm refuses it as an unknown flag (exit `2`), and `wtm version --output json` gives the `version` to compare.

- The repositories come from a registry beside the global config (`repos.json`, see [Where wtm keeps its state](state.md)). A repository joins when `wtm init` runs there and the first time any wtm command runs in it, so older ones join on their own.
- A repository that joins arrives as `repo.added`, followed right away by its `snapshot`. One that leaves (deleted, or no longer initialized) arrives as `repo.removed`: drop every worktree you hold for that `repo.common_dir`. A running global stream notices within 30 seconds; otherwise the registry drops it the next time it is written or a global stream starts.
- A stream that starts drops the repositories gone since the last one, and may send their `repo.removed` after its `ready`, for a repository it never sent a snapshot of: deleting what you do not hold is a no-op.
- A repository whose snapshot cannot be read (its main checkout was moved, say) is skipped with a warning on stderr rather than ending the stream.

## Recognising your own command

An integration that runs a wtm command and wants the events *that* command produced, not those of an agent in the next pane, sets `WTM_CORRELATION_ID`:

```sh
WTM_CORRELATION_ID=popup-42 wtm create feat/login --yes
```

- Every event the command publishes carries `"correlation_id":"popup-42"`, including those it publishes on the way (a `clean` that reparents children). A `snapshot`, or an event published without one, has no `correlation_id` field at all.
- wtm never reads the value: any string up to 256 bytes without a control character. Anything else is refused with exit `2` before the command does anything.
- Hooks inherit the variable, so a `wtm` command run from a hook is correlated too; a command run from `wtm ui` never is.

### End to end: create a worktree and open it once it is ready

Subscribe first, start the command on `ready`, and wait for *its* `worktree.provisioned`:

```sh
#!/bin/sh
id="open-$$"
wtm events --output json |
  jq --unbuffered -c --arg id "$id" 'select(.type == "ready" or .correlation_id == $id)' |
  while IFS= read -r ev; do
    case $(printf '%s' "$ev" | jq -r .type) in
      ready)
        [ -n "$started" ] && continue            # a reconnection sends ready again
        started=1
        WTM_CORRELATION_ID=$id wtm create feat/login --yes --quiet & ;;
      worktree.provisioned)
        if [ "$(printf '%s' "$ev" | jq -r .ok)" = true ]; then
          code "$(printf '%s' "$ev" | jq -r .worktree.path)"
        else
          printf 'on_create failed: %s\n' "$(printf '%s' "$ev" | jq -r .hook)" >&2
        fi
        break ;;
    esac
  done
```

Any language reads it the same way: start the process, read stdout line by line, decode each line as JSON. There is no socket to open and no client library to install.

### When it exits

`wtm events` exits `0` when interrupted or when its reader goes away. A daemon that is down or restarting never makes it exit: it waits and reconnects. Its error message goes to stderr, never stdout.

| Code | Means | Retry? |
| --- | --- | --- |
| `2` | bad usage: an unknown flag, an `--output` it does not know, a `--repo` that is not a directory | no: fix the invocation |
| `12` | the repository was never initialized with wtm (`wtm init`) | no |
| `20` | it received an event of a schema newer than its own | no: upgrade wtm |
| `21` | `--repo` is not in a git repository | no |
| anything else | an unexpected failure | yes, with a backoff |

The schema of every line ships with wtm: [`internal/schemas/events.v1.json`](../../internal/schemas/events.v1.json).

## Checking compatibility

An integration runs on whatever wtm its user installed, which may predate the stream. Ask first:

```console
$ wtm version --output json
{
  "version": "0.29.0",
  "events": 1
}
```

`version` is the binary's version (`dev` for a local build); `events` is the schema version of this stream, the `v` its events carry. Read two cases as "upgrade wtm", not as a failure: `wtm version` exiting `2` (an unknown command: a wtm older than the stream), and an `events` key missing or lower than the version you read. Other keys will be added as other contracts are versioned; ignore the ones you do not know.

## What it does not carry yet

`sync` rebasing a chain, the output of hooks (their outcome is: `provisioned` and `deprovisioned`) and whether a job is ready to answer are not on the stream; read them from the command's own output and `wtm run ps --output json`. See [Integrations](integrations.md) for the rest of what a tool can build on.
