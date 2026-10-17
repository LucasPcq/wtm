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
| `10` | the worktree or its path already exists (`create --if-not-exists` turns it into a success when the worktree there holds the requested branch, never when it is another branch's, e.g. `feat-x` for `feat/x`) |
| `11` | the branch does not exist |
| `12` | the repository was never initialized with wtm (`wtm init`) |
| `14` | a job or profile `run.toml` does not declare |
| `15` | `extract`: the changes conflict with the target worktree |
| `16` | no `run.toml` (`wtm run init`) |
| `17` | `upgrade`: this install cannot upgrade itself |
| `18` | `env --check` found drift |
| `19` | cancelled: an interactive run the user backed out of, or any run interrupted by Ctrl+C, SIGINT or SIGTERM (`--yes` included). See [Interrupting a run](#interrupting-a-run) |
| `20` | `events` received an event of a newer schema: upgrade wtm |
| `21` | not in a git repository (the current directory, or `events --repo`) |
| `22` | a state file (`config.toml`, `run.toml`, a worktree's `meta.json`, `pending-removals.toml`) was written by a newer wtm: this one reads it but refuses to change it. Upgrade wtm. See [State files](state.md#versions) |

`wtm resolve <branch>` and `wtm run url --job <name>` print a bare path or URL for `$(…)`: `cd "$(wtm resolve feat/login)"`. Every flag of every command is in `wtm <command> --help` and the [command reference](../wtm.md).

### Interrupting a run

The first Ctrl+C (or SIGINT, SIGTERM) cancels the run; behind a spinner or the run view it shows "Cancelling…" while the work stops. wtm stops between two units of work, never half-way through one:

| What was running | What an interrupt does |
| --- | --- |
| `clean`, `prune` | the worktree being removed is removed all the way (branch, data, event); the next ones are left untouched and listed as skipped, `interrupted` |
| `create`, `checkout`, `extract` | the worktree is created whole or not at all; one created just before the interrupt, or whose ports or `on_create` hooks it stopped, is kept and named; the branches not reached are listed as skipped, `interrupted` |
| a hook | the hook is stopped (its process group gets SIGINT, then SIGTERM, then SIGKILL); the hooks after it never start, `continue_on_error` included, and it is reported as `hook stopped` rather than by its shell's exit status. An `on_clean` hook stopped this way keeps the worktree |
| `sync`, `fast-forward` | a rebase in progress is aborted, the branch left where it was; nothing is pushed, and the branches not reached are reported `cancelled` |
| `run up`, `run start` | the jobs already started, the one being started included, keep running; the recap lists them, `wtm run down` stops them |
| `wtm ui` | quitting it (`q`, Ctrl+C) with a run in flight cancels that run as above and waits for it to stop, then exits `19`; a second Ctrl+C leaves at once |
| a fetch, a `git` talking to a remote | stopped, with what it started (ssh) — never a daemon it left behind on purpose (credential cache, fsmonitor, an ssh `ControlPersist` master) |

The command still writes its report or its JSON document, then exits `19`. Running it again finishes the work. A second Ctrl+C quits at once — once a worktree being created or removed is done — and is then killed by the signal (exit `130` in a shell) unless the report was already written.

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
