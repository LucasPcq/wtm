# next/ — wtm as use-case slices

`wtm-next` (`make next` → `bin/wtm-next`) is wtm rebuilt as plain functions: a pure domain, one package per use case behind an in-process API, adapters behind the small interfaces each use case declares, thin surfaces. It works on the same repositories and state files as `wtm`: same state directory, `meta.json`, `config.toml` keys, operation locks and events socket. Nothing here imports `github.com/LucasPcq/wtm/internal/…`.

## Layout

```
cmd/wtm-next/          wires the adapters into app.New and runs the command line
internal/
  wtm/                 the domain: pure, stdlib only; a subject with logic of its own gets a sub-package (branch, dotenv, envcopy, runenv, hookvars)
  app/                 the API every surface calls: locks, fingerprint, events around each use case
    <slice>/           one use case (create); provision/ is the one shared by several (a new worktree made whole)
  adapter/             what the app reaches out to: git, statefile, store, config, shell, oplock, events, proxyport
    <name>test/        a stateful fake of an adapter, for tests
  surface/             what calls the app: cli, output, form
```

## Import rules

`make lint` checks them (`tools/archlint`, rule `next`).

| Package | May import |
| -- | -- |
| `wtm`, `wtm/*` | stdlib, `wtm/*` |
| `app/<slice>` | `wtm/*`, `app/provision`, stdlib — never another slice, `adapter/*`, `surface/*`, `app` |
| `app` | `app/*`, `wtm/*`, stdlib |
| `adapter/<name>` | `wtm/*`, `adapter/statefile`, `adapter/shell` (the one way to start a child), stdlib, third party — never `app/*`, `surface/*`, another adapter |
| `adapter/<name>test` | what `adapter/<name>` may, and `adapter/<name>`; imported from a `_test.go` only |
| any test | never `github.com/LucasPcq/wtm/internal/…` |
| `surface/*` | `app`, `app/<slice>` (request and result types), `wtm/*`, `surface/*`, third party — never `adapter/*` |
| `cmd/wtm-next` | anything under `next/` |

A slice declares the interfaces it needs in the file of the struct that holds them; an adapter exports a concrete type that satisfies them, checked by the compiler in `cmd/wtm-next`.

## Comments

Close to zero. Each package has a one-line package doc; every other comment line counts, godoc and tests included, except a bare directive (`//go:build`, `//go:embed`, `//go:generate`, `//go-sumtype:decl <Type>`, `//nolint:<linters>`, nothing after). A line earns its place only as a why the code cannot show. `.next-comments` holds each package's count (absent = 0); `make lint` (rule `nextcomments`) refuses a package above it (delete, or raise the count in review) or below it (lower the count).

## Cancellation

- **Every child process runs in its own process group**, git included, and wtm signals the whole group itself: SIGINT, then SIGTERM, then SIGKILL, bounded by `exec.Cmd.WaitDelay`. The terminal's Ctrl+C therefore never reaches a child directly: `git worktree add` would otherwise undo itself on SIGINT while wtm records the worktree, and a hook would receive SIGINT twice.
- **The first Ctrl+C** cancels the operation: the worktree being added finishes its add, env copy and meta record (a shielded section), its hooks are stopped, and the later worktrees are skipped as interrupted. In human mode it prints `Cancelling — finishing <step> of <branch>; Ctrl+C again to stop now` on stderr. The command exits 19.
- **The second Ctrl+C** stops at once: the running children's groups are killed and the command exits 130.
- **A detached caller** (an HTTP or desktop adapter, later) passes `app` its own operation context, never a request's: a client that disconnects does not cancel the operation, an explicit cancel acts as the first Ctrl+C and a forced one as the second.
