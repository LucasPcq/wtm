# wtm — Worktree Manager

Orchestrate git worktrees and team dev workflows from the terminal.

`wtm` manages the full lifecycle of git worktrees — creation, environment
provisioning, hooks, stacked-branch syncing, per-worktree dev services, navigation,
and cleanup — replacing manual `git worktree` juggling with one streamlined workflow.

> **This README is a guide, not a reference.** Every command is self-documenting:
> run `wtm <command> --help` for its full flags, or browse the generated reference in
> [`docs/`](docs/wtm.md). The tables below are just a map.

## Dependencies

| Tool | Required | Purpose |
|---|---|---|
| `git` | ✅ Required | Worktree management |
| `gh` | ⭐ Recommended | PR listing, creation, checkout, and prune (merged/closed detection) |

`gh` is optional — worktree creation, navigation, hooks, and services all work without
it. Install and authenticate it to unlock GitHub features: [cli.github.com](https://cli.github.com).

## Installation

**Homebrew (macOS / Linux)**

```bash
brew install LucasPcq/tap/wtm
```

**Download binary** — grab the latest [release](https://github.com/LucasPcq/wtm/releases), extract, and move it onto your `PATH`:

```bash
tar -xzf wtm_*_darwin_arm64.tar.gz   # or _darwin_amd64 / _linux_amd64
sudo mv wtm /usr/local/bin/
```

**Go install**

```bash
go install github.com/LucasPcq/wtm@latest
```

### Updating

```bash
wtm upgrade          # update to the latest release
wtm upgrade --check  # see what's available without installing
```

`wtm upgrade` detects how wtm was installed: a standalone binary is replaced in
place after its checksum is verified, a Homebrew or `go install` binary is handed
to that tool. It updates the CLI itself — for your worktrees, that's `wtm sync`.

wtm also checks for new releases at most once a day and prints a notice on stderr.
It stays silent in CI, without a TTY, and under `--output json`. Disable it with
`WTM_NO_UPDATE_CHECK=1`, or in `~/.config/wtm/config.toml`:

```toml
[update]
check = false
```

## Quick Start

```bash
# 1. Shell integration — required so `go` can cd for you
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc && source ~/.zshrc

# 2. Initialize wtm in your repo
cd your-repo && wtm init

# 3. Everyday flow
wtm create feature/login     # create a worktree (env + hooks provisioned)
wtm go feature/login         # cd into it
wtm list                     # see all worktrees
wtm sync --all               # rebase the stack onto its parents
wtm clean feature/login      # remove it when the PR merges
```

## Teach your LLM to use wtm

Working with Claude Code or Cursor? Run:

```bash
wtm agents install
```

It detects `.claude/` and `.cursor/` (project and home) and installs a `using-wtm`
skill so your agent can drive every command via the [`--output json`](#machine-readable-output) contract — without being told how each session.

## Concepts

A few ideas explain how the commands fit together:

- **Worktree** — a checked-out branch in its own directory. wtm creates them under
  `base_path` (from `wtm init`) and records per-worktree metadata under
  `<git-common-dir>/wtm/`. A branch that already exists locally is checked out as-is
  (`create` and `checkout` both keep its commits — nothing to clean up first); only a
  branch another worktree already holds is refused. See [`create`](docs/wtm_create.md), [`list`](docs/wtm_list.md), [`clean`](docs/wtm_clean.md).
- **Stacking** — every worktree records the parent branch it was created from
  (`source_branch`) — the branch it was created from, or the parent you pick when reusing
  an existing branch. [`sync`](docs/wtm_sync.md) rebases a worktree (and its descendants,
  in cascade) onto its parent; [`tree`](docs/wtm_tree.md) shows the parent→child forest and
  which branches need syncing; [`reparent`](docs/wtm_reparent.md) rewires the parent after a
  middle branch merges.
- **Env provisioning** — on `create`, wtm copies `.env` files into the new worktree using
  a **strategy** (`example` / `main` / `parent`). See [Env strategies](#env-strategies).
- **A worktree's ports live in its `.env`** — that is what makes isolation hold when you
  start something yourself. wtm shifts the ports those files carry (see
  [Ports hard-coded in a `.env`](#ports-hard-coded-in-a-env)) and writes
  `COMPOSE_PROJECT_NAME` into the file each compose stack reads, so a `docker compose up`
  or a `pnpm dev` typed in a worktree is isolated with no wtm process involved.
- **Shell integration** — `go` changes your current directory, which a child process can't
  do for its parent shell. `eval "$(wtm shell-init)"` installs a shell function that makes
  it work. Without it, use [`resolve`](docs/wtm_resolve.md) to get a path.
- **Dev jobs** *(experimental)* — long-running **services** (dev servers, docker) and
  one-shot **tasks** (migrations, seeds) declared in `run.toml` and grouped into
  **profiles**, run per-worktree by a background daemon. Starting them **attaches**: the
  run view opens one pane per job, leaving it detaches without stopping anything, and
  `-d` skips it. The flow is still stabilizing — `wtm go` is the recommended way to enter
  a worktree today. See [`run`](docs/wtm_run.md) and [Run config](#run-config--runtoml).
- **Shared services** *(experimental)* — a job declared `scope = "shared"` runs **once for
  the repository** instead of once per worktree, in the main checkout: a postgres, a
  keycloak. Each worktree still keeps its own data through a `[job.namespace]` block, whose
  `attach` and `detach` commands you write — wtm names the namespace and hands them the
  worktree's environment, and knows nothing else about them. It exists for the case that
  makes isolation expensive: two worktrees of a monorepo with four databases and a JVM.

## Commands

Full flags live in `wtm <command> --help` and [`docs/`](docs/wtm.md). Overview:

### Worktrees

| Command | Purpose |
|---|---|
| [`create`](docs/wtm_create.md) | Create a new worktree (runs env provisioning + `on_create` hooks) |
| [`list`](docs/wtm_list.md) | List all worktrees |
| [`tree`](docs/wtm_tree.md) | Show the worktree forest (parent → child) |
| [`clean`](docs/wtm_clean.md) | Remove a worktree and its local branch |
| [`prune`](docs/wtm_prune.md) | Remove finished worktrees (merged / closed PR / gone) in one pass — merged/closed need `gh` |
| [`extract`](docs/wtm_extract.md) | Move uncommitted changes to another worktree (split an oversized PR) |
| [`env`](docs/wtm_env.md) | Detect and fix a worktree's `.env` drift against its template + value source |
| [`relocate`](docs/wtm_relocate.md) | Move worktrees to align with `base_path` and adopt external ones |
| [`ui`](docs/wtm_ui.md) | Open the full-screen worktree dashboard: browse state and PRs, create and delete worktrees |

### Navigate

| Command | Purpose |
|---|---|
| [`go`](docs/wtm_go.md) | cd into a worktree |
| [`resolve`](docs/wtm_resolve.md) | Print a branch's worktree path (for scripts / agents) |

### Stacked branches

| Command | Purpose |
|---|---|
| [`fast-forward`](docs/wtm_fast-forward.md) | Advance worktree branches to `origin/<branch>` — no rebase, no merge |
| [`sync`](docs/wtm_sync.md) | Rebase selected worktrees onto their parent, in cascade |
| [`reparent`](docs/wtm_reparent.md) | Change the parent a worktree is rebased onto |

### Dev jobs *(experimental)*

Per-worktree services + tasks. Functional, but the flow is still stabilizing. The
run module is **opt-in**: run `wtm run init` once to set it up (the global `wtm init`
no longer touches services). Until then, run commands stop with a hint pointing there.

| Command | Purpose |
|---|---|
| [`run init`](docs/wtm_run_init.md) | Set up run.toml (detect docker-compose + scripts, pre-fill ports, publish URLs, write and link .env keys) |
| [`run up`](docs/wtm_run_up.md) / [`down`](docs/wtm_run_down.md) | Start / stop a profile's jobs on one or more worktrees (`up` attaches, `-d` detaches) |
| [`run start`](docs/wtm_run_start.md) / [`stop`](docs/wtm_run_stop.md) | Start / stop a single job (`start` attaches, `-d` detaches) |
| [`run ps`](docs/wtm_run_ps.md) / [`list`](docs/wtm_run_list.md) | Running jobs, every repository / declared jobs + profiles |
| [`run logs`](docs/wtm_run_logs.md) | Reopen the run view on one or more worktrees' jobs |
| [`run url`](docs/wtm_run_url.md) / [`open`](docs/wtm_run_open.md) | Print / open where a job answers in this worktree |
| [`run export`](docs/wtm_run_export.md) / [`import`](docs/wtm_run_import.md) | Share a job layout between machines |
| [`run job`](docs/wtm_run_job.md) / [`profile`](docs/wtm_run_profile.md) | Add / remove / edit jobs and profiles |
| [`run proxy`](docs/wtm_run_proxy.md) | Report, install or remove the redirection that serves named URLs on port 80 |
| [`run daemon`](docs/wtm_run_daemon.md) | Inspect, stop or restart the process that runs the jobs |

### GitHub

| Command | Purpose |
|---|---|
| [`checkout`](docs/wtm_checkout.md) | Create a worktree from an existing pull request (needs `gh`) |

### Setup

| Command | Purpose |
|---|---|
| [`init`](docs/wtm_init.md) | Initialize wtm configuration |
| [`shell-init`](docs/wtm_shell-init.md) | Generate the shell integration function |
| [`config`](docs/wtm_config.md) | Inspect or edit the project config |
| [`agents`](docs/wtm_agents.md) | Install the `using-wtm` skill for LLM agents |
| [`schema`](docs/wtm_schema.md) | Extract the bundled JSON Schemas |
| [`upgrade`](docs/wtm_upgrade.md) | Update wtm itself to the latest release |

## Machine-readable output

Every data command supports `--output json` for scripting and LLM agents. JSON is
pretty-printed on stdout with `snake_case` fields; human messages stay on stderr; exit
codes are unchanged.

**The payload *is* the schema** — its shape mirrors the command's Go type and stays
stable. Discover the exact fields by running the command once:

```bash
wtm list --output json | jq '.[] | select(.is_dirty).branch'
wtm checkout 42 --output json | jq '.path'
wtm run ps --output json | jq '.[] | select(.status=="running" or .status=="detached").name'
```

`--quiet` (`-q`) is the other half: it silences the human report on **every** command
while leaving the exit code, the errors and the machine contracts untouched — a JSON
document, a resolved path, a shell script or a URL still comes through. It is the
output axis only, so a fully unattended run pairs it with `--yes`.

Non-interactive note: `--output json` never prompts, so destructive commands need an
explicit flag — `clean`/`prune` need `--yes` (or `--force`), and `sync` needs branch
args or `--all`. See each command's `--help`.

## Configuration

All wtm files live under `<git-common-dir>/wtm/` (`.git/wtm/` for a normal clone). Git
never commits anything inside `.git/`, so wtm is invisible to teammates and to
`git status` — worktree usage stays personal.

```
<git-common-dir>/wtm/
├── config.toml                    # project settings
├── run.toml                       # dev jobs + profiles
├── schemas/                       # JSON schemas for editor autocomplete
└── worktrees/<encoded-branch>/
    └── meta.json                  # per-worktree metadata (source branch, timestamp, env strategy)
```

Everything is plain TOML, validated at load time (unknown keys are rejected, not
silently ignored). Edit by hand, or use `wtm config show` / `wtm config edit` and the
`wtm run import` flow.

### Project config — `config.toml`

Generated by `wtm init`, per-clone, never committed.

```toml
[worktrees]
base_path   = "../.trees"   # where worktrees are created (relative to repo root)
base_branch = "main"        # default base for new worktrees

[env]
strategy = "example"        # example | main | parent (see below)

# Each detected value file and its committed template (schema). wtm distinguishes
# templates (.env.example / .dist / .sample / .template / .tmpl, committed) from
# value files (.env, gitignored). .env.local is detected and flagged local but
# stays syncable.
[[env.file]]
target   = ".env"
template = ".env.example"

[[env.file]]
target = ".env.local"
local  = true

[hooks]
on_create = [
  "pnpm install",                                             # string: runs from worktree root
  { cmd = "pnpm install", cwd = "apps/api" },                 # object: runs from a subdir
  { cmd = "pnpm install", cwd = "apps/web", continue_on_error = true },  # non-fatal
]
on_clean = [
  "docker compose down",                                      # runs right before a worktree is removed
]
```

`on_create` hooks run after a worktree is created; `on_clean` hooks run in the worktree
just before it is removed by `clean`/`prune` (e.g. to tear down external resources). A
non-zero hook aborts the operation unless the entry sets `continue_on_error`. Hooks
interpolate `{{worktree}}`, `{{branch}}`, `{{root}}`, and (for `on_create`) `{{from_branch}}`.

### Env strategies

| Strategy | Behavior |
|---|---|
| `example` | Copies `file.example` from the main worktree, renamed to `file`. Warns if `.example` is missing. |
| `main` | Copies the actual file from the main worktree. |
| `parent` | Copies from the source worktree (`--from`), falling back to `main`. |

The strategy is recorded per worktree. Later, [`wtm env`](docs/wtm_env.md) reconciles a
worktree's `.env` against its **template** (the committed schema) plus the **same value
source** — adding missing keys, and (with `--mode refresh`) settling values that drifted.
Override the source for a single run with `--from`; the report always shows which source
was used.

### Run config — `run.toml`

Optional and opt-in — created by `wtm run init` (which detects docker-compose files
and package scripts), not by the global `wtm init`. Declares dev **jobs** and groups
them into **profiles**. Per-clone, never committed — share layouts with
`wtm run export | wtm run import -`.

A job's `cmd` (and its `stop`) is a **`/bin/sh` line**, not a whitespace-split argv:
quotes, `&&`, pipes, redirections and globs behave as they do in a terminal, and `${VAR}`
expands from the job's environment. POSIX `sh` is used on every machine, never your own
interactive shell, so a shared `run.toml` behaves the same everywhere.

Two worktrees can run their stacks side by side — each has its own ports and resource
names — and `wtm run up feat-a feat-b` brings up as many as you name at once, each
independent of the others. The first time `wtm run up` finds another worktree's jobs
running it asks what to do about the machine's load, and can write the answer as
`concurrency = "parallel" | "exclusive"` at the top of the file so it never asks again.
`--parallel` and `--exclusive` override it for a single run; `--exclusive` is refused on
several worktrees, since it stops all but one.

```toml
[[job]]
name = "docker"
kind = "service"            # long-running; with `stop` it's detached
cmd  = "docker compose up -d"
stop = "docker compose down"
  [job.ports]              # host binding per worktree; template it as "${DB_PORT}:5432"
  DB_PORT = 5432

[[job]]
name = "web"
kind = "service"
cmd  = "pnpm dev"
  [job.ports]              # PORT=3000 on the main checkout, 3010 on the next worktree
  PORT = 3000

[[job]]
name = "migrate"
kind = "task"               # one-shot; blocks the profile, streams output, non-zero aborts
cmd  = "pnpm migrate"

[[profile]]
name    = "full"
jobs    = ["docker", "web", "migrate"]
default = true
```

Jobs are scoped per worktree at runtime: starting `docker` from worktree A runs it with
`cwd = A`; a separate process runs from worktree B. `wtm run down` only stops the current
worktree's jobs unless you pass `--all`.

Every job — and every `on_create` / `on_clean` hook — also runs with the worktree's own
identity in its environment, so two worktrees running the same services never share a
resource:

| Variable | Value |
| --- | --- |
| `WTM_BRANCH` | the branch, verbatim |
| `WTM_WORKTREE` | the branch as a slug safe for a Docker project, network or volume name |
| `WTM_ORDINAL` | the worktree's stable number. The main checkout is always `0`; every other worktree gets the smallest number free, kept for its whole life and released when it is cleaned |
| `WTM_PORT_OFFSET` | `WTM_ORDINAL` × the block (`port_offset_block`, 10 by default) — the main checkout keeps the project's default ports |
| `COMPOSE_PROJECT_NAME` | `<repo>-<WTM_WORKTREE>`, unless your own environment already defines it. The Docker daemon is machine-wide, so the repository qualifies the name: two clones both sitting on `main` do not share a stack |

`COMPOSE_PROJECT_NAME` is what keeps two worktrees' containers, networks and volumes
apart — nothing to declare, it works as soon as your jobs use `docker compose`. It reaches
everything compose names for you, and nothing your file names itself: a `container_name`,
or a volume's or network's explicit `name`, is resolved by the Docker daemon directly, so
the second worktree to start meets the first one's. `wtm run init` finds those and offers
to front them with the project — see [absolute names](#absolute-names) below.

**Ports** are declared per job, and wtm injects `base + WTM_PORT_OFFSET` under the name
you chose — the command itself needs no arithmetic:

```console
$ wtm run job add web --cmd "pnpm dev" --port PORT=3000
$ wtm run up
✓ web started · PORT=3010
```

**wtm checks that the port was actually bound.** Declaring a port only injects a
variable — nothing guarantees the command reads it. Once the jobs are up, `run up` dials
each declared port and reports the ones nothing answers on:

```console
$ wtm run up
✓ web started · WEB_PORT=5183

  Ports declared but not bound
  web · nothing is listening on WEB_PORT=5183
    but 5173 is listening — the base port
    the command ran, but the variable did not reach it
```

The second line is the signature of a variable that never arrived: a CLI that only takes
`--port`, a hard-coded port, a `.env` that wins, or a task runner filtering the
environment — **Turborepo does this by default** (`envMode: "strict"`), so a root
`turbo run dev` job needs `globalPassThroughEnv` in `turbo.json` for the ports to reach
its packages. wtm never edits those files; it tells you what it observed.

It never fails the run, and a healthy stack costs nothing — the check stops as soon as
every port answers. `--no-probe` skips it, and `port_probe_timeout` in run.toml sets the
budget (default 15s, a negative value turns it off).

A declaration overrides whatever the environment already sets for that variable, and the
job's `stop` command runs with the same ports its `cmd` did. For Docker, template the host
side of the mapping (`"${DB_PORT}:5432"`) and declare `DB_PORT = 5432`: the container port
never moves, only the binding does.

The `on_create` / `on_clean` hooks get those ports too, so the `docker compose down` of an
`on_clean` reads the same `${DB_PORT}` its `up` bound. A hook is not a job, though, so it
only gets the names a **single** job declares: if `web` and `api` both declare `PORT` on
different bases, `PORT` has no answer outside a job and is left unset rather than resolved
to one of the two.

`wtm run init` composes a configuration you can start, not an inventory of the repo. It
proposes everything it finds and **checks the fewest things**: only scripts whose name
contains `dev`, and not a root `dev` a workspace package also declares — that one is an
orchestrator (`turbo run dev`, `pnpm -r dev`) and running it beside the packages it fans
out to would start each of them twice on the same ports. Nothing unchecked is written.

It asks which of them starts the others — the relation is declared, never inferred from a
command. A root can name another root, so `dev` → `dev:shop` → the shop apps is written one
row at a time and what the top one holds is read through the whole chain; two roots may
also name the same app. Only a cycle is refused.

It also asks which jobs should answer under their own name, and proposes every service that
declares the port it listens on — `PORT`, or `<JOB>_PORT` for the ones after the first. A
port a job only dials (`DB_PORT`, `REDIS_PORT`) is never proposed: a name nothing answers
under is worse than no name at all. Unchecking a job withdraws the `url` it already had.

It then walks you through the ports detection pre-filled, and the **profiles** `wtm run
up` will offer: one per package, plus one gathering everything, which you rename, merge
or drop. Jobs at the repository root — a compose stack — join every profile, so starting
one package alone still brings its infrastructure up. Tasks are placed ahead of the
services that depend on them, so a profile brings the database up to date before starting
what reads it. In a single-package repo — or past a handful of packages, where a profile
each stops being something you can read — the split collapses to one profile.

A service detection found no port for is reported rather than asked about: inventing one
would move the guess onto you, and `wtm run up` will say the port was never bound anyway.

`wtm run init` writes those Docker declarations for you. It reads the `ports:` of the
compose files you pick: a mapping that already reads a variable is declared as-is, while a
literal `"5432:5432"` would bind the same port in every worktree and is therefore **not**
declared — wtm shows the line to write instead. Pass `--patch-compose` and it makes the
change itself:

```diff
   postgres:
     ports:
-      - "5432:5432"
+      - "${POSTGRES_PORT:-5432}:5432"
```

Only the port value is rewritten, at its exact position — comments, indentation and
quoting style are untouched — and the `:-5432` default keeps `docker compose up` working
on its own, with no dependency on wtm. Re-running `run init` backfills a compose job that
predates declarative ports without overwriting one you set by hand.

Dev servers are pre-filled the same way, from the env files next to their `package.json` —
a `PORT` or `*_PORT` entry in `.env.local`, `.env`, or a committed `.env.example`. Each
job takes the file in its own directory, so in a monorepo every package keeps its own port.
wtm declares the port and never rewrites a command. Where the job *reads* that port is a
question the wizard puts, job by job: from its own `.env` — wtm writes `KEY=<base>` there
and in the committed template, and your config reads it (`server.port:
Number(process.env.VITE_PORT)`) — or from the command, `--cmd 'pnpm dev --port ${PORT}'`.
The `.env` route is the pre-filled answer because it is the only one that still holds when
you start the app yourself; the command route isolates what `wtm run` starts and nothing
else, which the final report says in as many words. `--write-port-keys` takes the first
route for every job without asking. The port it declares is also the base the `[[env_port]]`
links below follow, so a `.env` holding both `PORT=5173` and a `VITE_API_URL` pointing at
it ends up with the two shifted together.

<a id="absolute-names"></a>
**Absolute names.** A compose file that pins its own names bypasses the project prefix,
which is what makes a second worktree fail outright — Docker refuses a duplicate
`container_name`, and a volume or network pinned by `name` is silently shared instead.
`run init` reports them, and `--patch-compose` fronts each with the project:

```diff
   postgres:
-    container_name: myapp-postgres
+    container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"
```

The `:-myapp` default reproduces the name the file used to pin, so `docker compose up`
on its own is unchanged. Ports and names are one question, not two — accepting half of
them still leaves two worktrees unable to run at once. A `name` under `external: true`
is left alone (sharing it is the declaration's whole point), as is one that already reads
a variable, and a volume declared as a bare key was never affected: compose already
prefixes it with the project. **A volume that pinned its `name` gains one per worktree,
each starting empty** — the data already written stays under the old name, and moving it
across is yours to do.

wtm declares only what it can actually isolate, and says why for the rest: a port range,
a mapping with no host port, a `ports:` list carrying a YAML **anchor or alias** (rewriting
it would move every service sharing it), a `${DB_PORT}` with no default (the file never
says which port it stands for), and a variable two services declare with two different
defaults. It also withdraws a detected port rather than write a `run.toml` its own loader
would refuse — two bases a multiple of the block apart — naming both sides.

A server that **ignores** `PORT` and only takes its port as a CLI flag — Vite is the usual
one — reads it back from the same variable, because `cmd` is a shell line:

```console
$ wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000
```

#### Ports hard-coded in a `.env`

Shifting a service's host port only helps if whatever connects to it follows. That is easy
when the consumer reads `${DB_PORT}` — but in most projects the port is not in a variable
of its own, it is **buried in a URL**: `DATABASE_URL=postgres://u:pw@localhost:5432/app`,
`API_URL=http://localhost:3000/api`. An app running on the host, outside Docker, then talks
to the wrong worktree.

An `[[env_port]]` link says which key carries which port:

```toml
[[env_port]]
file = ".env"
key  = "DATABASE_URL"
port = "POSTGRES_PORT"      # a port declared by one of the jobs above
```

The link names the key, never a position. wtm looks for the **declared base** inside the
value and shifts only that number, leaving credentials, host, path and query exactly as
they were:

```diff
-DATABASE_URL=postgres://u:pw@localhost:5432/app
+DATABASE_URL=postgres://u:pw@localhost:5442/app
```

`wtm run init` scans your configured `.env` targets and offers the keys whose value holds a
declared base; `--link-env` writes them without asking. Nothing is ever inferred without one
or the other. The rewrite then happens when a worktree is created — proposed interactively,
applied under `--yes` — and whenever `wtm env` reconciles, whose recap offers "Apply, but
leave the port values alone" beside the plain apply, so neither command imposes the pass
(`--check` reports without writing, and counts a pending shift as drift). `wtm env --mode refresh` compares linked values **modulo the offset**, so a
worktree holding `5442` against a `main` holding `5432` is not a conflict; a real difference
in the same value still is.

wtm reports rather than guesses when it cannot be sure: the key is missing, the base appears
more than once in the value, or neither the base nor any offset of it is there. Rewriting on
a guess could corrupt a URL, so those lines are named and left alone.

#### Values that carry an address, not a port

A port in a `.env` is enough for one app talking to itself. It is not enough the moment a
front end calls a separate API: the browser is on the worktree's **name**, so the `Origin`
it sends is a name, and a `CORS_ORIGIN` holding `http://localhost:5183` blocks it. Ports and
named URLs cannot both be half-true in the same file.

So a link writes the job's **whole origin** rather than its port number, whenever two things
hold at once — the job it names **publishes a url** for that very port, and the value **has
the shape of a URL**:

```diff
-VITE_API_URL=http://localhost:4001
+VITE_API_URL=http://api-dev.feat-x.monorepo.localhost
-CORS_ORIGIN=http://localhost:5173
+CORS_ORIGIN=http://web-dev.feat-x.monorepo.localhost
 PORT=4011                                    # a bare number stays a number
 DATABASE_URL=postgres://u:pw@localhost:5442/app   # Postgres has no name, and never will
```

Both conditions matter. The first leaves Postgres alone — the proxy only speaks HTTP. The
second leaves the binding keys alone: `PORT` belongs to a job that *does* publish a name, and
must still be a number. Without the redirection installed the address carries the proxy's
port (`…localhost:11080`), which changes nothing for CORS and nothing for cookie isolation —
a port is part of an origin, but never part of a *cookie's* origin.

Set `addressing = "ports"` at the top of `run.toml` to keep port numbers everywhere; it is a
real inverse, and a later `wtm env` puts the ports back. On a machine where the proxy is off,
ports are written whatever the project asked for, and a notice says so. Under `names` the
named URL becomes the only working entrance: opening `localhost:5183` directly sends an
`Origin` the API no longer knows. `wtm run url` and `wtm run open` hand out the right link.

The rewrite happens where wtm provisions: a worktree, when it is created and whenever
`wtm env` reconciles it. **Nothing ever writes the main checkout's `.env`** — it is the one
checkout that exists without wtm, the one a colleague clones and a `docker compose up` reads.
So under `names` its values still hold ports — and then **the working entrance is the port**,
not the name: the browser on `localhost:5175` sends an `Origin` the API's `CORS_ORIGIN`
recognises, while the named URL sends one it does not. wtm follows the file rather than the
setting: a worktree whose `.env` still spells ports is handed `http://localhost:<port>`
everywhere — `run up`, `run url`, `run open`, the run view, the `wtm ui` panel — with one line
saying why, and the named URL comes back the moment the file says so. The route is registered
either way, so nothing has to restart:

```bash
wtm env main        # the positional takes the main checkout like any other worktree
```

wtm only ever sees the keys declared as `[[env_port]]` links: a `CORS_ORIGIN` nothing links
to a declared port is invisible to both the pass and the warning, so silence means "nothing
linked is out of step", not "everything is right". And a `.env` that already holds named
origins whose port went stale — what `wtm run proxy install` does to every worktree at once —
keeps its names and is told they are out of step, rather than being sent back to ports.

Doing it is a choice, not a formality. Main then stops behaving as a checkout without wtm:
whoever reads that `.env`, or starts the stack from it, depends on the proxy being up. Two
moments make it worth doing — right after switching a project to `names`, and after
`wtm run proxy install`, which drops the `:11080` from the origins already written. Going
back takes three steps, `addressing` being a project setting with no per-worktree scope: set
`addressing = "ports"`, run `wtm env main`, then set `names` again — main stays on ports until
a `wtm env main` says otherwise. And `wtm create` from main is unaffected either way: a copied
value carrying main's segment is recognised and rewound to the new worktree's.

Two base ports must not differ by a **multiple of the block**, or two worktrees end up on
the same one — `3000` and `3010` are refused when `run.toml` is read, naming both sides,
which is the last moment the problem is still explainable. Neighbouring ports are fine:
a uniform offset preserves the gaps, so `5434`/`5435`/`5436` become `5444`/`5445`/`5446`
on the next worktree. Set `port_offset_block` at the top of `run.toml` when a project's
ports genuinely need more room than 10.

> **Upgrading:** jobs used to run with no `COMPOSE_PROJECT_NAME`, so `docker compose`
> named the project after the working directory. Stacks started before this version are
> under the old name and a new `run up` will not find them — stop them once with
> `docker compose -p <old-name> down`.
>
> The run daemon is global and outlives the command that started it, so a daemon started
> by an older binary would keep serving its own behavior. It is now refused rather than
> silently used: any `run` command names both versions and points at
> `wtm run daemon restart`, which hands the jobs over. Detached services survive that
> restart; foreground ones are stopped.

`run up` and `run start` **attach**: a full-screen view opens with one pane per job, and
`wtm run logs` reopens it later. Leaving the view (`q`, or Ctrl+C outside focus mode)
detaches — the daemon keeps the jobs running. `-d` starts them and hands the prompt back
instead. Without a terminal, or under `--output json`, no view opens: the run reports
itself as lines, which is what a script or an agent gets. Each job's output is also
journaled to `<git-common-dir>/wtm/logs/<url-escaped-branch>/<url-escaped-job>.log`
(5 MB x 3 within one run), and `run logs` reads that back for a job that is no longer
running. Starting a job clears its log first, so one file is one run: what `run logs`
replays never reaches back into an earlier one.

The daemon itself is disposable. It exits ~30 s after the last **foreground** job, while
detached services — those with a `stop` command, a `docker compose up -d` typically —
keep running without it: the real work belongs to Docker, not to wtm. What wtm keeps is
an index of what it started, in `~/.config/wtm/jobs.json`, which the next daemon reads
back. That is what makes `wtm run ps` still list your stacks after a reboot, and
`wtm run down` still stop them. Those stacks show as `detached` rather than `running`,
because nothing about them was ever verified — wtm launched them and has not seen them
since. `wtm run daemon status` reports what is up, and `stop` / `restart` are the way
out when you want the process gone.

### Global config

Created by `wtm init`, personal to each developer. It lives under the OS config
directory — `~/.config/wtm/config.toml` on Linux, `~/Library/Application Support/wtm/config.toml`
on macOS — and `wtm run proxy status` prints the resolved path.

```toml
shell = "zsh"          # zsh | bash | fish

[ui]
animations = true      # false disables every wtm ui animation (tab rule, new-row flash)

[proxy]
port = 11080           # where named job URLs are served, on the loopback only
enabled = true         # false sends every URL back to http://localhost:<port>
```

`ui.animations` defaults to on when absent — set it to `false` to turn off every
`wtm ui` animation at once, useful over a slow or laggy connection.

`[proxy]` serves each worktree's HTTP jobs under their own hostname
(`http://<job>.<worktree>.<repo>.localhost:11080`), so two worktrees stop sharing one
cookie jar — a job opts in with `url = { port = "PORT" }` in `run.toml`, which `wtm run
init` writes for the services it detects. Both keys default
to the values above; the proxy lives in the background daemon and dies with it, and a port
it cannot bind costs the names, never the jobs.

A job started **by another job** is served too. A root `turbo run dev` is one process
holding several apps, declared as `runs = ["web", "api"]`: starting it registers the name
of every published job it runs, so `http://web.<worktree>.<repo>.localhost:11080` answers
even though the only job the daemon holds is the runner. Those addresses are reported on
the runner — under its line in a run, on its row in `wtm ui` — and the apps it holds get
no row of their own while it is up: they are its subprocesses, not jobs beside it.

## IDE autocomplete + validation

Every TOML file `wtm init` writes starts with a `#:schema ./schemas/...json` directive.
Pair it with [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml)
(or the bundled JetBrains TOML plugin) for autocomplete, hover docs, and real-time
validation. Schemas ship with the binary and are written locally by `wtm init` — no
internet required. Re-extract them after upgrading:

```bash
wtm schema dump            # <git-common-dir>/wtm/schemas/{run,project}.schema.json
wtm schema dump --global   # global.schema.json, beside the global config
```

## Contributing to the docs

The [`docs/`](docs/wtm.md) reference is generated from the Cobra command tree — never
edit it by hand. After changing any command or flag, regenerate:

```bash
make docs
```

## License

MIT
