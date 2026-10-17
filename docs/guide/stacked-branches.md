# Stacked branches

Every worktree wtm creates records its **parent**: the branch it started from, and the one `wtm sync` rebases it onto. Branches built on each other form a stack, and this page covers working one: reading it with `wtm tree`, rebasing it with `wtm sync`, moving a branch under another parent, and what happens to the children when a parent is merged.

```bash
wtm create feat/login --yes
wtm create feat/login-ui --from feat/login --yes
wtm create fix/header --yes
```

```console
$ wtm tree

  main
  ├─ feat/login
  │  └─ feat/login-ui
  └─ fix/header
```

Without `--from`, the parent is the base branch. The parent is stored as `source_branch` in the worktree's [`meta.json`](state.md#metajson); for a branch that already exists, `--from` names the parent to record rather than a starting point (see [the source branch](creating-worktrees.md#the-source-branch)).

## Reading the tree

`wtm tree` draws parents above their children and adds, after each branch, the badges that say what the stack needs:

```console
$ wtm tree

  main
  ├─ feat/login        base ↑2  origin ↓1  ⚠ needs sync
  │  └─ feat/login-ui  base ↑1  ⚠ needs sync
  └─ fix/header        ⚠ dirty  ⚠ needs sync
```

| Badge | Meaning |
| -- | -- |
| `base ↑N` | commits ahead of the base branch |
| `origin ↑a ↓b` | ahead of / behind `origin/<branch>`; both arrows mean the two have diverged |
| `⚠ needs sync` | the parent's tip is not in the branch any more: the parent moved, a rebase is due |
| `⚠ dirty` | uncommitted changes |
| `⚠ rebasing` | a rebase is paused in that worktree |
| `(no worktree)` | a parent with no worktree, drawn as a root so its children stay attached |
| `▶ N running` | jobs of [`wtm run`](how-run-works.md) running in that worktree |

`--with-prs` adds each branch's pull request and whether it is open, merged or closed, read through the GitHub CLI (`gh`). `--output mermaid` prints the stack as a flowchart to paste into a PR description, and `--output json` gives the structured tree to a script.

`wtm tree` never fetches. The `origin` badges count against the remote-tracking refs as the last fetch left them: the `origin ↓1` above, a teammate's push to `feat/login`, appeared only after a `git fetch`. In `wtm ui`, `r` fetches and redraws them.

## Syncing a stack

`wtm sync` rebases worktrees onto their parents, parents before children, so each child lands on its parent's new tip. It first fetches the base branch and fast-forwards it, then, for each branch, fast-forwards it to its own `origin/<branch>` if that has moved, then rebases it. `--dry-run` shows the plan and changes nothing:

```console
$ wtm sync --all --dry-run

  Sync plan
  1.  feat/login ← main
  2.  fix/header ← main
  3.  feat/login-ui ← feat/login

  Base  main  ed7d4ee  (already up to date / no fast-forward)

  ✓ feat/login rebased onto main   3 commits
  ! fix/header skipped — uncommitted changes (descendants skipped)
  ✓ feat/login-ui rebased onto feat/login   1 commits

  2 branch(es) ready to push (force-with-lease).
```

Name the branches to sync (`wtm sync feat/login feat/login-ui`), pass `--all`, or give no argument to pick them. A worktree with uncommitted changes, a rebase already in progress, or a branch diverged from `origin/<branch>` is skipped, and so are its descendants.

The rebase is local. Interactively, sync then asks whether to force-push (with lease) the rebased branches. `--yes` never pushes: `--push` does, `--no-push` never does, whatever the mode.

```console
$ wtm sync feat/login feat/login-ui --yes --push

  = feat/login already up to date
  = feat/login-ui already up to date

  ✓ Pushed 2 branch(es) (force-with-lease): feat/login, feat/login-ui
```

A parent that no step covers — a branch with no worktree, or one left out of the selection — is never rebased. When it is behind its remote, sync offers to fast-forward it first; `--ff-parents` does it without asking, `--no-ff-parents` rebases onto it as it is, and `--yes` alone leaves it as it is.

### Conflicts

On a conflict, sync aborts that branch's rebase, leaves its worktree clean, names the files and skips its descendants. The exit code is 1.

```console
$ wtm sync feat/login feat/login-ui --yes

  ✗ feat/login conflict on main — aborted, tree clean (1 file); descendants skipped
        login.txt
  ! feat/login-ui skipped — ancestor feat/login was not synced
```

`--keep-conflict` leaves the rebase in progress instead, to resolve in the worktree:

```console
$ wtm sync feat/login feat/login-ui --yes --keep-conflict

  ✗ feat/login conflict on main — left in progress (1 file); descendants skipped
  ! feat/login-ui skipped — ancestor feat/login was not synced

  Conflicts left in progress — resolve, then continue the rebase:

  feat/login  /code/acme.trees/feat-login
     conflicting:
        login.txt
     → resolve, then: git rebase --continue    (abort: git rebase --abort)
```

A rebase finished by hand rewrites the parent's commits with the resolution in them, and two things follow. The next sync skips that branch as diverged from `origin/<branch>`: push it yourself with `git push --force-with-lease`. And its children, rebased in a later run, replay the parent's commits as they were before the resolution, so the same file conflicts again in the child: run that sync with `--keep-conflict` and `git rebase --skip` the parent's commit that conflicts.

## Fast-forwarding to origin

`wtm fast-forward` (alias `ff`) brings worktrees up to `origin/<branch>` and does nothing else: no rebase onto the parent, no merge. It fetches the branches it advances, so it does not depend on the last fetch.

```console
$ wtm fast-forward feat/login --yes

  ✓ 1 fast-forwarded
    feat/login
```

A branch that has diverged from origin is refused, and `--force` does not lift that: replaying local commits is sync's job. A worktree with uncommitted changes is refused too; `--force` fast-forwards it anyway, and git still refuses when a modified file would be overwritten.

## Changing a parent

`wtm reparent` changes the recorded parent and nothing else; the rebase happens on the next sync. The new parent must exist locally or as `origin/<branch>`, and the chain must not loop.

```console
$ wtm reparent feat/login-ui --to main --yes

  ✓ Reparented feat/login-ui: feat/login → main
  → wtm sync feat/login-ui   rebase onto the new parent
```

With no argument, it lets you pick the worktrees and the new parent.

## When a parent is merged

`wtm prune` removes the worktrees whose work is done: pull request merged or closed, read from GitHub through `gh` (so squash and rebase merges count), or remote branch deleted. Its children stay, and `--reparent-children` moves each one onto its nearest surviving ancestor:

```console
$ wtm prune --gone --yes --reparent-children

  ✓ 1 pruned · 1 reparented
    feat/login-ui
    feat/login-tests → main
```

Interactively, prune asks whether to reparent the children. With `--yes` and no `--reparent-children`, they are left **orphaned**: they still point at the removed branch, and `wtm tree` draws it as a `(no worktree)` root above them. `wtm clean` behaves the same and says so:

```console
$ wtm clean feat/login --yes

  ✓ Cleaned worktree and branch feat/login
  ! feat/login-ui still points at the removed parent feat/login — reparent it with `wtm reparent`

$ wtm tree

  main
  └─ fix/header     base ↑1  ⚠ needs sync
  feat/login        (no worktree)
  └─ feat/login-ui  base ↑4
```

`wtm reparent feat/login-ui --to main` attaches an orphan again. Removal itself, its safety checks and `--force` are covered in [removing worktrees](removing-worktrees.md).

After a squash merge, the child still carries its old parent's commits, while `main` has them as one squashed commit. Its next sync replays them onto `main` and can stop on a conflict; with `--keep-conflict`, `git rebase --skip` each commit that belonged to the parent and git keeps only the child's own.
