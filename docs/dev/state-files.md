# State files — how wtm writes what it keeps

Every file wtm writes goes through `internal/infra/fs`: one locked, atomic read-modify-write, and a schema version on each state file. The user-facing side (what the files are, exit code `22`) is in [`docs/guide/state.md`](../guide/state.md).

## Writing a file

```go
lock, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
	Path: path,
	Mutate: func(current []byte) (fs.Change, error) {
		// current is nil when the file does not exist
		return fs.Write{Data: next}, nil
	},
})
```

`UpdateFile` locks the file, reads it, hands the content to `Mutate`, then carries out what `Mutate` decides. A write goes to a temporary file beside the target, which is synced and renamed over it; the directory is synced after. A reader sees the old content or the new one, never half a file.

| `Mutate` returns | `UpdateFile` |
| -- | -- |
| `fs.Write{Data}` | writes `Data`; content equal to what is there is not rewritten. A new file takes `Perm` (`0644` when zero); an existing one keeps its mode |
| `fs.Keep{}` | leaves the file as it is, an absent one included |
| `fs.Remove{}` | deletes the file, under the same lock |
| an error | writes nothing and returns it |

`fs.Change` is a sealed union (`//go-sumtype:decl Change`): a decision about the file is one of three shapes, never a byte slice whose `nil` means something.

**Read inside `Mutate`.** The lock only prevents a lost update if the change is computed from `current`. A writer that renders a value it loaded earlier is atomic but can still overwrite another process's change: `run.toml` (`config.WriteRun`), the full re-render of `config.toml` (`config.WriteProjectConfig`, by a re-init only). The read-modify-write writers are `updateMetadata` (`meta.json`), `config.UpdateProjectConfig` (every other change to `config.toml`: the remembered answers, `base_path` from `relocate`, the `.env` targets from `run init`), `config.UpdatePendingRemovals` and `infra.UpdateRegistry` (`repos.json`).

**`ctx` bounds the wait for the lock, nothing else.** A free lock is taken even once `ctx` is done: a write that records work already carried out (a namespace created, a removal owed) must not be lost to an interrupt. A caller writing such a record after the interruptible part of its run passes `context.WithoutCancel(ctx)`, so it waits for a busy lock too.

## Where the lock lives

| File | Lock | Why |
| -- | -- | -- |
| wtm's own state (`.git/wtm/…`, the global directory) | `<path>.lock`, beside it | visible next to what it guards |
| a file in the user's tree (`.env`, a compose file) | `LockPath: fs.ScratchLockPath(path)` | in the temporary directory, named after the absolute path: no lock file in the user's checkout |
| `jobs.json` | none: `fs.ReplaceFile` | the daemon is its only writer, under `wtm.lock` |

A lock that cannot be taken (no `flock` on NFS or some WSL mounts, an unwritable directory) does not cancel the write: it runs unprotected and the returned `fs.Lock{Held: false}` says so, which `dispatch` reports as the warning `locks.unavailable`. `fs.WithFileLock` is the same lock around any work (the ordinal allocation's `ordinal.lock`).

These locks are short: one file, one write. The **operation locks** `dispatch` holds for a whole run (`<common>/wtm/locks/`) are another set, taken with `fs.TryLocks` (all or none, never waiting) beside a holder file written by `fs.HoldFile`: see [dispatch.md](dispatch.md#operation-locks).

## Schema versions

| File | Constant (`domain/statefile`) | Read by |
| -- | -- | -- |
| `config.toml` | `ProjectConfigSchema` | `config.Load`, `config.LoadProjectRaw` |
| global `config.toml` | `GlobalConfigSchema` | `config.LoadGlobal` |
| `run.toml` | `RunConfigSchema` | `config.LoadRun` |
| `meta.json` | `MetaSchema` | `service/worktree` (`loadMetadata`) |
| `pending-removals.toml` | `PendingRemovalsSchema` | `config.LoadPendingRemovals` |

- A file without `schema` is at 1. The key is written only from schema 2 on (`statefile.Written`): the TOML loaders of earlier releases reject an unknown key, so a file written at schema 1 stays readable by them.
- Every writer of a versioned file checks first, inside `Mutate`: `statefile.Refusal(statefile.Schema{Path, Found, Supported})` returns a `*statefile.NewerError` when the file is newer than this binary. It carries the schema as data, for an interface to present, and unwraps to `statefile.ErrSchemaNewer`, which `rules.ExitCode` maps to `22`. `dispatch` presents it as `412 state.schema_newer`.
- Reading a newer file is not an error. The CLI warns on loading `config.toml` and `run.toml` (`shared.WarnSchemaNewer`, rendered by `render.SchemaNewer`); `meta.json` and `pending-removals.toml` are only refused on write.
- `jobs.json` keeps its own `version` (`rules.ClassifyIndexVersion`): a newer index freezes the daemon's store.

**Raising a file's schema** is a change of format that an older binary must not overwrite: bump its constant, make the loader read both versions, and write the new one. The migration happens on write, by the newer binary only. Add a test that the previous schema still loads, and update the JSON schema in `internal/schemas/` (each declares `schema` already).

## What the lint holds

`statewrite` (`tools/archlint/statewrite.go`) refuses `os.WriteFile`, `os.Create` and `os.Rename`, as a call or a function value, outside `infra/`. Its permanent exceptions are in `stateWriteExempt`, each with its reason; a log is opened through `fs.OpenLog`, and the one still opened with `os.Create` (the `exec` log) is tracked in `.archlint-migrating` until it moves (see [lint.md](lint.md)). `metawriter` holds `updateMetadata` as the one sink of `meta.json`.

## Testing

`internal/infra/fs` is tested like the kernel: testify, a test file per source file holding only `Test…` functions, the shared helpers in `helpers_test.go`. Concurrency is tested for real — goroutines, and processes started from the test binary (`TestHelperProcess`) — under `-race`. A writer built on `UpdateFile` tests its own refusal of a newer schema: the file at schema N+1 is read, the write returns `statefile.ErrSchemaNewer`, the file is byte for byte unchanged (`internal/config/statefile_test.go`, `internal/service/worktree/metadata_test.go`).
