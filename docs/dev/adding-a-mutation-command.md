# Adding a worktree-mutating command

A *mutation command* changes worktree state: it creates, removes, moves or rewrites something, so it has fields to ask, safety refusals to honor and two bypass axes to expose. A new one is written **on the engine**: a `kernel.Command` in `internal/command/<name>/`, run through `internal/dispatch/`, served by `cli.Bind` and the form. A command already on `internal/flow/` moves there the same way, one at a time, `create` first.

The second half of this page, [A command on `flow/`](#a-command-on-flow), is the recipe of the commands not yet on the engine: every one of them today. It holds for a change to one of them until it moves; never start a new command there.

A read-only command (`list`, `tree`, `resolve`) needs none of this: parse flags, call the service, hand the result to `surface/cli/render/`. It becomes a `kernel.Query` in `internal/query/` when queries get their commands.

## What exists and what the pilot writes

The engine is built, with no command on it yet: `registry.Production` holds the `operations` query alone. Each piece the recipe names is one of these:

| Exists today | Written by the `create` pilot |
| -- | -- |
| `internal/kernel` (`Command`, fields, rules, errors, `Each`), `kernel/text`, `kernel/kerneltest` | `internal/command/create/`, the first package of `internal/command/` |
| `internal/dispatch` (`Register`, `Events`, locks, fingerprint), `testutil/dispatchtest` | its line in `registry.Production` |
| `internal/registry` (`Production`), `query/operations` | `surface/cli/create.go`, its `cli.Bind`, replacing `surface/cli/wt/create.go` |
| `cli.Bind`, `render.Detail`, the v2 envelope, the form (`surface/tui/form`) | `internal/flow/legacy`, the bridge (below) |
| the ports: `infra/git` (`git.Repo`, `gittest.Fake`), `store/` (`MetaDisk`, `ConfigDisk`, `EnvDisk`, `storetest.Fake`) | the `rules/` and `domain/` sub-packages its decisions move to |
| the engine's services: `service/gitwt`, `identity`, `runenv`, `hooks.Execute` | |
| the `archlint` rows of `command/` and `flow/legacy`, armed before those packages exist | |

The examples below are written for `command/create`; the create-shaped fixtures of the engine are the working version of each one:

| Fixture | Shows |
| -- | -- |
| `internal/kernel/fixture_test.go`, walked by `kernel_test.go` | the fields, the rules, a unit and its saga, from an empty form to the outcome, with both `kerneltest` checks |
| `internal/dispatch/fixture_test.go` | the command with its locks, `Forget`, a remembered field, a phase that allocates, a hook phase with its log, and its `Events` |
| `internal/surface/cli/fixture_test.go` | a command served by `cli.Bind` — text, list, select, bool and decisions fields, a nested path — a code of its own, a `render.Detail` (`pathDetail`) |
| `internal/surface/tui/form/fixture_test.go` | the same command asked by the form: a dependent `Skip`, a `config` default, a rule refusing an option, choices that load |

Read [commands.md](commands.md) (the contract) and [dispatch.md](dispatch.md) (how a run goes) first: this page orders the work, it does not repeat them.

## 1. The package

`internal/command/create/`, one file per subject, each named after what it declares: the request and its facts, the fields, the plan, the apply, the events, the codes. Its package doc says its one responsibility in a sentence (`pkgdoc`).

It imports `kernel`, `domain`, `rules`, `config`, `store`, `infra/git`, `infra/fs`, `infra/proc` and the engine's services — never `dispatch`, a surface, `flow/`, the root of `infra/` or a service of the commands on `flow/` (`layers`, `engine`). Another command only along an edge of `commandEdges` (`commanddag`), empty today: a command made of others declares the union of their locks and calls their pieces, never their `Apply` through `dispatch`.

## 2. Request, facts, plan, detail

```go
type Request struct { // its JSON names are the paths of its fields
	Branches  []string         `json:"branches"`
	From      string           `json:"from"`
	Isolation domain.Isolation `json:"isolation"`
	Remember  []string         `json:"remember"`
}

type Facts struct { // what Observe reads; everything that matters is in its JSON
	Worktrees  []git.Worktree   `json:"worktrees"`
	Branches   git.Branches     `json:"branches"`
	Config     domain.Config    `json:"config"`
	Run        domain.RunConfig `json:"run"`
	Remembered map[string]string `json:"remembered"`
}

type Plan struct { // a struct: every surface shows a plan as its fields
	Branches  []string         `json:"branches"`
	From      string           `json:"from"`
	Isolation domain.Isolation `json:"isolation"`
}

type Detail struct { // what a unit did, carried from the commit to the end
	Path    string `json:"path"`
	Ordinal *int   `json:"ordinal,omitempty"`
}
```

- The request carries the business inputs only: `--dry-run`, `--expect-fingerprint` and `--ask` are `dispatch.Options` (`DryRun`, `ExpectFingerprint`, `IgnoreRemembered`), `--yes` and `--output` are the command line's own route. `--force` belongs there: lifting a refusal is a business input.
- The facts are read once, independent of the request, and fingerprinted as JSON: a fact a check reads and the JSON leaves out is a fact the fingerprint does not guard ([dispatch.md](dispatch.md#the-fingerprint)).
- The plan detail must be a struct: `dispatch.Register` panics otherwise.

## 3. The ports and `Observe`

A command gets its ports, and the directories they work on, when it is built. They are the interfaces of `infra/git` and `store/`, never one the command declares (`ports`), and never built by the command (`portbuild`):

```go
type Ports struct {
	Git      git.ReadWriter
	Meta     store.Meta
	Config   store.ProjectConfig
	Env      store.EnvFiles
	StateDir string // <common>/wtm, the dispatcher's
}

func Command(ports Ports) kernel.Command[Request, Facts, Plan, Detail] {
	return kernel.Command[Request, Facts, Plan, Detail]{
		Name:    domain.CmdCreate,
		Observe: observer(ports),
		Forget:  forget,
		Fields:  fields(ports),
		Rules:   rules(),
		Locks:   locks,
		Plan:    plan,
		Apply:   applier(ports),
	}
}
```

`Observe` reads everything the fields, the rules and the plan decide on: git through `ports.Git`, the records through `identity.Read(ctx, identity.ReadParams{Git, Meta})`, `config.toml` with `config.Load(config.LoadParams{StateDir: ports.StateDir})` (merged with the global config, defaults applied, validated) and `run.toml` with `config.LoadRun(ports.StateDir)`, the remembered answers from the config it loaded. It writes nothing (`writesink` refuses a write reached from a reader, however it is built), and turns each sentinel it expects into a typed error with its code: a missing `config.toml` (`domain.ErrConfigNotFound`) is `kernel.Precondition(kernel.Problem{Code: CodeConfigNotFound, Cause: err})`, not an `internal` that happens to wrap it.

`Forget` returns the facts without the remembered answers, so `--ask` falls back on each field's other default.

## 4. Fields, rules, locks

Each field is a `kernel.FieldDef` ([commands.md](commands.md#fields)): its `FieldSpec` (path, type, `Label`/`Title`/`Description` — static text of the schema, read as it is by the flags, the form, the JSON schema and the docs — `Required`, `DependsOn`, `Choices` mode, `Constraints`), then `Skip`, `Default`, `Validate` and `Choices`.

- A branch picker is a `select` with `Choices: kernel.ChoicesSearch`. A choice that loads goes in `Choices(ctx, req, facts)`, the only I/O of a field, and still writes nothing.
- A field the repository may remember is `Rememberable`, its `Default` returning the remembered value with `kernel.OriginRemembered`; the command declares `kernel.Remember(kernel.RememberParams{Fields, Label, Description})` beside them, and its `Apply` keeps the answers it names (`ports.Config.Remember`).
- `DependsOn` lists what `Skip`, `Default` and `Validate` read: `kerneltest.CheckDependsOn` fails on a dependency left out.
- A check across fields is a `kernel.Rules` combinator (`Exclusive`, `OneOf`, `Requires`, `RequiredWhen`, `Distinct`, `NotSelfParent`), never a branch in `Validate`.
- `Locks(req)` names what the run holds: `kernel.LockRepo` shared, and each worktree it changes by branch, exclusive. A branch not known yet (an empty field before its default) is left out; `dispatch` takes the rest once the defaults are filled in.

**Bypass on the engine.** There is no `Resolve` and no `Unattended`: a field the request leaves empty takes its `Default` (a remembered answer first, `--ask` to ignore it), and one with no default is a `422 required` naming its flag. A safe default is a `Default`; a selection with no safe default has none. `--yes` only keeps the form away, and changes nothing in JSON, which is unattended already.

## 5. Codes, messages, exit codes

Everything the command says at run time is a `kernel.Code` and its `Params` ([commands.md](commands.md#a-commands-codes)):

```go
const (
	CodeBranchTaken    kernel.Code = "create.branch_taken"
	CodeConfigNotFound kernel.Code = "create.config_not_found"
)

var Messages = text.Messages{
	CodeBranchTaken:    "{branch} is already checked out at {path}",
	CodeConfigNotFound: "no wtm config in this repository: run wtm init",
}

var ExitCodes = map[kernel.Code]int{
	CodeBranchTaken:    domain.ExitCodeWorktreeExists,
	CodeConfigNotFound: domain.ExitCodeConfigNotFound,
}
```

A code with no message fails `make lint` (`catalog`), and so does an English sentence in `errors.New` or `fmt.Errorf` (`enginetext`; `"%w"` alone passes). `ExitCodes` keeps the number the legacy command exited with (10 to 18); a code it does not list exits on its kind. The `…Fmt` templates of `domain/` the legacy command printed become `Messages`.

## 6. `Plan` and `Apply`

`Plan(ctx, req, facts)` is pure over its inputs and returns `kernel.Plan[Plan]{Detail, Warnings}`: the dry run, what the form's recap and `--dry-run` show.

`Apply` runs one unit per item through `kernel.Each`, passing on what it is handed:

```go
func applier(ports Ports) func(context.Context, kernel.ApplyInput[Request, Facts, Plan, Detail]) (kernel.Outcome[Detail], error) {
	return func(ctx context.Context, in kernel.ApplyInput[Request, Facts, Plan, Detail]) (kernel.Outcome[Detail], error) {
		items := kernel.Each(ctx, kernel.EachParams[string, Detail]{
			Items:      in.Plan.Detail.Branches,
			Subject:    func(branch string) string { return branch },
			Unit:       func(branch string) kernel.Unit[Detail] { return unit(ports, in, branch) },
			Emit:       in.Emit,
			Shield:     in.Shield,
			Milestones: in.Milestones,
		})
		return kernel.Outcome[Detail]{Items: items}, nil
	}
}
```

- **The saga** is what must be all or nothing: the worktree step is `gitwt.Add` as its `Do` and `gitwt.Remove` with the same `gitwt.Params` as its `Undo`, then the `meta.json` record (`ports.Meta`). `Commit` returns the `Detail` once every step is done; `LeftBehind` names the follow-up (`clean --force`) for an undo that fails. `kerneltest.CheckSaga` fails each step in turn and wants the state back each time.
- **`Then`** is what follows the commit and is never undone: the phase that allocates the ordinal (`identity.Allocate`, its `Claim.Allocated` saying whether this run gave it), the `.env` files (`runenv`, `ports.Env`), then `on_create`: a phase with its `Log` (`rules.HooksLogPath`) that calls `hooks.Execute` with `Output: report.Output`. `dispatch` keeps the log and the surfaces draw the lines; nothing in the command prints.
- **`Before`** is what runs before anything changes and may be interrupted (stopping jobs before a removal).
- The engine's services are the only mutators a command calls (`writesink`); a mutator of the commands on `flow/` (`worktree.Create`…) is refused in `command/` (`chokepoint`).
- An item's status, reason and error come from `kernel.ItemFor` through `Each`; a warning about one subject is `report.Warn`, one about the run goes in `Outcome.Warnings`.

## 7. Events

A command never publishes (`publish`). It says what each milestone of its units changed, as pure functions of the facts and the milestone that build the `domain.Event`s without reading the repository again ([dispatch.md](dispatch.md#progress-hook-logs-and-events)):

```go
func Created(f Facts, m kernel.Committed[Detail]) []domain.Event        // worktree.created, before on_create
func Updated(f Facts, m kernel.PhaseDone[Detail]) []domain.Event        // worktree.updated after the phase that allocated the ordinal
func Provisioned(f Facts, item kernel.Item[Detail]) []domain.Event      // worktree.provisioned once the unit is over
```

`dispatch.Events` and `dispatch.EventOf` are types of `dispatch`, which `command/` never imports (`engine`): the command declares its functions over `kernel` types, and the registry wraps them into a `dispatch.Events[Facts, Detail]` at registration (`func(e dispatch.EventOf[…]) []domain.Event { return create.Created(e.Facts, e.Milestone) }`). `dispatch` stamps and publishes what they return. The dispatch fixture's `createEvents` is the timing to keep.

## 8. Registration

Its line in `registry.Production` (`internal/registry/registry.go`), with its real ports on the dispatcher's directories:

```go
dispatch.Register(dispatch.RegisterParams[create.Request, create.Facts, create.Plan, create.Detail]{
	Dispatcher: d,
	Command: create.Command(create.Ports{
		Git:      git.Repo{Dir: d.Dir(), Remote: domain.OriginRemote},
		Meta:     store.MetaDisk{StateDir: d.StateDir()},
		Config:   store.ConfigDisk{StateDir: d.StateDir()},
		Env:      store.EnvDisk{},
		StateDir: d.StateDir(),
	}),
	Messages:  create.Messages,
	ExitCodes: create.ExitCodes,
	Events:    createEvents(), // create.Created, Updated, Provisioned wrapped
}),
```

`Dispatcher` keeps its `Params` unexported today: the pilot adds the two accessors (`Dir()`, `StateDir()`) its first port needs. Outside the test doubles of `testutil/`, the registry is the only package that imports `command/` (`layers`) and the only one that builds a real port (`portbuild`). Two entries spelling one code panic at start-up, when the catalogue is built.

## 9. The command line

`internal/surface/cli/create.go` binds it ([cli.md](cli.md#binding-a-command)):

```go
var createCmd = cli.Bind(domain.CmdCreate, cli.Doc{Use: domain.CmdCreate + " [branch...]", Short: "…", Long: "…", Example: "…"},
	cli.Positional("branches"),
	cli.Flag(cli.FlagParams{Path: "fast_forward", Name: "ff"}),
	cli.Detail(createDetail{}),
)
```

- Every flag comes from a field: its path in kebab case, its help the field's `Description`. A `cli.Flag` keeps the spelling the legacy command had; `--yes`, `--output`, `--dry-run` and `--expect-fingerprint` are added once, `--ask` when a field is `Rememberable`.
- `createDetail` is a `render.Detail`: `Plan` gives the rows of the plan, `Item` draws a done item; `cli.GenericDetail()` when the generic rendering is enough. `TestEveryEntryOfTheRegistryIsServedOrDeclaredUnbound` fails until the entry is served and declares one.
- The legacy runner (`surface/cli/wt/create.go`) and its presenter leave in the same change; the command is attached to the root in its group (`root.go`).

The form needs nothing more: it reads the command's schema, state and choices through `dispatch` ([form.md](form.md)).

## 10. Where the rest goes

- **Decisions** the command shares with the services are pure functions in the sub-package of their subject (`rules/worktree`, `rules/runenv`…), never at the root of `rules/`; a type or a constant goes in the sub-package of its subject under `domain/`. `rootfreeze` counts the files and exported symbols at both roots in `.archlint-migrating`: moving a command's rules out lowers its counts, and a count higher than needed is reported as a note to lower it. Lower it in the same change.
- **Constants**: the command's name stays `domain.CmdCreate` (the name the registry, `cli.Bind` and the docs share); the flags need no constant, they are the paths of the fields. A path or a code is a constant of the command.
- **The exceptions it empties**: the `.archlint-migrating` lines of what it no longer uses (`gitexec`, `portbuild`, `stateowner`, `statewrite` sites); the mutators it no longer calls leave the `chokepoint` table (`tools/archlint/chokepoint.go`) with their last caller on `flow/`, and the `cli.Bind` line of `.deadcode-ignore` for the first command. Both files only shrink.

## 11. Tests

testify, the kernel's layout: a test file per source file holding only `Test…` functions, the example and its fakes in `fixture_test.go`, the shared helpers in `helpers_test.go`.

- **The kernel's checks**: `kerneltest.CheckDependsOn(t, kerneltest.DependsOnParams{Fields, Request, Facts})` on the fields, `kerneltest.CheckSaga(t, kerneltest.SagaParams{Saga, Snapshot})` on each saga.
- **Through `dispatch`**: `dispatchtest.New(t, dispatchtest.BenchParams{})` and `dispatchtest.Run(ctx, dispatchtest.RunParams{Bench, Command, Events, Request, Options})` give the `Result`, the error, the progress, and the events on `Bench.Bus` — a dry run, an apply, a `422`, a `409`, a `412`, an interruption.
- **The ports**: git with `gittest.NewFake(gittest.FakeParams{…})` (or `gittest.Build` for a real repository in the same shape), the state files with `storetest.NewFake(storetest.FakeParams{…})`. Both record their calls and fail a method on demand.
- **The hooks** run a real `/bin/sh` through `hooks.Execute`, in a temporary directory: a hook phase is tested for its lines, its log and its failure, not doubled.
- **The command line**: the parity test, and the behaviour the user observes. The characterization tests of the legacy command (`surface/cli/wt`) are the specification of what must not change; one that changes is a behaviour change, named in the PR.

## 12. What the lint catches

| Mistake | Rule |
| -- | -- |
| `command/` importing `dispatch`, a surface, `flow/` or a service of the commands on `flow/` | `layers`, `engine` |
| a surface importing `command/` | `engine` |
| `context.Background()` in `command/` | `engine` |
| a write reached from `Observe`, `Forget`, `Locks`, `Plan`, a field's functions, a rule or the events | `writesink` |
| a mutator of the engine's services called outside `command/` or a service | `writesink` |
| a mutator of `flow/` (`worktree.Create`…) called from `command/` | `chokepoint` |
| a `Publish`, or a `domain.Event` handed to a function, in `command/` | `publish` |
| an interface with an exported method declared in `command/` | `ports` |
| `git.Repo{…}` or `store.MetaDisk{…}` built outside the registry (and `testutil/`) | `portbuild` |
| a code with no message, or a message keyed by a bare string | `catalog` |
| English in an error, or a code built from a value | `enginetext` |
| git started outside `infra/git` | `gitexec` |
| a file written outside `infra/fs`, a state file outside `store/` | `statewrite`, `stateowner` |
| no package doc | `pkgdoc` |
| a new file or exported symbol at the root of `rules/` or `domain/` | `rootfreeze` |
| a type switch over `kernel.Error`, `kernel.Value` or `kernel.Progress` missing a variant | `sumtype` |
| an entry of the registry no command serves | `TestEveryEntryOfTheRegistryIsServedOrDeclaredUnbound` |

## 13. The pilot's checklist

In the PR that moves the command:

1. `make docs`: the flags and their help now come from the fields; the README table if the command moved group.
2. The agent skill, `internal/surface/cli/agents/assets/using-wtm/`: the command's JSON is the v2 envelope ([output.md](output.md#a-command-on-the-engine)) and its refusals exit 23 (`refused`), 24 (`conflict`, `lock.held`) and 25 (`precondition`, `plan.stale`). For `create`: `references/json.md` (its section on `create`, `extract`, `checkout`) and `references/worktrees.md` (its `create` section), and `SKILL.md`'s exit codes.
3. `CHANGELOG.md` and a `docs/guide/migrating-to-<version>.md` page for what a script reading the old JSON must change.
4. The `.archlint-migrating` and `.deadcode-ignore` lines it empties (step 10).
5. The table of [architecture.md](architecture.md#where-each-command-runs).
6. The `build-validator` subagent, and the command seen running in the `wtm-sandbox`.

## The bridge: `flow/legacy`

Two consumers on `flow/` use `create` today: `flow/extract`, which embeds create's steps (`create.Embed`) and provisions a new target through them, and the dashboard (`startCreate` in `surface/tui/dashboard/actions.go`), which runs `createflow.Run` in a modal. Once `create` is on the engine, they reach it through `internal/flow/legacy`, which the pilot writes:

- **One way.** `flow/legacy` imports `kernel`, `dispatch`, `flow`, `domain` and `rules`, never `command/`; nothing on the engine imports it, and only `surface/cli` does (`layers`). The other packages of `flow/` still reach neither `kernel` nor `dispatch`.
- **Through interfaces the consumers declare.** `flow/extract` declares, in `flow` terms, what it needs of create — its steps, their presets, the recap lines, the provision of one branch — where `create.Embedded` is today; the dashboard declares what it runs. `surface/cli` builds `flow/legacy` from the registry's entry and hands it to both; the dashboard never imports `flow/legacy` or `dispatch`.
- **`flow/create`** leaves with the pilot: its run is the command's, its steps the command's fields, and what `extract` embeds is answered by the bridge over the entry (`State`, `Choices`, a run).
- **`flowtest`** stays the double of the commands on `flow/`: `flow/extract` keeps its tests, with a double of the interface it declares in place of `create.Embed`. A command on the engine is tested with `dispatchtest`.

## A command on `flow/`

The recipe of the commands not yet on the engine, for a change to one of them. The model is [flow-layer.md](flow-layer.md).

### 1. Declare the vocabulary in `domain/`

Constants first, so nothing downstream invents a string:

- `domain.CmdSplit` for the command name, `domain.FlagInto` for each new flag (`internal/domain/constants.go`).
- Every user-visible label, description, recap field and skip reason. A step's prose lives in `constants.go`, not as a literal in the flow.
- A sentinel in `internal/domain/errors.go` for each required selection that has no safe default, worded so it names the flag: `ErrSplitTargetRequired`.
- If a surface will have to schedule it, an `OpKind` constant.

### 2. Put the decisions in `rules/`

Anything that is a *decision over data* — is this safe, which of these applies, what is the default here — is a pure function in the sub-package of its subject under `internal/rules/`, taking domain types and returning domain types. No I/O, no writing. It is then testable as a table and callable from the flow, the service and any surface.

### 3. Write the flow package

`internal/flow/split/split.go` — the run:

```go
type Request struct {
	Branch string
	Into   string
	Force  bool // the safety axis, if the command has refusals to lift
}

type Outcome struct {
	Branch  string
	Result  domain.SplitResult
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Split(Outcome) error // the typed conclusion, one per command
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &splitFlow{runCtx: ctx, ctx: params.Context, request: params.Request,
		prompter: params.Prompter, presenter: params.Presenter}
	return f.run()
}
```

Rules that are not negotiable:

- The package imports **only** `internal/service`, `internal/rules`, `internal/domain` and the stdlib. Never cobra, bubbletea, lipgloss, `internal/surface/cli/render`, `internal/surface/tui`, `internal/config` or `internal/surface/cli`. If you need something only `infra/` has, add a thin wrapper in `service/` — as `worktree.FindByBranch` does.
- `Request` carries **no `--yes` and no `--output`**. `--force` does belong there.
- The run's `context.Context` is the first argument, never a `Params` field. Every service call that may reach a subprocess or the daemon takes it, and a `Stage`'s work uses the one `Work` is handed (see [flow-layer.md](flow-layer.md#cancellation)).
- Errors are returned. A user abort is `presenter.Notice(flow.AbortedNotice)` followed by `Outcome{Aborted: true}, nil`.
- Long work goes through `presenter.Stage`; hook output through `presenter.HookPhase`; a line inside an ongoing phase through `presenter.Status`. The flow never prints.

`internal/flow/split/steps.go` — the questions:

```go
const (
	KeyBranch = "split.branch"
	KeyInto   = "split.into"
	KeyRecap  = "split.recap"
)

func (f *splitFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.SplitWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyBranch: f.request.Branch,
			KeyInto:   f.request.Into,
		}),
		Steps: []flow.Step{f.branchStep(), f.intoStep(), f.recapStep()},
	}
}
```

For **each** step, decide its `Resolve` — that is the bypass taxonomy, and it is the step's own business:

| The step is… | `Resolve` |
| -- | -- |
| a decision with a safe default | returns that `Answer`. Never destructive. |
| a required selection with no safe default | returns an error naming the flag, usually the domain sentinel |
| answerable only by a human | omitted entirely — `flow.Unattended` then refuses, naming `Label` and `Flag` |

And the rest of the fields:

- `Skip func(Answers) (skip bool, reason string)` when the step can become irrelevant. The reason is user-visible; put it in `constants.go`.
- `Build` for content derived from earlier answers, `Load` when deriving it does I/O (plus `LoadingMessage`). Never do slow work in `Build`.
- `Summarize` when the raw answer value is not what the user should read back.
- `Flag` so a refusal can name the flag that would have answered the step.
- `Blockers` on the `StepContent` of a step whose dangerous option is gated by safety refusals — one entry per refusal, never folded into the prose, so a surface can have them lifted one at a time.
- The **recap is always the last step** and always unconditional. Its `Build` names every part of the plan, including the parts a flag resolved — read the value from `Answers`, which returns presets too. A flag must never make a line disappear.

If a surface may run several of these at once, declare how:

```go
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindSplit, Mode: flow.ModeBlocking, TargetKey: KeyBranch}
}
```

### 4. Wire the command

`internal/surface/cli/wt/split.go` holds flag wiring and nothing else:

```go
func runSplit(cmd *cobra.Command, args []string) error {
	into, _ := cmd.Flags().GetString(domain.FlagInto)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return domain.ErrSplitJSONNeedsYes
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The one place --yes is read: which Prompter gets installed.
	interactive := rules.IsHumanFormat(format) && !yes && shared.StdinIsTerminal()

	_, err = splitflow.Run(cmd.Context(), splitflow.Params{
		Context:   shared.FlowContext(config),
		Request:   splitflow.Request{Branch: branchName, Into: into, Force: force},
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive}),
		Presenter: splitPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
```

Flag help strings are uniform:

- `--yes` / `-y` — *"Skip all prompts; resolve every decision from flags and safe defaults (…)"*
- `--force` — *"Lift safety refusals (…); still asks to confirm unless --yes"*

Register the command in its parent group and give it a `GroupID` (`domain.CmdGroup*`), or it renders under a stray "Additional Commands" heading.

### 5. Add the CLI presenter

In `internal/surface/cli/wt/presenter.go`, next to `createPresenter` and `cleanPresenter`:

```go
type splitPresenter struct{ shared.CLIPresenter }

func (p splitPresenter) Split(outcome splitflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return render.WriteSplitJSON(p.Cmd.OutOrStdout(), outcome.Result)
	}
	render.Frame(p.Cmd.OutOrStdout(), func() {
		render.FormatSplitResult(p.Cmd.OutOrStdout(), /* … */)
	})
	return nil
}
```

`shared.CLIPresenter` (`internal/surface/cli/shared/presenter.go`) already implements `Stage`, `HookPhase`, `Notice` and `Status` — embed it and add only the typed conclusion. The frame is applied **exactly once**, here; JSON and machine output are never framed.

### 6. Test it

- **Flow tests** in the flow package, with `flowtest.ScriptedPrompter` and `flowtest.Recorder`. Assert on the answers that were asked (`AskedKeys()`), on the `StepContent` a step produced (the recap prose is user-visible behavior), and on the outcome.
- **Unattended tests** with `flow.Unattended{}` directly: each required selection refuses and names its flag, each defaulted decision lands on the safe value.
- **Integration tests** at the Cobra level (`gittest.InitRepo` + `WTM_PROJECT_DIR` / `WTM_STATE_DIR`) for the two axes end to end: `--yes` without the required flag errors, `--force` alone still confirms, `--output json` requires `--yes`.

### 7. Documentation

1. `make docs` — regenerates `docs/`, never hand-edited.
2. Add the command to the `README.md` overview table, in the same group as the root `--help`.
3. Update the agent skill (`internal/surface/cli/agents/assets/using-wtm/`, the reference file of the command's topic) if the agent-facing surface changed (a new command, a new flag, a changed JSON shape, changed failure/abort semantics).
4. Run the `build-validator` subagent: `make lint` fails if `internal/flow/` gained a forbidden import.

### In the dashboard

Nothing in the flow changes. In `internal/surface/tui/dashboard/actions.go`, add a `startSplit` that checks `busyReason`, calls `beginOp(beginParams{Operation: splitflow.Operation()})`, and launches `splitflow.Run` in a `tea.Cmd` with the dashboard's `prompter` and a presenter embedding `dashboard.presenter` plus the typed conclusion. If the flow uses a step kind the dashboard's modal cannot render, that is the only work left — and `flowui` will refuse an unknown kind rather than guess, so you will hear about it immediately.
