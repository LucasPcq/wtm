## wtm config show

Print the project config.toml

```
wtm config show [flags]
```

### Examples

```
  wtm config show

  # Check the file, print nothing else
  wtm config show --validate

  wtm config show --output json
```

### Options

```
  -h, --help            help for show
      --output string   Output format: text or json (default "text")
      --validate        Validate the config instead of printing it
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm config](wtm_config.md)	 - Inspect or edit the project wtm config

