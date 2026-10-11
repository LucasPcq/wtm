# Ports and .env

Every worktree's jobs bind their own ports, shifted from the ones the project declares, so two worktrees run side by side. This page covers how a port is declared and checked, what `wtm run init` finds in compose files and env files, and how a port written into a `.env` follows the worktree.

```console
$ wtm run job add web --cmd "pnpm dev" --port PORT=3000 --yes
$ wtm run up -d                       # in feat/login, the first worktree
  ✓ web started · :3010
```

## Declaring a port

Declare the base port under the name the command reads; wtm injects `base + WTM_PORT_OFFSET` (see [the job environment](how-run-works.md#the-job-environment)), so the command needs no arithmetic. The main checkout keeps the base.

- A declaration overrides whatever the environment already sets for that variable, and `stop` runs with the same ports its `cmd` did.
- For Docker, template the host side of the mapping (`"${DB_PORT}:5432"`) and declare `DB_PORT = 5432`: the container port never moves, only the binding does.
- A server that ignores `PORT` and only takes a CLI flag (Vite is the usual one) reads the same variable, because `cmd` is a shell line:

  ```bash
  wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000
  ```

- Two base ports must not differ by a **multiple of the block**, or two worktrees land on the same one: `3000` and `3010` are refused when `run.toml` is read, naming both. Neighbouring ports are fine, since a uniform offset keeps the gaps (`5434`/`5435`/`5436` become `5444`/`5445`/`5446`). Raise `port_offset_block` when a project needs more room than 10.

## The port check

Declaring a port only injects a variable; nothing guarantees the command reads it. Once the jobs are up, `run up` and `run start` dial each declared port and report the ones nothing answers on:

```console
$ wtm run up -d
  ✓ web started · :5183

  ╭──────────────────────────────────────────────────────╮
  │ Ports declared but not bound                         │
  │ web · nothing is listening on WEB_PORT=5183          │
  │   but 5173 is listening — the base port              │
  │   the command ran, but the variable did not reach it │
  ╰──────────────────────────────────────────────────────╯
```

The "base port" line is the signature of a variable that never arrived: a CLI that only takes `--port`, a hard-coded port, a `.env` that wins, or a task runner filtering the environment. **Turborepo does this by default** (`envMode: "strict"`): a root `turbo run dev` needs `globalPassThroughEnv` in `turbo.json`. wtm never edits those files; it reports what it observed.

The check never fails the run and stops as soon as every port answers.

| Switch | Where | Effect |
| --- | --- | --- |
| `--no-probe` | `run up`, `run start` | skips the check for one run |
| `probe = false` | a `[[job]]` | skips it for that job; an interactive run offers to write it for a warning that comes back every time |
| `port_probe_timeout` | top of `run.toml` | the budget in seconds (15 by default; a negative value turns the check off) |
| `binds_no_port = true` | a service | listens on nothing by design (a watcher, a worker, a runner whose children hold the ports), so wtm stops offering it a port |

## Ports `wtm run init` detects

### Compose files

`run init` reads the `ports:` of the compose files you pick. A mapping that already reads a variable is declared as-is; a literal `"5432:5432"` would bind the same port in every worktree, so it is **not** declared and wtm shows the line to write. `--patch-compose` makes the change itself:

```diff
   postgres:
     ports:
-      - "5432:5432"
+      - "${POSTGRES_PORT:-5432}:5432"
```

Only the port value is rewritten, in place (comments, indentation and quoting untouched), and the `:-5432` default keeps `docker compose up` working without wtm. Re-running `run init` backfills a compose job that predates declarative ports without overwriting one you set by hand. Names a compose file pins itself are the other half of the same question: see [absolute names](how-run-works.md#absolute-names).

### Dev servers

Dev servers are pre-filled from the env files next to their `package.json`: a `PORT` or `*_PORT` entry in `.env.local`, `.env`, or a committed `.env.example`, each job taking the file in its own directory. wtm declares the port and never rewrites a command. Where the job *reads* it is asked job by job:

- **From its own `.env`** (the pre-filled answer): wtm writes `KEY=<base>` there and in the committed template, and your config reads it (`server.port: Number(process.env.VITE_PORT)`). It still holds when you start the app yourself. `--write-port-keys` takes this route for every job without asking.
- **From the command**: `--cmd 'pnpm dev --port ${PORT}'`. It isolates only what `wtm run` starts, which the final report says.

The declared port is also the base the [`.env` links](#ports-hard-coded-in-a-env) follow, so a `.env` holding `PORT=5173` and a `VITE_API_URL` pointing at it shifts both together.

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

A shifted port only helps if what connects to it follows, and the port is usually buried in a URL. An [`[[env_port]]` link](run-toml.md#env_port) says which key carries which job's port:

```toml
[[env_port]]
file = ".env"
key  = "DATABASE_URL"
job  = "docker"
port = "DB_PORT"
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

When the value is a URL and the linked job publishes a [named URL](addressing.md), the link writes the job's whole named origin instead of a shifted port, under `addressing = "names"` (the default): see [Addressing](addressing.md#addressing-what-a-env-value-holds).

## Values that are not a port

Some values say something no port can, such as which database a worktree holds in a [shared service](shared-services.md). An [`[[env]]` link](run-toml.md#env) writes such a key's whole value from a template, filled in for each worktree:

```toml
[[env]]
file  = "apps/api/.env"
key   = "DATABASE_URL"
job   = "postgres"
value = "postgresql://app:app@localhost:{port.POSTGRES_PORT}/{namespace}"
```

Both kinds of link are settled at the same moments: when an isolated worktree is created and whenever `wtm env` reconciles it, with no daemon needed.
