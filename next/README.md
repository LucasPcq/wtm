# next/ — wtm as use-case slices

`wtm-next` (`make next` → `bin/wtm-next`) is wtm rebuilt as plain functions: a pure domain, one package per use case behind an in-process API, adapters behind the small interfaces each use case declares, a command line per command over a shared presentation kit. It works on the same repositories and state files as `wtm`: same state directory, `meta.json`, `config.toml` keys, operation locks and `repos.json`; its event bus is its own (`wtm-next daemon`, schema v2), on the socket legacy's run daemon used. Nothing here imports `github.com/LucasPcq/wtm/internal/…`.

## Layout

```
cmd/wtm-next/          the composition root: builds each adapter once (wire.go), opens the project on demand, registers each command's cobra command on the root
internal/
  domain/              the contract every use case speaks: errors, warnings, events and their stream, outcomes (`Outcome`, `Result`, `Unit`, generic over a command's plan and detail; `Read`, a read's) and the envelope that reports them, operations and their locks, the vocabulary of a question and a form's schema, a follow-up (a command and its answers, or a link), a plan's recap (`recap.go`: `RecapRow`, `RecapValue`, the marks of an answer), and the JSON of each a surface receives, and the line of the event stream (`stream.go`: `Line`, `MarshalLine`, `StreamVersion`) — pure, stdlib only
    worktree/          branches and worktrees as git lists them: names, refs, where a branch stands against origin, where a branch asked for a new worktree lands, a checkout, meta.json, identity, the stack of worktrees by the branch each was created from (`StackOf`, which `command/tree` reads), a branch's pull request and its state (open, merged, closed)
    project/           what a use case reads of config.toml, run.toml (isolation included) and the global settings
    dotenv/            .env files parsed and rendered byte for byte
    env/               a worktree's environment: where its .env files come from, what wtm writes into them, what its jobs and hooks run with, its hook chain
    run/               what the run daemon says of the jobs it runs
                       (env imports worktree, project and dotenv; worktree imports project; domain imports worktree and env)
  command/<name>/      one command, one vertical slice: its use case and its command line
    usecase.go         the story: Run (operation, plan, fingerprint, apply), Plan, Apply, Observe, Questions
    port.go            the driven ports it reaches the world through
    rules.go           pure: facts → plan (a read: facts → its Detail of each item)
    questions.go       its form model, its schema (Questions.Form, Request.Answer and Request.Values: what the flags, --help, the command line and the wizard read) and its validation
    cli/               its driving adapter: the cobra command over form.Command, what its recap and dry run show of a plan (Preview) and how it draws an apply (Draw), the steps it asks its own way
  service/             what the use cases call
    operation/         the operation every mutating use case goes through: its record, its locks, its plan under them and fingerprinted, its events
    operationtest/     the contract every mutating use case's Run passes (no lock nor event on a dry run, a stale plan refused before any event, events in order), and the in-memory locks, publisher and event recorder of an operations service, for tests
    provision/         a new worktree made whole and a source fast-forwarded, shared by the commands that create one, with the questions each of them asks (questions.go: the fragment their forms embed, whose Answers are all Run takes), the fingerprint of a plan that provisions and the answers remembered
    repository/        the facts of the repository a command decides from (config, run.toml, snapshot), and what the reads share (the meta records, whether a worktree's directory is gone, the run daemon and the forge asked while the worktrees are read: Records, Gone, Present, Ask, Each), wired once, refusing a repository with no wtm config with the command's `<name>.config_not_found` (its follow-up, `init`, injected), and one whose config.toml does not parse or validate with `<name>.config_invalid`
    registry/          the repositories wtm knows, for a stream of every one: a repository joins once its state directory holds a wtm config and leaves once it no longer does, each announced on the bus (`repo.added`, `repo.removed`); the registry is left alone while the state directory is moved (`$WTM_STATE_DIR`): none joins, none leaves, it lists none and a stream joins none another process announces
  adapter/             what the ports reach out to: git, statefile, store, config, shell, oplock, events (the event bus: its daemon, its publisher, a subscription, the v2 schema), registry (repos.json), proxyport, daemon (the run daemon's jobs, never starting it), github (gh: the newest pull request of each branch whatever its state, with its checks and review)
    <name>test/        a stateful fake of an adapter, for tests (`gittest`, `storetest`, `configtest`, `registrytest`)
  ui/                  the presentation kit every command's cli shares, knowing no command
    cli/               the root: version, --quiet, help, signals, the exit code a command's failure sets
    output/            the streams (stdin, stdout, stderr, whether each is a terminal, a colour renderer each), frames and blocks, progress, failures, plans, the hook view, the v2 envelope writer
    form/              a command's form, generic over its schema: form.Command (its flags, its request, its launch, the conclusion of its run, its command line), form.Query (the same for a read, with its subject asked first, its picker and the machine formats it prints besides json, tree's mermaid), form.Stream (a stream's flags and request), the session and a step per field kind, the wizard that walks them to a recap and applies, or to the last step of a read's picker
    widget/            the inputs a step asks with: select list, multi-select, text field, text list, their styles, and the problem an input shows with the steps that get past it
    formtest/          for a command's tests: the driver that plays its wizard key by key, golden files, the v2 envelope decoded, a run of its command line (`Run`, `Printed`), a terminal that keeps pressing enter (`Enters`), a use case whose previews and applies a test scripts (`Previewing`)
  api/                 the catalogue of the commands and its streams (`api.Stream`, a command that sends lines until stopped), knowing none of them: each one's name, its schema (domain.Form), the Go types of its plan and its detail, its questions as the repository stands and its execution, the answers by field in and a domain.Envelope out with the status of its kind, for a surface other than the command line; and the schema it publishes (`api.v2.schema.json`, `api.Schema()`)
```
No file format lives in `domain/`: `adapter/config` owns the shapes of `config.toml` and `run.toml` (every key, strict decode) and maps them onto `domain/project`. `worktree.Meta` keeps its JSON tags, because `meta.json` is the record itself, shared with `wtm`; `worktree.Identity` keeps those of the events payload.

A use case reads in one place. `command/<name>/usecase.go` tells the whole request top to bottom in `Run`: begin the operation (`service/operation`), plan under its locks, fingerprint, stop on a dry run, refuse a stale plan, apply between the started and finished events; then `Plan` (observe what the repository holds, settle the request on it, decide), `Apply` and `Questions` when it asks. `rules.go` holds what it observes and decides from, pure; `questions.go` its form, the schema of its fields (labels, help, flags, options) and validation; `port.go` the interfaces it reaches the world through. The root of a command holds these four files, their tests and `cli/`, nothing else (archlint, rule `slicefiles`). A service is concrete and called directly; a port is an interface, for the outside world only. A package is split only when it passes about 1,000 lines and a seam is natural.

## The published schema

`internal/api/api.v2.schema.json` is the JSON Schema of everything the catalogue sends a surface: the v2 envelope, the form and its fields, the events (a `$ref` to `adapter/events/events.v2.json`), each command's request, plan, detail and envelope, and each stream's request and envelope. It is generated from the Go types, never edited by hand: `make schema` rewrites it, `make test-next` fails while it is stale and validates against it each `catalogue_*` golden of `cmd/wtm-next` and each `envelope_*` golden of a command's `cli`. `api.Schema()` returns it for a server; the site publishes it, with `events.v2.json`, on `wtm.sh/schemas/`. How it is built: `.claude/skills/go-architecture/references/contract.md`, "The published schema".

## Import rules

`make lint` checks them (`tools/archlint`, rule `next`).

| Package | May import |
| -- | -- |
| `domain`, `domain/*` | stdlib, `domain/*` |
| `service/<name>` | `domain/*`, stdlib — never another service, `command/*`, `adapter/*`, `ui/*` |
| `command/<name>` | `domain/*`, `service/*`, stdlib — never another command, its own `cli`, `adapter/*`, `ui/*` |
| `service/<name>test` | what `service/<name>` may, third party, and `service/<name>`; imported from a `_test.go` only |
| `command/<name>/cli` | `command/<name>`, `domain/*`, `ui/*`, stdlib, third party — never another command nor its `cli`, `service/*`, `adapter/*` |
| `adapter/<name>` | `domain/*`, `adapter/statefile`, `adapter/shell` (the one way to start a child), stdlib, third party — never `service/*`, `command/*`, `ui/*`, another adapter |
| `adapter/<name>test` | what `adapter/<name>` may, and `adapter/<name>`; imported from a `_test.go` only |
| `ui/<name>` | `domain/*`, `ui/*`, stdlib, third party — never `command/*`, `service/*`, `adapter/*` |
| `ui/<name>test` | what `ui/<name>` may, `ui/<name>` included; imported from a `_test.go` only |
| `api` | `domain/*`, stdlib — never `command/*`, `service/*`, `adapter/*`, `ui/*`, third party |
| any test | never `github.com/LucasPcq/wtm/internal/…`; it sits beside the package it tests, never in a directory of tests alone |
| `cmd/wtm-next` | anything under `next/` except a fake |

A package in no row is refused: the root of `next/`, a second `cmd/<name>`, anything outside these directories. A name ending in `test` under `service/`, `adapter/` or `ui/` is reserved for the kit of the package beside it: `service/latest`, with no `service/la`, is refused. A service is one package: a sub-package `service/<name>/<sub>` falls in the `service/<name>` row and may not import `service/<name>`; a command's `cli` likewise imports no sub-package of its own.

A code a command declares starts with its `Name` and a dot (`create.path_exists`); a code several commands share is a `domain` constant (archlint, rule `codeprefix`).

Only the edges read the environment: `ui/cli` once (`ReadEnv`), `ui/output` the terminal, `cmd/wtm-next` `$WTM_STATE_DIR`, an adapter what its own tool needs; never a domain package, a service or a command, which take the value as an input (archlint, rule `envread`).

A use case or a service declares the ports it needs in its `port.go`, interface types and function types only (archlint, rule `portfile`); an adapter exports a concrete type that satisfies them, checked by the compiler in `cmd/wtm-next`.

## Reading a command

`wtm-next create feat/x --from main`, hop by hop:

1. `cmd/wtm-next/main.go → main`: `ui/output.System()` reads the process's streams and `ui/cli.ReadEnv()` its environment once (the working directory, `$WTM_PROJECT_DIR`, `$WTM_CORRELATION_ID`), and `ui/cli.Main` runs the root with `command/create/cli.New(streams, env, openCreate, commandLine)` registered on it; `ui/cli/root.go → Execute` maps the failure a command returns onto its exit code.
2. `command/create/cli/command.go → createCommand.run` hands the command line to `ui/form/command.go → Command.Launch`: `uicli.Run.Begin` refuses an invalid correlation ID before any repository is looked up, the flags, registered from create's schema, become a `create.Request`, `openCreate(ctx, env.ProjectDir)` (`cmd/wtm-next/project.go`) finds the repository of that directory, always from its main worktree, and wires the use case (`cmd/wtm-next/wire.go → wire`), then it runs the wizard when the user can see and answer it (stdin and stderr terminals, `TERM` not `dumb`, human output, no `--yes` nor `--expect-fingerprint`: `output.Streams.CanPrompt`, `Command.Ask`), else `create.UseCase.Run` with its `domain.RunOptions`, the correlation ID among them; `Launch` hands the run to `Command.Conclude`.
3. `command/create/usecase.go → UseCase.Run`: `operation.Service.Begin` records the operation with the options' correlation ID, refusing an invalid one, and takes its locks (`service/operation/operation.go`), `operation.PlanUnderLocks` plans under them, `operation.Fingerprint` fingerprints the plan, a dry run stops there, `Op.Expect` refuses a stale plan, then `Apply` runs between the started and finished events.
4. `command/create/usecase.go → UseCase.Plan`: `Observe` reads the repository (`repository.Service.Observe`: config, snapshot, run.toml) and what create reads itself through the ports of `port.go`, `Resolve` (`questions.go`) settles the request on those facts, `Decide` (`rules.go`) turns them into the plan.
5. `command/create/usecase.go → UseCase.Apply`: an optional fast-forward of the source (`provision.Service.FastForwardEach`, `service/provision/fastforward.go`, each source it could not move a warning under create's code), then `provision.Service.Run` (`service/provision/provision.go`) per branch, with `provision.ParamsOf`: the worktree, its env, its meta record, its hooks; `provision.Service.Remember` then writes the answers asked to be remembered.
6. `adapter/git/worktree.go → Repo.AddWorktree`: the port's implementation runs `git worktree add`.
7. Back in `ui/form/command.go → Command.Conclude`: a refusal, or the dry run drawn from create's `Preview` (`form.PreviewOf`) and ended on the line that applies exactly that plan, or the outcome create's `Draw` draws (`command/create/cli/output.go → drawOutcome`, a call to `ui/output.DrawApplied`); with `--output json`, `create.Outcome.Envelope` (`domain.Outcome.Envelope`) reports the run as a `domain.Envelope`, which `ui/output.Envelope` writes with each field spelled as its flag.

A second command is its own `command/<name>/` (use case, ports, rules, questions, `cli/`) and one line in each list of `cmd/wtm-next`: its constructor in `main.go`'s `commands`, its opener in `project.go` (`opener(func(a app) <name>.UseCase { return a.<Name> })`), its binding in `catalogue.go` (`api.Bind(<name>.Name, open, UseCase.Questions, UseCase.Run)`), its adapters wired in `wire.go`. `registration_test.go` fails when a command of `main.go` is missing from the catalogue or from the wiring, and `contract_test.go` when it is not in the catalogue contract (`.claude/skills/go-architecture/references/contract.md`, "The catalogue contract").

A read, `command/list`, `command/status` or `command/tree`, walks the same hops with a `form.Query` in place of a `form.Command`, under the contract of a read (`.claude/skills/go-architecture/references/contract.md`, "A read").

A stream, `command/events`, never ends on its own: its use case is built once in `wire.go` (`eventStream`), not per repository in `app`, since it may follow every repository; its cli is a `form.Stream` (flags and `--output` only) that prints each line itself; `catalogue.go` lists it in `streams()` (`api.BindStream`) rather than the catalogue, and `registration_test.go` names it in `streamsListed` (`.claude/skills/go-architecture/references/contract.md`, "A stream"). Every opener of `project.go` registers the repository it opens through `service/registry` (`knownRepos` in `wire.go`: `repos.json`, the bus, and "initialized" read by `config.Project.Initialized` in the state directory `stateDir` locates; `movedState`, the one reader of `$WTM_STATE_DIR`, says when the registry is left alone), for a stream of every repository; the stream registers the one it watches after reading its config, never one it joins. `openEvents` opens a repository with the same pieces `wire` builds (`wireParams.facts`, `wireParams.repo`). Its request reads where its caller stands (`domain.Caller`): the form engine sets it from the command line's environment and `api.BindStream` from the call's directory, so a relative `--repo` is read from the same place on both, and a call naming no directory stands nowhere and streams every repository.

Todo: the follow-up a read's picker ends on is printed by `form.Query`; once every command it names has migrated, it runs in this process instead (`api.Catalogue.Find(followUp.Command)` with its answers), and its command line is spelled from that command's schema rather than by `commandLine` (`cmd/wtm-next/wire.go`, handed to each command's `cli.New`).

## Comments

Go's doc-comment conventions, as the `go-style` skill states them ([`comments.md`](../.claude/skills/go-style/references/comments.md)): every package has a package comment and every exported name a doc comment that starts with its name; a comment in a body says why, never what. `staticcheck` checks their shape in `make lint` (`staticcheck.conf`); `make next-lint` runs revive (`revive.toml`), which requires them.

## Cancellation

- **Every child process runs in its own process group**, git included, and wtm signals the whole group itself: SIGINT, then SIGTERM, then SIGKILL, bounded by `exec.Cmd.WaitDelay`. The terminal's Ctrl+C therefore never reaches a child directly: `git worktree add` would otherwise undo itself on SIGINT while wtm records the worktree, and a hook would receive SIGINT twice.
- **The first Ctrl+C** cancels the operation: the worktree being added finishes its add, env copy and meta record (a shielded section), its hooks are stopped, and the later worktrees are skipped as interrupted. In human mode it prints `Cancelling — finishing <step> of <branch>; Ctrl+C again to stop now` on stderr. The command exits 19.
- **The second Ctrl+C** stops at once: the running children's groups are killed and the command exits 130.
- **A detached caller** (an HTTP or desktop adapter, later) passes the use case its own operation context, never a request's: a client that disconnects does not cancel the operation, an explicit cancel acts as the first Ctrl+C and a forced one as the second.

## The branch

`next/` lives on the `0.31.0` branch, cut from `main` at 0.30.1. A fix to the legacy `wtm` is made on `main` and `main` is merged into `0.31.0`, so `next/` always runs against the legacy it will replace and the formats it writes. Each command migrated to `next/` arrives by its own pull request to `0.31.0`, with its proof against the legacy command in `docs/proof/`.
