# Commands — the engine's contract

Every command of wtm becomes a typed payload that is validated, previewed, then applied, so the CLI, an agent, a future API and a graphical interface drive wtm through the same engine. This page describes the contract that engine is built on: `internal/kernel/`, the pure part every command, `dispatch` and every surface share. It imports only the stdlib (enforced by `tools/archlint`): it carries no business vocabulary, and a command composes it with `internal/domain`, never the reverse.

No command is on the contract yet: `internal/flow/` ([flow-layer.md](flow-layer.md)) still runs every command. `create` is the first to move. A command on the contract runs through `internal/dispatch/` ([dispatch.md](dispatch.md)), which takes its locks, observes, validates, plans and applies it.

## Vocabulary

| Term | Meaning |
| -- | -- |
| `Command[Req, F, P, D]` | a mutation: `Req` the payload, `F` the facts `Observe` reads, `P` the plan, `D` the detail of each item |
| `Query[Req, V]` | a read: `Run` returns a view `V`, `Stream` emits until `ctx` ends |
| Request | the typed payload; its JSON names are the paths of its fields |
| Facts | the snapshot of the repository a command reads in `Observe`, independent of the request |
| `FieldDef` / `FieldSpec` / `FieldState` / `Choices` | a field's behaviour, its schema, its state in a form, its options |
| `Plan[P]` | what `Apply` will do: the dry run; `P` is a struct, shown as its fields |
| `Outcome[D]` | what `Apply` did: one `Item[D]` per subject, plus `FollowUps` and `Warnings`; `Failure()` is the error the run ends on |
| `FollowUp` | a next command proposed with its request filled in |
| `Progress` | what a command emits while it applies, as data: a sealed union, one type per kind |
| `Unit` | the work done for one item: `Before`, a `Saga`, then `Then` |
| `Milestones[D]` | where each unit stands as it gets there, typed by its detail: saga committed, phase done, unit over |

## The command

```go
type Command[Req, F, P, D any] struct {
	Name    string
	Observe func(context.Context) (F, error)
	Forget  func(F) F
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Locks   func(Req) []LockKey
	Plan    func(context.Context, Req, F) (Plan[P], error)
	Apply   func(context.Context, ApplyInput[Req, F, P, D]) (Outcome[D], error)
}
```

The only I/O of a command is in `Observe`, `FieldDef.Choices` and `Apply`; each takes a `ctx`. A command gets its ports (git, the state files, processes), and the directory they work in, when it is built, never through a kernel type: there is one source for the directory. `ApplyInput` hands `Apply` the request, the facts, the plan, the `Emitter` for progress, the `Shield` its sagas run under and the `Milestones` dispatch turns into events, the last two to pass on to `kernel.Each`. Everything else is a pure function of `(Req, F)`. `Locks` names the operation locks the command needs: the repository (`LockRepo`) or one worktree by its branch (`LockWorktree`), shared or exclusive. `Forget` returns the facts without the answers the user asked to remember: a run that asks again (`--ask`) falls back on each field's other default.

## Fields

A field is declared once, with its behaviour; only its results circulate.

```go
type FieldDef[Req, F any] struct {
	Spec     FieldSpec
	Skip     func(Req, F) (skip bool, reason Code)
	Default  func(Req, F) (Fallback, bool)
	Validate func(Req, F) []FieldError
	Choices  func(context.Context, Req, F) (Choices, error)
}
```

- **`FieldSpec`** is the schema, as data: `Path`, `Type`, `Label`, `Title`, `Description`, `Required`, `DependsOn`, the `Choices` mode, `Rememberable` (the "always use this answer" box) and `Constraints` (`Enum`, `MinLen`/`MaxLen`, `MinItems`/`MaxItems`, `Pattern`). A form, the JSON schema, the generated command line (a flag per field, its help the `Description`: [cli.md](cli.md)) and the docs read it; `CheckSpec` checks it. Constraints are data rather than struct tags because a form must read them, and because a rule that reads the facts cannot be a tag anyway.
- **`Type`** is the shape of the value, never its subject: `text`, `bool`, `select`, `multiselect`, `reorder`, `textlist`, `decisions` (a choice per key, as for drifting `.env` keys). A branch picker is a `select` whose `Choices` mode is `search`: a long list the user filters and can refresh, with one option pinned.
- **`Value`** is a sealed union: `kernel.Text` (a string field, named string types included), `kernel.Bool`, `kernel.List` (a `[]string`), `kernel.Decisions` (a `map[string]string`); `nil` is no value. The Go type of the request field decides which: `kernel.Get` reads it, `kernel.Set` returns a copy with it written (`nil` empties the field) and refuses another shape, and `CheckSpec` refuses a `FieldType` that does not fit the field (a `textlist` on a `string`) or a `Pattern` that does not compile: an error of the command, never a field error. A path is the field's JSON name, dotted through nested structs (`target.from`); an entry of a list is `path[index]` (`kernel.IndexPath`), one of decisions `path[key]`. The same string names the field in a flag error, the schema and the form. `kernel.Within` says whether a path is a field, one of its entries (`branches[2]`, `decisions[PORT]`) or a field nested in it.
- **`Default`** returns a `Fallback`: the value and its `Origin` (`remembered`, `config`, `default`). A value the request carries has origin `request`.
- **`FieldState`** is a field in a form: its `Status`, `Origin`, `Value`, `SkipReason`, `Errors`, and `Remembered`, the answer the repository remembers for it when the form was evaluated without it (`--ask`). It reads back from its JSON: each `Value` is decoded by its shape (a string is `Text`, a bool `Bool`, an array `List`, an object `Decisions`), so a surface that gets a form as JSON holds the same states.
- **`remember`** is the field a command declares to keep answers: `kernel.Remember(RememberParams{Fields, Label, Description})` builds it at `kernel.RememberPath`, a `multiselect` whose `Enum` is the paths of the `Rememberable` fields. The form fills it from its "always use this answer" boxes, the command line takes it as `--remember <path>`, and the command's `Apply` keeps the answers it names.

`kernel.Evaluate` computes a form: field by field in declaration order, a field is `skipped` (with its reason), `provided`, `defaulted` (the default written into the request) or `missing`; then each field not skipped is checked against its `Constraints`, then its `Validate`, then the `Rules` run over the whole request. `Complete` is true when nothing is wrong. A `Skip` sees the defaults of the fields declared before it. Handed the command's `Forget`, it evaluates on the facts `Forget` returns and sets each field's `Remembered` from the facts as given: one evaluation says both what the form falls back on and what the repository remembers.

`DependsOn` lists what `Skip`, `Default` and `Validate` read, so a client calls the engine again only when a field something depends on changes. `kerneltest.CheckDependsOn` holds every command to it.

## Rules across fields

```go
func (Request) Rules() kernel.Rules[Request, Facts] {
	return kernel.Rules[Request, Facts]{}.
		Distinct("branches").
		RequiredWhen("from", anyBranchExists).
		NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"})
}
```

| Combinator | Refuses | Code |
| -- | -- | -- |
| `Exclusive(paths...)` | more than one of them set | `exclusive_with`, `With` = the others set |
| `OneOf(paths...)` | none set, or more than one | `required_one_of` with `With`, or `exclusive_with` |
| `Requires(RequiresRule{Field, Needs})` | `Field` set without all of `Needs` | `requires`, `With` = the missing ones |
| `RequiredWhen(path, Condition)` | `path` empty while the condition holds | `required`, `Params[because]` = the condition's code |
| `Distinct(path)` | an entry given twice in a list | `distinct` on `path[i]` |
| `NotSelfParent(SelfParentRule{Parent, Children})` | a parent that is one of its children | `self_parent` |

Each combinator receives the values at its paths already read, so it holds only its rule. A rule over a field whose value is not the shape it reads (`Distinct` on a text field) is an error of the command, never a rule that finds nothing. A `Condition` is a pure predicate over `(Req, F)` with a code saying why it holds. `Rule.Paths` keeps a rule readable as data: `kernel.Disabled` tells a form whether picking an option would break a rule that holds today, and which, so the option arrives disabled with its reason. `With` is the only way to add a hand-written `Rule`.

## Errors

One shape for every failure, never a sentence. `Error` is a sealed union: the kind is the Go type, so an invalid request always names its fields and a refusal always its blockers.

```go
type Problem struct { // what every error carries
	Code     Code   // stable: "request.invalid", "lock.held", "unit.undo_failed"
	Params   Params // what the message needs
	FollowUp *FollowUp
	Cause    error
}

type Error interface { error; Kind() Kind; Base() *Problem; sealed() }

type InvalidError struct { Problem; Fields []FieldError } // 422, built by kernel.Invalid(fields)
type RefusedError struct { Problem; Blockers []Blocker }  // 403: each refusal and the field that lifts it
type Failure struct { Problem; kind Kind }                 // kernel.NotFound · Conflict · Precondition · Cancelled · Internal

type FieldError struct {
	Path     string // "from", "branches[2]", "decisions[PORT]"
	Code     Code   // required · required_one_of · invalid · not_found · one_of · exclusive_with · requires · distinct · self_parent · too_short · too_long · too_few · too_many · pattern · unknown_field
	Params   Params
	Accepted []string
	With     []string
}
```

`errors.As(err, &invalid)` with an `*kernel.InvalidError` finds that kind, `errors.As(err, &e)` with a `kernel.Error` finds any. A type switch over `kernel.Error` or `kernel.Value` must handle every variant: `go-sumtype` checks it in `make lint`. Each error marshals with its `"kind"`. `kernel.Classify` keeps an `Error` as it is, reads a cancellation as `cancelled` and files anything else as `internal` under the code it is given. `Error()` returns the code (and the cause): the text is not the engine's.

## Messages

The engine writes no text at run time. An error, a field error, a refusal, a warning, a badge, a progress status, and the status and reason of an item are a `Code` and its `Params`; the English is a catalogue's. What a command declares about its fields is not run-time text: `FieldSpec.Label`, `Title` and `Description`, and those `kernel.Remember` builds, are data of the command's schema, written beside it and read as they are by a form, the flags, the JSON schema and the docs.

`internal/kernel/text` turns a code into English. `text.Messages` maps the codes one package declares to their templates, each `{param}` filled from `Params`. `text.NewCatalog(sets...)` is the kernel's messages — its own codes, those of a run (`lock.held`, `locks.unavailable`, `plan.stale`, `state.schema_newer`, raised by `dispatch`: [dispatch.md](dispatch.md#errors)) and the label of each `Status` and `Reason` (`status.done`, `reason.not_reached`…) — plus each set it is handed; a code two sets spell panics. There is no global catalogue: `dispatch.Registry.Catalog()` builds the one of the entries a surface serves, and the surface hands it on, to `render` (`render.Wording`) and to the form (`form.Params.Catalog`). `Catalog.Message(code, params)` fills a template (a code missing from the catalogue reads as itself, which `catalog` below refuses), `Catalog.Field(fieldError)` adds the path, the accepted values and the other fields named, `Catalog.Status` and `Catalog.Reason` label an item. The CLI, the `message` of the JSON and a graphical interface all read it, and an interface may present a code differently (a link, a button).

### A command's codes

A command declares its codes, their messages and the exit codes of its own beside it:

```go
// command/create
const CodeBranchTaken kernel.Code = "create.branch_taken"

var Messages = text.Messages{
	CodeBranchTaken: "{branch} is taken",
}

var ExitCodes = map[kernel.Code]int{
	CodeBranchTaken: domain.ExitCodeWorktreeExists,
}
```

`internal/registry` hands both to `dispatch.Register` (`RegisterParams.Messages` and `ExitCodes`, `RegisterQueryParams` for a query), and the entry exposes them (`Entry.Messages()`, `Entry.ExitCodes()`). `ExitCodes` keeps the number the legacy command exited with; a code it does not list exits on its kind ([cli.md](cli.md#exit-codes)). The `…Fmt` templates of `domain/` move into a command's `Messages` as it migrates.

A command turns each sentinel it expects into a typed error with its code: the `domain.ErrBranchNotFound` a port returns becomes `kernel.NotFound(kernel.Problem{Code: CodeBranchNotFound, Params: …, Cause: err})`, never an `internal` that happens to wrap it. `internal` is for the unexpected: it reads as "internal error" with its cause, and exits on the code of the sentinel it wraps (`rules.ExitCode`), as a legacy command does.

`make lint` holds it (`catalog`, `tools/archlint/catalog.go`): every `kernel.Code` declared in `kernel/`, `dispatch/`, `command/` or `query/` — a typed constant, or a conversion of a constant, `kernel.Code("…")` — has its key in a `text.Messages` of its own package, the kernel's in `kernel/text`'s. A key written as a bare string is refused: a message speaks a declared code.

## Results

```go
type Item[T any] struct {
	Subject  string
	Status   Status // done · failed · skipped · cancelled
	Reason   Reason // interrupted · not_reached · unchanged · unsafe · locked · incomplete
	Error    Error
	Warnings []Warning // about this subject: a .env key the copy skipped
	Detail   T
}

type Outcome[T any] struct {
	Items     []Item[T]
	FollowUps []FollowUp
	Warnings  []Warning // about the run: the plan's, then Apply's
}
```

An item is built through `kernel.ItemFor[D](subject)`, whose `Done`, `Unchanged`, `Skipped(reason)`, `Failed(err, detail)`, `Cancelled(err, detail)` and `Incomplete(err, detail)` keep status, reason and error consistent; `Warned(warnings...)` adds the warnings about the subject first. `incomplete` is an item whose saga committed and a phase after it did not finish: what the saga did stands, `failed` or `cancelled` by its error's kind. `kernel.Retype` is the same item over another detail (its JSON, for a surface); nothing else builds an item. A `FollowUp` names a command, its request (`json.RawMessage`, an object) and why. Every type carries its JSON names: they are the contract agents read.

`Outcome.Failure()` is the error a run ends on, one rule for every reader: an interrupted item first (`cancelled`, or `skipped / interrupted`, its error or `unit.interrupted`), else the first item that failed, else none. `dispatch` publishes it on `operation.finished`, the command line exits on it.

## Units of work

The disk has no transactions: a unit is a saga, a sequence of steps that each know how to undo themselves.

```go
type Unit[D any] struct {
	Before []Prep     // {Name, Log, Run(ctx, Reporter)}: interruptible, nothing changed yet
	Saga   Saga[D]    // all or nothing, shielded from the interruption
	Then   []Phase[D] // {Name, Log, Run(ctx, Reporter, D) (D, error)}: after the commit, interruptible, never undone
}

type Saga[D any] struct {
	Steps      []SagaStep // {Name, Do(ctx, Reporter), Undo(ctx)}
	Commit     func(context.Context) (D, error)
	LeftBehind func(step string) *FollowUp
}

items := kernel.Each(ctx, kernel.EachParams[string, Detail]{
	Items: branches, Subject: ..., Unit: ..., Emit: in.Emit, Shield: in.Shield, Milestones: in.Milestones,
})
```

`kernel.Each` runs one unit per item and returns one `Item` per item:

- `ctx` is checked before each unit: the units never reached are `skipped / interrupted`. With `StopOnFailure`, those after a failure are `skipped / not_reached`.
- `Before` (stopping jobs, `on_clean` hooks) checks `ctx` before each prep; an interruption there leaves the item `cancelled` with nothing changed.
- The saga runs under the `Shield` it is handed (`proc.Shield`, so a second Ctrl+C waits for it) and goes to the end. A step that fails has the steps done undone in reverse, shielded too; the item is `failed` and nothing is left. A step with no `Undo` relies on the one before it.
- An `Undo` that fails stops the walk back: the item is `failed` with `unit.undo_failed`, `Params[left]` naming what is still standing, and the `FollowUp` of `LeftBehind` (`clean --force` for a worktree).
- `Commit` runs last, once every step is done; a commit that fails undoes them all. Nothing announces what could still be undone.
- `Then` checks `ctx` before each phase: an interruption stops the next ones and the item is `cancelled / incomplete`, its `Detail` keeping what was done. A phase that fails stops the next ones too, the item `failed / incomplete`.
- A failure the interruption caused counts as `cancelled`.

Every prep, step and phase is handed the `Reporter` of its item (`kernel.Report(emit, subject)`, which drops the progress when `emit` is nil): `Output` (a raw line of a hook), `Status` (a code and its params), `Phase`, and `Warn`, which keeps a warning the item carries once the unit is over. A prep or a phase that names a `Log` has its output kept in that file by `dispatch`, whatever a surface shows of it (the command computes the path: `rules.HooksLogPath` for a hook phase).

`Progress` is a sealed union (`//go-sumtype:decl`), each kind its own type, all embedding `Of{Op, Subject}`: `*UnitStarted`, `*PhaseStarted{Phase, Log}` and `*PhaseFinished{Phase}` per prep, step and phase, `*Output{Line}`, `*StatusChanged{Code, Params}`, then `*UnitFinished{Status}`. A surface reads it with a type switch, which `make lint` holds to every kind.

`Milestones[D]` are told where each unit stands, typed by its detail: `Committed{Subject, Detail}` once its saga committed, before the first phase; `PhaseDone{Subject, Phase, Detail}` after each phase of `Then` that finished; `Finished(Item)` once the unit is over, before its `unit.finished`. A unit never reached has none; a saga that failed has no `Committed`. `kernel` gives them no meaning: the command declares, over these types, the pure functions that turn each into its `domain.Event`s, and the registry hands them to `dispatch` as its `Events` ([dispatch.md](dispatch.md#progress-hook-logs-and-events)).

`kerneltest.CheckSaga` fails each step of a saga in turn, then its commit, and wants the state back exactly as it was each time: every command runs it on its sagas.

## Testing a command

`internal/kernel/kerneltest` holds the checks every command runs on itself:

- `CheckDependsOn(t, DependsOnParams{Fields, Request, Facts})` empties then changes, one at a time, every field a `FieldDef` does not declare in `DependsOn`, and fails when `Skip`, `Default` or `Validate` notices. It also fails on a path that names no field.
- `CheckSaga(t, SagaParams{Saga, Snapshot})`, above.

The tests of `internal/kernel` are its specification, readable alone, written with testify (`assert`, `require`):

- one test file per subject, holding only `Test…` functions: the source file it tests (`path_test.go`, `value_test.go`, `spec_test.go`, `rules_test.go`, `form_test.go`, `errors_test.go`, `result_test.go`, `progress_test.go`, `milestones_test.go`, `each_test.go`), or a subject of `Each` (`saga_test.go`, `failure_test.go`, `interrupt_test.go`, whose tests carry the cancellation rules of every mutation);
- `kernel_test.go`, which walks the example `create` from an empty form to its outcome, a subtest per step, and runs both checks on it;
- `fixture_test.go`, the example: its request, its facts, fakes of what it touches (`disk`, `recorder`) and the `create` command itself; `helpers_test.go`, the helpers the tests share.
