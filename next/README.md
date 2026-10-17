# next/ — wtm as use-case slices

`wtm-next` (`make next` → `bin/wtm-next`) is wtm rebuilt as plain functions: a pure domain, one package per use case behind an in-process API, adapters behind the small interfaces each use case declares, a command line per command over a shared presentation kit. It works on the same repositories and state files as `wtm`: same state directory, `meta.json`, `config.toml` keys, operation locks and events socket. Nothing here imports `github.com/LucasPcq/wtm/internal/…`.

## Layout

```
cmd/wtm-next/          the composition root: builds each adapter once (wire.go), opens the project on demand, registers each command's cobra command on the root
internal/
  domain/              the contract every use case speaks: errors, warnings, events and their stream, outcomes, operations and their locks, the vocabulary of a question — pure, stdlib only
    worktree/          branches and worktrees as git lists them: names, refs, where a branch stands against origin, where a branch asked for a new worktree lands, a checkout, meta.json, identity
    project/           what a use case reads of config.toml, run.toml (isolation included) and the global settings
    dotenv/            .env files parsed and rendered byte for byte
    env/               a worktree's environment: where its .env files come from, what wtm writes into them, what its jobs and hooks run with, its hook chain
                       (env imports worktree, project and dotenv; worktree imports project; domain imports worktree)
  command/<name>/      one command, one vertical slice: its use case and its command line
    usecase.go         the story: Run (operation, plan, fingerprint, apply), Plan, Apply, Observe, Questions
    port.go            the driven ports it reaches the world through
    rules.go           pure: facts → plan
    questions.go       its form model and its validation
    cli/               its driving adapter: the cobra command, its wizard, its human output and its JSON
  service/             what the use cases call
    operation/         the operation every mutating use case goes through: its record, its locks, its plan under them and fingerprinted, its events
    provision/         a new worktree made whole and a source fast-forwarded, shared by the commands that create one
  adapter/             what the ports reach out to: git, statefile, store, config, shell, oplock, events, proxyport
    <name>test/        a stateful fake of an adapter, for tests
  ui/                  the presentation kit every command's cli shares, knowing no command
    cli/               the root: version, --quiet, help, signals, the exit code a command's failure sets
    output/            frames and blocks, progress, failures, plans, the hook view, the v2 envelope writer
    form/              the components of a form: select list, text list, confirm, their styles
```
No file format lives in `domain/`: `adapter/config` owns the shapes of `config.toml` and `run.toml` (every key, strict decode) and maps them onto `domain/project`. `worktree.Meta` keeps its JSON tags, because `meta.json` is the record itself, shared with `wtm`; `worktree.Identity` keeps those of the events payload.

A use case reads in one place. `command/<name>/usecase.go` tells the whole request top to bottom in `Run`: begin the operation (`service/operation`), plan under its locks, fingerprint, stop on a dry run, refuse a stale plan, apply between the started and finished events; then `Plan` (observe what the repository holds, settle the request on it, decide), `Apply` and `Questions` when it asks. `rules.go` holds what it observes and decides from, pure; `questions.go` its form and validation; `port.go` the interfaces it reaches the world through. A service is concrete and called directly; a port is an interface, for the outside world only. A package is split only when it passes about 1,000 lines and a seam is natural.

## Import rules

`make lint` checks them (`tools/archlint`, rule `next`).

| Package | May import |
| -- | -- |
| `domain`, `domain/*` | stdlib, `domain/*` |
| `service/<name>` | `domain/*`, stdlib — never another service, `command/*`, `adapter/*`, `ui/*` |
| `command/<name>` | `domain/*`, `service/*`, stdlib — never another command, its own `cli`, `adapter/*`, `ui/*` |
| `command/<name>/cli` | `command/<name>`, `service/operation`, `domain/*`, `ui/*`, stdlib, third party — never another command nor its `cli`, `adapter/*`, another service |
| `adapter/<name>` | `domain/*`, `adapter/statefile`, `adapter/shell` (the one way to start a child), stdlib, third party — never `service/*`, `command/*`, `ui/*`, another adapter |
| `adapter/<name>test` | what `adapter/<name>` may, and `adapter/<name>`; imported from a `_test.go` only |
| `ui/*` | `domain/*`, `ui/*`, stdlib, third party — never `command/*`, `service/*`, `adapter/*` |
| any test | never `github.com/LucasPcq/wtm/internal/…` |
| `cmd/wtm-next` | anything under `next/` |

A use case or a service declares the interfaces it needs in its `port.go`; an adapter exports a concrete type that satisfies them, checked by the compiler in `cmd/wtm-next`.

## Reading a command

`wtm-next create feat/x --from main`, hop by hop:

1. `cmd/wtm-next/main.go → main`: `ui/cli.Main` runs the root with `command/create/cli.New(openCreate)` registered on it; `ui/cli/root.go → Execute` maps the failure a command returns onto its exit code.
2. `command/create/cli/command.go → createCommand.run`: the flags become a `create.Request`, `openCreate` (`cmd/wtm-next/project.go`) finds the repository and wires the use case (`cmd/wtm-next/wire.go → wire`), then `createCommand.create` runs the wizard on a terminal (`form.go → runForm`), else `create.UseCase.Run` with its `operation.Options`.
3. `command/create/usecase.go → UseCase.Run`: `operation.Service.Begin` takes the operation's locks (`service/operation/operation.go`), `operation.PlanUnderLocks` plans under them, `operation.Fingerprint` fingerprints the plan, a dry run stops there, `Op.Expect` refuses a stale plan, then `Apply` runs between the started and finished events.
4. `command/create/usecase.go → UseCase.Plan`: `Observe` reads the repository through the ports of `port.go` (git, config), `Resolve` (`questions.go`) settles the request on those facts, `Decide` (`rules.go`) turns them into the plan.
5. `command/create/usecase.go → UseCase.Apply`: an optional fast-forward of the source (`provision.Service.FastForward`, `service/provision/fastforward.go`), then `provision.Service.Run` (`service/provision/provision.go`) per branch: the worktree, its env, its meta record, its hooks.
6. `adapter/git/worktree.go → Repo.AddWorktree`: the port's implementation runs `git worktree add`.
7. Back in `command/create/cli/command.go → createCommand.conclude`: `output.go` draws the outcome (or the dry run, or the v2 envelope with `--output json`) with the kit of `ui/output`.

A second command is its own `command/<name>/` (use case, ports, rules, questions, `cli/`) and one line in `cmd/wtm-next`: its constructor in `main.go`'s `Commands`, its adapters wired in `wire.go`.

## Comments

Close to zero. Each package has a one-line package doc; every other comment line counts, godoc and tests included, except a directive (`//go:…`, `//go-sumtype:…`) and a `//nolint:<linters>` with nothing after it. A line earns its place only as a why the code cannot show. `.next-comments` holds each package's count (absent = 0); `make lint` (rule `nextcomments`) refuses a package above it (delete, or raise the count in review) or below it (lower the count).

## Cancellation

- **Every child process runs in its own process group**, git included, and wtm signals the whole group itself: SIGINT, then SIGTERM, then SIGKILL, bounded by `exec.Cmd.WaitDelay`. The terminal's Ctrl+C therefore never reaches a child directly: `git worktree add` would otherwise undo itself on SIGINT while wtm records the worktree, and a hook would receive SIGINT twice.
- **The first Ctrl+C** cancels the operation: the worktree being added finishes its add, env copy and meta record (a shielded section), its hooks are stopped, and the later worktrees are skipped as interrupted. In human mode it prints `Cancelling — finishing <step> of <branch>; Ctrl+C again to stop now` on stderr. The command exits 19.
- **The second Ctrl+C** stops at once: the running children's groups are killed and the command exits 130.
- **A detached caller** (an HTTP or desktop adapter, later) passes the use case its own operation context, never a request's: a client that disconnects does not cancel the operation, an explicit cancel acts as the first Ctrl+C and a forced one as the second.
