---
name: go-cli
description: Expert guidance for developing CLI applications in Go following strict architectural principles, using Cobra, Bubbletea, and Lipgloss. Use this skill whenever the user is building, extending, or refactoring a Go CLI — including adding commands, flags, subcommands, TUI components, config parsing, output formatting, error handling, or structuring packages. Trigger on any mention of "go CLI", "cobra", "bubbletea", "bubble tea", "lipgloss", "TUI", "go command", "go flag", "go binary", "charm", or any request to scaffold or improve a Go terminal application. Also trigger when the user references the project's CLAUDE.md principles in the context of Go code, when adding a new command, or when discussing test patterns for commands.
---

# Go CLI Development Skill

## Stack

- **Cobra** — command routing and flag parsing
- **BurntSushi/toml** — configuration loading (strict decode with unknown-key rejection)
- **Bubbletea** — TUI programs (Model/Update/View)
- **Lipgloss** — terminal styling, centralized in `internal/styles/`

---

## Core Principles (from CLAUDE.md)

Always apply these rules. They are non-negotiable.

### 1. Immutability first — extract functions, not variables

Prefer short variable declarations (`:=`) for values that do not change.
Extract logic into named functions rather than reassigning variables.

```go
// ❌ Avoid
var result string
if condition {
  result = "a"
} else {
  result = "b"
}

// ✅ Prefer
result := resolveResult(condition)
```

### 2. Structs for 2+ parameters — always use named fields

Any function taking 2 or more related parameters must use a struct. Always
initialize with named fields (no positional struct literals outside tests).

```go
// ❌ Avoid
func RunCommand(name string, timeout int, verbose bool) error

// ✅ Prefer
type RunCommandParams struct {
  Name    string
  Timeout int
  Verbose bool
}
func RunCommand(params RunCommandParams) error
```

### 3. Shared types — single source of truth

All domain types, enums, and error sentinels live in `internal/domain/`
(`constants.go`, `errors.go`, `types.go`, `jobs.go`, `init.go`). Never duplicate a type across packages.

### 4. Validate at the boundary

Validate all external input (flags, env vars, config files) in `internal/config/`
before it reaches the service layer. Use explicit guard clauses.

### 5. Centralized constants — no magic strings or numbers

All flag names, command names, exit codes, and format identifiers
must be declared as constants in `internal/domain/constants.go`.

```go
const (
  // CLI command names — used in Use: and exec.Command(bin, ...) call sites
  CmdRun      = "run"
  CmdUp       = "up"
  CmdDown     = "down"
  CmdStart    = "start"
  CmdStop     = "stop"

  // Flag names
  FlagOutput  = "output"
  FlagForce   = "force"
  FlagProfile = "profile"

  // Output format values
  OutputText  = "text"
  OutputJSON  = "json"
)
```

### 6. Early returns — flatten all conditionals

Never nest `if` blocks. The happy path is always the last statement.

### 7. No unsafe type assertions

Always use the comma-ok idiom. Prefer typed interfaces and concrete structs
over `any`/`interface{}`.

```go
// ❌  val := data.(string)
// ✅
val, ok := data.(string)
if !ok {
  return fmt.Errorf("expected string, got %T", data)
}
```

### 8. Comments — the exception, not the rule

Aim for near-zero comments: encode the meaning in names and signatures instead
(`Skip func(Answers) (skip bool, reason string)` needs no prose). Comment only the
why the code cannot carry — a non-obvious decision, an ordering constraint, a
workaround with its issue reference — plus a one-line package comment. Godoc on an
exported symbol only when its name and signature leave a caller guessing, never
systematically. When you modify a file, strip the comments in it that restate the
code, in the same change.

### 9. Clean architecture layers

```
cmd/
  root.go                     ← cobra root, version, help

internal/
  commands/                   ← flag wiring only → delegates to flow/service (zero logic)
    ui/                       ←   `wtm ui`: refuses JSON + missing TTY, hands off to tui/dashboard
  domain/                     ← types, errors, constants only (no methods, no functions)
  rules/                      ← pure business rules (stdlib + domain only, no I/O)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/
  flow/                       ← THE FLOW of a command, surface-independent
    decide/                   ←   branch/env decisions shared by the create-like flows
    create/                   ←   `wtm create`: the run (create.go) + its questions (steps.go)
    clean/                    ←   `wtm clean`: the run (clean.go) + its questions (steps.go)
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   create, list, clean, resolve
    env/                      ←   .env file provisioning strategies
    hooks/                    ←   on_create hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    process/                  ←   daemon, job manager, socket client
    github/                   ←   PR operations via gh CLI
    detect/                   ←   auto-detection (package manager, env files, scripts)
    integration/              ←   third-party adapters (VS Code, Cursor)
  output/                     ← format and print results (zero decision logic)
  styles/                     ← all Lipgloss styles + shared Indent constant
  tui/                        ← Bubbletea models (zero business logic, rendering only)
    flowui/                   ←   runs a flow.Session as the CLI wizard (the only translator
                                  between flow.Step and components.Step)
    dashboard/                ←   `wtm ui`, the second surface over flow/ (its own
                                  Prompter/Presenter, mouse zones via bubblezone)
  infra/                      ← I/O, git exec, filesystem wrappers
  testutil/gittest/           ← shared test helpers (InitRepo, CreateBranch)
  testutil/flowtest/          ← test doubles for the two flow seams (Prompter, Presenter)
```

**Hard rules:**
- `commands/` has zero business logic
- `domain/` imports only stdlib (unchanged)
- `rules/` imports only stdlib + internal/domain
- `flow/` imports **only** `internal/service/`, `internal/rules/`, `internal/domain/` and the
  stdlib — never cobra, bubbletea or lipgloss, and never `output/`, `tui/`, `config/` or
  `commands/`. It therefore cannot reach `infra/` either: add a thin `service/` wrapper
  instead (e.g. `worktree.FindByBranch`, `worktree.ListAll`). `build-validator` step 6
  checks this mechanically.
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`

**The `flow/` layer — where the flow of a mutation command lives.** A command that
mutates worktree state does **not** orchestrate: `commands/` reads the flags, builds a
`Request`, picks a `Prompter` and a `Presenter`, and calls `<cmd>.Run`. Three seams let a
second surface replay the same run:

- **`flow.Prompter`** — `Ask(Session) (Answers, error)` for a whole question-and-recap
  sequence, `Confirm(ConfirmParams) (bool, error)` for a standalone post-execution
  decision, `Interactive() bool`. Implementations: `tui/flowui` (the CLI wizard),
  `flow.Unattended` (`--yes` / no TTY / JSON), `tui/dashboard`. Read `Interactive()` only
  to (a) not offer a decision nobody can answer, (b) feed a pure rule that takes it as an
  input (`rules.DecidePush`).
- **`flow.Presenter`** — `Stage`, `HookPhase`, `Notice`, `Status`, plus **one typed
  conclusion per command** (`Created(Outcome) error`, `Cleaned(Outcome) error`). A flow
  never frames, never animates, never picks a stream. Errors are **returned**, never
  presented — there is no `Presenter.Error`.
- **`Request`** (declared by the command's flow package, e.g. `create.Request`) — what the
  surface already knows. It carries **no `--yes` and no `--output`**: the confirmation
  axis is the installed Prompter, the format is the surface's business. `--force` *does*
  belong there — it is the safety axis, a business input.

Steps are `flow.Step` values (`Kind`, `Key`, `Label`, `Title`, `Description`, `Options`,
`Skip`, `Build`, `Load`, `Resolve`, `Summarize`, `Flag`). `flow.Operation`
(`Kind`, `Mode`, `TargetKey`) is what a flow declares about how a surface must schedule
it; the CLI ignores it, `tui/dashboard/ops.go` enforces it.

`create` and `clean` are migrated. `extract`, `sync`, `prune`, `relocate`, `reparent`,
`checkout` and `env` still drive their `internal/tui/*` wizard packages directly — the
`components.Step` sections below still describe them. **Any new mutation command goes
through `flow/`.** Full reference: [`docs/dev/flow-layer.md`](../../../docs/dev/flow-layer.md)
and [`docs/dev/adding-a-mutation-command.md`](../../../docs/dev/adding-a-mutation-command.md).

### 10. Validate before commit — run `build-validator`

Before marking any task done, invoke the `build-validator` subagent.

---

## Cobra — Command Patterns

### Standard command

The real pattern used in this project — no dependency injection, uses helper functions:

```go
// internal/commands/start.go
func newRunStartCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   domain.CmdStart + " <job>",
    Short: "Start a single job",
    Args:  cobra.ExactArgs(1),
    RunE:  runStart,
  }
  shared.AddOutputFlag(cmd)
  return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
  dir, err := os.Getwd()
  if err != nil {
    return fmt.Errorf("get working directory: %w", err)
  }

  result, ok := loadConfig(cmd, dir)
  if !ok {
    return nil
  }

  // ... delegate to service, format output
}
```

Key conventions:
- `Use:` always uses `domain.CmdXxx` constants (+ literal arg placeholders)
- `shared.AddOutputFlag(cmd)` instead of duplicating the output flag registration
- `shared.LoadConfig(cmd, dir)` resolves the main worktree path **and** the state dir, then loads `<state-dir>/config.toml`. Returns `ConfigResult{Config, ProjectDir, StateDir}`.
- Unexported constructor (`newRunStartCmd`), registered by the parent group

### Mutation command — build a Request, call the flow

A command that **mutates worktree state** does not orchestrate anything: it reads the
flags, builds the `Request`, picks the two seams, and calls `<cmd>.Run`. This is the
whole runner (see `internal/commands/wt/create.go`, `clean.go`):

```go
func runClean(cmd *cobra.Command, args []string) error {
  force, _ := cmd.Flags().GetBool(domain.FlagForce)
  yes, _ := cmd.Flags().GetBool(domain.FlagYes)
  format, _ := cmd.Flags().GetString(domain.FlagOutput)

  if format == domain.OutputJSON && !yes {
    return domain.ErrCleanJSONNeedsYes
  }

  dir, err := os.Getwd()
  if err != nil {
    return fmt.Errorf("get working directory: %w", err)
  }
  config, err := shared.LoadConfig(cmd, dir)
  if err != nil {
    return err
  }

  // The ONLY thing --yes decides: which Prompter gets installed.
  interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes

  _, err = cleanflow.Run(cleanflow.Params{
    Context:   flowContext(config),                      // ProjectDir, StateDir, Config
    Request:   cleanflow.Request{Branch: branchName, Force: force /* … */},
    Prompter:  flowPrompter(flowPrompterParams{Interactive: interactive, Stderr: true}),
    Presenter: cleanPresenter{cliPresenter: newPresenter(cmd, format)},
  })
  return err
}
```

- `flowContext` / `flowPrompter` / `cliPresenter` are shared helpers in
  `internal/commands/wt/presenter.go`. `flowPrompter` returns `flow.Unattended{}` when
  `Interactive` is false, `flowui.New(...)` otherwise — that is the entire `--yes` wiring.
- The command's presenter embeds `cliPresenter` and adds **only** the typed conclusion
  (`Created`, `Cleaned`), which is where `--output json` branches and where
  `output.Frame` is applied — exactly once.
- No safety check, no picker fallback, no `need*` guard in the runner: those belong to
  the flow's steps. If you are writing an `if !interactive` in a runner beyond the line
  above, the logic is in the wrong layer.

The other half lives in `internal/flow/<cmd>/<cmd>.go`, and is always this shape:

```go
type Request struct {  // what the surface already knows — no --yes, no --output
  Branch string
  Force  bool          // only if the command has safety refusals to lift
}

type Outcome struct {  // data, never text
  Branch  string
  Result  domain.SplitResult
  Aborted bool
}

type Presenter interface {
  flow.Presenter       // Stage, HookPhase, Notice, Status
  Split(Outcome) error // the typed conclusion, one per command
}

type Params struct {
  Context   flow.Context
  Request   Request
  Prompter  flow.Prompter
  Presenter Presenter
}

func Run(params Params) (Outcome, error) {
  f := &splitFlow{ctx: params.Context, request: params.Request,
    prompter: params.Prompter, presenter: params.Presenter}
  return f.run()
}

// Only if a surface may run several at once (the dashboard enforces it, the CLI ignores it):
func Operation() flow.Operation {
  return flow.Operation{Kind: domain.OpKindSplit, Mode: flow.ModeBlocking, TargetKey: KeyBranch}
}
```

Inside `run()`: guard clauses first, then `f.prompter.Ask(f.session())`, then the work —
long work through `presenter.Stage`, hooks through `presenter.HookPhase`, a line inside a
phase through `presenter.Status`. `domain.ErrUserAborted` from `Ask` becomes
`presenter.Notice(flow.AbortedNotice)` + `Outcome{Aborted: true}, nil`. Every other error
is **returned**; the flow never prints.

### State-dir resolution

wtm stores all of its state under `<git-common-dir>/wtm/` (i.e. `.git/wtm/` for a normal clone), so nothing leaks into the user's working tree. Resolution helpers live in `internal/commands/shared/`:

```go
shared.StateDir(dir)                   // <git-common-dir>/wtm/
shared.WorktreeStateDir(WorktreeStateDirParams{Dir, Branch})  // <state-dir>/worktrees/<encoded-branch>/
```

The git common dir is fetched via `infra.GitCommonDir(GitCommonDirParams{Dir})` which wraps `git rev-parse --git-common-dir`. Branch names are encoded for filesystem safety with `rules.EncodeBranchSegment(branch)` (slashes → `%2F`).

Two env-var overrides exist for tests / CI:
- `WTM_PROJECT_DIR` — bypasses git for the main worktree path
- `WTM_STATE_DIR`   — bypasses git for the state dir

### Adding a new command

1. Create `internal/commands/<name>.go` with unexported constructor
2. Use `domain.CmdXxx` for the `Use:` field (add constant if new)
3. Register in the parent group's `NewXxxCmd()` function
4. Set the command's `GroupID` to the right root `--help` section
   (`domain.CmdGroup*` — Worktrees / Navigate / Stack / Jobs / GitHub / Setup). An
   unset `GroupID` renders under a stray "Additional Commands" heading.
5. **Read-only command** → follow the `runStart` pattern: getwd → loadConfig → delegate
   to `service/` → format via `output/`.
   **Mutation command** (creates/removes/moves/rewrites worktree state) → it goes
   through `flow/`: declare `Request`/`Outcome`/`Presenter`/`Params`/`Run` in
   `internal/flow/<name>/`, declare its questions as `flow.Step` in `steps.go`, and keep
   the runner to the shape in "Mutation command" above. Never put the flow in
   `commands/`, and never inject a service closure into a TUI package.
6. Regenerate the reference and update the guide: `make docs` (writes `docs/`, never
   hand-edited) and add the command to the `README.md` overview table. See CLAUDE.md
   "Docs & README".

Full recipe, step by step: [`docs/dev/adding-a-mutation-command.md`](../../../docs/dev/adding-a-mutation-command.md).

### Shared flag helpers

```go
// internal/commands/shared/helpers.go

// AddOutputFlag registers the standard --output flag on cmd.
func AddOutputFlag(cmd *cobra.Command) {
  cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
}
```

Call it as `shared.AddOutputFlag(cmd)` from a command constructor.

### TUI command — Cobra launches a Bubbletea program

The command runner is the only place where `tea.NewProgram` is called.
All TUI state lives in `internal/tui/`, never in `commands/`.

```go
func runBrowseTUI(ctx context.Context, items []domain.Item) error {
  model := tui.NewBrowseModel(items)
  program := tea.NewProgram(model, tea.WithAltScreen())

  result, err := program.Run()
  if err != nil {
    return fmt.Errorf("tui: %w", err)
  }

  finalModel, ok := result.(tui.BrowseModel)
  if !ok {
    return nil
  }
  return handleBrowseResult(finalModel.Selected())
}
```

---

## Configuration — BurntSushi/toml

The project uses BurntSushi/toml with a custom strict decoder that rejects unknown keys.

```go
// internal/config/strict.go
func decodeStrict(path string, out any, ignorePrefixes ...string) error
func decodeStrictBytes(source string, data []byte, out any, ignorePrefixes ...string) error
```

Struct tags are `toml:"..."` (+ `json:"..."` for types that get JSON-exported).
Config files include a `#:schema ./schemas/xxx.json` directive for editor support.

---

## Bubbletea — TUI Architecture

### Shared components

All reusable TUI primitives live in `internal/tui/components/`:
- `WizardModel` — multi-step form driver
- `SelectListModel` — single-choice list with filter
- `MultiSelectModel` — toggle-able multi-choice list
- `TextInputModel` — single-line input with validation
- `ConfirmModel` — yes/no dialog
- `RunStandaloneSelect(model)` / `RunStandaloneConfirm(model)` — one-shot wrappers

### Multi-step form → declare `flow.Step`, or `WizardModel` for a non-migrated wizard

**A mutation command declares its steps as `flow.Step` values in
`internal/flow/<cmd>/steps.go`, and never touches a Bubbletea model.** `internal/tui/flowui`
is the single translator: it turns a `flow.Session` into `components.Step`s and runs the
same `WizardModel` under the hood, so everything below about breadcrumbs and
back-navigation still holds — it is just no longer the command's business. `flowui`
refuses an unknown `StepKind` rather than guessing, so adding a kind means teaching every
surface to render it.

The `components.Step` API below remains the model for the wizards **not yet migrated**
(`extract`, `sync`, `prune`, `relocate`, `reparent`, `checkout`, `env`) and for
non-mutation pickers (`run`, `init`). Do not start a new mutation wizard here.

A flow with **2+ sequential decisions** (e.g. pick worktree → pick new parent) MUST be a
single `components.WizardModel`, exposed via a `RunWizard` in the screen package. The wizard
gives a **breadcrumb** (`Step 1/2`) and **back-navigation** (`Esc` steps back on step 2+,
cancels on step 1). Chaining several `RunStandaloneSelect`/`RunStandaloneConfirm` calls is a
bug: no breadcrumb, and `Esc` quits the whole flow instead of going back.

- Build `[]components.Step{}`; a step whose options depend on a previous answer uses
  `Build: func(prev []components.Step) any` to rebuild its model from `prev[i].Model.(...).Value()`.
- Prefer the runner `components.RunWizard(components.RunWizardParams{Steps, Stderr: true, ErrLabel,
  OnMsg, InitCmd, Loading, LoadingText})` — it centralises the program/assertion/abort boilerplate.
  It maps `Esc` at step 1 to `domain.ErrUserAborted`; otherwise pull values from
  `final.Steps()[i].Model.(components.SelectListModel).Value()`.
- Reference implementations: `internal/tui/relocate/wizard.go`, `internal/tui/checkout/wizard.go`.
  (`clean` and `sync` are no longer among them: their steps are declared in
  `internal/flow/<cmd>/steps.go` and run through `internal/tui/flowui`.)

Standalone wrappers (`RunStandaloneSelect`/`RunStandaloneConfirm`) are only for a **single**
one-shot decision where there is no prior step to go back to (e.g. `run up`'s profile picker).

### Wizard shape for worktree-mutation commands: `[inputs] → [decisions…] → recap`

**Migrated commands (`create`, `clean`) express this shape in `flow.Step`:**

```go
func (f *createFlow) session() flow.Session {
  return flow.Session{
    ErrLabel: domain.WizardErrLabel,
    // A value the request already carries: the step is NOT asked, but is still read back.
    Presets: flow.NewAnswers(map[string]string{KeyBranch: f.request.Branch, /* … */}),
    Steps: []flow.Step{
      f.branchStep(),        // StepText, Validate
      f.sourceStep(),        // StepBranchSelect, Build, Refresh
      f.envStep(),           // StepSelect, Summarize
      f.sourceUpdateStep(),  // StepSelect, Skip → conditional decision
      f.recapStep(),         // StepRecap, always last, unconditional
    },
  }
}
```

Field by field, and what each replaces:

| `flow.Step` field | Role | Replaces |
| -- | -- | -- |
| `Kind` | `StepText` · `StepSelect` · `StepBranchSelect` · `StepRecap` | the concrete model in `components.Step.Model any` |
| `Key` | identifies the answer in `Answers`; stable across surfaces | reading `prev[i].Model.(X).Value()` by index |
| `Skip func(Answers) (skip bool, reason string)` | the step became irrelevant, and why (user-visible ⊘ line) | `ChoiceStep.Decide`'s `apply`/`skipReason` |
| `Build func(Answers) (StepContent, error)` | re-derive title/description/options from earlier answers, synchronously | `Step.Build func(prev []Step) any` |
| `Load` + `LoadingMessage` | same, but it does I/O — the host shows a loading state | `OnEnter` + request/done msgs + `UpdateStepModel` |
| `Resolve func(Answers) (Answer, error)` | how the step answers itself with nobody to ask | the per-command `need*` / `canPrompt` guards |
| `Validate` | reject a `StepText` value inline | unchanged |
| `Summarize func(Answer) string` | what the summaries read back when the raw value is not it | `components.Step.Summary` |
| `Flag` | names the flag a refusal should point at | hardcoded messages |
| `StepContent.Blockers []flow.Blocker` | the safety refusals gating the step's dangerous option, **one by one** | a bulleted paragraph inside the recap prose |

Rules that carry over unchanged into the flow model:

- **The recap is the last step, always, and unconditional.** It is a `StepRecap` whose
  `Build` returns the plan as `Description` and the action as its `Options`; the host
  appends the cancel row (`domain.WizardCancelValue` → `domain.ErrUserAborted`).
- **Blocking warnings are not gate steps** — a diverged source or an env fallback is a
  `⚠` line in the recap description, not a question. The single cancellation point is
  the recap's cancel row (plus `Esc` on step 1).
- **Blockers are named individually.** `rules.CleanBlockers` → `StepContent.Blockers`
  lets the CLI print them as a list while the dashboard renders one checkbox each, gating
  the dangerous option. Never fold a refusal into prose only.
- **A genuinely post-execution decision stays a `Prompter.Confirm`** — a failed
  fast-forward, a `sudo` removal, an extract conflict. It cannot be decided upfront, so it
  is not a session step.
- **Bypass follows the two-axis taxonomy below.** In the flow model it is `Resolve`; you
  do not reimplement it.

---

**Non-migrated wizards** keep the `components.Step` shape below. Confirmations belong INSIDE the wizard, never as a trailing standalone (`Esc` on a standalone
aborts the whole flow instead of stepping back — the LUC-115 defect). LUC-116 further harmonised
every mutation wizard onto one shape: input/picker steps, then any optional-decision **selects**,
then a single **recap** as the last step. The rules:

- **Optional decision → `components.ChoiceStep`** (a select, never a Yes/No). Every option merely
  advances; `Esc` goes back; it never cancels the operation. `Decide func(prev []Step) (apply bool,
  skipReason string, params NewSelectListParams)` runs on entry: `apply == false` auto-skips with a
  reason shown in the summaries as `⊘ <Name> — <reason>`. Use for fast-forward-vs-keep, reparent-vs-
  leave-orphaned, push-vs-keep-local, on-conflict.
- **Final gate → `components.RecapStep`** (always last, unconditional). `Build func(prev []Step)
  RecapContent` returns the recap description (selections + `⚠` warning lines) and the command's
  action option(s); RecapStep appends the constant `domain.WizardCancelLabel` ("No, cancel") row
  carrying `domain.WizardCancelValue`. It renders a distinct "Review & confirm" header (`Step.Recap`).
  Read the outcome via the step's `SelectListModel.Value()`: `== domain.WizardCancelValue` →
  `domain.ErrUserAborted`, else it is the chosen action.
- **Blocking warnings are NOT gate steps** — fold a diverged source / env fallback / keep-conflict
  into a `⚠` line in the RecapStep description. The single cancellation point is "No, cancel"
  (plus `Esc` on step 1). Don't reintroduce an `AbortOnDecline` gate.
- A `ChoiceStep`/`RecapStep` **cannot be the wizard's first step** (the wizard never builds or
  auto-skips index 0). When a conditional select would otherwise be first (e.g. `clean --branch`
  with no picker), compute it synchronously and add a concrete step only when it applies. In the
  flow model this is not the command's problem: the step just carries a `Skip`
  (`internal/flow/clean/steps.go`, `reparentStep`) and `internal/tui/flowui/prompter.go` decides
  it against what is already known before the wizard starts.
- For an **async** recap (safety check, plan preview), keep a hand-built `SelectListModel` step with
  `Recap: true` and swap its model in via `OnMsg`/`UpdateStepModel` (RecapStep's `Build` is sync).
  In the flow model it is one field instead: `Load` + `LoadingMessage` on the step
  (`internal/flow/clean/steps.go` `deleteStep`, `internal/flow/sync/steps.go` `confirmStep`
  for the cascade preview).
- `ConfirmStep`/`ConfirmModel` stay only for genuine one-shot standalone Yes/No prompts outside a
  wizard (e.g. extract's conflict-marker `ConfirmResolve`). `ConfirmStep.Decide` also returns a
  `skipReason` for parity.
- Business data shown in a step arrives via an **injected closure** from the command layer (the TUI
  never imports `service`/`output`), e.g. `shared.EnvFallbackDecider`, sync's `PlanPreview`.
  This detour exists **only** because a TUI package may not call the service — a migrated
  command has no closures: `flow/` calls the service itself from `Skip`/`Build`/`Load`.
  Do not add a new one; migrate instead.
- The breadcrumb denominator is **fixed** (`len(steps)`); an auto-skipped step makes the position
  **jump** (3/5 → 5/5), so the recap reliably reads `n/n`.
- **One wizard for every interactive entry path.** A command with several entry forms (a picker,
  positional args, `--all`) routes them all through the *same* wizard, skipping the steps a form
  already fixes (e.g. `sync <branches>`/`--all` skip the multi-select but keep the on-conflict
  choice and the recap). Don't leave one path on a standalone confirm while another gets the
  wizard — see `internal/flow/sync/steps.go`, where the fixed selection becomes a preset the
  recap still reads back. A genuinely post-execution decision that reacts to a runtime outcome
  (sync's push after the rebase, a failed fast-forward, an extract conflict) legitimately stays a
  standalone prompt after the run — it can't be decided upfront.
- Bypass flags follow the two-axis taxonomy below (`--yes` = confirmations/decisions,
  `--force` = safety). Under `--yes` / `--output json` / no-TTY the wizard never runs: every
  decision resolves to a flag or safe default, and a missing **required selection** errors
  naming the flag — never a picker.

### Bypass flags for mutation commands: `--yes` vs `--force` (two axes) — MANDATORY

Every worktree-mutating command (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`,
`extract`, `checkout`) exposes bypass on two **orthogonal** axes. This is the standardized model
(matches `gcloud --quiet`, `terraform -auto-approve`/`-input=false`, `apt -y` vs `--force-yes`, and
[clig.dev](https://clig.dev)); every new or refactored mutation command MUST follow it exactly
(LUC-119). Keep the axes separate — do not let one imply the other.

- **`--yes` / `-y` — confirmation/decision axis. Runs fully unattended, ZERO prompts.** Every input
  resolves without interaction, one of three ways:
  1. **Decision / confirmation** (recap, reparent, push, on-conflict, fast-forward) → flag value,
     else a documented **safe default** — never destructive: `sync --yes` does **not** push (opt in
     with `--push`); `extract --yes` defaults on-conflict to **abort**; `clean`/`prune --yes` leave
     children **orphaned** unless `--reparent-children`. Route the default through a pure rule where
     one exists (`rules.DecidePush` takes a `Yes` field).
  2. **Required selection with no safe default** (extract's `--files`/`--to`/source, sync's
     branches/`--all`) → flag/arg, else **error naming the missing flag**. A picker MUST NOT appear
     under `--yes`.
  3. A picker runs only in a **fully interactive** run (no `--yes`, TTY, human output).
- **`--force` — safety axis, strictly separate.** Only lifts safety refusals (dirty / unpushed /
  open-PR / locked). It does **not** imply `--yes`: `--force` alone still runs the wizard/recap and
  asks to confirm (thread `--force` into the wizard as a preset so it lifts refusals without
  re-asking — in the flow model it is a field of the `Request`, read where the refusal is: see
  `internal/flow/clean/steps.go`, `resolveDelete`). JSON mode requires `--yes` (confirmations
  can't run); `--force` alone in JSON is rejected.

**How to wire it — migrated commands (`flow/`): you do NOT reimplement the three cases.**
They live once, in `flow.Unattended.Ask`. Each step declares its own resolution:

| Case | Declaration on the step | Example |
| -- | -- | -- |
| 1. Decision/confirmation with a safe default | `Resolve` returns an `Answer` | env → config default; source-update → `ff` only if `--ff`, else `keep`; reparent → `orphan`; every recap → its confirm value |
| 2. Required selection, no safe default | `Resolve` returns an **error naming the flag** (a `domain` sentinel, or `fmt.Errorf` mentioning `--from`) | create's source for a pre-existing branch; `domain.ErrCleanBranchRequired` |
| 3. Interactive-only | **omit `Resolve`** | `Unattended` refuses with a message built from `Step.Label` + `Step.Flag`; it cannot open a picker |

```go
func (Unattended) Ask(session Session) (Answers, error) {
  answers := session.Presets
  for _, step := range session.Steps {
    if _, known := answers.Get(step.Key); known { continue }        // 1a — the flag answered it
    if step.Skip != nil { /* skip with its reason */ }
    if step.Resolve == nil { return Answers{}, requiredErr(step) }  // 3 — refuse, naming the flag
    answer, err := step.Resolve(answers)                            // 1b — safe default, or 2 — error
    if err != nil { return Answers{}, err }
    answers = answers.With(step.Key, answer)
  }
  return answers, nil
}
```

The command then does exactly one thing on this axis:

```go
interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes
Prompter:    flowPrompter(flowPrompterParams{Interactive: interactive}), // Unattended when false
```

`--force` never goes through `Resolve`: it is a field of the `Request`, read by the flow
where the refusal is (see `resolveDelete` in `internal/flow/clean/steps.go` — it still runs
the safety check while answering, so `--yes` alone cannot remove a dirty worktree).

**How to wire it — non-migrated commands (the legacy path):**
1. Fold `--yes` into the command's interactivity flag once, at entry:
   `interactive := isTTY && rules.IsHumanFormat(format) && !yes`.
2. Gate every picker/prompt and every `need*` step on that `interactive` flag.
3. For each required selection, add a guard: `if !interactive && <flag unset> { return
   domain.Err<X>Required }` — a sentinel in `internal/domain/errors.go` whose message names the flag
   (see `ErrExtractFilesRequired`, `ErrExtractTargetRequired`). Because `--yes` makes `interactive`
   false, the wizard is now unreachable under `--yes`, so its RecapStep is unconditional (no
   `SkipConfirm` flag on the wizard — the recap always shows when the wizard runs at all).

**Help-string wording is uniform, both paths:** `--yes` → *"Skip all prompts; resolve every
decision from flags and safe defaults (requires …; errors if a selection is missing)"*;
`--force` → *"Lift safety refusals (…); still asks to confirm unless --yes"*.

### Recap completeness: a flag must never make a line disappear

A recap builder must name **every** part of the plan, even the parts a flag resolved.

**Migrated commands get this from `Session.Presets`.** A value the `Request` already
carries is put in `Presets`; the step is then **not asked**, but `Answers` still returns
it, so the recap builder reads one source and needs no fallback:

```go
Presets: flow.NewAnswers(map[string]string{   // "" means unanswered, not answered-with-nothing
  KeyBranch: f.request.Branch,
  KeySource: f.request.From,
  KeyEnv:    f.request.EnvFrom,
}),

func (f *createFlow) recap(answers flow.Answers) string {
  source := answers.Value(KeySource) // preset or picked — the recap cannot tell, and must not
  // …
}
```

Use `Answers.Answered(key)` (asked, and not skipped — false for a preset, a `Resolve`
fallback or a skip) only when the flow genuinely needs to know whether a human saw the
question; never to decide whether to print a recap line.
Reference: `internal/flow/create/steps.go` (`createFlow.recap`), pinned by
`TestRecapKeepsEveryLineWhateverAnsweredIt`.

**Non-migrated wizards** still do it by hand: each `build*Recap` / `recapStep` reads the
value from its wizard step and **falls back to the flag/arg** when that step was skipped.
References: `internal/tui/extract` `buildCombinedRecap` (`FixedFiles`/`FixedTarget`/`FixedKeep`),
`internal/tui/newwt` `buildCreateRecap` (`BranchName`/`Source`/`EnvOverride`), `internal/tui/checkout`
`buildCheckoutRecap` (`FromOverride`/`EnvOverride`). Add the fallback whenever you add a flag
that pre-fills a step.

### Async data in a wizard: `Step.Load` (flow) — `InitCmd` / `OnEnter` (legacy)

Never block the render doing slow I/O.

**Migrated commands say only "this content comes from I/O" and let the host deal with it:**

```go
step := flow.Step{
  Kind:           flow.StepRecap,
  Key:            KeyDelete,
  LoadingMessage: domain.CleanCheckLoading,
  Load:           content,  // runs off the render path; the host shows a loading state
}
```

`Build` (synchronous) is for fast, local derivation from earlier answers; `Load` is its
slow sibling. Choosing between them can itself depend on the request — `clean` uses
`Build` when the branch was given up front (already checked) and `Load` when it was picked
(checked then and there, over the network): see `internal/flow/clean/steps.go`
`deleteStep`. The `InitCmd`/`OnEnter`/`UpdateStepModel` plumbing still exists, but it is
`internal/tui/flowui`'s business, not the command's.

**Non-migrated wizards** wire the two async entry points themselves, by when the data is
known:

- **`InitCmd` + `Loading`/`LoadingText` + `OnMsg`** — one-shot load at wizard start, for data an
  early step needs that does **not** depend on a later answer (e.g. `checkout` streams open PRs
  into step 1). Set them via `RunWizardParams`.
- **`Step.OnEnter func(prev []Step) tea.Cmd`** — fires each time the wizard *advances* into the
  step (not on back-navigation), for slow work **derived from a prior answer**: a network call or
  git work proportional to the selection. Pair it with an `OnMsg` handler:
  1. `OnEnter` returns a `tea.Cmd` emitting a request message (carrying the prior-step values).
  2. `OnMsg` on that message: `cmd := w.StartLoading("…")`; return `tea.Batch(cmd, workCmd())` — the
     work runs off the UI goroutine so the spinner (shared loading callout) animates.
  3. `OnMsg` on the result message: `w.UpdateStepModel(idx, func(any) any { return realModel })` then
     `w.SetLoading(false)`.
  Guard against a premature commit while loading: an empty `SelectList` makes `Enter` a no-op
  (`clean`'s delete step); a `ConfirmModel` needs an explicit `if w.Loading() && key=="enter"` swallow
  in `OnMsg`. Expressed as a flow step, the async work is a `Load` and the guard comes free —
  the host holds `Enter` while it runs: `internal/flow/clean/steps.go` `deleteStep` (safety
  check), `internal/flow/sync/steps.go` `confirmStep` (cascade preview).
- Keep `Build` (synchronous) for **fast, local** derivation — reserve `OnEnter` for genuinely slow
  work; over-using it is needless complexity.

### Screen-specific TUI

Each screen lives in its own package under `internal/tui/`:
```
internal/tui/
  components/     ← shared primitives (wizard, selectlist, multiselect, confirm)
  flowui/         ← runs a flow.Session as the CLI wizard (create, clean)
  dashboard/      ← `wtm ui`, the second surface over flow/
  newwt/          ← create wizard (still used by extract's embedded sub-flow)
  checkout/       ← checkout wizard (PR picker → parent → env)
  runpicker/      ← run list / ps pickers
  runwizard/      ← run job / profile wizards
  inittui/        ← global + project init wizards
  clean/          ← deletion confirm
  extract/        ← extract wizard
  relocate/       ← relocate wizard
  worktreepicker/ ← shared worktree-selection picker
```

### Rules
- `Update` is a pure function — no side effects, only return `(tea.Model, tea.Cmd)`
- Never import `lipgloss` in TUI models — delegate all styling to `styles/`
- Never import `cobra` or `service` inside a `tui/` model
- TUI packages may import `domain/` (types), `styles/` (rendering) and `flow/` (to run a
  session — that is what `flowui` and `dashboard` do); a flow never imports a TUI package
- A surface that runs a flow off the UI goroutine reaches the model **only** through
  `tea.Msg` — see `internal/tui/dashboard` (`prompter` replies over a channel,
  `presenter` posts one `OutputLineMsg` per line via `flow.LineWriter`)

---

## Lipgloss — Centralized Styles

All styles live in `internal/styles/`, split by concern:
- `colors.go` — adaptive color palette
- `text.go` — typography styles + `Indent` constant
- `components.go` — composed styles (list items, badges, inputs)
- `indicators.go` — status indicators (DirtyIndicator, CleanIndicator)

The `Indent` constant is the canonical source for left-padding:
```go
// internal/styles/text.go
const Indent = "  "
```

`output/block.go` aliases it as `output.Indent`. TUI components use `styles.Indent`.
Never write a literal `"  "` for padding — always use the constant.

---

## Output — Formatted Printing

`internal/output/block.go` provides shared helpers for structured terminal output:

```go
output.Blank(w)                    // empty line — use ONLY as an inter-section separator
output.Success(w, "Done")          // ✓ Done
output.Warning(w, "Be careful")    // ! Be careful
output.Error(w, "Failed")          // ✗ Failed
output.Message(w, "Info")          // plain indented line
output.SectionTitle(w, "TITLE")    // bold title
output.InfoLine(w, "key", "value") // key  value
output.Announce(w, "Title", items) // raw block: title + key-values (no outer blanks)
```

JSON output uses `encodeJSON(w, v)` (pretty-printed, no HTML escaping).

### Vertical spacing — the frame (LUC-87)

Vertical top/bottom padding is centralized. Each command frames its human output
**exactly once**; helpers and table formatters return **raw** bodies with no outer
blank lines. The left padding stays in the primitives (`Indent`); the frame owns
only the top/bottom.

```go
// Simple buffered output — one leading + one trailing blank line:
output.Frame(w, func() {
    output.Success(w, "Created worktree feature-x")
})

// Streaming / split-stream (plan on stderr, result on stdout) — explicit pair:
output.FrameStart(cmd.ErrOrStderr())
output.FormatSyncPlan(cmd.ErrOrStderr(), plan)   // raw
// … spinner, work …
output.FormatSyncResult(cmd.OutOrStdout(), result) // raw
output.FrameEnd(cmd.OutOrStdout())
```

Rules:
- **Human output is framed once** via `Frame` (closure) or `FrameStart`/`FrameEnd`
  (streaming). Route on `rules.IsHumanFormat(format)`.
- **JSON and machine output are never framed** — `--output json` and shell-eval
  paths (`resolve` success, `shell-init`) stay strictly flush.
- **Helpers/formatters return raw bodies** — no leading/trailing `output.Blank`.
  `output.Blank` is allowed only as a genuine *inter-section* separator inside a body.
- **No stacked blanks** (`\n\n\n`+). Spinners do not self-pad — the frame owns the
  leading blank, so open the frame before starting a spinner.
- **A block has to earn its place**: it prints when it changes what the reader does next. Success contracts to a count, anomalies are named one by one; detail belongs to the command whose subject it is (ports → `wtm env`, not `create`); a successful run has a fixed shape whatever happened. See CLAUDE.md, "What a block of output has to earn".
- **A hook phase is shown, not kept**: `output.HookView` draws a bounded tail and replaces it with one result line per hook, logging the whole stream to `<state-dir>/hooks.log`. Terminals only (`output.IsTerminal`) — a pipe or `--output json` gets the raw stream. The phase reports through `flow.HookSink` (output + `domain.HookBeat`); `service/hooks` renders only its no-reporter fallback.
- **TUI views own their single top/bottom blank** (`WizardModel`/`standaloneModel`
  both open with one leading `\n`); don't add a manual blank before launching a wizard.

---

## Testing Patterns

### Unit tests — config/service/domain

Use `t.TempDir()` as the state dir, write fixtures directly inside it (no nested `.wtm/` segment):

```go
func TestMyFeature(t *testing.T) {
  stateDir := t.TempDir()
  // write test fixtures
  os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte(content), 0o644)
  // call function under test — config layer takes a state dir
  result, err := config.LoadRun(stateDir)
  // assert
}
```

### Integration tests — command-level via Cobra

Use `gittest.InitRepo` for the git repo, plus both `WTM_PROJECT_DIR` and `WTM_STATE_DIR` so the command short-circuits any git lookup:

```go
func TestRunExportEmpty(t *testing.T) {
  dir := gittest.InitRepo(t)
  stateDir := filepath.Join(dir, ".git", "wtm")
  t.Setenv("WTM_PROJECT_DIR", dir)
  t.Setenv("WTM_STATE_DIR", stateDir)

  // Write minimal config.toml into the state dir
  config.WriteProject(config.WriteProjectParams{StateDir: stateDir, Answers: ...})

  // Execute command via Cobra
  cmd := NewRunCmd()
  var out bytes.Buffer
  cmd.SetOut(&out)
  cmd.SetArgs([]string{domain.CmdExport})
  err := cmd.Execute()

  // Assert on stdout JSON
  var got domain.RunConfig
  json.Unmarshal(out.Bytes(), &got)
}
```

### Flow tests — a whole command run, no TUI

A migrated command is tested by calling `<cmd>.Run` (or the unexported flow struct) with
the doubles for the two seams, from `internal/testutil/flowtest`. No terminal, no
Bubbletea program, no golden file.

```go
prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
  create.KeyBranch: "feat/x",
  create.KeySource: "main",
}}
recorder := &flowtest.Recorder{}
```

- **`ScriptedPrompter`** walks the session the way a real host does — honoring `Skip`,
  `Build` and `Load` — and answers each step from the script. It records `Asked`
  (`AskedKeys()` gives a one-line assertion on *which questions were put*) and the
  `StepContent` each step produced, so a test can assert on the recap prose the user
  would have read. A step with nothing scripted is an **error**: a new question cannot
  slip into a flow unnoticed. `Abort: true` simulates the user backing out.
- **`Recorder`** implements `flow.Presenter` and collects `Stages`, `Hooks`, `Notices`,
  `Statuses`. It runs `Work()` and `Run(sink)` for real, so the service still executes.
- The **typed conclusion** is not on `Recorder` — embed it and add the one method:
  `type rec struct{ *flowtest.Recorder; got create.Outcome }` +
  `func (r *rec) Created(o create.Outcome) error { r.got = o; return nil }`.
- For the unattended path, `flow.Unattended{}` **is** the double: pass it directly and
  assert that each required selection refuses naming its flag, and each defaulted decision
  lands on the safe value (`internal/flow/unattended_test.go`).

### Characterization tests — before you refactor, not after

A refactor that moves a command's flow between packages must not change what a user sees. Pin the
observable behavior **first**, against the old code, then move the code and run those tests
unchanged. That is what `internal/tui/newwt/create_flow_test.go`,
`internal/commands/wt/create_wizard_test.go`, `create_noninteractive_test.go` and
`integration_test.go` are for: step composition per flag combination, recap completeness,
the `--yes`/`--force` axes, the JSON reparent default, idempotence on an absent worktree.

Do not "fix" one of them to make a refactor pass — a characterization test that had to be
edited is a behavior change, and needs to be named as one.

### Shared test helpers

`internal/testutil/gittest/gittest.go`:
```go
gittest.InitRepo(t)           // temp git repo with initial commit
gittest.CreateBranch(t, dir, name) // create a local branch
```

`internal/testutil/flowtest/flowtest.go`: `ScriptedPrompter` (a `flow.Prompter`) and
`Recorder` (a `flow.Presenter`) — see above.

---

## Project Checklist

Before calling `build-validator`, verify manually:
- [ ] All new types are in `internal/domain/`
- [ ] All flag and command names use constants from `constants.go`
- [ ] No function has more than 1 unstructured parameter
- [ ] All external input is validated in `internal/config/` before reaching service
- [ ] No nested conditionals — early returns throughout
- [ ] No type assertions without comma-ok
- [ ] No business logic in `commands/` or `tui/`
- [ ] A mutation command's flow is in `internal/flow/<cmd>/`, not in its runner
- [ ] `internal/flow/` imports no cobra/bubbletea/lipgloss, no `output`/`tui`/`config`/`commands`
- [ ] Every `flow.Step` has a deliberate `Resolve` (safe default, flag-naming error, or absent on purpose)
- [ ] No `lipgloss` imports outside `internal/styles/`
- [ ] No `cobra` or `bubbletea` imports inside `internal/service/`
- [ ] Pure functions (no I/O) live in internal/rules/, not in service/
- [ ] All async service calls in TUI wrapped as `tea.Cmd`
- [ ] `shared.AddOutputFlag(cmd)` used instead of manual flag registration
- [ ] `styles.Indent` used instead of literal `"  "` for padding
- [ ] New/renamed/removed command has a `GroupID`, and `make docs` + README overview were updated

Then run **`build-validator`**.
