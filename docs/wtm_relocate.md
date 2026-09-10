## wtm relocate

Move worktrees to align with base_path and adopt external ones

### Synopsis

Reconcile every worktree with the configured base_path. Worktrees not under it are
moved (git worktree move) and worktrees created outside wtm are adopted (their parent
recorded so `wtm sync` works). Pass --to to change base_path and move existing worktrees
to the new location. Dirty or locked worktrees are skipped unless --force; an occupied
target path is never overwritten.

```
wtm relocate [flags]
```

### Options

```
      --dry-run         Preview the plan without moving or adopting anything
      --force           Lift safety refusals (dirty/locked): move those worktrees too; still asks to confirm unless --yes
  -h, --help            help for relocate
      --output string   Output format: text or json (default "text")
      --to string       New base_path (relative to repo root); also moves existing worktrees there
  -y, --yes             Skip all prompts; resolve every decision from flags and safe defaults (parents default to the base branch)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

