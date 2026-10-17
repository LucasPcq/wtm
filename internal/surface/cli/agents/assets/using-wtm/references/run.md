# Running dev jobs: `wtm run`

How to start, stop and inspect the jobs a repository declares in `run.toml`. To declare or change jobs, profiles, ports, URLs or isolation, read `run-config.md`. Output shapes are in `json.md`.

## Contents

- [Before anything](#before-anything)
- [Naming the worktree, the job, the profile](#naming-the-worktree-the-job-the-profile)
- [`--yes` and `--force` in the run module](#--yes-and---force-in-the-run-module)
- [`run up` / `run down`](#run-up--run-down)
- [`run start` / `run stop`](#run-start--run-stop)
- [Refusals before anything starts](#refusals-before-anything-starts)
- [Other worktrees running: concurrency and port clashes](#other-worktrees-running-concurrency-and-port-clashes)
- [Shared services at run time](#shared-services-at-run-time)
- [Job statuses](#job-statuses)
- [After `run up`: the port check and Next origins](#after-run-up-the-port-check-and-next-origins)
- [`run ps`](#run-ps)
- [`run logs`](#run-logs)
- [`run url` / `run open`](#run-url--run-open)
- [Named URLs and the proxy](#named-urls-and-the-proxy)
- [The daemon](#the-daemon)

## Before anything

- The module is **opt-in**. Until `run.toml` declares at least one job or profile, every run command exits `16` except `run init`, `run import`, `run job add` and `run profile add` (which create the first declaration), plus `run ps`, `run daemon …` and `run proxy …`, which never read `run.toml` (see `run-config.md`).
- **Never let a run view open.** `run up` and `run start --job <service>` **attach by default** on a terminal, and `run logs` opens the same full-screen view. Always pass `-d` (or `--output json`, which never attaches): `-d` starts the jobs and returns immediately. A `task` runs inline and blocks until it exits whatever you pass, so `run start --job <task>` needs no `-d`.
- `wtm run list --output json` lists the declared jobs and profiles; `wtm run job list --output json` is the roster of the project's jobs.

## Naming the worktree, the job, the profile

- **Every `run` command takes the worktree as its first argument**: `run up [worktree]`, `run logs [worktree]`, `run start [worktree]`. Omit it and the current directory is used, which is what you want inside one worktree, and it never opens a picker on your paths (no TTY, or `--output json`). Name it to act on another worktree of the same repo without moving. The main checkout is a worktree like any other.
- The job or profile is a **flag**: `--job <name>`, `--profile <name>`. Each takes one value everywhere in the module (a second `--job` or `--profile` is a usage error).
- **`run up`, `run down`, `run stop` and `run logs` take several worktrees** (`run up feat-a feat-b -d`). They run concurrently and independently: one that aborts leaves the others running, and the command exits non-zero if any did. The JSON is one document per worktree whatever their number, in the order you named them, and every human line names its worktree. `run start` stays single-worktree.

## `--yes` and `--force` in the run module

- `--yes` is on every `run` command that can ask something (`run job`, `run profile`, `run list` and `run open` included). The read-only ones that never ask (`run url`, `run ps`, `run export`, `run daemon status`, `run proxy status`) have none. It runs unattended, never opens a picker, and resolves each question to its safe default; where there is none it errors naming the flag (`run start --yes` and `run stop --yes` require `--job`).
- `--force` exists on three commands only: `run up --force` and `run start --force` lift the refusal to start a job whose `touches` reach foreign data (below), and `run job rm --force` (see `run-config.md`). **Pass it only when the user asked.**
- **Backing out of a prompt exits non-zero** on every mutating run command (`up`, `down`, `start`, `stop`, `job add|edit|rm`, `profile add|edit|rm`, `open`): an aborted run did not do what was asked. The listings (`run list`, `run job|profile list`) exit 0 instead.

## `run up` / `run down`

`wtm run up [worktree…] --profile <name> -d --output json` starts **exactly one profile**; `wtm run down [worktree…] --output json` stops it.

- Without `--profile`, `run up` takes the profile marked default, else the only one declared. With several and none marked default it **fails naming `--profile`** and the profiles to choose from (no picker without a terminal): pass `--profile`, or read them from `run list --output json`.
- It starts **every job the profile lists**, tasks included, in the listed order. A `run.toml` declaring no profile starts every declared job in declared order. The step counter (`[2/5]`) covers exactly those jobs, so a smaller count than expected means the profile itself is short, never that wtm dropped something.
- A task blocks its profile, and a non-zero exit aborts it. A failing job aborts the rest and exits non-zero, leaving the services already started up (fix and re-run). The JSON is still written whole: the entry with `status: "error"` carries `message` (the daemon's one-line reason), `output` (everything the job wrote before it failed) and `exit_code`.
- `run up` refuses a profile asking for a runner and one of its own children (the same process twice on the same port).
- `run up` on a `detached` job relaunches its launcher rather than refusing "already running".
- `run down --profile <name>` stops that profile's jobs. `run down --all` stops the jobs of **every worktree of the current repository** (never another repository's, though the daemon is shared), without a prompt; its JSON holds one document per worktree it emptied.
- `run up` / `run start` print what was bound (`web started · PORT=3010`).

## `run start` / `run stop`

`wtm run start [worktree] --job <name> -d --output json` starts one job; `wtm run stop [worktree…] --job <name> --output json` stops one. `--job` is **required** on your paths: without a terminal there is no picker, and the command errors naming the flag.

- `run start` answers with the one job object it started.
- A `run stop` (or `run down --profile`) that found nothing up under that name in that worktree reports `not_running` (human: `= api not running`, exit 0), never `stopped`.

## Refusals before anything starts

- **Stopping never depends on `run.toml`.** `run down`, `run stop` and `run ps` still work when it cannot be read: they warn on stderr and stop (or list) what the daemon runs, and `run stop` takes `--job` as given. `run up` and `run start` refuse an unreadable `run.toml`.
- `run up` and `run start` refuse a worktree whose environment cannot be resolved (a detached HEAD, an unreadable `meta.json`), naming the cause: a job is never started on the main checkout's ports.
- They also refuse a worktree created before the isolation choice (no `isolation` in its `meta.json`) while `run.toml` declares something to isolate: its `.env` still holds its source's ports. The message names both ways out, `wtm env <wt> --isolation isolated` (own ports and compose project) or `--isolation verbatim` (keep the source's). **Ask the user which one**, then run the command again. `run url` / `run open` refuse such a worktree the same way.
- **Foreign data.** `run up` and `run start` refuse to start a job whose `touches` reach data this worktree does not own: its source's for a **verbatim** worktree, everyone's for a shared service with **no** namespace. A task a runner starts through its `runs` counts too (`migrate (run by dev)`). On your paths that is an error (exit 1) naming the jobs, `--force`, and the fix for each cause: `wtm env <wt> --isolation isolated` for a verbatim worktree, a `[job.namespace]` on the service for a shared one (isolating does nothing for it). **Pass `--force` only when the user asked** for the reset to reach that data. The main checkout is never stopped, and a job without `touches` is never checked.
- **An older daemon holding jobs.** `run up` / `run start` replace an older daemon that holds no job without asking; one that holds jobs is refused with `daemon <v> running with N job(s)` and the way out, `wtm run daemon restart`. Run that (see [The daemon](#the-daemon)) rather than retrying the start, which refuses identically.

## Other worktrees running: concurrency and port clashes

Another worktree already running jobs is not a conflict unless it holds your ports. Isolated worktrees sit a block of ports apart, so stacks cohabit; the question is about machine load.

- `run up` and `run start` ask about it once, and only on a terminal. On your paths (no TTY, `--output json`, or `--yes`) it resolves to leaving the others running and stops nothing.
- Force either way for one run with `--exclusive` (stop the others first) or `--parallel`; the two are mutually exclusive. `concurrency = "parallel" | "exclusive"` in `run.toml` settles it for good (it is what the question's "always" answers write).
- **A port clash overrides all of that.** When a job this run would start binds a port a job already up in another worktree binds (a verbatim worktree and its source, always), running side by side is impossible. A terminal is asked "stop <every other worktree> first" or "don't start"; your paths **error** (exit 1) listing each port, unless `--exclusive` (or `concurrency = "exclusive"`) was given, which stops every other worktree's jobs first, the holder's included. `--parallel` and `concurrency = "parallel"` cannot be honoured there. The way out that keeps both running is `wtm env <wt> --yes --isolation isolated`.
- `run up a b` where `a` and `b` share ports is refused before anything is stopped.
- `--exclusive` contradicts several worktrees (it stops all but one): `run up a b --exclusive` errors. Where the project settled on `exclusive` and the run starts several anyway, your paths take the safe default (everything starts, nothing else is stopped) and a notice says the setting was set aside.

## Shared services at run time

A job declared `scope = "shared"` (see `run-config.md`) runs **once for the whole repository**, in the main checkout, instead of once per worktree (a postgres, a keycloak). Expect:

- **No port offset**: its declared port is the port it binds in every worktree.
- Its published URL carries **no worktree segment** (`db.projet.localhost`, not `db.feat-x.projet.localhost`).
- Its logs are the same stream whichever worktree you read them from.
- The worktrees holding it report status **`joined`** with `pid: 0`: a claim on the one running instance, not a second process. Never count one service per worktree from it. Starting it from a worktree other than the main checkout reports `joined`, not `started`; main starting a service another worktree already runs joins it too and carves its own namespace.
- When the start carved out this worktree's namespace, the job's result carries it as `namespace` (`app_feat-x`); it is absent on a start refused as already running, which ran no create.
- `run stop` in a worktree releases only that worktree's claim. The service itself stops when the last hold goes, and **the main checkout's own start counts as one**, so a linked worktree letting go never takes down a service main asked for. A stop that let go without stopping reports **`released`** (the human line adds that it is still up elsewhere), not `stopped`.
- A namespace's `create` runs on **every** start of the shared service.
- Data owed by removed worktrees is dropped the next time wtm starts the service (see `worktrees.md`, `clean` and `prune`).

## Job statuses

A running job (`run ps`, `run up`) has one of six statuses, plus `reaped`. **`detached` is not a weaker `running`.**

- `running`: a foreground service the daemon holds a terminal for.
- `detached`: a service with a `stop` command (a `docker compose up -d`), from the moment its launcher exits. The real work runs outside wtm and there is **nothing to attach to**: `run logs` on it prints its persisted file and returns. While `detached` it is up: it counts as running for `run down`, and `run up` on it relaunches the launcher. For a compose launcher, when a daemon starts it asks `docker compose ps` about each entry, and one whose containers are gone (a `docker compose down` run by hand, a `docker system prune`) is reported `stopped` instead. Any other launcher, a machine without docker, or a failed call leaves it `detached`. The check runs when a daemon adopts the index, not on every listing: a `run ps` served by a daemon already up reports what that daemon holds.
- `crashed`: its process died on its own. `run down` settles it to `stopped`, with no new event.
- `stopped`: stopped by `run stop`, or by whoever took a verified compose stack down.
- `joined`: a worktree's claim on a shared service (`pid` 0 on a claim; on a launcher, whatever became of it).
- `reaped`: **wtm killed something**. A daemon killed without running a handler (`SIGKILL`, a crash, an OOM) leaves its foreground services alive and unreadable. The next daemon finds them from the index, proves the process group is the recorded one (its start time has to match), kills it, and reports the entry `reaped` **once**; the entry then leaves the index. `uptime` on such a row is the orphan's real age. A group whose identity could not be confirmed is never signalled and never reported. Nothing is reaped for a `detached` stack: those belong to Docker.

The result statuses of `up`/`down`/`start`/`stop` (`started`, `already_running`, `joined`, `done`, `stopped`, `released`, `not_running`, `error`) are listed in `json.md`.

## After `run up`: the port check and Next origins

Neither check fails the run or changes the exit code. Do not treat them as errors; report the finding and tell the user what to change. wtm never edits third-party config.

- **Ports declared but not bound.** Declaring a port injects a variable; nothing forces the command to read it. After the jobs start, wtm dials each declared port and reports the silent ones under "Ports declared but not bound". When the *base* port answers instead, the variable never reached the process: a `--port`-only CLI, a hard-coded port, a `.env` that wins, or a task runner filtering env (**Turborepo's default `envMode: "strict"` does exactly this**: a root `turbo run dev` job needs `globalPassThroughEnv` in `turbo.json`). When the base port turns out to be held by another worktree (the main checkout running alongside), the report names that worktree instead of blaming the job's command. `--no-probe` skips the check; `port_probe_timeout` in `run.toml` sets its budget (default 15s, negative disables) and `probe = false` on a job silences it for that job.
- **Next dev origins.** When the proxy serves a job whose directory holds a `next.config.*` without `allowedDevOrigins`, `run up` prints the exact line to add under "Next dev origins". Vite needs nothing: it allows `.localhost` already.
- `run up` / `run start` warn when a job with a `stop` command has a `cmd` without `-d` / `--detach` (such a `cmd` blocks the run; see `run-config.md`).
- **A crash after the check is not reported by `run up`**: it has returned. To catch one, watch `wtm events --output json` for `job.crashed` (see `events.md`), or check `run ps` later.

## `run ps`

`wtm run ps --output json` is **the one global listing**, and the only run command that works from anywhere: it lists what the daemon holds across every repository, so it needs neither a run-initialized repo nor a worktree. Each row carries `branch`, `path`, `project`, `started_at`, `exit_code`, `url`, and a runner's `held` (the apps it started and where they answer: a runner binds no port and has no `url` of its own). It only ever lists. A stopped or crashed job of a worktree that no longer exists on disk is left out; a job still up there is listed. Against a daemon of another version it warns that it should be restarted.

## `run logs`

- Without `--output json`, `run logs [worktree] --job <name>` opens the run view on a terminal. Without a terminal it writes every running job's output as `[job] line` on stdout and only ends when the jobs do: do not call it expecting it to return.
- **`--output json` is your form**: it replays each job's last **1000** lines, one document per worktree, and **never attaches**, so it returns even on a job still running. `--job` narrows it without changing its shape.
- Within a worktree the lines are **grouped by job** and chronological within a job, so `at` goes backwards where one job ends and the next begins. Never read `lines` as one merged timeline.
- It replays the job's **last start only**: starting a job clears its log, so a tail can never reach a previous run, including the log of a crash you just restarted past.
- The file itself is `<git-common-dir>/wtm/logs/<url-escaped-branch>/<url-escaped-job>.log` (rotated 5 MB x 3 *within* a run), if you need more than 1000 lines.
- It covers the lines the worktree's jobs wrote, not everything `run.toml` declares. Every worktree you named has its document, with `lines: []` when nothing was recorded. A job with no line either never started there or printed nothing; the document cannot tell which. It is not a roster: `wtm run job list --output json` is.

## `run url` / `run open`

- `wtm run url [worktree] --job <name>` writes a job's URL on stdout and nothing else, so it composes: `curl "$(wtm run url --job web --raw)/health"`. It never opens a picker on either axis, so it is always safe inside `$(…)`.
- **A job only has a URL if `run.toml` declares one** (`url = { port = "PORT" }`, see `run-config.md`). `run init` declares it for the services it detects; `run job add --url-port` for the rest.
- `--output json` without `--job` lists every published job as `[{job, url}]` and never picks for you; with `--job` it returns **that job alone**. In text mode one published job needs no `--job`; **several and no `--job` is an error naming `--job` and the jobs, never a picker**.
- `wtm run open [worktree] --job <name>` opens the same URL in a browser. It may offer a picker, but only in a fully interactive run, so **always name the job**; on your paths several published jobs and no `--job` errors like `run url`. `run open --output json` writes the address it opened in the same one-element array shape.

## Named URLs and the proxy

Two forms: the **named URL**, served by the proxy, and the **port URL** (`--raw`, `http://localhost:<port>`).

- With the proxy on (the default), a published job answers at `http://<job>.<worktree>.<repo>.localhost:11080`, in that order so a cookie set on `.<worktree>.<repo>.localhost` stays inside that worktree.
- **That URL may carry no port at all**: once `wtm run proxy install` redirects port 80 to the proxy, `run url` prints `http://<job>.<worktree>.<repo>.localhost`. Never assume a `:port` suffix; read the whole line `run url` gives you.
- **Prefer `--raw` for anything you dial yourself** (curl, a health check, a test runner): no proxy has to be up and every OS resolves it. Outside a browser `*.localhost` is not guaranteed to resolve on Linux. Only HTTP jobs get a name; postgres and redis stay on their ports by design.
- Under `addressing = "names"` the named URL is the only entrance a browser can use (the port URL sends an `Origin` the API no longer accepts), so for a browser always read the address from `run url`. Under `"ports"`, every run surface hands out `http://localhost:<port>` and registers no name. See `run-config.md` (Addressing).
- A worktree whose `.env` is out of step with the addressing mode gets one warning line naming `wtm env <worktree>`. Until it runs, a cross-origin call through the name is refused; `run url --raw` gives the port URL that works meanwhile.
- A runner registers one proxy route per published job it runs, so `http://web.<worktree>.<repo>.localhost` answers while the daemon only holds the runner; those addresses are reported on the runner as `held`, never on the children.
- **The proxy runs inside the background daemon and dies with it.** **The URL wtm reports is always one that works**: it comes from what the daemon really serves, not from the config. When the proxy is switched off, or its port is already taken, the jobs still start and every URL falls back to `http://localhost:<port>` (wtm says once why the names are off).
- **`[proxy]` in the global config** tunes it for the whole machine: `port` (default `11080`; `port = 0` only means the default) and `enabled` (default on). The global config sits under the OS config directory (`~/.config/wtm/` on Linux, `~/Library/Application Support/wtm/` on macOS); `wtm run proxy status` prints its resolved path, so never spell it yourself.
- `wtm run proxy status --output json` reports, as one object, what actually serves the names: the proxy's bind port, the public port announced in URLs, and whether the port-80 redirection is installed.
- `wtm run proxy install` needs no privilege (launchd binds port 80 and hands the socket to wtm), but it installs a LaunchAgent in the user's home, so **do not run it on your own initiative**: propose it and let the user decide. Without a terminal it refuses unless you pass `--yes`.

## The daemon

- **The daemon survives nothing, by design.** It exits about 30 s after the last *foreground* job; detached services keep running without it. It records what it started in `jobs.json`, beside the global config, so the next daemon picks those back up: after a reboot `wtm run ps` still lists the detached stacks and `wtm run down` still stops them. `run down`, `clean` and `prune` start a daemon by themselves when that index holds something for the worktree they act on.
- `wtm run daemon status --output json` reports whether a daemon is up, its build, its PID and what it holds, as one object. **`index_frozen: true` means the index belongs to a newer wtm**, so this build records nothing it starts: a detached stack will not be picked back up and an orphaned service is never reaped. It is the one state in which `run ps` and `run down` can be right now and useless after the daemon exits: report it rather than working around it. The field is absent otherwise.
- `wtm run daemon stop` ends it (detached services keep running). `wtm run daemon restart` hands its jobs to a daemon built from the current binary. Both prompt only when foreground services would be stopped; pass `--yes` (required without a terminal, and in JSON).
- **A daemon of another version can always be listed, stopped and replaced**: `run ps`, `run stop`, `run down`, `run daemon status|stop|restart` work against it. A job that will not stop because the daemon is of another build makes `clean`/`prune` refuse the removal; the error says `wtm run daemon restart`.
