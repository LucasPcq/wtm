## wtm reparent

Change the parent one or more worktrees are rebased onto

### Synopsis

Change the recorded parent (source branch) of one or more worktrees. Only the
metadata is updated — the rebase happens on the next `wtm sync`. Pass the worktrees
and --to <parent>, or run with no arguments to multi-select interactively. The new
parent must exist as a local or origin remote-tracking branch (origin/x), and the
resulting parent chain must stay acyclic.

```
wtm reparent [branch...] [flags]
```

### Options

```
  -h, --help            help for reparent
      --output string   Output format: text or json (default "text")
      --to string       New parent branch to rebase onto
  -y, --yes             Skip all prompts; resolve every decision from flags and safe defaults (needs at least one worktree and --to)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

