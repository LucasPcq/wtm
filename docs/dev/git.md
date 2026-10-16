# git — how the engine reaches a repository

`internal/infra/git` is the one way the engine's layers reach git: a repository and its remote, read and written by plain calls that decide nothing. The commands still on `flow/` keep the git functions at the root of `internal/infra/`, which shrink as their commands migrate ([twins](#twins-on-flow-and-on-the-engine)).

## The repository

```go
repo := git.Repo{Dir: dir, Remote: domain.OriginRemote}
worktrees, err := repo.Worktrees(ctx)
```

- Every call is `git -C <Dir> …`, started by `proc.Command` (so an interrupt settles it like any process wtm runs). An empty `Dir` is the current directory.
- The variables that scope git to one repository (`GIT_DIR`, `GIT_WORK_TREE`… `domain.GitRepoScopedEnv`) are dropped from git's environment: inherited from a hook, they would win over `-C`.
- The remote is handed in, never spelled by a caller: a branch is a `git.Ref`, `LocalBranch{Name}` or `RemoteBranch{Name}`, which the `Repo` turns into `refs/heads/<name>` or `refs/remotes/<Remote>/<name>`.
- Nothing is decided here: no default base branch, no conflict policy, no `Shield` (the saga that writes holds it). What chooses goes in `rules/<subject>` or `service/`: what a detached `HEAD` is, for one, is `rules/worktree.RebasingBranch`.
- A path is compared as git writes it: git records a worktree with its symlinks resolved (`/private/tmp/…` for `/tmp/…` on macOS). A path a command builds is read through `Locate` before it is compared with one git returned.

## Reading: `git.Reader`

| Method | git | Returns |
| -- | -- | -- |
| `Worktrees` | `worktree list --porcelain -z` | `[]Worktree`, the main one first, the linked ones sorted by path: `Path`, `Head`, `Main`, `Locked`, `Prunable`, and `State` |
| `MainWorktree` | — | the first of `Worktrees`, with no call of its own |
| `Toplevel` | `rev-parse --show-toplevel` | the root of the worktree `Dir` is in, as git writes it |
| `GitCommonDir` | `rev-parse --path-format=absolute --git-common-dir` | the `.git` every worktree shares |
| `Locate(path)` | — (the disk) | `Location{Path, Exists}`: the path as git would record it (the symlinks of the part that exists resolved, the rest as written; a relative path read from `Dir`), and whether anything is there |
| `Branches` | `for-each-ref refs/heads refs/remotes/<Remote>` | `Branches{Local, Remote}`, each a `Branch{Name, Tip}` named without its prefix, without `<Remote>/HEAD`: one call says which branches exist and where each points |
| `Dirty` | `status --porcelain` in the worktree | whether it holds a change not committed, untracked files included; an error for a path that is no worktree |
| `RefExists(Ref)` | `show-ref --verify --quiet` | exit `1` is an absent ref; any other failure is an error, never an absence |
| `Tip(Ref)` | `rev-parse --verify` | the commit the ref points at; an absent ref is an error |
| `AheadBehind{Of, Against}` | `rev-list --left-right --count Of...Against` | the commits `Of` has that `Against` lacks, and the reverse |

`State` is a sealed union (`//go-sumtype:decl State`): `OnBranch{Branch}`, `Rebasing{Branch}`, `Detached{}`, `Bare{}`. A rebase in progress detaches `HEAD`, and `git worktree list` never names its branch: `Worktrees` reads the worktree's admin directory (`rebase-merge/head-name` or `rebase-apply/head-name`), with no further git call, and `rules/worktree.RebasingBranch` says what it means (a worktree whose directory is gone is merely detached). `-z` keeps any byte of a path but NUL, a space or a newline included; it needs git 2.36.

## Writing: `git.Writer`

| Method | git | Returns |
| -- | -- | -- |
| `AddWorktree{Path, Checkout}` | `NewBranch{Name, From}`: `worktree add --no-track -b <Name> <Path> <From>`; `ExistingBranch{Name}`: `worktree add <Path> <Name>`, once `RefExists` found the local branch; `TrackingBranch{Name}`: `worktree add --track -b <Name> <Path> <Remote>/<Name>`, a branch only the remote has, resumed | — |
| `RemoveWorktree{Path, Force}` | `worktree remove [--force] <Path>` | — |
| `DeleteBranch{Name, Force}` | `branch -d` or `-D` | — |
| `Fetch` | `fetch <Remote>`: every branch, for a branch picker's refresh | — |
| `FetchRef(branch)` | `fetch <Remote> <branch>`: the remote branch moves, the local one stays | — |
| `FastForwardRef(branch)` | `fetch <Remote> <branch>:<branch>`: a branch checked out nowhere, created when absent | `Move{From, To}` |
| `MergeFFOnly{Worktree, Onto}` | `git -C <Worktree> merge --ff-only <Onto>`: a branch a worktree has checked out | `Move{From, To}` |

`Move` is where a fast-forward took the branch, read by `rev-parse` before and after: `From` is empty for a branch `FastForwardRef` created, and equals `To` when there was nothing to bring.

A new branch is born with its worktree: `AddWorktree` with `NewBranch` is the whole `worktree` step of a saga, and there is no separate branch creation to undo. Its undo is `RemoveWorktree`, then `DeleteBranch` only when the step created the branch: the pair is `service/gitwt` ([services.md](services.md)). `Checkout` is a sealed union: the start point exists only for a new branch. `git.CreatedBranch(checkout)` names the branch an add creates (`NewBranch`, `TrackingBranch`), the one its undo deletes.

What git does that a caller would not guess, each held by a test on real git:

- `worktree add <path> <name>` on a branch only the remote has creates a local branch tracking it, and no flag or setting stops it (`--no-guess-remote` only covers an add without a branch). `AddWorktree` with `ExistingBranch` refuses that case with `git.ErrNoLocalBranch` instead; a remote branch is resumed as `TrackingBranch`, or started from as `NewBranch{From: RemoteBranch}`.
- `worktree add -b` creates the branch before it looks at the path: refused for a folder that holds files, or for a worktree whose directory is gone, it leaves the branch. `gitwt.Add` deletes it.
- `worktree remove --force` keeps a locked worktree (git wants `--force` twice); a worktree whose directory is gone is removed without `--force`.
- `fetch <Remote> b:b` refused as no fast-forward still moves `<Remote>/b`.

## Errors

A failed call returns a `*git.Error{Args, Exit, Stderr, Err}`: the arguments, git's exit status (`-1` when git did not run to one: missing from `PATH`, or interrupted) and what it wrote to stderr. It unwraps to the `*exec.ExitError`, or to the context's error when the call was interrupted, so `errors.Is(err, context.Canceled)` holds. No call turns an error into `0`, `""` or `false`.

## Ports

A command receives the `Reader` and the `Writer` (or both, `git.ReadWriter`) when it is built (`create.Command(ports)`, [dispatch.md](dispatch.md#ports)): `registry.Production` builds it with `git.Repo{Dir: <the dispatcher's Dir>, Remote: domain.OriginRemote}`.

Its tests run on `gittest.Fake` (`internal/testutil/gittest`), git in memory: what its writes change its reads give back, it refuses what git refuses, `FakeParams.Fail` makes a method fail without running, and `Calls()` lists the calls made. `FakeParams` is the seed:

| Field | The repository starts with |
| -- | -- |
| `Main`, `Branch`, `Dir` | the main worktree on `Branch`, seen from `Dir` (`Main` when empty) |
| `Local`, `Remote` | more local branches; the remote's branches, fetched |
| `Worktrees` | linked worktrees, each `OnBranch`, `Rebasing` or `Detached`, `Locked` or `Prunable` (its directory gone); a branch one has out is a local branch |
| `Folders`, `Dirty` | folders holding files and no worktree; worktrees with changes, which a removal keeps without `Force` |
| `Commits` | a branch's commits past the root, oldest first; two branches share the commits they name alike |
| `Origin` | what the remote has of a branch and was not fetched yet: `Fetch`, `FetchRef` and `FastForwardRef` bring it |

A branch is its history, so `Tip`, `AheadBehind`, a fast-forward and `branch -d` (merged into the `HEAD` of `Dir`) follow the writes. A path is resolved as git resolves it: through the disk, for its symlinks only. A merge on a detached `HEAD` is not modelled and fails.

`gittest.Build(t, params)` makes a real repository from the same `FakeParams`, with a bare origin beside it and empty commits named by their message: a contract test declares its seed once and runs on both. `gittest.StateOf` is what a test compares before and after (the worktrees, `Head` left out, and the branches); `gittest.TempDir`, `Resolved` and `Symlinked` give paths as git writes them, or as it never does.

## Twins on `flow/` and on the engine

Each function at the root of `infra/` leaves with its last caller on `flow/`; its twin on the engine is:

| On `flow/` (`internal/infra`) | On the engine (`git.Repo`) |
| -- | -- |
| `ListWorktrees` | `Worktrees` |
| `FindMainWorktreePath` | `MainWorktree` |
| `Toplevel` | `Toplevel` |
| `GitCommonDir` | `GitCommonDir` |
| `ListLocalBranches`, `ListRemoteBranches` | `Branches` |
| `LocalBranchExists` | `RefExists(LocalBranch)` |
| `AheadBehind` | `AheadBehind` |
| `CreateWorktree` | `AddWorktree` |
| `RemoveWorktree` | `RemoveWorktree` |
| `DeleteLocalBranch` | `DeleteBranch` |
| `Fetch` | `Fetch` |
| `FetchBranch` | `FetchRef` |
| `FastForwardRef` | `FastForwardRef` |
| `FastForwardBranch` | `MergeFFOnly` |

## What the lint holds

`gitexec` (`tools/archlint/gitexec.go`) refuses a program started as `"git"` (`exec.Command`, `exec.CommandContext`, `proc.Command`, `proc.GroupCommand`) outside `infra/git`. Its one permanent exception is `testutil`, whose helpers build real repositories. The files at the root of `infra/` that still run git are in `.archlint-migrating`, each with its count of calls, and leave it with their last caller. `writesink` (`tools/archlint/writesink.go`) takes every method of `git.Writer`, on the interface and on `git.Repo`, as a write sink: a function that calls one is a mutator, one that calls only `git.Reader` stays a reader ([lint.md](lint.md)).

## Testing

`internal/infra/git` is tested like the kernel: testify, a test file per source file holding only `Test…` functions, the seed in `fixture_test.go`, the shared helpers in `helpers_test.go`. Every method runs on real repositories (`gittest.InitRepo`, `gittest.AddOrigin`, a clone to advance the remote), the delicate porcelain included: a detached `HEAD`, a rebase in progress with either backend, in the main or a linked worktree, a path with a space, a locked and a prunable worktree, a bare repository, a path through a symlink. `contract_test.go` runs every case on `gittest.Build` and `gittest.NewFake` from one seed reached through a symlink: both read the same repository, an add and its undo end where they started, a fast-forward, a fetch and a removal move the same refs, and both refuse the same calls, changing nothing, except the branch `worktree add -b` leaves, which both leave.
