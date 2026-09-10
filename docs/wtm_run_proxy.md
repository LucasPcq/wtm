## wtm run proxy

Inspect and install the redirection that serves named URLs on port 80

### Synopsis

Named job URLs carry the run proxy's port unless port 80 is redirected to it. These commands report that redirection and install or remove it.

### Options

```
  -h, --help   help for proxy
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)
* [wtm run proxy install](wtm_run_proxy_install.md)	 - Serve named URLs on port 80 so they drop their port
* [wtm run proxy status](wtm_run_proxy_status.md)	 - Report what actually serves named URLs on this machine
* [wtm run proxy uninstall](wtm_run_proxy_uninstall.md)	 - Remove the redirection and give named URLs their port back

