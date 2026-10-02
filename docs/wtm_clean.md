## wtm clean

Remove worktrees and their local branches

### Synopsis

Remove git worktrees and delete their local branches. The remote branch is never touched.
Without arguments, shows an interactive picker where several can be checked.

Several worktrees are removed one after the other: a failure does not stop the others, and
the run exits with the first failure's code. Under --yes, one unsafe worktree (locked,
dirty, unpushed, open PR) refuses the whole run before anything is removed, unless --force.

The removal runs in a fixed order: the worktree's jobs are stopped and checked gone (a job
that will not stop refuses the removal unless --force), the on_clean hooks run, git removes
the worktree — and only then is its data dropped. A failure before that last step leaves
the data where it was.

By default, clean DROPS the namespaces the worktree carved out of shared services (a
database per worktree in a shared postgres, say): the confirmation names each one, and
--output json reports each as dropped, deferred or kept. --keep-data withholds the drop. A
service that is down cannot take its data back: the form asks whether to start it now or
keep the data until wtm next starts it; --yes keeps it, --drop-data starts it. A namespace
another worktree reaches under the same name is never dropped.

```
wtm clean [branch...] [flags]
```

### Examples

```
  # Pick the worktrees to remove
  wtm clean

  wtm clean feat/login

  # Several at once, no prompts; their children move onto the nearest survivor
  wtm clean feat/login feat/signup --yes --reparent-children

  # Keep the databases it holds in shared services
  wtm clean feat/login --yes --keep-data --output json
```

### Options

```
      --drop-data           Drop the removed worktrees' data now, starting the shared services that are down to do it
      --force               Lift safety refusals (locked/dirty/unpushed/open-PR); still asks to confirm unless --yes
  -h, --help                help for clean
      --keep-data           Keep the namespaces the removed worktrees carved out of shared services
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto their nearest surviving ancestor (no prompt)
  -y, --yes                 Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

