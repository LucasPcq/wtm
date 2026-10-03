## wtm ui

Open the worktree dashboard

### Synopsis

Open a full-screen dashboard of the repository's worktrees.
The Worktrees tab lists them with their git state against both the base branch and
origin, and their pull requests; the Tree tab lays the same worktrees out as the
parent-child forest `wtm tree` prints; the Services tab gathers every worktree the
run daemon holds something up in, with the addresses its jobs answer on. `n`
creates a worktree; right-click a row (or press `m`) to reparent, sync, or delete
it; `a` opens the actions that run over several worktrees at once, syncing or
reparenting a selection of them; `L` reads a job's logs in the detail panel.
The list follows every worktree created, moved or removed, whoever did it, as
`wtm events` reports it; its local git state is re-read every 20 seconds, when
the terminal regains focus and after each action; the detail panel reloads when the selection
changes or an operation touches it, and pull requests load once. Nothing is
fetched on its own: `r` fetches the remote and refreshes all of it.
Press `?` for the key reference.

```
wtm ui [flags]
```

### Examples

```
  # Press ? inside for the key reference
  wtm ui
```

### Options

```
  -h, --help            help for ui
      --output string   Output format: text or json (default "text")
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

