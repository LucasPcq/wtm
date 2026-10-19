# The command contract

What every mutating command of `next/` promises its callers — people at a terminal, scripts, agents, other surfaces. A command keeps it through the shared pieces (`service/operation`, `ui/cli`, `ui/output`, `ui/form`, `api`); a deviation is an owner decision listed in its PR.

## Inputs

- A run is its `Request` (the command's fields, snake_case JSON tags) plus `domain.RunOptions` (`DryRun`, `ExpectFingerprint`, `Surface`, `CorrelationID`, `Report`). Everything a run depends on travels with it, so one process can serve several callers and repositories: the repository is the directory the use case is opened on (`open(ctx, dir)`, resolved to its main worktree), `--ask` is the request's `ask` field, the correlation ID is recorded by `operation.Service.Begin` on the operation and every event.
- The process environment is read once, at the edge: `ui/cli.ReadEnv` (working directory, `$WTM_PROJECT_DIR`, `$WTM_CORRELATION_ID`, an invalid one refused before any repository lookup), `ui/output.System` (which streams are terminals, `$TERM`), `$WTM_STATE_DIR` in `cmd/wtm-next/project.go`. An adapter reads only what its own tool needs (`os.UserConfigDir`, a child's environment). Domain, services and commands never read it. `[archlint]` (`envread`)
- The subject is positional (`create [branch…]`, `extract [source]`); a worktree that is not the subject is a flag named after its role (`--to`, `--from`). A flag whose absence means something (`--ff`) is a pointer in the request, set only when given.
- config.toml is validated at load (`adapter/config`): a file that does not parse or validate, an absolute `base_path` among them, refuses the run with `<command>.config_invalid` (precondition, exit 1, `invalid config.toml: <reason>`), as no config is `<command>.config_not_found`; `repository.Service.Observe` raises both.
- A field error names its path; the CLI spells it as its flag (`--files is required`, `<branches>[1]`).

## Interactive or unattended

- Interactive only when stdin and stderr (where the wizard draws) are terminals, `$TERM` is not `dumb`, the output is human, and neither `--yes` nor `--expect-fingerprint` is given (`output.Streams.CanPrompt`). Live progress needs only stderr a terminal (`CanAnimate`).
- Unattended otherwise: a field with a default takes it, a required field without one is a refusal naming its flag — never a picker. `--output json` is unattended by nature.
- `--yes` (`-y`), the confirmation axis: no question; each open decision takes its flag, else its remembered answer, else a documented safe default, never destructive.
- `--force`, the safety axis: it only lifts a safety refusal (dirty, unpushed, locked). It never implies `--yes`.
- A remembered answer (`config.toml` `[wizard.remembered]`, written by "always use this answer" or `--remember <field>`) stands in for the flag not given; `--ask` sets the memory aside for one run.

## Dry run and fingerprint

- `--dry-run` (`-n`) plans without a lock, an event or a change, and prints the plan, its warnings, its fingerprint and the line that applies exactly this plan (`… --expect-fingerprint <fp>`).
- `--expect-fingerprint <fp>` applies only if the facts, read under the locks, still give that fingerprint; otherwise `plan.stale` (exit 25), with the `--dry-run` that plans again as a follow-up (`plan.replan`, `uicli.Replan`).
- The fingerprint covers what the plan does (which files, their status), not the files' bytes.
- The wizard's recap is a dry run, applied with its fingerprint; a stale preview asks again.

## Output

- Human output is framed once (`output.Frame`) on stdout; progress, hook output and warnings during the run go to stderr; a refusal is framed on stderr. Colour is decided per stream (`NO_COLOR`, `TERM=dumb`, not a terminal).
- Success contracts to a count, anomalies are named one by one, every outcome ends with the next step (`→ wtm go <branch>`). A flag error prints the failure and one line pointing at `--help`, never the usage dump. `-q` silences human output, not errors, exit codes or JSON.
- `--output json` prints one v2 envelope on stdout and nothing else: `{v: 2, command, ok, op, items[{subject, status, reason, error, warnings, detail}], plan, fingerprint, follow_ups[{command, answers[{path, values}], dry_run, url, code, message, params}], warnings, error{kind, code, message, params, follow_up, fields[{path, code, flag, message…}]}}`. `Outcome.Envelope(command, err)` builds it, `domain.Envelope.JSON(spell)` writes it with each refused field spelled as the surface does (an API passes nil: the flag is the path).
- A step offered after the outcome (sync's "push now?", clean's retry with sudo) is a second exported entry of the use case, with its own request, operation and fingerprint, run like the first. The first never runs it: it returns a `domain.FollowUp{Command, Answers, DryRun, Code, Message, Params}`, on a failure in `error.follow_up`, else in `follow_ups` (a stale plan's replay, `DryRun` set, `uicli.Replan`): the wtm command it names, as its command line spells it after the program's name (`create`, `run up`), and its request as that command's answers, which a surface runs through the catalogue unchanged; a link to open instead has a `URL` and no command (list's pull request). `FollowUp` is deliberately one wire struct of these two shapes, not a sealed union: a surface tells them apart by `command` or `url`, and a JSON consumer reads one object type. Its `Argv` is the command line the CLI prints quoted for the shell (`output.FollowUpSteps`), never in the JSON: `form.Command` spells its own, and a follow-up naming another command gets it from a function `cmd/wtm-next` injects (`commandLine`, `provision.Service.LeftBehind`), so no service or `ui/` package knows that command.
- A surface also receives the request (its tags), its schema `domain.Form` (each field with its `kind`) and the events (`domain.MarshalEvent`); the events socket keeps its own v1 format (`adapter/events`).

## The catalogue

`api.Catalogue` (`next/internal/api`) lists the commands for a surface other than the CLI, one `api.Bind(name, open, questions, run)` per command in `cmd/wtm-next/catalogue.go`, its `Form` the schema of zero questions. A call's request is its `Answers` (`[{path, values}]`), its only input, folded by `Request.Answer` as the command line folds its flags. `Questions(ctx, call)` returns the form as the repository stands for that request (options, remembered answers, problems by path), what a surface renders and asks again after each answer. `Execute(ctx, api.Call{Dir, Answers, Options})` refuses a relative `Dir`, refuses an answer by its path (`request.invalid`, `fields`), runs it, follows a stale plan up with its replan (`domain.Replan`: its answers as a dry run), as the CLI does, (`Surface` defaults to `api`) and returns `api.Response{Status, Envelope}`: none 200, `invalid` 400, `refused` 403, `conflict` 409, `precondition` 412, `cancelled` 499, `internal` 500.

## Exit codes

| Code | Meaning |
| -- | -- |
| 0 | success, nothing to do, dry run |
| 1 | an item failed, an invalid config.toml (`<command>.config_invalid`), or a failure no other code names |
| 2 | invalid request (usage, field errors) |
| 12 | no wtm config (`<command>.config_not_found`, mapped by each command's `Exits`) |
| 15 | extract's conflict (`extract.conflict`) |
| 19 | cancelled (Ctrl+C once, a wizard left) |
| 21 | not a git repository |
| 22 | a state file written by a newer wtm |
| 23 | refused |
| 24 | conflict (a lock held, a worktree in the way) |
| 25 | precondition (`plan.stale`) |
| 130 | a second Ctrl+C: the process dies by the signal |

The kind decides (`ui/cli/exit.go`); a command maps its own codes in its `cli/` through `uicli.Run.Exits`.

## Locks, events, cancellation

- A mutating command runs inside an operation (`service/operation`): a record under the state directory, a shared lock on the repository, an exclusive lock per worktree, taken before observing; a plan naming a worktree the request did not is planned again once that lock is held. A dry run takes none.
- Events (`operation.*`, `unit.*`, `phase.*`, `worktree.created`/`provisioned`/`updated`/`numbered`) go out through the operation's report; a command adds an event only for a change another process must see.
- The first Ctrl+C cancels the context: the section in progress finishes (a shield), later items are skipped as interrupted, exit 19; the second kills the children's process groups. Details: `next/README.md`, "Cancellation".

## A read

A command that changes nothing, `command/list`, keeps this contract in place of the operation's.

- `Run(ctx, req, opts)` returns a `domain.Read[Detail]`, the signature `api.Bind` binds: one `Unit` per subject (`done` with its detail, `failed` with the error that kept it from being read, `cancelled` once the context is), the warnings, the follow-ups chosen. `[test]`
- It takes no lock, runs no operation and publishes no event: its envelope has no `op`, `plan` nor `fingerprint`, and it has no `contract_test.go`. `[test]` (`cmd/wtm-next`, `TestListIsReadThroughTheCatalogueWithNoOperation`)
- Its `cli` is a `form.Query`: the flags of its schema and `--output`, none of `--yes`, `--dry-run`, `--expect-fingerprint`, `--remember`, `--ask`. `[test]`
- What it cannot ask (a forge, the run daemon) is a warning under its own code, exit 0, the fields it would fill `null`; it never starts the run daemon. `[review]`
- It exits with the code of its first subject not read, after a pick too. `[test]`
- Its picker (`Query.Pick`) opens only when stdin, stdout and stderr are terminals, the output is human and `--quiet` is off; it walks `form.Script` steps with no recap (`Preview` nil), `Later` for a slow read, `Refresh` for `r`, and ends on one of the follow-ups its detail offers (list's `detail.actions`, each a command and its answers, whose argv `cmd/wtm-next` spells); leaving it is `cancelled`, exit 19. `[test]`
