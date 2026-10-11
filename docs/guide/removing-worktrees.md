# Removing worktrees

`wtm clean` removes the worktrees you name, and `wtm prune` removes the ones whose work is done: pull request merged or closed, remote branch deleted. Both stop the worktree's jobs, run its `on_clean` hooks, delete its local branch and give back its data, and both refuse a worktree that still holds work you could lose.

```console
$ wtm clean fix/header --yes

  ✓ Stopped services on fix/header

  Hooks · On Clean

  → echo tearing down fix/header

tearing down fix/header

  ✓ echo tearing down fix/header (2ms)

  ✓ Cleaned worktree and branch fix/header
```

## Cleaning one or several

`wtm clean <branch>...` takes one branch or several. Several are removed one after the other: a failure does not stop the others, and the run exits with the first failure's code. The main checkout is never a candidate. A branch with no worktree is reported as already absent.

Without a branch, `wtm clean` opens a picker where several can be checked, then shows what it will do before anything happens:

```console
$ wtm clean

  Step 1/4 • Select worktrees to clean

  Space to toggle, a to select all, / to filter, enter to confirm, esc to cancel.

▌ [ ] feat/login
  [ ] feat/login-ui
  [ ] fix/header
```

The last step is the confirmation. It lists each worktree and its branch, the children that will move (see [Children of a removed worktree](#children-of-a-removed-worktree)) and the shared-service namespaces that will be dropped. `--yes` skips every question; the branches must then be named, since there is no safe default for which worktree to remove.

## Pruning finished work

`wtm prune` looks at every worktree and selects those whose work is done:

| Flag | Selects a worktree whose… | Read from |
| -- | -- | -- |
| `--merged` | pull request was merged | GitHub, through `gh` |
| `--closed` | pull request was closed without merging | GitHub, through `gh` |
| `--gone` | upstream branch was deleted on the remote | git, after a fetch |

Without a flag, prune takes all three. "Merged" is read from GitHub and never guessed from local commits, so squash and rebase merges are detected. Without an authenticated `gh`, `--merged` and `--closed` match nothing and prune says so; `--gone` still applies. A branch with no pull request, or an open one, is never merged or closed. A branch that was never pushed has no upstream, so it is never gone.

Gone-detection first fetches the worktrees' branches from `origin`, dropping the remote-tracking refs of those deleted there; `--no-fetch` uses what is already fetched. The base branch and the main checkout are always spared.

```console
$ wtm prune --dry-run

  Would prune 1 worktree(s):
  feat/signup  remote branch gone

  Skipped:
  fix/header  dirty — pass --force
```

`--dry-run` previews and changes nothing. In a terminal, prune shows the matches as a checklist, unsafe ones unchecked, then asks to confirm. With `--yes` it removes every safe match and names the rest:

```console
$ wtm prune --yes

  ✓ 1 pruned · 1 skipped
    feat/signup
  ! Skipped fix/header (dirty — pass --force)
```

Each worktree goes through the whole removal (jobs, hooks, removal, data) before the next one starts. Unlike `clean`, the first failure stops the prune: the worktrees before it are gone, it and the ones after it are kept, and the report names where it stopped.

## Safety refusals

A worktree is **unsafe** to remove when it is:

- **locked** (`git worktree lock`);
- **dirty**: uncommitted changes, untracked files included;
- **unpushed**: commits its branch has that `origin/<branch>` does not;
- **under review**: its pull request is still open (read through `gh`).

Under `--yes`, one unsafe worktree refuses the whole `clean` before anything is removed, and the exit code is `1`:

```console
$ wtm clean feat/login fix/header --yes

  ✗ 2 worktree(s) cannot be removed safely, so nothing was removed:
      feat/login has 1 unpushed commit(s)
      fix/header has uncommitted changes
    pass --force to remove them anyway
```

In the interactive confirmation, the same check is shown per worktree, and the choice offered depends on it: when some of the selection is safe, the first option removes only those and keeps the others; when all of it is unsafe, the only way forward is a forced removal.

```console
  ┃  feat/login — 1 commit(s) not pushed to remote
  ┃
  ┃  Will delete 2 worktrees and their branches:
  ┃    feat/login  ~/code/acme.trees/feat-login
  ┃    fix/header  ~/code/acme.trees/fix-header

▌▸ Yes, delete the 1 safe one(s), keep the others
  ────────────────────────────────────────────────
   Yes, force delete all (bypass all checks)
  ────────────────────────────────────────────────
   No, cancel
```

`prune` never refuses a whole run: an unsafe match is skipped and named, as above, and the safe ones go.

## `--force` and `--yes`

The two flags answer different questions. `--yes` is about prompts: it runs unattended and takes the safe default for every decision. `--force` is about safety: it lifts the refusals above, and a job that will not stop no longer blocks the removal. `--force` alone does not skip anything: the confirmation still opens, with the forced removal as its answer. Together they remove without asking and without checking.

```console
$ wtm clean feat/login --yes --force

  ✓ Cleaned worktree and branch feat/login
  ! feat/login-ui still points at the removed parent feat/login — reparent it with `wtm reparent`
```

## Children of a removed worktree

A worktree created `--from` another records it as its parent, which [`wtm sync`](../wtm_sync.md) rebases it onto. When the parent is removed, the child is **orphaned**: it still names a branch that is gone.

- Interactively, `clean` and `prune` ask whether to reparent the children onto their nearest surviving ancestor (the base branch when there is none), or leave them orphaned.
- `--reparent-children` reparents without asking.
- Under `--yes` without it, children are left orphaned, and the report says so. [`wtm reparent`](../wtm_reparent.md) fixes one later.

Only what actually went counts: a parent that was refused or failed keeps its children.

## Jobs, hooks and data

Each worktree is removed in a fixed order:

1. Its jobs (from `wtm run`) are stopped and checked gone. A job that will not stop refuses the removal (`stop them with wtm run down <branch>, or pass --force`).
2. Its `on_clean` hooks run in the worktree, while its files are still there. A failing hook aborts the removal unless the entry sets `continue_on_error`. See [Hooks](configuration.md#hooks).
3. git removes the worktree.
4. Its data is given back: by default, the namespaces it carved out of shared services are dropped. `--keep-data` keeps them; a service that is down defers the drop. The details, `--drop-data` and `pending-removals.toml` are in [What `clean` and `prune` do with the data](shared-services.md#what-clean-and-prune-do-with-the-data).

A failure before step 4 leaves the data where it was.

## The branch

Removing a worktree deletes its local branch. The remote branch is never touched: after a clean, `feat/login` still exists on `origin`.

Without `--force`, the branch is deleted with `git branch -d`, which refuses a branch whose commits are on neither `HEAD` nor its upstream. A branch that was never pushed has no `origin/<branch>` to compare with, so its commits do not count as unpushed and pass the safety check; git then keeps the branch after the worktree is gone, and the run reports `delete branch: git branch -d: … not fully merged`. Delete it with `git branch -D <branch>` once you are sure. With `--force`, the branch is deleted with `git branch -D`.

When the worktree you stand in is removed, the [shell integration](getting-started.md) moves your shell to the main checkout.

For scripts, `wtm clean --yes --output json` returns `{"results": [...], "failed": [...]}` and `wtm prune --yes --output json` returns `pruned`, `skipped`, `reparented`, `orphaned` and `namespaces`; JSON requires `--yes`.
