# Migrating to 0.28

0.28 turns the `run` module into a per-worktree dev stack and settles its interface. Most of what changed only concerns you if you used `wtm run` or `wtm switch` in 0.27, or script wtm. This page lists what to do, then every breaking change in detail.

## Checklist

1. **Open a new shell** after upgrading (or re-run `eval "$(wtm shell-init)"`). The `wtm` shell function now returns the command's exit code; the old one always returned `0`.
2. **Decide the isolation of each worktree created with 0.27**: see [below](#worktrees-created-with-027).
3. **Stop stacks started by 0.27** once, with `docker compose -p <old-name> down`: they ran under another compose project name, and a new `run up` will not find them.
4. **Re-read your hooks**: they now run through `/bin/sh -c`.
5. **Update scripts and agents**: `--non-interactive` → `--yes`, `wtm switch` → `wtm go` + `wtm run up`, the new [JSON contract](#the-json-contract-of-wtm-run) and [exit codes](#exit-codes). Re-run `wtm agents install` so your agent's skill describes 0.28.

## Worktrees created with 0.27

A worktree created before 0.28 recorded no isolation (its `meta.json` has no `isolation` field). It keeps running on its source's ports and compose project:

- `wtm env <branch> --yes` reconciles its `.env` keys and **touches nothing run-related** (no port shift, no `COMPOSE_PROJECT_NAME`) and says so;
- `wtm run up` and `wtm run start` refuse it, naming the command to run.

Decide once per worktree:

- `wtm env <branch> --isolation isolated` adopts isolation: its own ports and a new compose project. Its current volumes (`<old project>_*`) are no longer used.
- `wtm env <branch> --isolation verbatim` keeps it on its source's values.

The interactive `wtm env` offers the same choice and says what it changes. See [Isolation](isolation.md).

## Hooks

- **Hooks run through `/bin/sh -c`.** They used to be split on spaces and run without a shell: `&&`, pipes, redirections, quotes and `$VAR` now behave as in a terminal. A shell character that was passed literally (`$`, `*`, `;`, `&`, quotes) now means something.
- **Placeholders are quoted for you.** `{{worktree}}`, `{{branch}}`, `{{root}}` and `{{from_branch}}` are quoted for the spot they land in, so a path holding `'` or `$` arrives intact: write `cd {{worktree}}`, without quotes of your own.
- **Run variables reach hooks under two conditions.** `COMPOSE_PROJECT_NAME`, `WTM_*` and the declared ports are passed to a hook only when `run.toml` declares a `docker compose` job **and** the worktree recorded its isolation. Otherwise a hook gets the environment it had in 0.27. A `COMPOSE_PROJECT_NAME` set by the worktree's own `.env` always wins.

## Commands

| 0.27 | 0.28 |
| --- | --- |
| `wtm switch <branch>` | `wtm go <branch>` then `wtm run up` |
| `--non-interactive` (`init`, `run init`) | `--yes` / `-y`, now fully unattended, global config included |
| `init --only <section> --yes` skipped the confirmation | it regenerates the section without the wizard |
| `run up <profile>`, `run start <job>` … | the worktree is the positional (a branch name, never a path), the job or profile a flag: `run up [worktree...] --profile <p>`, `run start [worktree] --job <j>` |
| `run up --profile a --profile b` | one profile per run; with several and no default, `--yes` fails naming `--profile`. A repeated single-value flag is refused |
| `run down --all` stopped every repository's jobs | it stops every worktree of the current repository only |
| `run import` merged into `run.toml` | it replaces the file; `--replace` and its `--force` are gone, and without a terminal it needs `--yes` |
| `run ps` offered an action picker | it lists; `wtm run logs` opens the view that acts on jobs |

Two refusals are new. When `run.toml` declares a job, a worktree whose derived name another live worktree already carries (`feat.x` beside `feat/x`: same compose project, same namespaces, same host) is refused at creation, exit `10`. And `wtm relocate` no longer moves a worktree whose jobs are running (`blocked_jobs`): `wtm run down <branch>` first.

`run.toml` is also validated more strictly: a job or profile name with a space, a `kind` other than `service` / `task`, two port bases a multiple of `port_offset_block` apart, a reference to an undeclared job, or a link on a `.env` file `config.toml` does not provision are refused. A refused file never fails a core command; it only disables the run part, with a warning.

## The JSON contract of `wtm run`

- A job object is keyed `name`; any other object pointing at a job says `job`.
- A worktree is always `branch` + `path` (no more `worktree` or `work_dir`).
- `run up`, `run down`, `run stop` and `run logs` always return an **array of per-worktree documents**, however many worktrees there are. `run stop` is no longer a single object; `run logs` is `[{branch, path, lines: [{job, at, text}]}]`.
- `run ps` returns `branch`, `path` and `project`, `held` for a runner, and no longer `released`.
- `run addressing` returns `settled`, `pending` and `main_left` as `{branch, path}` objects.
- Result statuses are `started`, `joined`, `done`, `stopped`, `released`, `not_running`, `already_running` and `error`. `stopped` is no longer reported for something that was not running. The shared-service status `attached` is now `joined` (an older daemon's `attached` is still read).
- A command with per-job results writes its whole document, then exits non-zero; one that fails before writes nothing on stdout.

## Exit codes

- **2**: a usage error: unknown flag or subcommand (including `wtm run <unknown>`), unreadable flag value, unknown `--output` format, extra argument. It was `1`.
- **14**: a job or profile `run.toml` does not declare, now also for `run job|profile edit|rm` and `run export --profile`. It is checked before the daemon is contacted: nothing was started or stopped.
- **16**: no `run.toml`; the message says to run `wtm run init`.

## The daemon

A 0.27 daemon still running is replaced automatically when it holds no job. When it holds some, `run up` refuses and names `wtm run daemon restart`, which replaces it: detached services survive the restart, foreground ones are stopped.

## For pre-release testers

Since the last `0.28.0-beta`: `attached` → `joined`; `--profile` takes one value; `run export` accepts `--output`; `run start` gains `--exclusive`, `--parallel` and `--no-probe`; no view opens unless stdin **and** stdout are terminals (`run up > run.log` no longer opens it); `Esc` in `run init` prints `= Aborted.` and exits with the abort code.
