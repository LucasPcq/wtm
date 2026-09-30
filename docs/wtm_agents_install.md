## wtm agents install

Install the using-wtm skill into .claude / .cursor

### Synopsis

Detects which skill destinations exist (project and home-level .claude and .cursor)
and installs the using-wtm skill into the ones you pick.

```
wtm agents install [flags]
```

### Examples

```
  wtm agents install

  # Every detected destination, no questions
  wtm agents install --yes

  # Also create the ones that don't exist yet, and report as JSON
  wtm agents install --all --yes --output json
```

### Options

```
      --all             Include destinations that don't yet exist (creates skill dirs)
  -h, --help            help for install
      --output string   Output format: text or json (default "text")
      --yes             Non-interactive: install into every detected destination
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm agents](wtm_agents.md)	 - Manage LLM agent integrations for wtm

