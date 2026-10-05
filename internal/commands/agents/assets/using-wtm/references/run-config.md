# Configuring dev jobs: `run.toml`

How a repository declares its dev jobs and how wtm isolates them per worktree. To start, stop or inspect jobs, read `run.md`. Output shapes are in `json.md`.

## Contents

- [The model](#the-model)
- [`run init`](#run-init)
- [`run job add` / `edit` / `rm`](#run-job-add--edit--rm)
- [`run profile add` / `edit` / `rm`](#run-profile-add--edit--rm)
- [Commands: `cmd` and `stop`](#commands-cmd-and-stop)
- [Ports](#ports)
- [Publishing a URL](#publishing-a-url)
- [Runners and portless services: `runs`, `binds_no_port`](#runners-and-portless-services-runs-binds_no_port)
- [Isolation: `isolated` and `verbatim`](#isolation-isolated-and-verbatim)
- [The worktree's identity in a job's environment](#the-worktrees-identity-in-a-jobs-environment)
- [Shared services and namespaces](#shared-services-and-namespaces)
- [`touches`: jobs that change data](#touches-jobs-that-change-data)
- [`.env` links: `[[env_port]]` and `[[env]]`](#env-links-env_port-and-env)
- [Addressing: names or ports](#addressing-names-or-ports)
- [`run export` / `run import`](#run-export--run-import)

## The model

- Jobs live in a per-clone `run.toml`, **managed by wtm**: change it through `run init`, `run job`, `run profile` and `run addressing`, never by hand. Every field a job declares has a flag on `run job add` / `run job edit` (only `probe` has none), and a write is refused exactly as loading the file would refuse it.
- A job is a `service` (long-running) or a `task` (one-shot; it blocks its profile until it exits, and a non-zero exit aborts the profile). Profiles are named, ordered job groups; `run up` starts one.
- The module is **opt-in**: `wtm init` does not configure it. Until a job or profile is declared, every run command exits `16` except `run init`, `run import`, `run job add` and `run profile add` (which create the first declaration), plus `run ps`, `run daemon …` and `run proxy …`, which never read `run.toml`. A `run.toml` that exists but cannot be read is a different error, not `16`.
- Project-wide keys at the top of `run.toml`: `isolation` (default for unattended creations), `concurrency` (`parallel` / `exclusive`, see `run.md`), `addressing` (`names` / `ports`), `port_offset_block` (default 10), `port_probe_timeout` (default 15s, negative disables).

## `run init`

`wtm run init --yes` sets up `run.toml` from detection (docker-compose files and package scripts). Without a TTY it auto-generates and **removes nothing**. `--non-interactive` is gone: use `--yes`. It writes `run.toml` and, with the flags below, may rewrite compose files and `.env`.

**What it composes.** It proposes every compose file and package script but checks only scripts whose name contains `dev`, and not a root `dev` that a workspace package also declares (an orchestrator like `turbo run dev` that would double-start those packages). **Nothing unchecked becomes a job.** Then:

- **Ports** are reviewed (detection below).
- **Profiles**: one per package plus one gathering everything, editable (rename, merge, remove, new). Root-cwd jobs join every profile, tasks are ordered ahead of the services that depend on them, and past six packages only the `all` profile is proposed. Two packages whose directories end in the same name (`apps/a/back`, `apps/b/back`) get distinct profile names instead of being merged.
- **URLs**: every service declaring the port it listens on (`PORT`, or `<JOB>_PORT`) is proposed published, and unchecking one withdraws the `url` it had. A port a job merely dials (`DB_PORT`, `REDIS_PORT`) is never proposed. A non-interactive run publishes the same set without asking.
- **Kinds**: a checked script outside the `dev` ones gets its `kind` asked, because a task blocks its profile.
- **A service with no detected port is asked about too** (declaring one is what keeps a second worktree from binding the same port), and a job whose command never mentions the variable wtm injects gets that command offered for editing.
- **Data tasks** (`touches`, see below) and **`[[env]]`** keys (it asks for the namespace fields; wtm proposes only the name and never a command) and **addressing** (whenever a job publishes a url).
- It ends on a **review step**: "No, cancel" aborts and writes nothing.

Non-interactively it takes the same answers without asking.

**Re-running it is symmetric.** Every step is pre-filled from the existing `run.toml`: what stays checked is kept, and what you uncheck is **removed**, along with the profile entries and `[[env_port]]` / `[[env]]` links naming it; a profile left with no job goes too. Only jobs the wizard itself proposed can be removed: one added with `run job add` appears in no detected list and is never touched. The URLs step (a job you unpublish stays unpublished) and the profiles step (deleting them all keeps them deleted) behave the same. **Only interactive runs remove: `--yes` never removes.** A re-run also backfills the ports of a compose job that predates them, without overwriting a declared port.

**Compose ports.** A mapping already reading a variable (`"${DB_PORT:-5432}:5432"`) is declared as is. A literal one (`"5432:5432"`) binds the same port in every worktree, so it is **not** declared: wtm reports it with the line to write and the `run job edit --port` that follows.

**Compose absolute names.** A service's `container_name`, or a top-level volume's or network's explicit `name`, is resolved by the Docker daemon, not by the compose project, so `COMPOSE_PROJECT_NAME` never reaches it and a second worktree collides (Docker refuses a duplicate `container_name`; a pinned volume or network is silently shared). wtm reports them. A name under `external: true` or already reading a variable is left alone.

**`--patch-compose`** rewrites both: literal port mappings get a variable (only the port value is edited in place, comments and formatting survive, and the `:-default` keeps `docker compose up` working without wtm), and pinned names are fronted with the project (`container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"`, the default reproducing the old name). Ports and names are **one confirmation**: accepting half still leaves two worktrees unable to run at once. A volume that pinned its `name` gains one per worktree, **each starting empty** (the data stays under the old name): **say so to the user before patching**. Non-interactively no project file is ever touched without this flag.

**Dev server ports from `.env`.** For `kind = "service"` jobs from package scripts, `run init` reads a `PORT` (or `*_PORT`) entry with a numeric value from the env files next to their `package.json` (`.env.local`, else `.env`, else a committed `.env.example`), matching a job to a directory by its `cwd`, so in a pnpm monorepo each package takes its own port and never inherits the root's. It declares the port and prints "Ports detected from .env" naming the source file; nothing is written to the `.env` without `--write-port-keys` and no command is ever rewritten. **Whether the command actually reads the variable is not checked**: `next dev` and most node servers read `PORT`, but a CLI that only takes a flag (vite) needs `--cmd 'pnpm dev --port ${PORT}'`. Never assume a declared port means an isolated one. The detected port is the base the `[[env_port]]` links then follow, so a `.env` holding `PORT=5173` and `VITE_API_URL=http://localhost:5173/api` ends up with **both** keys shifted per worktree.

**`--write-port-keys`** materializes a declared port as a `.env` key: for every job whose port no `.env` carries, it writes `KEY=<base>` into the `.env` of the job's own `cwd` **and** into that file's committed template, adds the `[[env_port]]` link, and, when the project does not provision that file, adds the `[env]` target to `config.toml`. This isolates a job **both** under `wtm run` and when a developer starts it themselves, provided the project's config reads the key (`server.port: Number(process.env.VITE_PORT)`); wtm never touches project code. It writes tracked files, so it is never inferred: non-interactively it takes the flag; interactively it is the wizard's route step, which asks each service declaring a port **where it reads it** (its `.env`, pre-filled, or its command) and offers command editing only to the jobs left on the command route.

**`--link-env`** writes the `[[env_port]]` links without asking (see below).

**What it refuses to declare, and reports instead**: port ranges, mappings with no host port, a `ports:` list carrying a YAML anchor or alias, `${VAR}` with no default, a variable two services give two different defaults. It also **withdraws** a detected port when two bases differ by a multiple of the block (it would make `run.toml` unloadable), naming both sides. Compose and `.env` ports are arbitrated together, so either can be the one withdrawn; a base already written by hand always outranks a detected one. A "Ports withdrawn" or "Ports left alone" section is expected behaviour, not a failure: read the fix it prints (a compose line, then a `run job edit --port` command).

## `run job add` / `edit` / `rm`

Fully drivable with flags, and they take `--yes` like every mutating command. A flag pre-fills its question; **`--yes` skips the questions**. Without a TTY they are already unattended, but pass `--yes` anyway: it is the one axis true on every surface. Drive them as `wtm run job add web --cmd '…' --yes --output json`. A name that is not a declared job exits `14`.

**`run job add <name> --cmd '<cmd>'`** (the name argument and `--cmd` are required under `--yes`). Declaration flags, shared with `edit`:

| Flag | Field |
|---|---|
| `--cmd` | `cmd` (a `/bin/sh` line) |
| `--stop` | `stop` (services only; makes a service `detached`) |
| `--kind service|task` | `kind` (default `service`) |
| `--cwd` | `cwd` (relative to the project root) |
| `--port NAME=PORT` (repeatable) | `[job.ports]` |
| `--url-port NAME`, `--url-host <host>` | `url = { port, host }` |
| `--runs <job>` (repeatable) | `runs` |
| `--touches <service>` (repeatable) | `touches` |
| `--binds-no-port` | `binds_no_port = true` |
| `--scope shared|worktree` | `scope` |
| `--namespace-name`, `--namespace-create`, `--namespace-remove`, `--namespace-env KEY=VALUE` (repeatable) | `[job.namespace]` |

**`run job edit <name>` patches**: a flag left out keeps that field, so `run job edit api --cmd '…'` changes the command alone and leaves kind, stop, cwd, ports and url intact (the job also keeps its position in the file).

- An explicit empty string clears: `--stop ''` drops the stop command, `--cwd ''` falls back to the project root, `--url-port ''` withdraws the published name, `--url-host ''` falls back to the job's name. `--binds-no-port=false` withdraws that flag.
- `--port NAME=PORT` **merges** into the declared ports (one entry changes without rewriting the others); `--port-clear` empties the table.
- `--runs`, `--touches`, `--binds-no-port`, `--scope` and the `--namespace-*` flags patch the same way: a list flag replaces the list, `''` drops it.
- `--name` renames and rewrites what names the job elsewhere in the file (profiles, `runs`, `touches`, `[[env_port]]` and `[[env]]` links). It refuses to rename a job worktrees hold data in, and warns when the rename moves a published address (a job with no `url.host`).
- With no flag it opens the form, so **always pass at least one flag**; under `--yes` or without a TTY it errors naming the flags it could have taken. A missing job argument errors rather than opening a picker. The form asks every field pre-filled; only `probe` has neither a flag nor a question, and is kept.

**`run job rm <name>`** refuses a job something still names, listing each kind of reference and each worktree. **`--force`** (only when the user asked) takes it out along with every reference naming it (the profiles that start it, the `runs` of any job that starts it, `touches`, `[[env_port]]` and `[[env]]` links), and even while a worktree still holds data in it (a shared service's namespace: `clean` will then no longer drop that data). Each removed reference is reported on its own line so you can tell the user what went with it.

## `run profile add` / `edit` / `rm`

- `run profile add <name> --jobs <job,…> --yes` (the name and `--jobs` required under `--yes`).
- `run profile edit <name>` patches the same way as `run job edit`: `--name` renames, `--jobs` replaces the list (its order is the start order, so give it in full), `--default` / `--default=false` hands the default over or takes it away. A flag left out keeps the field, no flag opens the form, and `--yes` or no TTY means an error rather than a picker.
- Taking the default from another profile (`add`/`edit --default`) or removing the default profile prints a `!` warning on stderr naming what `run up` now starts: the only profile left, or nothing (with several left and none default, `run up` needs `--profile`).
- `run job list` / `run profile list` list them (exit 0 even on a backed-out prompt).

## Commands: `cmd` and `stop`

- **`cmd` and `stop` are `/bin/sh` lines**, not whitespace-split argv: quotes, `&&`, pipes, redirections and globs work, and `${VAR}` expands from the job's environment. POSIX `sh` is always used, never the user's interactive shell: no `[[ ]]`, no process substitution. A `cmd` the shell cannot parse is refused when the job is written, naming the job.
- To isolate a server that ignores `PORT` and only takes a flag (vite), pass the declared port back on the command line, quoting for **your** shell so wtm receives the variable verbatim: `wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000`.
- **Declaring `stop` is what makes a service `detached`**: its `cmd` must exit once the work is started (e.g. `docker compose up -d`), and `run up` waits for it. A `cmd` that keeps running with a `stop` beside it blocks the run; `run up` / `run start` warn when such a `cmd` has no `-d` / `--detach`. Drop `stop` to run it in the foreground. The job's ports are given to `stop` too.

## Ports

**Port isolation is declarative.** A job declares the ports it binds on the main checkout, and wtm injects `base + WTM_PORT_OFFSET` under that name, so the command needs no arithmetic: `wtm run job add web --cmd "pnpm dev" --port PORT=3000` (repeat `--port` per variable). The main checkout gets `PORT=3000`, the next worktree `PORT=3010`.

- For Docker, template the host side in `docker-compose.yml` (`"${DB_PORT}:5432"`) and declare `--port DB_PORT=5432`: the container port never moves, only the binding. `run init --patch-compose` does both steps.
- A declaration **overrides** any inherited value for that variable.
- The `on_create` / `on_clean` hooks receive the ports too (so an `on_clean = "docker compose down"` reads the same `${DB_PORT}` the job bound), but only the names a **single** job declares: a variable two jobs declare on different bases has no answer outside a job and stays unset in a hook.
- **Two base ports must not differ by a multiple of the block**, or two worktrees land on the same port: `3000` and `3010` are refused when `run.toml` is read (both sides named), while `5434`/`5435`/`5436` are fine. Raise `port_offset_block` if a project genuinely needs more room.
- A job that binds a port without declaring it still collides across worktrees. Docker isolation is automatic for everything compose names itself; a `container_name` or a volume/network pinned by `name` escapes it (see `--patch-compose`).
- `probe = false` on a job silences the post-start port check for it (see `run.md`).

## Publishing a URL

- `url = { port = "PORT" }` on a job says which of its declared ports speaks HTTP (`--url-port PORT`). `host` (`--url-host api.app-1`) overrides the segment it is published under (default: the job's name); lowercase letters, digits and dashes, dot-separated. Two jobs claiming the same host makes `run.toml` refuse to load, naming both.
- `run init` proposes this for every service that declares the port it listens on.
- A job with no `url` keeps no name and stays reachable by its port: the right answer for anything that does not speak HTTP (postgres, redis).
- Where the named URL answers, and the proxy that serves it, are in `run.md`.

## Runners and portless services: `runs`, `binds_no_port`

- `binds_no_port = true` (`--binds-no-port`): this service listens on nothing by design (a build in watch mode, a worker, a runner whose children hold the ports). Without it wtm keeps naming the job under "These jobs will bind the same port in every worktree". A job that runs others is not itself reported as portless.
- `runs = ["web-dev", "api-dev"]` (`--runs`, repeatable, replaces the list): the declared jobs this one starts itself, typically a root `turbo run dev` behind a filter. **wtm infers nothing from the command**: the relation is written, never detected. It nests and fans out (a runner may name another runner, and two runners may name the same job); the only refusal is a cycle.
- What it does: the runner gets its children's ports at start time (a variable two children declare differently is left unresolved), the port check looks at those ports, `run up` refuses a profile asking for a runner and one of its own children, and starting the runner registers one proxy route per published job it runs. Those addresses are reported on the runner as `held: [{job, url}]`, never on the children; a child of a running runner has no row of its own in `wtm ui`.

## Isolation: `isolated` and `verbatim`

Decided **once per worktree, at creation**. `create`, `extract` and `checkout` ask it whenever `run.toml` declares something to isolate (a port, a namespace, a `.env` link, a compose stack), take `--isolation isolated|verbatim` on your paths, and record the answer in the worktree's `meta.json`.

- **`isolated`** (default): wtm writes the worktree's own ports, `COMPOSE_PROJECT_NAME` and `[[env]]` namespaces into its `.env`, and the daemon runs its jobs on the same shifted ports and carves its namespaces.
- **`verbatim`**: the `.env` stays **byte for byte** as copied (no port, no identity, no `[[env]]` value), and the daemon runs the worktree as that file describes it: offset 0 (its source's ports), no namespace carved, no `COMPOSE_PROJECT_NAME` imposed. The worktree **cannot run while its source does** (a port clash, see `run.md`), and a job whose `touches` reach its source's data is refused.
- The two halves (the `.env` and the jobs) never disagree: a `.env` on the source's ports with jobs on shifted ones would wire the worktree to its source silently.
- `isolation = "isolated" | "verbatim"` in `run.toml` sets the default for unattended runs. The main checkout is always isolated.
- `wtm env <wt> --yes --isolation isolated|verbatim` switches an existing worktree, and adopts one created before the choice existed (ask the user first: an adopted worktree runs under a new compose project, so its current volumes stop being used). Details in `worktrees.md` (`env`).

## The worktree's identity in a job's environment

Every job runs with these, so parallel worktrees do not fight over resources:

- `WTM_BRANCH`: the branch verbatim.
- `WTM_WORKTREE`: its slug, safe as a Docker project or network name.
- `WTM_ORDINAL`: `0` for the main checkout, then the smallest free number, stable for the worktree's life.
- `WTM_PORT_OFFSET`: `WTM_ORDINAL` times `port_offset_block` (10 by default); **0 for a verbatim worktree**.
- `WTM_ISOLATION`: `isolated` or `verbatim`.
- `COMPOSE_PROJECT_NAME`: `<repo>-<WTM_WORKTREE>`, derived from the target worktree (the caller's environment is never read), and **not set at all for a verbatim worktree**, whose copied `.env` or directory name decides.

**The main checkout's name never follows its branch**: it is the `COMPOSE_PROJECT_NAME` of main's own `.env` (in the directory its compose jobs run from), else the repository's slug alone; the shell's environment is ignored there. That is where shared services run, so switching main's branch keeps the same stack.

`on_create` / `on_clean` hooks get the same identity **only** when `run.toml` declares a `docker compose` job **and** the worktree recorded its isolation in `meta.json`; otherwise a hook's environment is untouched, and a `COMPOSE_PROJECT_NAME` set in the worktree's own `.env` wins for hooks.

On an isolated worktree, `COMPOSE_PROJECT_NAME` is **also written into the `.env` of the directory each compose job runs from**, at `create` and at `wtm env`, because compose interpolates that file: a `docker compose up` typed by hand in a worktree gets its own project, containers and volumes. It is a wtm-owned key, never reported as drift or conflict. When a run writes it, the report names it.

## Shared services and namespaces

**`scope = "shared"`** (`--scope shared`; `--scope worktree` puts it back) makes a job run once for the whole repository, in the main checkout (a postgres, a keycloak). It takes no port offset and its URL has no worktree segment; how it behaves at run time (`joined`, `released`) is in `run.md`. A shared job with **no** `[job.namespace]` is valid: one instance, one set of data (and a job touching it is refused, see `touches`).

**`[job.namespace]`** lets a shared job carve out a namespace per worktree (a database, a set of keycloak realms) so each worktree keeps its own data. Fields: `name`, `create`, `remove`, `env`.

- wtm runs the declared commands and knows nothing else about them. They get the worktree's whole environment plus `$WTM_NAMESPACE`, `$WTM_WORKTREE`, `$WTM_ORDINAL`. Configuration values use `{worktree}` / `{ordinal}`; commands use the `$WTM_*` variables.
- **`create` runs on every start of the shared service**, so it must be safe to run again: wtm keeps no record of having run it, and a `create` that fails when the namespace already exists fails the run.
- Flags: `--namespace-name 'app_{worktree}' --namespace-create '<cmd>'` (both required together), optionally `--namespace-remove '<cmd>'` and `--namespace-env KEY=VALUE` (repeatable, replaces the table). On `edit`, `--namespace-name ''` withdraws the whole block and `''` drops any other field.
- A namespace on a job that is not shared is refused, as the loader refuses it. Unsharing one is `--scope worktree --namespace-name ''`.
- wtm proposes only the name and never a command: `create`/`remove` are always the project's own, inline or a script path.
- When a user finds isolation expensive (an empty database to migrate and seed, a realm to rebuild), suggest a `create` that **clones** the data main uses, e.g. `CREATE DATABASE "$WTM_NAMESPACE" TEMPLATE app`, guarded by an existence check since `create` runs at every start. Do not suggest sharing main's database: one branch's migration would break the other.
- Removing a worktree drops its namespaces (see `worktrees.md`, `clean` and `prune`).

## `touches`: jobs that change data

`touches = ["postgres-pay"]` marks a job that changes data (a migration, a reset, a seed) and names the services it writes to. `run up` / `run start` then refuse to start it on data the worktree does not own (see `run.md`, Foreign data); a job without `touches` is never checked.

- `run init` asks it in its "Data tasks" step: one row per task, cycling through the shared and compose services, pre-set when the task's name carries a data verb (`reset`, `migrate`, `seed`, `init`, `orm`…) and shares a word with exactly one service (`orm:pay:reset` → `postgres-pay`). A task `run.toml` already gives touches keeps them.
- Outside the wizard: `run job add <job> --touches <service>` or `run job edit <job> --touches <service>` (repeatable, replaces the list, `''` drops it). A name that is not a declared job is refused.
- **Whenever you add a job that migrates, resets or seeds data, pass `--touches`**: nothing sets it unasked, and without it the job escapes the check.

## `.env` links: `[[env_port]]` and `[[env]]`

**`[[env_port]]` makes a port hard-coded in a `.env` follow the worktree.** A link names a key, not a position: `{file = ".env", key = "DATABASE_URL", job = "db", port = "POSTGRES_PORT"}` tells wtm that this key's value carries that job's port. wtm finds the declared base *inside* the value and shifts it, so `postgres://u:pw@localhost:5432/app` becomes `…:5442/app` while credentials, path and query are untouched. A bare `DB_PORT=5432` is the same mechanism.

- `run init` scans the configured `.env` targets, offers the keys whose value holds a declared base, and writes the confirmed links; `--link-env` writes them without asking. Nothing is ever inferred without one or the other.
- The rewrite happens at `create`, `extract` and `checkout` for an **isolated** worktree (never a verbatim one), and at `wtm env` (see `worktrees.md`).
- A key may follow **several** ports: `CORS_ORIGIN=http://localhost:5173,http://localhost:5174` gets one link per front-end, and every origin moves onto the port its own job binds. A port no job declares (an external origin) is left exactly as it was.
- `job` is **required**: two apps may each declare a `PORT`, and the name alone would not say which base the key follows. The error names the jobs that declare the port, so the fix is the line to write.
- Refused when `run.toml` is read: a link naming a port no job declares, an invalid key, the same `(file, key, job, port)` twice.
- **A link (`[[env_port]]` or `[[env]]`) on a `.env` that is not a configured `[env]` target of `config.toml` is ignored, never fatal**: `create`, `checkout` and `wtm env` still settle every other link and name the ignored one in `warnings` ("env_port KEY in FILE ignored: not a configured env file…"). Fix it by adding the file to `[env]` or deleting the link.
- Three refusals at rewrite time, reported and never guessed: the key is absent from the file, the base appears **more than once** in the value, or **neither the base nor any offset of it** is there. One key linked to two different ports is allowed on purpose, so a key linked by mistake to another job's port is not refused at load: it shows up at every create as the third refusal. Tell the user to delete that line.
- `wtm env --mode refresh` compares linked values **modulo the offset**: `5442` against a source holding `5432` is not a conflict, but a genuine difference in the same value still is.

**A linked value that is a URL gets the job's whole address, not its port**, when the job it names publishes a `url` for that very port: `VITE_API_URL=http://localhost:4001` becomes `http://api-dev.feat-x.monorepo.localhost` (or `…localhost:11080` without the port-80 redirection). The browser sends a name as its `Origin`, so a `CORS_ORIGIN` holding a port would block every cross-origin call. Both conditions are load-bearing: a bare `PORT=4011` stays a number even though its job publishes a name, and a `DATABASE_URL` stays on a port because Postgres has no name. `wtm env --mode refresh` reads a port, another worktree's address and this worktree's address as **one setting**, never a conflict against each other. Two refusals, reported: an `https` value (the proxy serves plain HTTP) and a URL pointing at a host no job here serves.

**`[[env]]` writes a key's whole value from a template**: the only way to express something opaque like a realm or a database name, and how a namespace reaches the app. Fields `file`, `key`, `job`, `value`, where `value` draws on `{namespace}`, `{port.NAME}`, `{origin}`, `{worktree}`, `{ordinal}` and nothing else. A shared service has one address for every worktree, so its URL stays an `[[env_port]]` while its realm becomes an `[[env]]`.

- Refused when `run.toml` is read: a key written by both tables, a placeholder outside the list.
- wtm owns an `[[env]]` key's line: a worktree's own value there is replaced, and `wtm env` never reports it as drift.
- `run init`'s `[[env]]` step lists every managed `.env` key and pre-checks those named after a shared service; a key whose value carries that service's port is left to `[[env_port]]`, and marking a key the port table already writes moves it rather than declaring it twice.

## Addressing: names or ports

`addressing` at the top of `run.toml` decides what a `.env` value pointing at another job holds: `"names"` (default when absent) or `"ports"`. `run init` asks it whenever a job publishes a url.

- It is the one setting with a consequence outside wtm: **named URLs are served by the run proxy, which lives in the run daemon**, so `VITE_API_URL=http://api.feat-x.repo.localhost:11080` answers while `wtm run` runs that job and **not** when the developer starts it themselves. A project whose author launches dev servers by hand wants `"ports"`.
- Only values of jobs that publish a url are affected: a `DATABASE_URL` or a bare `*_PORT` stays a port either way.
- It also decides what the run surfaces announce: under `"ports"`, `run up`, `run start`, `run ps`, `run url`, `run open` and the run view hand out `http://localhost:<port>` and register no name with the proxy (a name no `.env` knows of would fail CORS).
- When the proxy is disabled on the machine (`[proxy] enabled = false` in the global config), wtm writes ports whatever the mode says, and reports it in one notice.

**Switch it with `wtm run addressing <names|ports>`**, never by editing `run.toml`: the command writes the setting, then settles the worktrees whose `.env` spells the other one. Under `--yes` the mode argument is required and the worktrees are settled unless `--keep-env`. Running it with the mode already in place settles what an earlier `--keep-env` left behind. Setting `ports` is a real inverse: port numbers go back into values wtm wrote as addresses. JSON: see `json.md`.

**The main checkout is never provisioned, so under `names` its `.env` still holds ports.** `run addressing` settles main back to `ports`, never onto `names` (the output says main was left as is): moving main onto names makes it depend on the proxy, and only `wtm env main`, naming it, does that. Its jobs are still published under names, so a cross-origin call made through them is refused until `wtm env main` aligns it. Aligning main is a **choice**: it stops behaving as a checkout without wtm, and going back means `wtm run addressing ports` (which brings main back with every worktree) then `wtm run addressing names` (which leaves main on ports), `addressing` having no per-worktree scope. The same applies to a linked worktree whose port pass was declined.

Every surface hands out the named URL whatever the `.env` spells; a worktree whose `.env` is out of step gets one warning line naming `wtm env <worktree>` (a `!` line in the stream, a band in the run view, a note in `wtm ui`). The route is registered either way, so nothing restarts. Only keys declared as `[[env_port]]` links are seen, so silence means nothing **linked** is out of step. A `.env` already on names whose port went stale keeps its names and is told they are out of step.

## `run export` / `run import`

- `wtm run export` always writes the layout as a JSON document (and accepts `--output json`). `--profile <name>` exports only that profile and its jobs; a second `--profile` is a usage error.
- **`wtm run import` replaces the whole `run.toml`**: jobs, profiles, `[[env_port]]` links and project settings alike, so what the file held is lost. **It always needs `--yes` on your paths**: without a terminal to confirm on (a piped payload included) it refuses rather than replacing silently, and `--output json` requires `--yes` too. Nothing is reconciled afterwards: tell the user to run `wtm env` if the `.env` values must follow.
