# Worktrees: lifecycle, `.env`, PRs, setup

Everything here assumes the driving rules of `SKILL.md`: `--output json` on data commands, `--yes` on anything that changes state, `--force` only when the user asked. Output shapes are in `json.md`.

## Contents

- [Inventory: `list`](#inventory-list)
- [`create`](#create)
- [An existing local branch](#an-existing-local-branch)
- [Isolation at creation](#isolation-at-creation)
- [Remembered answers](#remembered-answers)
- [`clean` and `prune`](#clean-and-prune)
- [`extract`](#extract)
- [`env`](#env)
- [`exec`](#exec)
- [`relocate`](#relocate)
- [`checkout` (GitHub PRs)](#checkout-github-prs)
- [Navigate: `resolve`, `go`, `ui`](#navigate-resolve-go-ui)
- [Setup: `init`, `config`, `upgrade`, `agents`](#setup-init-config-upgrade-agents)

## Inventory: `list`

`wtm list --output json` is the flat inventory (branch, path, PR, services, dirty). Use `wtm tree` instead when the parent to child hierarchy matters (see `stacks.md`). Both expose an `origin` object describing divergence from `origin/<branch>`, distinct from `commits_ahead`, which counts commits against the **parent/base** branch.

## `create`

`wtm create <branch>... --from <base> --yes --output json` makes one worktree per branch, one after the other, provisions each `.env` and runs the `on_create` hooks. Once the run starts, the JSON is always an envelope, even for one branch: `{"results": [<create result>...], "failed": [{"branch", "path"?, "error", "exit_code"}...]}`. Every field named below lives on a branch's entry in `results`; read `.results[0]` for a single branch.

- **Several branches in one call** share `--from`, `--env-from`, `--isolation` and `--ff`. A failing branch does not stop the others: it lands in `failed` (with `path` when the worktree exists but a hook failed), the others in `results`, and the exit code is the first failure's. A branch listed twice is refused before anything is created (exit `2`), as are two branches reducing to the same name when `run.toml` declares jobs (exit `10`). Either refusal writes no envelope, only the error on stderr. Under `--ff`, the shared source is fast-forwarded once, and each existing branch of the list on its own (best effort, even when the source was already up to date).

- `--from` accepts a remote ref (`origin/x`).
- `--if-not-exists` makes it idempotent. It is about the **worktree**, not the branch: an existing branch with no worktree is still created. It also no-ops on a worktree holding the branch outside `base_path` and returns that path, even the **main** worktree's own path if the branch is checked out there (still `already_exists: true` in its `results` entry, just not a directory under `base_path`).
- `--ff` fast-forwards a behind-only `--from` branch to origin first, so the worktree starts up to date. A diverged branch is left as is (no prompt in JSON mode).
- `--isolation isolated|verbatim`: see [Isolation at creation](#isolation-at-creation).
- **A name two worktrees would share is refused** (exit `10`, before anything is created) when `run.toml` declares a job: `feat.x` next to a live `feat/x` would get the same compose project, namespaces and proxy host (`feat-x`), and `feat/a_b` next to `feat/a-b` the same host. The error names both branches and the shared name: pick another branch name. The same holds for `extract --to` and `checkout`.
- **The run module never fails a creation.** A `run.toml` that cannot be read or is refused (an unknown key, a job `kind` other than `service`/`task`, a global `[proxy] port` outside 1-65535), or a neighbour's unreadable `meta.json`, only skips the port pass: the worktree is created, the `.env` copied as is, the hooks run, and the JSON carries a `warnings` array naming the cause and "ports not settled, run `wtm env <branch>` once run.toml is fixed". Fix the cause, then run that command. An `[[env_port]]` or `[[env]]` on a file `config.toml` does not provision skips only that link: the rest of the pass runs and `warnings` names the link as ignored.
- On an isolated worktree, `create` settles the `.env` port values onto the ports the worktree binds (the `[[env_port]]` pass, see `run-config.md`), and reports it as `env_ports` in the branch's `results` entry.

## An existing local branch

An existing local branch is not an obstacle, and there is no `wtm clean` to run first. `create <branch>` checks out a same-named local branch **as is**, keeping commits that were never pushed; it never deletes or resets it. The branch's `results` entry sets `existing_branch: true` and `origin_state` (`up-to-date`/`behind`/`ahead`/`diverged`) so you can tell reuse from creation.

What wtm will not do is **guess its parent**. The branch was created outside wtm, so `--from` stops being a start point and names the **parent to record for `wtm sync`**, and it is **required**: the command errors instead of guessing, because `sync` and `tree` would treat the guess as fact. Ask the user which branch it stacks on if you do not know. `--ff` then updates `<branch>` itself rather than the source.

Only a branch **another worktree already holds** is refused (exit `10`).

## Isolation at creation

`create`, `extract` and `checkout` take `--isolation isolated|verbatim`: how the new worktree stands against its source. Without it, your paths take a [remembered answer](#remembered-answers), else `run.toml`'s `isolation`, else `isolated`.

- `isolated`: wtm writes the worktree's own ports, `COMPOSE_PROJECT_NAME` and `[[env]]` namespaces into its `.env`, and runs its jobs on the same shifted ports.
- `verbatim`: the `.env` stays byte for byte as copied, and the worktree's jobs run on its source's ports. Such a worktree **cannot run while its source does**.

`--isolation` only answers a creation. On a worktree `--if-not-exists` found already there, or an existing `extract --to` target, it is ignored; when it differs from the worktree's, a `warnings` entry says so and names `wtm env <branch> --isolation …`. The full model is in `run-config.md` (Isolation).

## Remembered answers

In the interactive wizard of `create`, `checkout` and `extract`, the user can tick "Always use this answer in this repo" on three questions: the env strategy, the isolation and whether to fast-forward a source behind origin. The answer lands in `config.toml` under `[wizard.remembered]` (`env_strategy`, `isolation`, `source_update`), and **it applies under `--yes` and `--output json` too**: it stands in for the flag you did not pass, ahead of the config default.

- **For a guaranteed result, pass the flag**: `--env-from`, `--isolation`, `--ff`. A flag always wins over a remembered answer.
- The JSON says what settled each of these answers: `origins` (see `json.md`). `"remembered"` means the user's memory chose, not your command.
- `--ask` ignores the memory for one run: under `--yes` the answers fall back to the config and the safe defaults. It forgets nothing.
- Never edit `[wizard.remembered]` on your own initiative: it is the user's preference. Nothing destructive can be remembered.

## `clean` and `prune`

`wtm clean <branch>... --yes --output json` removes one or more worktrees and their local branches (an absent one is reported `already_absent`, not failed). A failure on one does not stop the others. `wtm prune [filters] --yes --output json` batch-removes finished worktrees.

**Which worktrees `prune` finds finished.** It reads **GitHub PR state via the `gh` CLI**, not local commits: `--merged` (PR merged), `--closed` (PR closed without merging), `--gone` (remote branch deleted); no filter means all three. `--merged` and `--closed` need `gh` installed and authenticated: without it they match nothing and prune prints a notice on stderr. Only `--gone` works offline. JSON `reason` values are `pr_merged` / `pr_closed` / `gone` (there is no plain `merged`).

**Unsafe worktrees are refused** (locked, dirty, unpushed commits, or an open PR) unless `--force`. For `clean` under `--yes`, one unsafe worktree refuses the **whole** run before anything is removed: pass `--force`, or name only the safe ones. For `prune` they are reported under `skipped` (reason `locked`/`dirty`/`unpushed`/`open_pr`) instead of being removed, so committed work is never silently lost. For `prune`, when **every** match is unsafe, nothing survives the selection and the JSON is empty (`pruned: []` and `skipped: []`): read an empty result as "nothing was removed", not "nothing matched".

**Children.** Under `--yes`/JSON, surviving children are left orphaned unless you pass `--reparent-children` (they reparent onto their nearest ancestor that is not removed, the base when none is left).

**Each removal runs in a fixed order**: the worktree's jobs are stopped and checked gone, its `on_clean` hooks run (e.g. `docker compose down`), git removes the worktree, and **only then** is its data dropped and its hold on the shared services released. Any failure before the removal leaves the data intact.

- A job that will not stop (a daemon of another wtm build: the error says `wtm run daemon restart`) refuses the removal unless `--force`.
- A hook that exits non-zero aborts it unless its entry sets `continue_on_error`.
- `prune` runs the whole sequence on one worktree before the next and **stops at the first that fails**: the ones before are gone with their data, it and the ones after keep theirs, the JSON names it under `failed`, and the exit code is non-zero.
- If `git worktree remove` fails on undeletable files (e.g. root-owned Docker files), interactive runs offer a `sudo rm -rf` fallback; otherwise, since git has already forgotten the worktree, the removal is completed (branch deleted, data dropped) and a warning names the leftover directory to delete.
- A locked worktree (`git worktree lock`, often guarding a network mount or removable drive) is refused like the others, and `--force` lifts the lock to remove it: never pass `--force` on one without the user's explicit go-ahead.
- `clean`, `prune` and `run down` start a daemon by themselves when its index holds something for the worktree they act on.

**Shared-service data.** Both commands also give back the namespaces the removed worktrees carved out of shared services (they drop their databases). Only worktrees that actually started the shared job owe anything; one created and thrown away owes nothing.

- `--keep-data` withholds the drop, including under `--yes`. The interactive recap names each database it will drop.
- If a shared service is down, the interactive form asks (before the recap) whether to start it and drop now, or keep the data until the service next starts. Under `--yes` it is kept, and the drop is paid the next time wtm starts the service: any `run up` / `run start` that brings it up, or `prune`, which also reports what is still owed. A service started outside wtm pays nothing.
- **Pass `--drop-data` when the user wants the data gone**: it drops it now, starting the services that are down (and letting them go again afterwards). `--drop-data` and `--keep-data` are mutually exclusive.
- A debt whose worktree was re-created since is withdrawn, never paid.
- The JSON `namespaces` array reports each one as `dropped`, `deferred` or `kept` (see `json.md`).

## `extract`

`wtm extract <source> --files <a,b> --to <branch> --yes --output json` moves part of the `<source>` worktree's uncommitted changes onto another branch (to split an oversized PR).

- The source argument, `--files` and `--to` are all **required** under `--yes`/JSON: omitting one errors naming it; there is no picker.
- `--files` takes paths exactly as reported (spaces and non-ASCII are never quoted or escaped), including each untracked file of a brand-new directory individually. Pass a directory (`--files newmod/`) to take everything below it. Gitignored files are never listed: `.env` drift is `wtm env`'s job. A file entry may report `"status": "renamed"` with an extra `"orig_path"`: both paths move together, and `--files` names the new one.
- **On conflict it changes nothing and exits `15`.** `--yes` defaults on-conflict to abort; retry with `--on-conflict resolve` to apply git conflict markers, or choose a different `--to`. This covers both a file modified on both sides and one that merely already exists in the target. Exception: an untracked **binary** file already in the target cannot take conflict markers, so `resolve` will not help; pick a different `--to`.
- When `--to` names a branch that already exists locally, the rule of [An existing local branch](#an-existing-local-branch) applies: `--from` is **required** (the command errors naming it rather than guessing).
- When `--to` creates a worktree, its `.env` port values are settled exactly as `create` does (without asking under `--yes`), and `--isolation` and `--ff` (for the parent branch of the new target) work as on `create`. The JSON reports `isolation`, `env_ports` and `warnings` for the target; as on `create`, a `parent` env strategy that fell back to main is one of those `warnings`.

## `env`

`wtm env [worktree]` detects and fixes a worktree's `.env` drift. It reconciles the file against its committed **template** (the expected keys, read from the worktree itself) plus a single **value source** chosen by the worktree's recorded strategy, never a silent mix:

- `example`: template placeholders only.
- `main`: the main checkout.
- `parent`: the parent worktree **only**; a key the parent lacks stays `missing_unresolved` and is NOT pulled from main. The one exception (mirroring `create`): when there is no readable parent file at all (the parent has no worktree, or that file is not in it), it falls back to main for that file, flagged `parent_fallback: true` with `parent_branch` naming the parent.

The JSON `source` field names the source. `--from example|main|parent` is the only way to pull a different source for one run.

Modes and flags:

- `--mode add` (default) only fills missing keys and never touches an existing value; `--mode refresh` also settles values that diverge from the source. Linked port values are compared **modulo the offset** (see `run-config.md`).
- `--check` is a read-only drift report: it writes nothing, never prompts, and is the one mutating-command form that needs no `--yes` (`wtm env <wt> --check --output json` works alone). It counts a pending port shift as drift, and exits `18` when it finds any (the report or JSON is still written), `0` when the worktree is in sync — usable as a CI gate. The report (text or JSON) prints only the values wtm writes (ports, `COMPOSE_PROJECT_NAME`, `[[env]]`), with every password masked (a URL's, a `password=` pair); every other value is withheld (`"redacted": true`, see `json.md`). `--show-values` prints them all.
- Under `--yes`/JSON it is **report-only except safe additions**: keys missing from the child but resolved from a real source are added; **conflicts** stay unless `--on-conflict overwrite` (default `keep`); **orphans** stay unless `--prune`; keys with no real source value (`missing_unresolved`) are never auto-filled (they need the interactive prompt).
- Omit the worktree argument only interactively; under `--yes`, `--check`, JSON or without a terminal it errors (no picker). The positional takes the main checkout like any other worktree (`wtm env main`). A worktree that does not exist exits with `worktree not found: <name>`.
- An unknown `--mode`, `--from`, `--on-conflict`, `--isolation` or `--addressing` value exits `2` (`invalid --<flag> value "x": use …`). Refused before anything runs (exit `1`): `--prune` or `--on-conflict` with `--check` (it writes nothing), and `--on-conflict` under `--mode add` (which reports no conflict) — pass `--mode refresh`.
- Interactively, `--prune` and `--on-conflict overwrite` preselect the matching rows of the resolver, and `--isolation` / `--addressing` answer their step for whichever worktree is picked (a worktree refusing the flag is offered disabled: the main checkout for `--isolation verbatim`, every linked worktree for an `--addressing` other than the project's).
- Round-trip is preserved: comments, ordering and formatting are kept; only decided keys change, and a changed value keeps its line's quotes (an unquoted value with spaces stays unquoted), inline comment, `export` and CRLF ending.
- A fresh project (templates detected by `init`, no value files yet) has every expected key `missing_unresolved` and `source` reads `template (no .env to sync from)`; filling them interactively scaffolds the `.env` from the template.
- A configured file that exists **nowhere** (no value anywhere, no template either) is flagged `unresolvable: true` and never counts as clean: it names a path the repository does not have, and the fix is in `config.toml`, not in any worktree.
- It also runs the `[[env_port]]` pass (the `ports` block) and writes `COMPOSE_PROJECT_NAME` and `[[env]]` values on an isolated worktree. Those are wtm-owned keys, never reported as drift or conflict. A `run.toml` that cannot be used never stops the key reconciliation: only the port pass is skipped, and `warnings` names why (always on stderr, and in the JSON under `--output json`). A run that only shifted a port still reports what it wrote.
- **A run never changes how a worktree runs unless asked.** The interactive run asks it in one step, keeping the current state first: a linked worktree keeps or switches its isolation (a worktree created before the choice can also adopt it, or be recorded verbatim), the main checkout keeps or switches its addressing. `--yes`/JSON keep both unless `--isolation` / `--addressing` is given.

**Switching isolation: `--isolation isolated|verbatim`.** It settles the worktree's `.env` on that isolation and records it in `meta.json` **only once that succeeded**; a failed or cancelled run leaves the recorded isolation as it was.

- `isolated` on a verbatim worktree writes every port, identity and namespace value its creation left alone.
- `verbatim` puts the values wtm owns (linked ports, `[[env]]` values, `COMPOSE_PROJECT_NAME`) back to the source's (a key the source lacks is removed) and leaves every other key alone. `restored` lists them and `isolation_changed` says the record changed.
- Namespaces already created stay recorded, so `clean` still drops them.
- Refused with `--check` (a read-only run records nothing) and on the main checkout as `verbatim`.

**The main checkout's addressing: `--addressing ports|names`.** Under `addressing = "names"`, main's `.env` keeps whatever it spells (ports, as a checkout without wtm, until asked): `wtm env main --yes` reconciles its keys and leaves its addresses alone, and `--check` reads it against that same mode. `--addressing names` moves main onto the named URLs (it then depends on the run proxy: **ask the user first**), `--addressing ports` brings it back. Refused (exit `1`): `--addressing names` when `run.toml` addresses by ports, and on a linked worktree any mode other than the project's (linked worktrees follow `wtm run addressing`).
- **A worktree created before the isolation choice existed** (no `isolation` in its `meta.json`) has not adopted it: `wtm env <wt> --yes` reconciles its keys only (no port moved, no `COMPOSE_PROJECT_NAME` written, no ordinal allocated), with a `!` warning on stderr, and its JSON carries `"isolation_adoption": "not_adopted"` and no `isolation`. Adopt it with `wtm env <wt> --yes --isolation isolated` (`"isolation_adoption": "adopted"`): it then runs under a new compose project, so its current volumes are no longer used. **Ask the user first.**

## `exec`

`wtm exec <branch>... --yes --output json -- <command>` (or `--all` instead of names) runs a `/bin/sh -c` line in each worktree, in parallel (`--jobs N`, default CPU count), from the worktree root, with that worktree's run variables (the ones your shell carries about the current worktree are removed).

- stdin is closed: never pass an interactive command.
- Without names or `--all` it refuses under `--yes`; without `--` and a command it refuses too (exit `2`), since only the interactive wizard can ask for one. A name that matches no worktree exits `11` before anything runs; a name given twice runs once.
- Exit code: `0` when every command passed, `1` when any did not. Read each worktree's `status` and `exit_code` in the JSON to know which one failed and how; the process exit code never carries the child's.
- `--print` puts the full output in `output` instead of the 20-line `tail`.

## `relocate`

`wtm relocate --yes --output json` realigns worktrees with `base_path` and adopts externally-created ones. `--to <path>` sets a new `base_path` non-interactively (the interactive wizard, which you cannot drive, also offers it); the config is rewritten even when no worktree has to move.

- A worktree whose jobs are running is never moved (`blocked_jobs` in the JSON, exit non-zero, `--force` does not lift it): run `wtm run down <branch>`, then retry.
- An external worktree whose derived name another worktree already uses is left unadopted (`blocked_name`, exit non-zero).

## `checkout` (GitHub PRs)

`wtm checkout <number> --yes --output json` fetches a PR's branch into a worktree. The PR number and `--yes` are both **required** in JSON mode (no picker). `--yes` resolves the parent to the PR's base branch (a fact, not a guess) and the env strategy to the config default.

- A local branch of the PR's name is **reused as is**, never reset: the response sets `existing_branch: true` and `origin_state`. Under `--yes` no ref is touched even when the branch is behind, unless you pass `--ff`: it fast-forwards a behind-only branch to origin first, as on `create`, and leaves a diverged one as is. Without it, read `origin_state` and decide yourself.
- `--isolation` works as on `create`, and the JSON carries `isolation`, `env_ports` and `warnings` the same way. A name another worktree already reduces to is refused (exit `10`).
- Fork PRs are out of scope: fall back to `gh pr checkout <number>`. Creating a PR is out of scope too: use `gh pr create`.
- `gh` must be authenticated (`gh: …` on stderr means `gh auth login`).

## Navigate: `resolve`, `go`, `ui`

- `wtm resolve <branch> --output json` returns `{path, branch}`: use it to get a path.
- `wtm go` needs the user's shell integration to `cd`, so you cannot drive it. You rarely need to move at all: every `run` command takes the worktree as its first argument.
- `wtm ui` is the full-screen dashboard. **Never invoke it**: it holds the terminal until a human quits. Everything it shows is available as JSON via `wtm list` / `wtm tree`, and everything it does via `wtm create` / `wtm clean`.

## Setup: `init`, `config`, `upgrade`, `agents`

- **`wtm init`** bootstraps a repository (exit `12` elsewhere means it was not run). Unattended: `wtm init --yes [--base-branch … --env-strategy … --install-command … --clean-command …]`, global config included, never a prompt (`--non-interactive` is gone). Reconfigure one section later with `wtm init --only env|hooks|worktrees --yes`. Without a terminal it behaves the same even without `--yes`; an undetectable base branch errors naming `--base-branch`. Check `wtm init --help` for the full flag set. Dev jobs are **not** part of `wtm init`: they are opt-in, through `wtm run init` (see `run-config.md`).
- **`wtm config show --output json`** prints the resolved project config. `wtm config edit` and the `wtm init` wizard are interactive: ask the user to run them, or write the change to the path `config show` prints if you have a file-edit tool and the user authorized it.
- **`wtm upgrade`** updates the CLI itself, never worktrees (that is `wtm sync`). **Do not run it unless the user asked**: it replaces the binary you are driving. `--check` is the safe form: it reports availability and changes nothing. `--yes` skips the confirmation and is **required** with `--output json` unless `--check` is passed (without it the command exits `1` naming `--yes`). `--version <v>` pins a release and applies to a **standalone** install only; on a Homebrew or `go install` binary it errors. Exit `17` means the install cannot be upgraded (built from source: `git pull && make install`; binary needs sudo: the user re-runs it); network and checksum failures exit `1`.
- **`wtm agents install`** installs this `using-wtm` skill for Claude Code and Cursor.
