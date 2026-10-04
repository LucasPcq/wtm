# `run.toml` reference

`<git-common-dir>/wtm/run.toml` (`.git/wtm/run.toml` in a normal clone) declares the jobs of the `run` module. It is per-clone and never committed; share it with `wtm run export | wtm run import -`. `wtm run init` writes it from detection, `wtm run job …` / `wtm run profile …` edit it, and a hand edit is fine: the file is validated every time it is read, and every write puts its JSON schema beside it (`schemas/run.schema.json`) for editor autocomplete.

**Validation is strict.** An unknown key, a misspelt `kind`, a reference to an undeclared job, or two links claiming the same key refuse the file, naming the cause. A refused file never fails a core command (`create`, `extract`, `checkout`, `env`, `clean`…): only the run part is skipped, with a warning. `run up` and `run start` refuse it; `run down`, `run stop` and `run ps` warn and carry on, so what is running can always be stopped.

## Example

```toml
isolation   = "isolated"
addressing  = "names"
concurrency = "parallel"

[[job]]
name = "docker"
kind = "service"
cmd  = "docker compose up -d"
stop = "docker compose down"
  [job.ports]
  DB_PORT = 5432

[[job]]
name = "api"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/api"
  [job.ports]
  PORT = 4000
  [job.url]
  port = "PORT"

[[job]]
name    = "migrate"
kind    = "task"
cmd     = "pnpm migrate"
touches = ["docker"]

[[profile]]
name    = "all"
jobs    = ["docker", "migrate", "api"]
default = true

[[env_port]]
file = "apps/api/.env"
key  = "DATABASE_URL"
job  = "docker"
port = "DB_PORT"
```

## Top-level keys

| Key | Default | Meaning |
| --- | --- | --- |
| `isolation` | `"isolated"` | what a new worktree gets when nobody is asked: `"isolated"` or `"verbatim"`, see [Isolation](isolation.md) |
| `addressing` | `"names"` | what an `[[env_port]]` link writes into a value pointing at a job that publishes a URL: its named origin (`"names"`) or its port (`"ports"`), see [Addressing](addressing.md) |
| `concurrency` | unset (asked once) | the standing answer when another worktree runs jobs: `"parallel"` keeps them, `"exclusive"` stops them first |
| `port_offset_block` | `10` | the spacing between two worktrees' ports: worktree `n` binds `base + n × block` |
| `port_probe_timeout` | `15` | seconds `run up` / `run start` wait for a declared port to answer; a negative value turns the check off |

## `[[job]]`

| Key | Required | Meaning |
| --- | --- | --- |
| `name` | yes | unique, no spaces |
| `kind` | yes | `"service"` (long-running) or `"task"` (one-shot) |
| `cmd` | yes | a `/bin/sh` line |
| `stop` | no | services only: a `/bin/sh` line bringing the service down. Its presence makes `cmd` a launcher wtm waits on (`docker compose up -d`) |
| `cwd` | no | working directory, relative to the worktree root |
| `ports` | no | `NAME = base` pairs: the job runs with `NAME=base + offset` in its environment |
| `url` | no | `{ port = "NAME", host = "api" }`: publish the declared port `NAME` under a named URL; `host` defaults to the job's name |
| `probe` | no | `false` skips the port check for this job |
| `binds_no_port` | no | `true` for a service that listens on nothing by design (a watcher, a worker, a runner) |
| `runs` | no | the declared jobs this one starts itself (`turbo run dev`); no cycles |
| `touches` | no | the declared services whose data this job changes, see [foreign data](isolation.md#foreign-data-and-touches) |
| `scope` | no | `"shared"`: one instance for the repository, run in the main checkout, see [Shared services](shared-services.md). Absent means one per worktree |
| `namespace` | no | shared services only: `{ name, create, remove, env }`, the worktree's own part of the service. `name` and `create` are required together |

A job runs with the worktree's identity in its environment: `WTM_BRANCH`, `WTM_WORKTREE` (the branch as a slug), `WTM_ORDINAL` (the main checkout is `0`), `WTM_PORT_OFFSET`, `WTM_ISOLATION`, `COMPOSE_PROJECT_NAME` (isolated worktrees and the main checkout) and its declared ports. Two base ports a multiple of `port_offset_block` apart are refused, since two worktrees would meet on the same port.

## `[[profile]]`

| Key | Required | Meaning |
| --- | --- | --- |
| `name` | yes | unique, no spaces |
| `jobs` | yes | declared job names, in start order |
| `default` | no | `true` on at most one profile: what `run up` starts without `--profile` |

## `[[env_port]]`

Links a `.env` key to a declared port, so the port inside its value follows the worktree:

| Key | Meaning |
| --- | --- |
| `file` | a `.env` target configured in `config.toml` |
| `key` | the key whose value carries the port |
| `job` | the job declaring the port, required since two jobs may both declare a `PORT` |
| `port` | the declared port name |

wtm finds the declared base inside the value and shifts only that number or, under `addressing = "names"`, writes the job's whole named origin when the value is a URL and the job publishes one. A value where the base is missing or appears twice is reported and left alone.

## `[[env]]`

Writes a `.env` key's whole value from a template: what a worktree holds of a shared service, which no port can say:

| Key | Meaning |
| --- | --- |
| `file` | a `.env` target configured in `config.toml` |
| `key` | the key wtm owns |
| `job` | the job the value speaks about |
| `value` | a template over `{namespace}`, `{port.NAME}`, `{origin}`, `{worktree}`, `{ordinal}` |

A key is written by an `[[env]]` link or an `[[env_port]]` link, never both.
