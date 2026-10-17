---
name: go-architecture
description: The architecture of wtm's next/ — a hexagon of vertical slices where each command is one folder (its use case, its ports, its rules, its questions, its CLI) over shared services, a pure domain, driven adapters and a UI kit. Use this skill whenever you add, change, review or move code under next/ — a command, a use case, a port, a service, an adapter, the ui kit, the wiring in cmd/wtm-next — whenever someone asks where code should go, how a request flows through the code, how two commands share something, or how a CLI/TUI command should behave (flags, --yes/--force, dry run, JSON envelope, exit codes, wizard). Also use it for architecture questions about this repository (hexagonal, ports and adapters, vertical slices) even if next/ is not named. Pair it with the go-style skill, which governs how the Go itself is written.
---

# go-architecture — how next/ is built

`next/` is wtm rebuilt as a hexagon of vertical slices: **a command is one folder**, and inside it a **use case tells the whole request** top to bottom, reaching the outside world only through the **ports** it declares and calling **services** shared with other commands. The Go itself follows the `go-style` skill; this skill says where things go and how they talk. The legacy code under `internal/` follows `CLAUDE.md` and `docs/dev/` until it moves here.

## The map

```
next/
  cmd/wtm-next/         composition root: wire.go builds every adapter once and injects it; main.go registers each command's CLI
  internal/
    command/<name>/     one command, one vertical slice
      usecase.go        the use case: Run (the story), Plan, Apply, Questions, Observe
      port.go           the driven ports it calls (interfaces)
      rules.go          pure: facts → plan
      questions.go      pure: the request, its form model, its validation
      cli/              its driving adapter: cobra command, flags, wizard steps, output
    service/            what use cases call: operation (locks, plan under them, fingerprint, events), provision (a new worktree made whole), repository (the facts a command decides from) + <name>test contracts (operationtest)
    domain/             the model, pure: errors, events, operations and their run options, question vocabulary; worktree/ (with the stack graph), project/, env/, dotenv/
    adapter/<name>/     driven adapters implementing the ports (git, store, config, shell, oplock, events…) + <name>test fakes
    ui/                 the presentation kit, knowing no command: cli (root, exit codes, signals), output (frames, envelope), form (wizard engine, components)
```

Dependencies point inward, checked by `make lint` (archlint `next`): `domain` imports nothing of next/; a `service` imports `domain`; a `command` imports `domain` and `service`; its `cli/` imports its command, `domain` and `ui`, never a service; an `adapter` imports `domain`; `ui` imports `domain`; only `cmd/wtm-next` sees everything. A command never imports another command.

Why a package per command, when the Go guides prefer a package per concept and Ben Johnson warns against grouping by module: a command *is* a concept with its own API (its request, its plan, its outcome), large CLIs do the same (the GitHub CLI's `pkg/cmd/<group>/<cmd>/`), and the two risks of grouping by feature are closed by construction — import cycles by the dependency rule above, repeated names (`create.CreateRequest`) by naming inside the package (`create.Request`). Ports follow go-style `interfaces.md`: each use case declares, consumer-side, the small interfaces it calls.

## Reading a request

`wtm-next create feat/x --from main`, hop by hop:

1. `cmd/wtm-next/main.go`: the root runs with each command's CLI registered.
2. `command/create/cli/command.go`: flags → `create.Request` + `domain.RunOptions`; the wizard on a terminal, else `UseCase.Run`.
3. `command/create/usecase.go` → `Run`: begin the operation (locks), plan under the locks, fingerprint, stop on a dry run, refuse a stale plan, apply between the events.
4. `Plan`: `Observe` reads the repository through the ports of `port.go`; `Resolve` (`questions.go`) settles the request; `Decide` (`rules.go`) makes the plan.
5. `Apply`: its steps, calling `service/provision` for each new worktree.
6. `adapter/git`: the port's implementation runs git.
7. Back in `cli/command.go`: `output.go` draws the outcome with the `ui/` kit, or the JSON envelope.

## Where does this code go?

| The code… | Goes in |
| -- | -- |
| decides something from facts, no I/O, for one command | `command/<name>/rules.go` |
| validates or defaults the request, builds the form model and its schema (labels, help, flag names, option labels) | `command/<name>/questions.go` |
| sequences the request, calls ports and services | `command/<name>/usecase.go` |
| is an interface to the outside world one command needs | `command/<name>/port.go` |
| registers flags, renders output, draws the wizard steps of one command from its schema | `command/<name>/cli/` |
| is a pure rule or a type two commands need | `domain/…` |
| orchestrates I/O two commands need | `service/<name>` (with its own `port.go`) |
| is presentation two commands need | `ui/…` |
| talks to git, the filesystem, a process, a socket | `adapter/<name>`, behind a port |
| chooses which adapter backs which port | `cmd/wtm-next/wire.go` |

## Read next

- **`references/slice.md`** — the anatomy of a command folder: the order of `usecase.go`, the canonical `Run`, what `rules.go`/`questions.go`/`port.go` hold, how commands share code, codes and exit codes, testing a slice, the steps to add a command. Read it before writing or reviewing any file under `command/`.
- **`references/contract.md`** — what every command promises: request and options, interactive vs unattended, `--yes` vs `--force`, remembered answers, dry run and fingerprint, human output vs the JSON envelope, exit codes, locks and events. Read it before touching flags, output or a command's behaviour.
- **`references/cli.md`** — community conventions for a Go CLI (clig.dev, Cobra, the GitHub CLI's command anatomy, IOStreams) and how `next/` applies them.
- **`references/tui.md`** — community conventions for a Bubble Tea TUI (the Elm loop, commands for I/O, model composition, huh) and how the `ui/form` wizard engine applies them.

`next/README.md` is the human-facing map of the same architecture and carries the import table archlint enforces; keep it, this skill and the code in step in the same change.
