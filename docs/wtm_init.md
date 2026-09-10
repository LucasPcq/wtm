## wtm init

Initialize wtm configuration

### Synopsis

Interactive wizard to set up global config and project config in <git-common-dir>/wtm/config.toml.
Pass --non-interactive (or any config flag) to bootstrap from flags + auto-detection instead.
Use --only env|hooks|worktrees to re-run init for specific sections and regenerate them cleanly.
Services & tasks are configured separately with `wtm run init`.

```
wtm init [flags]
```

### Options

```
      --base-branch string       Default base branch for new worktrees
      --base-path string         Worktree directory, relative to repo root
      --clean-command string     Command to run before removing a worktree
      --env-strategy string      Env provisioning strategy: example, main, or parent
  -h, --help                     help for init
      --install-command string   Command to run after creating a worktree
      --non-interactive          Bootstrap from flags + auto-detection; never prompt
      --only strings             Re-init only these sections (env, hooks, worktrees); regenerates them cleanly
      --shell string             Global shell: zsh, bash, or fish
      --skip-clean               Skip on_clean hooks config
      --skip-env                 Skip .env provisioning config
      --skip-hooks               Skip on_create hooks config
      --yes                      Skip the re-init confirmation prompt
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

