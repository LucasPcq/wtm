## wtm sync

Rebase selected worktrees onto their parent, in cascade

### Synopsis

Rebase one or more managed worktrees onto their parent. Pass branch names to target
specific worktrees, --all to sync every worktree, or no arguments to pick interactively.
The base branch is fetched and fast-forwarded first, then each selected worktree is
rebased onto its parent in topological order (parents before children). The cascade is
local; on a conflict the branch is left clean (rebase aborted) and its selected
descendants are skipped. Pass --keep-conflict to leave a conflicting rebase in progress
in its worktree for manual resolution instead of aborting. A parent no step covers —
a branch with no worktree, or one left out of the selection — is never refreshed by the
cascade; when it is behind its remote you are offered to fast-forward it first
(--ff-parents / --no-ff-parents). After a successful cascade, optionally force-push
(with lease) the rebased branches.

```
wtm sync [branch...] [flags]
```

### Options

```
      --all             Sync every managed worktree
      --base string     Base branch to sync from (defaults to config or detected base)
      --dry-run         Preview the cascade without rebasing or pushing
      --ff-parents      Fast-forward the parents the cascade does not cover (no worktree, or left out of the selection) before rebasing onto them; no-op with --dry-run
  -h, --help            help for sync
      --keep-conflict   Leave a conflicting rebase in progress in its worktree instead of aborting
      --no-ff-parents   Never fast-forward those parents; rebase onto them as they are
      --no-push         Rebase locally only; never push
      --output string   Output format: text or json (default "text")
      --push            Force-push (with lease) rebased branches without prompting
  -y, --yes             Skip all prompts; resolve every decision from flags and safe defaults (requires branch args or --all; use --push to push)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

