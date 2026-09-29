## wtm checkout

Create a worktree from an existing pull request

### Synopsis

Create a worktree from a pull request.
A local branch of the PR's name is checked out as-is, keeping commits you never
pushed; interactive runs offer to fast-forward it when it is behind origin.
Without arguments, shows an interactive picker of open PRs.

```
wtm checkout [number] [flags]
```

### Options

```
      --env-from string    Override env strategy (example, main, parent)
      --from string        Parent branch for sync (defaults to the PR base branch)
  -h, --help               help for checkout
      --isolation string   How the new worktree stands against its source: isolated (its own ports, compose project and service slices, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated
      --mine               Show only your PRs
      --output string      Output format: text or json (default "text")
      --review             Show only PRs where you are requested as reviewer
  -y, --yes                Skip all prompts; resolve every decision from flags and safe defaults (PR number required)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

