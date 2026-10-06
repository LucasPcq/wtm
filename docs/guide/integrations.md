# Integrations

wtm is built to be driven by something other than a person: a script, a CI job, an AI agent, an editor or terminal plugin. Everything a person does through a wizard has an unattended form, and everything wtm prints for a person has a machine form.

```bash
wtm create feat/a feat/b --yes --output json | jq -r '.results[].path'
wtm list --output json | jq -r '.[] | select(.is_dirty) | .branch'
wtm events --output json | jq -c 'select(.type == "worktree.provisioned")'
```

| You want to | Use |
| --- | --- |
| run a command without prompts and parse its result | [`--yes` and `--output json`](#the-contract---yes-and---output-json) |
| teach an AI agent the commands | [the `using-wtm` skill](#ai-agents-wtm-agents-install) |
| react to worktrees changing, whoever changed them | [`wtm events`](#reacting-to-changes-wtm-events) |
| know whether the installed wtm speaks your contract | [`wtm version --output json`](#checking-the-installed-version) |
| tell your command's events from everyone else's | [`WTM_CORRELATION_ID`](#tying-events-to-your-command-wtm_correlation_id) |

## The contract: `--yes` and `--output json`

- **`--output json`** on every data command writes one JSON document on stdout. Human text, warnings and errors go to stderr, so stdout parses as is.
- **`--yes`** on every command that changes something, or could ask: it never prompts. A decision takes its flag, else a documented safe default that is never destructive (`sync --yes` does not push, `extract --yes` aborts on conflict, `clean --yes` leaves children orphaned unless `--reparent-children`). A required choice with no safe default is an error naming the missing flag, never a picker. JSON mode requires `--yes` on a mutating command.
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

Check the exit code first, and parse stdout only when it is non-empty. A command that got far enough to have per-item results (`create`, `clean`, `exec`, `run up`, `sync`, `prune`…) writes its whole document, then exits non-zero, with the failures named in it: `create` and `clean` exit with the first failure's code, `exec` with `1`. One that failed before writes nothing on stdout. The exit codes are stable:

| Code | Means |
| --- | --- |
| `0` | success |
| `1` | a generic error; `exec` when any command failed |
| `2` | bad usage: an unknown flag or command, a value that does not parse, two flags that cannot be combined, too many arguments |
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

`wtm list --output json` answers once. `wtm events --output json` keeps answering: a snapshot of every worktree, then one JSON line per change (created, provisioned, updated, relocated, reparented, deprovisioned, removed), whoever made it. Run outside a repository, it follows every repository wtm was used in. It is the way to keep a sidebar, a set of editor windows or terminal panes in step with the worktrees. The full contract, the events and the exit codes are in [The event stream](events.md).

## Checking the installed version

```console
$ wtm version --output json
{
  "version": "0.29.0",
  "events": 1
}
```

`version` is the binary's version (`dev` for a local build), and every other key is the version of a contract an integration reads; today `events`, the schema version of `wtm events`. Ignore keys you do not know. `wtm version` exiting `2`, or a missing `events` key, means a wtm older than the stream: ask the user to upgrade (`wtm upgrade`).

## Tying events to your command: `WTM_CORRELATION_ID`

```bash
WTM_CORRELATION_ID=popup-42 wtm create feat/login --yes
```

Every event that command publishes, including those of its hooks' own wtm commands, carries `"correlation_id":"popup-42"`, so a host can wait for *its* `worktree.provisioned` while an agent works in the next pane. Any string up to 256 bytes without a control character; anything else exits `2` before the command does anything. A complete example is in [End to end](events.md#end-to-end-create-a-worktree-and-open-it-once-it-is-ready).

## Built on wtm

[herdr-wtm](https://github.com/LucasPcq/herdr-wtm) runs your worktree commands from a herdr popup and keeps herdr's workspaces in sync with your worktrees, on top of `wtm events` and `WTM_CORRELATION_ID` (early preview).
