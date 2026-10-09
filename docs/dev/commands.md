# Commands — the engine's contract

Every command of wtm becomes a typed payload that is validated, previewed, then applied, so the CLI, an agent, a future API and a graphical interface drive wtm through the same engine. This page describes the contract that engine is built on: `internal/kernel/`, the pure part every command, `dispatch` and every surface share. It imports only the stdlib (enforced by `tools/archlint`): it carries no business vocabulary, and a command composes it with `internal/domain`, never the reverse.

No command is on the contract yet: `internal/flow/` ([flow-layer.md](flow-layer.md)) still runs every command. `create` is the first to move.

## Vocabulary

| Term | Meaning |
| -- | -- |
| `Command[Req, F, P, D]` | a mutation: `Req` the payload, `F` the facts `Observe` reads, `P` the plan, `D` the detail of each item |
| `Query[Req, V]` | a read: `Run` returns a view `V`, `Stream` emits until `ctx` ends |
| Request | the typed payload; its JSON names are the paths of its fields |
| Facts | the snapshot of the repository a command reads in `Observe`, independent of the request |
| `FieldDef` / `FieldSpec` / `FieldState` / `Choices` | a field's behaviour, its schema, its state in a form, its options |
| `Plan[P]` | what `Apply` will do: the dry run |
| `Outcome[D]` | what `Apply` did: one `Item[D]` per subject, plus `FollowUps` and `Warnings` |
| `FollowUp` | a next command proposed with its request filled in |
| `Progress` | what a command emits while it applies, as data |
| `Unit` | the work done for one item: `Before`, a `Saga`, then `Then` |

## The command

```go
type Command[Req, F, P, D any] struct {
	Name    string
	Observe func(context.Context, ObserveScope) (F, error)
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Locks   func(Req) []LockKey
	Plan    func(context.Context, Req, F) (Plan[P], error)
	Apply   func(context.Context, ApplyInput[Req, F, P]) (Outcome[D], error)
}
```

The only I/O of a command is in `Observe`, `FieldDef.Choices` and `Apply`; each takes a `ctx`. `ObserveScope` is data only (the directory observed from): a command gets its ports (git, the state files, processes) when it is built, never through a kernel type. `ApplyInput` hands `Apply` the request, the facts, the plan, the `Emitter` for progress and the `Shield` its sagas run under. Everything else is a pure function of `(Req, F)`. `Locks` names the operation locks the command needs: the repository (`LockRepo`) or one worktree by its branch (`LockWorktree`), shared or exclusive.

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

- **`FieldSpec`** is the schema, as data: `Path`, `Type`, `Label`, `Title`, `Description`, `Required`, `DependsOn`, the `Choices` mode, `Rememberable` (the "always use this answer" box) and `Constraints` (`Enum`, `MinLen`/`MaxLen`, `MinItems`/`MaxItems`, `Pattern`). A form, the JSON schema and the generated docs read it; `CheckSpec` checks it. Constraints are data rather than struct tags because a form must read them, and because a rule that reads the facts cannot be a tag anyway.
- **`Type`** is the shape of the value, never its subject: `text`, `bool`, `select`, `multiselect`, `reorder`, `textlist`, `decisions` (a choice per key, as for drifting `.env` keys). A branch picker is a `select` whose `Choices` mode is `search`: a long list the user filters and can refresh, with one option pinned.
- **`Value`** carries any field's value: `Text` (text, select, a named string type), `Bool`, `List`, `Decisions`. `kernel.Get` and `kernel.Set` read and write a request by path, through nested structs (`target.from`); `Set` returns a copy.
- **`Default`** returns a `Fallback`: the value and its `Origin` (`remembered`, `config`, `default`). A value the request carries has origin `request`.

`kernel.Evaluate` computes a form: field by field in declaration order, a field is `skipped` (with its reason), `provided`, `defaulted` (the default written into the request) or `missing`; then each field not skipped is checked against its `Constraints`, then its `Validate`, then the `Rules` run over the whole request. `Complete` is true when nothing is wrong. A `Skip` sees the defaults of the fields declared before it.

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
| `OneOf(paths...)` | none set, or more than one | `required` with `With`, or `exclusive_with` |
| `Requires(RequiresRule{Field, Needs})` | `Field` set without all of `Needs` | `requires`, `With` = the missing ones |
| `RequiredWhen(path, Condition)` | `path` empty while the condition holds | `required`, `Params[because]` = the condition's code |
| `Distinct(path)` | an entry given twice in a list | `distinct` on `path[i]` |
| `NotSelfParent(SelfParentRule{Parent, Children})` | a parent that is one of its children | `self_parent` |

A `Condition` is a pure predicate over `(Req, F)` with a code saying why it holds. `Rule.Paths` keeps a rule readable as data: `kernel.Disabled` tells a form whether picking an option would break a rule that holds today, and which, so the option arrives disabled with its reason. `With` is the only way to add a hand-written `Rule`.

## Errors

One shape for every failure, never a sentence.

```go
type Error struct {
	Kind     Kind              // invalid · not_found · conflict · refused · precondition · cancelled · internal
	Code     Code              // stable: "request.invalid", "unit.undo_failed"
	Params   map[string]string // what the message needs
	Fields   []FieldError      // invalid: field by field
	Blockers []Blocker         // refused: each refusal and the field that lifts it
	FollowUp *FollowUp
	Cause    error
}

type FieldError struct {
	Path     string // "from", "branches[2]", "decisions[PORT]"
	Code     Code   // required · invalid · not_found · one_of · exclusive_with · requires · distinct · self_parent · too_short · too_long · too_few · too_many · pattern
	Params   map[string]string
	Accepted []string
	With     []string
}
```

`kernel.Invalid(fields)` is the `422`. `kernel.Classify` keeps an `*Error` as it is, reads a cancellation as `cancelled` and files anything else as `internal` under the code it is given. `Error.Error()` returns the code (and the cause): the text is not the engine's.

## Messages

`Error`, `FieldError`, `Warning`, `Badge` and `Progress` carry a `Code` and `Params`. `internal/kernel/text` is the one catalogue that turns them into English: `text.Message(code, params)` fills `{param}` placeholders, `text.Field(fieldError)` adds the path, the accepted values and the other fields named. The CLI, the `message` of the JSON and a graphical interface all read it, and an interface may present a code differently (a link, a button).

A code is a constant of type `kernel.Code`. `TestEveryCodeDeclaredHasItsMessage` scans every package under `internal/` for those constants and fails on one missing from the catalogue, and on a catalogue entry no constant declares. The catalogue holds the kernel's own codes; where a command's messages live (in `kernel/text`, keyed by the code's string, or beside the command) is settled by the first command on the contract, `create`, and the `…Fmt` templates of `domain/` follow their command there.

## Results

```go
type Item[T any] struct {
	Subject string
	Status  Status // done · failed · skipped · cancelled
	Reason  Reason // interrupted · not_reached · unchanged · unsafe · locked
	Error   *Error
	Detail  T
}

type Outcome[T any] struct {
	Items     []Item[T]
	FollowUps []FollowUp
	Warnings  []Warning
}
```

A `FollowUp` names a command, its request (`json.RawMessage`) and why. Every type carries its JSON names: they are the contract agents read.

## Units of work

The disk has no transactions: a unit is a saga, a sequence of steps that each know how to undo themselves.

```go
type Unit[D any] struct {
	Before []Prep     // interruptible, nothing changed yet
	Saga   Saga[D]    // all or nothing, shielded from the interruption
	Then   []Phase[D] // after the commit: interruptible, never undone
}

type Saga[D any] struct {
	Steps      []SagaStep // {Name, Do, Undo}
	Commit     func(context.Context) (D, error)
	LeftBehind func(step string) *FollowUp
}

items := kernel.Each(ctx, kernel.EachParams[string, Detail]{
	Items: branches, Subject: ..., Unit: ..., Emit: emit, Shield: shield,
})
```

`kernel.Each` runs one unit per item and returns one `Item` per item:

- `ctx` is checked before each unit: the units never reached are `skipped / interrupted`. With `StopOnFailure`, those after a failure are `skipped / not_reached`.
- `Before` (stopping jobs, `on_clean` hooks) checks `ctx` before each prep; an interruption there leaves the item `cancelled` with nothing changed.
- The saga runs under the `Shield` it is handed (infra's, so a second Ctrl+C waits for it) and goes to the end. A step that fails has the steps done undone in reverse, shielded too; the item is `failed` and nothing is left. A step with no `Undo` relies on the one before it.
- An `Undo` that fails stops the walk back: the item is `failed` with `unit.undo_failed`, `Params[left]` naming what is still standing, and the `FollowUp` of `LeftBehind` (`clean --force` for a worktree).
- `Commit` runs last, once every step is done; a commit that fails undoes them all. Nothing announces what could still be undone.
- `Then` checks `ctx` before each phase: an interruption stops the next ones and the item is `cancelled`, its `Detail` keeping what was done. A phase that fails stops the next ones too, the item `failed`.
- A failure the interruption caused counts as `cancelled`.

Each unit emits `unit.started`, `phase.started` / `phase.finished` per prep, step and phase, then `unit.finished` with its status.

`kerneltest.CheckSaga` fails each step of a saga in turn, then its commit, and wants the state back exactly as it was each time: every command runs it on its sagas.

## Testing a command

`internal/kernel/kerneltest` holds the checks every command runs on itself:

- `CheckDependsOn(t, DependsOnParams{Fields, Request, Facts})` empties then changes, one at a time, every field a `FieldDef` does not declare in `DependsOn`, and fails when `Skip`, `Default` or `Validate` notices. It also fails on a path that names no field.
- `CheckSaga(t, SagaParams{Saga, Snapshot})`, above.

The tests of `internal/kernel` are its specification, readable alone: one test file per source file (`path_test.go`, `spec_test.go`, `rules_test.go`, `form_test.go`, `errors_test.go`, `result_test.go`, `unit_test.go`, whose interruption tests carry the cancellation rules of every mutation), and `kernel_test.go`, which walks one example `create` from an empty form to its outcome and runs both checks on it.
