## wtm clean

Remove a worktree and its local branch

### Synopsis

Remove a git worktree and delete the local branch. The remote branch is never touched.
Without arguments, shows an interactive picker.

```
wtm clean [branch] [flags]
```

### Options

```
      --force               Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes
  -h, --help                help for clean
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto the grandparent (no prompt)
  -y, --yes                 Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

