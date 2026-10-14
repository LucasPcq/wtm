# git — how the engine reaches a repository

`internal/infra/git` is the one way the engine's layers reach git: a repository and its remote, read and written by plain calls that decide nothing. The commands still on `flow/` keep the git functions at the root of `internal/infra/`, which shrink as their commands migrate.

## The repository

```go
repo := git.Repo{Dir: dir, Remote: domain.OriginRemote}
worktrees, err := repo.Worktrees(ctx)
```

- Every call is `git -C <Dir> …`, started by `proc.Command` (so an interrupt settles it like any process wtm runs). An empty `Dir` is the current directory.
- The variables that scope git to one repository (`GIT_DIR`, `GIT_WORK_TREE`… `domain.GitRepoScopedEnv`) are dropped from git's environment: inherited from a hook, they would win over `-C`.
- The remote is handed in, never spelled by a caller: a branch is a `git.Ref`, `LocalBranch{Name}` or `RemoteBranch{Name}`, which the `Repo` turns into `refs/heads/<name>` or `refs/remotes/<Remote>/<name>`.
- Nothing is decided here: no default base branch, no conflict policy, no `Shield` (the saga that writes holds it). What chooses goes in `rules/<subject>` or `service/`.

## Reading: `git.Reader`

| Method | git | Returns |
| -- | -- | -- |
| `Worktrees` | `worktree list --porcelain -z` | `[]Worktree`, the main one first: `Path`, `Head`, `Main`, `Locked`, `Prunable`, and `State` |
| `MainWorktree` | — | the first of `Worktrees`, with no call of its own |
| `Branches` | `for-each-ref refs/heads refs/remotes/<Remote>` | `Branches{Local, Remote}`, named without their prefix, without `<Remote>/HEAD` |
| `RefExists(Ref)` | `show-ref --verify --quiet` | exit `1` is an absent ref; any other failure is an error, never an absence |
| `AheadBehind{Of, Against}` | `rev-list --left-right --count Of...Against` | the commits `Of` has that `Against` lacks, and the reverse |

`State` is a sealed union (`//go-sumtype:decl State`): `OnBranch{Branch}`, `Rebasing{Branch}`, `Detached{}`, `Bare{}`. A rebase in progress detaches `HEAD`, and `git worktree list` never names its branch: `Worktrees` reads it from the worktree's admin directory (`rebase-merge/head-name` or `rebase-apply/head-name`), with no further git call. `-z` keeps any byte of a path but NUL, a space or a newline included; it needs git 2.36.

## Writing: `git.Writer`

| Method | git |
| -- | -- |
| `AddWorktree{Path, Checkout}` | `NewBranch{Name, From}`: `worktree add --no-track -b <Name> <Path> <From>`; `ExistingBranch{Name}`: `worktree add <Path> <Name>` |
| `RemoveWorktree{Path, Force}` | `worktree remove [--force] <Path>` |
| `DeleteBranch{Name, Force}` | `branch -d` or `-D` |
| `Fetch` | `fetch <Remote>`: every branch, for a branch picker's refresh |
| `FetchRef(branch)` | `fetch <Remote> <branch>`: the remote branch moves, the local one stays |
| `FastForwardRef(branch)` | `fetch <Remote> <branch>:<branch>`: a branch checked out nowhere; git refuses anything but a fast-forward |
| `MergeFFOnly{Worktree, Onto}` | `git -C <Worktree> merge --ff-only <Onto>`: a branch a worktree has checked out |

A new branch is born with its worktree: `AddWorktree` with `NewBranch` is the whole `worktree` step of a saga, and there is no separate branch creation to undo. Its undo is `RemoveWorktree`, then `DeleteBranch` only when the step created the branch: the pair is `service/gitwt` ([services.md](services.md)). `Checkout` is a sealed union: the start point exists only for a new branch.

## Errors

A failed call returns a `*git.Error{Args, Exit, Stderr, Err}`: the arguments, git's exit status (`-1` when git did not run to one: missing from `PATH`, killed) and what it wrote to stderr. It unwraps to the `*exec.ExitError`. No call turns an error into `0`, `""` or `false`.

## Ports

A command receives the `Reader` and the `Writer` when it is built (`create.Command(ports)`, [dispatch.md](dispatch.md#ports)): `registry.Production` builds it with `git.Repo{Dir: <the dispatcher's Dir>, Remote: domain.OriginRemote}`. Its tests run on `gittest.Fake` (`internal/testutil/gittest`), git in memory: what its writes change its reads give back, it refuses what git refuses, `FakeParams.Fail` makes a method fail without running, and `Calls()` lists the calls made.

## What the lint holds

`gitexec` (`tools/archlint/gitexec.go`) refuses a program started as `"git"` (`exec.Command`, `exec.CommandContext`, `proc.Command`, `proc.GroupCommand`) outside `infra/git`. Its one permanent exception is `testutil`, whose helpers build real repositories. The files at the root of `infra/` that still run git are in `.archlint-migrating`, each with its count of calls, and leave it with their last caller. `writesink` (`tools/archlint/writesink.go`) takes every method of `git.Writer`, on the interface and on `git.Repo`, as a write sink: a function that calls one is a mutator, one that calls only `git.Reader` stays a reader ([lint.md](lint.md)).

## Testing

`internal/infra/git` is tested like the kernel: testify, a test file per source file holding only `Test…` functions, the shared helpers in `helpers_test.go`. Every method runs on real repositories (`gittest.InitRepo`, `gittest.AddOrigin`, a clone to advance the remote), the delicate porcelain included: a detached `HEAD`, a rebase in progress with either backend, in the main or a linked worktree, a path with a space, a locked and a prunable worktree, a bare repository. `contract_test.go` runs the same cases on a real repository and on `gittest.Fake`: an add and its undo end in the state they started from, and both refuse the same calls without changing anything.
