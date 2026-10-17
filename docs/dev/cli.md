# The command line — the root and the wiring of a command

`internal/surface/cli` (package `cli`) is the command line: the Cobra root, the signals, `--quiet` and the exit code of every run. The commands themselves live in its sub-packages (`wt/`, `run/`, `checkout/`, `initcmd/`…), each a runner that turns flags into a flow's `Request` and holds no business logic ([flow-layer.md](flow-layer.md)).

`main.go` calls `cli.Execute`; `tools/gendocs` walks `cli.Root()`.

## The root

- `root.go` builds the tree and its groups. Its `PersistentPreRunE` checks the flag groups, the `--output` format and `WTM_CORRELATION_ID`, starts the update check, applies `--quiet` and clears the cancelled mark; its `PersistentPostRunE` turns a run the user backed out of (`shared.Cancelled`) into `domain.ErrAborted` wrapping `domain.ErrCancelled`.
- `usage.go` routes every refusal cobra makes on its own through `rules.Usage` (exit 2): flag errors (`markUsageErrors`), an unknown command or one given arguments it takes none of (`unknownCommand`, `wrapArgs`), the flag groups (`validateFlagGroups`), an unknown `--output` (`validateOutputFormat`), a malformed correlation id (`validateCorrelationID`).
- `signal.go` is the root context (`interruptible`): the first SIGINT or SIGTERM cancels it, the second kills what a cancellation still waits on and ends the process by that signal once no shielded step is left half done. `settle`, the last thing a run does, kills what a cancellation still waits on and waits for every shielded step (`proc.KillCancelled`, `proc.AwaitShields`), so nothing the run started outlives it or is cut off half-way by the exit.
- `quiet.go` is `--quiet` (`silenceHumanOutput`): the writers of a human-output command become `io.Discard`; JSON, any other `--output` and a command annotated `domain.AnnotationMachineOutput` are left alone ([output.md](output.md#--quiet)).
- `abort.go` is the line a failed run ends on (`reportFailure`, `abortLine`): a cancellation reads as `domain.AbortedMessage` with the unchanged glyph, a bare `domain.ErrAborted` whose report `--quiet` threw away as `domain.QuietAbortedMessage`, anything else as its error.

`execute` runs the tree and answers the exit code: a run the user backed out of or interrupted reads as cancelled (`rules.BackedOut`, `rules.Interrupted`), `settle` runs, a report already written (`domain.ErrAborted`) is not printed twice unless `--quiet` threw it away, and the code is `rules.ExitCode(err)`, which finds the sentinel the error wraps. `Execute` adds the update notice and `os.Exit`.

## Wiring a command

A command is a `cobra.Command` built in its sub-package and attached to the root in `root.go`, in its group. Its runner reads the flags, builds the flow's `Request`, and hands the flow a `Prompter` and a `Presenter`:

```go
// surface/cli/wt/sync.go
interactive := rules.IsHumanFormat(format) && shared.StdinIsTerminal() && !yes
_, err = syncflow.Run(cmd.Context(), syncflow.Params{
	Context:   shared.FlowContext(config),
	Request:   syncflow.Request{Branches: args, All: all, Push: push, DryRun: dryRun /* … */},
	Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
	Presenter: syncPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
})
```

`surface/cli/shared` holds what every runner shares: the flags (`AddOutputFlag`, `AddYesFlag`, `AddAskFlag`, `AddIsolationFlag`…), `RequireYesInJSON` (JSON without `--yes` is a usage error on a command that could ask), `Interactive` (stdin is a terminal, the output is human, no `--yes`), the project root and config (`ProjectRoot`, `LoadConfig`), the flow seams (`FlowContext`, `FlowPrompter`, `NewPresenter`) and the cancelled mark (`MarkCancelled`, `Cancelled`, `BackedOut`). Names, flags and messages are constants of `domain`.

After adding or changing a command or a flag, `make docs` regenerates `docs/wtm_*.md`.

## What the lint holds

- `layers`: `surface/cli` imports the flows, the services and `surface/tui`, never the other way round; `surface/cli/render` formats only.
- `yesflag`: a command that reads `shared.Interactive` registers `--yes`.
- `chokepoint`: a runner never calls a service mutator itself — it goes through its flow.

## Testing

The root's tests (`cancel_test.go`, `quiet_test.go`, `interrupt_test.go`, `shield_test.go`, `usage_test.go`) use the stdlib `testing`; their helpers sit in `helpers_test.go`. A command's tests sit in its sub-package.
