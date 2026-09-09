## wtm run daemon stop

Stop the daemon, leaving detached services running

### Synopsis

Stop the background daemon.
Foreground services die with it — they are drained through a terminal it owns.
Detached services (those with a stop command) keep running, and the next daemon picks them back up.

```
wtm run daemon stop [flags]
```

### Options

```
  -h, --help            help for stop
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the confirmation
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run daemon](wtm_run_daemon.md)	 - Inspect, stop or restart the process that runs the jobs

