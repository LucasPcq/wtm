# Developer documentation

Reference documentation for people (and agents) working **on** wtm. It describes the
code as delivered, not the design that preceded it.

> The rest of `docs/` is **generated** by `tools/gendocs` from the Cobra command tree
> (`make docs`) and must never be hand-edited. `docs/dev/` is hand-written and is the
> only manual content under `docs/`; gendocs only writes `wtm_*.md` at the root of
> `docs/`, so this subdirectory survives a regeneration untouched.

| Document | What it covers |
| -- | -- |
| [architecture.md](architecture.md) | The layers, who may call whom, and what each interdiction buys |
| [flow-layer.md](flow-layer.md) | `internal/flow/` — the three seams, the step model, the flow diagrams, one flow across three surfaces |
| [adding-a-mutation-command.md](adding-a-mutation-command.md) | End-to-end recipe for a new worktree-mutating command |
| [output.md](output.md) | What a command prints: the one question a block has to answer, the frame and the accent bar, the four levels, the two shapes of a conclusion, the glyph vocabulary, `--quiet` |
| [run-addressing.md](run-addressing.md) | Named URLs: proxy vs redirection vs public port, and what `addressing` writes into a `.env` |

For the coding standards themselves (immutability, struct params, constants, comment
density), see [`CLAUDE.md`](../../CLAUDE.md) and the `go-cli` skill in
`.claude/skills/go-cli/SKILL.md`.
