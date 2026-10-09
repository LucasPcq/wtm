# The gates — `make lint` and what it holds

`make lint` is the mechanical half of `CLAUDE.md`. Every rule in it exists because a reviewer would otherwise have to hold the layer rules in their head on every PR, and the ones nobody holds are the ones that drift. Read this before adding a rule, an exception, or arguing that a check is wrong.

```
make lint         # fmt + vet + arch + dead + staticcheck — all gating
make test         # go test ./... -race -count=1
make dupl         # clone report, informative only
make dead-strict  # deadcode without -test: code only a test still reaches, informative only
```

| Check | Catches | Why staticcheck cannot |
| -- | -- | -- |
| `fmt` | unformatted files | it is not a formatter, and this fails rather than rewrites: a formatting fix belongs in the commit that caused it |
| `vet` | the stdlib's own suspicions | — |
| `arch` (`tools/archlint`) | the project's own rules, below | it checks a package against itself, and knows nothing about this project's layers |
| `dead` (`deadcode`) | functions no path reaches, **test paths included** | it reports the unused *within* a package; a function exported and called by nobody is invisible to it |
| `staticcheck` | the rest | — |

## `tools/archlint`

Each rule is a `golang.org/x/tools/go/analysis` Analyzer resolved by type — an aliased import names the same object as a plain one. A finding prints `file:line:col: [rule] why`.

| Rule | Checks |
| -- | -- |
| `layers` | the import graph of [architecture.md](architecture.md#who-may-call-whom), from the `layers` table |
| `domain` | `internal/domain` declares types, errors and constants only — no function |
| `servicedag` | each `service/x → service/y` import against `serviceEdges` |
| `daemonblind` | the daemon — `service/process` and `service/proxy` — imports nothing that runs git and only allow-listed `infra/` |
| `styles` | only `internal/styles` instantiates a `lipgloss.Style` |
| `typeassert` | a type assertion without comma-ok |
| `yesflag` | a command that reads the interactive gate (`shared.Interactive`) without offering `--yes` (`shared.AddYesFlag`) |
| `chokepoint` | a service mutator called from anywhere but `internal/flow/` |
| `metawriter` | an exported function of `service/worktree` that reaches `writeMetadata`/`purgeState` is in the mutators table — the table is complete by construction |
| `emits` | a `flow/` package calling a mutator publishes that mutator's event, and has a test recording it (`flowtest.Recorder`) |
| `publish` | `process.Publish` is called from `service/events` only; the `flow` seam's `Publish` from `internal/flow/` only |
| `glyph`, `tuistyle`, `mutedline`, `fontcover` | the output vocabulary, below |

### The output vocabulary

The glyph table was written down and the surface diverged anyway — sixty commands, five renderings of "nothing to do", `!` alone rendered as a filled chip. Four rules hold the parts a table cannot say (the reasoning is in [output.md](output.md#three-rules-that-make-the-vocabulary-hold)). The first three run only over the layers that put glyphs on a screen (`surface/cli/render`, `styles`, `surface/tui`): `rules/` and `service/` are left out because `=` and `!` are ordinary bytes to an env parser or a pnpm workspace pattern, and a check that cannot tell those apart is one people work around. `fontcover` runs over every string, since the runes it is about are declared in `domain`.

| Rule | Catches |
| -- | -- |
| `glyph` | a vocabulary rune written as a literal — `"✓"`, `"!"`, `"→"` … — instead of its `domain` constant, which is how a seventh glyph appears and how an existing one takes a second meaning |
| `tuistyle` | `styles.Badge*` or `styles.Dashboard*` used from `internal/surface/cli/render`: a badge is a widget, and its padding made an attention line two columns wider than the failure line under it |
| `mutedline` | `Message(w, styles.Muted.Render(x))` — a bare line muted whole, which is the `=` register with its glyph filed off |
| `fontcover` | a non-letter rune missing from common monospace fonts, in any string of `internal/` — the terminal borrows it from a wider fallback face and it eats the space after it. The allowed set, `fontSafe`, was measured over thirteen fonts; `fontLegacy` holds the runes that predate the rule and may only shrink |

### Adding a rule

`tools/archlint` is where a new architectural rule goes. Its `layers` table is the dependency graph written once, so a new dependency between two layers is a deliberate edit to that table rather than something that lands unnoticed. Each rule is tested with `analysistest` against its own fixtures, one `txtar` archive per rule (`tools/archlint/testdata/<rule>.txtar`, a section per file named by its import path). The driver loads the packages once per target system (`darwin`, `linux`, `freebsd`), and a test checks that together they compile every file of the tree, so a build-constrained file is never skipped; `-warn`, `.archlint-migrating` and `fontLegacy` are applied after every analyzer has run. Adding a rule there is cheaper than adding a paragraph to `CLAUDE.md`, and it is the only kind of rule that survives.

A rule belongs in `make lint` when breaking it breaks something — a layer, a refusal, a surface that can no longer run a flow. A rule about how code reads belongs in review.

## Exceptions

- **`.deadcode-ignore`** — one regex per line **with its reason**: code reachable by a route the analysis cannot follow (so far, a method satisfying an interface asserted on an `any`). Anything unlisted fails.
- **`.archlint-migrating`** — `<rule> <path regex> <sites>` lines for what predates a rule; they report as `(migrating)` without failing. **It may only shrink**, and that is checked: each entry — like each rune of `fontLegacy` — records how many sites it covers, one site more fails `make lint`, and a count higher than needed is reported as a note to lower it. A new entry is a decision to take knowingly and belongs in a ticket, never a way to get a commit past the linter. It holds no `chokepoint` entry today: a new one is a regression, not a migration.

`make dupl` is deliberately outside `lint`: a clone is a judgement call. Two parallel families over unrelated types — `flow/run/job` and `flow/run/profile` — read better duplicated than behind a generic, so the report informs a review rather than gating one.

## The pre-commit hook

`.claude/hooks/pre-commit-gates.sh` runs `go mod tidy` (on a copy) and `make lint` on every `git commit`, as a Claude Code `PreToolUse` hook, and blocks the commit when either fails. It reads the same Makefile a human does — which is the whole reason the gates live there. It does **not** run the tests: at ~70 s that is a gate people disable, and the CI and the `build-validator` subagent run them. `WTM_SKIP_GATES=1 git commit …` gets past it for the case where the gate is itself wrong.

## Considered and left out

So it is not re-proposed:

| Rule | Measured | Why not |
| -- | -- | -- |
| Structs for 2+ inputs (`CLAUDE.md` §2) | 546 functions at 2+ non-carrier inputs, 17 at 4+ | A count cannot tell a related pair from a carrier pair. A gate at 2 fires on most of `surface/cli/render/`; one at 4 still fires on a syscall wrapper whose arity is the ABI, and on list widgets whose `renderRow` gains nothing from a struct. Encoding a rule that cannot tell the cases apart teaches people to work around the linter. It stays a review rule |
| Comment density (`CLAUDE.md` §8) | — | The ceiling is met as easily by deleting the comments that earn their place as the ones that do not, so the number measures the wrong thing. The rule itself needs rethinking before anything can check it |
| A `label  value` format string hand-aligning its own column | 1 match in `domain` (`DetailReviewDecisionFmt`), and it is a dashboard fragment, not a block | The regex that finds a hand-aligned label finds the one legitimate use too. `Announce` owning the alignment is the fix; a gate over it would teach people to space their labels differently rather than to use the helper |
