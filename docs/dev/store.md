# store — the one writer of each state file

`internal/store/` is the one writer of each state file wtm keeps: a worktree's `meta.json`, the changes to `config.toml` and the `.env` files wtm manages. Each file has one owner here, an interface, its implementation on disk and its double in `testutil/storetest`. `config/` keeps loading, parsing and validating; `store/` writes. Every write is an `fs.UpdateFile` that reads the file under its lock ([state-files.md](state-files.md)).

## The files and their owners

| Interface | On disk | File | Methods |
| -- | -- | -- | -- |
| `store.Meta` | `MetaDisk{StateDir}` | `<common>/wtm/worktrees/<branch>/meta.json` | `Load`, `Records`, `Update`, `Purge` |
| `store.ProjectConfig` | `ConfigDisk{StateDir}` | `<common>/wtm/config.toml` | `Remember` (and `ConfigDisk.Update`) |
| `store.EnvFiles` | `EnvDisk{}` | a worktree's `.env` files | `Read`, `Template`, `Copy`, `Edit` |

`run.toml`, `pending-removals.toml`, the logs and the full re-render of `config.toml` by `init` keep their writers in `config/` and `service/` until their commands migrate. Every render of `config.toml` (`config/wtm.toml.tmpl`, the re-init's and `ReviseProject`'s alike) writes a value only through the template functions `toml` and `tomlKey`, spelt by the TOML encoder, never between literal quotes: a hook command, a path or a remembered answer round-trips whatever quotes, backslashes or newlines it holds.

## What a write says

Every write returns what it did, for the command to report: `store.Written{Changed, Unlocked}` from `Meta.Update`, `ProjectConfig.Remember` (and `ConfigDisk.Update`) and `EnvFiles.Edit`, `store.EnvCopied{Files, Warnings, Unlocked}` from `EnvFiles.Copy`.

- `Changed` is a file rewritten: a write whose content is the one on disk is not, and says so.
- `Unlocked` is a write that ran without its file lock, on a filesystem without `flock` (`fs.Lock.Held` false): the write landed, and the command carries the warning `locks.unavailable` in its outcome.

## `meta.json`: `store.Meta`

```go
meta := store.MetaDisk{StateDir: stateDir}
written, err := meta.Update(ctx, store.MetaUpdate{
	Branch:  branch,
	Missing: store.MissingSkip,
	Change:  func(m *domain.WorktreeMetadata) error { m.Namespaces = append(m.Namespaces, job); return nil },
})
```

- The record of `feat/x` is under `worktrees/feat%2Fx/` (`rules.WorktreeMetaDir`), locked by `meta.json.lock` beside it.
- `Load` reads without the lock: a write renames a whole file over the record. An absent record is an error wrapping `os.ErrNotExist`. A record at a newer schema is read.
- `Records` is every record that reads, sorted by branch, read from disk rather than from the live worktrees: the namespaces a removed worktree still holds are found by it.
- `Update` is a read-modify-write under the lock: `Change` edits the record as it is on disk, so two commands changing different fields both land. A record written at a newer schema is refused (`*statefile.NewerError`), a `Change` that returns an error writes nothing, and a record left unchanged is not rewritten (`Changed` is false).

| `Missing` | Absent record | Unreadable record |
| -- | -- | -- |
| `MissingRefuse` (zero value) | an error wrapping `os.ErrNotExist` | an error |
| `MissingCreate` | `Change` starts from the zero record | `Change` starts from the zero record |
| `MissingSkip` | nothing is written: the main checkout never gets a record | an error |

The zero value refuses: a writer that creates a record says so (`Create`, the ordinal, the isolation, `Adopt`), and one that forgets creates nothing.

- `Purge` drops the record's directory, its lock included: a record left behind would keep its ordinal reserved. A branch that does not name one directory under `worktrees/` purges nothing (`rules.PurgeableMetaDir`).

## `config.toml`: `store.ProjectConfig`

- `Remember(ctx, rules.RememberedChange)` keeps and forgets remembered answers (`[wizard.remembered]`). A change that keeps and forgets nothing writes nothing. A command on the engine calls it from its `Apply` with the answers its `remember` field names (`kernel.RememberPath`); `service/memory` calls it for the commands still on `flow/`.
- `ConfigDisk.Update(ctx, func(*domain.ProjectConfig))` is the targeted change underneath: `config.ReviseProject` parses the file as it is on disk under its lock, without the defaults `config.Load` adds, applies the change and renders it. No default is written in, and no change another process made since the run loaded the config is lost. `relocate` (`base_path`) and `run init` (the `.env` targets) use it.
- Each write puts this binary's `project.schema.json` beside the file. A missing `config.toml` is `domain.ErrConfigNotFound`; one written at a newer schema is refused.

## The `.env` files: `store.EnvFiles`

- `Copy(ctx, envcopy.Request{Strategy, Files, TargetDir, MainWorktreePath, ParentWorktreePath})` provisions a new worktree's files from the source its strategy names (`example`: the template, `main`, `parent` falling back to `main`). Which source that is, is decided in `rules/envcopy`: `Candidates` names the paths whose presence decides it, the store stats each with `fs.Exists` (a path that cannot be stat'ed is an error, never taken for absent), and `Sources` decides from the set present. `EnvCopied` names the files it wrote and what it could not provision, `[]envcopy.Warning`, a sealed union: `NoTemplate`, `SourceMissing`, `ParentFallback`. It never prints them: a command on the engine carries them as `kernel.Warning` in its outcome, and `service/worktree.Create` writes them on stderr for the commands still on `flow/`.
- `Edit(ctx, EnvEdit{Path, Create, Edit})` reads the file under its lock, hands its lines to `Edit` and writes what comes back. Lines are parsed and rendered by `rules.ParseEnv` and `rules.RenderEnv`, so a changed key keeps its place, its comment and its `export`, and the user's lines, comments, order and line endings stay as they were. A file that renders as it was is not rewritten (`changed` is false). An absent file stays absent unless `Create` is set: the value file is created, the committed template never.
- The lock is `fs.ScratchLockPath(path)`, in the temporary directory: no lock file appears in the user's checkout.
- `Read(path)` is a file's lines, none for an absent file; `Template(dir, file)` is the template provisioning copies, the one detection pinned or else the first candidate on disk (`envcopy.TemplateCandidates`, `envcopy.Template`). A layer on the engine reads a `.env` through them, never `os.ReadFile`: `service/runenv` reads the main checkout's own files so.
- `EditEnv` is the decision of `Edit` without the disk, shared with the double.

Every writer of a `.env` goes through `Edit` and computes its change from the lines read under the lock, never from a read made before it. On the engine that is `runenv.Apply` ([services.md](services.md#runenv-the-run-environment-from-what-the-caller-read)); in `service/env`, for the commands still on `flow/`, the ports (`ApplyEnvPorts`), the keys wtm owns (`ApplyOwnedEnv`, `WritePortKeys`), the restore of a verbatim worktree and the sync of `wtm env`.

## Ports

A command on the engine receives these interfaces when it is built (`create.Command(ports)`, [dispatch.md](dispatch.md#ports)). `registry.Production` builds it with `store.MetaDisk{StateDir}` and `store.ConfigDisk{StateDir}`, on the dispatcher's `StateDir`, and `store.EnvDisk{}`. Its tests run on `storetest.Fake` (`internal/testutil/storetest`): the records, `config.toml` and the files in memory, where what its writes change its reads give back, it says what they changed and it refuses what the disk refuses. `FakeParams.Unreadable` holds a `meta.json` that does not read (`Load` fails, `Records` leaves it out, `Update` overwrites it under `MissingCreate` only), `FakeParams.Fail` makes a method fail without running, `Calls()` lists the calls made, and `Config()` and `File(path)` show what it holds. Its writes are never `Unlocked`.

## What the lint holds

- `layers`: `store` imports `config`, `domain`, `infra` and `rules`. `service` and `testutil` may import it; `flow` may not, and reaches it through a service (`service/memory`).
- `stateowner` (`tools/archlint/stateowner.go`): `fs.UpdateFile` and `fs.RemoveDir` are called from `store/` only, so a state file has one owner. The state files not yet in `store/` (`run.toml`, `pending-removals.toml`, `repos.json`, the renders of the inits, the compose patches, `selfupdate`'s `state.json`) are tracked in `.archlint-migrating` until their command moves.
- `writesink`: every write method of the three interfaces is a mutator, inferred from its implementation (`Meta.Update` writes because `MetaDisk.Update` does), so a reader of the engine that calls one is refused ([lint.md](lint.md)).
- `metawriter` (`tools/archlint/metawriter.go`): an exported function of `service/worktree` that reaches `Update` or `Purge` of `store.Meta` (or `MetaDisk`) is in the mutators table.

## Testing

`internal/store` is tested like the kernel: testify, a test file per source file holding only `Test…` functions, the shared helpers in `helpers_test.go`. Concurrency is tested for real, under `-race`: updates of one `meta.json` from goroutines and from processes started from the test binary (`TestHelperProcess`), remembered answers and `.env` edits from goroutines. A record and a `config.toml` without `schema` read as they always did, and a file at a newer schema is refused byte for byte. `contract_test.go` runs the same cases on the disk and on `storetest.Fake`: the records, a record that does not read, the memory, the copy, the reads and the edits end the same, each write says the same change, and both refuse the same calls without changing anything.
