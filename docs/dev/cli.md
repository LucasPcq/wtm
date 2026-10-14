# The command line — how a command of the registry is served

`internal/surface/cli` (package `cli`) is the command line: the Cobra root, the signals, `--quiet` and the exit code of every run, and the commands generated from the registry of [dispatch.md](dispatch.md). A command on the engine is written once in `command/`; its command line is a `cli.Bind` call, and everything else — flags, the run, the rendering, the JSON — is the same for all of them. The commands still on `flow/` keep their hand-written runners in the sub-packages (`wt/`, `run/`…) until they migrate.

`main.go` calls `cli.Execute`; `tools/gendocs` walks `cli.Root()`.

## The root

`root.go` builds the tree and its groups; `usage.go` routes every refusal cobra makes on its own through `ErrUsage`; `signal.go` is the root context (the first SIGINT or SIGTERM cancels it, the second ends the process once no shielded step is left half done) and `settle`, the last thing a run does: it kills what a cancellation still waits on and waits for every shielded step, so nothing the run started outlives it or is cut off half-way by the exit; `quiet.go` is `--quiet`, `abort.go` the line a silenced or interrupted run ends on. `execute` runs the tree and answers the exit code: a run the user backed out of or interrupted reads as cancelled (`rules.BackedOut`, `rules.Interrupted`), a report already written (`domain.ErrAborted`) is not printed twice unless `--quiet` threw it away, and the code comes from `exitCode` (below). `Execute` adds the update notice and `os.Exit`.

## Binding a command

```go
// surface/cli/create.go
var createCmd = cli.Bind(domain.CmdCreate, cli.Doc{
	Use: domain.CmdCreate + " [branch...]", Short: "…", Long: "…", Example: "…",
}, cli.Positional("branches"), cli.Flag(cli.FlagParams{Path: "fast_forward", Name: "ff"}), cli.Detail(createDetail{}))
```

The name is a constant of `domain`, the one the command declares as its `Name`: `surface/` never imports `command/` (`engine`). `Bind` looks the name up in the registry (`registry.Production`, read on a dispatcher with no `Params`: the tree is built outside any repository, for `--help` and the docs) and panics on a name it does not hold, so a wrong name never reaches a user. The engine builds the catalogue of that registry once, in `NewEngine`: two entries spelling one code stop wtm at start-up. Every sentence of a run comes from it. `Doc` is what only a terminal reads; the rest comes from the command's `FieldSpec`s.

| Option | |
| -- | -- |
| `Positional(path)` | the field takes the arguments instead of a flag: all of them for a list field, one otherwise (two is a `422 too_many`). A bool or decisions field cannot be positional |
| `Flag(FlagParams{Path, Name, Short})` | the field's flag is spelled `Name` instead of its path, or gets a short form |
| `Detail(render.Detail)` | the command gives the rows of its plan and renders its done items itself |
| `GenericDetail()` | the generic rendering is declared enough |

## Flags

A field's flag is its path in kebab case (`env_from` → `--env-from`, `target.base` → `--target-base`), its help is the field's `Description` (its `Label` when it has none) — the same text the form, the schema and the docs show — and its type follows the field's:

| `FieldType` | Flag |
| -- | -- |
| `text`, `select` | `--name value` |
| `bool` | `--name`, `--name=false` |
| `textlist`, `multiselect`, `reorder` | `--name value`, repeatable |
| `decisions` | `--name KEY=VALUE`, repeatable |

An `Enum` constraint is offered to shell completion. No check comes from a flag: the `Constraints` and the `Rules` of the command, applied by `dispatch`, are the only validation, and their `FieldError`s are spelled back as the flag or the positional (`binding.spell`: `--from`, `<branches>`, `<branches>[2]`).

The request is JSON built from the flags **passed** (`Changed`) and the arguments, nested along the paths: a flag left out never reaches the request, so its default stays dispatch's to decide (a remembered answer included). `--yes`, `--output`, `--dry-run` and `--expect-fingerprint` are added once to every command, `--ask` to a command with a `Rememberable` field; `--quiet` is the root's. A flag cobra cannot parse, or an argument a command takes none of, is a `422` like any other, envelope included.

## The run

`binding.run` is the same for every command — the route and the call in `run.go`, the conclusion and the failure in `conclusion.go`, the keys of the envelope a run reached in `envelope.go`:

1. **The route**, read once: human or JSON (`--output`), interactive (`shared.Interactive`: stdin is a terminal, the output is human, no `--yes`), and the `dispatch.Options` — `DryRun`, `ExpectFingerprint`, `IgnoreRemembered` (`--ask`), `Surface: dispatch.SurfaceCLI`, `CorrelationID` (`WTM_CORRELATION_ID`).
2. **The request** from the flags; a malformed one is a `422` without calling anything.
3. **The dispatcher** of the repository the command runs in (`projectDispatcher`: the main worktree, `<common>/wtm`, the event bus), built when a command runs and never when the tree is, and the command's `Entry`.
4. **The call.** An interactive run of a command goes to the engine's `Form` (an engine with none sends it the unattended way) with `FormParams{Entry, Request, Options, Names, Detail, Progress}`: the command's `Fillable`, the request the flags filled, the options, the spelling of each field, the command's `Detail` (the form draws its recap with it) and the progress view. The production engine's is the interactive form ([form.md](form.md)): it asks what is missing, shows the plan, then runs it with the fingerprint it showed. Every other run, and a query, goes to `Entry.Call` with the request and the options. `--yes` changes nothing in JSON. The progress goes to a `render.ProgressView` on stderr.
5. **The conclusion**, from the `Reply` (a switch over `CommandReply` and `QueryReply`): the envelope in JSON, else the outcome, the recap of a dry run or the failure (what each prints: [output.md](output.md#a-command-on-the-engine)). A failure returns `reported`, which carries the error to `exitCode` and its one-line form to `Execute`. A reply the form returns is concluded the same way: the form draws no conclusion.
6. **The follow-ups** of an interactive run that applied, one by one (`follow`): the engine's `Confirm` (the form's `FollowUp` in production) shows the command line that runs it; a yes runs that command of the tree through the form, its request filled in, and concludes it in turn. A follow-up this command line does not serve stays the line the conclusion printed. The run's own failure is the one that exits.

A follow-up, and the next step of a recap or a `412`, is printed as the command line that runs it: the target's binding spells the request (`Engine.commandLine`, `commandline.go`), kebab-case flags for a command not served here, each word a shell would read single-quoted.

## Exit codes

`exitCode` (`exit.go`) reads a failure of the engine by its code — the `ExitCodes` of the entry that ran (`reported` carries them), then `exitByCode`, the engine's own (`state.schema_newer` → 22) — then by its kind (`invalid` 2, `cancelled` 19, `refused` 23, `conflict` 24, `precondition` 25, the rest 1). An `internal` error, and every error of a legacy command, goes to `rules.ExitCode`, which finds the sentinel it wraps. A batch exits on its outcome's failure, the rule `operation.finished` publishes too (`kernel.Outcome.Failure`: its first interrupted item, else its first failed item). A command on the engine keeps the number its legacy error had in its own `ExitCodes` ([commands.md](commands.md#a-commands-codes)).

## Parity

`parityGaps` holds the registry and the tree together, and `TestEveryEntryOfTheRegistryIsServedOrDeclaredUnbound` runs it on the real ones: each entry is served by a command of the tree (`annotationEntry`) declaring its rendering (`Detail` or `GenericDetail`), or listed in `unbound` with why. An exclusion that no longer holds — the entry is served, or gone — is a gap too. A future surface — an HTTP route, a graphical screen — is held to the same registry by a parity check of its own.

## Adding a command to the command line

1. Its line in `registry.Production`, with its real ports, its `Messages` and its `ExitCodes`.
2. A file in `surface/cli` with its `cli.Bind`: `Doc`, the positional, a renamed flag if the legacy command had one, its `Detail` or `GenericDetail()`.
3. Attached to the root in `root.go`, in its group.
4. `make docs`: the flags and their help come from the fields.

## What the lint holds

- `layers`: `surface/cli` may import `dispatch`, `kernel` and `surface/tui/form`; `surface/cli/render` may import `kernel` (the results it renders), never `dispatch`.
- `engine`: `surface/` never imports `command/` or `query/` — a command is reached by its name in the registry.
- `yesflag`: the file that reads `shared.Interactive` registers `--yes` (`bind.go`).

## Testing

The tests of the generated command line follow the kernel's layout, with testify: `fixture_test.go` is a command shaped like `create` — text, list, select, bool and decisions fields, a rememberable one, a decisions map, a nested path, a query beside it — over a fake repository; `helpers_test.go` serves it on a root of its own with the real hooks, on the `dispatchtest` bench, and runs it through `execute` down to the exit code (`world.run`), stdin's terminal faked by `shared.StdinIsTerminal`. `run_test.go` holds the end-to-end cases (help, 422 in both formats, dry run and fingerprint, 412, envelope, exit codes, `--quiet`, the interactive route); `form_test.go` the follow-ups an interactive run offers, with a form that applies what it is given and a scripted `Confirm`; the other files test their source file. The generic rendering is tested in `render` on its own (`envelope_test.go`, `outcome_test.go`, `progress_test.go`…), the live view on a buffer (`ProgressViewParams.Live`). The tests of the root that do not go through the engine (`cancel_test.go`, `quiet_test.go`, `interrupt_test.go`, `shield_test.go`, `usage_test.go`) keep the stdlib `testing`; their helpers sit in the same `helpers_test.go`.
