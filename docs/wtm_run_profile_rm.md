## wtm run profile rm

Remove a profile from run.toml

### Synopsis

Remove a profile from <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing profiles; under --yes
the argument is required.
Jobs referenced by the profile are left untouched.

```
wtm run profile rm [name] [flags]
```

### Options

```
  -h, --help            help for rm
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the picker; [name] is then required
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

