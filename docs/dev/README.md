# Developer documentation

Reference documentation for people (and agents) working **on** wtm. It describes the code as delivered, not the design that preceded it.

> The rest of `docs/` is **generated** by `tools/gendocs` from the Cobra command tree (`make docs`) and must never be hand-edited. `docs/dev/` is hand-written and is the only manual content under `docs/`; gendocs only writes `wtm_*.md` and `commands.json` (the command reference grouped like `wtm --help`, read by the documentation site) at the root of `docs/`, so this subdirectory survives a regeneration untouched.

| Document | What it covers |
| -- | -- |
| [architecture.md](architecture.md) | The annotated package map, who may call whom and what each interdiction buys, how a command designates a worktree |
| [store.md](store.md) | `internal/store/` — the one writer of each state file: `meta.json` (`store.Meta`), the changes to `config.toml` and the remembered answers (`store.ProjectConfig`), the managed `.env` files (`store.EnvFiles`), what `stateowner` and `metawriter` hold |
| [state-files.md](state-files.md) | `internal/infra/fs` — how every file is written (`UpdateFile`, `fs.Change`, where the lock lives), the schema version of each state file and how to raise one, what `statewrite` holds |
| [flow-layer.md](flow-layer.md) | `internal/flow/`, the model of every command — the three seams, the step model, unattended resolution, embedding, scheduling, events, testing a flow |
| [adding-a-mutation-command.md](adding-a-mutation-command.md) | End-to-end recipe for a worktree-mutating command: vocabulary, rules, the flow package, its wiring, presenter, tests and docs |
| [cli.md](cli.md) | `internal/surface/cli` — the root (signals, `--quiet`, usage errors, exit code) and how a command's flags reach its flow |
| [output.md](output.md) | What a command prints: the one question a block has to answer, the frame and the accent bar, the four levels, the two shapes of a conclusion, the glyph vocabulary, `--quiet`, JSON |
| [run-addressing.md](run-addressing.md) | Named URLs: proxy vs redirection vs public port, and what `addressing` writes into a `.env` |
| [shared-services.md](shared-services.md) | `scope = "shared"`: one instance for the repository, one namespace per worktree, and why the job table is the reference count |
| [lint.md](lint.md) | `make lint` and what each gate holds: the `archlint` rules, the exception files, the pre-commit hook, the rules considered and left out |
| [identity.md](identity.md) | How wtm looks and speaks outside the terminal: positioning, voice, the colour tokens taken from the CLI, type, logo, layout, motion, anti-patterns, and where each surface applies them |
| [changelog.md](changelog.md) | How to write `CHANGELOG.md`: the release template, the rules, and how a section becomes the GitHub release notes |

For the coding standards themselves (immutability, struct params, constants, comments), see [`CLAUDE.md`](../../CLAUDE.md) and, for Go idioms with their sources, the `go-style` skill in `.claude/skills/go-style/SKILL.md`.

To see a change working in the real binary — an isolated sandbox driven with tmux, or recorded with VHS, optionally before/after — use the `wtm-sandbox` skill (`.claude/skills/wtm-sandbox/`). To open a pull request (base branch from `wtm tree`, that terminal proof attached with `gh --attach`, body template), use the `open-pr` skill (`.claude/skills/open-pr/`).
