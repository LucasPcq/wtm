# Adding or migrating a command

The checklist for a command of `next/`, new or migrated from `internal/`. `create` and `extract` are the references: open them beside this page and copy their shape.

## 1. The spec from the legacy

Read the legacy flow (`internal/flow/<cmd>/`), its `--help` (`docs/wtm_<cmd>.md`), and run it in a sandbox (`wtm-sandbox` skill). Write down its behaviour and every bug found; each bug gets a test that fails on the old behaviour. List what `next/` lacks for it (an adapter, a domain fact, a hook kind): each is part of the change or an owner decision, never dropped silently.

## 2. The files

```
command/<name>/
  usecase.go       UseCase, the outcome aliases, Run, Plan, Apply, Questions, Observe, the steps of Apply, helpers
  port.go          its ports (ports-adapters.md)
  rules.go         pure: Facts, the decisions, Decide, Plan
  questions.go     pure: Request, the form schema, NewQuestions, Resolve (questions.md)
  cli/             the command line over form.Command
  *_test.go        beside each file; fixture_test.go, helpers_test.go, contract_test.go
```

A command named with a dash lives without it (`fast-forward` → `command/fastforward`). It never imports another command: what it shares with an existing one moves to `domain/`, `service/` or `ui/` in the same change. A shared piece speaks the domain, and each command turns it into its own codes and messages: `worktree.Place` returns a sealed `worktree.Placement`, which create and extract each word as their own refusals; the fast-forward outcomes of `provision.Service.FastForwardEach` become each command's warnings the same way.

## 3. `usecase.go`

Sections in this order, each under go-style's separator: the use case, Run, Plan, Apply, Questions, Observe, the steps of Apply, helpers.

- **The use case:** `const Name = "<name>"`; `CodeConfigNotFound = Name + repository.ConfigNotFound` and `CodeConfigInvalid = Name + repository.ConfigInvalid`; `UseCase` with its ports and the concrete services it calls (`repository.Service`, `operation.Service`, and `provision.Service` when it creates a worktree); `Outcome = domain.Outcome[Plan, Detail]`, `Result = domain.Result[Detail]`, `Item = domain.Unit[Detail]`; its `Detail`, what the JSON envelope reports per item.
- **Run:** copy `create.UseCase.Run` and keep its numbered steps: `Operations.Begin(Name, req.Locks(), opts)` with `defer op.End()`; `operation.PlanUnderLocks` around `Plan`; `operation.Fingerprint(plan.Fingerprinted())`, returning on a dry run; `op.Expect`; `op.Started()`, `Apply`, `op.Finished(result.Failure())`. A dry run takes no lock and plans once. Add a numbered step only where the story differs, as extract's step 5 ends a run with nothing to extract. A step comment says why the step is there, not what the call does.
- **Plan:** `Observe` → `Resolve` → `Decide`, a paragraph each.
- **Apply:** a paragraph per step; a step longer than a screen becomes a function in "the steps of Apply". One item's failure stops none of the others; a cancelled context skips the items not started.
- **Questions:** `Observe`, then `NewQuestions(facts, req)`.
- **Observe:** `repository.Service.Observe` (config, run.toml, snapshot; no wtm config is `<name>.config_not_found`, an invalid one `<name>.config_invalid`), then what the command reads through its own ports.

## 4. `rules.go`

Pure: no port, no context. `Facts` (what `Observe` read); one decision per subject as a sealed `decision` declared `//go-sumtype:decl` (`command/create/rules.go:39`), the subject's plan or its refusal; `Decide(facts, req, …)`, with whatever else the command decides from (extract's takes the conflicts found), returning the `Plan` and its warnings (and an error when a refusal stops the whole request); `Plan.Locks()` (what the operation must hold) and `Plan.Fingerprinted()` (what the fingerprint covers); `Request.Locks()` (taken before planning).

## 5. Codes

`<name>.<snake_case>`, `<name>` being the command's `Name` constant, declared beside the code that raises them with their message or template, as a constant when another package reads it (`extract.CodeConflict`). archlint's `codeprefix` checks every value given to a `Code:` field or a `Code…`/`code…` constant in the command and its `cli/`; a code several commands share is a `domain` constant (`domain.CodePlanStale`). A refusal is a `*domain.Error{Kind, Code, Message, Params}`, a request problem a `domain.FieldError` on its path, a warning a `domain.Warning{Code, Message, Params}` (fast-forward warnings stay per command). The kind gives the exit code; the command maps its own codes in `cli/` (`contract.md`, "Exit codes").

## 6. `cli/`

Copy `command/create/cli/`; the engine behind it is `form.md`.

- `command.go`: `New(streams, env, open) *cobra.Command` with `Use` from `Name`, `Short`, `Long`, `Example` and `yesUsage`; its `uicli.Run{Command: <name>.Name, Streams, Env, Flags, Exits}`, `Exits` mapping the command's own codes to their exit (`<name>.config_not_found`, `<name>.config_invalid`, extract's conflict); `c.form.Bind(cmd, &c.io)`. Its run is `c.form.Launch(cmd, args, c.form.Conclude)`, or a `conclude` of its own that handles its one end case and hands the rest to `Conclude` (extract's "nothing to extract").
- `form.go`: `newForm(open)` returns its `form.Command[Request, Questions, Outcome]` with `YesUsage`, `Open: form.Opener[…](open)`, `Preview`, `Draw`, `ApplyLabel` and `Special` when it has steps of its own; `<name>Flags = newForm(nil).Flags()` spells the failures.
- `output.go`: `planRowsOf`, `preview` and `previewMarks` (the plan as `form.Preview`), `drawOutcome` (the apply, with the `ui/output` kit; `output.GoStep(branch)` for the step that enters a worktree, the branch quoted).
- `steps.go`: only for a field the engine cannot ask by its kind; `special` picks the field by `field.Spec().Path`, and within one path a step may still vary by the field's kind (extract's `toStep`, `command/extract/cli/steps.go`).
- `testdata/`: goldens for `--help`, the dry run, each outcome, each refusal, each JSON envelope, the wizard's screens, through `ui/formtest`.

No conclusion, envelope or dry-run drawing in a command's `cli/`: they are `form.Command`'s. Positional arguments are checked by cobra's `Args` (`cobra.ArbitraryArgs` for create's branches, `cobra.MaximumNArgs(1)` for extract's source). Tests cover both modes, a terminal (the wizard) and none (unattended, a missing field refused by its flag). A secret is never read from a flag.

## 7. The registrations in `cmd/wtm-next`

| File | Line |
| -- | -- |
| `main.go` | `<name>cli.New(streams, env, open<Name>)` in `Commands` |
| `project.go` | `open<Name> = opener(func(a app) <name>.UseCase { return a.<Name> })` |
| `catalogue.go` | `api.Bind(<name>.Name, <name>.Questions{}.Form(), open<Name>, <name>.UseCase.Run)` |
| `wire.go` | its field in `app`, its use case built from the adapters `wire` builds once |

`registration_test.go` fails when a command of `main.go` is missing from the catalogue or from the wiring.

## 8. Tests

| File | Checks |
| -- | -- |
| `rules_test.go`, `questions_test.go` | the pure decisions and the form, table-driven from facts in `fixture_test.go`; the round trip of `Form`, `Answer`, `Values` |
| `usecase_test.go` | `Run` over the fakes (`gittest`, `storetest`): what changed, the events, the locks, cancellation |
| `contract_test.go` | `operationtest.Contract` over `Run`, with a request that applies, a `Stale` and a `Fail` |
| `cli/*_test.go` | the goldens, the wizard screen by screen, the exit codes, the envelope, through `ui/formtest` (`form.md`, "Tests") |
| `cmd/wtm-next` | one end-to-end test wiring the real adapters |

A test file holds only `Test…` functions, its fakes in `fixture_test.go` and helpers in `helpers_test.go` (archlint `testlayout`); a subtest is named like an identifier, `stale_fingerprint_refused`, never a sentence (archlint `subtestname`).

Gates: `make lint-next`, `make test-next`, then the `build-validator` agent.

## 9. Proofs and the legacy gaps

- `next/docs/proof/<name>.sh` runs the same scenarios against `wtm` and `wtm-next` in sandboxes and captures each exit code, stdout, stderr and the state on disk; the captures are committed, and a rerun leaves `git diff next/docs/proof` empty (`next/docs/proof/README.md`).
- The PR shows the real output before (`wtm`) and after (`wtm-next`) and the state on disk compared.
- Each difference is settled, never a "known difference": the legacy when it is sensible; otherwise the uniform form of `next/`, listed in the PR and in `next/docs/proof/README.md` for the owner; one word keeps one meaning across commands (` · remembered` is recalled, ` · will be remembered` will be written); nothing the legacy offers disappears without the owner's decision; nothing odd is added only to match the legacy. A spec that departs from the legacy is validated by the owner before it is implemented. Each contract break (JSON, exit code, state on disk) is listed.

## 10. Done

The spec is written and its bugs listed; each bug is fixed with a failing-first test; the command speaks the uniform system of `next/` (output kit, envelope, texts, contract); a fresh-context review and a ponytail pass have run and their findings are fixed; the PR shows the real output before and after.
