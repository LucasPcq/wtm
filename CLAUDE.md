# CLAUDE.md — Go CLI Development Principles

Mandatory coding standards for `wtm`. When in doubt, consult the `go-cli` skill (`.claude/skills/go-cli/SKILL.md`). Use the fff MCP tools for all file search operations instead of default tools.

**Read on demand** — `docs/dev/` is the developer reference; open the page before working on its topic:

| Read | Before |
| -- | -- |
| [`docs/dev/architecture.md`](docs/dev/architecture.md) | adding a package, an import between layers, or a `service→service` edge (annotated package map) |
| [`docs/dev/commands.md`](docs/dev/commands.md) | touching `internal/kernel/`: the engine's contract (fields, rules, errors and their codes, results, `Each` / sagas) |
| [`docs/dev/dispatch.md`](docs/dev/dispatch.md) | touching `internal/dispatch/`: the registry, the run order, operation locks and holders, the fingerprint, operation events, the test bench |
| [`docs/dev/git.md`](docs/dev/git.md) | touching `internal/infra/git` or reaching git from a layer on the engine (`git.Repo`, `Reader` / `Writer`, `Ref`, `*git.Error`, the double `gittest.Fake`, `gitexec`) |
| [`docs/dev/services.md`](docs/dev/services.md) | touching `service/gitwt`, `service/identity`, `service/runenv` or `hooks.Execute`, the services a command on the engine reaches a worktree through (reader or mutator, the snapshot, `Allocate`, what publishes) |
| [`docs/dev/store.md`](docs/dev/store.md) | writing `meta.json`, `config.toml` or a managed `.env` (`store.Meta`, `ProjectConfig`, `EnvFiles`, the double `storetest.Fake`) |
| [`docs/dev/state-files.md`](docs/dev/state-files.md) | writing any file, or changing the format of a state file (`fs.UpdateFile`, `fs.Change`, locks, schema versions) |
| [`docs/dev/form.md`](docs/dev/form.md) | touching `internal/surface/tui/form`: the interactive form of a command on the engine (its loop over `dispatch`, which field it asks, a field per type, remembering, the recap and the run) |
| [`docs/dev/flow-layer.md`](docs/dev/flow-layer.md) | touching anything under `internal/flow/`, a Prompter/Presenter, a step kind, or the dashboard's run of a flow |
| [`docs/dev/adding-a-mutation-command.md`](docs/dev/adding-a-mutation-command.md) | adding a worktree-mutating command |
| [`docs/dev/cli.md`](docs/dev/cli.md) | touching `internal/surface/cli` itself: the root (signals, `--quiet`, exit code), a command generated from the registry (`cli.Bind`, its flags, its run), parity |
| [`docs/dev/output.md`](docs/dev/output.md) | adding a command or changing what one prints (frame, glyphs, block helpers, `--quiet`, JSON contract, hook view, the generic rendering and the v2 envelope of a command on the engine) |
| [`docs/dev/lint.md`](docs/dev/lint.md) | adding an `archlint` rule or an exception, or when `make lint` fails and the reason is unclear |
| [`docs/dev/changelog.md`](docs/dev/changelog.md) | writing a `CHANGELOG.md` entry |
| [`docs/dev/run-addressing.md`](docs/dev/run-addressing.md), [`docs/dev/shared-services.md`](docs/dev/shared-services.md) | touching the run proxy / addressing, or `scope = "shared"` jobs |

**Self-maintaining docs:** when a structural decision changes (new package, renamed layer, new dependency, new convention), update this file, the relevant `docs/dev/` page and/or the skills (`go-cli`, `build-validator`) in the same session. Standards must reflect the actual codebase.

## Docs & README

- **User-facing agent skill** — `internal/surface/cli/agents/assets/using-wtm/` is shipped to end users so their LLM can drive `wtm`: a short `SKILL.md` (driving rules, exit codes, which reference to read when) and `references/*.md` (worktrees, stacks, run, run-config, json). A new fact goes in the reference whose topic it is, never in `SKILL.md` unless every task needs it. Update it in the same session whenever the command surface or agent-relevant behaviour changes (new/renamed command or flag, `--output json` shape, failure/abort semantics, interactive-vs-non-interactive behaviour). Skip internal refactors and TUI-only changes.
- **`docs/wtm_*.md` is generated** by `tools/gendocs` from the Cobra tree — never hand-edit it. Whenever a command or flag is added, modified or removed: run `make docs`; if a command was added/renamed/removed, update the `README.md` overview table (grouped like the root `--help`). Never re-add per-command flag tables to the README.
- **`docs/dev/`** (developer docs) and **`docs/guide/`** (user guide: configuration, isolation, jobs and profiles, `wtm run`, shared services, addressing, `run.toml`, state files, migration notes) are hand-written. A concept a user needs and no `--help` can carry goes in the guide, linked from the README. Update both in the same change as the behaviour they describe.
- **The docs site** (`site/`, Starlight) is built from `docs/`, `README.md` and `CHANGELOG.md` as they are: `site/scripts/sync-docs.mjs` only adds front matter and turns repository links into site routes, so never write site-only Markdown in `docs/`. The landing page (`site/src/pages/index.astro`, sections in `site/src/components/landing/`) is a plain Astro page outside Starlight, like `privacy.astro` and `terms.astro` beside it (keep them true to what wtm sends over the network); its numbers come from `site/src/lib/facts.mjs` (git tags, `CHANGELOG.md`), `machine.mjs` (what wtm writes per worktree: ports, `.env` values, job environment) and `dashboard.mjs` (a `wtm ui` frame), tested by `npm test`; its terminals replay output captured with the `wtm-sandbox` skill, so re-capture them when a shown command's output changes. The docs (`src/styles/theme.css`) share its fonts, colours and branch mark. The docs home is `site/src/docs-home.mdx`, published at `/docs/` with `__BASE__` replaced. `DOMAIN` in `site/site.config.mjs` sets the site URL, the base path, the `CNAME` and the contact address; the command reference sidebar follows `docs/commands.json` (written by `make docs`); each release minor before the current one is archived from its tag (`site/site.config.mjs`), and `internal/schemas/*.json` are published under `/schemas/`. `make site` builds it (links validated), `make site-dev` serves it; the `docs-site` workflow publishes it to GitHub Pages on each release tag.
- **`README.md` is the product page**, not a reference: pitch, one short section per feature with its GIF, install, quick start, the command-overview table. Anything longer belongs in `docs/guide/`. GIFs are recorded from `docs/demos/*.tape` (VHS, `docs/demos/setup.sh`): when a change alters what a recorded command prints, run `make demos`.
- **`CHANGELOG.md`** is in English, in Keep a Changelog shape, following [`docs/dev/changelog.md`](docs/dev/changelog.md) (template + rules): a few one-line bullets per release linking into the guide, the migration detail in `docs/guide/migrating-to-<version>.md`. Each release section is published as the GitHub release notes by `make release-notes VERSION=x.y.z`.
- **Markdown:** never hard-wrap prose — one paragraph is one line.

## 1. Immutability first

Prefer `:=` for values that do not change. `var` only for zero-value initialization or package-level declarations. If a block reassigns a variable, extract it into a function.

## 2. Structs for 2+ inputs

A function taking 2 or more of **its own** inputs takes a single `<Name>Params` struct, initialized with named fields.

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

- **Carriers don't count:** `io.Writer`, `context.Context`, `testing.TB`, `*cobra.Command` are plumbing. `render.Warning(w, text)` respects the rule.
- **Where it bites:** misorderable neighbours (`RenameJobRefs(cfg, from, to)` — two strings the compiler lets you swap) and a list that grew (every service and flow entry point takes `<Name>Params`).
- **Symmetric pairs are exempt:** `differ(a, b string)`, `MergeRunConfigs(a, b)`, `ClampIndex(index, length int)` — the order is the meaning.
- This is a **review rule, not a lint rule**, deliberately (measured: see `docs/dev/lint.md`).

## 3. Shared types — no duplication

Types, enums, sentinel errors and constants are defined once in `internal/domain/`. Pure functions with no I/O (lookups, transforms, classification) live in `internal/rules/`. The engine's contract is the exception: `internal/kernel/` declares its own vocabulary (`Kind`, `Status`, `Reason`, `FieldType`, the `kernel.Code` constants), and a migrated command declares its codes as `kernel.Code` constants beside it, each with its message in `kernel/text`.

## 4. Validate all external input

Config files, CLI flags and environment variables are validated at the boundary (`config/` or command entry) with guard clauses. A command on the engine declares its checks as data: per-field `Constraints` in its `FieldSpec`, rules across fields as `kernel.Rules` combinators, both producing `kernel.FieldError` codes (`docs/dev/commands.md`). The service layer receives only clean data.

## 5. Centralized constants — no magic strings or numbers

Every string key, exit code, flag name, env var name and format identifier is a named constant in `internal/domain/constants.go`, except the engine's codes and params, which live in `internal/kernel/` and beside each migrated command, and what only the generated command line knows (`--expect-fingerprint`, the exit codes 23–25 of the engine's kinds), declared in `internal/surface/cli`.

```go
// ❌ os.Exit(1); cmd.Flags().String("output", ...)
// ✅ os.Exit(domain.ExitCodeError); cmd.Flags().String(domain.FlagOutput, ...)
```

## 6. Early returns — no nesting

Every error or guard returns immediately; the happy path is last. Never nest `if` blocks.

## 7. No unsafe type assertions

Always comma-ok (`s, ok := v.(string); if !ok { return fmt.Errorf("expected string, got %T", v) }`). Prefer typed interfaces and concrete structs over `any`. Type at the source, not downstream. A value that takes one of a closed set of shapes is a sealed interface (an unexported method) with one type per shape, declared `//go-sumtype:decl` on a line of its own so `make lint` checks every type switch over it (`kernel.Error`, `kernel.Value`) — never one struct whose fields are valid only for some value of another.

## 8. Comments — the exception, not the rule

Aim for near-zero comments; encode meaning in names and signatures (`Skip func(Answers) (skip bool, reason string)` needs no prose). Write one only for: **why, never what** (a non-obvious decision, an ordering constraint, an invariant); a workaround with its reference; a one-line package comment; godoc on an exported symbol only when its name and signature leave a caller guessing. Architecture belongs in `docs/`, not in header comments. `internal/flow` (~10% comment lines) is the density ceiling for a package. When you modify a file, delete the comments in it that restate the code.

## 9. Clean architecture layers

```
main.go         the entry point: cli.Execute
internal/
  domain/       types, errors, constants only
  rules/        pure functions (stdlib + domain only, no I/O); a new decision goes in the sub-package
                of its subject (rules/worktree, rules/runenv, rules/statefile), never at the root
  kernel/       the command engine's skeleton (stdlib only): fields, rules, errors, results, Each / sagas;
                text/ its message catalogue, kerneltest/ the checks every command runs
  dispatch/     the engine's impure half, the one way a surface reaches a command: registry,
                locks, Observe → Validate → Plan → fingerprint → Apply, operation events
  query/        the reads on the engine (operations/)
  config/       load & validate config.toml, run.toml, global config; writes the JSON schema beside each
  store/        the one writer of each state file: meta.json, config.toml's changes, the managed .env files
  flow/         each command's flow, surface-independent (one package per command)
  service/      impure orchestration: git exec, I/O, hooks, the run daemon, the event bus; gitwt,
                identity, runenv and hooks.Execute are the engine's, a reader or a mutator each
  styles/       all Lipgloss styles
  infra/        git/ the one way the engine reaches git (git.Repo); fs/ the only writer of state files
                (UpdateFile), proc/ processes and the Shield; at its root, the git functions of the
                commands still on flow/
  surface/      the interfaces
    cli/        the root (signals, --quiet, exit code); the commands generated from the
                dispatch registry (cli.Bind); flag wiring → flow for the others (zero business logic)
      render/   format and print results (zero decision logic); the generic rendering of a
                kernel result and the v2 envelope
    tui/        Bubbletea models (rendering only): form (the form of a command on the engine),
                flowui (the wizard of a command on flow/), dashboard (`wtm ui`), runview
```

The annotated map of every sub-package is in `docs/dev/architecture.md`.

**Hard rules** (the import and call rules are checked by `make lint`, `tools/archlint`):
- `surface/cli/` has zero business logic.
- `domain/` has types, errors and constants only — no methods, no free functions.
- `rules/` imports only stdlib and `internal/domain` — no I/O, no side effects.
- `kernel/` imports only the stdlib — no `domain/`, no business vocabulary: a command composes `kernel` with `domain`, never the reverse. I/O reaches it only as functions it is handed (`Observe`, `Apply`, the `Shield`). The engine writes no text: an error, a warning or a progress event is a `kernel.Code` plus params, and `kernel/text` holds the English.
- `dispatch/` imports only `kernel/`, `domain/`, `rules/`, `infra/` and `query/`: a command's ports reach it as interfaces. `command/` and `query/` never import `dispatch/` or `surface/`, `surface/` never imports them, and `command/` never calls `context.Background()`/`context.TODO()` (`engine`). See `docs/dev/dispatch.md`.
- `service/` never imports `cobra`, `bubbletea`, `lipgloss`.
- `surface/cli/render/` and `surface/tui/` have zero decision logic — only rendering. `render` reads results from `kernel/`, never runs anything through `dispatch/`.
- `surface/tui/form` reaches a command through `dispatch/` only (`Entry.Fillable`), never `flow/` or `service/`: it asks `State` and `Choices` what a field is rather than deciding it.
- `styles/` is the only package allowed to instantiate `lipgloss.Style`.
- `flow/` imports **only** `internal/service/`, `internal/rules/`, `internal/domain/` and the stdlib — never cobra, bubbletea, lipgloss, `surface/`, `config/` or `infra/`. Need something from `infra/`? Add a thin `service/` wrapper (e.g. `worktree.FindByBranch`).
- `service/x` imports `service/y` only along an edge declared in `tools/archlint` (`serviceEdges`).
- The daemon (`service/process`, `service/proxy`) is blind to git: no `service/worktree|branch|github|events`, no `config`, only allow-listed `infra/` functions.
- A service **mutator** (the table in `tools/archlint/chokepoint.go`) is called only from `internal/flow/`.
- git is run by `infra/git` only (`gitexec`): a layer on the engine reaches it through `git.Reader` / `git.Writer`; the git functions at the root of `infra/`, for the commands still on `flow/`, are tracked in `.archlint-migrating` and leave with their last caller. See `docs/dev/git.md`.
- `meta.json`, the changes to `config.toml` (the remembered answers, `base_path`, the `.env` targets) and the managed `.env` files are written by `store/` only, each through its interface (`store.Meta`, `ProjectConfig`, `EnvFiles`) doubled by `storetest.Fake`; `config/` loads and validates, `init`'s full render stays in `config/` until its migration. See `docs/dev/store.md`.
- **A file is named after what it declares**, and a port lives with its implementation: its interface, the types it takes and returns and the implementation share one file (`infra/git/read.go`, `store/meta.go`), never an interfaces-only file beside the code. A type a whole package turns around (`dispatch.Handle`) has its own file; options sit beside the call that takes them.
- A file is written through `infra/fs` only: `fs.UpdateFile(ctx, {Path, Mutate})` (lock, read, `Mutate` decides an `fs.Change`, atomic rename), never `os.WriteFile`/`os.Create`/`os.Rename` outside `infra/` (`statewrite`). A state file carries `schema` (absent = 1, written from 2 on) and a write refuses a newer one (`*statefile.NewerError`, exit 22). See `docs/dev/state-files.md`.

**Every new worktree-mutating command goes through flow/** — no exception. Declare `Request`/`Outcome`/`Presenter`/`Params`/`Run` in `internal/flow/<cmd>/` and its questions as `flow.Step` in `steps.go`; the runner in `surface/cli/` is only flags → `Request` → pick the Prompter and Presenter → `<cmd>.Run`. A runner that inspects state, orders service calls or gates a picker on `interactive` has put the flow in the wrong layer. A flow never frames, never animates, never picks a stream, and **returns** errors rather than presenting them. Recipe: `docs/dev/adding-a-mutation-command.md`; seams and step model: `docs/dev/flow-layer.md`.

**Output** — each command frames its human output exactly once (`render.Frame`, or `FrameStart`/`FrameEnd`) and the body writes to the writer `Frame` hands it; JSON and machine output are never framed. A block prints only if it changes what the reader does next: success contracts to a count, anomalies are named one by one. Glyph runes come from `domain` constants, the glyph carries the only colour on its line, and `Muted` is for chrome and non-event lines only. Full rules: `docs/dev/output.md`.

**A command on the engine** gets its command line from `cli.Bind` (`surface/cli/<name>.go`): a flag per field, one run, the generic rendering and the v2 envelope for all. It adds only its `cli.Doc`, a positional, a renamed flag, and its `render.Detail` or `GenericDetail()`; every entry of the registry is bound or declared unbound (parity test). See `docs/dev/cli.md`.

**Designating a worktree** — the subject is **positional**; a worktree that is not the subject is a **flag named after its role** (`extract [source] --to`, `create --from`, `run up [worktree] --profile`). Omitted positional → the current directory when that is a safe default for the command (`run`), a picker otherwise (`clean`). A resolved worktree is always the root as git spells it (`infra.Toplevel`), never `os.Getwd()`.

### Bypass flags — two orthogonal axes

Every worktree-mutating command (`create`, `clean`, `sync`, `fast-forward`, `prune`, `relocate`, `reparent`, `extract`, `checkout`, `env`, plus `init`/`run init`) follows this model:

- **`--yes` / `-y` — the confirmation axis: fully unattended, zero prompts.** Each input resolves as:
  1. **Decision / confirmation** → its flag, else the **remembered answer** (`[wizard.remembered]`, ticked "Always use this answer" in the wizard), else a documented **safe default**, never destructive (`sync --yes` does not push; `extract --yes` aborts on conflict; `clean`/`prune --yes` leave orphans unless `--reparent-children`). Only a `StepSelect` opting in with `Step.Memory` remembers, never a confirmation or a destructive option (`rules.RememberableValues`); `--ask` ignores the memory for one run.
  2. **Required selection with no safe default** → its flag/arg, else an **error naming the missing flag**. Never fall back to a picker.
  3. A picker runs only in a **fully interactive** run (no `--yes`, TTY, human output).
- `--yes` is the only spelling of that axis — no `--non-interactive`. JSON mode requires `--yes` on a command still on `flow/`; on a command on the engine JSON is unattended already, and `--yes` is accepted without effect.
- **`--force` — the safety axis, strictly separate.** It only lifts safety refusals (dirty / unpushed / open PR / locked). It does **not** imply `--yes`: `--force` alone still runs the wizard and asks to confirm, with `--force` threaded in as a preset.

Implementation: `interactive := isTTY && rules.IsHumanFormat(format) && !yes`; a migrated command expresses case 2 as a step `Resolve` that names the flag (see `internal/flow/sync/steps.go`). Decision defaults go through a pure rule where one exists (`rules.DecidePush`).

**Recap completeness:** a flag or a remembered answer never makes a recap line disappear, nor its line in the wizard trail — the recap reads the step's answer, else the flag/arg that resolved it (`Session.Presets`, or `Step.Given` for a step its `Skip` can rule out). A flag answers its step, never only preselects it. **Re-init completeness:** a re-init step shows the complete candidate list, pre-filled from the config on disk, and reads an answer that may be empty as `(value, asked)` so a step never reinstates what the user removed. Details: `docs/dev/flow-layer.md`.

## 10. Commit messages in English

Every commit message — subject and body — is in English, whatever language the conversation was held in. `CHANGELOG.md` is in English too (see Docs & README).

## 11. Validate before commit

```
make lint          # fmt + vet + arch (tools/archlint) + dead (deadcode) + sumtype (go-sumtype) + staticcheck — all gating
make test          # go test ./... -race -count=1
make docs          # regenerate docs/ from the Cobra tree
make demos         # re-record the README GIFs (needs vhs)
make release-notes VERSION=x.y.z   # print a CHANGELOG section as release notes
make dupl          # clone report, informative only
```

- Tests of the engine and of the layers built for it (`internal/kernel/`, `infra/fs`, `infra/git`, the `statefile` packages, `dispatch`, the generated command line in `surface/cli` and its rendering, the form in `surface/tui/form`, `store/`, the engine's services `service/gitwt`, `identity`, `runenv`, `hooks.Execute` and the `rules/` sub-packages…) use testify (`assert`, `require`); each test file holds only `Test…` functions, the example and fakes in `fixture_test.go`, shared helpers in `helpers_test.go`. The rest of the repository keeps stdlib `testing`.
- A new architectural rule goes into `tools/archlint`, not into a paragraph here. `.archlint-migrating` and `.deadcode-ignore` may only shrink; a new entry needs a stated reason. See `docs/dev/lint.md`.
- `.claude/hooks/pre-commit-gates.sh` runs `make lint` and a `go mod tidy` check on every `git commit` and blocks it on failure. It does not run the tests. `WTM_SKIP_GATES=1 git commit …` only when the gate itself is wrong.
- **Invoke the `build-validator` subagent before marking any task done** — it adds the `-race` test suite, dependency hygiene and the duplication report.
- **See it run before calling it done** — the `wtm-sandbox` skill drives the built binary in an isolated sandbox (tmux, VHS). **Open a PR with the `open-pr` skill**: base from `wtm tree`, that proof attached, the short body template.
- **Several issues at once** — the `orchestrate` skill makes this session the orchestrator: one wtm worktree and one Claude worker per Linear issue in herdr panes, product questions relayed to the user, PRs verified before they are reported.
