## wtm run job rm

Remove a job from run.toml

### Synopsis

Remove a job from <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing jobs; under --yes the
argument is required.
Fails if anything names the job — a profile, a runner's runs, a job's touches,
an [[env_port]] or an [[env]] link — or if a worktree still holds data in it
(a shared service's namespace, which clean finds by the job's name), unless
--force is given: the references are then stripped, and that data is left
for you to drop by hand.

```
wtm run job rm [name] [flags]
```

### Options

```
      --force           Remove it anyway: strip the profiles, runs, touches, [[env_port]] and [[env]] links naming it
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

