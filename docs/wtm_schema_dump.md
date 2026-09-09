## wtm schema dump

Write embedded schemas to <state-dir>/schemas/ (or the global config's schemas/ with --global)

### Synopsis

Extract every JSON Schema bundled with this wtm binary so editors can resolve the `#:schema` directives in your TOML files.
Project schemas land in <git-common-dir>/wtm/schemas/. Use --global to write the global schema next to the global wtm config, whose path `wtm run proxy status` prints.

```
wtm schema dump [flags]
```

### Options

```
      --global   Write the global config schema instead of the project ones
  -h, --help     help for dump
```

### Options inherited from parent commands

```
  -q, --quiet   Silence human output; errors and the exit code are unaffected, and --output json still emits its document
```

### SEE ALSO

* [wtm schema](wtm_schema.md)	 - Inspect or extract bundled JSON Schemas

