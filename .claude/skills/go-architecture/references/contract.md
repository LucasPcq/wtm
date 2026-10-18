# The command contract

What every mutating command of `next/` promises its callers — people at a terminal, scripts, agents, other surfaces. A command keeps it through the shared pieces (`service/operation`, `ui/cli`, `ui/output`, `ui/form`, `api`); a deviation is an owner decision listed in its PR.

## Inputs

- A run is its `Request` (the command's fields, snake_case JSON tags) plus `domain.RunOptions` (`DryRun`, `ExpectFingerprint`, `Surface`, `CorrelationID`, `Report`). Everything a run depends on travels with it, so one process can serve several callers and repositories: the repository is the directory the use case is opened on (`open(ctx, dir)`, resolved to its main worktree), `--ask` is the request's `ask` field, the correlation ID is recorded by `operation.Service.Begin` on the operation and every event.
- The process environment is read once, at the edge: `ui/cli.ReadEnv` (working directory, `$WTM_PROJECT_DIR`, `$WTM_CORRELATION_ID`, an invalid one refused before any repository lookup), `ui/output.System` (which streams are terminals, `$TERM`), `$WTM_STATE_DIR` in `cmd/wtm-next/project.go`. An adapter reads only what its own tool needs (`os.UserConfigDir`, a child's environment). Domain, services and commands never read it.
- The subject is positional (`create [branch…]`, `extract [source]`); a worktree that is not the subject is a flag named after its role (`--to`, `--from`). A flag whose absence means something (`--ff`) is a pointer in the request, set only when given.
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
- `--output json` prints one v2 envelope on stdout and nothing else: `{v: 2, command, ok, op, items[{subject, status, reason, error, warnings, detail}], plan, fingerprint, follow_ups[{argv, code, message, params}], warnings, error{kind, code, message, params, follow_up, fields[{path, code, flag, message…}]}}`. `Outcome.Envelope(command, err)` builds it, `domain.Envelope.JSON(spell)` writes it with each refused field spelled as the surface does (an API passes nil: the flag is the path). `argv` holds the request's words only; the caller adds its own surface flags.
- A step offered after the outcome (sync's "push now?") is a second entry of the use case; the first returns a `domain.FollowUp` (on a failure in `error.follow_up`, else in `follow_ups`). A follow-up naming another command gets its argv from `cmd/wtm-next`.
- A surface also receives the request (its tags), its schema `domain.Form` (each field with its `kind`) and the events (`domain.MarshalEvent`); the events socket keeps its own v1 format (`adapter/events`).

## The catalogue

`api.Catalogue` (`next/internal/api`) lists the commands for a surface other than the CLI, one `api.Bind(name, form, open, run)` per command in `cmd/wtm-next/catalogue.go`. `Execute(ctx, api.Call{Dir, Request, Options})` refuses a relative `Dir`, decodes `Request` strictly (`request.invalid`), runs it (`Surface` defaults to `api`) and returns `api.Response{Status, Envelope}`: none 200, `invalid` 400, `refused` 403, `conflict` 409, `precondition` 412, `cancelled` 499, `internal` 500.

## Exit codes

| Code | Meaning |
| -- | -- |
| 0 | success, nothing to do, dry run |
| 1 | an item failed, or a failure no other code names |
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
