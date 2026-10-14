# The gates — `make lint` and what it holds

`make lint` is the mechanical half of `CLAUDE.md`. Every rule in it exists because a reviewer would otherwise have to hold the layer rules in their head on every PR, and the ones nobody holds are the ones that drift. Read this before adding a rule, an exception, or arguing that a check is wrong.

```
make lint         # fmt + vet + arch + dead + sumtype + staticcheck — all gating
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
| `sumtype` (`go-sumtype`) | a type switch over a sealed interface (`kernel.Error`, `kernel.Value`) that misses a variant and has no default | Go has no sum types: nothing else knows the interface is closed. The `//go-sumtype:decl <Type>` line stands alone, outside the doc comment, which `gofmt` would rewrite |
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
| `metawriter` | an exported function of `service/worktree` that reaches `Update` or `Purge` of `store.Meta` (the one writer of `meta.json`, [store.md](store.md)) is in the mutators table — the table is complete by construction |
| `emits` | a `flow/` package calling a mutator publishes that mutator's event, and has a test recording it (`flowtest.Recorder`) |
| `publish` | `process.Publish` is called from `service/events` only; the `flow` seam's `Publish` from `internal/flow/` only; `dispatch.Publisher`'s from `dispatch/` only; and `command/`, `query/` and the engine's services call no `Publish` at all: a mutator returns what it changed, the command reports it, `dispatch` publishes it |
| `statewrite` | `os.WriteFile`, `os.Create`, `os.Rename`, `os.Remove`, `os.RemoveAll`, or an `os.OpenFile` whose flags may write (a flag the checker cannot fold counts as one), outside `infra/`, as a call or a function value: every other write goes through `infra/fs` ([state-files.md](state-files.md)). The writers that stay outside for good are in `stateWriteExempt`, each with its reason: log rotation, the daemon's stale socket, the binary swap of `upgrade`, `agents install`, `schema dump`, the shell's cd handshake, the proxy's launchd plist, the test doubles |
| `stateowner` | `fs.UpdateFile` or `fs.RemoveDir` called outside `store/`: each state file has one owner, behind an interface a fake doubles ([store.md](store.md)) |
| `writesink` | a reader or a mutator, never both. A function that reaches a write sink is a **mutator**: the sinks are `os`'s writes, `fs.UpdateFile` / `RemoveDir` / `ReplaceFile` / `OpenLog`, a program started (`proc.Command`, `proc.GroupCommand`, `exec.Command`) and every method of `git.Writer`, on the interface and on `git.Repo`. Above `infra/` nothing is listed: the mark is inferred and carried across packages as an analysis fact, through calls, closures and function values, and a method of an interface is a mutator when a type of its package writes in it (`store.Meta.Update` as `store.MetaDisk.Update`). `infra/` is classified by its ports, since its readers run git too. Refused: any write reached from `query/`, or from a function of the engine's contract other than `Apply` (`Observe`, `Choices`, `Plan`, `Skip`, a query's `Run`…), in place or named; and a mutator of the engine's services (`engineServices`) called from anywhere but `command/`, another service or `flow/` ([services.md](services.md)) |
| `pkgdoc` | a package of `service/`, `infra/` or `store/` without a package doc: it says its one responsibility in a sentence, and one that needs an "and" is two packages. Whether the sentence is good stays a review matter |
| `rootfreeze` | a file, or an exported symbol, at the root of `rules/` or of `domain/`: the roots only shrink, new code goes into the sub-package of its subject (`rules/worktree`, `domain/statefile`…), and a presentation rule into the surface that uses it |
| `engine` | `command/` and `query/` never import `dispatch` or `surface/`, `surface/` never imports `command/` or `query/`, and `command/` never calls `context.Background()` or `context.TODO()`: a command is reached through `dispatch` only, never calls it, and does I/O only with the `ctx` it is handed ([dispatch.md](dispatch.md)) |
| `catalog` | a `kernel.Code` declared in `kernel/`, `dispatch/`, `command/` or `query/` — a typed constant, or a conversion of a constant — with no key in a `text.Messages` of its package (the kernel's codes are checked in `kernel/text`, handed over as a package fact), and a message keyed by a bare string ([commands.md](commands.md#messages)) |
| `gitexec` | a program started as `"git"` (`exec.Command`, `exec.CommandContext`, `proc.Command`, `proc.GroupCommand`) outside `infra/git`: the engine reaches git through `git.Repo` ([git.md](git.md)). Its one permanent exception, in `gitExecExempt`, is `testutil`, whose helpers build real repositories |
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

`tools/archlint` is where a new architectural rule goes. Its `layers` table is the dependency graph written once, so a new dependency between two layers is a deliberate edit to that table rather than something that lands unnoticed. Each rule is tested with `analysistest` against its own fixtures, one `txtar` archive per rule (`tools/archlint/testdata/<rule>.txtar`, a section per file named by its import path). The driver loads the packages once per target system (`darwin`, `linux`, `freebsd`), and a test checks that together they compile every file of the tree, so a build-constrained file is never skipped; `-warn`, `.archlint-migrating` and `fontLegacy` are applied after every analyzer has run. A rule that needs to know what another package does carries it as an analysis fact (`writesink`'s `mutatorFact`): the driver loads every dependency with its syntax for that, and the checker runs the rule on each package before the packages that import it. Adding a rule there is cheaper than adding a paragraph to `CLAUDE.md`, and it is the only kind of rule that survives.

A rule belongs in `make lint` when breaking it breaks something — a layer, a refusal, a surface that can no longer run a flow. A rule about how code reads belongs in review.

## Exceptions

- **`.deadcode-ignore`** — one regex per line **with its reason**: code reachable by a route the analysis cannot follow (a method satisfying an interface asserted on an `any`), and the foundations of the command engine no command calls yet (`cli.Bind`, until the pilot `create` of step 3 of LUC-283 removes its entry). Anything unlisted fails.
- **`.archlint-migrating`** — `<rule> <path regex> <sites>` lines for what predates a rule; their sites are counted without failing, and `go run ./tools/archlint -tracked` lists them as `(migrating)`. **It may only shrink**, and that is checked: each entry — like each rune of `fontLegacy` — records how many sites it covers, one site more fails `make lint`, and a count higher than needed is reported as a note to lower it. A new entry is a decision to take knowingly and belongs in a ticket, never a way to get a commit past the linter. Each group of entries says the step of LUC-283 that empties it:
  - no `chokepoint`, `writesink`, `publish` or `pkgdoc` entry: a new one is a regression, not a migration;
  - `statewrite`: the `exec` log and the daemon's lock (the run module), the directories `relocate` and `extract` remove (their commands);
  - `stateowner`: the state files not yet in `store/` — `run.toml`, `pending-removals.toml`, `repos.json`, the renders of the inits, the compose patches, `selfupdate`'s `state.json`;
  - `gitexec`: the files at the root of `infra/` that run git for the commands still on `flow/`, one per file with its count of calls;
  - `rootfreeze`: one line per file at the root of `rules/` and `domain/`, counting the file and each exported symbol in it. A migration moves what its command uses into the sub-package of its subject and lowers the count; a new file has no line and fails.
- **A permanent exception** is not a migration and never goes in `.archlint-migrating`: it is a table in the rule's own file with a reason per entry (`stateWriteExempt`, `gitExecExempt`, `daemonInfra`, `fontSafe`).
- **Armed as the engine grows.** The rules of the lower layers hold the new code entirely and the old code through `.archlint-migrating`. `writesink` arms the engine's services one by one: `engineServices` lists `service/gitwt`, `identity`, `runenv` and `hooks`, and a service joins it when its command moves onto the engine; `writesink_test.go` checks the classification of the four against [services.md](services.md). `chokepoint` keeps the hand-written table of the mutators of the code on `flow/`, and leaves with it.

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
