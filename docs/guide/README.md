# wtm user guide

How wtm works beyond the first steps: its configuration, and the `run` module with the per-worktree isolation it rests on. They explain how the pieces fit together; the flags of each command live in `wtm <command> --help` and in the generated [command reference](../wtm.md).

New to wtm? Start with [Getting started](getting-started.md), then pick a setup from [Recipes](recipes.md). Scripting wtm or wiring it into another tool? Read [Integrations](integrations.md).

| Page | What it covers |
| --- | --- |
| [Getting started](getting-started.md) | a ten-minute tutorial: install, two worktrees running side by side, cleaning one |
| [Recipes](recipes.md) | complete setups: a pnpm/turbo monorepo, a docker compose app, a shared postgres, AI agents in parallel, stacked PRs, one command across worktrees (`wtm exec`) |
| [Troubleshooting](troubleshooting.md) | a port in use, a crashed job, the daemon's version, `wtm go`, exit 16, named URLs, `.env` drift |
| [Configuration](configuration.md) | `config.toml`, env strategies, hooks, the environment of `wtm exec`, the global config, editor autocomplete |
| [Isolation: isolated or verbatim](isolation.md) | how a worktree stands against its source, `COMPOSE_PROJECT_NAME`, adopting isolation on an older worktree, `touches` and foreign data |
| [Jobs, profiles and runners](jobs-and-profiles.md) | services and tasks, what `run up` starts, runners, the run view and `-d`, port checks, `run ps` statuses |
| [Shared services and namespaces](shared-services.md) | one instance for the repository, a namespace per worktree, `[[env]]` links, what `clean` and `prune` drop |
| [Named URLs and addressing](addressing.md) | the run proxy, named and port URLs, `url.host`, the `addressing` mode, port 80 on macOS |
| [How `wtm run` works](how-run-works.md) | the job environment (`WTM_*`, `COMPOSE_PROJECT_NAME`), ports and the port check, what `run init` proposes, ports and addresses in a `.env`, compose names |
| [`run.toml` reference](run-toml.md) | every key of the file, with its default |
| [Where wtm keeps its state](state.md) | the files under `<git-common-dir>/wtm/` and beside the global config |
| [Integrations](integrations.md) | building on wtm: `--yes` and `--output json`, exit codes, the agent skill, `wtm version --output json`, `WTM_CORRELATION_ID` |
| [The event stream](events.md) | `wtm events`: every worktree change as it happens, for editors, terminal plugins and agents |
| [Migrating to 0.28](migrating-to-0.28.md) | what changed for a v0.27 user, and what to do about it |
| [Migrating to 0.29](migrating-to-0.29.md) | the `create` / `clean --output json` envelopes, several branches in one run |

The module is opt-in: nothing here applies until `wtm run init` writes `run.toml`. Until then, the `run` commands that need it refuse (exit `16`) and point at `wtm run init`.
