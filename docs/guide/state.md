# Where wtm keeps its state

## Per repository: `<git-common-dir>/wtm/`

Everything wtm knows about a repository lives under its git common directory (`.git/wtm/` in a normal clone). Git never commits anything inside `.git/`, so none of it reaches your teammates or `git status`.

```
<git-common-dir>/wtm/
├── config.toml               # project settings (wtm init)
├── run.toml                  # jobs and profiles (wtm run init), optional
├── schemas/                  # JSON schemas, rewritten beside each file on every write
├── worktrees/<branch>/
│   └── meta.json             # one per worktree wtm created or adopted
├── logs/<branch>/<job>.log   # each job's output, cleared when the job starts
├── hooks/<phase>-<branch>.log  # the raw output of the last on_create / on_clean run
├── pending-removals.toml     # namespace drops owed by a clean while their service was down
└── ordinal.lock              # serialises the allocation of worktree numbers
```

`<branch>` is the branch name URL-escaped into one path segment (`feat/x` → `feat%2Fx`).

### `meta.json`

| Field | Meaning |
| --- | --- |
| `source_branch` | the parent `wtm sync` rebases onto |
| `created_at` | creation time |
| `env_strategy` | how its `.env` files were provisioned: `example`, `main` or `parent` |
| `ordinal` | its stable number, from which its ports are derived (`base + ordinal × port_offset_block`). Allocated on first need and released when the worktree is cleaned; the main checkout is `0` and has no `meta.json` |
| `isolation` | `isolated` or `verbatim`. Absent on a worktree created before v0.28, whose [adoption](isolation.md#worktrees-created-before-v028) is pending |
| `namespaces` | the shared services it created a namespace in, which `clean` and `prune` drop |

### `pending-removals.toml`

A `clean` or `prune` that could not drop a namespace (its shared service was down, the drop timed out, or was kept under `--yes`) records the debt here. It is paid the next time wtm starts that service or runs `prune`, and withdrawn if a worktree of the same name is created again first. See [Shared services](shared-services.md#what-clean-and-prune-do-with-the-data).

## Per machine: beside the global config

The global config lives in the OS config directory (`~/.config/wtm/` on Linux, `~/Library/Application Support/wtm/` on macOS), and the run daemon keeps its files next to it:

```
<config dir>/wtm/
├── config.toml   # your personal settings: shell, [ui], [proxy]
├── state.json    # what wtm writes for itself (the update check)
├── wtm.sock      # the run daemon's socket, shared by every repository
├── wtm.lock      # held by the one daemon running
├── jobs.json     # the daemon's index of what it started
├── repos.json    # every repository wtm was used in, for `wtm events` run outside one
└── repos.json.lock
```

`jobs.json` is what makes the daemon disposable: it exits about 30 s after its last foreground job, detached services keep running without it, and the next daemon reads the index back, so `wtm run ps` still lists a compose stack after a reboot and `wtm run down` still stops it. `wtm run daemon status` reports what is up; `wtm run daemon restart` replaces a daemon of another wtm build.

On macOS, `wtm run proxy install` adds one file of its own: a LaunchAgent under `~/Library/LaunchAgents`, removed by `wtm run proxy uninstall`.
