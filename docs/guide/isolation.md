# Isolation: isolated or verbatim

Two worktrees of the same repository run the same services. Isolation is what keeps them from colliding on a port, a Docker container or a database, and wtm lets each worktree decide, once, whether it wants that.

## The two answers

`create`, `extract` and `checkout` record how the new worktree stands against its **source** (the worktree or branch it was created from), in the worktree's `meta.json`:

- **Isolated** (the default). The worktree gets its own ports (`base + WTM_PORT_OFFSET`), its own compose project (`COMPOSE_PROJECT_NAME=<repo>-<worktree>`, so its own containers, networks and volumes) and its own namespace in each shared service. These values are written into its `.env` files when it is created, and applied when `wtm run` starts its jobs, so a `docker compose up` or a `pnpm dev` typed by hand is isolated too.
- **Verbatim**. The `.env` is kept as it was copied, and `wtm run` runs the worktree on the ports and the data that file names: its source's. `COMPOSE_PROJECT_NAME` is not set by wtm: the copied `.env` decides, so the worktree **shares its source's compose volumes and data**. Only one of the two can be up at a time: `run up` and `run start` say so and offer to stop the other one, rather than letting a port bind fail.

The question is only asked when `run.toml` declares something a worktree could isolate: a port, a namespace, an `[[env_port]]` or `[[env]]` link, a compose stack. Without any, both answers do the same thing.

Pick per worktree with `--isolation isolated|verbatim`; set the project's default with `isolation = "verbatim"` at the top of `run.toml`. Under `--yes`, a creation takes the flag, else the project default, else `isolated`.

## Changing your mind

`wtm env <branch> --isolation isolated|verbatim` settles an existing worktree on the other answer:

- `--isolation isolated` writes every port, compose project and namespace value the worktree was left without.
- `--isolation verbatim` puts the values wtm owns (linked ports, `[[env]]` values and `COMPOSE_PROJECT_NAME`) back to the source's, removes the ones the source lacks, and leaves every other key alone. The worktree then shares its source's compose volumes again. Namespaces the worktree already created stay recorded, so `wtm clean` still drops them.

The new isolation is recorded only once the `.env` is in line with it: a run that fails or is cancelled records nothing. The interactive `wtm env` shows the values it will put back before it does, and its recap can keep a worktree verbatim from then on.

## Worktrees created before v0.28

A worktree created by an earlier wtm has no `isolation` in its `meta.json`. It keeps running on its source's ports and compose project until you decide:

- `wtm env <branch> --yes` reconciles its keys and **touches nothing run-related**: no port shift, no `COMPOSE_PROJECT_NAME`. The report says the adoption is pending.
- The interactive `wtm env <branch>` offers to adopt isolation, naming what changes: a new compose project, so the volumes it uses today (`<old project>_*`) are no longer used.
- `wtm env <branch> --isolation isolated` adopts it explicitly; `--isolation verbatim` records that it stays on its source's values.

Until one of these runs, `wtm run up` and `wtm run start` refuse the worktree and name the command to run.

## Hooks

`on_create` and `on_clean` hooks get the worktree's run variables (`COMPOSE_PROJECT_NAME`, `WTM_*` and the declared ports) **only when** `run.toml` declares a job running `docker compose` **and** the worktree recorded its isolation. Otherwise a hook runs with the environment it had before the run module existed. When the worktree's own `.env` sets `COMPOSE_PROJECT_NAME`, that value is the one the hook gets.

## Foreign data and `touches`

Some tasks change data: a migration, a reset, a seed. Run against data the worktree does not own, they change it for someone else too. wtm calls that **foreign data**:

- a verbatim worktree's source's data (the two share a database);
- a shared service's data when it declares no `[job.namespace]` (every worktree shares it).

wtm cannot read that from a command, so a job declares it: `touches = ["postgres"]` names the services whose data it changes. `wtm run init` asks it task by task and pre-fills what the names make obvious; `wtm run job add|edit --touches` sets it by hand.

Before starting a job whose `touches` reach foreign data (including a job started by a runner through `runs`), `run up` and `run start` stop and ask. Under `--yes` they refuse, and name the way out: `--force` runs it anyway; `wtm env <branch> --isolation isolated` gives a verbatim worktree its own data; a `[job.namespace]` gives each worktree its own part of a shared service.
