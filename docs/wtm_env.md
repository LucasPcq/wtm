## wtm env

Reconcile a worktree's .env against its template and value sources

### Synopsis

Detect and fix .env drift in a worktree: add expected-but-missing keys and
(with --mode refresh) settle values that diverge from the source.

Values come from the strategy the worktree was created with, shown in the report:
example → the template's placeholders, main → the main checkout, parent → the
parent worktree only (main is read only when the parent has no worktree or no such
file). Override it per run with --from.

Pass a worktree branch, or omit it to pick interactively. --check prints a
read-only drift report. Non-interactively (--yes / --output json) it applies only
safe additions; conflicts need --on-conflict and orphans need --prune.

When run.toml declares ports, a second pass follows on the reconciled files of an
isolated worktree: each [[env_port]] link and [[env]] value is settled on its own
ports and namespaces, and COMPOSE_PROJECT_NAME is written for a compose job. An invalid
run.toml skips that pass with a warning; the keys are still reconciled. The main
checkout is a worktree like any other here: `wtm env main` settles it on run.toml's
declared ports, which it never shifts.

A worktree created before the isolation choice existed (no isolation in its
meta.json) keeps its source's ports and COMPOSE_PROJECT_NAME: non-interactively
only its keys are reconciled, and the report says so. The wizard offers to adopt
isolation — a new compose project, so its current volumes are no longer used —
and --isolation isolated adopts it explicitly.

--isolation verbatim puts the values wtm owns (linked ports, [[env]] values,
COMPOSE_PROJECT_NAME) back to the source's and leaves every other key alone: the
worktree then shares its source's compose volumes and data. The wizard shows those
values first, and its recap can also keep a worktree verbatim from then on. Either
isolation is recorded only once the .env is in line with it: a run that fails or
is cancelled records nothing.

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

