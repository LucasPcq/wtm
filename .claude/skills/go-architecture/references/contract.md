# The command contract

What every mutating command of `next/` promises its callers — people at a terminal, scripts, agents — whatever its subject. A new command implements all of it through the shared pieces (`service/operation`, `ui/cli`, `ui/output`, `ui/form`); a deviation is a decision written in the command's docs.

## Contents

- Inputs: request and options
- Interactive or unattended
- `--yes` and `--force`: two axes
- Remembered answers
- Dry run and fingerprint
- Output
- The catalogue
- Exit codes
- Locks and events

## Inputs: request and options

- A command's input is its `Request` (the command's own fields, JSON tags in snake_case) plus `domain.RunOptions` (`DryRun`, `ExpectFingerprint`, `Surface`, `CorrelationID`, `Report`), the same for every command. Everything an execution depends on travels with it, so one process can serve several callers and repositories: the correlation ID in the options (`operation.Service.Begin` records it on the operation and every event, and refuses an invalid one), the repository as the directory the use case is opened from (`open(ctx, dir)`, always resolved to its main worktree), `--ask` as the request's `ask` field. Only the CLI reads the environment, once (`uicli.ReadEnv`), except the process-wide `$WTM_STATE_DIR`, and it refuses an invalid `$WTM_CORRELATION_ID` before any repository lookup.
- The subject is positional (`create [branch…]`, `extract [source]`); a worktree that is not the subject is a flag named after its role (`--to`, `--from`).
- A flag whose absence means something (`--ff`, `--keep`) becomes a pointer in the request only when it was given.
- Every field error names its path; the CLI turns the path into its flag (`form.Command.Flags`, from the schema's `Form.Flags`) so the message says `--files is required`, never `files is required`.

## Interactive or unattended

- A run is interactive only when stdin and stderr (where the wizard draws) are terminals, `TERM` is not `dumb`, the output is human, `--yes` is not given and no `--expect-fingerprint` is passed; then the wizard asks what the request leaves open. Live progress follows the same terminal rule on stderr alone.
- Otherwise it is unattended: a field with a default takes it, a required field without one is a refusal naming its flag. Never fall back to a picker.
- `--output json` is unattended by nature; `--yes` is accepted with it and changes nothing.

## `--yes` and `--force`: two axes

- `--yes` (`-y`) is the confirmation axis: no question, no confirmation; every open decision takes its flag, else its remembered answer, else a documented safe default that is never destructive.
- `--force` is the safety axis: it only lifts a safety refusal (dirty, unpushed, locked…). It never implies `--yes`: `--force` alone still asks.

## Remembered answers

- A rememberable question (an env strategy, an isolation, a source update) offers "always use this answer in this repo"; `--remember <field>` does the same from the command line. The answer lands in `config.toml` `[wizard.remembered]`.
- A remembered answer stands in for the flag not given, interactive or not, ahead of the config default; `--ask` ignores the memory for one run.
- A flag or a remembered answer never makes a recap line disappear: the recap shows the value and where it came from.

## Dry run and fingerprint

- `--dry-run` (`-n`) plans without changing anything and prints the plan, its warnings, its fingerprint and the command line that applies exactly this plan (`… --expect-fingerprint <fp>`).
- `--expect-fingerprint <fp>` applies only if the facts still give that fingerprint; otherwise it refuses with `plan.stale` (exit 25) and prints the dry-run line that shows the plan the facts give now.
- The fingerprint covers what the plan does (which files, their status), not the files' bytes: a file edited between the preview and the apply is applied as it is then.
- The wizard's recap is a dry run: it previews the plan, then applies it with its fingerprint; a stale preview asks again.

## Output

- Human output is framed once (`ui/output.Frame`) on stdout; progress, hook output and warnings that come while it runs go to stderr; refusals are framed on stderr. Colour is decided per stream (`NO_COLOR`, `TERM=dumb`, not a terminal), so `2>log` gets no escape codes.
- A flag error (unknown flag, bad value) prints the failure, then one line pointing at the help (`→ wtm-next create --help`), never the usage dump.
- `--output json` prints one v2 envelope on stdout, never framed, nothing else on stdout: `{v: 2, command, ok, op, items[{subject, status, reason, error, warnings, detail}], plan, fingerprint, follow_ups[{argv, code, message, params}], warnings, error{kind, code, message, params, follow_up{argv, code, message, params}, fields[{path, code, flag, message…}]}}`; `argv` is the command line to run next, word by word. A stale plan's refusal carries in `follow_ups` the `--dry-run` that plans again (code `plan.replan`), the line the human output prints. A follow-up's `argv` carries the request's words only: the caller adds its own surface flags (`--output json`, `--yes`), so the replay drops them by design.
- The envelope is data: each command's `func (o Outcome) Envelope(err *domain.Error) domain.Envelope` reports how its run ended, `err` being `Run`'s refusal; it is the one JSON shape of an outcome. `domain.Envelope.JSON(spell)` writes it, `spell` naming each refused field as the surface does: the CLI passes its flag table (`--files`, `<branches>[1]`), an API passes nil and each field's `flag` is its path. `*domain.Error` and `domain.FieldError` encode as the envelope's `error` and `fields` with the path; `Cause` and `Template` are never emitted (`message` carries them).
- What else a surface receives is JSON too: the request (its tags), its schema `domain.Form` (each field with its `kind`: `select`, `multi_select`, `text`, `text_list`, `confirm`, then what that kind carries, and the `problems`), and the events (`domain.MarshalEvent`/`UnmarshalEvent`: `{"type": …, fields}`, the socket's names where it publishes the event — `unit.finished`, `worktree.created`, `worktree.provisioned`, `operation.started`/`finished` — plus `unit.started`, `phase.started`, `phase.finished`, `phase.output` and `worktree.numbered`). The events socket keeps its own v1 format (`adapter/events`).
- `-q`/`--quiet` silences human output; errors and the exit code are unchanged, and JSON still prints its document.
- Success contracts to a count; anomalies are named one by one; every outcome ends with the next step (`→ wtm go <branch>`).

## The catalogue

- `next/internal/api` lists the commands for a surface other than the CLI, knowing none of them: `api.Catalogue` of `api.Command{Name, Form, Execute}`, built in `cmd/wtm-next/catalogue.go` by one `api.Bind(name, Questions{}.Form(), open, UseCase.Run)` per command.
- `Execute(ctx, api.Call{Dir, Request, Options})` refuses a `Dir` that is not absolute (`request.invalid`, before opening anything), opens the use case on `Dir` for that call, decodes `Request` strictly (an unknown field or anything after the one JSON object is `request.invalid`; keys match case-insensitively, as `encoding/json` does), runs it with `Options` (`Surface` defaults to `api`) and returns an `api.Response{Status, Envelope}`.
- The status comes from the kind of the failure: none 200, `invalid` 400, `refused` 403, `conflict` 409, `precondition` 412 (`plan.stale` among them), `cancelled` 499 (nginx's "client closed request": a caller's cancel, and an operation a server shutdown interrupted), `internal` 500 (a failed item included). The CLI's exit codes stay in `ui/cli`.

## Exit codes

| Code | Meaning |
| -- | -- |
| 0 | success, nothing to do, dry run |
| 1 | an item failed (hook failure, half move…) |
| 2 | invalid request (usage, field errors) |
| 12 | no wtm config (`<command>.config_not_found`) |
| 15 | extract's conflict (`extract.conflict`) |
| 19 | cancelled (Ctrl+C once, a wizard cancelled) |
| 21 | not a git repository |
| 22 | a state file written by a newer wtm |
| 23 | refused |
| 24 | conflict (a lock held, a worktree in the way) |
| 25 | precondition (`plan.stale`) |
| 130 | killed by a second Ctrl+C |

A command maps its own codes in its `cli/`; everything else follows the kind (`ui/cli/exit.go`).

## Locks and events

- Every mutating command runs inside an operation (`service/operation`): a record under the state directory, a shared lock on the repository and an exclusive lock per worktree it touches, taken before observing; a plan that names a worktree the request did not is planned again once that lock is held.
- Events (`operation.started`/`finished`, `worktree.created`/`provisioned`/`updated`) go to the events socket through the operation's report; a command adds an event only for a change other processes must see.
