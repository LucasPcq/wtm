# Shared services and namespaces

Isolation duplicates everything: two worktrees of a project with four postgres containers and a keycloak run eight postgres and two JVMs. A **shared service** runs once for the whole repository instead, and gives each worktree its own **namespace** in it — a database, a realm.

## Declaring one

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
  name   = "app_{worktree}"                 # app_feat-x in worktree feat/x
  create = "scripts/db-worktree-add.sh"     # run on every start: must be safe to run again
  remove = "scripts/db-worktree-drop.sh"    # run by wtm clean and wtm prune
```

`wtm run init` asks which compose services to share, then their namespace, row by row; `wtm run job add|edit` take the same fields as flags (`--scope shared`, `--namespace-name`, `--namespace-create`, `--namespace-remove`, `--namespace-env KEY=VALUE`).

Without a `[job.namespace]` the service is **shared outright**, data included: every worktree reads and writes the same data, which is [foreign data](isolation.md#foreign-data-and-touches) to all of them.

## Where it runs

The real service runs in the **main checkout**, which never takes a port offset: a declared `5432` is the `5432` it binds, in every worktree. A worktree that starts it — `run up` of a profile holding it, or `run start` — starts it in the main checkout if it is not up yet, then holds it: `run ps` shows that hold as `joined`. The service stops only once no worktree holds it any more; `run down` in one worktree reports `released` for a service others still hold.

## The namespace commands

wtm does not know what a database or a realm is: it runs your two commands at the right moment, with the right environment.

- `name` is data, never executed: wtm fills in `{worktree}` (the branch as a slug) and `{ordinal}`. `app_{worktree}` is the proposal.
- `create` runs on **every** start of the shared service, retried for a short while in case the service is not accepting connections yet. It must create the namespace if it is absent and do nothing if it is there.
- `remove` runs when the worktree is removed, never on `run stop` or `run down`. Leave it empty to keep the data.

Both are `/bin/sh` lines (or a script path) that read `$WTM_NAMESPACE`, `$WTM_WORKTREE`, `$WTM_ORDINAL`, the worktree's declared ports and URLs, and the extra variables of `namespace.env`. A `create` that clones main's database (`CREATE DATABASE "$WTM_NAMESPACE" TEMPLATE app`) starts each worktree from main's data without sharing it — see [the developer notes](../dev/shared-services.md#starting-a-namespace-from-mains-data) for a complete script.

A worktree records each namespace it actually created in its `meta.json` (`namespaces`), the moment the service reports started. A worktree created and thrown away without ever starting the service owes nothing.

## Telling the app: `[[env]]`

A port link (`[[env_port]]`) says where the shared service answers — the same address for everyone. Which namespace a worktree holds is said by an `[[env]]` link, which writes a key's **whole** value from a template:

```toml
[[env]]
file  = "apps/api/.env"
key   = "DATABASE_URL"
job   = "postgres"
value = "postgresql://app:app@localhost:{port.POSTGRES_PORT}/{namespace}"
```

The placeholders are `{namespace}`, `{port.NAME}` (a port of that job, as it resolves in the worktree), `{origin}` (the job's published address), `{worktree}` and `{ordinal}`; anything else is refused when `run.toml` is read. A key may be written by an `[[env]]` link or an `[[env_port]]` link, never both. The links are settled when a worktree is created and whenever `wtm env` reconciles it — no daemon needed.

## What `clean` and `prune` do with the data

Removing a worktree runs in a fixed order: its jobs are stopped and checked gone, its `on_clean` hooks run, git removes the worktree, and **only then** is its data dropped. A failure before that last step leaves the data where it was.

- **By default the namespaces are dropped**: the confirmation names each one, and `--output json` reports each as `dropped`, `deferred` or `kept`.
- `--keep-data` withholds the drop.
- A shared service that is down cannot drop anything. The interactive form asks whether to start it now or keep the data until wtm next starts it; `--yes` keeps it (`deferred`), `--drop-data` starts the service, drops, and lets it go again.
- A deferred drop is recorded in `pending-removals.toml` and paid the next time wtm starts that service (`run up`, `run start`) or runs `prune`. A drop that takes longer than 30 s is deferred the same way. If a worktree of the same name is created again before then, the debt is withdrawn, never paid.
- A namespace another live worktree reaches under the same name is never dropped (`kept`).
