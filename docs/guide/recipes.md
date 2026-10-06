# Recipes

Complete setups for common projects. Each one shows the `run.toml` it ends with (in `.git/wtm/run.toml`) and the commands that use it. `wtm run init` writes most of this from detection; the recipes show where to take it by hand, with `wtm run job add|edit` or an editor. Every key is described in the [`run.toml` reference](run-toml.md).

- [A pnpm or turbo monorepo](#a-pnpm-or-turbo-monorepo)
- [A docker compose app](#a-docker-compose-app)
- [One postgres, a database per worktree](#one-postgres-a-database-per-worktree)
- [Several AI agents, each in its own worktree](#several-ai-agents-each-in-its-own-worktree)
- [Stacked pull requests](#stacked-pull-requests)
- [Run a command across worktrees](#run-a-command-across-worktrees)

## A pnpm or turbo monorepo

One root script (`turbo run dev`, `pnpm -r --parallel run dev`) starts every app. Declare each app as a job with its own port, then the root script as a **runner** that `runs` them:

```toml
[[job]]
name = "web"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/web"
  [job.ports]
  WEB_PORT = 3000
  [job.url]
  port = "WEB_PORT"

[[job]]
name = "api"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/api"
  [job.ports]
  API_PORT = 4000
  [job.url]
  port = "API_PORT"

[[job]]
name = "dev"
kind = "service"
cmd  = "pnpm turbo run dev"
runs = ["web", "api"]

[[profile]]
name    = "dev"
jobs    = ["dev"]
default = true
```

- The runner is started with its children's ports in its environment (`WEB_PORT=3010`, `API_PORT=4010` in the first worktree), and their named URLs are published under it. `run ps` lists the runner with the apps it holds beneath it.
- **Give each app its own variable name.** A runner passes one environment to every child, so two apps both reading `PORT` cannot get two values. Each app reads its own: `"dev": "next dev --port ${WEB_PORT:-3000}"` in `apps/web/package.json`.
- **Turborepo filters the environment by default** (`envMode: "strict"`), so the ports never reach the apps. Let them through in `turbo.json`: `"globalPassThroughEnv": ["WEB_PORT", "API_PORT"]`.
- `web` and `api` stay startable on their own: `wtm run start --job api` starts one app without the runner. wtm refuses to start an app its running runner already holds.

`wtm run init` asks, for each root script, which declared jobs it runs. By hand:

```bash
wtm run job add dev --cmd 'pnpm turbo run dev' --runs web --runs api --yes
wtm run profile add dev --jobs dev --default --yes
```

## A docker compose app

A compose file whose services each worktree runs on its own. wtm sets `COMPOSE_PROJECT_NAME` per worktree (`acme-feat-login`), so containers, networks and volumes are already separate. What is left is the host ports, which must read a variable:

```yaml
# docker-compose.yml
services:
  db:
    image: postgres:16
    ports:
      - "${DB_PORT:-5432}:5432"
  redis:
    image: redis:7
    ports:
      - "${REDIS_PORT:-6379}:6379"
```

```toml
[[job]]
name = "stack"
kind = "service"
cmd  = "docker compose up -d"
stop = "docker compose down"
  [job.ports]
  DB_PORT    = 5432
  REDIS_PORT = 6379

[[job]]
name = "api"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/api"
  [job.ports]
  PORT = 4000
  [job.url]
  port = "PORT"

[[profile]]
name    = "dev"
jobs    = ["stack", "api"]
default = true

[[env_port]]
file = "apps/api/.env"
key  = "DATABASE_URL"
job  = "stack"
port = "DB_PORT"
```

- With a `stop` command, `cmd` is a launcher: wtm waits for `docker compose up -d` to exit and runs `docker compose down` on `wtm run down`. `run ps` shows the stack as `detached`.
- The `[[env_port]]` link rewrites the port inside `DATABASE_URL` (`postgresql://app:app@localhost:5432/app` becomes `…:5442/app` in the first worktree) when the worktree is created, and whenever `wtm env` reconciles it.
- `wtm run init` finds literal host ports (`"5432:5432"`) and absolute names (`container_name`, a volume's `name`) and offers to rewrite them; `--patch-compose` does it unattended. A renamed volume starts empty.
- A `docker compose up` typed by hand in the worktree is isolated too, since the `.env` carries the ports and `COMPOSE_PROJECT_NAME`.

See [How `wtm run` works](how-run-works.md) for compose names and the port check.

## One postgres, a database per worktree

Ten worktrees do not need ten postgres containers. A **shared** service runs once, in the main checkout, and each worktree gets its own database in it, created on start and dropped on `wtm clean`:

```toml
[[job]]
name  = "postgres"
kind  = "service"
cmd   = "docker compose up -d postgres"
stop  = "docker compose stop postgres"
scope = "shared"
  [job.ports]
  POSTGRES_PORT = 5432
  [job.namespace]
  name   = "app_{worktree}"
  create = "scripts/db-worktree-add.sh"
  remove = "scripts/db-worktree-drop.sh"
    [job.namespace.env]
    PGPASSWORD = "postgres"

[[job]]
name    = "migrate"
kind    = "task"
cmd     = "pnpm db:migrate"
cwd     = "apps/api"
touches = ["postgres"]

[[job]]
name = "api"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/api"
  [job.ports]
  PORT = 4000
  [job.url]
  port = "PORT"

[[profile]]
name    = "dev"
jobs    = ["postgres", "migrate", "api"]
default = true

[[env]]
file  = "apps/api/.env"
key   = "DATABASE_URL"
job   = "postgres"
value = "postgresql://postgres:postgres@localhost:{port.POSTGRES_PORT}/{namespace}"
```

The two commands are yours; wtm runs them with `$WTM_NAMESPACE` (`app_feat-login`), the job's ports and the `namespace.env` variables. `create` runs on **every** start, so it must do nothing when the database exists:

```sh
#!/bin/sh
# scripts/db-worktree-add.sh
set -e
psql="psql -h localhost -p $POSTGRES_PORT -U postgres -v ON_ERROR_STOP=1"
exists=$($psql -tAc "SELECT 1 FROM pg_database WHERE datname = '$WTM_NAMESPACE'")
[ "$exists" = 1 ] && exit 0
$psql -c "CREATE DATABASE \"$WTM_NAMESPACE\" TEMPLATE app"
```

```sh
#!/bin/sh
# scripts/db-worktree-drop.sh
set -e
psql -h localhost -p "$POSTGRES_PORT" -U postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS \"$WTM_NAMESPACE\" WITH (FORCE)"
```

- `TEMPLATE app` starts each worktree from a copy of main's data (`app`, the database main's `.env` names), so there is nothing to seed. It refuses while main has open connections; drop the `TEMPLATE` clause for an empty database.
- The `[[env]]` link writes the whole `DATABASE_URL`, pointing each worktree at its own database. `migrate` declares `touches = ["postgres"]`; since every worktree has its own namespace, it runs without a question.
- `run ps` shows the worktrees holding the service as `joined`; it stops once none holds it.
- `wtm clean feat/login` drops the database after removing the worktree; `--keep-data` keeps it. When postgres is down, `--yes` defers the drop to its next start and `--drop-data` starts it to drop now.

The same fields exist as flags: `wtm run job add postgres --scope shared --namespace-name 'app_{worktree}' --namespace-create … --namespace-remove … --namespace-env PGPASSWORD=postgres`. See [Shared services](shared-services.md).

## Several AI agents, each in its own worktree

Give each agent a branch, a directory and a running stack of its own. Every command below prompts for nothing, and `--output json` gives it a document to read instead of text:

```bash
wtm agents install                                       # once: the using-wtm skill for Claude Code / Cursor

branch=agent/fix-checkout
wtm create "$branch" --if-not-exists --yes --output json  # {"results": [{"branch", "path", "isolation", ...}], "failed": []}
cd "$(wtm resolve "$branch")"

wtm run up "$branch" -d --yes --output json              # per-job status, ports and URLs
api=$(wtm run url "$branch" --job api)                   # http://api.agent-fix-checkout.acme.localhost:11080
curl -s "$api/health"

wtm run logs "$branch" --output json                     # the last 1000 lines of each job
wtm run down "$branch" --yes
wtm clean "$branch" --yes                                # add --force once the work is pushed elsewhere
```

- `run up --yes` leaves the other worktrees' jobs running, so agents starting at the same time do not stop each other. Setting `concurrency = "parallel"` in `run.toml` makes that the answer for people too.
- Under `--yes` a missing choice is an error naming its flag, never a picker: `run start` needs `--job`, `create` needs the branch.
- Exit codes are stable: `10` the worktree already exists, `11` the branch does not exist, `12` the repository was never initialized with wtm, `14` a job or profile `run.toml` does not declare, `16` no `run.toml`, `18` a `wtm env --check` that found drift, `19` an interactive run you backed out of (so `wtm create x && wtm go x` stops there), `20` a `wtm events` that received an event of a newer schema, `21` not in a git repository, `2` a usage error. Which of them `wtm events` treats as final is in [The event stream](events.md#when-it-exits).
- `wtm run ps --output json` lists everything running, across repositories, and `wtm list --output json` every worktree with its state.
- `clean --yes` still refuses a worktree with uncommitted or unpushed work; that refusal is lifted only by `--force`.

`wtm agents install` adds a skill to `.claude/` or `.cursor/` that teaches the agent these commands. Re-run it after upgrading wtm.

## Stacked pull requests

Each branch builds on the previous one, and every worktree records its parent:

```bash
wtm create feat/api --yes
wtm create feat/api-client --from feat/api --yes
wtm create feat/checkout-ui --from feat/api-client --yes
```

```console
$ wtm tree

  main
  └─ feat/api
     └─ feat/api-client
        └─ feat/checkout-ui
```

When `main` moves, or you amend `feat/api`, rebase the chain in order, parents first:

```bash
wtm sync --all --dry-run              # the plan, nothing changed
wtm sync feat/api feat/api-client feat/checkout-ui
wtm sync --all --yes --push           # unattended, then force-push with lease
```

On a conflict, `sync` aborts that branch's rebase and skips its descendants; `--keep-conflict` leaves the rebase in progress to resolve by hand. `--yes` never pushes unless `--push` is given.

When `feat/api` is merged (squash or rebase merges included), move its child onto `main` and remove it:

```bash
wtm reparent feat/api-client --to main --yes
wtm sync feat/api-client feat/checkout-ui --yes --push
wtm clean feat/api --yes
```

Or in one pass once several PRs are merged, with `gh` installed: `wtm prune --merged --reparent-children --yes`. `wtm tree --output mermaid` prints the stack as a flowchart for a PR description.

`prune` only reads the branches that have a worktree: it asks GitHub for their pull requests in one query, however old the pull request, and its fetch refreshes only their remote-tracking refs, so a repository with thousands of branches costs it no more than one with ten. The remote-tracking refs of the other branches are left as they are: `git fetch --prune` refreshes them.

## Run a command across worktrees

Several branches in flight, one lockfile bump or one test suite to run on all of them. Create them in one go, run the command everywhere in parallel, remove them in one go:

```console
$ wtm create feat/login feat/billing fix/header --yes
$ wtm exec --all -- 'pnpm lint && pnpm test'

  ✓ main (41.2s)
  ✗ feat/billing (exit 1, 38.7s)
  ✓ feat/login (40.1s)
  ✓ fix/header (39.5s)

  ✗ pnpm lint && pnpm test · 4 worktrees: 1 failed
    ✗ feat/billing (exit 1, 38.7s)
        FAIL  src/invoice.test.ts > rounds the total
      log  /code/acme/.git/wtm/exec/feat%2Fbilling.log
    ✓ 3 passed

$ wtm clean feat/login fix/header --yes
```

- Everything after `--` is one `/bin/sh -c` line, run from each worktree's root: quote it when it holds `&&` or a pipe, or your own shell takes them.
- Each command gets **its own worktree's** environment: the variables your shell carries about the worktree you stand in are removed, and the target's run variables (compose project, shifted ports) are added when it has them, as for its hooks. See [The environment of `wtm exec`](configuration.md#the-environment-of-wtm-exec).
- Name worktrees (`wtm exec feat/login feat/billing -- pnpm test`), or `--all` for every one, the main checkout included. Without either, `wtm exec` opens a wizard that also asks for the command.
- `--jobs N` caps how many run at once (one per CPU by default): `wtm exec --all --jobs 2 -- pnpm install`. stdin is closed, so an interactive command cannot wait for input.
- Successes show one line; failures show the tail of their output. `--print` shows every worktree's full output: `wtm exec --all --yes --print -- git log -1 --oneline`. Each worktree's whole output is kept in `.git/wtm/exec/<branch>.log`.
- The run exits `1` when any command failed; each worktree's own exit code is in the report, and in `--output json` (`{command, results: [{branch, path, status, exit_code, …}], failed: [branch…]}`) for a script or an agent.

`create` and `clean` with several branches go one after the other, and a failure does not stop the others: the run ends with what succeeded and what failed, and exits with the first failure's code. Under `--yes`, `clean` refuses the whole batch before removing anything when one worktree is unsafe (dirty, unpushed, open PR, locked): pass `--force`, or name only the safe ones. Their `--output json` is an envelope, `{"results": [...], "failed": [...]}` (see [Migrating to 0.29](migrating-to-0.29.md)).
