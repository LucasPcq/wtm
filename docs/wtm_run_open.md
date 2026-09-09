## wtm run open

Open a job's URL in the browser

### Synopsis

Hand a job's URL to the desktop's own opener. [worktree] defaults to the current one, and is picked interactively when there is a terminal. Naming the job with --job is required outside a fully interactive run — a picker never runs under a pipe, under --yes or in --output json mode.

```
wtm run open [worktree] [flags]
```

### Options

```
  -h, --help            help for open
      --job string      Job whose URL to open (required outside a fully interactive run)
      --output string   Output format: text or json (default "text")
      --raw             Open the direct http://localhost:<port> address
  -y, --yes             Skip the pickers; --job is then required when several jobs publish a url
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

