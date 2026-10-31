---
name: go-architecture
description: Where code goes in wtm's next/ (the wtm-next rewrite) and the rules that hold it there — a command is one folder (usecase.go, port.go, rules.go, questions.go, cli/) over shared services, a pure domain, adapters with their fakes, a ui/ kit and the wiring in cmd/wtm-next. Use it whenever you add, migrate, change, review or move code under next/ — a command, its Run, a question or flag, a port, an adapter or fake, a service, the form engine, the JSON envelope, exit codes, the registrations — and whenever someone asks where wtm code goes or how two commands share something, even without saying next/ (during the migration, new code goes there). Not for the legacy internal/ (frozen; CLAUDE.md and docs/dev/ govern it). Pair it with go-style for the Go itself.
---

# go-architecture — where code goes in next/

`next/` is wtm rebuilt as vertical slices over a small hexagon: **a command is one folder** whose `usecase.go` tells the whole request, reaching the world only through the **ports** it declares and calling **services** shared with the other commands. This skill says where code goes and which rules hold it there; the Go itself follows `go-style`. `next/README.md` keeps the import table archlint enforces and the hop-by-hop reading of a request.

## The map

```
next/
  cmd/wtm-next/        composition root: main.go (the commands), project.go (openers), catalogue.go (api), wire.go (adapters)
  internal/
    command/<name>/    one command: usecase.go, port.go, rules.go, questions.go, cli/
    service/<name>/    what use cases share with I/O: operation, provision, repository, registry, teardown; <name>test kits
    domain/            the pure model and vocabulary; worktree/, project/, env/, dotenv/, run/ by subject
    adapter/<name>/    git, store, config, shell, oplock, events (the bus), registry, webserver, statefile, proxyport, daemon (the run daemon's protocol and jobs.json), process, joblog, proxy (the run proxy), github; <name>test fakes
    ui/                cli (root, exit codes, env), output (streams, frames, envelope), form (flags, wizard, conclusion), widget (a step's inputs), formtest (the cli tests' kit)
    api/               the catalogue of commands, and its streams, for a surface other than the CLI
    web/               the local HTTP server over api (wtm-next ui): routes, guard, SSE, HTTP statuses, the embedded front
    daemon/            the run daemon: the jobs of the machine, blind to git, over the ports of its port.go
  web/                 the web front (React + Vite), no Go file; `make ui` builds it into internal/web/dist
  docs/proof/          scripts that run wtm and wtm-next side by side, and their captures
```

## Where does this code go?

| The code… | Goes in |
| -- | -- |
| sequences one command's request: Run, Plan, Apply, Questions, Observe | `command/<name>/usecase.go` |
| decides from facts, pure, for one command (Facts, Decide, Plan, its locks and fingerprint) | `command/<name>/rules.go` |
| is one command's request, its form schema and its validation | `command/<name>/questions.go` |
| is a port one command needs (an interface, or a function type) | `command/<name>/port.go` |
| draws one command's output, or asks one of its fields its own way | `command/<name>/cli/` |
| words one command's fields for the command line: flag, label, help, option labels | `command/<name>/cli/words.go` (`references/questions.md`, "Words") |
| is a pure rule, type or vocabulary two commands need | `domain/<subject>/` (`worktree`, `project`, `env`, `dotenv`, `run`), the root `domain` only for the cross-cutting contract (errors, events, outcomes, forms) |
| orchestrates I/O two commands need | `service/<name>/` (concrete, with its own `port.go`) |
| is a question two commands ask | the fragment of the service that consumes its answers (`service/provision/questions.go`) |
| is presentation two commands need, or any field kind of the wizard | `ui/output`, `ui/form`, an input in `ui/widget` |
| talks to git, a file, a process, a socket | `adapter/<name>/`, behind a port |
| is an in-memory stand-in for an adapter, for tests | `adapter/<name>test/` |
| checks a promise every use case keeps, for tests | `service/<name>test/` (`operationtest.Contract`) |
| serves the catalogue over HTTP: a route, the guard, server-sent events, the status of an error's kind | `web/`, given what it cannot reach as functions in `web.Params` |
| runs, watches, stops or recovers a job's process in the run daemon | `daemon/` (its ports in `daemon/port.go`); its protocol and its client in `adapter/daemon`, the processes in `adapter/process`, the logs in `adapter/joblog`, the proxy in `adapter/proxy` |
| chooses which adapter backs which port | `cmd/wtm-next/wire.go` |
| names another command (a follow-up) | a `domain.FollowUp` of that command's name and answers; its command line is spelled by a function `cmd/wtm-next` injects, the only package that knows every command |

Share on the second user, not before: until a second command needs it, code stays in its slice.

One exception to a pure domain: `run.Session` (`domain/run/stream.go`: a channel, an `io.Writer` and a `Release` func) is a client attached to a job's terminal, which the run daemon's core hands its server across the adapter boundary; neither may import the other, so the domain is the one place they share. Add no other such value.

## The rules

Each rule names what enforces it: `[archlint]` (`tools/archlint`, rule `next` unless another is named), `[revive]`, `[staticcheck]`, `[sumtype]` (`go-sumtype`), `[test]` (a test fails), or `[review]` (nothing but a reviewer catches it: check it yourself).

1. **Imports follow the table of `next/README.md`.** Dependencies point inward; a command never imports another command, a command's `cli` never imports a service or an adapter, only `cmd/wtm-next` imports adapters, nothing imports a fake outside a test, nothing in `next/` imports `internal/`. A package in no row is refused. `[archlint]`
2. **A command is one folder with five parts** — `usecase.go`, `port.go`, `rules.go`, `questions.go`, `cli/` — and nothing else at its root but tests and their `testdata/`. Layers are files, not packages; `cli/` is the one sub-package, so cobra and Bubble Tea stay out of the use case. `[archlint]` (`slicefiles`)
3. **`Run` is the story, in numbered steps, the same in every command (a read: `references/contract.md`):** begin the operation, plan under its locks, fingerprint and stop on a dry run, refuse a stale plan, apply between the started and finished events. A dry run takes no lock and plans once. A command adds a step only where its story differs (extract ends early when nothing is left to extract). `[test]` (`contract_test.go` runs `operationtest.Contract`) and `[review]` for the shape.
4. **A command's codes start with its name:** `<Name>.<snake_case>` (`extract.conflict`), `Name` being the command's constant, declared beside the code that raises them; a code several commands share is a `domain` constant. `[archlint]` (`codeprefix`)
5. **Every command of `main.go` is in `catalogue.go` and wired in `wire.go`, and is driven by the catalogue alone:** its questions, its answers by field, its dry run, its apply held to the fingerprint, its stale plan refused with a replan (a read: its questions, then the read), each JSON shape under a golden. Its `cli/` draws and never decides: a recap, an anomaly or a next step is data of the use case. `[test]` (`cmd/wtm-next/registration_test.go`, `contract_test.go`), `[review]` (no decision in `cli/`)
6. **Ports are consumer-side and small:** a command or a service declares in its `port.go` only the methods it calls, as an interface, or a function type for a single function (`extract.Shield`). A service is concrete, never a port; an adapter exports a concrete type. `[archlint]` (`portfile`: a `port.go` declares only interface and function types), `[review]` (consumer-side and small)
7. **A fake is an adapter's twin** in `adapter/<name>test`: it keeps the adapter's contract, and a parity test runs the same cases on both. `[archlint]` (where it lives, who imports it), `[review]` (the parity)
8. **The process environment is read once, at the edge:** `ui/cli.ReadEnv` (working directory, `$WTM_PROJECT_DIR`, `$WTM_CORRELATION_ID`), `ui/output.System` (the streams and `$TERM`), `$WTM_STATE_DIR` in `cmd/wtm-next`; an adapter reads only what its own tool needs (`os.UserConfigDir`, a child's `os.Environ`). Domain, services and commands never read it: what they need travels in the request, the run options or the wiring. `[archlint]` (`envread`: the stdlib's environment readers, `os.Getenv` to `os.UserConfigDir`, only in `ui/cli`, `ui/output`, `cmd/wtm-next`, `adapter/*`), `[review]` (what an adapter reads)
9. **A value that takes one of a closed set of shapes is a sealed interface** (an unexported method, one type per shape) declared `//go-sumtype:decl` on a line of its own, so every type switch over it is checked: `domain.Field`, `domain.Event`, `worktree.Placement`, a command's `decision`. `[sumtype]`
10. **Every package has a package comment, every exported name a doc comment** that starts with its name and states its contract. `[revive]` (`exported`, `package-comments`) and `[staticcheck]` (`checks = ["all"]`)
11. **A long file reads in named sections:** `usecase.go` (the use case, Run, Plan, Apply, Questions, Observe, the steps of Apply, helpers), `rules.go` (a mutation: facts, the decision of each subject, Decide, the plan, the recap, the outcome; a read: Facts, The detail, then its report under its name: List, Tree, Report) and `questions.go`, each section under go-style's separator (`// ` and 76 dashes, then its title). A file is named after what it declares; a package splits only past about 1,000 lines at a natural seam. `[review]`
12. **Tests share one layout:** testify, only `Test…` functions in a test file, fixtures and fakes in `fixture_test.go`, helpers in `helpers_test.go`, goldens in `testdata/` behind `-update` (`formtest.Golden` in a `cli` test). A test function reads as a sentence (`TestAStaleFingerprintIsRefused`); a subtest name is a short identifier (`existing`, `stale_fingerprint_refused`), never a sentence. A `*Params` struct only for many inputs, two swappable values of one type, or a list that grows (go-style `testing.md`, `readability.md`). `[archlint]` (`testlayout`, `subtestname`), `[review]` (testify, goldens, `*Params`)
13. **The core gives data and decisions, each surface gives its words:** a form carries no label, help, flag nor option label, and an option carries facts, not a badge; the CLI words a field in its `cli/words.go`, the web in its screen's `words.ts`, each exhaustive over what the schema publishes. `[test]` (`formtest.Worded`, the web's typed Records), `[review]` (no copy in `questions.go`)
14. **No legacy gap in limbo:** each difference between `wtm` and `wtm-next` is an owner-approved deviation listed in the PR, or a bug fixed with its test (`references/new-command.md`, step 9). `[review]`

## Read next, by task

| You are… | Read |
| -- | -- |
| adding or migrating a command | `references/new-command.md` |
| adding a question, a flag or a field, or sharing one between commands | `references/questions.md` |
| touching what a command promises: `--yes`/`--force`, dry run and fingerprint, JSON envelope, exit codes, locks, events, a read, a stream | `references/contract.md` |
| adding a port, an adapter, a fake or a test kit | `references/ports-adapters.md` |
| changing the wizard engine (`ui/form`) or a command's own step | `references/form.md` |
| following a request hop by hop, or checking what a package may import | `next/README.md` |

Change the code, `next/README.md` and this skill in the same change when a rule moves.
