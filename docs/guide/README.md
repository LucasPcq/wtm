# wtm user guide

How wtm works beyond `--help`. Every flag of every command is in `wtm <command> --help` and the generated [command reference](../wtm.md); these pages explain how the pieces fit together.

## Start here

- **[Getting started](getting-started.md)**: install, two worktrees running side by side, cleaning one up. Ten minutes.
- **[Installation](installation.md)**: every way to install, a specific version, updating.
- **[Platform support](platform-support.md)**: macOS, Linux, WSL2, and what each one supports.
- **[Compared to other tools](compared.md)**: Worktrunk, gtr, workmux, the agents' own worktrees, workz, worktree-compose, portless, Conductor, Superset, Paseo.

## Worktrees

- **[Creating worktrees](creating-worktrees.md)**: `wtm create` from a branch, several at once or on an existing branch, `wtm checkout` from a pull request, `wtm extract` to split uncommitted work, the wizard and `--yes`.
- **[Stacked branches](stacked-branches.md)**: each worktree's parent, the tree and its badges, syncing a stack parents first, fast-forward, reparent, merged parents and orphans.
- **[Removing worktrees](removing-worktrees.md)**: `clean` and `prune`, the safety refusals, `--force` and `--yes`, orphaned children, the branch and the data.
- **[Configuration](configuration.md)**: `config.toml`, how `.env` files are provisioned, `on_create` / `on_clean` hooks, the environment of `wtm exec`, moving worktrees, the global config.
- **[A worktree's state](status.md)**: `wtm status`, what a worktree runs, what it lacks, and the command that fixes each problem.
- **[Where wtm keeps its state](state.md)**: the files under `<git-common-dir>/wtm/` and beside the global config.

## Dev stacks with `wtm run`

Opt-in: nothing in this section applies until `wtm run init` writes `run.toml`.

- **[How `wtm run` works](how-run-works.md)**: the job environment, compose project names, what `run init` proposes, the daemon.
- **[Ports and .env](ports-and-env.md)**: declaring ports, the port check, what `run init` detects in compose and env files, `[[env_port]]` and `[[env]]` links.
- **[Jobs, profiles and runners](jobs-and-profiles.md)**: services and tasks, what `run up` starts, the run view, several worktrees at once.
- **[Isolation](isolation.md)**: isolated or verbatim, what a worktree shares with its source, and `touches` for data it does not own.
- **[Shared services](shared-services.md)**: one postgres for the repository, a namespace per worktree.
- **[Named URLs](addressing.md)**: one address per job and worktree, the run proxy, port 80 on macOS.
- **[`run.toml` reference](run-toml.md)**: every key, with its default.

## Scripts and agents

- **[Integrations](integrations.md)**: driving wtm from scripts, agents and other tools: `--yes`, `--output json`, exit codes, the agent skill.
- **[The event stream](events.md)**: `wtm events`, every worktree change as it happens.

## Recipes and help

- **[Recipes](recipes.md)**: complete setups to copy: a pnpm/turbo monorepo, a docker compose app, a shared postgres, agents in parallel, sharing `run.toml` with the team, one command across worktrees.
- **[Troubleshooting](troubleshooting.md)**: a port in use, a crashed job, `wtm go` not changing directory, `.env` drift, and the other usual suspects.

## Upgrading

- **[Migrating to 0.29](migrating-to-0.29.md)**: the `create` / `clean --output json` envelopes, two exit codes.
- **[Migrating to 0.28](migrating-to-0.28.md)**: the `run` module, hooks, commands and JSON that changed since 0.27.
