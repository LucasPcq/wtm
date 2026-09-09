## wtm run start

Start a single job

### Synopsis

Start one job of [worktree] — the current one when omitted, picked interactively when there is a terminal.
The job is named with --job; without it, a fully interactive run offers a picker.
A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.
-d starts it and returns the prompt instead.
A task always runs inline and blocks until it exits, with or without -d.

```
wtm run start [worktree] [flags]
```

### Options

```
  -d, --detach          Start the service and return immediately instead of opening its output
  -h, --help            help for start
      --job string      Job to start (required without a terminal or in --output json mode)
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip all prompts; --job is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

