# The gates — `make lint` and what it holds

`make lint` is the mechanical half of `CLAUDE.md`. Every rule in it exists because a reviewer would otherwise have to hold the layer rules in their head on every PR, and the ones nobody holds are the ones that drift. Read this before adding a rule, an exception, or arguing that a check is wrong.

```
make lint         # fmt + vet + arch + dead + sumtype + next-lint + staticcheck — all gating
make test         # go test ./... -race -count=1
make lint-next    # the same gates over next/ and tools/ only (archlint whole) — for a change that leaves internal/ alone
make test-next    # go test ./next/... ./tools/... -race -count=1
make dupl         # clone report, informative only
make dead-strict  # deadcode without -test: code only a test still reaches, informative only
```

| Check | Catches | Why staticcheck cannot |
| -- | -- | -- |
| `fmt` | unformatted files | it is not a formatter, and this fails rather than rewrites: a formatting fix belongs in the commit that caused it |
| `vet` | the stdlib's own suspicions | — |
| `arch` (`tools/archlint`) | the project's own rules, below | it checks a package against itself, and knows nothing about this project's layers |
| `dead` (`deadcode`) | functions no path reaches, **test paths included** | it reports the unused *within* a package; a function exported and called by nobody is invisible to it |
| `sumtype` (`go-sumtype`) | a type switch over a sealed interface (`fs.Change`, `envcopy.Warning`) that misses a variant and has no default | Go has no sum types: nothing else knows the interface is closed. The `//go-sumtype:decl <Type>` line stands alone, outside the doc comment, which `gofmt` would rewrite |
| `staticcheck` | the rest | — |

## `tools/archlint`

Each rule is a `golang.org/x/tools/go/analysis` Analyzer resolved by type — an aliased import names the same object as a plain one. A finding prints `file:line:col: [rule] why`.

| Rule | Checks |
| -- | -- |
| `layers` | the import graph of [architecture.md](architecture.md#who-may-call-whom), from the `layers` table. A row's entry names a layer or a part of one (`surface/cli/render`), and a package of `internal/` with no row is a finding: a new layer is declared before it imports anything. `rules/` imports no stdlib package that reaches outside the process (`os` and below, `io/ioutil`, `net` and below but `net/url` and `net/netip`, `syscall`) |
| `domain` | `internal/domain` declares types, errors and constants only — no function |
| `servicedag` | each `service/x → service/y` import against `serviceEdges`: a daemon that starts importing git, or a cycle in the making, is an edit to the table rather than an import nobody saw |
| `daemonblind` | the daemon — `service/process` and `service/proxy` — imports nothing that runs git and only allow-listed `infra/`: it outlives the command that started it and serves every worktree of every repository, so it is handed what it needs and never reads a repository's config; `service/proxy` is served from inside it, and `service/events` is refused already so the event broker never becomes schema-aware by accident |
| `styles` | only `internal/styles` instantiates a `lipgloss.Style` |
| `typeassert` | a type assertion without comma-ok |
| `yesflag` | a command that reads the interactive gate (`shared.Interactive`) without offering `--yes` (`shared.AddYesFlag`) |
| `chokepoint` | a service mutator called from anywhere but `internal/flow/`, whatever the caller's layer: a command, the dashboard or another service reaching one directly is a mutation no flow sees, so no second surface can run it and no event can report it. A call inside the mutator's own package is its implementation, not an escape. Each entry lists the events its flow publishes after the call (`emits`), none for a change that is not part of a worktree's identity |
| `metawriter` | an exported function of `service/worktree` that reaches `Update` or `Purge` of `store.Meta` (the one writer of `meta.json`, [store.md](store.md)) is in the mutators table — the table is complete by construction |
| `emits` | a `flow/` package calling a mutator publishes that mutator's event, and has a test recording it (`flowtest.Recorder`): a flow that calls a mutator and none of its emitters has changed a worktree no consumer will hear about. archlint loads packages without their tests, so the test files are read from disk |
| `publish` | `process.Publish` is called from `service/events` only, and the `flow` seam's `Publish` from `internal/flow/` only: every event goes out stamped and versioned, and an event published outside a flow reports a change no flow made, which no other surface could replay |
| `statewrite` | `os.WriteFile`, `os.Create`, `os.Rename`, `os.Remove`, `os.RemoveAll`, or an `os.OpenFile` whose flags may write (a flag the checker cannot fold counts as one), outside `infra/`, as a call or a function value: every other write goes through `infra/fs` ([state-files.md](state-files.md)). The writers that stay outside for good are in `stateWriteExempt`, each with its reason: log rotation, the daemon's stale socket, the binary swap of `upgrade`, `agents install`, `schema dump`, the shell's cd handshake, the proxy's launchd plist, the test fixtures. A file truncated in place by `os.WriteFile` or `os.Create` is half a file to whoever reads it meanwhile, and a hand-rolled rename or removal takes no lock |
| `stateowner` | `fs.UpdateFile` or `fs.RemoveDir` called outside `store/`: each state file has one owner, behind an interface ([store.md](store.md)): called from anywhere, a state file has as many owners as callers, each with its own idea of its schema |
| `pkgdoc` | a package of `service/`, `infra/` or `store/` without a package doc: it says its one responsibility in a sentence, and one that needs an "and" is two packages. Whether the sentence is good stays a review matter |
| `rootfreeze` | a file, or an exported symbol, at the root of `rules/` or of `domain/`: the roots only shrink, new code goes into the sub-package of its subject (`rules/statefile`, `domain/statefile`…), and a presentation rule into the surface that uses it |
| `glyph`, `tuistyle`, `mutedline`, `fontcover` | the output vocabulary, below |

### The output vocabulary

A glyph table alone does not hold a surface of sixty commands: "nothing to do" gets several renderings, and a glyph takes a second meaning. Four rules hold the parts a table cannot say (the reasoning is in [output.md](output.md#three-rules-that-make-the-vocabulary-hold)). The first three run only over the layers that put glyphs on a screen (`surface/cli/render`, `styles`, `surface/tui`): `rules/` and `service/` are left out because `=` and `!` are ordinary bytes to an env parser or a pnpm workspace pattern, and a check that cannot tell those apart is one people work around. `fontcover` runs over every string, since the runes it is about are declared in `domain`.

| Rule | Catches |
| -- | -- |
| `glyph` | a vocabulary rune written as a literal — `"✓"`, `"!"`, `"→"` … — instead of its `domain` constant, which is how a seventh glyph appears and how an existing one takes a second meaning |
| `tuistyle` | `styles.Badge*` or `styles.Dashboard*` used from `internal/surface/cli/render`: a badge is a widget, and its padding made an attention line two columns wider than the failure line under it |
| `mutedline` | `Message(w, styles.Muted.Render(x))` — a bare line muted whole, which is the `=` register with its glyph filed off |
| `fontcover` | a non-letter rune missing from common monospace fonts, in any string of `internal/` and `next/` — the terminal borrows it from a wider fallback face and it eats the space after it. The allowed set, `fontSafe`, was measured over thirteen fonts (Cascadia Code, DejaVu Sans Mono, Fira Code, Hack, IBM Plex Mono, Inconsolata, JetBrains Mono, Roboto Mono, Source Code Pro, Ubuntu Mono, Menlo, Monaco, SF Mono); `fontLegacy` holds the runes that predate the rule and may only shrink. Box drawing and block elements pass: terminals draw them as their own sprites |

### `next/`

The driver loads `internal/` and `next/` in one run (`go run ./tools/archlint [root…]`, both by default). The rules written for `internal/` act on its packages only (`internalOnly` in `analyzers.go`); `typeassert` and `fontcover`, which hold for any Go code of the project, act on both. The rules below are `next/`'s own, from [`next/README.md`](../../next/README.md) and the `go-style` skill. The driver loads no tests, so the rules about tests read the `_test.go` files from disk:

| Rule | Catches |
| -- | -- |
| `next` | an import outside the package's row of the `next/` import table (`domain/`, `service/<name>`, `service/<name>test`, `command/<name>`, `command/<name>/cli`, `adapter/<name>`, `adapter/<name>test`, `ui/<name>`, `ui/<name>test`, `api`, `cmd/wtm-next`), a package in no row (the root of `next/` and any `cmd/` but `cmd/wtm-next` included), a name ending in `test` under `service/`, `adapter/` or `ui/` with no package of the name before the suffix beside it, an import of `internal/` from a source or a test file, a directory holding only tests (the driver loads no tests, so no other check would read it), and a fake (`<layer>/<name>test`) imported from anything but a test or the fake's own package |
| `codeprefix` | a code a command declares (the constant value given to a `Code:` field, or to a `Code…`/`code…` constant or variable) in `command/<name>`, its `cli` or their sub-packages that does not start with the command's `Name` and a dot (`fast-forward.`, even in `command/fastforward`): a code shared by several commands is a `domain` constant. A code passed by position or through a helper is not seen |
| `subtestname` | a subtest named with a space (Google "Subtest names": short, like an identifier, a description in a field of its own): a string literal `t.Run` or `b.Run` takes as its name, on a `*testing.T` or `*testing.B` the function holds, whatever the second argument. The name is followed within the test function through a concatenation, a local variable assigned a literal, and the innermost `range` around the call over a literal table (a map's key, a slice's element, a row's field, keyed or by position). The rule reads every `_test.go`, the sources of a test kit (`<layer>/<name>test`, whose `operationtest.Contract` every command runs) too, and the value of a `name` field in any composite literal of a test, a branch's name included. A table declared in another file, a name built by `fmt.Sprintf` or a helper is not seen |
| `testlayout` | a top-level declaration of a `_test.go` that is not a `Test…`, `Benchmark…`, `Fuzz…` or `Example…` function (another function, a method, a type, a variable holding a function literal), outside `fixture_test.go` (the example, the fakes), `helpers_test.go` (the shared helpers) and `export_test.go` (what an external test reaches of the package, the stdlib idiom). A constant or a table variable stays beside its test |
| `portfile` | a `port.go` that declares anything but interface types and function types (`extract.Shield`): a struct, a named basic type, an alias, a constant, a variable or a function. The types a port takes and returns sit beside the code that owns them |
| `envread` | a stdlib function that reads the environment (`os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.ExpandEnv`, `os.UserHomeDir`, `os.UserConfigDir`, `os.UserCacheDir`, `syscall.Getenv`, `syscall.Environ`), called or taken as a value, outside the edges that read it: `ui/cli` (the root reads it once, `ReadEnv`), `ui/output` (the streams' `TERM`), `cmd/wtm-next` (`$WTM_STATE_DIR`) and `adapter/*` (what its own tool needs, `os.UserConfigDir` included). Tests are not checked: a test that re-executes its binary as a helper process reads its own variable |
| `slicefiles` | an entry at the root of `command/<name>` other than `usecase.go`, `port.go`, `rules.go`, `questions.go`, their tests (`*_test.go`), `testdata/` (the goldens of a root test) and `cli/`: a decision goes in `rules.go`, a question in `questions.go`, what two commands share in a service. Dotfiles are ignored |

Comments in `next/` follow Go's doc-comment conventions (the `go-style` skill): `staticcheck` runs every check there, the naming and doc-comment ones included (`next/staticcheck.conf`, which staticcheck reads for the packages under it), and `make next-lint`, part of `make lint`, runs revive with `next/revive.toml` (`exported`, `package-comments`, naming and error-flow rules).

### Adding a rule

`tools/archlint` is where a new architectural rule goes. Its `layers` table is the dependency graph written once, so a new dependency between two layers is a deliberate edit to that table rather than something that lands unnoticed. Each rule is tested with `analysistest` against its own fixtures, one `txtar` archive per rule (`tools/archlint/testdata/<rule>.txtar`, a section per file named by its import path). The driver loads the packages once per target system (`darwin`, `linux`, `freebsd`), and a test checks that together they compile every file of the tree, so a build-constrained file is never skipped; `-warn`, `.archlint-migrating` and `fontLegacy` are applied after every analyzer has run. Adding a rule there is cheaper than adding a paragraph to `CLAUDE.md`, and it is the only kind of rule that survives.

A rule belongs in `make lint` when breaking it breaks something — a layer, a refusal, a surface that can no longer run a flow. A rule about how code reads belongs in review.

## Exceptions

- **`.deadcode-ignore`** — one regex per line **with its reason**: code reachable by a route the analysis cannot follow (a method satisfying an interface asserted on an `any`). Anything unlisted fails. Its ceiling is `deadcodeCeiling` in `tools/archlint/ceiling.go`: a pattern it does not list fails `make lint`.
- **`.archlint-migrating`** — `<rule> <path regex> <sites>` lines for what predates a rule; their sites are counted without failing, and `go run ./tools/archlint -tracked` lists them as `(migrating)`. **It may only shrink**, and that is checked twice. Each entry — like each rune of `fontLegacy` — records how many sites it covers: one site more fails `make lint` and lists every site the entry covers, the new one among them; a count higher than needed is reported as a note to lower it. And the file never grows past its ceiling, `migratingCeiling` in `tools/archlint/ceiling.go`: an entry the ceiling does not list, or a count above its own, fails. Shrinking the file needs no edit of the ceiling; growing it is an edit to archlint, a decision taken knowingly in a ticket and in review, never a way to get a commit past the linter. Its groups of entries:
  - no `chokepoint`, `publish` or `pkgdoc` entry: a new one is a regression, not a migration;
  - `statewrite`: the `exec` log, the daemon's lock, the directories `relocate` and `extract` remove;
  - `stateowner`: the state files not yet in `store/` — `run.toml`, `pending-removals.toml`, `repos.json`, the renders of the inits, the compose patches, `selfupdate`'s `state.json`;
  - `rootfreeze`: one line per file at the root of `rules/` and `domain/`, counting the file and each exported symbol in it. Moving a symbol into the sub-package of its subject lowers the count; a new file has no line and fails.
- **A permanent exception** is not a migration and never goes in `.archlint-migrating`: it is a table in the rule's own file with a reason per entry (`stateWriteExempt`, `daemonInfra`, `fontSafe`).

`make dupl` is deliberately outside `lint`: a clone is a judgement call. Two parallel families over unrelated types — `flow/run/job` and `flow/run/profile` — read better duplicated than behind a generic, so the report informs a review rather than gating one.

## The pre-commit hook

`.claude/hooks/pre-commit-gates.sh` runs `go mod tidy` (on a copy) and `make lint` on every `git commit`, as a Claude Code `PreToolUse` hook, and blocks the commit when either fails. It reads the same Makefile a human does — which is the whole reason the gates live there. It does **not** run the tests: at ~70 s that is a gate people disable, and the CI and the `build-validator` subagent run them. `WTM_SKIP_GATES=1 git commit …` gets past it for the case where the gate is itself wrong.

## Considered and left out

So it is not re-proposed:

| Rule | Measured | Why not |
| -- | -- | -- |
| Structs for 2+ inputs (`CLAUDE.md` §2) | 546 functions at 2+ non-carrier inputs, 17 at 4+ | A count cannot tell a related pair from a carrier pair. A gate at 2 fires on most of `surface/cli/render/`; one at 4 still fires on a syscall wrapper whose arity is the ABI, and on list widgets whose `renderRow` gains nothing from a struct. Encoding a rule that cannot tell the cases apart teaches people to work around the linter. It stays a review rule |
| A `label  value` format string hand-aligning its own column | 1 match in `domain` (`DetailReviewDecisionFmt`), and it is a dashboard fragment, not a block | The regex that finds a hand-aligned label finds the one legitimate use too. `Announce` owning the alignment is the fix; a gate over it would teach people to space their labels differently rather than to use the helper |
