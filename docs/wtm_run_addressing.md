## wtm run addressing

Switch how the .env files spell a job's address

### Synopsis

Set run.toml's addressing — named URLs (http://api.feat-x.myrepo.localhost) or
port URLs (http://localhost:4012) — then settle the .env of the worktrees that spell
the other one. Settling runs even when the mode is already the one given, for a
worktree an earlier switch left out of step.

The main checkout is settled back to ports, never onto names: it is the checkout
that works without wtm, and `wtm env main` is how it is moved onto names.

Without an argument, prompts for the mode; under --yes the argument is required
and the worktrees are settled unless --keep-env is passed.

```
wtm run addressing [names|ports] [flags]
```

### Options

```
  -h, --help            help for addressing
      --keep-env        Switch run.toml only, leaving the worktrees' .env files as they are
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the prompts; [names|ports] is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

