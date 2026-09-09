## wtm run job add

Add a job to run.toml

### Synopsis

Append a job to <git-common-dir>/wtm/run.toml.

Every flag pre-fills the corresponding question, so the form opens on what was
already given. --yes skips the questions altogether: [name] and --cmd are then
required, and every other field falls back to its documented default.

--cmd and --stop are /bin/sh lines: quotes, && and ${VAR} behave as in a terminal,
so a declared port can be passed as a flag — --cmd 'pnpm dev --port ${PORT}'.

```
wtm run job add [name] [flags]
```

### Options

```
      --cmd string         Command to run, as a /bin/sh line
      --cwd string         Working directory (relative to project root)
  -h, --help               help for add
      --kind string        Job kind: service or task (default "service")
      --output string      Output format: text or json (default "text")
      --port stringArray   Base port as NAME=PORT, repeatable (e.g. --port PORT=3000)
      --stop string        Stop command, as a /bin/sh line (services only)
      --url-host string    Host segment to publish under, defaulting to the job's name
      --url-port string    Publish this declared port under a name (e.g. --url-port PORT)
  -y, --yes                Skip all prompts; [name] and --cmd are then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

