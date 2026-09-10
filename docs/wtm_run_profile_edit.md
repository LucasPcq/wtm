## wtm run profile edit

Edit an existing profile

### Synopsis

Edit a profile declared in <git-common-dir>/wtm/run.toml.

Pass --name, --jobs or --default to change those fields and nothing else: a
flag left out keeps the field as it is. --jobs replaces the whole list — its
order is the start order, so it is given in full — and --default=false takes
the default away without handing it to another profile.

With no such flag, the form opens pre-filled with the current values, and
without an argument it prompts to pick from the existing profiles.

```
wtm run profile edit [name] [flags]
```

### Options

```
      --default         Mark this profile as the default (--default=false takes it away)
  -h, --help            help for edit
      --jobs strings    Comma-separated existing job names, in start order (replaces the list)
      --name string     Rename the profile
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip all prompts; a field flag is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

