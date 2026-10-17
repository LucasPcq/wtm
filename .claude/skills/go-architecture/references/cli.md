# Go CLI architecture and UX — community conventions

The recognized conventions for a Go command line, each as a rule, its reason, and where it comes from. Sources were read on 2026-10-17; a quote is verbatim from the page cited, anything else is a paraphrase of it. Read with `tui.md` for the interactive half.

## Sources

| Short name | Where | Status |
| -- | -- | -- |
| clig | [Command Line Interface Guidelines](https://clig.dev/) | read; the UX reference most Go CLIs cite |
| gh | [cli/cli](https://github.com/cli/cli) at `trunk`: [`docs/project-layout.md`](https://github.com/cli/cli/blob/trunk/docs/project-layout.md), [`pkg/cmd/pr/close/close.go`](https://github.com/cli/cli/blob/trunk/pkg/cmd/pr/close/close.go), [`pkg/cmd/pr/merge/merge_test.go`](https://github.com/cli/cli/blob/trunk/pkg/cmd/pr/merge/merge_test.go), [`pkg/cmdutil/factory.go`](https://github.com/cli/cli/blob/trunk/pkg/cmdutil/factory.go), [`pkg/cmd/factory/default.go`](https://github.com/cli/cli/blob/trunk/pkg/cmd/factory/default.go), [`pkg/iostreams/iostreams.go`](https://github.com/cli/cli/blob/trunk/pkg/iostreams/iostreams.go), [`internal/prompter/prompter.go`](https://github.com/cli/cli/blob/trunk/internal/prompter/prompter.go), [`internal/ghcmd/cmd.go`](https://github.com/cli/cli/blob/trunk/internal/ghcmd/cmd.go), [`pkg/cmd/root/root.go`](https://github.com/cli/cli/blob/trunk/pkg/cmd/root/root.go), [`pkg/cmdutil/errors.go`](https://github.com/cli/cli/blob/trunk/pkg/cmdutil/errors.go), [`pkg/cmdutil/json_flags.go`](https://github.com/cli/cli/blob/trunk/pkg/cmdutil/json_flags.go) | source read |
| kubectl | [kubectl conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-cli/kubectl-conventions.md) and [`pkg/cmd/delete/delete.go`](https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/delete/delete.go) | read |
| cobra | [user guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md), [completions](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md), godoc of [`command.go`](https://github.com/spf13/cobra/blob/main/command.go) | read |
| 12-factor | [12 Factor CLI Apps](https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46) (Jeff Dickey, 2018) | **not verified**: Medium answered 403 and no mirror was reachable; nothing below rests on it |

## 1. A command's anatomy

- **One package per command, its constructor returning a `*cobra.Command`.** Why: the command's flags, help, logic and tests move together and nothing else imports them. Source: gh `docs/project-layout.md` ("Each command lives in a path following the pattern pkg/cmd/<command>/<subcommand>/<subcommand>.go"; "Keep command-specific logic in the command's own package"); kubectl conventions, "Command implementation conventions" (`NewCmd<CommandName>`).
- **Parse flags into one options struct, then hand it to a run function that never sees cobra.** Why: the run function is testable with a struct literal, and parsing is testable without running. Source: gh `pr/close/close.go` (`CloseOptions`, `RunE` fills it, `closeRun(opts)` does the work); kubectl conventions ("The `<CommandName>Options` struct contains a field for every flag and argument").
- **Split the run into complete → validate → run.** Why: defaults derived from the environment, checks, and the effect are three different failure classes. Source: kubectl `delete.go` (`o.Complete(f, args, cmd)`, `o.Validate()`, `o.RunDelete(f)`); the conventions page names the same order as `ToOptions`, `Validate`, `Run` and asks `Run` to take a `context.Context` "so that cancellation propagates from `cmd.Context()`".
- **Make the run function injectable for tests: `NewCmdX(f *cmdutil.Factory, runF func(*XOptions) error)`.** Why: a flag-parsing test captures the options and asserts them without running anything. Source: gh `pr/close/close.go` (`if runF != nil { return runF(opts) }`), `pr/merge/merge_test.go` (`NewCmdMerge(f, func(o *MergeOptions) error { opts = o; return nil })`).
- **Use `RunE`, return errors, and decide the exit code in one place.** Why: a command that calls `os.Exit` cannot be tested nor composed. Source: cobra user guide, "Returning and handling errors" ("If you wish to return an error to the caller of a command, `RunE` can be used. The error can then be caught at the execute function call."); gh `internal/ghcmd/cmd.go` (one switch over the error after `ExecuteContextC`).
- **Keep `main` bare.** Why: everything below it is a library a test can call. Source: cobra user guide, "Using the Cobra Library" / "Create your main.go" ("In a Cobra app, typically the main.go file is very bare. It serves one purpose: initializing Cobra.").
- **Never write to `os.Stdout`/`os.Stderr` directly from a command.** Why: tests and wrappers substitute the streams. Source: kubectl conventions ("Commands never write to `os.Stdout`/`os.Stderr` directly").

## 2. Dependency injection

- **gh: a `Factory` struct of eagerly built services and lazy constructors** (`IOStreams`, `Prompter`, `GitClient`, `HttpClient func() (*http.Client, error)`, `Config func()`, `BaseRepo func()`), copied by each `NewCmdX` into its options. Why: a command that never needs the network or the config never builds them, and a test builds a `Factory` literal with only what it stubs. Source: gh `pkg/cmdutil/factory.go`, `pkg/cmd/factory/default.go` (`New` wires them in dependency order: "Depends on Config and IOStreams"), `pr/close/close_test.go` (`&cmdutil.Factory{IOStreams: ios, HttpClient: …}`).
- **Its known cost: a service locator.** Each command reaches the whole bag, and gh's own comment on `Config` records the lazy/eager tension ("It would be nice if Config were just loaded once at startup… for now, we eagerly load the config and don't fail if there is an error"). Source: gh `pkg/cmdutil/factory.go`.
- **kubectl: depend on the narrowest interface that works.** Why: a command that takes only `genericiooptions.IOStreams` and a `RESTClientGetter` says what it reaches. Source: kubectl conventions, "kubectl client conventions" and "Command implementation conventions".
- **Rule to take from both:** build each expensive dependency lazily (on first use), hand a command only what it uses, and keep one place that knows the concrete types. A composition root with constructor injection satisfies all three without a factory object.

## 3. Streams and the terminal

- **Primary output to stdout; logs, progress, warnings and errors to stderr.** Why: stdout stays safe to pipe. Source: clig, "The Basics" ("The primary output for your command should go to `stdout`." / "Log messages, errors, and so on should all be sent to `stderr`."); kubectl conventions, "Output conventions".
- **Carry the streams in one object that also knows whether each is a TTY, and let tests force it.** Why: TTY-dependent behaviour (colour, prompts, spinners, pager) becomes testable without a pseudo-terminal. Source: gh `pkg/iostreams` (`IsStdinTTY`/`IsStdoutTTY`/`IsStderrTTY` with setters, `iostreams.Test()` returning buffers, `ios.SetStdoutTTY(isTTY)` in `close_test.go`).
- **Disable colour when the stream is not a TTY, `NO_COLOR` is set and non-empty, `TERM=dumb`, or `--no-color` is passed.** Why: escape codes corrupt files and pipes, and users opt out globally. Source: clig, "Output" (the four conditions, verbatim). gh also honours `CLICOLOR`/`CLICOLOR_FORCE` ([go-gh `pkg/term/env.go`](https://github.com/cli/go-gh/blob/trunk/pkg/term/env.go)).
- **Decide colour per stream, not once for the process.** Why: stdout piped and stderr on a terminal is the common case (`cmd | jq`). Source: clig's condition names both streams ("`stdout` or `stderr` is not an interactive terminal"); gh's `ColorScheme` is built from its `IOStreams`.
- **No animation when the output is not a terminal.** Why: a spinner redraws as garbage in a log. Source: clig, "Output" ("If `stdout` is not an interactive terminal, don't display any animations."); gh enables its progress indicator only when stdout and stderr are both TTYs (`System()`).
- **Print something within 100 ms and show progress on long work.** Why: silence reads as a hang. Source: clig, "Robustness" ("Print something to the user in <100ms."; "Show progress if something takes a long time.").
- **When a command changes state, say what changed.** Source: clig, "Output" ("If you change state, tell the user.").

## 4. Interactivity

- **Prompt only when stdin is a TTY; gh also requires stdout to be one.** Why: a script or CI job must never hang on a question. Source: clig, "Interactivity" ("Only use prompts or interactive elements if `stdin` is an interactive terminal (a TTY)."); gh `IOStreams.CanPrompt()` returns `IsStdinTTY() && IsStdoutTTY()` unless prompts are disabled.
- **Offer an explicit non-interactive escape.** Why: a TTY is not proof a human wants questions (an agent in a pty, `script`). Source: clig, "Interactivity" ("If `--no-input` is passed, don't prompt or do anything interactive."); gh: `IOStreams.SetNeverPrompt` makes `CanPrompt()` false (where gh sets it from was not read).
- **Without a terminal, require the flags instead of asking.** Source: clig, "Arguments and flags" ("If `stdin` is not an interactive terminal, skip prompting and just require those flags/args.").
- **Hide prompting behind an interface.** Why: a test supplies a mock and asserts the questions. Source: gh `internal/prompter.Prompter` (`Select`, `MultiSelect`, `Input`, `Confirm`…), its moq-generated `PrompterMock` used in `merge_test.go`. gh now backs it with `huh` (an accessible variant with `WithAccessible(true)`, and an experimental full `huh` prompter), with the older survey prompter as default.
- **Confirm before anything dangerous, scaled to the danger; `--force` skips it.** Source: clig, "Arguments and flags" (three levels, mild/moderate/severe; "Consider asking them to type something non-trivial such as the name of the thing they're deleting"); clig, "Interactivity" ("Confirm before doing anything dangerous").
- **Offer a dry run.** Source: clig, "Interactivity" ("Consider giving the user a way to "dry run" the operation…"); kubectl conventions, `--dry-run=none|client|server` on every mutation.
- **Never read secrets from flags; never echo a password.** Source: clig, "Arguments and flags" ("Do not read secrets directly from flags."), "Interactivity".

## 5. Machine output

- **Offer structured output behind a flag, and print nothing else on stdout when it is on.** Why: one stray human line breaks a parser. Source: clig, "Output" ("Display output as formatted JSON if `--json` is passed."); gh `pkg/cmdutil/json_flags.go` (`--json fields`, `--jq`, `--template`); kubectl conventions (`-o json|yaml|name|jsonpath…` through `PrintFlags`).
- **Keep human output free to change, machine output additive.** Source: clig, "Future-proofing" ("Keep changes additive where you can."; "Changing output for humans is usually OK.").
- **`-q/--quiet` suppresses non-essential output only.** Source: clig, "Output" ("provide a `-q` option to suppress all non-essential output").

## 6. Errors and exit codes

- **Zero on success, non-zero on failure, and map distinct codes to the failures a caller acts on.** Source: clig, "The Basics" ("Return zero exit code on success, non-zero on failure." / "Map the non-zero exit codes to the most important failure modes."). gh's map: `0` ok, `1` error, `2` cancelled, `4` auth, `8` pending (`internal/ghcmd/cmd.go`).
- **Catch errors and rewrite them for humans, most important line last, no stack traces.** Source: clig, "Errors" ("Catch errors and rewrite them for humans."; "Put the most important information at the end of the output."); clig, "Robustness" (avoid "scary-looking stack traces"); offer debug output behind a flag or a file.
- **Suggest the next command.** Source: clig, "Help" ("Suggest commands the user should run."). gh prints `Try authenticating with: gh auth login` on a 401 (`internal/ghcmd/cmd.go`).
- **Silence cobra's own error and usage printing and render errors yourself.** Why: cobra otherwise prints usage on every runtime error. Source: cobra godoc (`SilenceErrors` "quiet errors down stream", `SilenceUsage` "silence usage when an error occurs"); gh `pkg/cmd/root/root.go` sets both and installs its own `SetFlagErrorFunc` and usage func.
- **Print usage only for a usage error, typed as such.** Source: gh `cmdutil.FlagError` ("Such errors cause the application to display the usage message"), `printError` in `internal/ghcmd/cmd.go`.
- **Type the special outcomes: silent failure, user cancellation, pending.** Why: the root picks the exit code from the type, never from text. Source: gh `pkg/cmdutil/errors.go` (`SilentError`, `CancelError`, `PendingError`, `IsUserCancellation`).

## 7. Signals

- **On Ctrl-C, exit as soon as possible; bound clean-up with a timeout; say what a second Ctrl-C does.** Source: clig, "Signals and control characters" ("If a user hits Ctrl-C (the INT signal), exit as soon as possible."; "Add a timeout to any clean-up code so it can't hang forever."; "Tell the user what will happen when they hit Ctrl-C again, in case it is a destructive action.").
- **Keep Ctrl-C working while blocked on I/O or a prompt.** Source: clig, "Interactivity" ("If your program hangs on network I/O etc, always make Ctrl-C still work.").
- **Propagate cancellation through `cmd.Context()`.** Source: kubectl conventions (`Run` "takes a `context.Context`, so that cancellation propagates from `cmd.Context()`"); gh runs `rootCmd.ExecuteContextC(ctx)`.
- **Make operations crash-only and recoverable.** Why: a killed run must be safe to re-run. Source: clig, "Robustness" ("Make it crash-only."; "Make it recoverable.").

## 8. Arguments, flags, help, completions

- **Prefer flags to positional arguments; one positional kind at most.** Source: clig, "Arguments and flags" ("Prefer flags to args.").
- **Use standard flag names:** `-h/--help`, `-f/--force`, `-n/--dry-run`, `-q/--quiet`, `-o/--output`, `--no-input`, `--version`, `-d/--debug`. Source: clig, "Arguments and flags" (list). clig lists no `-y/--yes`.
- **Same flag, same meaning, in every subcommand.** Source: clig, "Subcommands" ("Be consistent across subcommands."); kubectl conventions, "Flag conventions".
- **Validate positional arguments with cobra's `Args`** (`ExactArgs`, `MatchAll`, …); declare relations with `MarkFlagsMutuallyExclusive` / `MarkFlagsOneRequired`. Source: cobra user guide, "Positional and Custom Arguments", "Flag Groups".
- **Lead help with examples; concise by default, extensive on `--help`.** Source: clig, "Help" ("Lead with examples."); cobra's `Example` field.
- **Embed help in the command's source.** Source: gh `docs/project-layout.md` ("Help text is embedded in the command's source").
- **Ship completions; complete flag values with `RegisterFlagCompletionFunc`, nouns with `ValidArgsFunction`, and `ShellCompDirectiveNoFileComp` where files make no sense.** Source: cobra completions doc, "Dynamic completion of nouns", "Specify dynamic flag completion".
- **No arbitrary abbreviations of subcommands, no catch-all subcommand.** Source: clig, "Future-proofing".

## 9. Testing a command

- **Parse tests:** build the command with an injected `runF`, `SetArgs`, execute, assert the captured options. Source: gh `pr/merge/merge_test.go` (`Test_NewCmdMerge`).
- **Run tests:** call the run function with an options literal over fake streams (`iostreams.Test()`), a forced TTY state, stubbed HTTP (`httpmock`), and a prompter mock. Source: gh `pr/close/close_test.go`, `pr/merge/merge_test.go`.
- **Never touch the real world in a unit test.** Source: gh `docs/project-layout.md`, "Writing tests" (commands read the repo, call the API, run git — "Tests must not do these things for real"; stub them, prefer table-driven tests).
- **Test both modes of every command: TTY and non-TTY.** Why: behaviour forks on it. Source: gh `runCommand(rt, isTTY, cli)` helper in `close_test.go`.
