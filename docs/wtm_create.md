## wtm create

Create a new worktree

### Synopsis

Create a git worktree with env provisioning, metadata, and hooks.
A branch that already exists locally is checked out as-is, keeping its commits.
Its parent can't be inferred, so --from then names the branch recorded for
`wtm sync` — asked in the wizard, required without it.
Without arguments, prompts for the branch name interactively.

```
wtm create [branch] [flags]
```

### Options

```
      --env-from string   Override env strategy (example, main, parent)
      --ff                Fast-forward to origin before creating — the source branch, or the branch itself when it already exists locally (non-interactive; skipped when it has diverged)
      --from string       Source branch to start from — or, when the branch already exists locally, the parent to record for wtm sync (required there without the wizard)
  -h, --help              help for create
      --if-not-exists     Succeed silently if the worktree already exists (idempotent)
      --output string     Output format: text or json (default "text")
  -y, --yes               Skip all prompts; resolve every decision from flags and safe defaults (branch name required; source defaults to the base branch for a new branch, and --from is required for one that already exists)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

