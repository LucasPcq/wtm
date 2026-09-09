## wtm run daemon

Inspect, stop or restart the process that runs the jobs

### Synopsis

Jobs are started by a background daemon shared by every repository.
It exits on its own once no foreground job is left; detached services keep running without it and are picked back up by the next one.

### Options

```
  -h, --help   help for daemon
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)
* [wtm run daemon restart](wtm_run_daemon_restart.md)	 - Hand the jobs over to a daemon built from this binary
* [wtm run daemon status](wtm_run_daemon_status.md)	 - Report whether a daemon is running, and which build it is
* [wtm run daemon stop](wtm_run_daemon_stop.md)	 - Stop the daemon, leaving detached services running

