# Changelog

All notable changes to wtm are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and wtm adheres to [Semantic Versioning](https://semver.org); how to write an entry is in [docs/dev/changelog.md](docs/dev/changelog.md).

## [Unreleased]

## [0.29.0] - 2026-10-04

A live event stream for integrations, batch `create` and `clean`, `wtm exec`, and `checkout`, `extract` and `env` aligned on the conventions of `create`.

### Highlights

- **`wtm events`** streams worktree changes live: a snapshot of every worktree, then one event per creation, move, reparent, isolation change or removal, wherever it comes from (another shell, an agent, `wtm ui`); `--output json` gives JSON Lines for editors and terminal plugins, and the stream reconnects on its own when the daemon restarts. → [Event stream](docs/guide/events.md)
- **`wtm create`** and **`wtm clean`** take several worktrees in one run (`wtm create feat/a feat/b fix/c`, `wtm clean feat/a feat/b`, or tab in the wizard / multi-select in the picker): shared questions are asked once and one failure does not stop the others. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)
- **`wtm exec`** runs one command in several worktrees in parallel, each with its own environment (shifted ports, compose project): `wtm exec --all -- pnpm test`, with a per-worktree summary, the output of failures and `--output json`; without arguments it opens a wizard. → [Configuration](docs/guide/configuration.md#the-environment-of-wtm-exec)

### Breaking

- **`wtm create --output json`** always answers with an envelope `{"results": [...], "failed": [...]}`: read `.results[0].path` instead of `.path`. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)
- **`wtm clean --output json`** always answers with an envelope `{"results": [...], "failed": [...], ...}`: read `.results[0].already_absent` instead of `.already_absent`. → [Migrating to 0.29](docs/guide/migrating-to-0.29.md)
- **Exit code `21`** is returned by any command run outside a git repository (was `1`) and by `wtm events --repo` pointing at a non-git directory (was `2`): check for `21` in scripts that tested those codes. → [When it exits](docs/guide/events.md#when-it-exits)
- **Exit code `19`** is returned by every interactive cancellation (Esc, Ctrl-C, "No, cancel", a declined confirmation), so `wtm create x && wtm go x` stops there: treat `19` as "cancelled" in scripts that expected `0`.

### Added

- **`wtm events`** outside a repository follows every repository wtm has been used in: one snapshot per repository, a single `ready`, then `repo.added` and `repo.removed`; repositories are recorded in `repos.json` next to the global config by `wtm init` and the first command run in a repository. → [Every repository at once](docs/guide/events.md#every-repository-at-once)
- **`worktree.provisioned`** event in `wtm events`, sent once the `on_create` hooks have run (even with no hook), with their result: `ok`, and on failure the hook and its exit code. → [Event stream](docs/guide/events.md#what-the-stream-carries)
- **`worktree.deprovisioned`** event in `wtm events`, sent after the `on_clean` hooks of `clean` and `prune` with their result; `ok: false` means a hook interrupted the removal, the worktree is still there and no `removed` follows. → [Event stream](docs/guide/events.md#what-the-stream-carries)
- **`WTM_CORRELATION_ID`**: a command run with it copies the id into the `correlation_id` of every event it publishes, children included, so a host recognises its own events. → [Recognising your own command](docs/guide/events.md#recognising-your-own-command)
- **`wtm version --output json`** reports the wtm version and the version of each contract it exposes (`events`), so an integration can check compatibility; `wtm --version` is unchanged. → [Checking compatibility](docs/guide/events.md#checking-compatibility)
- **`wtm events`** has stable exit codes: `12` (repository not initialized), `20` (newer schema), `21` (not a git repository) and `2` (usage) are final, anything else is worth retrying; the message goes to stderr, never stdout. → [When it exits](docs/guide/events.md#when-it-exits)
- **`wtm ui`** creates several worktrees in one run, each appearing in the list as soon as it exists, and deletes several from its global menu ("Delete worktrees").
- **`wtm clean`** and **`wtm prune`** refuse a locked worktree (`git worktree lock`) like a dirty one, with their own message instead of git's raw error; `--force` lifts the lock.
- **`wtm list`** and **`wtm tree`** mark locked worktrees `! locked`, and their JSON carries `is_locked`; the `wtm ui` delete modal lists the lock as a separate blocker to lift.
- **Ctrl-C** cancels a wizard at any step, like Esc on the first one, and a standalone confirmation too (the one for `extract` conflicts).

### Changed

- **`wtm clean --yes`** with several worktrees refuses the whole batch when a single one is unsafe, unless `--force`.
- **`wtm clean`** and **`wtm prune`** move the children of a removed chain to the closest remaining ancestor.
- **`wtm ui`** picks up worktrees created, moved or removed elsewhere immediately; the 20 s refresh only updates git state (modified, ahead, behind).
- **`wtm checkout`** always shows the recap before creating the worktree, even with every flag given, and carries the `parent` → main fallback warning there instead of a separate question; a cancellation says so.
- **`wtm checkout`** offers to update a local branch behind origin before the recap, and `--ff` does it under `--yes`.
- **`wtm extract`** always shows the recap, even with every flag given.
- **`wtm extract --to`** with a new target asks the same questions as `create` (parent, isolation, update from origin) instead of a separate question after the wizard.
- **`wtm extract`** proposes the chosen source's own parent rather than the base branch, and refuses a branch held by another worktree as soon as it is typed.
- **`wtm extract`** ends like `create`: aligned `source` and `path` fields, path relative to `base_path`, colour on the glyph only, a `→` next step after a conflict, and "nothing to extract" shown as `=` naming the source.
- **`wtm env`** reports what it did: the verdict first, a count per file ("2 added · 1 overwritten"), and only what is left to handle key by key (a kept conflict, a key without a value, an orphan); `--check` still lists every key.
- **`wtm env`** sends warnings to stderr, and its JSON gains `path` and a per-key `action`.
- **`wtm env`** applies every flag in the wizard: `--prune` and `--on-conflict overwrite` preselect the resolver's rows, `--isolation` applies to the worktree picked (main is greyed out for `verbatim`), and the recap names the mode, the value source and the isolation.
- **`wtm env`** picker badge counts the keys to add.
- **`wtm env --check`** exits `18` when it finds drift (the report is still written) and `0` when the worktree is up to date, ready for CI.
- **`wtm env`** uses one vocabulary throughout (add, fill, overwrite, keep, prune, skip), in the past tense in the report and JSON (`pruned` instead of "remove"/"removed").
- **Wizards** show the step title in the breadcrumb ("Step 3/4 • Resolve drift — feat/a"), so the worktree concerned is on screen at every question.
- **Wizards** wrap step descriptions and errors at the terminal width instead of cutting them.
- **Invalid flag values** exit `2` with one message, `invalid --<flag> value "x": use …`, for `--isolation` and `--env-from` everywhere and for the flags of `wtm env`.
- **Branch names git would reject** (`bad..name`, `a b`, `x.lock`…) are refused upfront with exit `2`, in the wizard and as arguments, instead of failing mid-creation.
- **`wtm create`** and **`wtm checkout`** under `--yes` report the `parent` strategy falling back to main's `.env` with a warning line, and a `warnings` entry in JSON.
- **`wtm extract --yes`** reports the `parent` → main fallback like `create`.
- **Fast-forward** declined after a failure prints `= Aborted.` like other cancellations.

### Fixed

- **`wtm relocate --to`** rewrites `base_path` even when no worktree has to move (under `--yes` it answered "already aligned" and left the config untouched), `--dry-run` announces it, and the emptied old `base_path` directory is removed.
- **`wtm relocate --output json`** returns `steps: []` instead of `null`, and always fills `base_path`.
- **`wtm checkout`** and **`wtm create`** reject an unknown `--env-from` before creating anything, instead of leaving a half-provisioned worktree.
- **`wtm checkout`** rejects a `--from` naming no branch, and a PR branch already held by another worktree, before asking any question.
- **`wtm env --check`** counts keys to add instead of reporting "No drift".
- **`wtm env`** no longer counts a key filled in the wizard or an overwritten conflict as still to resolve.
- **`wtm env`** rejects `--prune` or `--on-conflict` with `--check`, and `--on-conflict` with `--mode add`, instead of ignoring them.
- **`wtm extract`** no longer ignores `--from` and `--ff` when the target is picked in the wizard.
- **`wtm extract`** warns that `--from` and `--ff` are ignored when the target already exists, as it did for `--isolation`.
- **A branch can no longer be its own parent**: `create <b> --from <b>` and `extract --to <b> --from <b>` are refused, and the parent picker no longer offers the branch being created.
- **Recaps** of `create`, `checkout` and `extract` align the `Update:` line with the other fields.

## [0.28.0] - 2026-09-30

Each worktree can run its own services (dev servers, a `docker compose` stack) on its own ports and under its own name, next to the others; the `run` module stays opt-in, and nothing changes without a `run.toml`. Read the [migration guide](docs/guide/migrating-to-0.28.md) before upgrading if you used `wtm run` or `wtm switch` in 0.27, or script wtm.

### Highlights

- **The `run` module**: per-worktree services and tasks, grouped in profiles and run by a background daemon; `wtm run init` detects compose files and scripts and writes the config once, `wtm run up` starts the stack of the current worktree. → [Jobs and profiles](docs/guide/jobs-and-profiles.md)
- **Per-worktree isolation**: shifted ports (`3000` → `3010`), its own `COMPOSE_PROJECT_NAME`, and an *isolated* or *verbatim* choice at creation. → [Isolation](docs/guide/isolation.md)

### Breaking

- **`wtm switch`** is removed: use `wtm go` then `wtm run up`. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md)
- **`--non-interactive`** is removed: use `--yes`, which runs `init` and `run init` without any question. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md)
- **Hooks** run through `/bin/sh -c` and receive their placeholders already quoted: drop your own quotes around `{{worktree}}` and re-read hooks with shell characters. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#hooks)
- **`run` commands** take the worktree as positional argument and the job or profile as a flag (`run up [worktree...] --profile <p>`), and `run up` starts a single profile: update scripts accordingly. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#commands)
- **`run` JSON** changes shape (`branch` + `path`, one array per worktree), and exit codes `2` (usage) and `14` (unknown job or profile) apply everywhere: update consumers. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#the-json-contract-of-wtm-run)
- **`run down --all`** no longer leaves the current repository, **`run import`** replaces `run.toml` instead of merging, and **`run.toml`** is validated more strictly: see the guide for each replacement. → [Migrating to 0.28](docs/guide/migrating-to-0.28.md#commands)

### Added

- **Ports live in `.env`**: wtm rewrites the port inside the values that carry it (`DATABASE_URL`…) without touching the rest of the line. → [How `wtm run` works](docs/guide/how-run-works.md)
- **Named URLs** per job and worktree (`http://web.feat-login.acme.localhost:11080`) served by a local proxy, on port 80 on macOS with `wtm run proxy install`. → [Addressing](docs/guide/addressing.md)
- **Shared services**: one postgres for the whole repository, one database per worktree, dropped on `clean`. → [Shared services](docs/guide/shared-services.md)
- **`run up`** stops before a migration touches data the worktree does not own.
- **`wtm run up feat-a feat-b`** runs several worktrees at once, with a full-screen view per run (`-d` to give the terminal back), and **`wtm ui`** shows and drives the services.
- **`--quiet`** on every command.
- **`create`**, **`extract`** and **`checkout`** report isolation and ports in `--output json`.
- **A [user guide](docs/guide/README.md)** and a redesigned README.

### Changed

- **Output** is more readable and consistent: a `┃` bar marks wtm's blocks, and hooks show while they run then sum up in one line.
- **`wtm env`** also settles a worktree's ports and isolation.
- **Two worktrees** can no longer carry the same derived name (`feat.x` and `feat/x`).

### Fixed

- **The `wtm` shell function** returns the command's exit code (open a new shell after upgrading).
- **`wtm init`** no longer installs every workspace package separately.
- **Rewriting a `.env` value** keeps its quotes, comment and line endings.

## [0.27.1] - 2026-09-09

A child worktree no longer pushes to its parent's branch.

### Fixed

- **`wtm create`** from a parent that exists only on `origin` no longer sets `origin/<parent>` as the child's upstream, so `git push` no longer targets the parent's branch, whatever `branch.autoSetupMerge` says.
- **`wtm prune --gone`** no longer offers to remove a never-pushed child worktree because its parent's branch was deleted from the remote.
- **Not retroactive**: in a child worktree created earlier, if `git rev-parse --abbrev-ref @{upstream}` names another branch, run `git branch --unset-upstream`, then publish with `git push -u origin HEAD`.

## [0.27.0] - 2026-08-20

Self-update with `wtm upgrade`, `wtm fast-forward`, and a dashboard help overlay that reads like a reference.

### Added

- **`wtm upgrade`** updates wtm according to how it was installed: a standalone binary is replaced in place after checking its SHA256, a Homebrew or `go install` binary is handed to its manager, a binary built from source is refused; `--check` shows what is available, `--version` pins a release.
- **Update notice**: wtm mentions a newer version at the end of a command without blocking or slowing it, and the `wtm ui` header shows the installed version and the update call.
- **`wtm fast-forward`** moves a branch to `origin/<branch>` without rebase or merge; a diverged branch is refused, even with `--force` (use `wtm sync`); available from the dashboard and the CLI (`--all` or interactive selection).

### Changed

- **`wtm ui` help overlay** is split into four sections (NAV, ACT, MOUSE, VIEW), side by side on a wide screen and stacked on a narrow one, sized to the screen with scrolling, responds to the wheel and clicks, and lists `h`/`l`.

### Fixed

- **`wtm sync`** interactive shows its recap instead of an empty picker.
- **Loading placeholders** no longer show "No matches" (the `sync` plan, the `clean` worktree checks).

## [0.26.1] - 2026-08-20

The `wtm ui` detail panel no longer flickers.

### Fixed

- **`wtm ui`** no longer reloads the detail panel every three seconds; it reloads when the selection changes, when an operation touches its branch, and on `r`.

## [0.26.0] - 2026-08-20

A full-screen dashboard, `wtm ui`, to drive worktrees; no existing command, flag or JSON payload changes.

### Highlights

- **`wtm ui`**: a full-screen dashboard running `create`, `clean`, `reparent`, `prune` and `sync` with their questions, safety refusals and streamed output in a dedicated panel, with keyboard and mouse navigation and full help on `?`.

### Added

- **`wtm ui` Tree tab** shows the forest of stacked branches and reparents from the dashboard.
- **`wtm ui` detail panel** describes a worktree's work: last commit and recent activity, working tree state (`4 modified · 2 untracked · 1 staged` with diff size), children, `.env` drift, what blocks a removal, and the pull request with its CI checks and review decision; empty sections are hidden.
- **`wtm ui` detail panel** tells apart data being reloaded, a legitimate absence (`not configured`) and a failure (`⚠ unavailable — <reason>`), so a broken `gh` no longer reads as "no PR".
- **`wtm ui`** opens the pull request in the browser with `p` or a click on its line.
- **`wtm list`** marks the current worktree `● active`, in text output and the interactive picker, and **`wtm resolve`** shows it too.
- **`ui.animations`** in the global config (`~/.config/wtm/config.toml`): `false` turns off every dashboard animation.

### Changed

- **New colour palette** for the human output of every command, still adaptive to light/dark and respecting `NO_COLOR`.

### Fixed

- **`wtm sync`** refines the parent's state, keeps the parent step visible, and refreshes parents the cascade does not cover.
- **README** no longer documents an `agent` key in the global config, which made every command fail with `unknown keys in config.toml: agent`.

## [0.25.0] - 2026-08-17

`create`, `checkout` and `extract` reuse an existing local branch.

### Breaking

- **`wtm create <existing-branch> --yes`** and **`wtm extract --to <existing-branch> --yes`** require `--from`, since the parent of a branch created outside wtm cannot be guessed: pass `--from <parent>`. A new branch keeps its `base_branch` default, and `checkout <PR>` keeps the PR base.

### Added

- **`wtm checkout <PR>`** reuses an existing local branch instead of asking for a `wtm clean`, and offers an interactive fast-forward when it is behind origin (never under `--yes` or JSON); the recap announces the reuse before the fetch.
- **`wtm create <existing-branch>`** and **`wtm extract --to <existing-branch>`** reuse the branch: the source becomes an optional sync parent in the wizard, `--ff` targets the reused branch, and the recap and output show `Branch: x (existing local branch — reused)` and `Parent:`.
- **JSON** of `create` and `checkout` gains `existing_branch`, `origin_state` (`up-to-date`/`behind`/`ahead`/`diverged`), `origin_ahead` and `origin_behind`.

### Changed

- **A branch checked out in another worktree** exits `10` with a `wtm go <branch>` hint instead of a raw git error with exit `1`; `--if-not-exists` returns the existing worktree, even when it is main.
- **`create`** inspects the target branch once per run instead of making dozens of redundant git calls.

## [0.24.1] - 2026-08-11

`wtm extract` handles untracked files properly.

### Added

- **`wtm extract --files`** accepts a directory and takes every change below it.
- **`wtm extract --output json`** reports staged renames with status `renamed` and an `orig_path` field, tagged `ren` in the picker.

### Changed

- **`wtm extract`** removes directories left empty in the source after moving their files.
- **`wtm extract --on-conflict resolve`** covers untracked files already present in the target (conflict markers via an empty-base merge); `abort` stays the default, including under `--yes`, identical content is no longer a conflict, and a binary file still aborts with exit `15`.

### Fixed

- **`wtm extract`** lists each file of a brand-new directory individually instead of a single all-or-nothing entry, so `--files newmod/x.go` works.
- **`wtm extract`** handles paths with spaces or non-ASCII characters, tracked files included.
- **`wtm extract`** handles staged renames, moving both the deletion and the addition.

## [0.24.0] - 2026-07-08

`wtm env` detects and resolves `.env` drift between worktrees.

### Added

- **`wtm env [branch]`** detects and resolves a worktree's `.env` conflicts according to the strategy chosen at creation (main, parent, example); without an argument it opens a picker.

## [0.23.0] - 2026-07-07

`on_clean` hooks, generalized `.env` detection and harmonized output.

### Breaking

- **`.env` config** moves from a flat `copy_files` to structured `[[env.file]]` entries (`target`, `template`, `local`), with no automatic migration: regenerate it with `wtm init --only env` or edit `config.toml` by hand.

### Added

- **`[hooks] on_clean`** runs in the worktree just before `clean`/`prune` remove it (e.g. `docker compose down`); a non-zero exit aborts the removal unless the entry sets `continue_on_error`; placeholders `{{worktree}}`, `{{branch}}`, `{{root}}`.
- **`wtm init --clean-command`** and **`--skip-clean`** configure `on_clean` non-interactively.
- **`sudo rm -rf` fallback**: when `git worktree remove` hits files the user cannot delete (typically root-owned Docker files), an interactive run offers `sudo rm -rf` as a last resort, never under `--yes` or JSON, and never on a dangerous path (filesystem root, `$HOME`, the repository or its ancestors).
- **`.env` detection** recognises `.env.example`, `.env.dist`, `.env.sample`, `.env.template` and `.env.tmpl` as templates (in that order), tells templates from value files, and flags `.env.local` as `local = true`.

### Changed

- **`wtm init`** frames its final recap like other commands, and output conventions (icons, blank lines, counts) are harmonized across commands.
- **The `example` strategy** copies the detected template (`.env.dist`, `.env.sample`…) instead of hard-coding `.env.example`.

### Fixed

- **`wtm sync`** shows a "Pushing to origin…" spinner, drops a double blank line above the recap, and lists conflicting files vertically, capped at 5 (`…+N more`).

## [0.22.0] - 2026-07-03

An opt-in `run` module, unified `--yes`/`--force` bypass and harmonized wizards.

### Breaking

- **`--output json`** requires `--yes` on every mutating command (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`, `extract`, `checkout`), and a required selection without a default errors naming the flag instead of opening a picker: pass `--yes` and the named flags.
- **The `run` module** is opt-in: `wtm init` no longer configures services (`--skip-services` and `--only services` are removed), use `wtm run init`; any `run` command on an uninitialized module exits `16`.

### Added

- **`wtm run init`** sets up `run.toml` from detection (docker-compose and package scripts), with a wizard pre-filled on re-run or non-interactive generation, merging additively without overwriting existing jobs.
- **`origin` divergence badges** in `list`, `tree`, pickers and JSON (`base ↑N`, `origin ↑a ↓b`), read from cached remote-tracking refs without fetching; `r` refreshes.
- **`--ff`** fast-forwards a source that is only behind non-interactively, with the stale-source check extended to `create --from` and `extract`.
- **`wtm extract [source]`** picks the source in a first step (worktrees with changes only), so you can extract from anywhere; `[source]` is required non-interactively and with `--output json`, and `-y/--yes` skips the final recap.
- **`wtm reparent`** reparents several worktrees onto one new parent in a single pass, the recap listing each one's current parent.

### Changed

- **`--yes` and `--force`** are two separate axes on all 8 mutating commands: `--yes` resolves each decision by its flag or a safe default (sync does not push, extract aborts on conflict, clean/prune leave orphans) and never falls back to a picker; `--force` only lifts safety refusals and never implies `--yes`.
- **Wizards** keep every confirmation inside the wizard with a breadcrumb and Back, show skip reasons and a constant `No, cancel`; `clean` is a single wizard (picker → removal → reparent) and `relocate` edits `base_path` in the same wizard.
- **`wtm sync`** makes the plan preview the final wizard step, Esc going back instead of aborting.
- **`wtm agents install`** updates an already installed skill and reports `created`, `updated`, `unchanged` or `skipped` per destination.
- **`wtm init`** points out pre-existing worktrees and suggests `wtm relocate` to adopt them.

### Fixed

- **`wtm prune`** detects merged/closed from the GitHub PR state instead of local commits: `--merged` is a merged PR, `--closed` a PR closed without merge, `--gone` a deleted remote branch; a branch without a PR is never tagged, a warning goes to stderr when `gh` is missing, and JSON values are `pr_merged`/`pr_closed`/`gone`.
- **`wtm clean`** and **`wtm relocate`** in a non-TTY run without `--yes` error out instead of starting a wizard on a non-interactive stdin.
- **`wtm clean --reparent-children`** is honoured in the interactive wizard.

## [0.21.0] - 2026-07-01

Clean up finished work in one pass with `wtm prune`, keep a conflicting rebase open with `sync --keep-conflict`, and get JSON output from every command.

### Added

- **`wtm prune`** removes every worktree whose work is done in one pass, and reparents surviving children onto their grandparent, as `clean --reparent-children` does.
- **`wtm prune --merged`**, **`--closed`** and **`--gone`** narrow it to branches with no commit ahead of the base (squash merges not detected), with a merged or closed PR (needs `gh`), or whose remote branch was deleted.
- **`wtm prune --gone`** runs `git fetch --prune` first, unless **`--no-fetch`**.
- **`wtm prune`** shows the matches for review on a terminal (unsafe ones unchecked), then asks to confirm the prune, then asks separately about reparenting children.
- **`wtm prune`** never touches the main worktree or the base branch, and sends the shell back to the base repository when it removes the current worktree.
- **`wtm prune`** treats a dirty worktree, unpushed commits or an open PR as unsafe and needs `--force`; under `--yes` or `--output json` they are reported under `skipped` with a reason (`dirty`, `unpushed`, `open_pr`) instead of removed.
- **`wtm prune --dry-run`** previews without changing anything; **`--yes`** skips the prompts (required with `--output json`); non-interactively, children stay orphaned unless **`--reparent-children`**.
- **`wtm sync --keep-conflict`** leaves a conflicting rebase in progress in its worktree for manual resolution instead of aborting it.
- **`wtm sync`** detects a rebase already in progress (status `rebase_in_progress`) and blocks its descendants instead of retrying.
- **`wtm sync --output json`** reports `conflict_files`, `kept_in_progress` and `path`.

### Changed

- **`--output json`** is available on every command, with a stable payload (empty lists as `[]`, never `null`, never framed).
- **`wtm --help`** groups commands into sections: Worktrees, Navigate, Stacked branches, Dev jobs, GitHub, Setup.
- The full command reference under `docs/` is generated from the CLI, and the README is a concise guide pointing to it.
- The **`using-wtm`** agent skill is tighter and documents `prune` and `sync --keep-conflict`.

### Fixed

- **`wtm sync`** captures the conflicting files before aborting, and finds the branch of a worktree stuck mid-rebase.

## [0.20.0] - 2026-07-01

Branch pickers show remote branches and divergence, and multi-select lists can be filtered.

### Added

- **`wtm create --from origin/x`** creates a worktree from a remote-tracking branch you have not checked out locally.
- **`wtm reparent --to origin/x`** reparents onto a remote integration branch; `reparent` accepts a local branch or an `origin/x` ref.
- Branch pickers (`create`, `checkout`, `reparent`, `relocate`, `init`) list local and `origin` branches, remote ones grouped after a separator and tagged `remote`, hidden when a local branch has the same name.
- Branch pickers tag a local branch that drifted from `origin/` with its ahead/behind count (`↓5`, `↑2`, `↑2 ↓5`).
- Branch pickers fetch `origin` in the background on open and refresh the badges; **`r`** fetches again; offline, the last known counts are shown.
- **`wtm create`** offers to fast-forward a source branch that is strictly behind `origin/` before creating the worktree.
- **`wtm create`** skips that fast-forward when the branch's worktree has uncommitted changes, and asks whether to create from the local branch as is (default no).
- **`wtm create`** warns when the source branch has diverged from `origin/` and, on confirmation, creates from the local branch, keeping its commits.
- Multi-select lists (`sync`, `extract`, the `init` wizard) filter on **`/`** (case-insensitive substring); **`a`** toggles all filtered items, selections survive filter changes, **`Esc`** clears the filter, then cancels.

### Changed

- Worktree lists have uniform row spacing, compact coloured badges aligned in columns, a tinted selected row with a left-edge marker, and a right-aligned status with a glyph (`✓ clean`, `⚠ dirty`).
- **`create`**, **`extract`** and **`checkout`** ask before the `parent` env strategy copies `.env` from the main worktree because the source branch has no local worktree.

## [0.19.0] - 2026-06-27

A stacked-branch workflow: see the tree, reparent a branch, and sync only what you pick.

### Breaking

- **`wtm sync`** no longer cascades over everything by default: pass branch names, pick them interactively, or use **`--all`**; with `--output json`, branch names or `--all` are required.

### Added

- **`wtm tree`** shows the forest of worktrees (parent → child), annotated with `↑N`, `● dirty` and `⚠ needs sync` when a parent moved ahead and the child needs a rebase.
- **`wtm tree`** shows a parent without a worktree as a greyed `(no worktree)` root, and marks a `source_branch` cycle `⚠ cycle` instead of crashing.
- **`wtm tree --with-prs`** adds PR numbers and merged/closed state; **`--output text|json|mermaid`** prints for agents or as a Mermaid `flowchart TD` to paste into a PR.
- **`wtm reparent <branch> --to <parent>`** changes a worktree's recorded parent after creation; the rebase happens on the next `wtm sync`.
- **`wtm reparent`** runs as a wizard showing the current parent, or from arguments and `--output json`, and refuses cycles and self-parenting.
- **`wtm clean`** detects the children it would orphan and offers to reparent them onto the grandparent (`Esc` cancels the whole clean), or **`--reparent-children`** non-interactively.
- **`wtm sync`** opens a multi-select picker when run without arguments on a terminal.

### Changed

- **`wtm sync`** always refreshes the base first (unless the main worktree is dirty); selecting the base only fetches and fast-forwards it.
- **`wtm sync`** exits with code `11` on an unknown branch argument.
- Every command's human output has the same vertical spacing; JSON and machine output (`resolve`, `shell-init`) stay unpadded.
- Loading spinners are one bordered loader across commands; without a terminal or with JSON, no spinner is drawn and no `\r` reaches a pipe.

### Fixed

- The **`relocate`** wizard breadcrumb, which names the worktree, no longer scrolls off screen on long lists; older completed steps fold into `… (N earlier steps)`, in every wizard.
- A fast task's first output is no longer lost from its stream.

## [0.18.0] - 2026-06-24

`wtm checkout` replaces the `pr` group.

### Breaking

- **`wtm pr checkout`** is now **`wtm checkout`**: update scripts and aliases; its `--output json` stays `{number, branch, path}`.
- **`wtm pr list`** is removed: use `wtm list --with-prs` or the `checkout` wizard.

### Added

- **`wtm checkout [number]`** creates a worktree from a pull request; without a number, a wizard (PR → parent branch → env strategy) streams open PRs in the background, disabling those already checked out or from a fork.
- **`wtm checkout --review`** / **`--mine`** filter PRs, **`--from <branch>`** sets the sync parent (default: the PR's base), **`--env-from example|main|parent`** overrides the env strategy; each skips its wizard step.

## [0.17.0] - 2026-06-24

Worktree commands move to the top level.

### Breaking

- **`wtm wt`** is removed: `wtm wt list` becomes `wtm list`, and likewise `create`, `clean`, `sync`, `relocate`, `go`, `switch`, `extract`; update scripts and aliases, and re-run `eval "$(wtm shell-init)"`.

### Fixed

- A detached job's output is no longer truncated when its process exits.

## [0.16.0] - 2026-06-22

`wt relocate` gathers worktrees under `base_path`, and unused configuration is removed.

### Breaking

- **`wtm pr create`** is removed: use `gh pr create`.
- The project `config.toml` refuses `[agents]`, `[integrations]` and `[github]`, and the global config refuses `agent`: delete those lines or re-run `wtm init`.

### Added

- **`wtm wt relocate`** moves scattered worktrees under `base_path` and adopts external worktrees into wtm, with a preview before running.
- **`wtm wt relocate`** runs as a wizard, or without a terminal via `--to`, `--force` and `--output json` (per-worktree status: `moved`, `moved_adopted`, `adopted`, `skipped`, …).

### Removed

- The default-agent setting: the `agent` key, the `--agent` flag and its `init` wizard step.
- **`[github] auto_draft`**, unused since `pr create` was removed.

## [0.15.0] - 2026-06-20

`wt sync` rebases the whole chain of worktrees in one command.

### Added

- **`wtm wt sync`** updates every worktree in topological order: fast-forwards each branch from its own `origin/<branch>`, then rebases it `--onto` its refreshed parent, replaying only its own commits, all locally.
- **`wtm wt sync`** shows a recap (parent, target commit, before → after, replayed commits) before offering a single push of the rebased branches with `--force-with-lease`.
- **`wtm wt sync --dry-run`** previews offline; **`--base <branch>`**, **`--push`** (the only way to push with `--output json`), **`--no-push`** and **`-y`/`--yes`**.
- **`wtm wt sync --output json`** reports a status per branch: `synced`, `up_to_date`, `skipped_dirty`, `skipped_ancestor`, `diverged`, `conflict` (rebase aborted, tree clean), `error`, `unknown_parent`; it exits non-zero on any `conflict` or `error`.

### Fixed

- **`wtm wt sync`** reports a failing git command as a blocking `error` instead of a false `up_to_date`, and pushes only when `origin/<branch>` is really missing.

## [0.14.0] - 2026-06-20

Worktree lists show up instantly while PRs stream in.

### Added

- **`wtm wt list --with-prs`** includes PRs in non-interactive output, identically in text and JSON.

### Changed

- **`wt list`**, **`wt go`** and **`wt switch`** show worktrees immediately; PR badges fill in as they arrive, with a progress banner and an install/login hint when `gh` is unavailable.
- **`wt list`** no longer fetches PRs by default in non-interactive output.
- Worktree pickers fetch lighter PR data; `pr list` keeps the full detail.
- The **Open PR** action is available immediately, its URL resolved while loading.

## [0.13.0] - 2026-06-20

`wtm init` reworked: skip sections, re-initialise one with `--only`, edit `on_create` hooks.

### Added

- **`wtm init`** opens each optional section (`env`, `hooks`, `services`) with a Configure / Skip step; a skipped section is written commented out, ready to enable.
- **`wtm init --skip-env`**, **`--skip-hooks`**, **`--skip-services`** skip sections non-interactively.
- **`wtm init --only <section>`** re-initialises `worktrees`, `env`, `hooks` or `services` (CSV or repeated) without touching the others, pre-filled from the existing config; `run.toml` keeps its profiles.
- **`wtm init`** edits `on_create` hooks as a list: add, edit, remove and reorder (`shift+↑/↓`) entries, each with `cmd`, optional `cwd` and `continue_on_error`.

### Changed

- **`wtm init`** points to `--only` when a config already exists.

### Removed

- The install command and monorepo packages steps of `wtm init`: use the `on_create` hook editor.

## [0.12.0] - 2026-06-20

`wt extract` moves uncommitted changes between worktrees.

### Added

- **`wtm wt extract`** moves part of the current worktree's uncommitted changes to a new or existing worktree, through a Files → Target → Mode wizard or `--files`, `--to`, `--from`, `--keep`, `--on-conflict` and `--output json`.
- **`wtm wt extract`** moves files by default; **`--keep`** copies them.
- **`wtm wt extract`** cleans the source only once the whole extraction applied; on any conflict the source is left untouched.
- **`wtm wt extract --on-conflict abort`** (default) changes nothing and exits with code `15`; **`resolve`** writes git conflict markers in the target and keeps the source intact.

## [0.11.0] - 2026-06-19

wtm can be driven by agents, and detached services stream their startup logs.

### Breaking

- A command run outside an initialised repository exits with code `12` instead of `0`: adjust scripts relying on a silent success.
- **`wtm pr create`** exits with code `13` when a PR already exists, instead of `0`; in JSON the existing PR is printed on stdout.

### Added

- **`wtm pr create --yes`** pushes an unpushed branch and skips prompts (implied by `--output json`).
- **`wtm init --non-interactive`**, with `--agent`, `--shell`, `--base-path`, `--base-branch`, `--env-strategy` and `--install-command`, bootstraps a project from flags, then detection, then defaults; it fails if the base branch cannot be found.
- **`wtm wt create --if-not-exists`** succeeds with `already_exists: true` when the worktree exists.
- Exit codes per failure: `10` worktree exists, `11` branch not found, `12` config not found, `13` PR already exists, `14` job not declared.
- **`wtm run up`** streams a detached service's startup output (`docker compose up -d` creating networks and containers) instead of a spinner.

### Changed

- **`wtm wt clean`** succeeds as a no-op on a worktree already gone (`already_absent: true`).
- **`run stop`** / **`run down`** are no-ops on a job already stopped; an undeclared job exits with code `14`.
- **`wtm pr checkout`** on a fork PR is still refused, with a message pointing to `gh pr checkout`.

### Fixed

- **`wtm pr create --output json`** no longer stops silently on an unpushed branch.

## [0.10.0] - 2026-06-09

`run up` and `run start` launch and tail in one step, and profiles run jobs in your order.

### Added

- **`wtm run up`** / **`run start`** start jobs and stream their output straight away: services in the background, tasks live.
- **`run profile add`** / **`edit`** add an Order step to reorder jobs (`shift+↑/↓` or `J`/`K`); the order is saved in `run.toml` and followed at run time.

### Changed

- A failed task aborts the rest of the profile cleanly and shows its failure logs.
- Loading spinners and ellipses (`…`) are consistent across commands.
- The **`wt go`** / **`wt switch`** picker shows a loading spinner, fetches worktrees in parallel and shows a callout when `gh` is missing.

## [0.9.0] - 2026-06-08

An interactive `wt list`, `run.toml` export/import and commands to edit jobs and profiles.

### Breaking

- wtm keeps its config, `run.toml`, schemas and per-worktree metadata in `<git-common-dir>/wtm/` instead of `.wtm/` in the repository: move existing files there or re-run `wtm init`.
- **`wtm run list --output json`** uses lowercase keys (`job`, `name`, `kind`) instead of PascalCase, matching `run.schema.json`.
- The shell wrapper changed: re-run `eval "$(wtm shell-init)"` (or re-source your shell config) to get the return to the base repository.

### Added

- **`wtm wt list`** shows a loading spinner, a banner when the GitHub CLI is missing or not authenticated, and an **Open PR** action.
- Removing the worktree you are in (`wt list` → Clean, or `wtm wt clean`) sends the shell back to the base repository.
- **`wtm init`** offers the `package.json` scripts, and each pnpm workspace's, as jobs: `dev`/`start`/`serve`/`watch` (and `dev:*`, `*:dev`) preselected as services, the others as tasks.
- **`wtm run export [--profile <name>]`** prints `run.toml` as JSON on stdout, optionally one profile and its jobs.
- **`wtm run import [file|-]`** merges a JSON payload into `run.toml`, skipping duplicates with a warning; **`--replace --force`** overwrites the file.
- **`wtm run job add|rm|edit|list`** and **`wtm run profile add|rm|edit|list`** manage `run.toml` declarations, by wizard or by flags (`--cmd`, `--kind`, `--stop`, `--cwd`; `--jobs`, `--default`).
- **`run job rm`** / **`run profile rm`** open a picker without an argument; **`run job rm --force`** also removes the job from profiles.
- **`run job edit`** / **`run profile edit`** open a picker then a pre-filled wizard, renames included; orphaned references are reported.
- **`run job list`** / **`run profile list`** open an Edit/Remove picker on a terminal and print the list with `--output json` or in a pipe.
- **`wtm config show`** and **`wtm config edit`** show and edit the config without digging into the git directory.

### Changed

- Setting a profile as default unsets the previous one instead of failing, and the wizard warns before switching.
- Text inputs re-validate on every key: an error stays visible while the value is invalid and clears once it is valid.

## [0.8.0] - 2026-05-01

Config files are decoded strictly and come with JSON Schemas for IDE autocomplete.

### Added

- JSON Schemas for `run.toml`, the project `config.toml` and the global `config.toml` ship in the binary, are written next to them by `wtm init`, and each generated file starts with a `#:schema` directive.
- **`wtm schema dump`** writes the embedded schemas to disk, to refresh them after an upgrade; **`--global`** targets the global one.
- Editors with Taplo ("Even Better TOML" for VS Code, Cursor, JetBrains) get autocomplete on fields and enums, hover docs and live errors.

### Fixed

- Unknown keys in a config file (`[[profiles]]` for `[[profile]]`) are rejected with `unknown keys in <path>: profiles` instead of ignored.

## [0.7.2] - 2026-04-29

The terminal is restored after detaching from a job's logs.

### Fixed

- Leaving **`wtm run logs`** on a job with an interactive TUI (turbo, vite, vim) no longer leaves the terminal in mouse tracking, alternate screen and hidden cursor, on Ctrl+C, EOF or a connection error.

## [0.7.1] - 2026-04-29

Stopping a job stops its whole process tree.

### Fixed

- **`wtm run stop`** / **`run down`** signal the job's whole process group instead of only `npm`/`pnpm`, wait for it to exit before marking it stopped, and send SIGKILL after 5 s if SIGTERM is ignored.

## [0.7.0] - 2026-04-29

Services and one-shot tasks are unified as jobs in `run.toml`.

### Breaking

- **`.wtm/services.toml`** is replaced by **`.wtm/run.toml`**, with `[[job]]` (`kind = "service"` or `"task"`) and `[[profile]]` (`jobs = [...]`): rewrite your file, the old one is no longer read.
- **`wtm svc`** is renamed **`wtm run`** (`up`, `down`, `ps`, `logs`, `start`, `stop`, `list`): update scripts and aliases.
- The **`wt switch`** shell wrapper calls `wtm run up`: regenerate it with `wtm shell-init`.

### Added

- **`kind = "task"`** declares a one-shot command (migration, seed, formatter) that must succeed before the profile continues; a failure aborts the rest of the profile.
- Tasks stream their output live, and leave `run ps` once they exit.
- **`wtm run ps`** shows a `KIND` column in its table and picker.
- **`run.toml`** is validated before anything runs: `kind` is required, a task cannot have `stop`, profiles may only reference declared jobs.

### Changed

- **`wtm init`** writes detected docker-compose files as detached `[[job]]` services with a `stop` command.
- The **`using-wtm`** agent skill uses the new vocabulary (jobs, kinds, `run.toml`, `wtm run`).

## [0.6.2] - 2026-04-13

More output polish.

### Fixed

- The **`wt go`** / **`wt switch`** picker keeps its colours when run through the shell wrapper.
- **`svc up`** puts a blank line between the "stop other services?" prompt and the result.

## [0.6.1] - 2026-04-12

Output polish.

### Changed

- **`svc up`**, **`pr create`** and **`svc ps`** pad their warnings consistently.
- **`wtm init`** highlights its "No .wtm/config.toml found" intro and pads its success messages.
- **`pr create`** adds a blank line after the "Open in browser?" prompt, whatever the answer.

### Fixed

- The **`wt go`** / **`wt switch`** picker keeps its highlight and badges when run through the shell wrapper.

## [0.6.0] - 2026-04-12

wtm can be driven by LLM agents.

### Added

- **`--output json`** on `wt list`, `wt create`, `wt clean` (with `--force`), `pr list`, `pr create`, `pr checkout`, `svc list`, `svc ps`, `svc up`, `svc down`, `svc start` and `svc stop`, with human text on stderr.
- **`wtm svc list`** lists declared services and profiles; on a terminal, a picker offers `up`/`down` on a profile and `start`/`stop`/`logs` on a service.
- **`wtm svc ps`** lists the services the daemon is running (name, status, PID, worktree), with `stop`/`logs`/`restart` actions and "Stop all running services".
- **`wtm agents install`** installs a `using-wtm` skill into the `.claude/` or `.cursor/` directories it finds, project or global.
- **`wtm init`** detects `docker-compose*.yml`/`.yaml` files and scaffolds matching services, using `docker compose` (v2) or `docker-compose` (v1).
- **`wtm svc down --all`** stops every service of every worktree.

### Changed

- **`wt switch`** without an argument shows the same picker as `wt list`.
- **`svc up`**, **`svc down`** and **`svc start`** show a spinner while waiting on the daemon.
- Errors carrying captured output (docker compose) print as an indented block.
- **`wtm svc down`** without `--all` only touches the current worktree.

### Fixed

- A service with a `stop` command no longer reports `✓ started` when `docker compose up -d` fails; the error is shown with its output.
- **`svc down`** (and `wt clean`, `svc up --exclusive`) no longer stops services of other worktrees.

## [0.5.1] - 2026-04-12

Pickers and shell navigation work through the shell wrapper.

### Fixed

- The **`wt go`** / **`wt switch`** picker is visible when run through the shell wrapper.
- "Go to worktree" from **`pr list`** and **`wt list`** navigates instead of saying shell integration is required.
- The shell wrapper (bash, zsh, fish) lets any subcommand change the directory.

## [0.5.0] - 2026-04-12

New pickers and wizards, `wt switch`, and focus removed in favour of services.

### Breaking

- **`wtm wt focus`** is removed, along with active-worktree tracking: use `svc up` / `svc down`.
- The **`on_focus`** / **`on_blur`** hooks are removed: keep only `on_create`, and let the service manager run Docker.
- The dashboard is hidden while it is reworked: `wtm` without arguments shows help.

### Added

- **`wtm wt switch [branch]`** goes to a worktree and runs `svc up`, with `--exclusive`, `--parallel` and `--profile`.
- **`svc up`** detects services running in other worktrees and asks to stop them; **`--exclusive`** stops them, **`--parallel`** skips the question.
- **`wt clean`** stops a worktree's running services before deleting it.
- Pickers and wizards highlight the full row, filter on **`/`**, show a step breadcrumb and go back with **`Esc`**.
- The **`pr list`** picker offers "Go to worktree" when the PR branch has one, "Checkout into worktree" otherwise.
- The **`wt list`** picker shows right-aligned badges: parent, PR, services, dirty/clean.

### Changed

- All messages share one style and indent, every command is padded top and bottom, and help text is indented to match.
- Errors print through one styled `✗` handler.
- The PR detail view drops its box.
- Action lists group navigation, service and destructive actions with separators.

### Fixed

- Services using `docker compose up -d` are tracked as running and stopped properly.
- **`svc up`**, **`svc start`** and **`svc down`** read `services.toml` from the main worktree when run from another one.

### Removed

- The docker-compose file selection and hook steps of the `wtm init` wizard.

## [0.4.1] - 2026-04-12

GitHub access goes through the `gh` CLI.

### Breaking

- **`wtm auth login|status|logout`** are removed: install `gh` and run `gh auth login`.
- **`WTM_GITHUB_TOKEN`** is no longer read: use `GH_TOKEN`.

### Changed

- **`pr list`**, **`pr create`**, **`pr checkout`** and the dashboard's PR panel use `gh`.
- The README lists the dependencies: `git` required, `gh` recommended.

## [0.4.0] - 2026-04-10

GitHub integration and pull-request commands.

### Breaking

- Worktree commands move under **`wtm wt`** (`wtm wt new`, `wtm wt ls`, `wtm wt go`, …) and service commands under **`wtm svc`** (`up`, `down`, `start`, `stop`): update scripts and aliases.

### Added

- **`wtm auth login`** signs in to GitHub with the OAuth device flow; **`auth status`** shows the token state, **`auth logout`** revokes it; `WTM_GITHUB_TOKEN` accepts a personal access token.
- **`wtm pr list`** lists the repository's pull requests, with **`--mine`** and **`--review`**, also in the dashboard (`p`).
- **`wtm pr create`** creates a PR from the current branch through a wizard: title, body, draft, reviewers.
- **`wtm pr checkout`** creates a worktree from an existing PR's branch.
- **`wtm svc start`** / **`stop`** act on single services, **`up`** / **`down`** on whole profiles.

### Changed

- The dashboard splits worktrees and PRs 50/50 and multiplexes logs.

### Fixed

- Dashboard focus handling.

## [0.3.0] - 2026-04-04

Services run in a background daemon, each in its own terminal.

### Breaking

- Project config moves from `.wtm.toml` to **`.wtm/config.toml`**: move the file.

### Added

- **`wtm up`** starts services from `.wtm/services.toml` profiles in a background daemon, scoped to the worktree; a picker opens when several profiles exist and no `--profile` is given.
- **`wtm down`** stops the worktree's services.
- **`wtm logs`** attaches to a service's terminal, with full colours.
- The dashboard starts (`u`), stops (`x`) and attaches to (`s`) the selected worktree's services, and shows their status.
- Duplicate services or profiles in `services.toml` raise a warning.

## [0.2.1] - 2026-04-03

A fix for the dashboard launched from the shell.

### Fixed

- Opening the dashboard through the shell wrapper.

## [0.2.0] - 2026-04-03

An interactive dashboard.

### Added

- **`wtm`** without arguments opens a full-screen dashboard of every worktree.
- The dashboard lists worktrees with branch, clean/dirty status, commits ahead and focus indicator.
- The dashboard's detail panel shows path, source branch, unpushed commits, context notes and modified files, scrollable.
- The dashboard creates (`n`), cleans (`d`), focuses (`f`) and navigates to (`Enter`) worktrees.
- Focusing from the dashboard streams hook output live in a split panel, closed with `Esc`.
- **`Tab`** / **`Shift+Tab`** cycle panels; **`j`/`k`** or arrows scroll the active one.
- **`wtm new`** asks for the branch name when none is given.

### Fixed

- Hook errors in the dashboard show in the detail panel instead of corrupting the screen.

## [0.1.2] - 2026-04-02

Fixes for commands run from a child worktree.

### Fixed

- Commands run from a child worktree find the project config.
- Blur hooks no longer fail when the previous worktree's directory is gone.
- The shell wrapper returns to the main worktree after cleaning the current one.

## [0.1.1] - 2026-04-02

Initial release.

### Added

- **`wtm init`** sets up global and project configuration through a wizard.
- **`wtm new [branch]`** creates a worktree with env provisioning, metadata and hooks.
- **`wtm ls`** lists worktrees with their git status (clean/dirty, commits ahead).
- **`wtm go [branch]`** moves to a worktree through shell integration.
- **`wtm focus [branch]`** switches the active worktree and runs `on_blur` / `on_focus` hooks.
- **`wtm clean [branch]`** removes a worktree, refusing when it is dirty, unpushed or has an open PR.
- **`wtm shell-init`** generates the shell wrapper for zsh, bash and fish.
- TOML config: `.wtm.toml` in the project, `~/.config/wtm/config.toml` globally.
- Three env strategies: `example`, `main`, `parent`.
- Hooks with template variables, `continue_on_error` and timings.
- Detection of the base branch, env files, package manager, Docker Compose and pnpm workspaces.
- Install with Homebrew (`brew install LucasPcq/tap/wtm`), GitHub Releases binaries (macOS/Linux, amd64/arm64) or `go install github.com/LucasPcq/wtm@latest`.

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
