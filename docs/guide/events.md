# The event stream: `wtm events`

`wtm events` tells whatever reads it what wtm did to a repository's worktrees, as it happens: a terminal plugin opening a pane for a new worktree, an editor closing the window of a removed one, an agent waiting for a sibling to finish provisioning (`worktree.provisioned`). Without it, an integration can only guess, comparing `wtm list --output json` before and after a command it ran itself, and missing everything that happened anywhere else.

The stream reports every change, whoever made it: a command in another shell, an agent driving wtm, or the `wtm ui` dashboard, which is itself one of its readers.

```sh
wtm events                     # one line per change, for a person watching
wtm events --output json       # JSON Lines: the contract an integration reads
wtm events --repo ~/code/app   # another repository than the current one
```

## What the stream carries

A subscription opens on a **snapshot**: one `snapshot` event listing every worktree of the repository, then a single `ready`. After that it carries one event per change, until you interrupt it or its reader goes away: `wtm events --output json | head -n 2` prints the current state and returns as soon as `head` does, however quiet the repository.

| Type | Sent when | Extra field |
| --- | --- | --- |
| `snapshot` | the stream opens, and again after every reconnection | `worktrees`: every worktree as it is now |
| `ready` | right after the snapshots, each time they are sent | — |
| `worktree.created` | `create`, `checkout` or `extract` brought a worktree into existence, before its `on_create` hooks run | — |
| `worktree.provisioned` | the `on_create` hooks of a worktree `create`, `checkout` or `extract` just made have run — also sent when there are none, so it always follows a `created` | `ok`; when `false`, `hook` (the command that failed) and `exit_code` |
| `worktree.updated` | a field of a worktree's identity changed: its isolation (`wtm env --isolation`), its ordinal (the first time something needs its ports), its parent and creation date (adopted by `wtm relocate`) | `changed`: the fields that changed |
| `worktree.relocated` | `wtm relocate` moved it; a worktree created outside wtm and adopted by `relocate` first appears this way, never as `created` | `from_path`: where it was |
| `worktree.reparented` | `wtm reparent`, or a `clean` / `prune` that moved its children past a removed parent | `from_parent`: its previous parent |
| `worktree.removed` | `clean` or `prune` removed it | — |

Every event carries `v` (the schema version), `type` and `ts` (RFC 3339, UTC); every event but `ready` carries `repo`, and every `worktree.*` event the `worktree` it is about. A `removed` carries the last state the worktree had. An event published by a command started with `WTM_CORRELATION_ID` carries it as `correlation_id` — see [Recognising your own command](#recognising-your-own-command).

```jsonc
{"v":1,"type":"snapshot","ts":"2026-10-03T09:12:01.512Z","repo":{"root":"/code/app","common_dir":"/code/app/.git"},"worktrees":[{"branch":"main","path":"/code/app","parent":"","ordinal":0,"isolation":"isolated","is_main":true,"created_at":""}]}
{"v":1,"type":"ready","ts":"2026-10-03T09:12:01.513Z"}
{"v":1,"type":"worktree.created","ts":"…","repo":{…},"worktree":{"branch":"feat/login","path":"/code/.trees/feat-login","parent":"main","ordinal":null,"isolation":"isolated","is_main":false,"created_at":"2026-10-03T09:13:40Z"}}
{"v":1,"type":"worktree.provisioned","ts":"…","repo":{…},"worktree":{…},"ok":false,"hook":"pnpm install","exit_code":1}
{"v":1,"type":"worktree.updated","ts":"…","repo":{…},"worktree":{…,"ordinal":1},"changed":["ordinal"]}
{"v":1,"type":"worktree.relocated","ts":"…","repo":{…},"worktree":{…},"from_path":"/code/old/feat-login"}
{"v":1,"type":"worktree.reparented","ts":"…","repo":{…},"worktree":{…,"parent":"main"},"from_parent":"feat/auth"}
{"v":1,"type":"worktree.removed","ts":"…","repo":{…},"worktree":{…}}
```

### The worktree identity

| Field | Meaning |
| --- | --- |
| `branch` | the branch it holds; with `repo.common_dir`, the key of the worktree |
| `path` | where it is on disk |
| `parent` | the branch it was created from (empty for the main checkout) |
| `ordinal` | the number its ports are offset by; `null` until something first needs it, `0` for the main checkout |
| `isolation` | `isolated` or `verbatim` — see [Isolation](isolation.md) |
| `is_main` | whether it is the main checkout |
| `created_at` | when wtm created or adopted it, RFC 3339; empty when it never did |

Nothing volatile is in it — dirty, ahead, behind, pull request, services. Those change without any wtm command running; read them from `wtm list --output json` when you need them.

The repository is `repo.common_dir`, git's common directory with symlinks resolved: the same from any of its worktrees, however its path was spelled. `repo.root` is the main checkout.

## Reading it right

- **Treat every event as an upsert** keyed by `(repo.common_dir, branch)`, and every `snapshot` as a reset of that repository's state. Applying an event twice changes nothing.
- **A reconnection is not an error.** The stream rides on wtm's background daemon. If the daemon stops (`wtm run daemon stop`, an upgrade), `wtm events` starts it again — `wtm events` and `wtm ui` are what keep it running while they are open — then opens again on a fresh `snapshot` and `ready`. Events in between are not replayed: the new snapshot already holds their result. A daemon of another wtm version is used as it is: it relays events it does not read; only one too old to know `wtm events` is replaced, and only while it runs no job.
- **Ignore what you do not know.** A field or a type you do not recognise is skipped, never an error: new ones are added without changing `v`. `v` moves only on a breaking change. `wtm events` itself exits with code `20` if it receives an event of a schema newer than its own: upgrade wtm.
- **Delivery is opportunistic.** A command publishes its event only if the daemon is running, and never starts it, so nobody pays for the stream unless something listens. A reader that falls far behind is disconnected rather than waited for, and resynchronises from the snapshot it gets on reconnecting.

## Recognising your own command

An integration that runs a wtm command and wants the events *that* command produced — not those of an agent working in the next pane — sets `WTM_CORRELATION_ID` when it starts it:

```sh
WTM_CORRELATION_ID=popup-42 wtm create feat/login --yes
```

Every event the command publishes carries `"correlation_id":"popup-42"`, including the ones it publishes on the way (a `clean` that reparents children). wtm never reads the value: any string up to 256 bytes without a control character. Anything else is refused with exit `2` before the command does anything. An event published without one has no `correlation_id` at all, and a `snapshot` never has one. Hooks inherit the variable, so a `wtm` command run from a hook is correlated too; a command run from `wtm ui` never is.

### When it exits

`wtm events` exits only when it is interrupted (`0`), when its reader goes away (`0`), or when retrying cannot help. A daemon that is down or restarting never makes it exit: it waits and reconnects on its own. Its error message goes to stderr, never to stdout, so a JSON Lines reader never has to parse it.

| Code | Means | Retry? |
| --- | --- | --- |
| `2` | bad usage: an unknown flag, an `--output` it does not know, a `--repo` that is not a directory | no: fix the invocation |
| `12` | the repository was never initialized with wtm (`wtm init`) | no |
| `20` | it received an event of a schema newer than its own | no: upgrade wtm |
| `21` | the current directory, or `--repo`, is not in a git repository | no |
| anything else | an unexpected failure | yes, with a backoff |

The schema of every line ships with wtm: [`internal/schemas/events.v1.json`](../../internal/schemas/events.v1.json).

## Checking compatibility

An integration runs on whatever wtm its user has installed, which may predate the stream. Ask before reading it:

```sh
wtm version --output json
```

```json
{
  "version": "0.30.0",
  "events": 1
}
```

`version` is the binary's version (`dev` for a local build); `events` is the schema version of this stream, the `v` its events carry. Read the two cases that mean "upgrade wtm" as such, not as a failure: `wtm version` exiting `2` (an unknown command: a wtm older than this probe, and older than the stream), and an `events` key that is missing or lower than the version you read. More keys will be added as other contracts are versioned; ignore the ones you do not know.

## A minimal consumer

```sh
wtm events --output json | while IFS= read -r line; do
  type=$(printf '%s' "$line" | jq -r .type)
  case "$type" in
    worktree.created) printf 'open  %s\n' "$(printf '%s' "$line" | jq -r .worktree.path)" ;;
    worktree.removed) printf 'close %s\n' "$(printf '%s' "$line" | jq -r .worktree.path)" ;;
  esac
done
```

Any language reads it the same way: start the process, read stdout line by line, decode each line as JSON. There is no socket to open and no client library to install.

## What it does not carry yet

Jobs starting and exiting, `sync` rebasing a chain, and the output of hooks are not on the stream in this version (their outcome is: `worktree.provisioned`); read them from `wtm run ps --output json` and the command's own output.
