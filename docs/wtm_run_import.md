## wtm run import

Replace run.toml with a JSON run config

### Synopsis

Read a JSON run config payload from a file (or stdin) and make it the run.toml.

Pass "-" or omit the argument to read from stdin.

The payload replaces the whole file — jobs, profiles, .env port links and
project settings alike. The run is confirmed before anything is written; pass
--yes to run unattended.

Nothing is reconciled after the write: run wtm env to settle the .env files
against the new configuration.

```
wtm run import [file] [flags]
```

### Options

```
  -h, --help            help for import
      --output string   Output format: text or json (default "text")
  -y, --yes             Replace run.toml without confirming
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

