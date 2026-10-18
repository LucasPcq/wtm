# A command's slice

How one command of `next/` is laid out, how its files read, and how it shares code with the other commands. `create` and `extract` are the two references: open them beside this page.

## Contents

- The folder
- `usecase.go`: the story first
- `rules.go`, `questions.go`, `port.go`
- `cli/`: the driving adapter
- Sharing between commands
- A second step
- Codes, messages and exit codes
- Testing a slice
- Adding a command

## The folder

```
command/<name>/
  usecase.go      the use case: what it holds, Run (the story), Plan, Apply, Questions, Observe, the steps, helpers
  port.go         the driven ports it calls (interfaces), nothing else
  rules.go        pure: the facts it decides from, Decide → Plan, the plan's locks and fingerprint
  questions.go    pure: the request, its form model, its defaults and its validation
  cli/            its driving adapter: cobra command over form.Command, its own wizard steps, output
```

Layers are files, not folders: in Go a folder is a package, so a folder per layer would push every type across a package boundary and give every command a package named `usecase` to alias. The one sub-folder is `cli/`, so cobra and Bubble Tea stay out of the use case's imports.

## `usecase.go`: the story first

A reader opens `usecase.go` and reads the whole request from the top. Order its sections, each under a separator (see go-style `comments.md`):

1. **The use case**: the `UseCase` struct (its ports, the services it calls, its paths), `Outcome`.
2. **Run**: the story, its steps numbered.
3. **Plan**: observe → resolve → decide.
4. **Apply**: the steps that change the world.
5. **Questions**: what the wizard asks.
6. **Observe**: reading the facts through the ports.
7. **The steps of Apply**, in the order Apply calls them.
8. **Helpers**.

`Run` has the same shape in every command; a reader who knows one knows all:

```go
// Run carries out the request under its operation: it takes the locks, plans
// under them, fingerprints the plan, stops on a dry run, refuses a stale plan
// and applies it between the operation's started and finished events.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Take the operation's locks before reading anything, so the facts cannot change under the plan.
	op, err := u.Operations.Begin(name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not is planned again once that lock is held.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be applied; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other facts is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}
```

A command adds a step only where its story differs (extract stops early when there is nothing to extract). `Plan` and `Apply` are short stories of their own, one paragraph per step (go-style `readability.md`); a step that grows past a screen becomes its own function in section 7.

The numbered step comments are the one place a body carries a comment per paragraph, and each must earn it the way go-style `comments.md` asks: say what the step is for or what is not obvious about it (why the locks come before observing, why the plan runs again, why a dry run stops after the fingerprint), never restate the call below it. `UseCase`, `Run`, `Plan`, `Apply`, `Questions`, `Observe`, `Request`, `Outcome` and every other exported name carry a doc comment that states their contract.

## `rules.go`, `questions.go`, `port.go`

- `rules.go` is pure (no port, no context): `Facts` and the queries over them, `Decide(facts, req, …) (Plan, []domain.Warning, error)`, the `Plan` type with `Locks()` and `Fingerprinted()`. A refusal decided here carries the command's code.
- `questions.go` is pure: `Request`, the form model (`Questions`, built from the domain question vocabulary `domain.Choice`, `domain.Confirm`, `domain.Answer`), `NewQuestions(facts, req)` and `Resolve(facts, req)` (defaults, then validation into field errors), and the schema (below).
- The schema is the request as data a surface renders without knowing the command: `Questions.Form() domain.Form` lists its fields in the order the wizard asks them (a step per field, so "Step n/m" counts the fields plus the recap) with the request's problems (`Form.Problems`, each naming a field, an entry of one or `remember[i]`; `domain.ProblemsOf` picks a field's), `Request.Answer(path, values) (Request, error)` writes a field's answer back, leaving the request as it was when the value is refused (`"remember"` answers the fields to remember, `"ask"` the run's `--ask`), and `Request.Values(path) []string` reads it back spelled the same way, nil when the request leaves the field to its default (the generic code cannot read a struct's fields). A `domain.Field` is a sealed interface, one type per kind — `SelectField`, `MultiSelectField`, `TextField`, `TextListField`, `ConfirmField` — each carrying only what is valid for it; every kind embeds a `FieldSpec`: path, flag, label, `Help` (one text: the wizard's description and the flag's usage), origin, skip. Options are `domain.Option` with their label, badge and `Short` (how the trail names it, when not by its value); a confirmation's two options are valued `true` and `false`, as `Answer` takes them (`NoFirst` lists no first); a select may be `Refreshable` (r reads its options again), a multi-select says its `Empty` problem. The form is computable from a zero `Questions`, since the flags and `--help` exist before any repository read: the `ui/form` engine registers the flags and usages from it (`Form.Flags`, `--remember` included when a field is rememberable), spells the command line from it and `Request.Values`, and builds a step per field, which reads its help, options, skip and problems from the form of the questions observed last. Labels, help, flag names, option labels and a rule the wizard shows (extract's `Questions.NewTarget`) live in `questions.go`; a field two steps answer (extract's target, picked as a worktree then typed as a branch) appears once per step. Round-trip tests per command check that folding the answers of `Form()` (`domain.Fold(Request{}, form.Answers())`, the fold the engine uses too) gives back the request, and that `Answer` takes back what `Values` spells.
- `port.go` declares only interfaces, one per outside thing the use case reaches (git, config, a store), each with exactly the methods this command calls (go-style `interfaces.md`: consumer-side, small). Services are not ports: the use case holds `operation.Service` and `provision.Service` as concrete fields, and calls `repository.Observe` as a free function.

## `cli/`: the driving adapter

`cli/` hands its use case, `Request`, `Questions` and `Outcome` to the generic `form.Command` of `ui/form`, which registers the flags from the schema, turns the command line into a `Request` and `domain.RunOptions`, runs the wizard on a terminal or `UseCase.Run` otherwise, then hands the run to the command's conclusion, which draws the outcome with the `ui/` kit. Files: `command.go` (the cobra command, its run and its conclusion, which takes the form's `Ran` and spells the apply and replan lines with `form.Command.Argv`), `form.go` (its `form.Command`: `Open`, the use case as a `form.UseCase`, how the recap draws its plan), `output.go` (its plan rows and items; the envelope is the use case's `Outcome.Envelope`), and `steps.go` only for the steps it asks its own way (`form.Command.Special`: extract's file picker and its target/branch pair, embedding the engine's `form.Asked`, `TextStep` or `MultiSelectStep`; create has none). The `Outcome` gives the engine `PlanFingerprint()`, the fingerprint a preview's apply is held to. CLI conventions: `cli.md`; TUI conventions: `tui.md`.

## Sharing between commands

A command never imports another command (archlint refuses it). Two commands share code in five places only, chosen by what the code is:

| What both need | Where | Example |
| -- | -- | -- |
| a pure rule or a vocabulary | `domain/…` | `worktree.Place` (where a branch lands), `worktree.StackOf` (the stack of worktrees, from the live ones and their meta records), `worktree.Repository` (the facts every command decides from: main path, config, run.toml, snapshot), `worktree.SelectChanges`, `domain.WorktreeLocks`, `domain.IsolationAnswer`, `domain.Choice` |
| a reading of the repository | a port method returning a domain value, or a service reading through its ports | `worktree.Snapshot` from the git port, `repository.Observe` (config, snapshot, run.toml) |
| an orchestration with I/O | `service/<name>` (concrete, its own ports) | `provision.Service.Run`, `provision.Service.FastForwardEach` (typed outcomes each command words as its warnings), `operation.Service` |
| presentation | `ui/…` | `uicli.Run` (a command's run plumbing: streams, output format, quiet, failures, envelope), the wizard engine, `output.DryRun`, `output.FollowUpSteps` |
| questions two commands ask | the service that consumes their answers, as a schema fragment each command embeds | `provision.Questions` (env strategy, isolation, source update): create and extract embed its `Given` in their `Request` and its `Fields` in their `Form`, and `provision.Worktree` takes only its `Answers` |
| the command line of another command | injected by `cmd/wtm-next`, the only package that knows every command | `provision.Service.LeftBehind` (the `wtm clean <branch> --force` a failed undo suggests) |

Questions two commands ask are one fragment, owned by the service that consumes their answers, never rewritten by a command: the command embeds the fragment's request (`provision.Given`, its JSON fields flat in the command's request; the command's `Answer` and `Values` hand it every path they do not know), its questions (`provision.Questions`, built from the facts by `provision.NewQuestions`) and its fields, spliced into its `Form` where the wizard asks them (`Questions.Fields`, with the command's own wording where its subjects differ: `provision.Wording`). The service enforces it by its input type: `provision.Worktree` takes `provision.Answers`, whose fields are unexported and which only `Questions.Answers()` returns, so a command cannot provision a worktree without asking these questions, and a question added to the fragment reaches every command that does — its flag, its help, its step, its remembered answer. A fragment's questions that do not apply (extract's target exists) are skipped with their reason, never left out of the form.

Extract on the second use, not before; until then a slice keeps its own code. A shared piece speaks the domain's language; each command maps it onto its own codes and messages (`worktree.Place` returns a sealed `Placement`, create and extract each turn it into their refusals).

## A second step

A command that offers a step after its outcome (sync's "push now?" once the result is read, clean's retry with sudo) gives it a second exported entry in its use case: its own request, operation, fingerprint and CLI spelling, run like the first. The first entry never runs it: it returns a `domain.FollowUp`, the argv of the line that triggers the second step, with its code, params and message. The CLI prints the argv word by word quoted for the shell (`output.FollowUpSteps`), the JSON envelope carries it as `error.follow_up{argv, code, message, params}`, and a caller runs it if it wants. A follow-up rides on a failure (`error.follow_up`) or in the envelope's top-level `follow_ups`, the same `{argv, code, message, params}`: a stale plan's `--dry-run` replay goes there (`uicli.Replan`), and so will the follow-ups of a success. A follow-up that names another command gets its argv from `cmd/wtm-next`, so no service or `ui/` package knows that command.

## Codes, messages and exit codes

- A command owns its codes, `<command>.<name>` (`create.path_exists`, `extract.conflict`), declared beside the code that raises them with their message.
- A refusal is a `*domain.Error{Kind, Code, Message, Params}`; a request problem is a field error naming its path, which the CLI spells as its flag.
- Exit codes come from the kind (`ui/cli/exit.go`); a command overrides one for its own code in its `cli/` (`uicli.Failed.Exits`: `create.config_not_found` → 12, `extract.conflict` → 15).

## Testing a slice

- `rules_test.go`, `questions_test.go`: pure, table-driven, from facts built in `fixture_test.go`.
- `usecase_test.go`: the use case over the stateful fakes of the adapters (`adapter/gittest`, `adapter/storetest`), checking what it changed, the events, the locks.
- `contract_test.go`: `operationtest.Contract` over `Run`, with a request that applies a change, so its events are there to check: a dry run takes no lock and publishes nothing, a stale plan is refused before any event, the events arrive in order and each unit started finishes.
- `cli/`: golden files in `cli/testdata/` for help, dry run, outcomes, refusals, the JSON envelope; the wizard through its frames.
- The adapters are tested against the real thing (real git in a temp repo); one end-to-end test per command in `cmd/wtm-next` wires the real adapters.
- Behaviour against the legacy `wtm`: the scripts of `next/docs/proof/` (`create.sh`, `tty.sh`, `extract.sh`).

## Adding a command

1. `command/<name>/port.go`: the ports it needs, method by method.
2. `rules.go` and `questions.go`: facts, plan, request, form, validation — pure, tested first.
3. `usecase.go`: `UseCase`, `Run` (the canonical shape above), `Plan`, `Apply`, `Questions`, `Observe`, `Outcome.Envelope`.
4. `cli/`: the command over `form.Command`, its output, the steps it asks its own way if any; a refusal goes through `uicli.Run.Refuse` with the request's argv, which adds the `--dry-run` replay to a stale plan (a next step, or `follow_ups` in JSON).
5. `cmd/wtm-next`: its adapters wired in `wire.go`, its cobra command registered in `main.go`, its binding in `catalogue.go`.
6. Anything it shares with an existing command moves to `domain/`, `service/` or `ui/` in the same change.
