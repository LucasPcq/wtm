# The command contract

What every mutating command of `next/` promises its callers — people at a terminal, scripts, agents — whatever its subject. A new command implements all of it through the shared pieces (`service/operation`, `ui/cli`, `ui/output`, `ui/form`); a deviation is a decision written in the command's docs.

## Contents

- Inputs: request and options
- Interactive or unattended
- `--yes` and `--force`: two axes
- Remembered answers
- Dry run and fingerprint
- Output
- Exit codes
- Locks and events

## Inputs: request and options

- A command's input is its `Request` (the command's own fields, JSON tags in snake_case) plus `operation.Options` (`DryRun`, `ExpectFingerprint`, `Surface`, `Report`), the same for every command.
- The subject is positional (`create [branch…]`, `extract [source]`); a worktree that is not the subject is a flag named after its role (`--to`, `--from`).
- A flag whose absence means something (`--ff`, `--keep`) becomes a pointer in the request only when it was given.
- Every field error names its path; the CLI turns the path into its flag (`flag.go`) so the message says `--files is required`, never `files is required`.

## Interactive or unattended

- A run is interactive only when stdin is a terminal, the output is human, `--yes` is not given and no `--expect-fingerprint` is passed; then the wizard asks what the request leaves open.
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

- `--dry-run` plans without changing anything and prints the plan, its warnings, its fingerprint and the command line that applies exactly this plan (`… --expect-fingerprint <fp>`).
- `--expect-fingerprint <fp>` applies only if the facts still give that fingerprint; otherwise it refuses with `plan.stale` (exit 25) and prints the dry-run line that shows the plan the facts give now.
- The wizard's recap is a dry run: it previews the plan, then applies it with its fingerprint; a stale preview asks again.

## Output

- Human output is framed once (`ui/output.Frame`) on stdout; progress, hook output and warnings that come while it runs go to stderr; refusals are framed on stderr.
- `--output json` prints one v2 envelope on stdout, never framed, nothing else on stdout: `{v: 2, command, ok, op, items[{subject, status, reason, error, warnings, detail}], plan, fingerprint, follow_ups, warnings, error{kind, code, message, params, follow_up, fields[{path, code, flag, message…}]}}`.
- `-q`/`--quiet` silences human output; errors and the exit code are unchanged, and JSON still prints its document.
- Success contracts to a count; anomalies are named one by one; every outcome ends with the next step (`→ wtm go <branch>`).

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
