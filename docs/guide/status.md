# A worktree's state: `wtm status`

`wtm status` answers one question about a worktree: is it ready, and if not, what do I run? It reads everything a worktree is made of in one pass — how it runs, which of its `.env` files exist, what its jobs are doing — and names each problem with the command that fixes it.

```
$ wtm status feat/x

  ! feat/x — 2 problems

  path        /code/acme.trees/feat-x
  isolation   isolated
  addressing  names
  ports       +10
  env         2 files · 1 missing

  JOBS
  web  running  http://web.feat-x.acme.localhost:8080
  api  crashed  http://api.feat-x.acme.localhost:8080

  ! apps/web/.env is missing
  → wtm env feat/x --yes

  ! api crashed (exit 1)
  → wtm run start feat/x --job api -d --yes
```

`ports` says which ports the worktree binds: `+10` is its offset from the main checkout's ports, `base` the main checkout's own, `source's` a verbatim worktree sharing its source's. A worktree created before the [isolation](isolation.md) choice reads `not chosen`.

Without an argument, in a terminal, it opens the same worktree picker as the `run` commands, the cursor on the worktree you are in: Enter reads that one, and you can pick any other from wherever you stand (Esc backs out). Without a terminal, with `--output json`, `--quiet` or `--yes`, it reads the worktree you are in and asks nothing. `wtm status --all` reads every worktree of the repository as a table, then names each problem under the worktree it belongs to:

```
$ wtm status --all

  ! 5 worktrees · 2 need attention

     WORKTREE       ISOLATION   PORTS     JOBS                   ENV
     main           isolated    base      2 stopped              2 files
     feat/login     isolated    +10       2 running              2 files
  !  feat/payments  isolated    +20       1 running · 1 crashed  1 missing
  !  fix/legacy     not chosen  —         2 stopped              2 files
     docs/readme    verbatim    source's  2 stopped              2 files

  feat/payments
  ! apps/web/.env is missing
  → wtm env feat/payments --yes
  ! api crashed (exit 1)
  → wtm run start feat/payments --job api -d --yes

  fix/legacy
  ! predates the isolation choice: run up and run start refuse it
  → wtm env fix/legacy --isolation isolated --yes
```

Without a `run.toml`, the table keeps only the `ENV` column.

## What it never does

It changes nothing, and asks nothing but that picker, so it needs no `--yes`, in JSON either. It never starts the run daemon: when none is running, it reads the job index the daemon keeps and checks each process itself, so a job that died with the daemon reads `crashed` rather than `running`. It never numbers a worktree no run has numbered yet (its offset and addresses are then left out). And it never prints a value from a `.env` file: it only checks which files exist.

It exits `0` whatever it finds. A script reads `problems` in the JSON document rather than the exit code.

## Jobs

Every job `run.toml` declares is listed, in its order, with the state [`wtm events`](events.md#jobs) reports: `starting`, `running`, `crashed`, `exited` (a task that finished) or `stopped`. A job nothing started yet is `stopped`. A job still running that `run.toml` no longer declares comes last. The address is the one `wtm run url` gives.

## Problems

| Code | Means | The fix it prints |
|---|---|---|
| `env_missing` | a `.env` file `config.toml` declares is absent | `wtm env <b> --yes`, which rebuilds it the way `wtm create` provisions it and settles its ports; `--from example` when the strategy's source has no copy but a template exists; `wtm config edit` when nothing exists to rebuild it from |
| `isolation_pending` | the worktree was created before the [isolation](isolation.md) choice, and `run up` refuses it | `wtm env <b> --isolation isolated --yes` (or `--isolation verbatim` to keep its source's ports) |
| `job_crashed` | a job ended on its own | `wtm run start <b> --job <name> -d --yes`, after reading `wtm run logs` |

The codes are stable: an agent's loop is `wtm status --output json`, run each `fix`, `wtm run up -d --yes`, and `status` again.

## Waiting for a job

There is no `wait` command. Poll `wtm status --output json` until the jobs you need are `running`, or subscribe to [`wtm events`](events.md#jobs) before starting them and read until a `job.started`, `job.crashed` or `job.exited` for yours. `running` means the process is up, not that it answers yet: request its `url` when that matters.
