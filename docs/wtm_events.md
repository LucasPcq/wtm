## wtm events

Stream the repository's worktree changes as they happen

### Synopsis

Print the repository's worktrees, then every change made to them, whoever made it:
a command in another shell, an agent, or `wtm ui`. The stream opens on a snapshot of
every worktree and a ready line, then carries one event per change — created,
updated, relocated, reparented, removed. With --output json each line is one JSON
object (JSON Lines), the contract an integration reads; its schema ships with wtm.
If the run daemon stops, the stream waits for it and opens again on a fresh
snapshot: treat every event as an upsert keyed by branch, and every snapshot as a
reset. It runs until interrupted or until the reader of its pipe goes away, and
exits with code 20 if it receives an event of a schema newer than its own.

```
wtm events [flags]
```

### Examples

```
  # Watch this repository's worktrees
  wtm events

  # The JSON Lines contract, filtered with jq
  wtm events --output json | jq -c 'select(.type == "worktree.created")'

  # Another repository than the current one
  wtm events --repo ~/code/app --output json
```

### Options

```
  -h, --help            help for events
      --output string   Output format: text or json (default "text")
      --repo string     Watch the repository holding this path instead of the current one
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

