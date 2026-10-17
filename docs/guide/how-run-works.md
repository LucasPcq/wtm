# How `wtm run` works

`wtm run` starts your dev stack once per worktree, each with its own ports, Compose project and data. This page explains the model: the jobs, the environment each one receives, what `wtm run init` proposes and the daemon that keeps the jobs running. How ports are declared, checked and written into a `.env` is in [Ports and .env](ports-and-env.md).

`wtm run init` writes `.git/wtm/run.toml` from the compose files and package scripts it detects (a complete file is in the [`run.toml` reference](run-toml.md#example)); `wtm run up` starts it, once per worktree. A job is a command and the ports it binds:

```toml
[[job]]
name = "web"
kind = "service"
cmd  = "pnpm dev"
  [job.ports]
  PORT = 3000               # 3000 on the main checkout, 3010 on the first worktree
```

- The module is opt-in (`wtm run init`, not `wtm init`), per clone and never committed: share a layout with `wtm run export > run.json` in one clone and `wtm run import run.json` in another.
- `cmd` and `stop` are **`/bin/sh` lines**, not argv: quotes, `&&`, pipes, redirections, globs and `${VAR}` behave as in a terminal. POSIX `sh` is used everywhere, never your interactive shell, so a shared `run.toml` behaves the same on every machine.
- Jobs are scoped per worktree: `web` started from worktree A runs with `cwd = A`, and a separate process runs from worktree B. `wtm run up feat/login fix/header` brings up several at once, each independent. `wtm run down` stops the current worktree's jobs; `--all` stops every worktree of this repository, never another one.
- When another worktree's jobs are already running, `run up` asks once whether to keep them running or stop them first: see [Several worktrees at once](jobs-and-profiles.md#several-worktrees-at-once).

## The job environment

Every job gets the worktree's identity, so two worktrees running the same services never share a resource. For `feat/login`, the first worktree of `acme`:

| Variable | Example | Value |
| --- | --- | --- |
| `WTM_BRANCH` | `feat/login` | the branch, verbatim |
| `WTM_WORKTREE` | `feat-login` | the branch as a slug safe for a Docker project, network or volume name |
| `WTM_ORDINAL` | `1` | the worktree's stable number: `0` for the main checkout, else the smallest free number, kept for its whole life and released when it is cleaned |
| `WTM_PORT_OFFSET` | `10` | `WTM_ORDINAL` × `port_offset_block` (10 by default); the main checkout keeps the project's default ports, and so does a verbatim worktree |
| `WTM_ISOLATION` | `isolated` | `isolated` or `verbatim`, as chosen at creation |
| `COMPOSE_PROJECT_NAME` | `acme-feat-login` | `<repo>-<WTM_WORKTREE>`, see below |

`COMPOSE_PROJECT_NAME` keeps two worktrees' containers, networks and volumes apart with nothing to declare:

- It is derived from the worktree, never from your shell (which may belong to another worktree), and qualified by the repository: two clones both on `main` do not share a stack.
- The main checkout is named without its branch (its own `.env`'s value, else `<repo>`), so the shared services it hosts stay one stack whatever it has checked out.
- A verbatim worktree gets none: its copied `.env` decides.
- It reaches what compose names for you, not what your file names itself: a `container_name`, or a volume's or network's explicit `name`, is global to the Docker daemon. See [absolute names](#absolute-names).

`on_create` / `on_clean` hooks get the same variables under the conditions given in [The environment a hook gets](configuration.md#the-environment-a-hook-gets).

## What `wtm run init` proposes

`wtm run init` composes a configuration you can start, not an inventory: it proposes everything it finds and checks the fewest things.

| Step | What it proposes |
| --- | --- |
| Jobs | only scripts whose name contains `dev` are checked, minus a root `dev` a workspace package also declares: that one is an orchestrator (`turbo run dev`, `pnpm -r dev`) and would start each package twice on the same ports. Nothing unchecked is written |
| Runners | which job starts which others. The relation is declared, never inferred from a command; a root can name another root (`dev` → `dev:shop` → the shop apps), two roots may name the same app, only a cycle is refused |
| Named URLs | a [named URL](addressing.md) for every service that declares the port it listens on (`PORT`, or `<JOB>_PORT` after the first). A port a job only dials (`DB_PORT`, `REDIS_PORT`) is never proposed. Unchecking a job withdraws the `url` it had |
| Ports | the values [detection](ports-and-env.md#ports-wtm-run-init-detects) pre-filled from compose files and env files. A service with no port found is reported rather than given an invented one; `run up` will say if it never bound |
| Profiles | one per package plus one gathering everything, to rename, merge or drop. Root jobs (a compose stack) join every profile; tasks are placed ahead of the services that depend on them. A single-package repo, or one past a handful of packages, gets one profile |


### Absolute names

A compose file that pins its own names bypasses the project prefix: Docker refuses a second `container_name`, and a volume or network pinned by `name` is silently shared. `run init` reports them, and `--patch-compose` fronts each with the project:

```diff
   postgres:
-    container_name: acme-postgres
+    container_name: "${COMPOSE_PROJECT_NAME:-acme}-postgres"
```

The `:-acme` default reproduces the old name, so `docker compose up` alone is unchanged. [Ports](ports-and-env.md#compose-files) and names are one question: accepting half still leaves two worktrees unable to run at once.

- Left alone: a `name` under `external: true` (sharing is its point), a name that already reads a variable, and a volume declared as a bare key (compose already prefixes it).
- **A volume that pinned its `name` gains one per worktree, each starting empty**: the data stays under the old name, and moving it is yours to do.

## The daemon

Jobs run under one background daemon per machine; the run view and `-d` are described in [Jobs, profiles and runners](jobs-and-profiles.md#the-run-view-or--d). The daemon is disposable: it exits ~30 s after the last **foreground** job, while detached services (those with a `stop`, typically `docker compose up -d`) keep running without it. Its index of what it started, `jobs.json` beside the [global config](configuration.md#global-config), is read back by the next daemon, so `wtm run ps` still lists your stacks after a reboot (as `detached`: wtm has not seen them since, see [`run ps` statuses](jobs-and-profiles.md#run-ps-statuses)) and `wtm run down` still stops them. `wtm run daemon status` reports what is up; `stop` / `restart` get rid of the process.

A daemon started by an older binary is refused rather than silently used: any `run` command names both versions and points at `wtm run daemon restart`, which hands the jobs over (detached services survive, foreground ones are stopped).
