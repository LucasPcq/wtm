## wtm run down

Stop a worktree's running jobs

### Synopsis

Stop the jobs running in [worktree] — the current one when omitted, picked interactively when there is a terminal.
With --profile, stops only that profile's jobs.
Jobs running in other worktrees are never touched, unless --all is given: it stops every worktree of this repository, without asking, and lists each one it emptied. Other repositories are never touched.

```
wtm run down [worktree...] [flags]
```

### Options

```
      --all              Stop the jobs of every worktree of this repository
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

