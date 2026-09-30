# wtm user guide

How wtm works beyond the first steps: its configuration, and the `run` module with the per-worktree isolation it rests on. They explain how the pieces fit together; the flags of each command live in `wtm <command> --help` and in the generated [command reference](../wtm.md).

| Page | What it covers |
| --- | --- |
| [Configuration](configuration.md) | `config.toml`, env strategies, hooks, the global config, editor autocomplete |
| [Isolation: isolated or verbatim](isolation.md) | how a worktree stands against its source, `COMPOSE_PROJECT_NAME`, adopting isolation on an older worktree, `touches` and foreign data |
| [Jobs, profiles and runners](jobs-and-profiles.md) | services and tasks, what `run up` starts, runners, the run view and `-d`, port checks, `run ps` statuses |
| [Shared services and namespaces](shared-services.md) | one instance for the repository, a namespace per worktree, `[[env]]` links, what `clean` and `prune` drop |
| [Named URLs and addressing](addressing.md) | the run proxy, named and port URLs, `url.host`, the `addressing` mode, port 80 on macOS |
| [How `wtm run` works](how-run-works.md) | the job environment (`WTM_*`, `COMPOSE_PROJECT_NAME`), ports and the port check, what `run init` proposes, ports and addresses in a `.env`, compose names |
| [`run.toml` reference](run-toml.md) | every key of the file, with its default |
| [Where wtm keeps its state](state.md) | the files under `<git-common-dir>/wtm/` and beside the global config |
| [Migrating to 0.28](migrating-to-0.28.md) | what changed for a v0.27 user, and what to do about it |

The module is opt-in: nothing here applies until `wtm run init` writes `run.toml`. Until then, the `run` commands that need it refuse (exit `16`) and point at `wtm run init`.
