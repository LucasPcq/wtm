## wtm

Orchestrate git worktrees and team dev workflows from the terminal

```
wtm [flags]
```

### Examples

```
  # Once per repository
  wtm init

  # A worktree per branch, then jump into it
  wtm create feat/login
  wtm go feat/login

  # Every worktree, its PR and its services, on one screen
  wtm ui
```

### Options

```
  -h, --help    help for wtm
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm agents](wtm_agents.md)	 - Manage LLM agent integrations for wtm
* [wtm checkout](wtm_checkout.md)	 - Create a worktree from an existing pull request
* [wtm clean](wtm_clean.md)	 - Remove worktrees and their local branches
* [wtm config](wtm_config.md)	 - Inspect or edit the project wtm config
* [wtm create](wtm_create.md)	 - Create one or more worktrees
* [wtm env](wtm_env.md)	 - Reconcile a worktree's .env against its template and value sources
* [wtm events](wtm_events.md)	 - Stream the repository's worktree changes as they happen
* [wtm exec](wtm_exec.md)	 - Run one command in several worktrees, in parallel
* [wtm extract](wtm_extract.md)	 - Move uncommitted changes to another worktree
* [wtm fast-forward](wtm_fast-forward.md)	 - Advance worktree branches to their origin counterpart
* [wtm go](wtm_go.md)	 - Switch to a worktree
* [wtm init](wtm_init.md)	 - Initialize wtm configuration
* [wtm list](wtm_list.md)	 - List all worktrees
* [wtm prune](wtm_prune.md)	 - Remove finished worktrees (merged, closed PR or gone) in one pass
* [wtm relocate](wtm_relocate.md)	 - Move worktrees to align with base_path and adopt external ones
* [wtm reparent](wtm_reparent.md)	 - Change the parent one or more worktrees are rebased onto
* [wtm resolve](wtm_resolve.md)	 - Resolve a branch to its worktree path
* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)
* [wtm schema](wtm_schema.md)	 - Inspect or extract bundled JSON Schemas
* [wtm shell-init](wtm_shell-init.md)	 - Generate shell integration function
* [wtm sync](wtm_sync.md)	 - Rebase selected worktrees onto their parent, in cascade
* [wtm tree](wtm_tree.md)	 - Show the worktree forest (parent → child)
* [wtm ui](wtm_ui.md)	 - Open the worktree dashboard
* [wtm upgrade](wtm_upgrade.md)	 - Update wtm to the latest release

