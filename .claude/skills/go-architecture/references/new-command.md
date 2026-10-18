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

A command named with a dash lives without it (`fast-forward` → `command/fastforward`). It never imports another command: what it shares with an existing one moves to `domain/`, `service/` or `ui/` in the same change.

## 3. `usecase.go`

Sections in this order, each under go-style's separator: the use case, Run, Plan, Apply, Questions, Observe, the steps of Apply, helpers.

- **The use case:** `const Name = "<name>"`; `CodeConfigNotFound = Name + repository.ConfigNotFound`; `UseCase` with its ports and the concrete services it calls (`repository.Service`, `operation.Service`, and `provision.Service` when it creates a worktree); `Outcome = domain.Outcome[Plan, Detail]`, `Result = domain.Result[Detail]`, `Item = domain.Unit[Detail]`; its `Detail`, what the JSON envelope reports per item.
- **Run:** copy `create.UseCase.Run` and keep its numbered steps: `Operations.Begin(Name, req.Locks(), opts)` with `defer op.End()`; `operation.PlanUnderLocks` around `Plan`; `operation.Fingerprint(plan.Fingerprinted())`, returning on a dry run; `op.Expect`; `op.Started()`, `Apply`, `op.Finished(result.Failure())`. A dry run takes no lock and plans once. Add a numbered step only where the story differs, as extract's step 5 ends a run with nothing to extract. A step comment says why the step is there, not what the call does.
- **Plan:** `Observe` → `Resolve` → `Decide`, a paragraph each.
- **Apply:** a paragraph per step; a step longer than a screen becomes a function in "the steps of Apply". One item's failure stops none of the others; a cancelled context skips the items not started.
- **Questions:** `Observe`, then `NewQuestions(facts, req)`.
- **Observe:** `repository.Service.Observe` (config, run.toml, snapshot; no wtm config is `<name>.config_not_found`), then what the command reads through its own ports.

## 4. `rules.go`

Pure: no port, no context. `Facts` (what `Observe` read); one decision per subject as a sealed `decision` declared `//go-sumtype:decl` (`command/create/rules.go:39`), the subject's plan or its refusal; `Decide(facts, req)` returning the `Plan` and its warnings (and an error when a refusal stops the whole request); `Plan.Locks()` (what the operation must hold) and `Plan.Fingerprinted()` (what the fingerprint covers); `Request.Locks()` (taken before planning).

## 5. Codes

`<name>.<snake_case>`, `<name>` being the command's `Name` constant, declared beside the code that raises them with their message or template, as a constant when another package reads it (`extract.CodeConflict`). archlint's `codeprefix` checks every value given to a `Code:` field or a `Code…`/`code…` constant in the command and its `cli/`; a code several commands share is a `domain` constant (`domain.CodePlanStale`). A refusal is a `*domain.Error{Kind, Code, Message, Params}`, a request problem a `domain.FieldError` on its path, a warning a `domain.Warning{Code, Message, Params}` (fast-forward warnings stay per command). The kind gives the exit code; the command maps its own codes in `cli/` (`contract.md`, "Exit codes").

## 6. `cli/`

> Outline until the conclusion moves into `form.Command` (wave 2); until then copy `command/create/cli/`.

- `command.go`: `New(streams, env, open) *cobra.Command` with `Use` from `Name`, `Short`, `Long`, `Example`, its `uicli.Run` (`Exits`); the run hands the command line to `form.Command.Launch`; the conclusion draws the envelope, a refusal, the dry run or the outcome (moving into `form.Command`).
- `form.go`: its `form.Command[Request, Questions, Outcome]` (`form.md`).
- `output.go`: the plan rows and the outcome, with the `ui/output` kit.
- `steps.go`: only for a field the engine cannot ask by its kind.
- `testdata/`: goldens for `--help`, the dry run, each outcome, each refusal, each JSON envelope, the wizard's frames.

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
| `cli/*_test.go` | the goldens, the wizard frame by frame, the exit codes |
| `cmd/wtm-next` | one end-to-end test wiring the real adapters |

Gates: `make lint-next`, `make test-next`, then the `build-validator` agent.

## 9. Proofs and the legacy gaps

- `next/docs/proof/<name>.sh` runs the same scenarios against `wtm` and `wtm-next` in sandboxes and captures each exit code, stdout, stderr and the state on disk; the captures are committed, and a rerun leaves `git diff next/docs/proof` empty (`next/docs/proof/README.md`).
- The PR shows the real output before (`wtm`) and after (`wtm-next`) and the state on disk compared.
- Each difference is settled, never a "known difference": the legacy when it is sensible; otherwise the uniform form of `next/`, listed in the PR and in `next/docs/proof/README.md` for the owner; one word keeps one meaning across commands (` · remembered` is recalled, ` · will be remembered` will be written); nothing the legacy offers disappears without the owner's decision. Each contract break (JSON, exit code, state on disk) is listed.

## 10. Done

The spec is written and its bugs listed; each bug is fixed with a failing-first test; the command speaks the uniform system of `next/` (output kit, envelope, texts, contract); a fresh-context review and a ponytail pass have run and their findings are fixed; the PR shows the real output before and after.
