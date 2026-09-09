## wtm upgrade

Update wtm to the latest release

### Synopsis

Bring wtm up to the latest published release, doing the right thing for how it was
installed. A standalone binary is replaced in place after its SHA256 is verified
against the release checksums. A Homebrew or `go install` binary is handed to that
tool instead — replacing a package-manager-owned binary would desynchronize it.
A binary built from source is refused, since no published release corresponds to it.

This updates the CLI itself, not your worktrees — that is `wtm sync`.

--check reports what is available without changing anything. --yes skips the
confirmation (required with --output json). --version pins an explicit release and
applies to standalone installs only.

```
wtm upgrade [flags]
```

### Options

```
      --check            Report whether a newer release exists without installing anything
  -h, --help             help for upgrade
      --output string    Output format: text or json (default "text")
      --version string   Install a specific release instead of the latest (standalone installs only)
  -y, --yes              Skip the confirmation prompt
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

