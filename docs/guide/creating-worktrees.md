# Creating worktrees

A worktree comes to exist in one of three ways: `wtm create` starts it from a branch, `wtm checkout` from a pull request, and `wtm extract` carves it out of another worktree's uncommitted changes. All three place it, provision its `.env` files, record its parent and isolation, and run the `on_create` hooks the same way. [Getting started](getting-started.md) shows a first `create`; this page covers the variations.

```console
$ wtm create feat/login --yes

  ✓ Created worktree feat/login

  from  main
  env   example · 3 ports settled (offset +10)
  path  ../acme.trees/feat-login

  → wtm go feat/login
```

## What a creation does

1. **Places the worktree** under `base_path` from `config.toml`, in a directory named after the branch with `/` replaced by `-`: `feat/login` lands in `../acme.trees/feat-login`. `wtm go feat/login` moves there.
2. **Creates or reuses the branch.** A new branch starts from the source (the `from` line). A branch that already exists locally is checked out as-is, keeping its commits.
3. **Provisions the `.env` files** with the repository's env strategy (`example`, `main` or `parent`), or `--env-from` for this worktree only. The strategies are in [Configuration](configuration.md#env-strategies).
4. **Settles its ports** when `run.toml` links `.env` keys to ports: "3 ports settled (offset +10)" means the copied values were moved to this worktree's own ports. See [How `wtm run` works](how-run-works.md).
5. **Records** the source branch, env strategy and isolation in its `meta.json` ([Where wtm keeps its state](state.md)).
6. **Runs the `on_create` hooks** from the new worktree, after the `.env` is in place so a hook can read it:

```console
$ wtm create feat/search --yes

  Hooks · On Create

  ✓ echo installing in feat/search (2ms)

  ✓ Created worktree feat/search
```

A hook that exits non-zero fails the command (exit 1) but leaves the worktree in place, so you can fix the cause and run the step by hand, or remove it with `wtm clean`. Hook syntax, `continue_on_error` and the log of the last run are in [Configuration](configuration.md#hooks).

## The source branch

Without `--from`, a new branch starts from `base_branch` (`main` here). `--from` names another source, any local branch:

```console
$ wtm create feat/login-ui --from feat/login --yes

  ✓ Created worktree feat/login-ui

  from  feat/login
  env   example · 3 ports settled (offset +40)
  path  ../acme.trees/feat-login-ui
```

The source is also the parent wtm records: `wtm sync` rebases the branch onto it, and `wtm tree` draws it underneath. A chain of branches built this way is a stack, covered in [Stacked branches](stacked-branches.md).

`--ff` fast-forwards the source to `origin` before branching from it, and is skipped when the two have diverged. The wizard offers the same when the source is behind.

## A branch that already exists

When the branch exists locally, `create` checks it out instead of creating it. Nothing tells wtm what it was branched from, so it does not guess: `--from` is required without the wizard.

```console
$ wtm create fix/typo --yes

  ✗ fix/typo already exists locally: pass --from to record its parent branch (it can't be inferred, and `wtm sync` needs it)

$ wtm create fix/typo --from main --yes

  ✓ Created worktree fix/typo on existing branch

  parent  main
  env     example · 3 ports settled (offset +50)
  path    ../acme.trees/fix-typo

  Reused your existing local branch fix/typo.
```

A branch already checked out in another worktree is refused with exit code 10, since git allows a branch in one worktree only. `--if-not-exists` turns that case into a success that points at the existing worktree, which makes `create` safe to retry from a script:

```console
$ wtm create feat/login --if-not-exists --yes

  = Worktree feat/login already exists at ../acme.trees/feat-login
```

`create` starts a new branch from the local source, never from `origin`: to work on a branch that exists only on the remote, fetch it first (`git fetch origin feat/x:feat/x`) and `create` reuses it, or use `wtm checkout` when it has a pull request.

## Several branches at once

`create` takes several branches. They share one source and one set of answers, and are created one after the other; a failure does not stop the others.

```console
$ wtm create feat/billing fix/header --yes

  ✓ feat/billing
    ../acme.trees/feat-billing
  ✓ fix/header
    ../acme.trees/fix-header

  2 created
```

When at least one fails, the run lists what was created and what failed, and exits non-zero.

## Isolation

Each new worktree records whether it runs on its own ports, compose project and namespaces (`isolated`, the default) or on its source's (`verbatim`). The wizard asks it only when `run.toml` declares something to isolate; `--isolation` answers it, and under `--yes` the default is `run.toml`'s `isolation`, else `isolated`.

```bash
wtm create hotfix/prod --isolation verbatim --yes
```

`create`, `checkout` and `extract` all take `--isolation`. What each answer changes, and how to switch later with `wtm env`, is in [Isolation](isolation.md).

## From a pull request: `wtm checkout`

`wtm checkout 42` creates a worktree on the branch of pull request #42. Without a number it lists the open pull requests to pick from; `--review` keeps those waiting for your review, `--mine` your own. It needs [`gh`](https://cli.github.com), authenticated for the repository.

- The branch is fetched from `origin`. When a local branch of that name exists, it is checked out as-is, keeping commits you never pushed; if it is behind `origin`, the wizard offers to fast-forward it, and `--ff` does so under `--yes`.
- The recorded parent is the pull request's base branch; `--from` names another.
- A pull request from a fork is refused: `gh pr checkout` handles those.
- The `.env` provisioning, isolation and hooks are those of `create`, with the same `--env-from` and `--isolation`.

## Splitting work off: `wtm extract`

`wtm extract` moves some of a worktree's uncommitted changes to another worktree, new or existing: to split a pull request that grew too large, or to set aside a fix made on the wrong branch. Committed work is not extracted.

```console
$ wtm extract feat/login --files apps/api --to feat/login-api --yes

  ✓ Moved 2 files to feat/login-api

    new  apps/api/auth/session.ts
    new  apps/api/auth/token.ts

  source  feat/login · files removed
  env     3 ports settled (offset +80)
  path    ../acme.trees/feat-login-api
```

- **The source** is the positional argument, or picked among the worktrees with changes. **The files** are `--files`, each a file or a directory taking everything below it; the wizard lists modified, deleted and untracked files one by one, inside new directories too. Gitignored files are never listed.
- **The target** is `--to`. A branch with no worktree is created like `create` would, with its `.env`, isolation and hooks. Its parent defaults to the source's own parent, so the new branch sits beside the source; `--from feat/login` stacks it on the source instead. A `--to` branch that already exists locally needs `--from`, as with `create`.
- **Move or copy.** The files are removed from the source once they are in the target; `--keep` leaves them in both.

### Conflicts

A selected file that was also modified in the target, or an untracked file that already exists there with other content, is a conflict. By default the extraction stops before anything is removed from the source, and exits with code 15:

```console
$ wtm extract feat/login --files apps/web/package.json --to fix/header --yes

  ✗ cannot apply the selected changes: apps/web/package.json was also modified in "fix/header" — resolve it there, extract to another worktree, or re-run with --on-conflict resolve
```

`--on-conflict resolve` (or accepting the wizard's question) writes conflict markers into the target instead, and still removes nothing from the source: resolve the markers in the target as after a rebase, then discard the same files in the source.

```console
$ wtm extract feat/login --files apps/web/package.json --to fix/header --on-conflict resolve --yes

  ! Applied to fix/header with conflicts

  Conflicts to resolve
    apps/web/package.json

  Nothing was removed from feat/login: discard the applied changes in fix/header to undo.
```

## The wizard and `--yes`

Run in a terminal without `--yes`, each command opens a wizard for whatever its flags did not answer, then a recap to confirm. For `create`: the source branch, the env strategy, the isolation, the fast-forward of a source behind `origin`, then the recap.

```
  Step 5/5 • Confirm & create

  ✓ Source branch: main
  ✓ Env strategy: config default
  ✓ Isolation: isolated
  ⊘ Source update — source already up to date

  ┃  Branch:    feat/profile
  ┃  Source:    main
  ┃  Env:       config default
  ┃  Isolation: isolated

▌▸ Yes, create worktree
   No, cancel
```

`wtm create` with no branch asks for the branch names first (tab adds another, enter continues). `--yes` asks nothing: every answer comes from a flag or its default, and a selection with no safe default is an error naming the flag to pass (the branch for `create`, the pull request number for `checkout`, the source, `--files` and `--to` for `extract`). `--output json` requires `--yes`; the contract for scripts and agents is in [Integrations](integrations.md).

To remove worktrees again, see [Removing worktrees](removing-worktrees.md).
