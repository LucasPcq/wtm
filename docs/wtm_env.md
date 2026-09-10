## wtm env

Reconcile a worktree's .env against its template and value sources

### Synopsis

Detect and fix .env drift in a worktree: add expected-but-missing keys and
(with --mode refresh) settle values that diverge from the source.

Values come from the strategy the worktree was created with (example → template
placeholders, main → the main worktree, parent → the parent worktree then main),
shown in the report; override it per run with --from.

Pass a worktree branch, or omit it to pick interactively. --check prints a
read-only drift report. Non-interactively (--yes / --output json) it applies only
safe additions; conflicts need --on-conflict and orphans need --prune.

```
wtm env [worktree] [flags]
```

### Options

```
      --check                Read-only drift report; write nothing
      --from string          Override the value source strategy (example, main, parent)
  -h, --help                 help for env
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

