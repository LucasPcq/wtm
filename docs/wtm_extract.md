## wtm extract

Move uncommitted changes to another worktree

### Synopsis

Move a subset of a worktree's uncommitted changes to another worktree
(new or existing) to split an oversized PR or isolate unrelated work.

The source worktree is the first thing chosen: pass its branch as [source],
or omit it to pick interactively from the worktrees that have changes. A source
is required when there is no terminal or with --output json.

A --to branch that already exists locally is checked out as-is, keeping its
commits. Its parent can't be inferred, so --from then names the branch recorded
for `wtm sync` — asked in the wizard, required without it.

Untracked files are listed one by one, including inside brand-new directories,
so you can take part of a new folder; gitignored files are never listed.

On conflict it aborts by default, leaving the source intact; --on-conflict resolve
applies conflict markers in the target so you can resolve them like a rebase.
A file that merely already exists in the target counts as a conflict too.

```
wtm extract [source] [flags]
```

### Options

```
      --ff                   Fast-forward the parent branch to origin before creating the target (non-interactive; skipped when it has diverged)
      --files strings        Files to extract, or a directory to take everything below it (skips interactive selection)
      --from string          Parent branch when creating the target worktree
  -h, --help                 help for extract
      --keep                 Copy instead of move (keep the changes in the source)
      --on-conflict string   On conflict: abort (default) or resolve (write conflict markers in the target)
      --output string        Output format: text or json (default "text")
      --to string            Target worktree branch; created if it does not exist
  -y, --yes                  Skip all prompts; resolve every decision from flags and safe defaults (requires a source arg, --files and --to; --from is also required when --to already exists locally; errors if a selection is missing)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

