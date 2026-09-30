# How `wtm run` works

Optional and opt-in: created by `wtm run init` (which detects docker-compose files
and package scripts), not by the global `wtm init`. Declares dev **jobs** and groups
them into **profiles**. Per-clone, never committed; share layouts with
`wtm run export | wtm run import -`.

A job's `cmd` (and its `stop`) is a **`/bin/sh` line**, not a whitespace-split argv:
quotes, `&&`, pipes, redirections and globs behave as they do in a terminal, and `${VAR}`
expands from the job's environment. POSIX `sh` is used on every machine, never your own
interactive shell, so a shared `run.toml` behaves the same everywhere.

Two worktrees can run their stacks side by side (each has its own ports and resource
names), and `wtm run up feat-a feat-b` brings up as many as you name at once, each
independent of the others. The first time `wtm run up` finds another worktree's jobs
running it asks what to do about the machine's load, and can write the answer as
`concurrency = "parallel" | "exclusive"` at the top of the file so it never asks again.
`--parallel` and `--exclusive` override it for a single run; `--exclusive` is refused on
several worktrees, since it stops all but one. `wtm run start` asks the same question and
takes the same two flags. A worktree that shares its ports with one
already running (a verbatim worktree and its source) is not a question of load: `run up`
and `run start` offer to stop the other one or not to start, and refuse under `--yes` unless
`--exclusive` was given.

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
worktree's jobs unless you pass `--all`, which stops every worktree of this repository, never another one.

Every job (and every `on_create` / `on_clean` hook, under the condition given in
[Project config](configuration.md#project-config-configtoml)) also runs with the worktree's own identity in its environment, so two worktrees running the same services never share a
resource:

| Variable | Value |
| --- | --- |
| `WTM_BRANCH` | the branch, verbatim |
| `WTM_WORKTREE` | the branch as a slug safe for a Docker project, network or volume name |
| `WTM_ORDINAL` | the worktree's stable number. The main checkout is always `0`; every other worktree gets the smallest number free, kept for its whole life and released when it is cleaned |
| `WTM_PORT_OFFSET` | `WTM_ORDINAL` × the block (`port_offset_block`, 10 by default); the main checkout keeps the project's default ports, and so does a verbatim worktree |
| `WTM_ISOLATION` | `isolated` or `verbatim`, as chosen when the worktree was created |
| `COMPOSE_PROJECT_NAME` | `<repo>-<WTM_WORKTREE>`, derived from the worktree itself, never from the shell the command is typed in, which may belong to another worktree. The Docker daemon is machine-wide, so the repository qualifies the name: two clones both sitting on `main` do not share a stack. Not set for a verbatim worktree: its copied `.env` decides. The main checkout is named without its branch (its own `.env`'s value, else `<repo>`), so the shared services it hosts stay one stack whatever it has checked out |

`COMPOSE_PROJECT_NAME` is what keeps two worktrees' containers, networks and volumes
apart. Nothing to declare: it works as soon as your jobs use `docker compose`. It reaches
everything compose names for you, and nothing your file names itself: a `container_name`,
or a volume's or network's explicit `name`, is resolved by the Docker daemon directly, so
the second worktree to start meets the first one's. `wtm run init` finds those and offers
to front them with the project; see [absolute names](#absolute-names) below.

**Ports** are declared per job, and wtm injects `base + WTM_PORT_OFFSET` under the name
you chose; the command itself needs no arithmetic:

```console
$ wtm run job add web --cmd "pnpm dev" --port PORT=3000
$ wtm run up
✓ web started · PORT=3010
```

**wtm checks that the port was actually bound.** Declaring a port only injects a
variable: nothing guarantees the command reads it. Once the jobs are up, `run up` dials
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
environment. **Turborepo does this by default** (`envMode: "strict"`), so a root
`turbo run dev` job needs `globalPassThroughEnv` in `turbo.json` for the ports to reach
its packages. wtm never edits those files; it tells you what it observed.

It never fails the run, and a healthy stack costs nothing: the check stops as soon as
every port answers. `--no-probe` skips it, and `port_probe_timeout` in run.toml sets the
budget (default 15s, a negative value turns it off). `probe = false` on a job skips it for
that job, and `binds_no_port = true` marks a service that listens on nothing by design (a
watcher, a worker), so it is no longer offered a port.

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
contains `dev`, and not a root `dev` a workspace package also declares. That one is an
orchestrator (`turbo run dev`, `pnpm -r dev`) and running it beside the packages it fans
out to would start each of them twice on the same ports. Nothing unchecked is written.

It asks which of them starts the others: the relation is declared, never inferred from a
command. A root can name another root, so `dev` → `dev:shop` → the shop apps is written one
row at a time and what the top one holds is read through the whole chain; two roots may
also name the same app. Only a cycle is refused.

It also asks which jobs should answer under their own name, and proposes every service that
declares the port it listens on: `PORT`, or `<JOB>_PORT` for the ones after the first. A
port a job only dials (`DB_PORT`, `REDIS_PORT`) is never proposed: a name nothing answers
under is worse than no name at all. Unchecking a job withdraws the `url` it already had.

It then walks you through the ports detection pre-filled, and the **profiles** `wtm run
up` will offer: one per package, plus one gathering everything, which you rename, merge
or drop. Jobs at the repository root (a compose stack) join every profile, so starting
one package alone still brings its infrastructure up. Tasks are placed ahead of the
services that depend on them, so a profile brings the database up to date before starting
what reads it. In a single-package repo (or past a handful of packages, where a profile
each stops being something you can read), the split collapses to one profile.

A service detection found no port for is reported rather than asked about: inventing one
would move the guess onto you, and `wtm run up` will say the port was never bound anyway.

`wtm run init` writes those Docker declarations for you. It reads the `ports:` of the
compose files you pick: a mapping that already reads a variable is declared as-is, while a
literal `"5432:5432"` would bind the same port in every worktree and is therefore **not**
declared; wtm shows the line to write instead. Pass `--patch-compose` and it makes the
change itself:

```diff
   postgres:
     ports:
-      - "5432:5432"
+      - "${POSTGRES_PORT:-5432}:5432"
```

Only the port value is rewritten, at its exact position (comments, indentation and
quoting style are untouched), and the `:-5432` default keeps `docker compose up` working
on its own, with no dependency on wtm. Re-running `run init` backfills a compose job that
predates declarative ports without overwriting one you set by hand.

Dev servers are pre-filled the same way, from the env files next to their `package.json`:
a `PORT` or `*_PORT` entry in `.env.local`, `.env`, or a committed `.env.example`. Each
job takes the file in its own directory, so in a monorepo every package keeps its own port.
wtm declares the port and never rewrites a command. Where the job *reads* that port is a
question the wizard puts, job by job: from its own `.env` (wtm writes `KEY=<base>` there
and in the committed template, and your config reads it: `server.port:
Number(process.env.VITE_PORT)`), or from the command, `--cmd 'pnpm dev --port ${PORT}'`.
The `.env` route is the pre-filled answer because it is the only one that still holds when
you start the app yourself; the command route isolates what `wtm run` starts and nothing
else, which the final report says in as many words. `--write-port-keys` takes the first
route for every job without asking. The port it declares is also the base the `[[env_port]]`
links below follow, so a `.env` holding both `PORT=5173` and a `VITE_API_URL` pointing at
it ends up with the two shifted together.

<a id="absolute-names"></a>
**Absolute names.** A compose file that pins its own names bypasses the project prefix,
which is what makes a second worktree fail outright: Docker refuses a duplicate
`container_name`, and a volume or network pinned by `name` is silently shared instead.
`run init` reports them, and `--patch-compose` fronts each with the project:

```diff
   postgres:
-    container_name: myapp-postgres
+    container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"
```

The `:-myapp` default reproduces the name the file used to pin, so `docker compose up`
on its own is unchanged. Ports and names are one question, not two: accepting half of
them still leaves two worktrees unable to run at once. A `name` under `external: true`
is left alone (sharing it is the declaration's whole point), as is one that already reads
a variable, and a volume declared as a bare key was never affected: compose already
prefixes it with the project. **A volume that pinned its `name` gains one per worktree,
each starting empty**: the data already written stays under the old name, and moving it
across is yours to do.

wtm declares only what it can actually isolate, and says why for the rest: a port range,
a mapping with no host port, a `ports:` list carrying a YAML **anchor or alias** (rewriting
it would move every service sharing it), a `${DB_PORT}` with no default (the file never
says which port it stands for), and a variable two services declare with two different
defaults. It also withdraws a detected port rather than write a `run.toml` its own loader
would refuse (two bases a multiple of the block apart), naming both sides.

A server that **ignores** `PORT` and only takes its port as a CLI flag (Vite is the usual
one) reads it back from the same variable, because `cmd` is a shell line:

```console
$ wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000
```

## Ports hard-coded in a `.env`

Shifting a service's host port only helps if whatever connects to it follows. That is easy
when the consumer reads `${DB_PORT}`, but in most projects the port is not in a variable
of its own, it is **buried in a URL**: `DATABASE_URL=postgres://u:pw@localhost:5432/app`,
`API_URL=http://localhost:3000/api`. An app running on the host, outside Docker, then talks
to the wrong worktree.

An `[[env_port]]` link says which key carries which port:

```toml
[[env_port]]
file = ".env"
key  = "DATABASE_URL"
job  = "docker"             # required: the job declaring the port
port = "DB_PORT"            # a port that job declares
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
or the other. The rewrite then happens when an **isolated** worktree is created (never for
a verbatim one, whose `.env` is kept as copied) and whenever `wtm env` reconciles, whose
recap offers "Apply, and keep this worktree's .env verbatim from now on" beside the plain
apply, so neither command imposes the pass (`--check` reports without writing, and counts a
pending shift as drift). `wtm env --mode refresh` compares linked values **modulo the offset**, so a
worktree holding `5442` against a `main` holding `5432` is not a conflict; a real difference
in the same value still is.

wtm reports rather than guesses when it cannot be sure: the key is missing, the base appears
more than once in the value, or neither the base nor any offset of it is there. Rewriting on
a guess could corrupt a URL, so those lines are named and left alone.

A value that is not a port, such as which database or which realm a worktree holds in a [shared service](shared-services.md), is written whole by an `[[env]]` link, from a template over `{namespace}`, `{port.NAME}`, `{origin}`, `{worktree}` and `{ordinal}`:

```toml
[[env]]
file  = "apps/api/.env"
key   = "DATABASE_URL"
job   = "postgres"
value = "postgresql://app:app@localhost:{port.POSTGRES_PORT}/{namespace}"
```

A key is written by an `[[env]]` link or an `[[env_port]]` link, never both.

## Values that carry an address, not a port

A port in a `.env` is enough for one app talking to itself. It is not enough the moment a
front end calls a separate API: the browser is on the worktree's **name**, so the `Origin`
it sends is a name, and a `CORS_ORIGIN` holding `http://localhost:5183` blocks it. Ports and
named URLs cannot both be half-true in the same file.

So a link writes the job's **whole origin** rather than its port number, whenever two things
hold at once: the job it names **publishes a url** for that very port, and the value **has
the shape of a URL**:

```diff
-VITE_API_URL=http://localhost:4001
+VITE_API_URL=http://api-dev.feat-x.monorepo.localhost
-CORS_ORIGIN=http://localhost:5173
+CORS_ORIGIN=http://web-dev.feat-x.monorepo.localhost
 PORT=4011                                    # a bare number stays a number
 DATABASE_URL=postgres://u:pw@localhost:5442/app   # Postgres has no name, and never will
```

Both conditions matter. The first leaves Postgres alone: the proxy only speaks HTTP. The
second leaves the binding keys alone: `PORT` belongs to a job that *does* publish a name, and
must still be a number. Without the redirection installed the address carries the proxy's
port (`…localhost:11080`), which changes nothing for CORS and nothing for cookie isolation:
a port is part of an origin, but never part of a *cookie's* origin.

This is `addressing = "names"`, the default when `run.toml` does not say.
`wtm run addressing ports` keeps port numbers everywhere, and `wtm run addressing names` goes
back: it writes `addressing` in `run.toml`, then offers to settle the worktrees whose `.env`
spells the other one. It is a real inverse, and `--keep-env` switches the setting alone. The
main checkout follows the rule below: the switch brings it back to ports, and never moves it
onto names. On a machine where the proxy is off,
ports are written whatever the project asked for, and a notice says so. Under `names` the
named URL becomes the only working entrance: opening `localhost:5183` directly sends an
`Origin` the API no longer knows. `wtm run url` and `wtm run open` hand out the right link.

The rewrite happens where wtm provisions: a worktree, when it is created and whenever
`wtm env` reconciles it. **Nothing moves the main checkout's `.env` onto names unless you name it**: it is the one
checkout that exists without wtm, the one a colleague clones and a `docker compose up` reads.
So under `names` its values still hold ports, and then **the working entrance is the port**,
not the name: the browser on `localhost:5175` sends an `Origin` the API's `CORS_ORIGIN`
recognises, while the named URL sends one it does not. wtm still hands out the name everywhere
(`run up`, `run url`, `run open`, the run view, the `wtm ui` panel) and adds one line saying
the `.env` is out of step and which command aligns it (`--raw` gives the port URL). The route is
registered either way, so nothing has to restart:

```bash
wtm env main        # the positional takes the main checkout like any other worktree
```

wtm only ever sees the keys declared as `[[env_port]]` links: a `CORS_ORIGIN` nothing links
to a declared port is invisible to both the pass and the warning, so silence means "nothing
linked is out of step", not "everything is right". And a `.env` that already holds named
origins whose port went stale (what `wtm run proxy install` does to every worktree at once)
keeps its names and is told they are out of step, rather than being sent back to ports.

Doing it is a choice, not a formality. Main then stops behaving as a checkout without wtm:
whoever reads that `.env`, or starts the stack from it, depends on the proxy being up. Two
moments make it worth doing: right after switching a project to `names`, and after
`wtm run proxy install`, which drops the `:11080` from the origins already written. Going
back is `wtm run addressing ports` then `wtm run addressing names`: the first brings main back to
ports with every other worktree, the second moves the others onto names again and leaves main
where it is: a pass over every worktree may return main to ports, only `wtm env main` takes it
to names. And `wtm create` from main is unaffected either way: a copied
value carrying main's segment is recognised and rewound to the new worktree's.

Two base ports must not differ by a **multiple of the block**, or two worktrees end up on
the same one: `3000` and `3010` are refused when `run.toml` is read, naming both sides,
which is the last moment the problem is still explainable. Neighbouring ports are fine:
a uniform offset preserves the gaps, so `5434`/`5435`/`5436` become `5444`/`5445`/`5446`
on the next worktree. Set `port_offset_block` at the top of `run.toml` when a project's
ports genuinely need more room than 10.

> **Upgrading:** jobs used to run with no `COMPOSE_PROJECT_NAME`, so `docker compose`
> named the project after the working directory. Stacks started before this version are
> under the old name and a new `run up` will not find them; stop them once with
> `docker compose -p <old-name> down`.
>
> The run daemon is global and outlives the command that started it, so a daemon started
> by an older binary would keep serving its own behavior. It is now refused rather than
> silently used: any `run` command names both versions and points at
> `wtm run daemon restart`, which hands the jobs over. Detached services survive that
> restart; foreground ones are stopped.

`run up` and `run start` **attach**: a full-screen view opens with one pane per job, and
`wtm run logs` reopens it later. Leaving the view (`q`, or Ctrl+C outside focus mode)
detaches: the daemon keeps the jobs running. `-d` starts them and hands the prompt back
instead. Unless both stdin and stdout are a terminal (`wtm run up > run.log` included), or under `--output json`, no view opens: the run reports
itself as lines, which is what a script or an agent gets. Each job's output is also
journaled to `<git-common-dir>/wtm/logs/<url-escaped-branch>/<url-escaped-job>.log`
(5 MB x 3 within one run), and `run logs` reads that back for a job that is no longer
running. Starting a job clears its log first, so one file is one run: what `run logs`
replays never reaches back into an earlier one.

The daemon itself is disposable. It exits ~30 s after the last **foreground** job, while
detached services (those with a `stop` command, a `docker compose up -d` typically)
keep running without it: the real work belongs to Docker, not to wtm. What wtm keeps is
an index of what it started, `jobs.json` next to the [global config](configuration.md#global-config), which the next daemon reads
back. That is what makes `wtm run ps` still list your stacks after a reboot, and
`wtm run down` still stop them. Those stacks show as `detached` rather than `running`,
because nothing about them was ever verified: wtm launched them and has not seen them
since. The other statuses `run ps` shows are `running`, `joined` (a worktree's hold on a
shared service), `stopped`, `crashed` and `reaped`; see
[`run ps` statuses](jobs-and-profiles.md#run-ps-statuses). `wtm run daemon status`
reports what is up, and `stop` / `restart` are the way out when you want the process gone.

