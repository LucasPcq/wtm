# Jobs, profiles and runners

```bash
wtm run job add db  --cmd 'docker compose up -d' --stop 'docker compose down' --port DB_PORT=5432 --yes
wtm run job add migrate --kind task --cmd 'pnpm db:migrate' --yes
wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000 --yes
wtm run profile add dev --jobs db,migrate,web --default --yes

wtm run up -d          # starts the default profile in the current worktree
wtm run ps             # what runs, everywhere
wtm run down           # stops this worktree's jobs
```

## Jobs

A **job** is the unit wtm runs, declared as a `[[job]]` in `run.toml`. Its `kind` is one of two:

- a **service** is long-running: a dev server, a docker stack. Without a `stop` command wtm tracks its process and stops it with SIGTERM. With one, its `cmd` is a **launcher** (`docker compose up -d`): wtm waits for it to exit, considers the real work owned by something else (Docker), and runs `stop` to bring it down.
- a **task** is one-shot: a migration, a seed. It runs to the end, its output streams live, and a non-zero exit aborts the profile it belongs to.

`cmd` and `stop` are `/bin/sh` lines: quotes, `&&`, pipes and `${VAR}` behave as in a terminal. A job runs in the worktree it was started for (plus its `cwd`), so the same job runs once per worktree, unless it is a [shared service](shared-services.md).

Declare jobs with `wtm run init` (detected from compose files and package scripts) or `wtm run job add`; `wtm run job edit` changes any field from a flag.

## Profiles

A **profile** is a named, ordered group of jobs:

```toml
[[profile]]
name    = "shop"
jobs    = ["db", "migrate", "shop-api"]   # migrate runs to the end before shop-api starts
default = true
```

`wtm run up` starts **exactly one profile**: `--profile`, else the one marked `default = true`, else the only one declared. With several and no default, an interactive run asks (the cursor on the default); `--yes` and runs without a terminal fail naming `--profile`. A `run.toml` declaring no profile starts every job.

| Command | Does |
| --- | --- |
| `wtm run up [worktree...] --profile <p>` | start a profile, in one or several worktrees |
| `wtm run start [worktree] --job <j>` | start one job, outside any profile |
| `wtm run down [worktree...] [--profile <p>]` | stop what a worktree runs, or one profile of it; `--all` covers every worktree of the repository |
| `wtm run stop [worktree...] --job <j>` | stop one job |

## Runners

In a monorepo, one root script often starts several apps: `turbo run dev`, `pnpm -r dev`, a compose file with several services. Declare that relation on the runner, since wtm never infers it from the command:

```toml
[[job]]
name = "dev"
kind = "service"
cmd  = "pnpm turbo run dev"
runs = ["shop-web", "shop-api"]
```

The runner carries its children's ports and named URLs, and wtm will not start an app twice, once by the runner and once on its own. While the runner is up, its children have no row of their own: their addresses are reported under it. The [pnpm/turbo recipe](recipes.md#a-pnpm-or-turbo-monorepo) is a complete setup.

## The run view, or `-d`

`run up`, `run start` on a service and `run logs` open the **run view**: a full-screen view with one pane per job. Leaving it (`q`) **detaches** (the jobs keep running in the background daemon), and `wtm run logs` reopens it. Ctrl+C outside focus mode detaches too once every job has started; while `run up` or `run start` is still starting them, it cancels the start instead: the view shows "Cancelling…" until the start has stopped, then exits with code 19, and a second Ctrl+C quits at once.

- `-d` starts the jobs and gives the prompt back instead. A task always runs inline, with or without `-d`.
- No view opens unless both stdin and stdout are a terminal (`wtm run up > run.log` included), nor under `--output json`: the run reports itself as lines, which is what a script or an agent gets.
- Each job's output is also written to `<git-common-dir>/wtm/logs/<branch>/<job>.log` (5 MB × 3 within one run), cleared when the job starts, so `run logs` replays a job that is no longer running: `wtm run logs --job api`, or `--output json` for the last 1000 lines of each job.

## Checking the ports

Declaring a port only injects a variable. Once the jobs are up, `run up` and `run start` dial each declared port and report the ones nothing answers on, the sign of a command that never read its variable (an example is in [The port check](how-run-works.md#the-port-check)). The check never fails the run and stops as soon as every port answers.

| Switch | Where | Effect |
| --- | --- | --- |
| `--no-probe` | `run up`, `run start` | skips the check for one run |
| `probe = false` | a `[[job]]` | skips it for that job; an interactive run offers to write it for a warning that comes back every time |
| `port_probe_timeout` | top of `run.toml` | the budget in seconds (15 by default; a negative value turns the check off) |
| `binds_no_port = true` | a service | listens on nothing by design (a watcher, a worker, a runner whose children hold the ports), so wtm stops offering it a port |

## Several worktrees at once

```bash
wtm run up feat/a feat/b -d     # two stacks side by side, each on its own ports
wtm run up feat/c --exclusive   # stop the other worktrees' jobs first
```

The first time `run up` or `run start` finds jobs running in another worktree, it asks what to do about the machine's load, and can remember the answer as `concurrency = "parallel" | "exclusive"` at the top of `run.toml`. `--parallel` and `--exclusive` answer for one run; `--exclusive` is refused on several worktrees, since it stops all but one, and `--yes` alone keeps the others running.

Worktrees that share their ports (a [verbatim](isolation.md) worktree and its source) cannot run together whatever the setting: `run up` and `run start` offer to stop the other one or not to start, and refuse under `--yes` unless `--exclusive` was given.

## `run ps` statuses

`wtm run ps` lists what the daemon holds across every repository, from anywhere. A runner binds no port, so its ADDRESS is empty: the apps it started are listed under its row with their addresses (`held` in the JSON). A job's `status` is one of:

| Status | Meaning |
| --- | --- |
| `running` | a process wtm started and still watches |
| `detached` | a service whose launcher exited, leaving the work to something wtm does not own (a compose stack); nothing about it is verified, and it survives the daemon |
| `joined` | this worktree's hold on a [shared service](shared-services.md) running in the main checkout; it owns no process |
| `stopped` | stopped on request |
| `crashed` | a service that exited without being asked to (`exit_code` in the JSON says how) |
| `reaped` | a service that outlived the daemon which owned it, taken down by the next one |

The results of `run up`, `run down` and `run stop` use their own vocabulary: `started`, `joined`, `done` (a task that ran to the end), `stopped`, `released` (a shared service this worktree let go of, still up for others), `already_running`, `not_running` (nothing was up under that name) and `error`.
