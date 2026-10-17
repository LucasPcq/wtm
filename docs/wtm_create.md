## wtm create

Create one or more worktrees

### Synopsis

Create one or more git worktrees with env provisioning, metadata, and hooks.
Several branches are created one after the other from the same source; a failure
does not stop the others, and the run ends with what was created and what failed.
A branch that already exists locally is checked out as-is, keeping its commits.
Its parent can't be inferred, so --from then names the branch recorded for
`wtm sync` — asked in the wizard, required without it.
When run.toml declares jobs, a branch whose derived name a live worktree or another
branch of the run already carries (feat.x next to feat/x) is refused.
Without arguments, the wizard asks for the branches: tab adds another, enter continues.

```
wtm create [branch...] [flags]
```

### Examples

```
  # Answer the wizard: branches, source, env strategy, isolation
  wtm create

  # Three worktrees from the base branch, no prompts
  wtm create feat/login feat/billing fix/header --yes

  # A stacked branch on top of feat/login
  wtm create feat/login-ui --from feat/login --yes

  # For a script or an agent: idempotent, with a JSON envelope
  wtm create feat/login --if-not-exists --yes --output json
```

### Options

```
      --ask                Ask again the questions this repository remembers an answer to, to change or forget it
      --env-from string    Override env strategy (example, main, parent)
      --ff                 Fast-forward to origin before creating — the source branch, or the branch itself when it already exists locally (answers the wizard's question; skipped when it has diverged)
      --from string        Source branch to start from — or, when the branch already exists locally, the parent to record for wtm sync (required there without the wizard)
  -h, --help               help for create
      --if-not-exists      Succeed silently if the worktree already exists (idempotent)
      --isolation string   How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated
      --output string      Output format: text or json (default "text")
  -y, --yes                Skip all prompts; resolve every decision from flags and safe defaults (branch names required; source defaults to the base branch for a new branch, and --from is required for one that already exists)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

