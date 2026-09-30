---
name: using-wtm
description: Use this skill whenever the user works with git worktrees in any way (create, list, switch to, clean or prune them), wants to split an oversized PR or move uncommitted changes into another worktree, wants to start, stop, inspect or configure per-worktree dev jobs, services, ports, URLs or docker compose stacks, wants to check out a GitHub pull request into its own worktree, or manages stacked branches (rebase a chain onto its parent, reparent after a merge, fast-forward from origin). Trigger even when the user never says "wtm": if the repository has wtm set up, these tasks go through it. Always pass --output json on wtm data commands and --yes on anything that would prompt; never drive an interactive picker and never launch the `wtm ui` dashboard.
---

# Using wtm

wtm manages **git worktrees** (one branch, one directory, provisioned `.env` and hooks), **per-worktree dev jobs** (long-running services and one-shot tasks, run by a background daemon on ports and names isolated per worktree) and **GitHub pull requests**. It is built to be driven by an LLM: every data command takes `--output json` and prints machine-parseable results on stdout, while human messages stay on stderr.

This file holds the rules that apply to every command. The detail of each area lives in `references/`, loaded on demand (routing table below). wtm also documents itself:

- `wtm <cmd> --help` gives the full, always-current flags of any command.
- `wtm <cmd> --output json` run once shows a command's exact JSON schema. The payload mirrors wtm's Go structs (stable `snake_case` fields): trust what you see over any field list you remember.

## Driving rules

You drive wtm without a terminal a human is watching, so everything below exists to keep a command from waiting on input nobody can give, or from doing something the user did not ask for.

1. **Always pass arguments.** Without one, most commands drop into an interactive picker you cannot navigate. Get the branch, PR number, profile or job name from a discovery call first (see below). A required selection with no safe default is never guessed on your paths: the command errors naming the missing flag instead of opening a picker.
2. **Never launch a full-screen surface.** Two exist: `wtm ui` (the worktree dashboard) and the **run view** that `wtm run up`, `wtm run start --job <service>` and `wtm run logs` open on a terminal. Both hold the terminal until someone presses `q`, and you can neither read them nor leave them. Use `wtm list --output json` or `wtm tree` instead of `wtm ui`, and pass `-d` (or `--output json`) to every `run up` / `run start`. wtm opens no view under `--output json` or unless both stdin and stdout are a terminal, but do not rely on that. Suggest `wtm ui` to the user when they want to browse worktrees themselves.
3. **Always add `--output json` on data commands.** JSON goes to stdout; human text and warnings go to stderr, which you can ignore unless the exit code is non-zero. wtm may print a one-line update notice on stderr (at most once a day, never under `--output json`, never in CI or without a TTY); it never touches stdout.
4. **`--yes` on every command that changes state or could ask something.** JSON mode is non-interactive, so a mutating command (`create`, `clean`, `prune`, `sync`, `fast-forward`, `relocate`, `reparent`, `extract`, `checkout`, `env`, the `run` commands that can ask, `upgrade`) errors under `--output json` without `--yes`. `--yes` resolves every confirmation or decision from its flag, else from a documented **safe default that is never destructive** (for example `sync --yes` does not push, `extract --yes` aborts on conflict, `clean --yes` leaves children orphaned). Read-only commands (`list`, `tree`, `resolve`, `config show`, `run list`, `run ps`, `run logs`, `env --check`) take `--output json` with no `--yes`.
5. **`--force` is a separate axis: safety, not confirmation.** It only lifts safety refusals (dirty, unpushed, open PR, locked, foreign data) and never implies `--yes`; `--force` alone is rejected in JSON mode. **Never add `--force` on your own initiative**: it exists so that a destructive action is always the user's explicit choice.
6. **`--quiet`** silences the human report and nothing else: the exit code, the errors and every machine contract (`--output json`, `resolve`, `run url`, `shell-init`, `run export`) still come through. It is the output axis only and does not stop prompts, so pair it with `--yes` on a mutating command.
7. **Trust exit codes, then parse.** `0` is success; the granular codes below let you branch precisely. On failure, surface the stderr text. Check the exit code, and parse stdout only when it is non-empty (a command that failed before producing results writes nothing on stdout).
8. **Operations are idempotent, so retrying is safe.** `create --if-not-exists` no-ops on an existing worktree, `clean` no-ops on an absent one, `run up` / `run down` / `run stop` re-run cleanly.
9. **Do not run on your own initiative** what reaches beyond the repository: `wtm upgrade` (replaces the binary you are driving), `wtm run proxy install` (installs a LaunchAgent in the user's home). Propose them; the user decides.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | generic error |
| `2` | bad usage: an unknown flag or command, a flag value that does not parse, an unknown `--output` format, too many arguments |
| `10` | worktree (or its path) already exists, or the branch is checked out in another worktree, or (with run jobs declared) its derived name is taken |
| `11` | branch not found |
| `12` | config not found: repo not initialized (`wtm init`) |
| `14` | the job or profile named (`--job`, `--profile`, `run job|profile edit|rm <name>`) is not declared in `run.toml`; checked before anything is asked of the daemon, so nothing was started or stopped |
| `15` | `extract`: selected changes conflict with the target worktree |
| `16` | no run.toml (no job or profile declared): run `wtm run init` |
| `17` | `upgrade`: this install cannot be upgraded (built from source, or the binary is not writable) |

## Discover names before you act

| Goal | Command |
|---|---|
| All worktrees (branch, path, PR, services, dirty?) | `wtm list --output json` |
| Worktree forest (parent to child, which need sync) | `wtm tree --output json` |
| Open PRs | `gh pr list --json number,title,headRefName,state,isDraft,url` |
| Declared jobs and profiles | `wtm run list --output json` |
| Jobs running right now, every repo | `wtm run ps --output json` |
| Where a job answers in a worktree | `wtm run url [worktree] --output json` |
| What serves the named URLs (bind port, public port, redirection) | `wtm run proxy status --output json` |
| What a job printed | `wtm run logs [worktree] --job <name> --output json` |
| Resolved project config | `wtm config show --output json` |
| A branch's worktree path | `wtm resolve <branch> --output json` |

## Which reference to read

Open the reference **before** running a command from its area: each one lists the flags that are required on your paths, the refusals to expect and the traps specific to its commands.

| You want to | Read |
|---|---|
| Create, list, remove (`clean`, `prune`), relocate a worktree; split changes into another worktree (`extract`); fix `.env` drift (`env`); check out a PR (`checkout`); get a path (`resolve`, `go`); set wtm up (`init`, `config`, `upgrade`, `agents`) | `references/worktrees.md` |
| Work with stacked branches: `tree`, `sync`, `reparent`, `fast-forward` | `references/stacks.md` |
| Run, stop or inspect dev jobs: `run up`, `down`, `start`, `stop`, `logs`, `ps`, `url`, `open`, `list`, the daemon, the proxy, statuses, concurrency | `references/run.md` |
| Configure dev jobs: `run init`, `run job add|edit|rm`, `run profile`, `run addressing`, `run export|import`, and what `run.toml` means (ports, isolation, shared services, namespaces, `touches`, `[[env_port]]`, `[[env]]`, named URLs) | `references/run-config.md` |
| Parse a command's JSON output, or branch on a `status` value | `references/json.md` |

Before any `wtm run …` command, read `references/run.md`; before changing a job, a profile or `run.toml`, read `references/run-config.md`. Never edit `run.toml` by hand: every field has a flag, and a write is refused exactly as loading the file would refuse it.

## Failure handling

On a non-zero exit, read stderr, then:

- `2`: fix the invocation; check `wtm <cmd> --help`.
- `10`: the path is taken, or the branch is checked out in another worktree. Get its path with `wtm resolve <branch> --output json` (the user enters it with `wtm go <branch>`), or pick a different branch name. `--if-not-exists` on `create` turns this into a success returning the existing worktree's path. With run jobs declared it can also mean two branches reduce to the same name (`feat.x` beside `feat/x`): pick another branch name.
- `11`: wrong name; re-run the relevant discovery call.
- `12`: repo not initialized. Run `wtm init --yes` with flags (see `references/worktrees.md`), or ask the user to run the interactive `wtm init`.
- `14`: the job or profile is not declared; check `wtm run list --output json`.
- `15`: `extract` changed nothing; see `references/worktrees.md` for the retry.
- `16`: run `wtm run init --yes` to create `run.toml` (see `references/run-config.md`), then re-run the command.
- `17`: nothing to retry. Report the message: a source build updates with `git pull && make install`, an unwritable binary needs the user to re-run with sudo.
- `gh: …` on stderr: `gh` is not authenticated; tell the user to run `gh auth login`.
- A `run up` / `run down` that exited non-zero still wrote its whole document: the entries with `status: "error"` name the failing job, with `message`, `output` and `exit_code` (see `references/json.md`).
- A `sync` that exited non-zero: some branch is `conflict` or `error` (see `references/stacks.md`).

## Escalate to the user when

- A command needs shell integration (`wtm go` changes the shell's directory; you cannot drive it).
- The user wants to *browse* worktrees rather than get an answer from you: tell them to run `wtm ui` themselves.
- A destructive action (`clean` / `prune --force`, `run up --force` on foreign data, `run job rm --force`) was not explicitly authorized.
- A choice has no safe default and only the user knows the answer: the parent a pre-existing branch stacks on (`--from`), `isolated` vs `verbatim` for a worktree created before the isolation choice, adopting a new compose project (its current volumes stop being used), patching compose files that pin volume names (the new volumes start empty).
- `wtm config edit` is the natural answer: ask the user to run it, or read with `wtm config show` and write the change to the printed path if you have a file-edit tool and the user authorized it.
- You cannot supply a value that `wtm init --yes` requires.
- A finding is about third-party config wtm never edits (`turbo.json` pass-through, Next `allowedDevOrigins`, a command that ignores its port): report it with the fix wtm printed.
