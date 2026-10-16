## wtm create

Create one or more worktrees

### Synopsis

Create one or more git worktrees with env provisioning, metadata, and hooks.
Several branches are created one after the other from the same source; a failure
does not stop the others, and the run ends with what was created and what failed.
A branch that already exists locally is checked out as-is, keeping its commits;
its parent can't be inferred, so --from then names the branch recorded for `wtm sync`.
A branch only origin has is created from it, tracking it.
When run.toml declares jobs, a branch whose derived name a live worktree or another
branch of the run already carries (feat.x next to feat/x) is refused.
Without arguments, the form asks for the branches: tab adds another, enter continues.

```
wtm create [branch...] [flags]
```

### Examples

```
  # Answer the form: branches, source, env strategy, isolation
  wtm create

  # Three worktrees from the base branch, no prompts
  wtm create feat/login feat/billing fix/header --yes

  # A stacked branch on top of feat/login
  wtm create feat/login-ui --from feat/login --yes

  # See the plan first, then apply exactly that plan
  wtm create feat/login --dry-run
  wtm create feat/login --expect-fingerprint <hash>

  # For a script or an agent: idempotent, with the v2 JSON envelope
  wtm create feat/login --if-not-exists --output json
```

### Options

```
      --ask                         Ask again the questions this repository remembers an answer to, to change or forget it
      --dry-run                     Show the plan and its fingerprint, apply nothing
      --env-from string             How the .env files of the new worktrees are provisioned: example (from its template), main (copied from the main checkout) or parent (from the source's worktree); defaults to config.toml's strategy
      --expect-fingerprint string   Apply only if the facts still have this fingerprint, the one --dry-run printed
      --ff                          Fast-forward to origin first the branch each worktree starts from, when it is behind; one that has diverged or holds uncommitted changes starts as it is
      --from string                 The branch a new branch starts from, or the parent recorded for wtm sync of one that already exists
  -h, --help                        help for create
      --if-not-exists               Succeed without a change when the worktree already exists (idempotent)
      --isolation string            How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated
      --output string               Output format: text or json (default "text")
      --remember stringArray        The answers to always use in this repository: env_from, isolation, fast_forward (repeatable)
  -y, --yes                         Run unattended: no question, no confirmation; what is not given takes its default
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

