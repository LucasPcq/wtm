# wtm user guide

How wtm works beyond `--help`. Every flag of every command is in `wtm <command> --help` and the generated [command reference](../wtm.md); these pages explain how the pieces fit together.

## Start here

- **[Getting started](getting-started.md)**: install, two worktrees running side by side, cleaning one up. Ten minutes.
- **[Recipes](recipes.md)**: complete setups to copy: a pnpm/turbo monorepo, a docker compose app, a shared postgres, agents in parallel, stacked PRs, one command across worktrees.
- **[Troubleshooting](troubleshooting.md)**: a port in use, a crashed job, `wtm go` not changing directory, `.env` drift, and the other usual suspects.

## Worktrees

- **[Configuration](configuration.md)**: `config.toml`, how `.env` files are provisioned, `on_create` / `on_clean` hooks, the environment of `wtm exec`, the global config.
- **[Where wtm keeps its state](state.md)**: the files under `<git-common-dir>/wtm/` and beside the global config.

## Dev stacks with `wtm run`

Opt-in: nothing in this section applies until `wtm run init` writes `run.toml`.

- **[Isolation](isolation.md)**: isolated or verbatim, what a worktree shares with its source, and `touches` for data it does not own.
- **[Jobs, profiles and runners](jobs-and-profiles.md)**: services and tasks, what `run up` starts, the run view, port checks.
- **[How `wtm run` works](how-run-works.md)**: the job environment, ports, what `run init` proposes, compose project names.
- **[Shared services](shared-services.md)**: one postgres for the repository, a namespace per worktree.
- **[Named URLs](addressing.md)**: one address per job and worktree, the run proxy, port 80 on macOS.
- **[`run.toml` reference](run-toml.md)**: every key, with its default.

## Integrations

- **[Integrations](integrations.md)**: driving wtm from scripts, agents and other tools: `--yes`, `--output json`, exit codes, the agent skill.
- **[The event stream](events.md)**: `wtm events`, every worktree change as it happens.

## Upgrading

- **[Migrating to 0.29](migrating-to-0.29.md)**: the `create` / `clean --output json` envelopes, two exit codes.
- **[Migrating to 0.28](migrating-to-0.28.md)**: the `run` module, hooks, commands and JSON that changed since 0.27.
