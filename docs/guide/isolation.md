# Isolation

Two worktrees of the same repository run the same services. Isolation keeps them from colliding on a port, a Docker container or a database, and each worktree decides once whether it wants it:

```bash
wtm create feat/login --yes                         # isolated: its own ports, compose project and data
wtm create hotfix/prod --isolation verbatim --yes   # verbatim: its source's .env, ports and data
```

With a compose stack and a `.env` key linked to the `web` job's port (`PORT`, base `5173`), the first worktree gets offset `+10`:

```console
$ cat ../.trees/feat-login/.env       # isolated
PORT=5183
COMPOSE_PROJECT_NAME=acme-feat-login
$ cat ../.trees/hotfix-prod/.env      # verbatim: as copied from its source
PORT=5173
```

## The two answers

`create`, `extract` and `checkout` record how the new worktree stands against its **source** (the worktree or branch it was created from), in its `meta.json`:

| | Isolated (the default) | Verbatim |
| --- | --- | --- |
| Ports | its own: `base + WTM_PORT_OFFSET` | its source's |
| Compose project | `COMPOSE_PROJECT_NAME=<repo>-<worktree>`: its own containers, networks and volumes | not set by wtm: the copied `.env` decides, so it **shares its source's volumes and data** |
| Shared services | its own namespace in each | its source's |
| `.env` | the values above written at creation, and applied when `wtm run` starts its jobs, so a hand-typed `docker compose up` or `pnpm dev` is isolated too | kept exactly as copied |
| Running beside its source | yes | one at a time: `run up` and `run start` offer to stop the other one rather than let a port bind fail |

- The question is only asked when `run.toml` declares something a worktree could isolate: a port, a namespace, an `[[env_port]]` or `[[env]]` link, a compose stack. Without any, both answers do the same thing.
- `--isolation isolated|verbatim` picks per worktree; `isolation = "verbatim"` at the top of `run.toml` sets the project default. Under `--yes`, a creation takes the flag, else the project default, else `isolated`.
- `on_create` and `on_clean` hooks get the worktree's run variables only once it has recorded its isolation, among the conditions given in [The environment a hook gets](configuration.md#the-environment-a-hook-gets).
- A worktree that recorded no isolation (created by wtm 0.27) is refused by `run up` until you decide: see [Migrating to 0.28](migrating-to-0.28.md#worktrees-created-with-027).

## Changing your mind

`wtm env feat/login` asks it: the step offers to keep the worktree's isolation (first, so reconciling the `.env` changes nothing else) or to switch it, and the recap names what the switch puts back. Unattended, a run keeps the isolation unless the flag says otherwise:

```bash
wtm env feat/login --isolation verbatim --yes    # back onto its source's values
wtm env feat/login --isolation isolated --yes    # its own ports, project and namespaces again
```

- `--isolation isolated` writes every port, compose project and namespace value the worktree was left without.
- `--isolation verbatim` puts the values wtm owns (linked ports, `[[env]]` values, `COMPOSE_PROJECT_NAME`) back to the source's, removes those the source lacks, and leaves every other key alone. The worktree shares its source's compose volumes again; namespaces it already created stay recorded, so `wtm clean` still drops them.
- The new isolation is recorded only once the `.env` is in line with it: a run that fails or is cancelled records nothing. The interactive `wtm env` shows the values it will put back before anything is written.

## Foreign data and `touches`

Some tasks change data: a migration, a reset, a seed. Run against data the worktree does not own, they change it for someone else too. wtm calls that **foreign data**:

- a verbatim worktree's source's data (the two share a database);
- a shared service's data when it declares no `[job.namespace]` (every worktree shares it).

wtm cannot read that from a command, so a job declares it:

```toml
[[job]]
name    = "migrate"
kind    = "task"
cmd     = "pnpm db:migrate"
touches = ["postgres"]      # the services whose data it changes
```

`wtm run init` asks it task by task and pre-fills what the names make obvious; `wtm run job add|edit --touches` sets it by hand.

Before starting a job whose `touches` reach foreign data (including a job started by a runner through `runs`), `run up` and `run start` stop and ask. Under `--yes` they refuse and name the way out:

```bash
wtm run up hotfix/prod --yes                      # refused: migrate would change its source's database
wtm run up hotfix/prod --yes --force              # run it anyway
wtm env hotfix/prod --isolation isolated --yes    # or give the worktree its own data
```

A `[job.namespace]` on the shared service gives each worktree its own part of it instead (see [Shared services](shared-services.md)).
