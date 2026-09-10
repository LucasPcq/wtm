## wtm run down

Stop a worktree's running jobs

### Synopsis

Stop the jobs running in [worktree] — the current one when omitted, picked interactively when there is a terminal.
With --profile, stops only that profile's jobs.
Jobs running in other worktrees are never touched.

```
wtm run down [worktree...] [flags]
```

### Options

```
      --all              Stop jobs across every worktree (bypasses per-worktree scoping)
  -h, --help             help for down
      --output string    Output format: text or json (default "text")
      --profile string   Stop only this profile's jobs
  -y, --yes              Skip all prompts; stops what the worktree has running
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

