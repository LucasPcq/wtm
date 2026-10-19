# Spike: a GUI over the catalogue (throwaway)

`wtm-next serve` serves `api.Catalogue` over local HTTP to one embedded page (`internal/ui/web`): `list` as a table, `create` as a form built from its `Questions`, preview by dry run, apply with the fingerprint, progress streamed by SSE. The test: no worktree logic in the server or the page.

## What held, with no change to a command

- **The form is data.** The page renders `create`'s `Form` by `kind` and shows what the core decided: options with badges, the pinned base branch, the origin of each answer (`config`, `default`, `remembered`), a skipped field and why, "always use this answer", the problems by path.
- **The request is the form's answers.** The page sends `[{path, values}]`, folded with `Request.Answer` exactly as the CLI folds its flags.
- **Dry run then apply.** `DryRun` returns the plan and its fingerprint; `ExpectFingerprint` refuses a stale plan (`plan.stale`, 412).
- **Progress.** `RunOptions.Report` to SSE is plumbing: `domain.MarshalEvent` already writes every event as JSON.
- **The envelope.** The same v2 envelope as `--output json`, refusals with their field errors, the HTTP status from the error's kind.
- **Cost.** A `questions` round trip (one observation of the repository) takes ~40 ms, `list` ~65 ms on the sandbox repository.

## What the spike had to add to the core

- `api.Command.Questions` and `api.Ask`: the catalogue only exposed the zero form (`create.Questions{}.Form()`), not `UseCase.Questions(ctx, req)`.
- `api.Call.Answers`: a request as the form's answers, beside the JSON request decoded from the Go struct's tags (two spellings of one request; the answers are the one every surface shares).

## Where a GUI would have to duplicate logic

1. **The recap of a plan** is in `create/cli/output.go`: `planRowsOf` decides "From" or "Parent", "(existing)", "(resumes …)", "(already open)", and the marks of recalled answers need `plan.Recalled()`, which the plan's JSON does not carry. The page shows the raw plan.
2. **The anomalies of an outcome** (`anomaliesOf`: a branch behind origin, the env ports left alone) are chosen in `create/cli`, not reported as warnings in the envelope.
3. **The actions of a worktree** (`list.Offers`) are a rule of the command the envelope does not carry, and a follow-up is an argv: a GUI needs "command + answers", not a shell line.

Each is presentation or a decision living in `cli/` that belongs in the use case's data; moving it serves the TUI, the JSON and any GUI at once.

## Smaller notes

- The questions of an empty request already carry "branches is required": a GUI decides when to show a problem (touched or not).
- A refused dry run (empty branches) still reports an `op` id.
- One server serves one repository (`Dir` fixed at start); the catalogue already takes the directory per call.
- Security: a token in the URL fragment sent as `X-Wtm-Token`, and a call from another origin refused.
