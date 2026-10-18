# The form engine: `ui/form`

> Outline, completed once the wave-2 form PR merges (the conclusion moves into `form.Command`, the widgets move to their own `ui/` package, `form.Field` the widget is renamed, a `ui/formtest` kit arrives). What follows is true of the code today; the sections marked *pending* will be rewritten.

`ui/form` renders any command's schema (`domain.Form`) as flags, a command line and a wizard, knowing no command. A command hands it its use case and reads back how the run ended; it writes a step of its own only for a question the engine cannot ask by its kind.

## What a command gives it

`form.Command[R, Q, O]` (`ui/form/command.go`), built in the command's `cli/form.go`:

- `Open(ctx, dir)`: opens the project's use case as a `form.UseCase[R, Q, O]` (its `Questions` and `Run`).
- `YesUsage`: the usage of `--yes`, saying what an unattended run requires.
- `Plan(previewed, width, styles)`: how the recap draws the plan.
- `ApplyLabel(previewed)`: the recap's option that applies, nil for "Yes, apply".
- `Special(session, at, field)`: the command's own step for a field, nil for the engine's.

## What it does with the schema

- `Bind`: registers each field's flag and usage from `Q{}.Form()` (`Form.Flags`, `--remember` when a field is rememberable), plus the run flags (`--dry-run`, `--expect-fingerprint`, `--yes`, `--output`, `--ask`).
- `Launch`: turns the command line into `R` (`domain.Fold` of `R.Answer`) and `domain.RunOptions`, opens the use case on the run's project directory, then runs the wizard when the user can see and answer it (`Streams.CanPrompt`, human output, no `--yes` nor `--expect-fingerprint`), else `UseCase.Run`; hands the `Ran{Request, Outcome}` and the error to the command's conclusion.
- `Argv(command, req)`: spells the request back as a command line from `R.Values` (the apply line, the `--dry-run` replay).
- *pending:* the conclusion (JSON envelope with `uicli.Replan`, refusal, framed dry run with its apply line, outcome, `wtm go` step) moves here from each command's `cli/command.go`.

## The wizard

- **A state machine**: the current step is data, every transition happens in `Update` on a message; going back forgets an answer without a reset per step, since the `Session` folds each step's answer into the request given, in step order.
- **No I/O in `Update`**: after each answer the questions are observed again (`UseCase.Questions`) in a `tea.Cmd`, and each step reads its label, help, options, pinned value, remembered answer, skip and problems from the form observed last. The recap previews the plan the same way (a dry run), then applies it with its fingerprint.
- **A step per field kind**: select, confirmation, text list, text, multi-select. A step settled by a flag, a remembered answer or a default shows in the trail with its origin; a skipped one shows its reason.
- **Problems** show as their templates write them, the step's label standing for `{path}`; only a template's own first word is capitalised.
- **The streams are the command's**: the wizard draws on stderr (`tea.WithInput`/`WithOutput`), cancelled through the command's context; Ctrl+C in raw mode is a key the model turns into an abort (exit 19).

## A command's own step

Only for a question the engine cannot ask by its kind (extract's file picker, its target picked as a worktree then typed as a branch). In `cli/steps.go`, returned by `Special` for the field's path, embedding the engine's `form.Asked`, `form.TextStep` or `form.MultiSelectStep` and overriding only what differs (its standing, its check, its tone). *pending:* the widget names after the split.

## Tests

- *pending:* the shared wizard driver and golden helper of `ui/formtest`. Today each `cli` package drives the wizard frame by frame in `wizard_test.go` and compares goldens in `cli/testdata/` (`-update` rewrites them), with the colour profile pinned.
