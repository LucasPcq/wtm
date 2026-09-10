# CLAUDE.md — Go CLI Development Principles

This file defines the mandatory coding standards for this project. All contributions must comply. When in doubt, consult the `go-cli` skill.

**Self-maintaining docs:** When a structural or architectural decision changes (new package, renamed layer, new dependency, new convention), update this file and/or the relevant skills (`go-cli`, `build-validator`) in the same session. Standards must always reflect the actual codebase.

**User-facing agent skill:** `internal/commands/agents/assets/using-wtm.skill.md` is the skill shipped to end users so their LLM can drive the `wtm` CLI. Whenever a change alters the user-facing command surface or agent-relevant behavior (new/renamed command or flag, changed `--output json` shape, new failure/abort semantics, changed interactive-vs-non-interactive behavior), update this skill in the same session so it stays aligned with the released CLI. Skip purely internal refactors and TUI-only changes that don't affect how an agent invokes wtm.

**Docs & README:** the full command reference under `docs/` is **generated** from the Cobra command tree by `tools/gendocs` — never hand-edit it. The one exception is `docs/dev/`, hand-written developer documentation (architecture, the `flow/` layer, how to add a mutation command): gendocs only writes `wtm_*.md` at the root of `docs/`, so that subdirectory survives a regeneration. Keep it in step with the code the same way this file is. `README.md` is a lean guide (concepts + a grouped command-overview table linking into `docs/`), not a flag reference. Whenever a command or flag is **added, modified, or removed**:
1. run `make docs` to regenerate `docs/` (also runs automatically before `make release`);
2. if a command was added/renamed/removed, update the `README.md` overview table (grouped by the same sections as the root `--help`) and, if relevant, the Concepts or Configuration sections. Do **not** re-add per-command flag tables to the README — `wtm <cmd> --help` and `docs/` are the source of truth. This is mandatory alongside the agent skill above.

Use the fff MCP tools for all file search operations instead of default tools.

---

## 1. Immutability first

Prefer short variable declarations (`:=`) for values that do not change. Use `var` only for zero-value initialization or package-level declarations. If a block requires reassigning a variable, extract it into a function.

## 2. Structs for 2+ inputs

Any function or method that accepts 2 or more of its own inputs must take a single struct argument. Always initialize structs with named fields.

```go
// ❌
func Connect(host string, port int) error

// ✅
type ConnectParams struct {
  Host string
  Port int
}
func Connect(params ConnectParams) error
```

**"Its own inputs" excludes what the function carries rather than reads.** A destination, a cancellation, a test handle, the command being wired — `io.Writer`, `context.Context`, `testing.TB`, `*cobra.Command` — are plumbing every reader already knows, and folding them into a struct hides them instead of naming them. `output.Warning(w, text)` is the rule respected, not broken; so is a method whose receiver is the subject and whose single parameter is the question asked of it.

The hazard the rule is about is precise, and worth naming so the rule is applied where it bites rather than everywhere:

- **Misorderable neighbours.** `RenameJobRefs(cfg, from, to)` — two `string`s the compiler will happily let you swap, silently renaming the wrong way round. Two parameters of the same type are the strongest case for a struct, whatever the count.
- **A list that grew.** Four inputs are already unreadable at the call site, and a fifth is added without anyone noticing that a caller passed them in the wrong order. This is why every service and flow entry point in the tree takes `<Name>Params`.

A symmetric pair is the counter-case: `differ(a, b string)`, `MergeRunConfigs(a, b)`, `ClampIndex(index, length int)` — where the two are peers and the order is the meaning, a struct adds ceremony and removes nothing.

**This is a review rule, not a lint rule, and deliberately so.** It was measured before being left out of `make lint` (see section 11): 546 functions in the tree take 2 or more non-carrier inputs, 17 take 4 or more. A gate at 2 would fire on most of `output/`; a gate at 4 still fires on a syscall wrapper whose arity *is* the ABI, and on list widgets whose `renderRow` gains nothing from a struct. A count cannot tell a related pair from a carrier pair, and encoding a rule that cannot is worse than holding it in review — it teaches people to work around the linter. Apply it when you see one of the two hazards above.

## 3. Shared types — no duplication

All types, enums, sentinel errors, and constants are defined once in `internal/domain/`. Import from there everywhere. Never copy-paste a type across packages.

Pure functions with no I/O (lookups, transforms, classification) live in `internal/rules/`, not in `internal/domain/` or `internal/service/`.

## 4. Validate all external input

Config files, CLI flags, and environment variables are validated at the boundary (in `config/` or at command entry). Use `go-playground/validator` struct tags or explicit guard clauses. Service layer receives only clean data.

## 5. Centralized constants — no magic strings or numbers

Every string key, exit code, flag name, env var name, and format identifier must be a named constant in `internal/domain/constants.go`.

```go
// ❌
os.Exit(1)
cmd.Flags().String("output", ...)

// ✅
const (
  ExitCodeError   = 1
  FlagOutput      = "output"
)
```

## 6. Early returns — no nesting

Every error or guard condition returns immediately. The happy path is last. Never nest `if` blocks; flatten with early returns.

## 7. No unsafe type assertions

Always use the comma-ok idiom for type assertions. Prefer typed interfaces and concrete structs over `any`/`interface{}`. Type at the source, not downstream.

```go
// ❌
s := v.(string)

// ✅
s, ok := v.(string)
if !ok {
  return fmt.Errorf("expected string, got %T", v)
}
```

## 8. Comments — the exception, not the rule

Aim for **near-zero** comments. A well-named function with a typed signature explains itself 99% of the time, and a comment that restates the code is noise that buries the few that matter. Encode the meaning in names and signatures first: `Skip func(Answers) (skip bool, reason string)` needs no prose, and a named result beats a line describing the second return value.

Write a comment only when the code cannot carry the information:
- **Why, never what**: a non-obvious decision, an ordering constraint, an invariant a reader would otherwise break
- A workaround, with its reference (issue URL or ticket)
- A package comment (`// Package x …`), one line
- Godoc on an exported symbol **only** when its name and signature leave a caller guessing — not systematically

Documenting a pattern or an architecture belongs in `docs/` or in this file, not in a header comment repeated across files. `internal/flow` is the reference for the density to aim for: 413 comment lines out of 4055, ~10%. Small files sit higher — a 20-line type whose whole point is one non-obvious decision is mostly that decision — so read the figure as a ceiling for a package, never as a quota per file.

**Migration:** the repo predates this rule, so it is applied as files are touched, not in one sweep. When you modify a file, bring the comments **in that file** into line — delete the ones that restate the code — in the same change.

## 9. Clean architecture layers

```
cmd/                          ← entry points, cobra setup only
internal/
  commands/                   ← flag wiring, delegates to flow/service (zero business logic)
    run/crud/                 ←   the preamble `run job` and `run profile` share: config,
                                  run.toml, the opt-in guard, and the two seams
    ui/                       ←   `wtm ui`: refuses JSON and a missing TTY, then hands off to tui/dashboard
  domain/                     ← types, errors, constants only (no methods, no functions)
  rules/                      ← pure functions (stdlib + domain only, no I/O)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/, plus the global config (config.GlobalPath)
  flow/                       ← the flow of each command, surface-independent (see below):
                                the vocabulary (Step, Session, Prompter, Presenter)
    decide/                   ←   branch/env decisions shared by the create-like flows
    envports/                 ←   settling a fresh .env's host ports onto the ones the
                                  worktree binds — shared by `create` and `extract`
    create/                   ←   `wtm create`: the run (create.go) + its questions (steps.go)
    clean/                    ←   `wtm clean`: the run (clean.go) + its questions (steps.go)
    reparent/                 ←   `wtm reparent`: the run (reparent.go) + its questions (steps.go)
    prune/                    ←   `wtm prune`: the run (prune.go) + its questions (steps.go)
    sync/                     ←   `wtm sync`: the run (sync.go) + its questions (steps.go)
    runlogs/                  ←   the jobs a surface shows (`Board`), their live streams,
                                  and the profile start sequence (reports events, not steps)
    run/                      ←   the `run` module's flows, mirroring its command tree:
      target/                 ←     the questions they share (worktree, job, profile,
                                    and the published-url step `run open` asks)
      urls/                   ←     where every address the module hands out is computed
      seam/                   ←     the daemon as a flow uses it: board, env, log dir,
                                    port prober, and the start sequence a surface drives
      up/ down/ start/        ←     one package per command, as everywhere else
      stop/ logs/ open/ url/
      list/                   ←     `run list`: which entry was picked and what to do to it
      job/ profile/           ←     CRUD on run.toml's declarations, one package per group
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   git worktree operations (create, list, remove)
    env/                      ←   .env provisioning (create) + drift reconciliation (`wtm env`, sync.go)
    hooks/                    ←   on_create hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    integration/              ←   third-party adapters: handing a URL to the desktop's
                                  own opener (editor/agent detection lives in detect/)
    proxy/                    ←   the run proxy: the host→job routing table and the
                                  loopback server the daemon owns (`[proxy]`)
    detect/                   ←   auto-detection (base branch, env files, package manager)
  output/                     ← format and print results (zero decision logic)
  styles/                     ← all Lipgloss styles (only package allowed to instantiate lipgloss.Style)
  tui/                        ← Bubbletea models (zero business logic, rendering only)
    flowui/                   ←   runs a flow.Session as a wizard (the only translator
                                  between flow.Step and components.Step)
    dashboard/                ←   `wtm ui`: the full-screen worktree dashboard, the second
                                  surface over flow/ (its own Prompter/Presenter, mouse
                                  zones via bubblezone). It also hands the terminal to
                                  runview (`handoff.go`) for the run flows that draw
    runview/                  ←   a job's raw PTY output replayed through a terminal
                                  emulator (`github.com/charmbracelet/x/vt`)
  infra/                      ← I/O, git exec, filesystem wrappers
```

**Hard rules:**
- `commands/` has zero business logic
- `domain/` has types, errors, and constants only — no methods, no free functions
- `rules/` imports only stdlib and `internal/domain` — no I/O, no side effects
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`
- `flow/` imports **only** `internal/service/`, `internal/rules/`, `internal/domain/` and the stdlib — never cobra, bubbletea or lipgloss, and never `output/`, `tui/`, `config/` or `commands/`. It therefore cannot reach `infra/` either: add a thin `service/` wrapper instead (e.g. `worktree.FindByBranch`, `worktree.ListAll`)

**The frame (vertical padding and the accent bar):** centralized in one place. Each command frames its human output **exactly once** with `output.Frame` (or the `output.FrameStart`/`output.FrameEnd` pair for streaming/split-stream); helpers and formatters return **raw** bodies (no outer blank lines). `Frame` hands its body a writer and the body must write to **that** one, never to the writer `Frame` was given: that is what puts the accent bar (`┃`, column zero, `output.Barred`) on every line of the block, in the one place the padding is already applied. A streaming pair wraps its own body writer in `output.Barred`. The bar is what marks a block as wtm's own in a scrollback of `git`, `pnpm` and `docker` — so it goes on a terminal only: a pipe, a CI log or a redirection gets the bare text, and a grep over that log never has to know about it. JSON (`--output json`) and machine output (shell-eval: `resolve` success, `shell-init`, `run url`, `run export`) are never framed and therefore never barred. Route on `rules.IsHumanFormat(format)`. Progress is **not** framed and not barred — a spinner writes on the raw stream and is erased, and the bar marks what stays. A run's mid-run output that *does* stay — a status line, a hook phase's result lines — is one block of its own, joined through `shared.OpenBlock`; the conclusion is the second block, which is what "exactly once" counts. Whether a block is already open is a property of the **surface**, tracked in `output` (`BlockOpen`, and `FrameStart` writing no blank on a surface already at a boundary), not of the presenter: the frame beside a run's block is written by code that never sees it, and stdout and stderr are one surface when both are the same terminal. A flow therefore never reports from inside a `Stage` — a spinner owns the stream and repaints over the line. `HookView` is the one place the two meet: it bars every line it composes and writes its cursor moves raw, since a bar drawn before one survives the erase a column off. See the `go-cli` skill (Output section) for the full convention.

**What a block of output has to earn.** The frame is settled above; *whether a block prints at all* is the rule that was missing, and it is the one that decides how a command reads. A block earns its place when it changes what the reader does next. Three consequences, in the order they bite: The four levels a block may take (a flat line for an act, a table for an inventory, a pill-titled block for a state you come back to, a bordered callout for something still to act on), the two shapes of a conclusion (the act and the counted readout), the glyph vocabulary, `--quiet` and the two-stream/three-register split are all in [`docs/dev/output.md`](docs/dev/output.md) — read it before adding a command or changing what one prints.

**Three rules hold the glyph vocabulary together**, and they are the half that was missing when the table alone let sixty commands diverge. The glyph carries the **only colour on its line** — the message stays in the terminal's own foreground, so a block reads as text with a margin of signals; `=` and `›` are the exception and mute their line whole, because there the line *is* the non-event. Every glyph is **one column** — no badges outside the TUI. And `Muted` has **exactly two jobs**: chrome (a field's label, a table's header, a tree's connectors) and a non-event line; **secondary detail is indented, never muted**. Two failure registers only — `!` for something left to do, `✗` for a failure — so `output.Danger` is gone. The runes live in `domain` (`GlyphSuccess`, `GlyphAttention`, …), never as literals in `output/`. The four block helpers are arbitrated by shape: `SectionTitle` for a caller drawing its own body, **`Announce` for `label  value` rows** (it owns the alignment — never hand-space a label inside a format string), `Section` for free lines, `Callout` — the only bordered one — for what the reader still has to act on. And "each command frames its human output exactly once" counts *uninterrupted blocks*: a prompt between two blocks makes two frames, as does a split across the two streams.

- **Success contracts, anomaly expands.** A pass that did exactly what was asked is a count; a refusal, a conflict or a link matching nothing is named one by one. The port pass is the reference: `rules.EnvPortAnomalyLines` still lists every link wtm declined to act on, while the twelve values it settled are `4 ports shifted (+10)` on the recap's env line. Giving the nominal path as much room as the actionable one is what makes a CLI read as noise.
- **Detail belongs to the command whose subject it is.** Ports are the subject of `wtm env` and `run init`; in `create` and `extract` they are a side effect, so they collapse to one line there. A reader who wants the values runs the command that is about them — or opens the file the run just wrote.
- **A successful run has a fixed shape.** What makes output feel bloated is that its size varies with what happened, so it can never be recognised at a glance. `wtm create` is six lines whether it settled three ports or thirty.

**What is shown is not what is kept.** A hook that runs for forty seconds must be visible while it runs — silence reads as a hang — and must not survive in the scrollback, which nobody rereads. `output.HookView` is the shape: a bounded tail redrawn in place, erased and replaced by one `✓ <hook> (12.4s)` line, and the tail kept on screen when the hook failed. It only ever applies to a terminal this process may repaint (`output.IsTerminal`): a pipe, a CI log or `--output json` gets the raw stream, unconditionally and unchanged. The log is the surface's, not the view's: `commands/shared.DrawHookPhase` opens `<state-dir>/hooks/<phase>-<branch>.log` (`output.HookLog`, `rules.HooksLogPath`) on **every** path and tees the raw stream into it — a run whose output the reader could not watch is exactly the one whose record has to survive. That one function is also the only place either surface draws a hook phase; `create` reaches it through `CLIPresenter.HookPhase`, `extract` and `checkout` through `RunCreateHooksPhase`, because the two drifted apart once already. A phase reports through `flow.HookSink` — the raw output *and* the `domain.HookBeat` of each hook starting and finishing — so the surface decides what to draw; `service/hooks` renders only the fallback for a caller that installed no reporter.

**The `flow/` layer (LUC-175).** A command's flow lives in `internal/flow/`, not in `commands/`: `runCreate`/`runClean` read the flags, decide *who may be asked* and *where output goes*, then call `create.Run` / `clean.Run`. One package per command, each splitting the run from the questions it asks. Three seams let a second surface (`tui/dashboard`) replay the same flow:
- **`flow.Prompter`** answers the questions: `Ask(Session)` for a whole question-and-recap sequence, `Confirm` for a standalone post-execution question, `Interactive()` to know whether a decision may be offered at all. Implementations: `tui/flowui` (the CLI wizard), `flow.Unattended` (`--yes` / no TTY / JSON), and the dashboard. `Interactive()` is only ever read to (a) not offer a decision nobody can answer and (b) feed a pure rule that takes it as input (`rules.DecidePush`).
- **`flow.Presenter`** shows the phases (`Stage`, `HookPhase`, `Notice`, `Status`) plus one typed per-command conclusion (`Created`, `Cleaned`). A flow never frames, never animates and never picks a stream; errors are **returned**, never presented.
- **`flow.Request`** (`CreateRequest`, `CleanRequest`) carries what the surface already knows. It holds no `--yes` and no output format: the confirmation axis is the installed Prompter, the format is the surface. `--force` *does* belong there — it is the safety axis, a business input.

A run reports through a **`seam.Watcher`** rather than a `Presenter` alone: the surface has to be drawing before the first job is asked for, so it is the surface that calls the start sequence and hands back its `Outcome`. That is the one thing `Stage` cannot express, and the only reason the run flows' `Presenter` is wider than `flow.Presenter`.

Steps are declared as `flow.Step` values (`Kind`, `Key`, `Label`, `Options`, `Skip`, `Build`, `Load`, `Resolve`, `Summarize`). `Resolve` is the entire bypass taxonomy in one place: returning an `Answer` is a decision with a safe default, returning an error refuses the run naming the flag, and a step with no `Resolve` can only be answered interactively (`flow.Unattended` never falls back to a picker). A value the request already carries goes in `Session.Presets`: the step is not asked but is still read back, which is what keeps a flag from erasing a recap line. The `StepContent` a step builds also carries `Blockers` — the safety refusals standing in the way of its dangerous option, named one by one instead of folded into the prose, so a surface can have each of them lifted separately (`rules.CleanBlockers` feeds `internal/flow/clean/steps.go`).

**Handing the terminal over.** A flow whose surface is a second full-screen program — `run up` and `run logs` in the dashboard — goes through `tea.Exec` with a `tea.ExecCommand` that runs `runview` **in this process**, so its result comes back typed rather than as an exit code. Bubbletea releases and restores the terminal around it, but restores neither the mouse tracking it turned off nor anything else a surface enabled after startup: a mouse-driven surface has to ask for it again (`dashboard/handoff.go`).

**`flow.Operation`** (`Kind`, `Mode`, `TargetKey`) is what a flow declares about *how it is scheduled*, for a surface that runs several at once. `Mode` says how long it holds that surface — `ModeBlocking` (`clean`) keeps it until the run ends, `ModeBackground` (`create`) gives it back and locks its target instead — and `TargetKey` names the answer carrying the worktree it locks, known only once that step is answered. The CLI ignores it (one run, one terminal); `internal/tui/dashboard/ops.go` is where it is enforced, once, rather than at every action site.

Adding a kind means teaching every surface to render it: `flowui` refuses an unknown kind rather than guessing. Test doubles for the two seams live in `internal/testutil/flowtest`. `create`, `clean`, `reparent`, `prune`, `sync` and the whole `run` module are migrated — `up`, `down`, `start`, `stop`, `logs`, `list`, `open`, `url` and the eight `run job` / `run profile` commands (`ps` asks nothing, so it is not a flow). **Four mutation commands are still out: `extract` (LUC-182), `checkout`, `relocate` and `env`**, each driving its service straight from its runner. They are listed in `.archlint-migrating`, which reports them on every `make lint` and may only shrink — `tui/newwt` stays until `extract` follows.

A **non-mutating mode** (`prune --dry-run`) belongs in the `Request`, not in the runner: it changes what the run does, not how it reads. The flow returns its `Outcome` before asking anything and before touching anything, and any rule that reads `Interactive()` must take the mode as an input too — a surface may install an interactive Prompter for a preview. See `rules.PruneClassifyForce` and `docs/dev/flow-layer.md`.

**Every new worktree-mutating command goes through `flow/`** — no exception, and no new command written on the pre-`flow` model even to match a neighbour that has not migrated yet. Concretely: declare `Request`/`Outcome`/`Presenter`/`Params`/`Run` in `internal/flow/<cmd>/`, its questions as `flow.Step` in `steps.go`, and keep the runner in `commands/` to flags → `Request` → pick the two seams → `<cmd>.Run`. A runner that inspects state, orders service calls or gates a picker on `interactive` beyond choosing the Prompter has put the flow in the wrong layer, and a service closure injected into a `tui/` package is the same mistake in its older form. The developer reference is `docs/dev/` (`flow-layer.md`, `adding-a-mutation-command.md`); the import rule above is checked mechanically by `make lint` (`tools/archlint`, rule `layers`).

**How a command designates a worktree (LUC-211).** One rule, no exception: **the subject is positional, and a worktree that is not the subject is a flag named after its role.** `clean [branch]`, `env [worktree]`, `extract [source]`, `sync [branch...]` take their subject positionally; `extract --to`, `create --from`, `sync --base` name a second worktree. The `run` module follows the same rule with the worktree as its subject — `run up [worktree] --profile`, `run start [worktree] --job` — so the job and the profile are flags. A new command adds no third form.

Omitting the positional resolves in one of two ways, and which one is not a matter of taste: **the current directory when it is a safe default for that command, a picker otherwise.** `run` has one (you are standing in the worktree whose services you want), so a non-interactive run silently takes it — category 1 of the bypass model below, no exception to write. `clean` has none (which worktree would it destroy?), so it errors or opens a picker — category 2.

Whatever answers, a resolved worktree is always **the worktree root as git spells it** (`infra.Toplevel`), never a raw `os.Getwd()`. The daemon keys a job on `name + WorkDir` by string equality *and* runs it there, resolving `run.toml`'s `cwd` against it: a subdirectory, or macOS's `/var` where git says `/private/var`, splits one worktree into two keys and mis-resolves every relative `cwd`.

**Mutation commands — bypass flags (two orthogonal axes):** every worktree-mutating command (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`, `extract`, `checkout`, `env`) exposes bypass on two independent axes. This is the standardized model (aligned with `gcloud --quiet`, `terraform -input=false`, `apt -y` vs `--force-yes`, and [clig.dev](https://clig.dev)); every new or refactored mutation command MUST follow it.
- **`--yes` / `-y` = the confirmation/decision axis — runs fully unattended, zero prompts.** Every input resolves in one of three ways, no interaction:
  1. **Decision / confirmation** (recap, reparent, push, on-conflict, fast-forward) → its flag value, else a documented **safe default** (never destructive: `sync --yes` does not push — use `--push`; `extract --yes` aborts on conflict; `clean`/`prune --yes` leave orphans unless `--reparent-children`).
  2. **Required selection with no safe default** (which files for `extract`, which worktrees for `sync`, source/branch args) → its flag/arg, else **error naming the missing flag**. Never fall back to an interactive picker under `--yes`.
  3. A picker only ever runs in a **fully interactive** run (no `--yes`, TTY, human output).
- **`--force` = the safety axis, strictly separate.** It only lifts safety refusals (dirty / unpushed / open-PR / locked). It does **not** imply `--yes`: `--force` alone still runs the wizard and asks to confirm (thread `--force` into the wizard as a preset so refusals are lifted without re-asking). JSON mode requires `--yes`.

Implementation rule: fold `--yes` into the command's `interactive` boolean (`interactive := isTTY && IsHumanFormat(format) && !yes`); every picker/prompt gates on `interactive`, and each required-selection guard returns a sentinel error when it is false. See `internal/commands/wt/extract.go`, and — for a migrated command — `internal/flow/sync/steps.go` (`selectionStep`'s `Resolve`, which names `--all` instead of falling back to a picker). Route decision defaults through a pure rule where one exists (`rules.DecidePush` takes a `Yes` field).

**Recap completeness:** every recap builder reads the value from its wizard step, **else falls back to the flag/arg** that resolved it. A flag must never make a line disappear from the recap. A migrated command gets this from `Session.Presets` (a preset step is not asked but is still read back — see `internal/flow/create/steps.go` `createFlow.recap`); the others do it in their recap builder (e.g. `internal/tui/extract` `buildCombinedRecap`, `internal/tui/newwt` `buildCreateRecap`, `internal/tui/checkout` `buildCheckoutRecap`).

**Re-init completeness:** a re-init step always shows the **complete** list of candidates, pre-filled from the config on disk when that config speaks about them, and from detection otherwise — never a subset. Any step whose answer may legitimately be empty is read as a pair `(value, asked)`: empty-and-asked withdraws, empty-and-not-asked leaves the proposal standing. The pairs are `URLsAsked`, `ProfilesAsked`, `EnvLinksAsked` and `SelectionAsked` in `domain.InitProjectAnswers`. This is the write-side counterpart of the rule above: a flag must not erase a recap line, and a step must not reinstate what the user removed.

Two corollaries a new step must respect. Its pre-fill reads the **existing config**, not the detection, wherever the config has an opinion (`rules.ProposedScriptKind`, `rules.URLCandidatesFor`). And a step that **removes** may only remove what it proposed itself: `rules.DeselectedJobs` never reaches a job written by `run job add`, because such a job matches no detected script or compose file. Removal goes through `rules.RemoveJob`, the one place that also strips profile entries and `[[env_port]]` links.

## 10. Commit messages in English

Every commit message — subject and body — is written in **English**, whatever language the conversation that produced the change was held in. The repository, its code, its comments and its docs are in English; the history is read alongside them.

## 11. Validate before commit

`make lint` is the mechanical half of this file. It is not a formality: every rule in it exists because a reviewer would otherwise have to hold section 9 in their head on every PR, and the ones nobody holds are the ones that drift.

```
make lint     # fmt + vet + arch + dead + staticcheck — all gating
make test     # go test ./... -race -count=1
make dupl     # clone report, informative only
```

| Check | Catches | Why staticcheck cannot |
| -- | -- | -- |
| `fmt` | unformatted files | it is not a formatter, and this fails rather than rewrites: a formatting fix belongs in the commit that caused it |
| `vet` | the stdlib's own suspicions | — |
| `arch` (`tools/archlint`) | the layer graph of section 9, the `styles/` monopoly on `lipgloss.Style`, type assertions without comma-ok, a command that reads the interactive gate without offering `--yes`, a worktree-mutating command that skips `flow/`, and the three output rules below | it checks a package against itself, and knows nothing about this project's layers |
| `dead` (`deadcode`) | functions no path reaches, **test paths included** | it reports the unused *within* a package; a function exported and called by nobody is invisible to it |
| `staticcheck` | the rest | — |

**The output vocabulary is enforced, not remembered.** The glyph table was written down and the surface diverged anyway — sixty commands, five renderings of "nothing to do", `!` alone rendered as a filled chip. Three `archlint` rules hold the parts that a table cannot say, and they run only over the layers that put glyphs on a screen (`output`, `styles`, `tui`): `rules/` and `service/` are left out because `=` and `!` are ordinary bytes to an env parser or a pnpm workspace pattern, and a check that cannot tell those apart is one people work around.

| Rule | Catches |
| -- | -- |
| `glyph` | a vocabulary rune written as a literal — `"✓"`, `"!"`, `"→"` … — instead of its `domain` constant, which is how a seventh glyph appears and how an existing one takes a second meaning |
| `tuistyle` | `styles.Badge*` or `styles.Dashboard*` used from `internal/output`: a badge is a widget, and its padding made an attention line two columns wider than the failure line under it |
| `mutedline` | `Message(w, styles.Muted.Render(x))` — a bare line muted whole, which is the `=` register with its glyph filed off. Muted has two jobs, chrome and a glyphed non-event line; secondary detail is subordinated by an indent |

**`tools/archlint` is where a new architectural rule goes.** Its `layers` table is section 9's dependency graph written once, so a new dependency between two layers is a deliberate edit to that table rather than something that lands unnoticed. Adding a rule there is cheaper than adding a paragraph here, and it is the only kind of rule that survives.

The exceptions to `dead` live in `.deadcode-ignore`, one regex per line **with its reason** — reachable by a route the analysis cannot follow (a method satisfying an interface asserted on an `any`, so far). Anything unlisted fails.

`.archlint-migrating` is the same idea for what predates a rule: `<rule> <path regex>` lines that report as `(migrating)` without failing. **It may only shrink.** A new entry is a decision to take knowingly and belongs in a ticket, never a way to get a commit past the linter.

`make dupl` is deliberately outside `lint`: a clone is a judgement call. Two parallel families over unrelated types — `flow/run/job` and `flow/run/profile` — read better duplicated than behind a generic, so the report informs a review rather than gating one.

**Considered and left out, so it is not re-proposed:**

| Rule | Measured | Why not |
| -- | -- | -- |
| Section 2, structs for 2+ inputs | 546 functions at 2+ non-carrier inputs, 17 at 4+ | A count cannot tell a related pair from a carrier pair. A gate at 2 fires on most of `output/`; one at 4 still fires on a syscall wrapper whose arity is the ABI. It stays a review rule |
| Section 8, comment density | — | The ceiling is met as easily by deleting the comments that earn their place as the ones that do not, so the number measures the wrong thing. The rule itself needs rethinking before anything can check it |
| A `label  value` format string hand-aligning its own column | 1 match in `domain` (`DetailReviewDecisionFmt`), and it is a dashboard fragment, not a block | The regex that finds a hand-aligned label finds the one legitimate use too. `Announce` owning the alignment is the fix; a gate over it would teach people to space their labels differently rather than to use the helper |

A rule belongs in `make lint` when breaking it breaks something — a layer, a refusal, a surface that can no longer run a flow. A rule about how code reads belongs in review, and writing it down here is how it survives.

### The gates are enforced, not remembered

`.claude/hooks/pre-commit-gates.sh` runs `make lint` and the tidy check on every `git commit`, as a `PreToolUse` hook, and blocks the commit when either fails. It exists because saying "run the checks before every commit" was not enough: this repository shipped a stale `go.sum` after the checks had been run once, earlier in a session, and treated as still true six commits later. A hook cannot forget, and it reads the same Makefile a human does — which is the whole reason the gates were moved there.

It deliberately does **not** run the tests: at ~70s that is a gate people disable, and the CI already runs them. `WTM_SKIP_GATES=1 git commit …` gets past it for the case where the gate is itself wrong.

Run the **`build-validator`** subagent at the end of every development session, and before a commit that changes anything the hook does not cover — it adds the test suite with `-race`, dependency hygiene, and the duplication report.

```
→ Invoke build-validator before marking any task done.
```

