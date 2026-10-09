# Developer documentation

Reference documentation for people (and agents) working **on** wtm. It describes the code as delivered, not the design that preceded it.

> The rest of `docs/` is **generated** by `tools/gendocs` from the Cobra command tree (`make docs`) and must never be hand-edited. `docs/dev/` is hand-written and is the only manual content under `docs/`; gendocs only writes `wtm_*.md` and `commands.json` (the command reference grouped like `wtm --help`, read by the documentation site) at the root of `docs/`, so this subdirectory survives a regeneration untouched.

| Document | What it covers |
| -- | -- |
| [architecture.md](architecture.md) | The annotated package map, who may call whom and what each interdiction buys, how a command designates a worktree |
| [commands.md](commands.md) | `internal/kernel/` — the engine's contract: commands, fields and forms, rules across fields, errors and their message catalogue, results, units of work (`Each`, sagas), the checks every command runs |
| [flow-layer.md](flow-layer.md) | `internal/flow/` — the three seams, the step model, unattended resolution, embedding, scheduling, events, testing a flow |
| [adding-a-mutation-command.md](adding-a-mutation-command.md) | End-to-end recipe for a new worktree-mutating command |
| [output.md](output.md) | What a command prints: the one question a block has to answer, the frame and the accent bar, the four levels, the two shapes of a conclusion, the glyph vocabulary, `--quiet` |
| [run-addressing.md](run-addressing.md) | Named URLs: proxy vs redirection vs public port, and what `addressing` writes into a `.env` |
| [shared-services.md](shared-services.md) | `scope = "shared"`: one instance for the repository, one namespace per worktree, and why the job table is the reference count |
| [lint.md](lint.md) | `make lint` and what each gate holds: the `archlint` rules, the exception files, the pre-commit hook, the rules considered and left out |
| [changelog.md](changelog.md) | How to write `CHANGELOG.md`: the release template, the rules, and how a section becomes the GitHub release notes |

For the coding standards themselves (immutability, struct params, constants, comment density), see [`CLAUDE.md`](../../CLAUDE.md) and the `go-cli` skill in `.claude/skills/go-cli/SKILL.md`.

To see a change working in the real binary — an isolated sandbox driven with tmux, or recorded with VHS, optionally before/after — use the `wtm-sandbox` skill (`.claude/skills/wtm-sandbox/`). To open a pull request (base branch from `wtm tree`, that terminal proof attached with `gh --attach`, body template), use the `open-pr` skill (`.claude/skills/open-pr/`).
