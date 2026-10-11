# Integrations

wtm is built to be driven by something other than a person: a script, a CI job, an AI agent, an editor or terminal plugin. Everything a person does through a wizard has an unattended form, and everything wtm prints for a person has a machine form.

```bash
wtm create feat/a feat/b --yes --output json | jq -r '.results[].path'
wtm list --output json | jq -r '.[] | select(.is_dirty) | .branch'
wtm events --output json | jq -c 'select(.type == "worktree.provisioned")'
```

## The contract: `--yes` and `--output json`

- **`--output json`** on every data command writes one JSON document on stdout. Human text, warnings and errors go to stderr, so stdout parses as is.
- **`--yes`** on every command that changes something, or could ask: it never prompts. A decision takes its flag, else a documented safe default that is never destructive (`sync --yes` does not push, `extract --yes` aborts on conflict, `clean --yes` leaves children orphaned unless `--reparent-children`). A required choice with no safe default is an error naming the missing flag, never a picker. JSON mode requires `--yes` on a mutating command: without it, the command exits `2` before doing anything.
- **`--force`** is a separate axis: it lifts safety refusals (dirty, unpushed, open PR, locked, foreign data) and never implies `--yes`.
- **`--quiet`** silences the human report only: errors, the exit code and the JSON still come through.

```console
$ wtm create feat/login --if-not-exists --yes --output json
{
  "results": [
    {
      "branch": "feat/login",
      "path": "/code/.trees/feat-login",
      "metadata": {"source_branch": "main", "created_at": "2026-10-04T16:52:28Z", "env_strategy": "example", "isolation": "isolated"},
      "already_exists": false,
      "existing_branch": false,
      "isolation": "isolated"
    }
  ],
  "failed": []
}
```

Check the exit code first, and parse stdout only when it is non-empty. A command that got far enough to have per-item results (`create`, `clean`, `exec`, `run up`, `sync`, `prune`…) writes its whole document, then exits non-zero, with the failures named in it: `create` and `clean` exit with the first failure's code, `exec` with `1`. One that failed before writes nothing on stdout.

### Exit codes

The codes are stable, and the same for every command:

| Code | Means |
| --- | --- |
| `0` | success |
| `1` | a generic error; `exec` when any command failed |
| `2` | bad usage: an unknown flag or command, a value or an argument that does not parse (`checkout feat/c`), two flags that cannot be combined, too many arguments, `--output json` without `--yes` |
| `10` | the worktree or its path already exists (`create --if-not-exists` turns it into a success) |
| `11` | the branch does not exist |
| `12` | the repository was never initialized with wtm (`wtm init`) |
| `14` | a job or profile `run.toml` does not declare |
| `15` | `extract`: the changes conflict with the target worktree |
| `16` | no `run.toml` (`wtm run init`) |
| `17` | `upgrade`: this install cannot upgrade itself |
| `18` | `env --check` found drift |
| `19` | an interactive run the user backed out of; never under `--yes` |
| `20` | `events` received an event of a newer schema: upgrade wtm |
| `21` | not in a git repository (the current directory, or `events --repo`) |

`wtm resolve <branch>` and `wtm run url --job <name>` print a bare path or URL for `$(…)`: `cd "$(wtm resolve feat/login)"`. Every flag of every command is in `wtm <command> --help` and the [command reference](../wtm.md).

## AI agents: `wtm agents install`

```bash
wtm agents install --yes          # every detected .claude / .cursor destination
wtm agents install --all --yes    # also create the ones that do not exist yet
```

It installs the `using-wtm` skill into Claude Code (`.claude/`) or Cursor (`.cursor/`), in the project or your home directory: the commands, their unattended forms, the exit codes and the JSON shapes, loaded when the agent works with worktrees. Re-run it after upgrading wtm so the skill describes the binary you have. The [agents recipe](recipes.md#several-ai-agents-each-in-its-own-worktree) shows a full session.

## Reacting to changes: `wtm events`

`wtm list --output json` answers once. `wtm events --output json` keeps answering: a snapshot of every worktree, then one JSON line per change (created, provisioned, updated, relocated, reparented, deprovisioned, removed, and each job started, crashed or stopped), whoever made it. Run outside a repository, it follows every repository wtm was used in. It is the way to keep a sidebar, a set of editor windows or terminal panes in step with the worktrees. The full contract is in [The event stream](events.md), with [`wtm version --output json`](events.md#checking-compatibility) to check that the installed wtm speaks it and [`WTM_CORRELATION_ID`](events.md#recognising-your-own-command) to pick out the events of a command you started.

## Built on wtm

[herdr-wtm](https://github.com/LucasPcq/herdr-wtm) runs your worktree commands from a herdr popup and keeps herdr's workspaces in sync with your worktrees, on top of `wtm events` and `WTM_CORRELATION_ID` (early preview).
