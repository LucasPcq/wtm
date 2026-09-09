## wtm run url

Print where a job is reachable in a worktree

### Synopsis

Write a job's URL on stdout and nothing else, for $(…). [worktree] defaults to the current one, and no picker ever opens here — an ambiguity is an error naming --job. --raw prints the job's own port instead of its name, which every OS resolves and no proxy has to serve.

```
wtm run url [worktree] [flags]
```

### Options

```
  -h, --help            help for url
      --job string      Job whose URL to print (required when several jobs publish one)
      --output string   Output format: text or json (default "text")
      --raw             Print the direct http://localhost:<port> address
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

