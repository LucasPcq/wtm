## wtm tree

Show the worktree forest (parent → child)

### Synopsis

Render the forest of managed worktrees, parents above their children, with the
orchestration signals that matter for a stacked-branch workflow: commits ahead
(↑N), uncommitted changes (⚠ dirty), and "needs sync" when a parent has moved and
the child must be rebased. Parents with no worktree appear as greyed virtual roots.

--with-prs adds PR numbers and merged/closed markers (fetched eagerly). --output
json emits the structured tree for agents; --output mermaid emits a flowchart to
paste into a PR or Notion.

```
wtm tree [flags]
```

### Options

```
  -h, --help            help for tree
      --output string   Output format: text or json (default "text")
      --with-prs        Include GitHub PR info (open/merged/closed; fetched eagerly)
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

