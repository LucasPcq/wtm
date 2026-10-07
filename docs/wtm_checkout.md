## wtm checkout

Create a worktree from an existing pull request

### Synopsis

Create a worktree from a pull request.
A local branch of the PR's name is checked out as-is, keeping commits you never
pushed; it is fast-forwarded when it is behind origin if you accept, or with
--ff under --yes.
Without arguments, shows an interactive picker of open PRs.

```
wtm checkout [number] [flags]
```

### Examples

```
  # Pick among the open pull requests
  wtm checkout

  # Only the ones waiting for your review
  wtm checkout --review

  wtm checkout 42

  # No prompts, with a JSON result
  wtm checkout 42 --yes --output json
```

### Options

```
      --ask                Ask again the questions this repository remembers an answer to, to change or forget it
      --env-from string    Override env strategy (example, main, parent)
      --ff                 Fast-forward the PR's branch to origin when it already exists locally and is behind (non-interactive; skipped when it has diverged)
      --from string        Parent branch for sync (defaults to the PR base branch)
  -h, --help               help for checkout
      --isolation string   How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated
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

