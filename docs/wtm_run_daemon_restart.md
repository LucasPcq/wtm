## wtm run daemon restart

Hand the jobs over to a daemon built from this binary

### Synopsis

Stop the running daemon and start one from this binary.
This is the way out of a version mismatch: the daemon is what runs the jobs, so an older one keeps serving its own behavior until it is replaced.
Detached services keep running across the restart and are picked back up; foreground ones are stopped.

```
wtm run daemon restart [flags]
```

### Options

```
  -h, --help            help for restart
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the confirmation
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run daemon](wtm_run_daemon.md)	 - Inspect, stop or restart the process that runs the jobs

