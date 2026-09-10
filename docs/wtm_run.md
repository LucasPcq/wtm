## wtm run

Manage dev jobs (services + tasks)

### Synopsis

Run commands and profiles declared in <git-common-dir>/wtm/run.toml — long-running services and one-shot tasks.

### Options

```
  -h, --help   help for run
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal
* [wtm run daemon](wtm_run_daemon.md)	 - Inspect, stop or restart the process that runs the jobs
* [wtm run down](wtm_run_down.md)	 - Stop a worktree's running jobs
* [wtm run export](wtm_run_export.md)	 - Export run.toml as JSON on stdout
* [wtm run import](wtm_run_import.md)	 - Replace run.toml with a JSON run config
* [wtm run init](wtm_run_init.md)	 - Configure the run module (services & tasks) for this repo
* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml
* [wtm run list](wtm_run_list.md)	 - List jobs and profiles declared in run.toml
* [wtm run logs](wtm_run_logs.md)	 - Attach to a job's output
* [wtm run open](wtm_run_open.md)	 - Open a job's URL in the browser
* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml
* [wtm run proxy](wtm_run_proxy.md)	 - Inspect and install the redirection that serves named URLs on port 80
* [wtm run ps](wtm_run_ps.md)	 - List currently running jobs
* [wtm run start](wtm_run_start.md)	 - Start a single job
* [wtm run stop](wtm_run_stop.md)	 - Stop a single job
* [wtm run up](wtm_run_up.md)	 - Start a profile's jobs
* [wtm run url](wtm_run_url.md)	 - Print where a job is reachable in a worktree

