## wtm fast-forward

Advance worktree branches to their origin counterpart

### Synopsis

Fast-forward one or more managed worktrees to origin/<branch>, and nothing else:
no rebase onto the parent, no merge. Pass branch names, --all for every worktree, or
no arguments to pick interactively. A branch that has diverged from origin is refused —
`wtm sync` is the command that replays local commits onto it, and --force does not lift
that refusal. A worktree with uncommitted changes is refused too; --force fast-forwards
it anyway, and git still refuses if a modified file would be overwritten.

```
wtm fast-forward [branch...] [flags]
```

### Options

```
      --all             Fast-forward every managed worktree
      --force           Fast-forward a worktree that has uncommitted changes
  -h, --help            help for fast-forward
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip all prompts; resolve every decision from flags and safe defaults (requires branch args or --all)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

