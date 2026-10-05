# Changelog

All notable changes to wtm are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and wtm adheres to [Semantic Versioning](https://semver.org); how to write an entry is in [docs/dev/changelog.md](docs/dev/changelog.md).

## [Unreleased]

### Fixed

- **`run.toml`**: a link to a `.env` that `config.toml` does not configure is ignored with a warning, and no longer stops the other `.env` values from being written. → [run.toml](docs/guide/run-toml.md#env)
- **`wtm create`** keeps the names you are typing when the branch fetch finishes, and a refreshed branch or worktree picker keeps its highlighted row.

## [0.29.0] - 2026-10-04

wtm opens up to other tools with a live event stream, and works on several worktrees at once.

### Highlights

- **`wtm events`** streams every worktree change as it happens, as JSON Lines for editors, terminal plugins and agents. → [Event stream](docs/guide/events.md)
- **`wtm create`** and **`wtm clean`** take several worktrees in one run: `wtm create feat/a feat/b`. → [Recipes](docs/guide/recipes.md#run-a-command-across-worktrees)
- **`wtm exec`** runs one command in several worktrees in parallel, each with its own ports: `wtm exec --all -- pnpm test`. → [Recipes](docs/guide/recipes.md#run-a-command-across-worktrees)

### Breaking

- **`wtm create --output json`** and **`wtm clean --output json`** answer with an envelope: read `.results[0]`. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)
- **Exit code `21`** for any command run outside a git repository (was `1`). → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)
- **Exit code `19`** for an interactive cancellation (was `0`), so `wtm create x && wtm go x` stops there. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)

### Added

- **`wtm events`** outside a repository follows every repository wtm has been used in. → [Every repository at once](docs/guide/events.md#every-repository-at-once)
- **`worktree.provisioned`** and **`worktree.deprovisioned`** events tell when a worktree's hooks have run, and whether they passed.
- **`WTM_CORRELATION_ID`** tags the events a command publishes, so a tool recognises its own. → [Integrations](docs/guide/integrations.md)
- **`wtm version --output json`** reports the version of each machine contract, for integrations to check compatibility.
- **Locked worktrees** (`git worktree lock`) are refused by `clean` and `prune` unless `--force`, and marked in `list` and `tree`.
- **`wtm ui`** creates and deletes several worktrees at once, and picks up changes made elsewhere immediately.

### Changed

- **`wtm env --check`** exits `18` on drift, ready for CI.
- **`wtm checkout`** and **`wtm extract`** ask the same questions as `create` and always show a recap before acting.
- **`wtm env`** reports a count per file and only the keys left to handle; warnings go to stderr.
- **Invalid flag values** and branch names git would reject are refused upfront with exit `2`, before anything is created.

### Fixed

- **`wtm relocate --to`** rewrites `base_path` even when no worktree has to move.
- **`wtm create`** and **`wtm checkout`** reject an unknown `--env-from` before creating a half-provisioned worktree.
- **A branch can no longer be its own parent** (`create b --from b`).
- **`wtm extract`** no longer ignores `--from` and `--ff` when the target is picked in the wizard.

## [0.28.0] - 2026-09-30

Each worktree runs its own services on its own ports; read the [migration guide](docs/guide/migrating-to-0.28.md) if you used `wtm run`, `wtm switch` or script wtm.

### Highlights

- **The `run` module**: per-worktree services and tasks grouped in profiles; `wtm run init` writes the config, `wtm run up` starts the stack. → [Jobs and profiles](docs/guide/jobs-and-profiles.md)
- **Per-worktree isolation**: shifted ports (`3000` → `3010`) rewritten in `.env`, and its own `COMPOSE_PROJECT_NAME`; `wtm env` settles them. → [Isolation](docs/guide/isolation.md)
- **Named URLs** per job and worktree (`http://web.feat-login.acme.localhost:11080`), on port 80 on macOS with `wtm run proxy install`. → [Addressing](docs/guide/addressing.md)

### Breaking

- **`wtm switch`** is removed: use `wtm go` then `wtm run up`. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md)
- **`--non-interactive`** is removed: use `--yes`. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md)
- **Hooks** run through `/bin/sh -c` with placeholders already quoted: drop your own quotes around `{{worktree}}`. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#hooks)
- **`run` commands** take the worktree as argument and the job or profile as a flag, and their JSON changes shape: update scripts. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#commands)
- **`run down --all`** stays in the current repository, **`run import`** replaces instead of merging, and `run.toml` is validated more strictly. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#commands)

### Added

- **Shared services**: one postgres for the repository, one database per worktree, dropped on `clean`. → [Shared services](docs/guide/shared-services.md)
- **`run up`** stops before a migration touches data the worktree does not own.
- **`wtm run up feat-a feat-b`** runs several worktrees at once, and **`wtm ui`** shows and drives the services.

### Changed

- **Output** is more consistent: a `┃` bar marks wtm's blocks, hooks sum up in one line, and `--quiet` works everywhere.

### Fixed

- **The `wtm` shell function** returns the command's exit code (open a new shell after upgrading).
- **`wtm init`** no longer installs every workspace package separately.
- **Rewriting a `.env` value** keeps its quotes, comment and line endings.

## [0.27.1] - 2026-09-09

A child worktree no longer pushes to its parent's branch.

### Fixed

- **`wtm create`** from a parent only on `origin` no longer makes it the child's upstream, so `git push` targets the child's branch.
- **`wtm prune --gone`** no longer offers to remove a never-pushed child whose parent's remote branch was deleted.
- **Not retroactive**: in an older child whose upstream names another branch, run `git branch --unset-upstream`, then `git push -u origin HEAD`.

## [0.27.0] - 2026-08-20

Self-update with `wtm upgrade`, and `wtm fast-forward`.

### Added

- **`wtm upgrade`** updates wtm the way it was installed, checksum verified; `--check` shows what is available, `--version` pins a release.
- **Update notice**: wtm mentions a newer version at the end of a command, without slowing it.
- **`wtm fast-forward`** moves a branch to `origin/<branch>` and refuses a diverged one (use `wtm sync`); also in the dashboard.

### Changed

- **`wtm ui` help overlay** reads like a reference: four sections, sized to the screen, scrollable.

### Fixed

- **`wtm sync`** interactive shows its recap instead of an empty picker.

## [0.26.1] - 2026-08-20

The `wtm ui` detail panel no longer flickers.

### Fixed

- **`wtm ui`** reloads the detail panel when the selection or its branch changes, and on `r`, instead of every three seconds.

## [0.26.0] - 2026-08-20

A full-screen dashboard, `wtm ui`, to drive worktrees.

### Highlights

- **`wtm ui`**: a full-screen dashboard running `create`, `clean`, `reparent`, `prune` and `sync`, with keyboard, mouse and help on `?`.

### Added

- **`wtm ui` Tree tab** shows the stacked branches and reparents from the dashboard.
- **`wtm ui` detail panel** shows the last commit, working tree state, children, `.env` drift, removal blockers and the PR with its CI checks.
- **`wtm ui`** tells a failing `gh` apart from "no PR", and opens the PR in the browser with `p`.
- **`wtm list`** and **`wtm resolve`** mark the current worktree `● active`.
- **`ui.animations = false`** in the global config turns off dashboard animations.

### Changed

- **New colour palette** for every command's output, still respecting `NO_COLOR`.

### Fixed

- **`wtm sync`** keeps the parent step visible and refreshes parents the cascade does not cover.
- **README** no longer documents an `agent` key that made every command fail.

## [0.25.0] - 2026-08-17

`create`, `checkout` and `extract` reuse an existing local branch.

### Breaking

- **`wtm create <existing-branch> --yes`** and **`wtm extract --to <existing-branch> --yes`** require `--from <parent>`: wtm cannot guess the parent.

### Added

- **`wtm checkout <PR>`** reuses an existing local branch instead of asking for `wtm clean`, and offers to fast-forward it.
- **`wtm create <existing-branch>`** and **`wtm extract --to <existing-branch>`** reuse the branch, announced in the recap.
- **JSON** of `create` and `checkout` reports whether the branch existed and how it stands against origin.

### Changed

- **A branch checked out in another worktree** exits `10` with a `wtm go <branch>` hint; `--if-not-exists` returns that worktree.

## [0.24.1] - 2026-08-11

`wtm extract` handles untracked files properly.

### Added

- **`wtm extract --files`** accepts a directory and takes every change below it.

### Changed

- **`wtm extract`** removes directories left empty in the source.
- **`wtm extract --on-conflict resolve`** also covers untracked files already in the target; identical content is no longer a conflict.

### Fixed

- **`wtm extract`** lists each file of a new directory separately, so `--files newmod/x.go` works.
- **`wtm extract`** handles paths with spaces or non-ASCII characters, and staged renames.

## [0.24.0] - 2026-07-08

`wtm env` detects and resolves `.env` drift between worktrees.

### Added

- **`wtm env [branch]`** detects and resolves a worktree's `.env` conflicts, following the strategy chosen at creation.

## [0.23.0] - 2026-07-07

`on_clean` hooks, generalized `.env` detection and harmonized output.

### Breaking

- **`.env` config** moves from `copy_files` to `[[env.file]]` entries, with no automatic migration: run `wtm init --only env` or edit `config.toml`.

### Added

- **`[hooks] on_clean`** runs before `clean`/`prune` remove a worktree (e.g. `docker compose down`); a failure aborts unless `continue_on_error`.
- **`wtm init --clean-command`** and **`--skip-clean`** configure `on_clean` non-interactively.
- **`sudo rm -rf` fallback**: an interactive run offers it when `git worktree remove` hits root-owned files, never on a dangerous path.
- **`.env` detection** recognises `.env.dist`, `.env.sample` and other templates, and flags `.env.local` as local.

### Changed

- **The `example` strategy** copies the detected template instead of hard-coding `.env.example`.
- **Output** conventions are harmonized across commands; `wtm sync` shows a spinner while pushing.

## [0.22.0] - 2026-07-03

An opt-in `run` module and one `--yes`/`--force` model across commands.

### Highlights

- **`wtm run init`** sets up `run.toml` from detected docker-compose files and package scripts, without overwriting existing jobs.
- **`--yes` and `--force`** are separate: `--yes` answers every question with a flag or a safe default, `--force` only lifts safety refusals.

### Breaking

- **`--output json`** requires `--yes` on every mutating command, and a missing required selection errors naming its flag: pass both.
- **The `run` module** is opt-in: `wtm init` no longer configures services (`--skip-services`, `--only services` removed); use `wtm run init`.

### Added

- **`origin` divergence badges** (`origin ↑a ↓b`) in `list`, `tree`, pickers and JSON, read without fetching; `r` refreshes.
- **`--ff`** fast-forwards a source that is only behind, also on `create --from` and `extract`.
- **`wtm extract [source]`** picks the source first, so you can extract from anywhere.
- **`wtm reparent`** moves several worktrees onto one new parent in a single pass.

### Changed

- **Wizards** keep every confirmation inside, with a breadcrumb and Back; `clean`, `relocate` and `sync` each run as one wizard.
- **`wtm agents install`** updates an already installed skill.
- **`wtm init`** points out pre-existing worktrees and suggests `wtm relocate`.

### Fixed

- **`wtm prune`** reads merged/closed from the GitHub PR state instead of local commits; a branch without a PR is never tagged.
- **`wtm clean`** and **`wtm relocate`** without a terminal or `--yes` error out instead of starting a wizard.
- **`wtm clean --reparent-children`** is honoured in the wizard.

## [0.21.0] - 2026-07-01

`wtm prune` cleans up finished work in one pass, and every command speaks JSON.

### Added

- **`wtm prune`** removes every worktree whose work is done, reparenting surviving children onto their grandparent.
- **`--merged`**, **`--closed`** and **`--gone`** narrow `prune` to branches with no commit ahead, a merged or closed PR, or a deleted remote.
- **`wtm prune`** needs `--force` for a dirty, unpushed or open-PR worktree, and never touches main or the base branch.
- **`wtm prune --dry-run`** previews without changing anything.
- **`wtm sync --keep-conflict`** leaves a conflicting rebase in progress for manual resolution instead of aborting it.
- **`wtm sync`** detects a rebase already in progress and blocks its descendants.

### Changed

- **`--output json`** is available on every command, with a stable payload.
- **`wtm --help`** groups commands into sections.
- **Command reference** under `docs/` is generated from the CLI.

### Fixed

- **`wtm sync`** captures conflicting files before aborting, and finds the branch of a worktree stuck mid-rebase.

## [0.20.0] - 2026-07-01

Branch pickers show remote branches and divergence, and multi-select lists can be filtered.

### Added

- **`wtm create --from origin/x`** creates a worktree from a remote branch you never checked out.
- **`wtm reparent --to origin/x`** reparents onto a remote branch.
- **Branch pickers** list `origin` branches too, and tag drifted local ones with `↑2 ↓5`; `r` fetches again.
- **`wtm create`** offers to fast-forward a source branch behind `origin/`, and warns when it has diverged.
- **Multi-select lists** filter on `/`; `a` toggles all filtered items.

### Changed

- **Worktree lists** are redesigned: aligned badges, a tinted selected row, a status glyph.
- **`create`**, **`extract`** and **`checkout`** ask before the `parent` strategy copies `.env` from the main worktree.

## [0.19.0] - 2026-06-27

A stacked-branch workflow: see the tree, reparent a branch, and sync only what you pick.

### Breaking

- **`wtm sync`** no longer syncs everything by default: pass branch names, pick them interactively, or use `--all`.

### Added

- **`wtm tree`** shows the forest of worktrees, flagging a child that needs a rebase with `⚠ needs sync`.
- **`wtm tree --with-prs`** adds PR state; **`--output mermaid`** prints a flowchart to paste into a PR.
- **`wtm reparent <branch> --to <parent>`** changes a worktree's parent; the rebase happens on the next `wtm sync`.
- **`wtm clean`** offers to reparent the children it would orphan onto the grandparent, or `--reparent-children`.
- **`wtm sync`** without arguments opens a multi-select picker.

### Changed

- **`wtm sync`** always refreshes the base first, and exits `11` on an unknown branch.
- **Output** has the same spacing and loader across commands; no spinner or `\r` reaches a pipe.

### Fixed

- **A fast task's** first output is no longer lost.

## [0.18.0] - 2026-06-24

`wtm checkout` replaces the `pr` group.

### Breaking

- **`wtm pr checkout`** is now **`wtm checkout`**: update scripts and aliases.
- **`wtm pr list`** is removed: use `wtm list --with-prs` or the `checkout` wizard.

### Added

- **`wtm checkout [number]`** creates a worktree from a pull request; without a number, a wizard lists open PRs.
- **`--review`**, **`--mine`**, **`--from`** and **`--env-from`** filter PRs and preset the wizard's answers.

## [0.17.0] - 2026-06-24

Worktree commands move to the top level.

### Breaking

- **`wtm wt`** is removed: `wtm wt list` becomes `wtm list`, and so on; update scripts and re-run `eval "$(wtm shell-init)"`.

### Fixed

- **A detached job's output** is no longer truncated when its process exits.

## [0.16.0] - 2026-06-22

`wt relocate` gathers worktrees under `base_path`, and unused configuration is removed.

### Breaking

- **`wtm pr create`** is removed: use `gh pr create`.
- **`[agents]`, `[integrations]`, `[github]`** and the global `agent` key are refused: delete them or re-run `wtm init`.

### Added

- **`wtm wt relocate`** moves scattered worktrees under `base_path` and adopts external ones, with a preview; `--to` and `--output json` for scripts.

### Removed

- **The default-agent setting**: the `agent` key, the `--agent` flag and its `init` step.
- **`[github] auto_draft`**, unused since `pr create` was removed.

## [0.15.0] - 2026-06-20

`wt sync` rebases the whole chain of worktrees in one command.

### Added

- **`wtm wt sync`** rebases every worktree onto its refreshed parent in topological order, replaying only its own commits, locally.
- **`wtm wt sync`** shows a recap, then offers one `--force-with-lease` push of the rebased branches.
- **`--dry-run`**, **`--base`**, **`--push`**, **`--no-push`** and **`--yes`** on `wt sync`.
- **`wtm wt sync --output json`** reports a status per branch and exits non-zero on a conflict or error.

### Fixed

- **`wtm wt sync`** reports a failing git command as an error instead of `up_to_date`.

## [0.14.0] - 2026-06-20

Worktree lists show up instantly while PRs stream in.

### Added

- **`wtm wt list --with-prs`** includes PRs in non-interactive and JSON output.

### Changed

- **`wt list`**, **`wt go`** and **`wt switch`** show worktrees immediately, PR badges filling in as they arrive.
- **`wt list`** no longer fetches PRs by default in non-interactive output.

## [0.13.0] - 2026-06-20

`wtm init` reworked: skip sections, re-initialise one with `--only`, edit `on_create` hooks.

### Added

- **`wtm init`** lets you skip `env`, `hooks` or `services`, written commented out; `--skip-env`, `--skip-hooks`, `--skip-services` do it non-interactively.
- **`wtm init --only <section>`** re-initialises one section without touching the others.
- **`wtm init`** edits `on_create` hooks as a list: add, edit, remove, reorder.

### Removed

- **The install command and monorepo packages steps** of `wtm init`: use the `on_create` hook editor.

## [0.12.0] - 2026-06-20

`wt extract` moves uncommitted changes between worktrees.

### Added

- **`wtm wt extract`** moves part of the current worktree's uncommitted changes to a new or existing worktree; `--keep` copies them instead.
- **`wtm wt extract`** leaves the source untouched unless the whole extraction applied.
- **`--on-conflict abort`** (default) changes nothing and exits `15`; **`resolve`** writes conflict markers in the target.

## [0.11.0] - 2026-06-19

wtm can be driven by agents, and detached services stream their startup logs.

### Breaking

- **Outside an initialised repository**, commands exit `12` instead of `0`: adjust scripts relying on a silent success.
- **`wtm pr create`** exits `13` when a PR already exists, instead of `0`.

### Added

- **`wtm init --non-interactive`** bootstraps a project from flags, then detection, then defaults.
- **`wtm pr create --yes`** pushes an unpushed branch and skips prompts.
- **`wtm wt create --if-not-exists`** succeeds when the worktree already exists.
- **Exit codes per failure**: `10` worktree exists, `11` branch not found, `12` config not found, `13` PR exists, `14` job not declared.
- **`wtm run up`** streams a detached service's startup output instead of a spinner.

### Changed

- **`wt clean`**, **`run stop`** and **`run down`** succeed as no-ops when there is nothing to remove or stop.

### Fixed

- **`wtm pr create --output json`** no longer stops silently on an unpushed branch.

## [0.10.0] - 2026-06-09

`run up` and `run start` launch and tail in one step, and profiles run jobs in your order.

### Added

- **`wtm run up`** / **`run start`** start jobs and stream their output straight away.
- **`run profile add`** / **`edit`** order a profile's jobs, followed at run time.

### Changed

- **A failed task** aborts the rest of the profile and shows its logs.
- **The `wt go` / `wt switch` picker** loads faster and shows a callout when `gh` is missing.

## [0.9.0] - 2026-06-08

An interactive `wt list`, `run.toml` export/import and commands to edit jobs and profiles.

### Breaking

- **Config and metadata** move from `.wtm/` to `<git-common-dir>/wtm/`: move existing files or re-run `wtm init`.
- **`wtm run list --output json`** uses lowercase keys (`job`, `name`, `kind`).
- **The shell wrapper** changed: re-run `eval "$(wtm shell-init)"`.

### Added

- **`wtm wt list`** is interactive, with an **Open PR** action and a hint when `gh` is missing.
- **Removing the current worktree** sends the shell back to the base repository.
- **`wtm init`** offers `package.json` scripts, workspaces included, as jobs.
- **`wtm run export`** / **`run import`** move `run.toml` in and out as JSON.
- **`wtm run job`** and **`wtm run profile`** `add|rm|edit|list` manage `run.toml` by wizard or flags.
- **`wtm config show`** and **`wtm config edit`** reach the config without digging into the git directory.

### Changed

- **Setting a default profile** unsets the previous one instead of failing.

## [0.8.0] - 2026-05-01

Config files are decoded strictly and come with JSON Schemas for IDE autocomplete.

### Added

- **JSON Schemas** for `run.toml` and both `config.toml` are written next to them, for autocomplete in Taplo-based editors.
- **`wtm schema dump`** refreshes the schemas on disk after an upgrade.

### Fixed

- **Unknown keys** in a config file are rejected instead of ignored.

## [0.7.2] - 2026-04-29

The terminal is restored after detaching from a job's logs.

### Fixed

- **`wtm run logs`** on a job with a TUI (turbo, vite, vim) no longer leaves the terminal broken on exit.

## [0.7.1] - 2026-04-29

Stopping a job stops its whole process tree.

### Fixed

- **`wtm run stop`** / **`run down`** stop the job's whole process group, with SIGKILL after 5 s if SIGTERM is ignored.

## [0.7.0] - 2026-04-29

Services and one-shot tasks are unified as jobs in `run.toml`.

### Breaking

- **`.wtm/services.toml`** is replaced by **`.wtm/run.toml`**, with `[[job]]` and `[[profile]]`: rewrite your file.
- **`wtm svc`** is renamed **`wtm run`**: update scripts and aliases.
- **The `wt switch` shell wrapper** calls `wtm run up`: regenerate it with `wtm shell-init`.

### Added

- **`kind = "task"`** declares a one-shot command (migration, seed) that streams live and must succeed before the profile continues.
- **`run.toml`** is validated before anything runs.

### Changed

- **`wtm init`** writes detected docker-compose files as detached services.

## [0.6.2] - 2026-04-13

More output polish.

### Fixed

- **The `wt go` / `wt switch` picker** keeps its colours through the shell wrapper.

## [0.6.1] - 2026-04-12

Output polish.

### Changed

- **`svc up`**, **`svc ps`**, **`pr create`** and **`wtm init`**: consistent padding and spacing.

### Fixed

- **The `wt go` / `wt switch` picker** keeps its highlight and badges through the shell wrapper.

## [0.6.0] - 2026-04-12

wtm can be driven by LLM agents.

### Added

- **`--output json`** on the `wt`, `pr` and `svc` commands, with human text on stderr.
- **`wtm svc list`** lists declared services and profiles, with actions on a terminal.
- **`wtm svc ps`** lists running services, with stop, logs and restart actions.
- **`wtm agents install`** installs a `using-wtm` skill into the `.claude/` or `.cursor/` directories it finds.
- **`wtm init`** detects docker-compose files and scaffolds matching services.
- **`wtm svc down --all`** stops every service of every worktree.

### Changed

- **`wt switch`** without an argument shows the `wt list` picker.
- **`wtm svc down`** without `--all` only touches the current worktree.

### Fixed

- **A service with a `stop` command** no longer reports `✓ started` when `docker compose up -d` fails.
- **`svc down`**, `wt clean` and `svc up --exclusive` no longer stop other worktrees' services.

## [0.5.1] - 2026-04-12

Pickers and shell navigation work through the shell wrapper.

### Fixed

- **The `wt go` / `wt switch` picker** is visible through the shell wrapper.
- **"Go to worktree"** from `pr list` and `wt list` navigates instead of asking for shell integration.
- **The shell wrapper** lets any subcommand change the directory.

## [0.5.0] - 2026-04-12

New pickers and wizards, `wt switch`, and focus removed in favour of services.

### Breaking

- **`wtm wt focus`** and active-worktree tracking are removed: use `svc up` / `svc down`.
- **`on_focus`** / **`on_blur`** hooks are removed: keep `on_create`, and let services run Docker.
- **The dashboard** is hidden while it is reworked: `wtm` alone shows help.

### Added

- **`wtm wt switch [branch]`** goes to a worktree and runs `svc up`.
- **`svc up`** offers to stop services running in other worktrees; `--exclusive` stops them, `--parallel` skips the question.
- **`wt clean`** stops a worktree's services before deleting it.
- **Pickers and wizards** filter on `/`, show a breadcrumb and go back with `Esc`.
- **The `pr list` picker** offers to go to or check out the PR's worktree; **`wt list`** shows badges.

### Changed

- **Output**: one style and indent for every message and error.

### Fixed

- **`docker compose up -d` services** are tracked and stopped properly.
- **`svc` commands** read `services.toml` from the main worktree when run from another.

### Removed

- **The docker-compose and hook steps** of the `wtm init` wizard.

## [0.4.1] - 2026-04-12

GitHub access goes through the `gh` CLI.

### Breaking

- **`wtm auth login|status|logout`** are removed: install `gh` and run `gh auth login`.
- **`WTM_GITHUB_TOKEN`** is no longer read: use `GH_TOKEN`.

### Changed

- **`pr` commands** and the dashboard's PR panel use `gh`.

## [0.4.0] - 2026-04-10

GitHub integration and pull-request commands.

### Breaking

- **Worktree commands** move under `wtm wt` and service commands under `wtm svc`: update scripts and aliases.

### Added

- **`wtm auth login`** signs in to GitHub with the device flow, with `auth status` and `auth logout`.
- **`wtm pr list`** lists pull requests, with `--mine` and `--review`, also in the dashboard.
- **`wtm pr create`** creates a PR from the current branch through a wizard.
- **`wtm pr checkout`** creates a worktree from an existing PR.
- **`wtm svc start`** / **`stop`** act on single services, **`up`** / **`down`** on profiles.

### Changed

- **The dashboard** splits worktrees and PRs, and multiplexes logs.

### Fixed

- **Dashboard** focus handling.

## [0.3.0] - 2026-04-04

Services run in a background daemon, each in its own terminal.

### Breaking

- **Project config** moves from `.wtm.toml` to `.wtm/config.toml`: move the file.

### Added

- **`wtm up`** starts a profile's services from `.wtm/services.toml` in a background daemon, scoped to the worktree.
- **`wtm down`** stops them; **`wtm logs`** attaches to a service's terminal.
- **The dashboard** starts, stops and attaches to the selected worktree's services.

## [0.2.1] - 2026-04-03

A fix for the dashboard launched from the shell.

### Fixed

- **The dashboard** opens through the shell wrapper.

## [0.2.0] - 2026-04-03

An interactive dashboard.

### Added

- **`wtm`** without arguments opens a full-screen dashboard of every worktree.
- **The dashboard** shows each worktree's status and details, and creates, cleans, focuses and navigates to worktrees.
- **Focusing** from the dashboard streams hook output live.
- **`wtm new`** asks for the branch name when none is given.

### Fixed

- **Hook errors** in the dashboard no longer corrupt the screen.

## [0.1.2] - 2026-04-02

Fixes for commands run from a child worktree.

### Fixed

- **Commands run from a child worktree** find the project config.
- **Blur hooks** no longer fail when the previous worktree's directory is gone.
- **The shell wrapper** returns to the main worktree after cleaning the current one.

## [0.1.1] - 2026-04-02

Initial release.

### Added

- **`wtm init`** sets up global and project configuration (`.wtm.toml`) through a wizard.
- **`wtm new [branch]`** creates a worktree with env provisioning, metadata and hooks.
- **`wtm ls`** lists worktrees with their git status.
- **`wtm go [branch]`** moves to a worktree through shell integration.
- **`wtm focus [branch]`** switches the active worktree and runs `on_blur` / `on_focus` hooks.
- **`wtm clean [branch]`** removes a worktree, refusing when it is dirty, unpushed or has an open PR.
- **`wtm shell-init`** generates the shell wrapper for zsh, bash and fish.
- **Env strategies** `example`, `main` and `parent`, and hooks with template variables.
- **Detection** of the base branch, env files, package manager, Docker Compose and pnpm workspaces.
- **Install** with Homebrew (`brew install LucasPcq/tap/wtm`), release binaries or `go install`.

[Unreleased]: https://github.com/LucasPcq/wtm/compare/v0.29.0...HEAD
[0.29.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.29.0
[0.28.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.28.0
[0.27.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.27.1
[0.27.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.27.0
[0.26.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.26.1
[0.26.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.26.0
[0.25.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.25.0
[0.24.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.24.1
[0.24.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.24.0
[0.23.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.23.0
[0.22.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.22.0
[0.21.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.21.0
[0.20.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.20.0
[0.19.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.19.0
[0.18.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.18.0
[0.17.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.17.0
[0.16.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.16.0
[0.15.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.15.0
[0.14.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.14.0
[0.13.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.13.0
[0.12.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.12.0
[0.11.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.11.0
[0.10.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.10.0
[0.9.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.9.0
[0.8.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.8.0
[0.7.2]: https://github.com/LucasPcq/wtm/releases/tag/v0.7.2
[0.7.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.7.1
[0.7.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.7.0
[0.6.2]: https://github.com/LucasPcq/wtm/releases/tag/v0.6.2
[0.6.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.6.1
[0.6.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.6.0
[0.5.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.5.1
[0.5.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.5.0
[0.4.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.4.1
[0.4.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.4.0
[0.3.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.3.0
[0.2.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.2.1
[0.2.0]: https://github.com/LucasPcq/wtm/releases/tag/v0.2.0
[0.1.2]: https://github.com/LucasPcq/wtm/releases/tag/v0.1.2
[0.1.1]: https://github.com/LucasPcq/wtm/releases/tag/v0.1.1
