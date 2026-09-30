## wtm env

Reconcile a worktree's .env against its template and value sources

### Synopsis

Detect and fix .env drift in a worktree: add expected-but-missing keys and
(with --mode refresh) settle values that diverge from the source.

Values come from the strategy the worktree was created with (example → template
placeholders, main → the main checkout, parent → the parent worktree then main),
shown in the report; override it per run with --from.

Pass a worktree branch, or omit it to pick interactively. --check prints a
read-only drift report. Non-interactively (--yes / --output json) it applies only
safe additions; conflicts need --on-conflict and orphans need --prune.

A worktree created before the isolation choice existed (no isolation in its
meta.json) keeps its source's ports and COMPOSE_PROJECT_NAME: non-interactively
only its keys are reconciled, and the report says so. The wizard offers to adopt
isolation — a new compose project, so its current volumes are no longer used —
and --isolation isolated adopts it explicitly.

--isolation verbatim puts the values wtm owns (linked ports, [[env]] values,
COMPOSE_PROJECT_NAME) back to the source's and leaves every other key alone; the
wizard shows them first. Either isolation is recorded only once the .env is in
line with it: a run that fails or is cancelled records nothing.

```
wtm env [worktree] [flags]
```

### Options

```
      --check                Read-only drift report; write nothing
      --from string          Override the value source strategy (example, main, parent)
  -h, --help                 help for env
      --isolation string     Settle the worktree on an isolation, recorded once its .env is in line: isolated (wtm moves its ports, compose project and namespaces, in the .env and at run time) or verbatim (the values wtm owns go back to the source's, and it runs on the ports its .env keeps)
      --mode string          Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts) (default "add")
      --on-conflict string   Non-interactive conflict resolution: keep (default) or overwrite
      --output string        Output format: text or json (default "text")
      --prune                Remove orphan keys (present in the .env but in no source)
  -y, --yes                  Skip all prompts; apply safe additions and flag-driven decisions only
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

