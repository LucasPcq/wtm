## wtm run start

Start a single job

### Synopsis

Start one job of [worktree] — the current one when omitted, picked interactively when there is a terminal.
The job is named with --job; without it, a fully interactive run offers a picker.
A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.
-d starts it and returns the prompt instead.
A task always runs inline and blocks until it exits, with or without -d.
Like `run up`, it reports what run.toml gets wrong before starting, checks the job's declared ports
once it is up (see --no-probe and run.toml's port_probe_timeout), and asks once what to do about
the jobs other worktrees are running; --exclusive and --parallel answer for one run.

```
wtm run start [worktree] [flags]
```

### Options

```
  -d, --detach                 Start the service and return immediately instead of opening its output
      --exclusive              Stop jobs on other worktrees before starting
      --force wtm run --help   Lift the refusal to start a job whose touches reach foreign data (see wtm run --help); other questions are still asked unless --yes
  -h, --help                   help for start
      --job string             Job to start (required without a terminal or in --output json mode)
      --no-probe               Skip the check that each declared port was actually bound
      --output string          Output format: text or json (default "text")
      --parallel               Start without stopping other worktrees
  -y, --yes                    Skip all prompts; --job is then required, and the other worktrees' jobs keep running unless --exclusive
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

