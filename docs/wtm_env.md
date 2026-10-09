## wtm env

Reconcile a worktree's .env against its template and value sources

### Synopsis

Detect and fix .env drift in a worktree: add the keys its sources have and it
lacks, and with --mode refresh settle the values that diverge from the source.
Values come from the strategy the worktree was created with (example, main or
parent); --from overrides it for one run. When run.toml declares ports, the
values wtm owns are then settled on the worktree's isolation.

Pass a worktree, or omit it to pick one. --check reports and writes nothing.
A report prints only what wtm writes (the host:port it moves, owned values);
every other part of a value, secrets included, is withheld unless --show-values.
Unattended (--yes, no terminal, --output json) it applies safe additions only:
conflicts need --on-conflict, orphans --prune.

A run keeps how the worktree runs unless asked: the wizard offers to switch a
worktree between isolated and verbatim (--isolation), and the main checkout
between port and named addresses (--addressing) — see the isolation and
addressing guides.

```
wtm env [worktree] [flags]
```

### Examples

```
  # Pick a worktree and reconcile its .env files
  wtm env

  # Read-only drift report, for a script
  wtm env feat/login --check --output json

  # Also settle the values that diverge, and drop the keys no source has
  wtm env feat/login --mode refresh --on-conflict overwrite --prune --yes

  # Give a worktree created before 0.28 its own ports and compose project
  wtm env feat/login --isolation isolated --yes

  # Reconcile the main checkout, moving its addresses back to ports
  wtm env main --addressing ports --yes
```

### Options

```
      --addressing string    Write the main checkout's linked addresses as ports (as without wtm) or names (served by the run proxy); default: what its .env spells
      --check                Read-only drift report; write nothing
      --from string          Override the value source strategy (example, main, parent)
  -h, --help                 help for env
      --isolation string     Switch the worktree to isolated (its own ports, compose project and namespaces) or verbatim (the values wtm owns back to the source's)
      --mode string          Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts) (default "add")
      --on-conflict string   Conflict resolution with --mode refresh: keep (default) or overwrite
      --output string        Output format: text or json (default "text")
      --prune                Remove orphan keys (present in the .env but in no source)
      --show-values          Print every value whole, secrets included; by default a report shows only what wtm writes
  -y, --yes                  Skip all prompts; resolve every decision from flags and safe defaults (additions only)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

