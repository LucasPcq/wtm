## wtm status

Show a worktree's whole state: isolation, env files, jobs and what to fix

### Synopsis

Read one worktree's state in one document: its branch and path, how it runs
(isolation, addressing, port offset), which declared .env files it lacks, every
job run.toml declares with its state (the states `wtm events` reports) and its
address, and each problem found with the exact command that fixes it.

[worktree] defaults to the current one. It changes nothing, asks nothing, never
starts the run daemon (with none running it reads the job index and checks the
processes itself) and needs no --yes. No .env value appears in its output.
It exits 0 whatever it finds: read `problems`.

```
wtm status [worktree] [flags]
```

### Examples

```
  wtm status

  wtm status feat/login --output json

  # Every worktree, one line each, problems expanded
  wtm status --all

  # The commands that fix what it found
  wtm status --output json | jq -r '.problems[].fix'
```

### Options

```
      --all             Read every worktree of the repository (JSON: an array of the same documents)
  -h, --help            help for status
      --output string   Output format: text or json (default "text")
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

