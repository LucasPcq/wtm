# Jobs, profiles and runners

## Jobs

A **job** is the unit wtm runs, declared as a `[[job]]` in `run.toml`. Its `kind` is one of two:

- a **service** is long-running — a dev server, a docker stack. Without a `stop` command wtm tracks its process and stops it with SIGTERM. With one, its `cmd` is a **launcher** (`docker compose up -d`): wtm waits for it to exit, then considers the real work owned by something else — Docker — and runs `stop` to bring it down.
- a **task** is one-shot — a migration, a seed. It runs to the end, its output streams live, and a non-zero exit aborts the profile it belongs to.

`cmd` and `stop` are `/bin/sh` lines: quotes, `&&`, pipes and `${VAR}` behave as in a terminal. A job runs in the worktree it was started for (plus its `cwd`), so the same job runs once per worktree — unless it is a [shared service](shared-services.md).

Declare jobs with `wtm run init` (detected from compose files and package scripts) or `wtm run job add`; change them with `wtm run job edit`, which also accepts every field as a flag.

## Profiles

A **profile** is a named, ordered group of jobs: `[[profile]] name = "shop", jobs = ["db", "migrate", "shop-api"]`. Order matters — a task placed before a service runs to the end before the service starts.

`wtm run up` starts **exactly one profile**: `--profile`, else the profile marked `default = true`, else the only one declared. With several profiles and no default, an interactive run asks which (the cursor on the default); `--yes` and runs without a terminal fail naming `--profile`. A `run.toml` declaring no profile at all starts every job.

`wtm run start --job <name>` starts a single job outside any profile. `wtm run down` stops what a worktree runs (`--profile` narrows it to one profile, `--all` covers every worktree of the repository); `wtm run stop --job <name>` stops one job.

## Runners

In a monorepo, one root script often starts several apps at once — `turbo run dev`, `pnpm -r dev`, a compose file with several services. Declare that relation on the runner: `runs = ["shop-web", "shop-api"]`. wtm never infers it from the command.

It is what lets the runner carry its children's ports and named URLs, and what keeps wtm from starting an app twice — once by the runner, once on its own. While the runner is up, its children have no row of their own: their addresses are reported under the runner.

## The run view, or `-d`

`run up`, `run start` on a service and `run logs` open the **run view**: a full-screen view with one pane per job. Leaving it (`q`, or Ctrl+C outside focus mode) **detaches** — the jobs keep running in the background daemon — and `wtm run logs` reopens it later.

`-d` starts the jobs and gives the prompt back instead. No view ever opens unless both stdin and stdout are a terminal, nor under `--output json`: the run then reports itself as lines, which is what a script or an agent gets. A task always runs inline, with or without `-d`.

Each job's output is also written to `<git-common-dir>/wtm/logs/<branch>/<job>.log`, cleared when the job starts, so `run logs` can replay a job that is no longer running.

## Checking the ports

Declaring a port only injects a variable. Once the jobs are up, `run up` and `run start` dial each declared port and report the ones nothing answers on — the sign of a command that never read its variable. The check never fails the run and stops as soon as every port answers.

- `--no-probe` skips it for one run; `probe = false` on a job skips it for that job (an interactive run offers to write it for a warning that comes back every time).
- `port_probe_timeout` at the top of `run.toml` sets the budget in seconds (15 by default; a negative value turns the check off).
- `binds_no_port = true` says a service listens on nothing by design — a watcher, a worker, a runner whose children hold the ports — so wtm stops offering it a port.

## Several worktrees at once

The first time `run up` or `run start` finds jobs running in another worktree, it asks what to do about the machine's load, and can remember the answer as `concurrency = "parallel" | "exclusive"` in `run.toml`. `--parallel` and `--exclusive` answer for one run. Worktrees that share their ports — a [verbatim](isolation.md) worktree and its source — cannot run together whatever the setting: wtm offers to stop the other one.

## `run ps` statuses

`wtm run ps` lists what the daemon holds across every repository, from anywhere. A job's `status` is one of:

| Status | Meaning |
| --- | --- |
| `running` | a process wtm started and still watches |
| `detached` | a service whose launcher exited, leaving the work to something wtm does not own (a compose stack); nothing about it is verified, and it survives the daemon |
| `joined` | this worktree's hold on a [shared service](shared-services.md) running in the main checkout; it owns no process |
| `stopped` | stopped on request |
| `crashed` | a service that exited without being asked to (`exit_code` in the JSON says how) |
| `reaped` | a service that outlived the daemon which owned it, taken down by the next one |

The results of `run up`, `run down` and `run stop` use their own vocabulary: `started`, `joined`, `done` (a task that ran to the end), `stopped`, `released` (a shared service this worktree let go of, still up for others), `not_running` (nothing was up under that name) and `error`.
