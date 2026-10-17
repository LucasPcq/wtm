# Bubble Tea architecture — community conventions

The recognized conventions for a terminal UI in Go with Bubble Tea, and when to reach for `huh` instead of a hand-written form. Sources were read on 2026-10-17; a quote is verbatim, anything else paraphrases the source cited. Read with `cli.md` for when a UI may run at all.

## Sources

| Short name | Where | Status |
| -- | -- | -- |
| tea | [Bubble Tea README](https://github.com/charmbracelet/bubbletea) and its [commands tutorial](https://github.com/charmbracelet/bubbletea/blob/main/tutorials/commands/README.md); godoc of [`options.go` at v1.3.6](https://github.com/charmbracelet/bubbletea/blob/v1.3.6/options.go) | read |
| leg100 | Louis Garman, [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/), 24 Aug 2024 | read; a personal blog, widely linked from the Bubble Tea community, not an official Charm document; its author calls it "not a tutorial but a series of tips (and tricks)" |
| teatest | [`charmbracelet/x/exp/teatest`](https://github.com/charmbracelet/x/tree/main/exp/teatest) | source read; lives in Charm's `exp` tree, so no compatibility promise (leg100 tip 9 says the same) |
| huh | [huh README](https://github.com/charmbracelet/huh) and source (`form.go`, `group.go`, `keymap.go`, `field_select.go` on `main`) | read |
| gh | [`internal/prompter/prompter.go`](https://github.com/cli/cli/blob/trunk/internal/prompter/prompter.go) | read, as a large CLI that adopted huh |

**Versions.** The `main` branches have moved to v2 under new module paths: `charm.land/bubbletea/v2`, `charm.land/huh/v2` (its `go.mod` requires `charm.land/bubbletea/v2 v2.0.8`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`). A program on `github.com/charmbracelet/bubbletea` v1 cannot embed huh v2 without moving to Bubble Tea v2 (README: "Upgrading from v1?" → `UPGRADE_GUIDE_V2.md`). `teatest` on `main` still requires `github.com/charmbracelet/bubbletea v1.3.5`. What follows holds for both majors unless it names one.

## 1. The Elm loop

- **State lives in the model; only `Update` changes it; `View` is a pure function of it.** Why: one writer, no locks, a frame reproducible from a state. Source: tea README, "The Elm Architecture" (`Update` is "called when "things happen."" and its job is to "look at what has happened and return an updated model in response"; `View`: "We look at the model in its current state and use it to build a `tea.View`" — v2 wording, v1 returns a string).
- **Never mutate the model outside `Update`, even through a pointer receiver.** Why: a goroutine writing the model races the event loop. Source: leg100 tip 4 ("Use receiver methods on your model judiciously": changing state outside the event loop creates race conditions; make changes in `Update()`).
- **Keep `Update` and `View` fast.** Why: they gate every frame and every key. Source: leg100 tip 1 ("Bubble Tea can only process messages as fast as as your `Update()` and `View()` methods.").
- **Do all I/O in a `tea.Cmd`, and come back with a message.** Why: a command runs off the loop; its result re-enters through `Update` like any event. Source: tea commands tutorial ("`Cmd`s are functions that perform some I/O and then return a `Msg`."; "Checking the time, ticking a timer, reading from the disk, and network stuff are all I/O and should be run through commands."; "Internally, `Cmd`s run asynchronously in a goroutine."; "The `Msg` they return is collected and sent to our update function for handling.").
- **`Init` returns the first I/O as a command.** Source: tea README ("`Init` can return a `Cmd` that could perform some initial I/O.").
- **Do not rely on the order of concurrent messages; use `tea.Sequence` when order matters.** Why: command results arrive in completion order; only keyboard input is ordered. Source: leg100 tip 5 ("You cannot rely on the ordering of concurrently sent messages.").
- **A panic inside a command is not recovered and leaves the terminal raw.** Why: Bubble Tea recovers panics in the event loop, not in commands. Source: leg100 tip 8 ("Bubble Tea recovers gracefully from panics occuring in the event loop but not if it occurs in a command.").

## 2. Running a program inside a CLI

- **Hand the program the command's context: `tea.WithContext(ctx)`.** Why: cancelling the operation stops the UI. Source: tea `options.go` v1.3.6 ("useful if you want to cancel the execution from outside. When a Program gets cancelled it will exit with an error ErrProgramKilled").
- **Pass the streams explicitly: `tea.WithInput`, `tea.WithOutput`.** Why: the command's streams, not the process's, so a test can drive it. Source: tea `options.go` (`WithOutput`: "by default, is stdout"). Render a prompt on stderr when stdout carries data: huh does so by default (`NewForm` sets `tea.WithOutput(os.Stderr)`; `WithOutput` godoc: "Default is STDOUT when [Form] is accessible …, STDERR otherwise").
- **If the CLI owns signals, say so: `tea.WithoutSignalHandler()`.** Source: tea `options.go` ("useful if you want to handle signals yourself"). In raw mode Ctrl+C arrives as a key, so the model must turn `ctrl+c` into a quit or an abort itself.
- **Log to a file, never to the screen the program owns.** Source: tea README, "Debugging" (`tea.LogToFile("debug.log", "debug")`, `tail -f debug.log`; delve in headless mode); leg100 tip 2 (dump every message to a file behind a `DEBUG` variable).

## 3. Composing models

- **Grow a tree of models: the root routes messages to children and composes their views.** Why: one model per concern, each testable alone. Source: leg100 tip 6 ("Any non-trivial Bubble Tea program outgrows a single model."; the root routes messages to child models and composes their views, creating children upfront or on demand).
- **Route keys to the focused child only; broadcast the rest (window size, ticks, results) to those that need them.** Source: leg100 tip 6 (routing by the root); the same split is what `huh.Form` does between its groups and the focused field (`Form.Update`).
- **Reuse `bubbles` components** (text input, list, viewport, spinner) before writing one. Source: tea README, "Libraries" ("Common Bubble Tea components such as text inputs, viewports, spinners and so on").
- **A child returns a `tea.Cmd`; the parent batches it, never runs it.** Why: running it inline would be I/O in `Update`. Source: tea commands tutorial (commands run asynchronously, results come back as messages).

## 4. Multi-step flows

- **Model a wizard as an explicit state machine: the current step is data, every transition happens in `Update` on a message.** Why: back navigation, skipping and re-entry become transitions, not control flow spread across callbacks. Source: this is the shape of `huh` (a form is a list of groups with a selector and `NextGroup`/`PrevGroup`, a hidden group is skipped by `WithHideFunc`) — a convention derived from it, not a rule any source states in these words.
- **Recompute what a step shows from the answers so far, not from a snapshot taken at start.** Source: huh README, "Dynamic Forms" ("Simply replace properties with their equivalent `Func` to recompute the properties value"; a binding re-runs the function only when it changes; "`huh` will automatically handle caching for you").
- **When recomputing needs I/O, issue a command and show a loading state until its message lands.** Why: §1, I/O never in `Update`. Source: tea commands tutorial; leg100 tip 1.

## 5. Window size and layout

- **Store `tea.WindowSizeMsg` in the model and lay out from it; never hard-code heights.** Why: terminals resize and hard-coded arithmetic breaks on the next edit. Source: leg100 tip 7 ("The code is brittle, using hard coded heights…"; measure with lipgloss `Width()`/`Height()` and track `tea.WindowSizeMsg`).
- **Pass the size down to children.** Source: leg100 tip 6/7; huh exposes `Form.WithWidth`/`WithHeight` and `Group.WithWidth`/`WithHeight`.

## 6. Testing

- **Unit-test `Update` as a function: send a message, assert the model and the returned command.** Why: deterministic, no terminal. Source: follows from §1 (Elm loop); no source above prescribes it in words.
- **End-to-end: `teatest.NewTestModel(t, m, teatest.WithInitialTermSize(w, h))`, `tm.Type(...)`/`tm.Send(msg)`, `teatest.WaitFor(t, tm.Output(), cond)`, then `tm.FinalModel(t)` or `tm.FinalOutput(t)`.** Source: teatest `teatest.go` (those functions); leg100 tip 9 ("For end to end testing of your TUI, Charm have developed teatest"; it simulates key presses, waits for output, and supports golden files).
- **Golden frames: `teatest.RequireEqualOutput(t, out)` compares to `testdata/<test>.golden`, regenerated with `-update`.** Source: teatest `teatest.go` (`RequireEqualOutput`, backed by `x/exp/golden`). Pin the colour profile in tests so goldens do not depend on the CI terminal.
- **Record demos with VHS from a committed tape.** Source: leg100 tip 10.

## 7. huh — forms instead of hand-written models

What huh gives (verified in its README and source):

- **Groups of fields, shown one group at a time** ("you can think of groups as pages"), fields `Input`, `Text`, `Select[T]`, `MultiSelect[T]`, `Confirm`, `Note`, `FilePicker`. Source: huh README, "Tutorial" and "Field Reference"; `field_*.go` (Note, FilePicker).
- **Validation per field** (`Validate(func(T) error)`): "The form will mark erroneous fields and display error messages accordingly." Source: huh README, "Tutorial"; `field_select.go`.
- **Dynamic fields:** `TitleFunc`, `DescriptionFunc`, `OptionsFunc(f, binding)`, recomputed when the binding changes, cached. Source: huh README, "Dynamic Forms"; `field_select.go`.
- **Skipping:** `Group.WithHide(bool)` / `WithHideFunc(func() bool)` ("checks if this group should be skipped"); a field may report `Skip()`. Source: huh `group.go`, `form.go` (`Field` interface).
- **Back navigation:** `shift+tab` is bound to "back" on every field's keymap; `Form.PrevGroup()`/`NextGroup()` exist. Source: huh `keymap.go`, `form.go`.
- **Accessible mode:** `form.WithAccessible(true)` renders line-by-line prompts "designed specifically for screen readers", reading from the form's input; it switches on automatically when `TERM=dumb`. Source: huh README, "Accessibility"; `form.go` (`NewForm`: `if os.Getenv("TERM") == "dumb" { … f.WithAccessible(true) }`). gh uses exactly this for its accessible prompter (`prompter.go`).
- **Themes** (Charm, Dracula, Catppuccin, Base 16, Default, or custom) and a key map (`WithKeyMap`), help (`WithShowHelp`). Source: huh README, "Themes"; `form.go`.
- **Embeddable:** "A `huh.Form` is just a `tea.Model`". Run standalone with `form.RunWithContext(ctx)`, which returns `huh.ErrUserAborted` on Ctrl+C/abort and `huh.ErrTimeout` after `WithTimeout`. Source: huh README, "What about Bubble Tea?"; `form.go`.
- **One-off prompts:** each field has `Run()`. Source: huh README; `form.go` (`Field` interface).

What huh does **not** give, or not without custom code:

- **No non-interactive mode.** huh has no "fill from flags and skip" switch: the caller decides not to run the form, or hides groups already answered. Accessible mode reads answers from an `io.Reader`, which is a line protocol, not a scripting interface. Source: huh `form.go` (no such option; `WithInput`/`WithOutput` only redirect).
- **No trail of answers settled elsewhere.** A hidden group shows nothing; showing "Source branch · main · --from" for a flag-settled step needs a `Note` or a custom view.
- **Dynamic functions run synchronously** inside the form's update/view cycle. Recomputing from git (re-reading facts after an answer) either blocks the loop or needs the form embedded in a parent model that fetches in a command and rebuilds the fields.
- **Fields are the stock set.** Anything else (an editable list where `tab` adds an entry with a per-entry badge, a select row with an "always use this answer" toggle) is a custom type implementing huh's exported `Field` interface (Bubble Tea model + `Focus`/`Blur`/`Error`/`Skip`/`Run`/`RunAccessible`/…), which then also owes its accessible rendering.
- **Back navigation moves focus, it does not undo.** Resetting later answers to their initial values when the user steps back is the caller's logic.
- **A version line:** huh v2 needs Bubble Tea v2 (see Versions).

**When to use huh:** a fixed or mildly dynamic questionnaire whose fields are stock types, where accessibility and theming for free outweigh a custom look. **When to hand-roll:** the flow is driven by facts that change with each answer and need I/O to recompute, steps are settled by several sources (flag, remembered answer, default) that must stay visible, or the last step is a preview of an effect rather than a question. A middle path, used by gh: hide prompting behind a small interface (`Prompter`), implement it with huh, keep the flow logic in the command.

## 8. How `ui/form` applies this

`next/`'s wizard is hand-rolled, for the reasons of §7 (facts read again after each answer, steps settled by a flag, a remembered answer or a default shown in the trail, a recap that previews the plan). It renders any command's schema:

- **`form.Command[R, Q, O]`** is what a command's `cli/` hands its use case to: `Open`, a closure that opens the project's use case as a `form.UseCase[R, Q, O]` (its `Questions` and `Run`), and how the recap draws its plan. From the schema (`Q{}.Form()`) it registers the flags by field kind, builds `R` from the flags and args (`domain.Fold` of `R.Answer`), launches (the wizard on a terminal, else `Run`) and spells the command line (`Argv`, from `R.Values`).
- **The `Wizard`** is the state machine of §4: the current step is data, every transition happens in `Update`, the repository is read off the loop after each answer (`Script.Observe` in a `tea.Cmd`), and the recap previews the plan the same way.
- **The `Session`** holds what the wizard asked: the request is the one given with each step's answer folded in step order, so going back forgets an answer without a reset per step.
- **A step per field kind** (select, confirmation, text list, text, multi-select) reads its label, help, options, pinned, remembered, skip and problems from the form observed last; a command gives its own step only where its question is its own (`Special`: extract's file picker, its target picked as a worktree then typed as a branch), embedding `form.Asked`, `form.TextStep` or `form.MultiSelectStep` and overriding only what differs (its standing, its check, its tone).
- **Problems** are shown as their templates write them, the step's label standing for `{path}`; only a template's own first word is capitalised, never a value it starts with.

