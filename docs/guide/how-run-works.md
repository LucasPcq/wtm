# How `wtm run` works

`wtm run init` writes `.git/wtm/run.toml` from the compose files and package scripts it detects; `wtm run up` starts it, once per worktree:

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

- The module is opt-in (`wtm run init`, not `wtm init`), per clone and never committed: share a layout with `wtm run export | wtm run import -`.
- `cmd` and `stop` are **`/bin/sh` lines**, not argv: quotes, `&&`, pipes, redirections, globs and `${VAR}` behave as in a terminal. POSIX `sh` is used everywhere, never your interactive shell, so a shared `run.toml` behaves the same on every machine.
- Jobs are scoped per worktree: `docker` started from worktree A runs with `cwd = A`, and a separate process runs from worktree B. `wtm run up feat-a feat-b` brings up several at once, each independent. `wtm run down` stops the current worktree's jobs; `--all` stops every worktree of this repository, never another one.
- When another worktree's jobs are already running, `run up` asks once about the machine's load: see [Several worktrees at once](jobs-and-profiles.md#several-worktrees-at-once).

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

`on_create` / `on_clean` hooks get the same variables and ports, under the condition given in [Project config](configuration.md#project-config-configtoml), so an `on_clean`'s `docker compose down` reads the `${DB_PORT}` its `up` bound. A hook only gets the port names a **single** job declares: if `web` and `api` both declare `PORT` on different bases, `PORT` is unset in a hook.

## Ports

Declare the base port under the name the command reads; wtm injects `base + WTM_PORT_OFFSET`, so the command needs no arithmetic:

```console
$ wtm run job add web --cmd "pnpm dev" --port PORT=3000
$ wtm run up
✓ web started · PORT=3010
```

- A declaration overrides whatever the environment already sets for that variable, and `stop` runs with the same ports its `cmd` did.
- For Docker, template the host side of the mapping (`"${DB_PORT}:5432"`) and declare `DB_PORT = 5432`: the container port never moves, only the binding does.
- A server that ignores `PORT` and only takes a CLI flag (Vite is the usual one) reads the same variable, because `cmd` is a shell line:

  ```bash
  wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000
  ```

- Two base ports must not differ by a **multiple of the block**, or two worktrees land on the same one: `3000` and `3010` are refused when `run.toml` is read, naming both. Neighbouring ports are fine, since a uniform offset keeps the gaps (`5434`/`5435`/`5436` become `5444`/`5445`/`5446`). Raise `port_offset_block` when a project needs more room than 10.

### The port check

Declaring a port only injects a variable; nothing guarantees the command reads it. Once the jobs are up, `run up` dials each declared port and reports the ones nothing answers on:

```console
$ wtm run up
✓ web started · WEB_PORT=5183

  Ports declared but not bound
  web · nothing is listening on WEB_PORT=5183
    but 5173 is listening — the base port
    the command ran, but the variable did not reach it
```

The "base port" line is the signature of a variable that never arrived: a CLI that only takes `--port`, a hard-coded port, a `.env` that wins, or a task runner filtering the environment. **Turborepo does this by default** (`envMode: "strict"`): a root `turbo run dev` needs `globalPassThroughEnv` in `turbo.json`. wtm never edits those files; it reports what it observed.

The check never fails the run and stops as soon as every port answers; its switches (`--no-probe`, `probe = false`, `port_probe_timeout`, `binds_no_port`) are in [Checking the ports](jobs-and-profiles.md#checking-the-ports).

## What `wtm run init` proposes

`wtm run init` composes a configuration you can start, not an inventory: it proposes everything it finds and checks the fewest things.

| Step | What it proposes |
| --- | --- |
| Jobs | only scripts whose name contains `dev` are checked, minus a root `dev` a workspace package also declares: that one is an orchestrator (`turbo run dev`, `pnpm -r dev`) and would start each package twice on the same ports. Nothing unchecked is written |
| Runners | which job starts which others. The relation is declared, never inferred from a command; a root can name another root (`dev` → `dev:shop` → the shop apps), two roots may name the same app, only a cycle is refused |
| Named URLs | every service that declares the port it listens on (`PORT`, or `<JOB>_PORT` after the first). A port a job only dials (`DB_PORT`, `REDIS_PORT`) is never proposed. Unchecking a job withdraws the `url` it had |
| Ports | the values detection pre-filled. A service with no port found is reported rather than given an invented one; `run up` will say if it never bound |
| Profiles | one per package plus one gathering everything, to rename, merge or drop. Root jobs (a compose stack) join every profile; tasks are placed ahead of the services that depend on them. A single-package repo, or one past a handful of packages, gets one profile |

### Compose ports

`run init` reads the `ports:` of the compose files you pick. A mapping that already reads a variable is declared as-is; a literal `"5432:5432"` would bind the same port in every worktree, so it is **not** declared and wtm shows the line to write. `--patch-compose` makes the change itself:

```diff
   postgres:
     ports:
-      - "5432:5432"
+      - "${POSTGRES_PORT:-5432}:5432"
```

Only the port value is rewritten, in place (comments, indentation and quoting untouched), and the `:-5432` default keeps `docker compose up` working without wtm. Re-running `run init` backfills a compose job that predates declarative ports without overwriting one you set by hand.

### Dev-server ports

Dev servers are pre-filled from the env files next to their `package.json`: a `PORT` or `*_PORT` entry in `.env.local`, `.env`, or a committed `.env.example`, each job taking the file in its own directory. wtm declares the port and never rewrites a command. Where the job *reads* it is asked job by job:

- **From its own `.env`** (the pre-filled answer): wtm writes `KEY=<base>` there and in the committed template, and your config reads it (`server.port: Number(process.env.VITE_PORT)`). It still holds when you start the app yourself. `--write-port-keys` takes this route for every job without asking.
- **From the command**: `--cmd 'pnpm dev --port ${PORT}'`. It isolates only what `wtm run` starts, which the final report says.

The declared port is also the base the [`[[env_port]]` links](#ports-hard-coded-in-a-env) follow, so a `.env` holding `PORT=5173` and a `VITE_API_URL` pointing at it shifts both together.

<a id="absolute-names"></a>
### Absolute names

A compose file that pins its own names bypasses the project prefix: Docker refuses a second `container_name`, and a volume or network pinned by `name` is silently shared. `run init` reports them, and `--patch-compose` fronts each with the project:

```diff
   postgres:
-    container_name: myapp-postgres
+    container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"
```

The `:-myapp` default reproduces the old name, so `docker compose up` alone is unchanged. Ports and names are one question: accepting half still leaves two worktrees unable to run at once.

- Left alone: a `name` under `external: true` (sharing is its point), a name that already reads a variable, and a volume declared as a bare key (compose already prefixes it).
- **A volume that pinned its `name` gains one per worktree, each starting empty**: the data stays under the old name, and moving it is yours to do.

### What it will not declare

wtm declares only what it can isolate, and says why for the rest:

| Found | Why it is left out |
| --- | --- |
| a port range, or a mapping with no host port | nothing to shift |
| a `ports:` list with a YAML anchor or alias | rewriting it would move every service sharing it |
| `${DB_PORT}` with no default | the file never says which port it stands for |
| one variable two services declare with different defaults | no single base |
| two detected bases a multiple of the block apart | `run.toml` would be refused; both sides are named |

## Ports hard-coded in a `.env`

A shifted port only helps if what connects to it follows, and the port is usually buried in a URL. An `[[env_port]]` link says which key carries which port:

```toml
[[env_port]]
file = ".env"
key  = "DATABASE_URL"
job  = "docker"             # required: the job declaring the port
port = "DB_PORT"            # a port that job declares
```

wtm finds the **declared base** inside the value and shifts only that number; credentials, host, path and query stay as they were:

```diff
-DATABASE_URL=postgres://u:pw@localhost:5432/app
+DATABASE_URL=postgres://u:pw@localhost:5442/app
```

- `wtm run init` offers the keys of your configured `.env` targets whose value holds a declared base; `--link-env` writes them without asking. Nothing is linked otherwise.
- The rewrite happens when an **isolated** worktree is created (never a verbatim one) and whenever `wtm env` reconciles; its isolation step can switch the worktree to verbatim instead, and `--check` counts a pending shift as drift.
- `wtm env --mode refresh` compares linked values **modulo the offset**: `5442` in a worktree against `5432` in `main` is not a conflict.
- wtm reports rather than guesses when the key is missing, the base appears more than once, or neither the base nor any offset of it is there.

A value that is not a port, such as which database a worktree holds in a [shared service](shared-services.md), is written whole by an `[[env]]` link from a template over `{namespace}`, `{port.NAME}`, `{origin}`, `{worktree}` and `{ordinal}`:

```toml
[[env]]
file  = "apps/api/.env"
key   = "DATABASE_URL"
job   = "postgres"
value = "postgresql://app:app@localhost:{port.POSTGRES_PORT}/{namespace}"
```

A key is written by an `[[env]]` link or an `[[env_port]]` link, never both.

## Values that carry an address, not a port

When the browser is on the worktree's **name**, it sends a named `Origin`, and a `CORS_ORIGIN` holding `http://localhost:5183` blocks it. So under `addressing = "names"` (the default), a link writes the job's **whole origin**:

```diff
-VITE_API_URL=http://localhost:4001
+VITE_API_URL=http://api-dev.feat-x.monorepo.localhost
-CORS_ORIGIN=http://localhost:5173
+CORS_ORIGIN=http://web-dev.feat-x.monorepo.localhost
 PORT=4011                                    # a bare number stays a number
 DATABASE_URL=postgres://u:pw@localhost:5442/app   # Postgres has no name, and never will
```

It does so only when the linked job **publishes a url** for that port (so Postgres is left alone: the proxy only speaks HTTP) **and** the value **has the shape of a URL** (so `PORT` stays a number). Without the port-80 redirection the origin carries the proxy's port (`…localhost:11080`), which changes nothing for CORS or cookies.

- `wtm run addressing ports|names` switches the mode, and the main checkout keeps ports unless you run `wtm env main --addressing names`: see [Addressing](addressing.md#addressing-what-a-env-value-holds).
- Under `names`, the named URL is the only working entrance of a worktree: `localhost:5183` sends an `Origin` the API no longer knows. `wtm run url` and `wtm run open` hand out the right link.
- wtm only sees keys declared as links: a `CORS_ORIGIN` linked to nothing is invisible to both the pass and the warning, so silence means "nothing linked is out of step". A `.env` holding named origins whose port went stale (after `wtm run proxy install`) keeps its names and is told they are out of step.

## The daemon

Jobs run under one background daemon per machine; the run view and `-d` are described in [Jobs, profiles and runners](jobs-and-profiles.md#the-run-view-or--d). The daemon is disposable: it exits ~30 s after the last **foreground** job, while detached services (those with a `stop`, typically `docker compose up -d`) keep running without it. Its index of what it started, `jobs.json` beside the [global config](configuration.md#global-config), is read back by the next daemon, so `wtm run ps` still lists your stacks after a reboot (as `detached`: wtm has not seen them since, see [`run ps` statuses](jobs-and-profiles.md#run-ps-statuses)) and `wtm run down` still stops them. `wtm run daemon status` reports what is up; `stop` / `restart` get rid of the process.

A daemon started by an older binary is refused rather than silently used: any `run` command names both versions and points at `wtm run daemon restart`, which hands the jobs over (detached services survive, foreground ones are stopped).

> **Upgrading from 0.27:** jobs used to run without `COMPOSE_PROJECT_NAME`, so stacks started back then are under a directory-named project a new `run up` will not find. Stop them once with `docker compose -p <old-name> down`. See [Migrating to 0.28](migrating-to-0.28.md).
