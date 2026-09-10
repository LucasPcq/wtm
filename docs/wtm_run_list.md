## wtm run list

List jobs and profiles declared in run.toml

### Synopsis

Show the jobs and profiles configured for the project.
In a TTY, offers an interactive picker with start/stop/logs actions.

```
wtm run list [flags]
```

### Options

```
  -h, --help            help for list
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the interactive picker; print the table instead
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

