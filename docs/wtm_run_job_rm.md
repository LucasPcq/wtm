## wtm run job rm

Remove a job from run.toml

### Synopsis

Remove a job from <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing jobs; under --yes the
argument is required.
Fails if the job is referenced by any profile, unless --force is given
(in which case the references are stripped from those profiles too).

```
wtm run job rm [name] [flags]
```

### Options

```
      --force           Also strip references from profiles that use this job
  -h, --help            help for rm
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the picker; [name] is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

