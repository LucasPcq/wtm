## wtm list

List all worktrees

### Synopsis

List all git worktrees with their status, PR info, and running services.

```
wtm list [flags]
```

### Options

```
  -h, --help            help for list
      --output string   Output format: text or json (default "text")
      --with-prs        Include GitHub PR info in non-interactive output (fetched eagerly)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

