# A worktree's state: `wtm status`

One read-only call answers "is this worktree ready, and if not, what do I run?". Use it instead of combining `list`, `env --check`, `run url` and `run ps`.

```sh
wtm status [worktree] --output json      # one worktree: the current one when omitted
wtm status --all --output json           # every worktree: an array of the same documents
```

It changes nothing, asks nothing and needs no `--yes`. It never starts the run daemon: with none running, it reads the job index and checks the processes itself. It never writes a `.env` value into its output. It **exits `0` whatever it finds**: branch on `problems`, not on the exit code. `--all` with a `[worktree]` is a usage error (exit `2`).

## The document

```json
{
  "branch": "feat/x", "path": "/repo.trees/feat-x", "main": false,
  "isolation": "isolated", "addressing": "names", "offset": 10,
  "run_config": true,
  "env": { "declared": 2, "missing": ["apps/web/.env"] },
  "jobs": [
    { "name": "api", "kind": "service", "state": "running", "url": "http://api.feat-x.acme.localhost:8080" },
    { "name": "web", "kind": "service", "state": "crashed", "exit_code": 1, "url": "http://web.feat-x.acme.localhost:8080" }
  ],
  "problems": [
    { "code": "env_missing", "message": "apps/web/.env is missing", "fix": "wtm env feat/x --yes" },
    { "code": "job_crashed", "message": "web crashed (exit 1)", "fix": "wtm run start feat/x --job web -d --yes" }
  ]
}
```

- `addressing` and `offset` are `null` without a `run.toml` (`run_config: false`), and `offset` is `null` for a worktree no run has numbered yet (its jobs then have no `url` either). Reading a worktree never numbers it.
- `jobs` lists every job `run.toml` declares, in its order, then any job still up that it no longer declares. `state` is the same vocabulary as the `job.*` events: `starting`, `running`, `crashed`, `exited` (a task that finished), `stopped`. A job nothing has started is `stopped`. `exit_code` appears on a crash only. A shared service the worktree holds carries `shared: true` and `owner` (where it runs).
- `env.declared` counts the `.env` files `config.toml` declares; `env.missing` names the ones the worktree lacks. Drift inside an existing file is `wtm env <worktree> --check --output json`.
- `problems` is always present, empty when there is nothing to fix.

## Problems and their fixes

Every problem carries a stable `code` and the exact `fix` to run, unattended and ready to pass to a shell as is.

| `code` | Means | `fix` |
|---|---|---|
| `env_missing` | a declared `.env` file is absent | `wtm env <b> --yes`, which rebuilds it as `create` does (the template under `example`, else the parent's or main's copy) and settles its ports; `wtm env <b> --from example --yes` when the strategy's source has no copy but a template exists; `wtm config edit` when nothing exists to rebuild it from: tell the user, the file has to be declared where it exists or written by hand |
| `isolation_pending` | the worktree predates isolation: `run up` and `run start` refuse it | `wtm env <b> --isolation isolated --yes` (the user's choice: `--isolation verbatim` keeps the source's ports; ask when it matters) |
| `job_crashed` | a job ended on its own (`exit_code` when known) | `wtm run start <b> --job <name> -d --yes`; read why first with `wtm run logs <b> --job <name> --output json` |

The loop: `status` → run each `fix` → `wtm run up <b> -d --yes --output json` → `status` again.

## Waiting for a job

There is no `run wait`. Either poll `wtm status <b> --output json` until every job you need is `running` (or one is `crashed`: stop and read its logs), or subscribe to `wtm events --output json` before starting and read until a decisive `job.*` event for your job — `job.started`, then `job.crashed` or `job.exited` if it does not stay up (see `references/events.md`). `running` says the process is up, not that it answers yet: probe its `url` when that matters.
