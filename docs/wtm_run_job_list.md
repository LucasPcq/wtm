## wtm run job list

List jobs from run.toml

### Synopsis

List jobs declared in <git-common-dir>/wtm/run.toml.

In a TTY, opens an interactive picker. Selecting a job offers Edit or Remove.
Use --output json, --yes (or pipe stdout) for a non-interactive listing.

```
wtm run job list [flags]
```

### Options

```
  -h, --help            help for list
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the picker; print the table instead
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

