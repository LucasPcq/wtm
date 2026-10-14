# Dispatch — how a command runs

`internal/dispatch/` is the impure half of the command engine, beside the pure contract of [commands.md](commands.md): the one way a surface reaches a command or a query. It takes the operation locks, observes, fills in the defaults, validates, plans, checks the fingerprint, applies and publishes what happened, in the same order for every command, so the locks, the fingerprint and the events apply whatever the surface (the CLI, the form, later an API).

Its surfaces are the generated command line ([cli.md](cli.md)) and the interactive form ([form.md](form.md)). No command runs through it yet: the registry holds the `operations` query, and `create` is the first command (step 3 of LUC-283).

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
	Messages:   create.Messages,  // its codes in English (commands.md#messages)
	ExitCodes:  create.ExitCodes, // the exit codes of its own
	Events:     create.Events(),  // the events of its milestones
})
listed := dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{
	Query: operations.Query(d.Operations),
})
```

`Register` panics on a command whose plan detail `P` is not a struct: every surface shows a plan as its fields ([output.md](output.md#a-command-on-the-engine)). `Events` is how the command says what its units changed ([below](#progress-hook-logs-and-events)); a command with none publishes only the operation's events.

`internal/registry` is the composition root: `registry.Production(d)` is the registry the surfaces serve, built on that dispatcher, every command with its real ports on its `Dir` and `StateDir` ([Ports](#ports)), every query. It is the only package that imports `command/` and `query/`; only the production engine of `surface/cli` imports it (and `tools/gendocs` through `cli.Root()`). `dispatch` imports neither, nor `store/`. A command enters the product by its line there. The command line reads it once on a dispatcher with no `Params` for the names, the fields and the catalogue (that dispatcher is never run), then on the dispatcher of the repository a command runs in.

`Now`, `NewOpID` and `Shield` default to the clock, a random id and `proc.Shield`; the test bench replaces them.

- **`Handle[Req, F, P, D]`** is a command bound to its dispatcher: typed, for a surface written for that command.
- **`Registry`** is the list of what a surface can run, an `Entry` per command or query, found by `Lookup(name)`. An `Entry` erases the types: `Call` reads the request as JSON (its names are the paths of the fields) and answers a sealed `Reply`, a `CommandReply` (the `Result` with its plan and item details as JSON, statuses typed) or a `QueryReply` (the view as JSON). A field the request does not have is a `422` naming it (`unknown_field`), never ignored. `Entry.Fillable()` is the entry a form fills: a command's, never a query's. `Entry.Messages()` and `Entry.ExitCodes()` are what it was registered with, and `Registry.Catalog()` is the catalogue of the kernel and of every entry, which panics on a code two entries spell ([commands.md](commands.md#messages)).

## The form

| Call | Returns | Cost |
| -- | -- | -- |
| `Schema()` | the `FieldSpec` of every field | none: read it once |
| `Observe(ctx)` | `Snapshot{Facts, Fingerprint}` | the command's I/O; observe again to refresh |
| `State(FormParams{Request, Facts, IgnoreRemembered})` | `kernel.Form`: each field's state, the defaults filled in, `Complete`; with `IgnoreRemembered`, evaluated on the facts `Forget` returns, each field carrying what they remembered (`FieldState.Remembered`) | pure, `kernel.Evaluate` |
| `Choices(ctx, ChoicesParams{Path, Request, Facts})` | the field's options, each that would break a rule carrying its `Refusal` (`kernel.Disabled`) | the field's own I/O |

A form keeps the snapshot while the user types: entering a value calls `State` again, never `Observe`. The fingerprint of the snapshot shown in the recap is the one handed to `Run`.

A surface that serves every command reaches the same calls through a `Fillable`, with no type of the command's: the request is JSON, decoded as `Call` decodes it (a field the request does not have is a `422`), and the facts are an opaque `Observed` the entry hands back to itself.

| `Fillable` | The `Handle` call |
| -- | -- |
| `Observe(ctx)` → `Observed{Fingerprint}` | `Observe` |
| `State(FillParams{Request, Facts, IgnoreRemembered})` → `kernel.Form[json.RawMessage]` | `State` |
| `Choices(ctx, FillChoicesParams{Path, Request, Facts, IgnoreRemembered})` | `Choices`, on the facts `Forget` returns when `IgnoreRemembered` is set |
| `Set(SetParams{Request, Path, Value})` → the request with the value at its path | `kernel.Set` on the typed request: a value of the wrong shape is a `422` |

Facts another entry observed are an internal error. The choices of a path that names no field are a `404` with `unknown_field`. An option is judged as the value it would set (`kernel.Disabled`): the list once it is added for a list field, the bool it spells for a `bool` field; an option of a `decisions` field is a choice for one key among several and is not judged alone.

## The run

`Handle.Run(ctx, RunParams{Request, Options, Progress})` goes through these steps, always in this order:

1. **Locks** of the request as received. The holder file, then the command's locks (`Command.Locks(req)`), all or none and without waiting. A key another operation holds is a `409 lock.held`. A dry run takes none, holder included: the fingerprint guards the run that applies its plan.
2. **`Observe`**, under the locks: no other wtm process changes the facts between here and the end of `Apply`. The fingerprint is computed now.
3. **Defaults and validation**, `kernel.Evaluate`, on the facts `Forget` returns when `IgnoreRemembered` is set (`--ask`), so each field falls back on its other default: an incomplete request is a `422` naming its fields.
4. **Locks of the complete request.** The keys `Command.Locks` names once the defaults are filled in (a branch a default chose) and the operation does not hold yet, all or none and without waiting (`409` otherwise); the holder file is rewritten to list them. When any were taken, back to step 2: the facts read before may be stale.
5. **`Plan`.** A dry run stops here and returns the plan and the fingerprint: nothing applied, nothing published.
6. **The fingerprint check.** `ExpectFingerprint` different from the facts just read is a `412 plan.stale`.
7. **`Apply`**, handed the request with its defaults, the facts, the plan, the progress `Emitter`, the `Shield` and the `Milestones`. The plan's warnings are carried into the outcome's, first and once each.
8. **Release** of the locks, then the `Result`.

| `Options` | |
| -- | -- |
| `DryRun` | stop after `Plan` |
| `ExpectFingerprint` | the fingerprint a preview returned (`--expect-fingerprint`, a form's recap) |
| `IgnoreRemembered` | drop the remembered answers before the defaults (`--ask`) |
| `Surface` (`dispatch.SurfaceCLI`, `SurfaceTUI`…), `CorrelationID` | recorded in the holder and on the events |

`Result{Op, Fingerprint, Plan, Outcome}` carries the plan always and the outcome unless the run was a dry run. A run that could not take its locks (no `flock`, an unwritable state dir) goes on unprotected and says so with the warning `locks.unavailable`, in the outcome and in the plan.

## Errors

Every error `Run` returns is a `kernel.Error`. Its codes and their params are the engine's, declared in `kernel` beside the others (every surface handles them), with their messages in `kernel/text`.

| Code | Kind | As HTTP | Params |
| -- | -- | -- | -- |
| `request.invalid` | `invalid` | 422 | `fields`: each `FieldError`, `unknown_field` included |
| `lock.held` | `conflict` | 409 | `key` (`repo`, `wt:<branch>`), and the holder: `op`, `command`, `pid`, `surface`, `started_at` |
| `plan.stale` | `precondition` | 412 | `expected`, `found` |
| `state.schema_newer` | `precondition` | 412 | `path`, `found`, `supported` |

`state.schema_newer` is the `*statefile.NewerError` a write returns (exit code `22`), found by `errors.As` wherever it was wrapped. Anything else goes through `kernel.Classify`: a cancellation is `cancelled`, the rest `internal`, an error of the command among them: facts that do not encode as JSON (no fingerprint could guard a run on them), a worktree lock with no name.

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
- **A worktree key names its worktree.** `Command.Locks` leaves out a branch it does not know yet (an empty field before its default); a `LockWorktree` key with no name is an internal error.
- **From before `Observe` to the end of `Apply`**, grown once the defaults name more keys. No nested run: `command/` never imports `dispatch`, a command made of others declares the union of their keys, and a `FollowUp` is a new call.
- **Without `flock`** (NFS, some WSL mounts) the run is unprotected and warns `locks.unavailable`.
- **A query locks nothing**: the atomic writes of `infra/fs` give it a file before or after a write, never half of one.

`query/operations` lists the operations in progress from the holder files still held: `operations.Query(d.Operations)`. A holder file is an `operation.Operation` (`internal/domain/operation`), which `dispatch` writes and the query lists.

## The fingerprint

The fingerprint is the SHA-256 of the facts as JSON (maps in key order), cut to 24 hex characters: one notion of "what was read", the same in the preview and in the run. Only what the facts export to JSON counts: a fact a command checks belongs in its JSON.

## Progress, hook logs and events

`Apply` is handed an `Emitter` that, for each `kernel.Progress`:

- stamps it with the operation's `op_id`;
- writes the output of a phase that names its `Log` (a hook phase: the command computes `rules.HooksLogPath`) to that file, opened by `fs.OpenLog` when the phase starts and closed when it ends, whatever the surface shows of it; `dispatch` knows no hook;
- publishes `unit.finished` when a unit ends;
- forwards it to the surface's `Progress`.

`Apply` is also handed the `Milestones` of its units. `dispatch` passes each, with the facts the run observed, to the command's `Events`, pure functions declared at registration:

```go
type Events[F, D any] struct {
	Committed func(EventOf[F, kernel.Committed[D]]) []domain.Event // the saga committed, before the first phase
	PhaseDone func(EventOf[F, kernel.PhaseDone[D]]) []domain.Event // a phase after it finished
	Finished  func(EventOf[F, kernel.Item[D]]) []domain.Event      // the unit is over
}
```

Each returns the `domain.Event`s to publish (none, one or several), built from the facts and the detail without reading the repository again; `dispatch` stamps them (`ts` from its clock, `op_id`, `correlation_id`) and publishes them at once. For `create`: `worktree.created` at the commit, before `on_create`; `worktree.updated` after the phase that allocated the ordinal; `worktree.provisioned` when the unit is over. `kernel` gives a milestone no meaning; the command does.

On the bus (`docs/guide/events.md`) an applied run sends `operation.started`, the events of its commands' milestones, one `unit.finished` per item, then `operation.finished` with `ok` and, on failure, the `code` of `Outcome.Failure()` (an interruption first, else the first item that failed) or of the error `Apply` returned. Each carries `op_id`, `ts` and the `correlation_id`; the publisher keeps them. They are sent even once the run is interrupted. A dry run and a run refused before `Apply` publish nothing. Output lines never go on the bus.

## Ports

`kernel` imports only the stdlib, so a port never travels through its types. A command receives its ports, and the directory they work in, when it is built, `create.Command(ports)`, and captures them in `Observe`, `Choices` and `Apply`. Each port is an interface declared beside what it wraps (`infra/git`, `store/`, `infra/proc`), with its double in `testutil/`; `registry.Production` builds each command with the real ones, on the dispatcher's `Dir` and `StateDir`: git is `git.Reader` and `git.Writer`, built as `git.Repo{Dir: <Dir>, Remote: domain.OriginRemote}` and doubled by `gittest.Fake` ([git.md](git.md)); the state files are `store.Meta`, `store.ProjectConfig` and `store.EnvFiles`, built as `store.MetaDisk{StateDir: <StateDir>}`, `store.ConfigDisk{StateDir: <StateDir>}` and `store.EnvDisk{}` and doubled by `storetest.Fake` ([store.md](store.md)). `dispatch` runs on its own ports only: `Params` above.

## What the lint holds

- `layers`: `dispatch` imports `kernel`, `domain`, `rules` and `infra`, never `command/`, `query/`, `store/`, `service/` or a surface; `registry` imports what it assembles (`command/`, `query/`, `dispatch`, `kernel`, `domain`, `infra`, `store/`), and `surface/cli` imports it.
- `catalog`: every code `dispatch`, `command/` and `query/` declare has its message in the `text.Messages` of its package ([commands.md](commands.md#messages)).
- `engine` (`tools/archlint/engine.go`): `command/` and `query/` never import `dispatch` or `surface/`; `surface/` never imports `command/` or `query/`; `command/` never calls `context.Background()` or `context.TODO()`.
- `publish`: `dispatch.Publisher.Publish` is called from `dispatch/` only, and `command/`, `query/` and the engine's services publish nothing; an event is what a command reports.
- `writesink`: only `Apply` writes. Every other function of a command (`Observe`, `Choices`, `Plan`, `Skip`…) and every query reaches no write sink, and `dispatch` calls no mutator of the engine's services ([lint.md](lint.md)).

## Testing

`internal/testutil/dispatchtest` runs a command through `dispatch` with no surface: `dispatchtest.New(t, BenchParams{})` is a dispatcher on doubles (a recording `Bus`, a fixed `Clock`, operations numbered `op-1`, `op-2`…, a temporary state dir), and `dispatchtest.Run(ctx, RunParams{Bench, Command, Events, Request, Options})` returns the `Result`, the error and the `Progress` collected. A command's own ports are doubled in `testutil/` (git by `gittest.Fake`, the state files by `storetest.Fake`). It replaces `flowtest` as the commands migrate.

The tests of `internal/dispatch` follow the kernel's layout, with testify: a test file per source file holding only `Test…` functions (`fill_test.go` for the `Fillable`); `fixture_test.go`, an example shaped like `create` over a fake repository, with its `Events` (created at the commit, updated once `allocate` gave the ordinal, provisioned at the end) and a command of `bool` and `decisions` fields; `helpers_test.go`, the helpers and `TestHelperProcess`, which holds locks from another process for the all-or-nothing test.
