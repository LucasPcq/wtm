## wtm run up

Start a profile's jobs

### Synopsis

Start every job in a profile, in declared order, in each [worktree] — the current one when omitted, picked interactively when there is a terminal.
Several worktrees start concurrently and independently: one that aborts leaves the others running,
and the run exits non-zero if any of them did.
Without --profile, uses the default profile (or shows a picker if multiple exist).
Once the jobs are up, each declared port is checked: a port nothing answers on is
reported rather than announced as bound. It never fails the run — see --no-probe
and run.toml's port_probe_timeout.
Tasks block the profile and abort it on failure; services launch in the background.
When another worktree is already running jobs, wtm asks once what to do about it and can
remember the answer as run.toml's `concurrency`; --exclusive and --parallel override it
for one run. --exclusive is refused on several worktrees, since it stops all but one.
The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.

```
wtm run up [worktree...] [flags]
```

### Options

```
  -d, --detach            Start the jobs and return immediately instead of opening their output
      --exclusive         Stop jobs on other worktrees before starting (one worktree only)
  -h, --help              help for up
      --no-probe          Skip the check that each declared port was actually bound
      --output string     Output format: text or json (default "text")
      --parallel          Start without stopping other worktrees
      --profile strings   Profiles to start; repeatable. A job several of them name starts once. Defaults to the default profile, or a picker when several exist
  -y, --yes               Skip all prompts; leaves the other worktrees' jobs running unless --exclusive
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

