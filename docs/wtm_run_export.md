## wtm run export

Export run.toml as JSON on stdout

### Synopsis

Emit the current run config as JSON. Pipe to a file and use with wtm run import to share configurations.

```
wtm run export [flags]
```

### Options

```
  -h, --help             help for export
      --profile string   Export only this profile and its jobs
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

