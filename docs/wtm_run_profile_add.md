## wtm run profile add

Add a profile to run.toml

### Synopsis

Append a profile to <git-common-dir>/wtm/run.toml.

Every flag pre-fills the corresponding question, so the form opens on what was
already given. --yes skips the questions altogether: [name] and --jobs are then
required, and the profile is not the default unless --default says so.

```
wtm run profile add [name] [flags]
```

### Options

```
      --default         Mark this profile as the default
  -h, --help            help for add
      --jobs strings    Comma-separated existing job names, in start order
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip all prompts; [name] and --jobs are then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

