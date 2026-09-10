## wtm run proxy install

Serve named URLs on port 80 so they drop their port

### Synopsis

Install a per-user LaunchAgent: launchd binds port 80 on the loopback and hands the socket to wtm, which relays it to the run proxy. No sudo, no system file — everything lives in ~/Library/LaunchAgents and `wtm run proxy uninstall` removes it.

```
wtm run proxy install [flags]
```

### Options

```
      --dry-run         Print every file in full and write nothing
  -h, --help            help for install
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the confirmation
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run proxy](wtm_run_proxy.md)	 - Inspect and install the redirection that serves named URLs on port 80

