## wtm run job add

Add a job to run.toml

### Synopsis

Append a job to <git-common-dir>/wtm/run.toml.

Every flag pre-fills the corresponding question, so the form opens on what was
already given. --yes skips the questions altogether: [name] and --cmd are then
required, and every other field falls back to its documented default.

--runs, --touches and --binds-no-port declare how the job relates to the others;
--scope shared and the --namespace-* flags declare a service run once for the
whole repository and each worktree's slice of it. The file is refused exactly
as loading it would refuse it: a namespace only on a shared service, with both a
name and a create command; --runs and --touches naming declared jobs.

--cmd and --stop are /bin/sh lines: quotes, && and ${VAR} behave as in a terminal,
so a declared port can be passed as a flag — --cmd 'pnpm dev --port ${PORT}'.

```
wtm run job add [name] [flags]
```

### Options

```
      --binds-no-port               This service listens on nothing by design, so stop offering it a port
      --cmd string                  Command to run, as a /bin/sh line
      --cwd string                  Working directory (relative to project root)
  -h, --help                        help for add
      --kind string                 Job kind: service or task (default "service")
      --namespace-create string     Command carving the slice out, run on every start of the shared service (must be safe to rerun)
      --namespace-env stringArray   Extra variable for the namespace commands as KEY=VALUE, repeatable ({worktree} and {ordinal} are filled in)
      --namespace-name string       Name of each worktree's slice of a shared service, e.g. app_{worktree}
      --namespace-remove string     Command dropping the slice, run by wtm clean
      --output string               Output format: text or json (default "text")
      --port stringArray            Base port as NAME=PORT, repeatable (e.g. --port PORT=3000)
      --runs stringArray            Declared job this one starts itself, repeatable (a turbo or compose runner)
      --scope string                shared runs one instance for the whole repository; worktree (the default) one per worktree
      --stop string                 Stop command, as a /bin/sh line (services only)
      --touches stringArray         Declared service whose data this job changes (a migration, a reset, a seed), repeatable
      --url-host string             Host segment to publish under, defaulting to the job's name
      --url-port string             Publish this declared port under a name (e.g. --url-port PORT)
  -y, --yes                         Skip all prompts; [name] and --cmd are then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

