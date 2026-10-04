## wtm version

Print wtm's version and the versions of its machine contracts

### Synopsis

Print the version of this wtm binary, the same line as `wtm --version`.
With --output json it also gives the version of each contract an integration
reads, so a host can tell it is talking to a wtm it understands before relying on
it: `events` is the schema version of `wtm events`. New keys are added as new
contracts appear; a reader ignores the ones it does not know.

```
wtm version [flags]
```

### Examples

```
  wtm version

  # What a host checks before reading wtm events
  wtm version --output json | jq .events
```

### Options

```
  -h, --help            help for version
      --output string   Output format: text or json (default "text")
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

