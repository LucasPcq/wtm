## wtm run stop

Stop a single job

### Synopsis

Stop one running job of [worktree] — the current one when omitted, picked interactively when there is a terminal.
The job is named with --job; without it, a fully interactive run offers a picker.

```
wtm run stop [worktree...] [flags]
```

### Options

```
  -h, --help            help for stop
      --job string      Job to stop (required without a terminal or in --output json mode)
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip all prompts; --job is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

