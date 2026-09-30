<h1 align="center">wtm</h1>

<p align="center">
  <strong>One branch, one worktree, one isolated dev stack.</strong><br>
  A worktree manager for teams that work on several branches at once, and let their agents do too.
</p>

<p align="center">
  <a href="https://github.com/LucasPcq/wtm/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/LucasPcq/wtm?sort=semver"></a>
  <a href="https://github.com/LucasPcq/wtm/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/LucasPcq/wtm/actions/workflows/ci.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue"></a>
</p>

<p align="center">
  <img alt="wtm create walks through its wizard, wtm go jumps in, wtm run up opens the run view" src="docs/assets/hero.gif" width="800">
</p>

## Why wtm

`git worktree` gives each branch its own directory. Everything around it is still yours to do: copy the `.env`, install dependencies, remember which directory holds which branch, rebase a stack of branches in the right order and, the hard part, run two branches' dev servers without them fighting over the same ports, containers and databases.

wtm does that part:

- **A worktree is ready when it is created:** `.env` copied, hooks run, ports shifted so it never collides with another one.
- **Each worktree runs its own stack:** dev servers and `docker compose`, on their own ports and under their own name, side by side.
- **Stacked branches stay in order:** every worktree knows its parent; `wtm sync` rebases the whole chain.
- **It speaks JSON:** data commands take `--output json`, changes take `--yes`, and `wtm agents install` teaches Claude Code or Cursor to drive it.

## Install

```bash
brew install LucasPcq/tap/wtm
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc   # lets `wtm go` change directory
```

<details>
<summary>Other ways to install</summary>

**Binary:** download the latest [release](https://github.com/LucasPcq/wtm/releases), extract it, and move `wtm` onto your `PATH`:

```bash
tar -xzf wtm_*_darwin_arm64.tar.gz   # or _darwin_amd64 / _linux_amd64
sudo mv wtm /usr/local/bin/
```

**Go**

```bash
go install github.com/LucasPcq/wtm@latest
```

**Shell integration:** `bash` and `fish` work the same way: `eval "$(wtm shell-init)"` in `~/.bashrc`, `wtm shell-init | source` in `config.fish`.

**Updating:** `wtm upgrade` updates wtm the way it was installed (Homebrew, `go install` or a standalone binary). wtm checks for a new release at most once a day; `WTM_NO_UPDATE_CHECK=1` turns that off.

</details>

wtm needs `git`. [`gh`](https://cli.github.com) is optional and unlocks the GitHub features (`checkout` a PR, PR status, `prune` of merged branches).

## Quick start

```bash
cd your-repo
wtm init                      # once per repository

wtm create feat/login         # a new worktree, provisioned
wtm go feat/login             # jump into it
wtm list                      # every worktree and its state
wtm clean feat/login          # remove it once the PR is merged
```

To run your dev stack per worktree, `wtm run init` detects your `docker-compose` files and package scripts and writes the config once; `wtm run up` starts it.

## Features

### Worktrees, provisioned

`wtm create` makes the worktree, copies the `.env` files from their template, the main checkout or the parent, and runs your `on_create` hooks (`pnpm install`, …). `wtm clean` and `wtm prune` remove them, one at a time or every branch whose PR is merged, and refuse a worktree with uncommitted or unpushed work unless you say `--force`.

### An isolated stack per worktree

Each worktree gets its own ports (`3000` on the main checkout, `3010` on the next, …), its own `COMPOSE_PROJECT_NAME`, and its own address: `http://web.feat-login.acme.localhost:11080` (on port 80 once `wtm run proxy install` redirects it, on macOS). Run as many branches as you like at the same time; a shared postgres can hold one database per worktree.

<p align="center">
  <img alt="wtm run up on two worktrees at once: each runs its own api and web on its own named URLs" src="docs/assets/isolation.gif" width="800">
</p>

### A dashboard for all of it

`wtm ui` shows every worktree, the branch tree, PR status and the running services, and lets you create, clean and start things from one screen.

<p align="center">
  <img alt="The wtm ui dashboard: worktrees, a new one created from the dashboard, the branch tree" src="docs/assets/dashboard.gif" width="800">
</p>

### Stacked branches

Every worktree records the branch it came from. `wtm tree` draws the forest, `wtm sync` rebases a branch and its descendants onto their parents, and `wtm reparent` rewires the chain when a middle branch merges.

```console
$ wtm tree
  main
  ├─ feat/login
  │  └─ feat/login-ui
  └─ fix/typo
```

### Built for scripts and agents

Data commands take `--output json` (the document is the schema), every change takes `--yes` so nothing prompts, and every command takes `--quiet`. `wtm agents install` adds a skill to Claude Code and Cursor so your agent drives wtm without being told how.

## Commands

Every command documents itself: `wtm <command> --help`, or the generated [reference](docs/wtm.md).

### Worktrees

| Command | Purpose |
|---|---|
| [`create`](docs/wtm_create.md) | Create a new worktree (runs env provisioning + `on_create` hooks) |
| [`list`](docs/wtm_list.md) | List all worktrees |
| [`tree`](docs/wtm_tree.md) | Show the worktree forest (parent → child) |
| [`clean`](docs/wtm_clean.md) | Remove a worktree and its local branch |
| [`prune`](docs/wtm_prune.md) | Remove finished worktrees (merged / closed PR / gone) in one pass (merged/closed need `gh`) |
| [`extract`](docs/wtm_extract.md) | Move uncommitted changes to another worktree (split an oversized PR) |
| [`env`](docs/wtm_env.md) | Detect and fix a worktree's `.env` drift against its template + value source |
| [`relocate`](docs/wtm_relocate.md) | Move worktrees to align with `base_path` and adopt external ones |
| [`ui`](docs/wtm_ui.md) | Open the full-screen worktree dashboard: browse state and PRs, create and delete worktrees |

### Navigate

| Command | Purpose |
|---|---|
| [`go`](docs/wtm_go.md) | cd into a worktree |
| [`resolve`](docs/wtm_resolve.md) | Print a branch's worktree path (for scripts / agents) |

### Stacked branches

| Command | Purpose |
|---|---|
| [`fast-forward`](docs/wtm_fast-forward.md) | Advance worktree branches to `origin/<branch>`, with no rebase and no merge |
| [`sync`](docs/wtm_sync.md) | Rebase selected worktrees onto their parent, in cascade |
| [`reparent`](docs/wtm_reparent.md) | Change the parent a worktree is rebased onto |

### Dev jobs

Opt-in: `wtm run init` sets it up once per repository.

| Command | Purpose |
|---|---|
| [`run init`](docs/wtm_run_init.md) | Set up run.toml (detect docker-compose + scripts, pre-fill ports, publish URLs, write and link .env keys) |
| [`run up`](docs/wtm_run_up.md) / [`down`](docs/wtm_run_down.md) | Start / stop a profile's jobs on one or more worktrees (`up` attaches, `-d` detaches) |
| [`run start`](docs/wtm_run_start.md) / [`stop`](docs/wtm_run_stop.md) | Start one job / stop one job, in one or more worktrees (`start` attaches, `-d` detaches) |
| [`run ps`](docs/wtm_run_ps.md) / [`list`](docs/wtm_run_list.md) | Running jobs, every repository / declared jobs + profiles |
| [`run logs`](docs/wtm_run_logs.md) | Reopen the run view on one or more worktrees' jobs |
| [`run url`](docs/wtm_run_url.md) / [`open`](docs/wtm_run_open.md) | Print / open where a job answers in this worktree |
| [`run export`](docs/wtm_run_export.md) / [`import`](docs/wtm_run_import.md) | Share a job layout between machines |
| [`run job`](docs/wtm_run_job.md) / [`profile`](docs/wtm_run_profile.md) | Add / remove / edit jobs and profiles |
| [`run addressing`](docs/wtm_run_addressing.md) | Switch the `.env` files between named URLs and port URLs, and settle the worktrees on it |
| [`run proxy`](docs/wtm_run_proxy.md) | Report, install or remove the redirection that serves named URLs on port 80 |
| [`run daemon`](docs/wtm_run_daemon.md) | Inspect, stop or restart the process that runs the jobs |

### GitHub

| Command | Purpose |
|---|---|
| [`checkout`](docs/wtm_checkout.md) | Create a worktree from an existing pull request (needs `gh`) |

### Setup

| Command | Purpose |
|---|---|
| [`init`](docs/wtm_init.md) | Initialize wtm configuration |
| [`shell-init`](docs/wtm_shell-init.md) | Generate the shell integration function |
| [`config`](docs/wtm_config.md) | Inspect or edit the project config |
| [`agents`](docs/wtm_agents.md) | Install the `using-wtm` skill for LLM agents |
| [`schema`](docs/wtm_schema.md) | Extract the bundled JSON Schemas |
| [`upgrade`](docs/wtm_upgrade.md) | Update wtm itself to the latest release |

## Documentation

- **[Getting started](docs/guide/getting-started.md):** from install to two branches running side by side, in ten minutes. Then [recipes](docs/guide/recipes.md) for common setups and [troubleshooting](docs/guide/troubleshooting.md).
- **[User guide](docs/guide/README.md):** [configuration](docs/guide/configuration.md), [isolation](docs/guide/isolation.md), [jobs and profiles](docs/guide/jobs-and-profiles.md), [how `wtm run` works](docs/guide/how-run-works.md), [shared services](docs/guide/shared-services.md), [named URLs](docs/guide/addressing.md), the [`run.toml` reference](docs/guide/run-toml.md), [where wtm keeps its state](docs/guide/state.md).
- **[Command reference](docs/wtm.md)**, generated from `--help`.
- **[Changelog](CHANGELOG.md)**, and [migrating to 0.28](docs/guide/migrating-to-0.28.md) if you come from 0.27.

## Contributing

`make lint` and `make test` must pass. The command reference under `docs/` is generated: change a command, then run `make docs`. The GIFs above are recorded from `docs/demos/*.tape` with [VHS](https://github.com/charmbracelet/vhs): `make demos` re-records them.

## License

[MIT](LICENSE)
