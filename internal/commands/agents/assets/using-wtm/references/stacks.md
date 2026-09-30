# Stacked branches: `tree`, `sync`, `reparent`, `fast-forward`

Every worktree records the branch it came from (its parent). These commands read and maintain that chain. Everything here assumes the driving rules of `SKILL.md`: always `--output json --yes` on the mutating ones, never `--force` unasked. Full output shapes are in `json.md`.

## `tree`

`wtm tree --output json` is the parent to child forest. Use it rather than `wtm list` when hierarchy or orchestration order matters. `--output mermaid` prints the same forest as a Mermaid flowchart, for a document or a PR description.

- `needs_sync` is the key signal: that node's parent moved past it.
- `origin` (`{ahead, behind, state}`, or `null` when the branch has no origin counterpart) describes divergence from `origin/<branch>`, with `state` one of `up-to-date`/`behind`/`ahead`/`diverged`. It is distinct from `commits_ahead`, which counts commits against the **parent/base** branch.

The recorded parent of a worktree comes from `create --from`, `checkout` (the PR's base) or `reparent`. For a branch created outside wtm, `create` requires `--from` precisely because `sync` and `tree` treat the recorded parent as fact (see `worktrees.md`).

## `sync`

`wtm sync <branch…> --yes --output json` (or `--all`) rebases the selected worktrees onto their recorded parent, in cascade (parents before children), fetching first.

- The selection is required: branch arguments or `--all`. Without a TTY, in plain output, with neither `--yes` nor `--dry-run`, it refuses naming `--yes` instead of opening a picker.
- Local only: `--yes` does **not** push. Pass `--push` to force-push with lease.
- A conflict aborts that branch's rebase and skips its descendants, unless `--keep-conflict` leaves the rebase in progress.
- **The base is only involved when it is actually a target**: a step rebases onto it, the selection names it, or the run covers everything (`--all`, which includes the root). Otherwise it is neither fetched nor fast-forwarded and `base_targeted` is `false`; an explicit selection whose every worktree hangs off another parent leaves the base completely alone.

**Parents no step covers.** A parent that is not itself rebased by the cascade (a branch with no worktree, or one left out of the selection) is not refreshed. Every run reports those parents in `parent_updates`, with `behind` (commits the local ref lacks) and `children` (the worktrees rebased onto it). Their `status`:

- `behind`: left as is. Re-run with `--ff-parents` to refresh it.
- `fast_forwarded`: refreshed.
- `diverged`: no fast-forward exists. Reconcile it by hand; the flag will never move it.
- `ff_failed`: the refresh was asked for and could not happen (`detail` says why, e.g. a dirty parent worktree). Passing the flag again will not help.

`--ff-parents` refreshes them first, `--no-ff-parents` never; `--yes` alone does not fast-forward parents. `--dry-run` reports them without any network call and never refreshes, so `--ff-parents` is a no-op there.

**When `sync` exits non-zero**, some branch is `status: conflict` or `error` in the JSON.

- `conflict` (default mode): the rebase was aborted and the branch and its descendants skipped. The user resolves it manually in its worktree, then re-runs `sync`.
- `kept_in_progress: true`: the rebase is paused in that step's `path`, for the user to finish with `git rebase --continue`.
- `diverged` (exit stays 0): the user must reconcile that branch before re-running.

## `reparent`

`wtm reparent <branch…> --to <parent> --yes --output json` changes the recorded parent of one or more worktrees to the same new parent. Metadata only: the rebase happens on the next `sync`. Use it after a middle branch merges. The worktrees and `--to` are required under `--yes`/JSON.

JSON: `{"reparented": [{branch, old_parent, new_parent}, …]}`.

When `clean` or `prune` removes a parent, pass `--reparent-children` there to reparent its surviving children onto the grandparent (otherwise they are left orphaned under `--yes`).

## `fast-forward`

`wtm fast-forward <branch…> --yes --output json` (alias `ff`, or `--all`) advances the selected worktrees to `origin/<branch>` and **nothing else**: no rebase onto the parent, no merge. Use it to pull down work pushed to a branch you already agree with; use `sync` when the branch has to be replayed onto its parent.

- With neither branch arguments nor `--all` it refuses naming `--all` rather than opening a picker. `--output json` requires `--yes`.
- Two refusals, only one of them liftable:
  - a **diverged** branch is refused and `--force` does **not** lift it (advancing it would drop local commits; run `wtm sync` instead);
  - a worktree with **uncommitted changes** is refused and `--force` lifts it, though git still refuses if a modified file would be overwritten.
- A run over several branches keeps going past the ones it could not move and reports every one.

JSON: an array of `{branch, status, old_tip, new_tip, behind, detail?}`, `status` one of `already up to date`, `fast-forwarded from origin`, `diverged`, `no origin counterpart`, or `failed` (`detail` says why). A run in which any branch is `failed` exits non-zero; `diverged` and `no origin counterpart` are reported facts, not failures.

`create --ff` and `extract --ff` fast-forward a behind-only source/parent branch as part of a creation (see `worktrees.md`).
