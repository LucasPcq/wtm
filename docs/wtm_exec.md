## wtm exec

Run one command in several worktrees, in parallel

### Synopsis

Run a shell line in each selected worktree, in parallel, and report which passed.
Everything after -- is run with /bin/sh -c from the worktree's root, with that
worktree's environment: the variables describing the worktree you stand in are
removed, and the target's run variables (compose project, shifted ports) are added
when it has them — the same ones its hooks get. stdin is closed. Each worktree's whole
output is kept in a log under the state directory; failures show its tail.
Pass worktree names (branches), --all, or nothing to pick interactively. The run
exits 1 when any command failed; each worktree's own exit code is in the report.

```
wtm exec [worktree...] -- <command> [flags]
```

### Examples

```
  # Run the tests on two branches
  wtm exec feat/login feat/signup -- pnpm test

  # Reinstall everywhere after a lockfile bump, two at a time
  wtm exec --all --jobs 2 -- pnpm install

  # Read each worktree's last commit
  wtm exec --all --yes --print -- git log -1 --oneline

  # For an agent
  wtm exec --all --yes --output json -- pnpm typecheck
```

### Options

```
      --all             Run in every worktree, the main checkout included
  -h, --help            help for exec
      --jobs int        How many commands run at once (0: one per CPU)
      --output string   Output format: text or json (default "text")
      --print           Also show the full output of every worktree, successes included
  -y, --yes             Skip all prompts (requires worktree names or --all)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

