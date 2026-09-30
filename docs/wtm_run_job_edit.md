## wtm run job edit

Edit an existing job

### Synopsis

Edit a job declared in <git-common-dir>/wtm/run.toml.

Pass any of --name, --cmd, --kind, --stop, --cwd, --port, --port-clear,
--url-port, --url-host, --runs, --binds-no-port, --touches, --scope,
--namespace-name, --namespace-create, --namespace-remove or --namespace-env to
change those fields and nothing else: a flag left out keeps the field as it is,
and passing an empty string clears it (--stop '' drops the stop command,
--url-port '' withdraws the published name, --namespace-name '' withdraws the
whole [job.namespace]).

--scope shared runs one instance for the whole repository, in the main checkout;
--scope worktree puts it back to one per worktree. A [job.namespace] is only
accepted on a shared service and needs both a name and a create command, which
runs on every start and so must be safe to run again. --runs and --touches name
declared jobs; the file is refused exactly as loading it would refuse it.

--port merges into the ports the job already declares, so one entry can be
changed without rewriting the others; --port-clear empties the table.
--name also rewrites what names this job elsewhere in the file: the profiles,
the runners' runs, the touches, and the [[env_port]] and [[env]] links. It is
refused while a worktree holds data in the job, which clean finds by its name.

With no such flag, the form opens pre-filled with the current values, and
without an argument it prompts to pick from the existing jobs.

```
wtm run job edit [name] [flags]
```

### Options

```
      --binds-no-port               This service listens on nothing by design, so stop offering it a port (--binds-no-port=false to undo)
      --cmd string                  Command to run, as a /bin/sh line
      --cwd string                  Working directory relative to project root (pass '' to drop it)
  -h, --help                        help for edit
      --kind string                 Job kind: service or task
      --name string                 Rename the job, updating the profiles, runs, touches, [[env_port]] and [[env]] links that name it
      --namespace-create string     Command carving the slice out, run on every start of the shared service (must be safe to rerun)
      --namespace-env stringArray   Extra variable for the namespace commands as KEY=VALUE, repeatable — replaces the table (pass '' to drop it)
      --namespace-name string       Name of each worktree's slice of a shared service (pass '' to withdraw the whole [job.namespace])
      --namespace-remove string     Command dropping the slice, run by wtm clean (pass '' to drop it)
      --output string               Output format: text or json (default "text")
      --port stringArray            Base port as NAME=PORT, repeatable — merged into the declared ports
      --port-clear                  Drop every port this job declares
      --runs stringArray            Declared job this one starts itself, repeatable — replaces the list (pass '' to drop it)
      --scope string                shared runs one instance for the whole repository; worktree one per worktree
      --stop string                 Stop command, as a /bin/sh line (pass '' to drop it)
      --touches stringArray         Declared service whose data this job changes (a migration, a reset, a seed), repeatable — replaces the list (pass '' to drop it)
      --url-host string             Host segment to publish under (pass '' to fall back to the job's name)
      --url-port string             Publish this declared port under a name (pass '' to withdraw the url)
  -y, --yes                         Skip all prompts; a field flag is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

