# CLAUDE.md — Go CLI Development Principles

Mandatory coding standards for `wtm`. How Go is written follows the `go-style` skill (`.claude/skills/go-style/`: community conventions, every rule sourced); the architecture of `next/` follows the `go-architecture` skill (`.claude/skills/go-architecture/`). The rules below are this repository's own; where one departs from `go-style` (§2 Params structs, §8 comments), it governs `internal/` only, and `next/` follows `go-style`. Use the fff MCP tools for all file search operations instead of default tools.

**Read on demand** — `docs/dev/` is the developer reference; open the page before working on its topic:

| Read | Before |
| -- | -- |
| [`docs/dev/architecture.md`](docs/dev/architecture.md) | adding a package, an import between layers, or a `service→service` edge (annotated package map) |
| [`docs/dev/store.md`](docs/dev/store.md) | writing `meta.json`, `config.toml` or a managed `.env` (`store.Meta`, `ProjectConfig`, `EnvFiles`) |
| [`docs/dev/state-files.md`](docs/dev/state-files.md) | writing any file, or changing the format of a state file (`fs.UpdateFile`, `fs.Change`, locks, schema versions) |
| [`docs/dev/flow-layer.md`](docs/dev/flow-layer.md) | touching anything under `internal/flow/`, a Prompter/Presenter, a step kind, or the dashboard's run of a flow |
| [`docs/dev/adding-a-mutation-command.md`](docs/dev/adding-a-mutation-command.md) | adding a worktree-mutating command: vocabulary, rules, the flow package, its wiring, presenter, tests |
| [`docs/dev/cli.md`](docs/dev/cli.md) | touching `internal/surface/cli` itself: the root (signals, `--quiet`, exit code), flag wiring |
| [`docs/dev/output.md`](docs/dev/output.md) | adding a command or changing what one prints (frame, glyphs, block helpers, `--quiet`, JSON contract, hook view) |
| [`docs/dev/lint.md`](docs/dev/lint.md) | adding an `archlint` rule or an exception, or when `make lint` fails and the reason is unclear |
| [`docs/dev/identity.md`](docs/dev/identity.md) | touching the landing, the docs theme, the README pitch, the social preview or any launch material (positioning, voice, colours, type, logo, layout) |
| [`docs/dev/changelog.md`](docs/dev/changelog.md) | writing a `CHANGELOG.md` entry |
| [`docs/dev/run-addressing.md`](docs/dev/run-addressing.md), [`docs/dev/shared-services.md`](docs/dev/shared-services.md) | touching the run proxy / addressing, or `scope = "shared"` jobs |

**Self-maintaining docs:** when a structural decision changes (new package, renamed layer, new dependency, new convention), update this file, the relevant `docs/dev/` page and/or the skills (`go-style`, `go-architecture`) and the `build-validator` agent in the same session. Standards must reflect the actual codebase.

## Docs & README

Two skills carry the detail: **`wtm-docs`** for any documentation change (where a fact belongs, the guide page shape, the guide index, audits) and **`wtm-site`** for the landing, the legal pages, the docs theme and the README pitch (identity rules in code, before/after screenshots). What every change must respect:

- **Docs follow the code in the same change.** A command, flag, output or behaviour that changes updates its docs in the same session: `make docs` regenerates `docs/wtm_*.md` and `docs/commands.json` from the Cobra tree (never hand-edit them), the guide (`docs/guide/`) and developer docs (`docs/dev/`) are hand-written.
- **User-facing agent skill** — `internal/surface/cli/agents/assets/using-wtm/` ships to end users so their LLM can drive `wtm`: a short `SKILL.md` and `references/*.md` by topic. Update it whenever the command surface or agent-relevant behaviour changes (command or flag, `--output json` shape, failure/abort semantics, interactive vs non-interactive). Skip internal refactors and TUI-only changes.
- **The docs site** (`site/`, Starlight) is built from `docs/`, `README.md` and `CHANGELOG.md` as they are: never write site-only Markdown in `docs/`. The guide sidebar is built from the sections of `docs/guide/README.md`, so a guide page not listed there does not appear. `make site` builds it and validates links, `make site-dev` serves it; the `docs-site` workflow publishes it on each release tag. `site/site.config.mjs` holds the domain, the base path and the archived versions.
- **The landing and the README** follow `docs/dev/identity.md`; their terminals and visuals replay real output (the `wtm-sandbox` skill, `make demos` for the README GIFs), re-captured when a shown command's output changes.
- **`CHANGELOG.md`** follows [`docs/dev/changelog.md`](docs/dev/changelog.md); each release section becomes the GitHub release notes (`make release-notes VERSION=x.y.z`).
- **Markdown:** English, never hard-wrapped — one paragraph is one line.

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

Types, enums, sentinel errors and constants are defined once in `internal/domain/`. Pure functions with no I/O (lookups, transforms, classification) live in `internal/rules/`.

## 4. Validate all external input

Config files, CLI flags and environment variables are validated at the boundary (`config/` or command entry) with guard clauses. The service layer receives only clean data.

## 5. Centralized constants — no magic strings or numbers

Every string key, exit code, flag name, env var name and format identifier is a named constant in `internal/domain/constants.go`.

```go
// ❌ os.Exit(1); cmd.Flags().String("output", ...)
// ✅ os.Exit(domain.ExitCodeError); cmd.Flags().String(domain.FlagOutput, ...)
```

## 6. Early returns — no nesting

Every error or guard returns immediately; the happy path is last. Never nest `if` blocks.

## 7. No unsafe type assertions

Always comma-ok (`s, ok := v.(string); if !ok { return fmt.Errorf("expected string, got %T", v) }`). Prefer typed interfaces and concrete structs over `any`. Type at the source, not downstream. A value that takes one of a closed set of shapes is a sealed interface (an unexported method) with one type per shape, declared `//go-sumtype:decl` on a line of its own so `make lint` checks every type switch over it (`fs.Change`, `envcopy.Warning`) — never one struct whose fields are valid only for some value of another.

## 8. Comments — the exception, not the rule

Encode meaning in names and signatures (`Skip func(Answers) (skip bool, reason string)` needs no prose).

- **What stays:** a quirk of git, macOS or `flock` the code cannot show; a workaround with its reference; an ordering invariant or a non-obvious decision (**why, never what**); a one-line package doc; godoc on an exported symbol whose name and signature leave a caller guessing.
- **What goes:** a comment that restates the name or the code; a doc comment on an unexported declaration (its why moves into the body, beside the line it explains); architecture prose (→ `docs/dev/`); section banners and copied headers in test files.

When you modify any file, delete the comments in it that restate the code.

## 9. Clean architecture layers

```
main.go         the entry point: cli.Execute
internal/
  domain/       types, errors, constants only; a new one goes in the sub-package of its subject
                (domain/statefile), never at the root, which only shrinks
  rules/        pure functions (stdlib + domain only, no I/O); a new decision goes in the sub-package
                of its subject (rules/statefile, rules/envcopy), never at the root
  config/       load & validate config.toml, run.toml, global config; writes the JSON schema beside each
  store/        the one writer of each state file: meta.json, config.toml's changes, the managed .env files
  flow/         the flow of each command, surface-independent (one package per command)
  service/      impure orchestration: git exec, I/O, hooks, the run daemon, the event bus
  styles/       all Lipgloss styles
  infra/        git, filesystem and process wrappers; fs/ the only writer of state files (UpdateFile),
                proc/ processes and the Shield
  surface/      the interfaces
    cli/        the root (signals, --quiet, exit code); flag wiring → flow (zero business logic)
      render/   format and print results (zero decision logic)
    tui/        Bubbletea models (rendering only): flowui (the wizard of a command), dashboard (`wtm ui`), runview
```

The annotated map of every sub-package is in `docs/dev/architecture.md`.

**Hard rules** (the import and call rules are checked by `make lint`, `tools/archlint`):
- `surface/cli/` has zero business logic.
- `domain/` has types, errors and constants only — no methods, no free functions.
- `rules/` imports only stdlib and `internal/domain` — no I/O, no side effects.
- `service/` never imports `cobra`, `bubbletea`, `lipgloss`.
- `surface/cli/render/` and `surface/tui/` have zero decision logic — only rendering.
- `styles/` is the only package allowed to instantiate `lipgloss.Style`.
- `flow/` imports **only** `internal/service/`, `internal/rules/`, `internal/domain/` and the stdlib — never cobra, bubbletea, lipgloss, `surface/`, `config/` or `infra/`. Need something from `infra/`? Add a thin `service/` wrapper (e.g. `worktree.FindByBranch`).
- `service/x` imports `service/y` only along an edge declared in `tools/archlint` (`serviceEdges`).
- The daemon (`service/process`, `service/proxy`) is blind to git: no `service/worktree|branch|github|events`, no `config`, only allow-listed `infra/` functions.
- A service **mutator** (the table in `tools/archlint/chokepoint.go`) is called only from `internal/flow/`.
- Every package of `service/`, `infra/` and `store/` has a package doc stating its one responsibility (`pkgdoc`). The roots of `rules/` and `domain/` only shrink (`rootfreeze`): no new file nor exported symbol there.
- `meta.json`, the changes to `config.toml` (the remembered answers, `base_path`, the `.env` targets) and the managed `.env` files are written by `store/` only, each through its interface (`store.Meta`, `ProjectConfig`, `EnvFiles`); `config/` loads and validates, and keeps `init`'s full render. See `docs/dev/store.md`.
- **A file is named after what it declares**, and a port lives with its implementation: its interface, the types it takes and returns and the implementation share one file (`store/meta.go`), never an interfaces-only file beside the code. A type a whole package turns around has its own file; options sit beside the call that takes them.
- A file is written through `infra/fs` only: `fs.UpdateFile(ctx, {Path, Mutate})` (lock, read, `Mutate` decides an `fs.Change`, atomic rename), never `os.WriteFile`/`os.Create`/`os.Rename`/`os.Remove`/`os.RemoveAll`/a writing `os.OpenFile` outside `infra/` (`statewrite`), and `fs.UpdateFile` only from `store/`, the owner of each state file (`stateowner`). A state file carries `schema` (absent = 1, written from 2 on) and a write refuses a newer one (`*statefile.NewerError`, exit 22). See `docs/dev/state-files.md`.

**Every worktree-mutating command goes through `flow/`** — `Request`/`Outcome`/`Presenter`/`Params`/`Run` in `internal/flow/<cmd>/`, its questions as `flow.Step` in `steps.go`, a runner that is only flags → `Request` → Prompter and Presenter → `<cmd>.Run`; a flow never frames, never picks a stream, and **returns** errors. Recipe: `docs/dev/adding-a-mutation-command.md`; model: `docs/dev/flow-layer.md`.

**Output** — each command frames its human output exactly once (`render.Frame`, or `FrameStart`/`FrameEnd`) and the body writes to the writer `Frame` hands it; JSON and machine output are never framed. A block prints only if it changes what the reader does next: success contracts to a count, anomalies are named one by one. Glyph runes come from `domain` constants, the glyph carries the only colour on its line, and `Muted` is for chrome and non-event lines only. Full rules: `docs/dev/output.md`.

**Designating a worktree** — the subject is **positional**; a worktree that is not the subject is a **flag named after its role** (`extract [source] --to`, `create --from`, `run up [worktree] --profile`). Omitted positional → the current directory when that is a safe default for the command (`run`), a picker otherwise (`clean`). A resolved worktree is always the root as git spells it (`infra.Toplevel`), never `os.Getwd()`.

### Bypass flags — two orthogonal axes

Every worktree-mutating command (`create`, `clean`, `sync`, `fast-forward`, `prune`, `relocate`, `reparent`, `extract`, `checkout`, `env`), and `init`/`run init`, which wire it by hand, follow this model:

- **`--yes` / `-y` — the confirmation axis: fully unattended, zero prompts.** Each input resolves as:
  1. **Decision / confirmation** → its flag, else the **remembered answer** (`[wizard.remembered]`, ticked "Always use this answer" in the wizard), else a documented **safe default**, never destructive (`sync --yes` does not push; `extract --yes` aborts on conflict; `clean`/`prune --yes` leave orphans unless `--reparent-children`). Only a `StepSelect` opting in with `Step.Memory` remembers, never a confirmation or a destructive option (`rules.RememberableValues`); `--ask` ignores the memory for one run.
  2. **Required selection with no safe default** → its flag/arg, else an **error naming the missing flag**. Never fall back to a picker.
  3. A picker runs only in a **fully interactive** run (no `--yes`, TTY, human output).
- `--yes` is the only spelling of that axis — no `--non-interactive`. JSON mode requires `--yes`.
- **`--force` — the safety axis, strictly separate.** It only lifts safety refusals (dirty / unpushed / open PR / locked). It does **not** imply `--yes`: `--force` alone still runs the wizard and asks to confirm, with `--force` threaded in as a preset.

Implementation: `interactive := isTTY && rules.IsHumanFormat(format) && !yes`; a command expresses case 2 as a step `Resolve` that names the flag (see `internal/flow/sync/steps.go`). Decision defaults go through a pure rule where one exists (`rules.DecidePush`).

**Recap completeness:** a flag or a remembered answer never makes a recap line disappear, nor its line in the wizard trail — the recap reads the step's answer, else the flag/arg that resolved it (`Session.Presets`, or `Step.Given` for a step its `Skip` can rule out). A flag answers its step, never only preselects it. **Re-init completeness:** a re-init step shows the complete candidate list, pre-filled from the config on disk, and reads an answer that may be empty as `(value, asked)` so a step never reinstates what the user removed. Details: `docs/dev/flow-layer.md`.

## 10. Commit messages in English

Every commit message — subject and body — is in English, whatever language the conversation was held in. `CHANGELOG.md` is in English too (see Docs & README).

## 11. Validate before commit

```
make lint          # fmt + vet + arch (tools/archlint) + dead (deadcode) + sumtype (go-sumtype) + next-lint (revive on next/) + staticcheck — all gating
make next          # build bin/wtm-next, the next/ pilot
make test          # go test ./... -race -count=1
make docs          # regenerate docs/ from the Cobra tree
make demos         # re-record the README GIFs and stills (needs vhs)
make release-notes VERSION=x.y.z   # print a CHANGELOG section as release notes
make dupl          # clone report, informative only
```

- Tests of `infra/fs`, `store/`, the `statefile` packages and the `rules/` sub-packages use testify (`assert`, `require`); each test file holds only `Test…` functions, the example and fakes in `fixture_test.go`, shared helpers in `helpers_test.go`. The rest of the repository keeps stdlib `testing`.
- A new architectural rule goes into `tools/archlint`, not into a paragraph here. `.archlint-migrating` and `.deadcode-ignore` may only shrink; a new entry needs a stated reason. See `docs/dev/lint.md`.
- `.claude/hooks/pre-commit-gates.sh` runs `make lint` and a `go mod tidy` check on every `git commit` and blocks it on failure. It does not run the tests. `WTM_SKIP_GATES=1 git commit …` only when the gate itself is wrong.
- **Invoke the `build-validator` subagent before marking any task done** — it adds the `-race` test suite, dependency hygiene and the duplication report.
- **See it run before calling it done** — the `wtm-sandbox` skill drives the built binary in an isolated sandbox (tmux, VHS). **Open a PR with the `open-pr` skill**: base from `wtm tree`, that proof attached, the short body template.
- **Several issues at once** — the `orchestrate` skill makes this session the orchestrator: one wtm worktree and one Claude worker per Linear issue in herdr panes, product questions relayed to the user, PRs verified before they are reported.
