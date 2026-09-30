## wtm run export

Export run.toml as JSON on stdout

### Synopsis

Emit the current run config as JSON on stdout, whatever --output says: like run url, this is machine output and is never framed. Pipe to a file and use with wtm run import to share configurations.

```
wtm run export [flags]
```

### Examples

```
  wtm run export > run.json

  # One profile and its jobs
  wtm run export --profile backend > backend.json

  # Copy the layout into another clone
  wtm run export | (cd ../other-clone && wtm run import - --yes)
```

### Options

```
  -h, --help             help for export
      --output string    Output format: text or json (default "text")
      --profile string   Export only this profile and its jobs
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

