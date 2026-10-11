# Dispatch — how a command runs

`internal/dispatch/` is the impure half of the command engine, beside the pure contract of [commands.md](commands.md): the one way a surface reaches a command or a query. It takes the operation locks, observes, fills in the defaults, validates, plans, checks the fingerprint, applies and publishes what happened, in the same order for every command, so the locks, the fingerprint and the events apply whatever the surface (the CLI, the form, later an API).

No command runs through it yet: `create` is the first (step 3 of LUC-283), the generated CLI (LUC-288) and the form (LUC-289) are its first surfaces.

## Building it

```go
d := dispatch.New(dispatch.Params{
	Dir:       dir,       // what a command observes
	StateDir:  stateDir,  // <common>/wtm: the locks and the hook logs
	Publisher: publisher, // the event bus (events.Publisher); nil publishes nothing
})
created := dispatch.Register(dispatch.RegisterParams[create.Request, create.Facts, create.Plan, create.Detail]{
	Dispatcher: d,
	Command:    create.Command(ports),
})
registry := dispatch.Registry{created, dispatch.RegisterQuery(operations.Query(d.Operations))}
```

`Now`, `NewOpID` and `Shield` default to the clock, a random id and `proc.Shield`; the test bench replaces them.

- **`Handle[Req, F, P, D]`** is a command bound to its dispatcher: typed, for a surface written for that command.
- **`Registry`** is the list of what a surface can run, an `Entry` per command or query, found by `Lookup(name)`. An `Entry` erases the types: `Call` reads the request as JSON (its names are the paths of the fields) and answers a sealed `Reply`, a `CommandReply` (the `Result` with its plan and item details as JSON, statuses typed) or a `QueryReply` (the view as JSON). A field the request does not have is a `422` naming it (`request.unknown_field`), never ignored.

## The form

| Call | Returns | Cost |
| -- | -- | -- |
| `Schema()` | the `FieldSpec` of every field | none: read it once |
| `Observe(ctx)` | `Snapshot{Facts, Fingerprint}` | the command's I/O; observe again to refresh |
| `State(FormParams{Request, Facts})` | `kernel.Form`: each field's state, the defaults filled in, `Complete` | pure, `kernel.Evaluate` |
| `Choices(ctx, ChoicesParams{Path, Request, Facts})` | the field's options, each that would break a rule carrying its `Refusal` (`kernel.Disabled`) | the field's own I/O |

A form keeps the snapshot while the user types: entering a value calls `State` again, never `Observe`. The fingerprint of the snapshot shown in the recap is the one handed to `Run`.

## The run

`Handle.Run(ctx, RunParams{Request, Options, Progress})` goes through these steps, always in this order:

1. **Locks.** The holder file, then the command's locks (`Command.Locks(req)`), all or none and without waiting. A key another operation holds is a `409 lock.held`.
2. **`Observe`**, under the locks: no other wtm process changes the facts between here and the end of `Apply`. The fingerprint is computed now.
3. **`Forget`**, when `IgnoreRemembered` is set (`--ask`): the facts lose the remembered answers, so each field falls back on its other default.
4. **Defaults and validation**, `kernel.Evaluate`: an incomplete request is a `422` naming its fields.
5. **`Plan`.** A dry run stops here and returns the plan and the fingerprint: nothing applied, nothing published.
6. **The fingerprint check.** `ExpectFingerprint` different from the facts just read is a `412 plan.stale`.
7. **`Apply`**, handed the request with its defaults, the facts, the plan, the progress `Emitter` and the `Shield`.
8. **Release** of the locks, then the `Result`.

| `Options` | |
| -- | -- |
| `DryRun` | stop after `Plan` |
| `ExpectFingerprint` | the fingerprint a preview returned (`--expect-fingerprint`, a form's recap) |
| `IgnoreRemembered` | drop the remembered answers before the defaults (`--ask`) |
| `Surface`, `CorrelationID` | recorded in the holder and on the events |

`Result{Op, Fingerprint, Plan, Outcome}` carries the plan always and the outcome unless the run was a dry run. A run that could not take its locks (no `flock`, an unwritable state dir) goes on unprotected and says so with the warning `locks.unavailable`, in the outcome and in the plan.

## Errors

Every error `Run` returns is a `kernel.Error`. The codes of `dispatch` are `kernel.Code` constants in `codes.go`, their messages in `kernel/text`.

| Code | Kind | As HTTP | Params |
| -- | -- | -- | -- |
| `request.invalid` | `invalid` | 422 | `fields`: each `FieldError`, `request.unknown_field` included |
| `lock.held` | `conflict` | 409 | `key` (`repo`, `wt:<branch>`), and the holder: `op`, `command`, `pid`, `surface`, `started_at` |
| `plan.stale` | `precondition` | 412 | `expected`, `found` |
| `state.schema_newer` | `precondition` | 412 | `path`, `found`, `supported` |

`state.schema_newer` is the `*statefile.NewerError` a write returns (exit code `22`), found by `errors.As` wherever it was wrapped. Anything else goes through `kernel.Classify`: a cancellation is `cancelled`, the rest `internal`.

## Operation locks

| File | Mode | Taken by |
| -- | -- | -- |
| `<common>/wtm/locks/repo.lock` | shared (`LockShared`) | every command that targets worktrees |
| `<common>/wtm/locks/repo.lock` | exclusive (`LockExclusive`) | `prune`, applying the run config, the addressing switch, a batch `reparent` |
| `<common>/wtm/locks/wt/<branch>.lock` | exclusive | every command on that worktree (the branch escaped as in `worktrees/`) |
| `<common>/wtm/locks/ops/<op_id>.json` | exclusive, held by its operation | every run: the holder |

- **All or nothing, never waiting.** `fs.TryLocks` takes the keys in sorted order, each with `LOCK_NB`, and releases them all on the first refusal. Two sets taken in opposite orders cannot deadlock.
- **The holder is a file per operation**: `{op_id, pid, command, surface, correlation_id, started_at, locks}`, written and locked before it appears (`fs.HoldFile`), removed on release. The lock files themselves hold nothing, so several shared holders of `repo.lock` are each named by their own file. A `409` names the operation whose holder lists the refused key.
- **A dead process holds nothing.** Its locks go with its descriptors; its holder file is no longer locked, and `fs.HeldFiles` removes it on the next read. There is no force-unlock.
- **From before `Observe` to the end of `Apply`.** No nested run: `command/` never imports `dispatch`, a command made of others declares the union of their keys, and a `FollowUp` is a new call.
- **Without `flock`** (NFS, some WSL mounts) the run is unprotected and warns `locks.unavailable`.
- **A query locks nothing**: the atomic writes of `infra/fs` give it a file before or after a write, never half of one.

`query/operations` lists the operations in progress from the holder files still held: `operations.Query(d.Operations)`.

## The fingerprint

The fingerprint is the SHA-256 of the facts as JSON (maps in key order), cut to 24 hex characters: one notion of "what was read", the same in the preview and in the run. Only what the facts export to JSON counts: a fact a command checks belongs in its JSON.

## Progress, hook logs and events

`Apply` is handed an `Emitter` that, for each `kernel.Progress`:

- stamps it with the operation's `op_id`;
- writes the output of a hook phase (a phase named `on_create` or `on_clean`) to its log, `rules.HooksLogPath` under the state dir, opened by `fs.OpenLog` when the phase starts and closed when it ends, whatever the surface shows of it;
- publishes `unit.finished` when a unit ends;
- forwards it to the surface's `Progress`.

On the bus (`docs/guide/events.md`) an applied run sends `operation.started`, one `unit.finished` per item, then `operation.finished` with `ok` and, on failure, the error's `code`. Each carries `op_id` and the `correlation_id`. They are sent even once the run is interrupted. A dry run and a run refused before `Apply` publish nothing. Output lines never go on the bus.

## Ports

`kernel` imports only the stdlib, so a port never travels through its types (`kernel.ObserveScope` is data only). A command receives its ports when it is built, `create.Command(ports)`, and captures them in `Observe`, `Choices` and `Apply`. Each port is an interface declared beside what it wraps (`infra/git`, `store/`, `infra/proc`), with its double in `testutil/`; `dispatch` builds the registry with the real ones. `dispatch` runs on its own ports only: `Params` above.

## What the lint holds

- `layers`: `dispatch` imports `kernel`, `domain`, `rules` and `infra`, never `service/` or a surface.
- `engine` (`tools/archlint/engine.go`): `command/` and `query/` never import `dispatch` or `surface/`; `surface/` never imports `command/` or `query/`; `command/` never calls `context.Background()` or `context.TODO()`.

## Testing

`internal/testutil/dispatchtest` runs a command through `dispatch` with no surface: `dispatchtest.New(t, BenchParams{})` is a dispatcher on doubles (a recording `Bus`, a fixed `Clock`, operations numbered `op-1`, `op-2`…, a temporary state dir), and `dispatchtest.Run(ctx, RunParams{Bench, Command, Request, Options})` returns the `Result`, the error and the `Progress` collected. A command's own ports are doubled beside the package that declares them. It replaces `flowtest` as the commands migrate.

The tests of `internal/dispatch` follow the kernel's layout, with testify: a test file per source file holding only `Test…` functions; `fixture_test.go`, an example shaped like `create` over a fake repository; `helpers_test.go`, the helpers and `TestHelperProcess`, which holds locks from another process for the all-or-nothing test.
