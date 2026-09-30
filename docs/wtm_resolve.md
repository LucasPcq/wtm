## wtm resolve

Resolve a branch to its worktree path

```
wtm resolve [branch] [flags]
```

### Examples

```
  wtm resolve feat/login

  # Use it in a script
  cd "$(wtm resolve feat/login)"

  wtm resolve feat/login --output json
```

### Options

```
  -h, --help            help for resolve
      --output string   Output format: text or json (default "text")
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

