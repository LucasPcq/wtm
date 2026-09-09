## wtm run ps

List currently running jobs

### Synopsis

Show the jobs managed by the background daemon (name, kind, status, PID, uptime, worktree).
It lists every repository the daemon knows, so it works from anywhere — inside a
run-initialized repository or not.
To act on those jobs, open the run view with `wtm run logs`, which covers as many
worktrees as you select.

```
wtm run ps [flags]
```

### Options

```
  -h, --help            help for ps
      --output string   Output format: text or json (default "text")
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

