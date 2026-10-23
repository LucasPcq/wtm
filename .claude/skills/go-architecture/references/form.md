# The form engine: `ui/form`, `ui/widget`, `ui/formtest`

`ui/form` renders any command's schema (`domain.Form`) as flags, a command line, a wizard and the conclusion of the run, knowing no command. A command hands it its use case and how to show a plan and an apply; it writes a step of its own only for a question the engine cannot ask by its kind. `ui/widget` holds the inputs a step asks with; `ui/formtest` the kit every `cli` test shares.

## What a command gives it

`form.Command[R, Q, O]` (`ui/form/command.go`), built by `newForm` in the command's `cli/form.go` (`command/create/cli/form.go`):

| Field | What it is |
| -- | -- |
| `YesUsage` | the usage of `--yes`, saying what an unattended run requires |
| `Open` | `form.Opener[R, Q, O](open)`: the project's use case as a `form.UseCase[R, Q, O]` (its `Questions` and `Run`) |
| `Preview(previewed) form.Preview` | `form.PreviewOf` for a plan embedding `domain.Recapped`; the plan not yet applied: `Rows` (`output.PlanRow`, each answer with its mark), `Warnings`, `Recalled` (the fields answered from memory, which the recap's hint names). The recap and the dry run both draw it. |
| `Draw(w, applied, view)` | the outcome of an apply, drawn from `output.View`: paths shown from `view.Cwd`, follow-ups spelled by `view.CommandLine` (`output.FollowUpStep`); `form.Query.Draw(w, read, req, view)` takes the same view and the request read, which a layout may follow (`status --all` draws its table however many worktrees) |
| `ApplyLabel(previewed)` | the recap's option that applies, nil for "Yes, apply" |
| `Special(session, at, field)` | the command's own step for a field, nil for the step of its kind |

`O` satisfies `form.Outcome` (`PlanFingerprint`, `Applied`, `Failure`, `Envelope`), which `domain.Outcome` does: a command's `Outcome` alias needs nothing more.

## What it does with the schema

- `Bind(cmd, &run)`: registers each field's flag and usage from `Q{}.Form()` by its kind, plus `--remember` when a field is rememberable, `--ask`, `--yes`, `--output`, `--dry-run`, `--expect-fingerprint` (`uicli.FlagDryRun`, `uicli.FlagExpectFingerprint`).
- `Launch(cmd, args, conclude)`: refuses an invalid `--output` or correlation ID, turns the command line into `R` and `domain.RunOptions`, opens the use case on the project's directory, runs the wizard when the user can see and answer it (`Streams.CanPrompt`, human output, no `--yes` nor `--expect-fingerprint`), else `UseCase.Run` with its progress; hands the `form.Ran{Request, Outcome}` and the error to `conclude`.
- `Conclude(cmd, ran, err)`, in order: the JSON envelope (`Outcome.Envelope` with the request's answers, which a stale plan is followed up with as its replan); a refusal; the framed dry run from `Preview` with the line that applies it (`--expect-fingerprint <hash>`); the outcome from `Draw`; then the failure the apply ended on, which sets the exit code.
- `Argv(command, req)`: spells the request back as a command line from `R.Values` (the apply line, the `--dry-run` replay).

A cli's run is `c.form.Launch(cmd, args, c.form.Conclude)`. An end case of its own wraps `Conclude` and hands it everything else: extract's "nothing to extract" (`command/extract/cli/command.go`, `conclude`).

## The wizard

- **A state machine**: the current step is data, every transition happens in `Update` on a message; going back forgets an answer without a reset per step, since the `Session` folds each step's answer into the request given, in step order.
- **No I/O in `Update`**: after each answer the questions are observed again (`UseCase.Questions`) in a `tea.Cmd`, and each step reads its label, help, options, pinned value, remembered answer, skip and problems from the form observed last. The recap previews the plan the same way (a dry run), then applies it with its fingerprint.
- **A step per field kind** (`ui/form/fieldstep.go`): select, confirmation, text list, text, multi-select. A step settled by a flag, a remembered answer or a default shows in the trail with its origin; a skipped one shows its reason.
- **Problems** show as their templates write them, the step's label standing for `{path}`; only a template's own first word is capitalised.
- **The streams are the command's**: the wizard draws on stderr, cancelled through the command's context; Ctrl+C in raw mode is a key the model turns into an abort (exit 19).

## A command's own step

Only for a question the engine cannot ask by its kind (extract's file picker, its target picked as a worktree then typed as a branch). In `cli/steps.go`, `special` picks the field by its path (`switch field.Spec().Path`); within one path a step may still vary by the field's kind (extract's `toStep`, `command/extract/cli/steps.go`). The step embeds the engine's `form.Asked`, `*form.TextStep` or `*form.MultiSelectStep` and overrides only what differs (its `Standing`, its check, its tone). A `form.Step` builds its input in `Open` and returns it from `Widget()`: a `widget.Widget` from `ui/widget` (`SelectList`, `MultiSelect` with `Pick` and `Tone`, `Text`, `TextList`; `Option`, `Verdict`, `Size`, `Help`, `Styles`, `Wrap`). A new input kind is a widget there, never in a command.

A step is Bubble Tea code: it lays out from the size `tea.WindowSizeMsg` gave, never a hard-coded height; it returns its `tea.Cmd` and the wizard batches it, never runs it in `Update`; and a panic inside a `tea.Cmd` is not recovered (Bubble Tea recovers only the event loop) and leaves the terminal raw.

## Tests

- **`ui/formtest`**, imported from `_test.go` files only:
  - `Driver`: `formtest.Start(w, tea.WindowSizeMsg{…})` plays a wizard in the test's goroutine; `Keys`, `Send`, `Run`, `Screen`; `Key` and `Plain` for one key and an unstyled screen.
  - `Golden(t, name, got)` compares with `testdata/<name>.golden`; it registers `-update`, so a package that imports it declares no `-update` of its own.
  - `Envelope[P]` and `Decoded[P](t, stdout)` read the v2 JSON envelope, `P` being the command's plan.
- **A command's `cli` tests** embed `*formtest.Driver` in their own driver (`command/create/cli/helpers_test.go`) and keep goldens in `cli/testdata/`: `--help`, the dry run, each outcome, each refusal, each envelope, the wizard's screens.
- **`ui/form`'s own tests** are in `package form_test`; `export_test.go` holds exports only.
