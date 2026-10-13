# The engine's services — gitwt, identity, runenv, hooks

A command on the engine reaches a worktree through four services, each with one responsibility: `service/gitwt` adds and removes it in git, `service/identity` says who it is, `service/runenv` resolves its run environment, `service/hooks.Execute` runs its hooks. They stand beside `service/worktree`, which keeps serving the commands still on `flow/`, and none of them reads what its caller already read: git through `git.Reader` (`infra/git`, [git.md](git.md)), the state files through `store/` ([store.md](store.md)), `run.toml` loaded once by the caller.

Each function is a **reader** (it reads, never writes, fetches or allocates) or a **mutator** (it writes, and its caller is a command's `Apply`). A service publishes nothing: a mutator says what it changed, the command reports it, and `dispatch` is the one that publishes, through its `Publisher` port ([dispatch.md](dispatch.md#progress-hook-logs-and-events)). None of them imports `kernel/`: what a saga or a progress needs is a plain function the command wraps.

| Package | Its responsibility | Readers | Mutators |
| -- | -- | -- | -- |
| `service/gitwt` | a git worktree's lifecycle: added with its branch, removed with what the add created | — | `Add`, `Remove` |
| `service/identity` | who a worktree is, read from one snapshot; only `Allocate` gives it a number | `Read`, the methods of `Snapshot` | `Allocate` |
| `service/runenv` | a worktree's run environment, from the `run.toml` and the identity it is handed | `JobEnv`, `ComposeProject`, `Env` | — |
| `service/hooks` | a worktree's lifecycle hooks, run in order, with what they print and how they ended | — | `Execute` (and `RunHooks`, for the commands on `flow/`) |

## gitwt: a worktree as a saga step

`Add` and `Remove` take the same `gitwt.Params{Git, Path, Checkout}`, so they are the `Do` and the `Undo` of the saga's `worktree` step:

```go
worktree := gitwt.Params{Git: ports.Git, Path: path, Checkout: git.NewBranch{Name: branch, From: from}}
step := kernel.SagaStep{
	Name: "worktree",
	Do:   func(ctx context.Context) error { return gitwt.Add(ctx, worktree) },
	Undo: func(ctx context.Context) error { return gitwt.Remove(ctx, worktree) },
}
```

`Add` checks no guard: what may be added is decided in `Observe`, from a snapshot read under the operation's locks (`rules/worktree.CreationOf`, below). `Remove` forces the removal, since the steps after the add put files in the worktree, then deletes the branch only when `Checkout` is a `NewBranch`: a branch that existed before the add is never deleted.

## identity: one snapshot, one allocation

`identity.Read(ctx, ReadParams{Git, Meta})` lists the worktrees once and loads the `meta.json` of each live one by `store.Meta.Load`. A record is `Recorded{Meta}`, `Unrecorded` or `Unreadable{Err}`: an unreadable neighbour fails only the question that needs it, never the snapshot. `Observe` reads it once; every question is a method of the `Snapshot`, which never reads again:

| Method | Answers |
| -- | -- |
| `IsMain(branch)` | the main checkout has the branch out |
| `Ordinal(branch)` | its settled number, `domain.MainWorktreeOrdinal` for the main checkout; `domain.ErrOrdinalUnallocated` when it has none, `ErrOrdinalUnreadable` when a neighbour's record cannot be read. It never allocates. |
| `Isolation(branch)` | the recorded choice; isolated without a readable record |
| `Managed(branch)` | wtm recorded its creation (`rules/worktree.Managed`) |
| `NameClash(branch)` | a live worktree whose derived names the branch would share; asked when `run.toml` declares jobs |
| `CheckedOutAt(branch)`, `Occupant(path)` | where a branch is out, which branch a folder holds: the inputs of `rules/worktree.CreationOf` |

`Allocate(ctx, AllocateParams{Git, Meta, StateDir, Branch})` gives the smallest number no live worktree holds and writes it by `store.Meta.Update`. It takes `ordinal.lock` under the state dir and reads the snapshot again under it, git included, since a snapshot read before the lock may predate a worktree another process has since numbered. It returns `Claim{Ordinal, Allocated}`: `Allocated` is the change the command reports, for `dispatch` to publish `worktree.updated`.

## runenv: the run environment from what the caller read

`runenv.Params{ProjectDir, Run, Identity}` carries everything: the `domain.RunConfig` loaded once, and the `Identity{Branch, Ordinal, IsMain, Isolation}` read from the snapshot, the number settled by `Allocate` first, since nothing here allocates. `rules/runenv.CaseOf` decides which environment applies: `Verbatim` (nothing), `Main` (its own compose project, its `[[env]]` keys plain), or `Linked` (offset by its ordinal, names derived from its branch).

- `JobEnv` is what its jobs and hooks learn: identity, offset, declared ports.
- `ComposeProject` is the `COMPOSE_PROJECT_NAME` written into its `.env`.
- `Env(EnvParams{Params, WorktreePath, EnvFiles, Proxy, PublicPort})` is `EnvWrites`, what its `.env` files are brought in line with: the `[[env_port]]` links, the `[[env]]` value links, the keys wtm writes whole, the bases, the offset. The public port is the caller's to ask the daemon.

Its one read is the main checkout's own `.env` files: the compose project they name, the template values a stamped `[[env]]` key goes back to. What it resolves is written by `store.EnvFiles.Edit`; the copy at creation is `store.EnvFiles.Copy`.

## hooks.Execute: lines as progress

`Execute(ctx, ExecuteParams{Hooks, WorkDir, Vars, Env, Output})` runs the hooks one after another, as `RunHooks` does, and prints nothing: each line they print, stdout and stderr alike, goes to `Output`, a hook's unfinished last line flushed when it ends. The command runs it inside the phase it opens and hands it its progress:

```go
kernel.Phase[CreateDetail]{Name: string(domain.HookOnCreate), Run: func(ctx context.Context, detail CreateDetail) (CreateDetail, error) {
	return detail, hooks.Execute(ctx, hooks.ExecuteParams{Hooks: cfg.Hooks.OnCreate, WorkDir: detail.Path, Output: kernel.Report(in.Emit, branch).Output})
}}
```

It writes no log: `dispatch` keeps the phase's lines in `hooks/<phase>-<branch>.log`, and the generic rendering draws the phase. The error is the result: a `hooks.Failure` naming the hook (`domain.ErrHookFailed`, its exit code), `domain.ErrHookStopped` for a hook interrupted, with its lines emitted up to the interruption, or `domain.ErrCancelled` for an interrupt between two hooks.

## Decisions in `rules/<subject>`

What these services decide is pure, in a sub-package named after its subject:

- `rules/worktree`: `CreationOf` decides what a worktree asked for becomes before git adds anything (`Add`, `Unchanged` under `--if-not-exists`, `CheckedOutElsewhere`, `PathTaken`), the branch asked before the folder; `Managed` is a worktree wtm recorded a creation for.
- `rules/runenv`: `CaseOf`, the environment a worktree is given.

The rules they share with `service/worktree` (`AllocateOrdinal`, `KeepsOrdinal`, `WorktreeJobEnv`, `EnvValueWrites`…) are still at the root of `rules/`. They move to their subject with the migration of `create`, which is their first caller on the engine.

## Beside `service/worktree`

The four packages duplicate what `service/worktree` does for the commands on `flow/`, without its reads. The twin of each disappears with its last caller on `flow/`:

| On `flow/` | On the engine |
| -- | -- |
| `infra.CreateWorktree`, the removal in `worktree.Clean` | `gitwt.Add`, `gitwt.Remove` |
| `worktree.EnsureOrdinal` | `identity.Allocate` |
| `worktree.Ordinal`, `IsolationOf`, `IsMain`, `CheckNameFree`, `isManaged` | `identity.Snapshot` and its methods |
| the guards of `worktree.Create` | `rules/worktree.CreationOf` |
| `worktree.BranchEnv`, `ResolveEnvPorts` | `runenv.JobEnv`, `runenv.Env` |
| `worktree.RunCreateHooks` | `hooks.Execute` |

## What the lint holds

The `service` row of `layers` keeps `kernel/` out of the four packages, and `publish` keeps them from publishing; they import no other service, so `serviceEdges` holds no edge for them. `writesink` ([lint.md](lint.md)) reads the table above from the code: a function that reaches a write sink (a write of `store/`, a method of `git.Writer`, a program started by `infra/proc`) is a mutator, any other a reader. A mutator of these four packages is called only from `command/`, another service or `flow/`, never from `query/`, a surface or `dispatch`, and no reader of a command (`Observe`, `Choices`, `Plan`…) reaches one. `writesink_test.go` checks that the code classifies them as the table does. The four are listed in `engineServices`; the services of the next migrations join it, and `chokepoint` keeps the hand-written table of the mutators of the code on `flow/`.

## Testing

The four packages are tested like the kernel: testify, a test file per source file holding only `Test…` functions, the shared helpers in `helpers_test.go`.

- `gitwt`: each add and its remove end in the state they started from, for a new and an existing branch, on a real repository and on `gittest.Fake`.
- `identity`: reading twice calls nothing but `Worktrees` and `Load`; `Allocate` from several processes on a real repository never gives a number twice.
- `runenv`: each case, verbatim, main and linked, on a `RunConfig` given in the test.
- `hooks`: a hook interrupted is reported stopped, its lines emitted up to the interruption.
