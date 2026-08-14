# wtm — Worktree Manager

Orchestrate git worktrees and team dev workflows from the terminal.

`wtm` manages the full lifecycle of git worktrees — creation, environment
provisioning, hooks, stacked-branch syncing, per-worktree dev services, navigation,
and cleanup — replacing manual `git worktree` juggling with one streamlined workflow.

> **This README is a guide, not a reference.** Every command is self-documenting:
> run `wtm <command> --help` for its full flags, or browse the generated reference in
> [`docs/`](docs/wtm.md). The tables below are just a map.

## Dependencies

| Tool | Required | Purpose |
|---|---|---|
| `git` | ✅ Required | Worktree management |
| `gh` | ⭐ Recommended | PR listing, creation, checkout, and prune (merged/closed detection) |

`gh` is optional — worktree creation, navigation, hooks, and services all work without
it. Install and authenticate it to unlock GitHub features: [cli.github.com](https://cli.github.com).

## Installation

**Homebrew (macOS / Linux)**

```bash
brew install LucasPcq/tap/wtm
```

**Download binary** — grab the latest [release](https://github.com/LucasPcq/worktree-manager-cli/releases), extract, and move it onto your `PATH`:

```bash
tar -xzf worktree-manager-cli_*_darwin_arm64.tar.gz   # or _darwin_amd64 / _linux_amd64
sudo mv wtm /usr/local/bin/
```

**Go install**

```bash
go install github.com/LucasPcq/wtm@latest
```

## Quick Start

```bash
# 1. Shell integration — required so `go`/`switch` can cd for you
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc && source ~/.zshrc

# 2. Initialize wtm in your repo
cd your-repo && wtm init

# 3. Everyday flow
wtm create feature/login     # create a worktree (env + hooks provisioned)
wtm go feature/login         # cd into it
wtm list                     # see all worktrees
wtm sync --all               # rebase the stack onto its parents
wtm clean feature/login      # remove it when the PR merges
```

## Teach your LLM to use wtm

Working with Claude Code or Cursor? Run:

```bash
wtm agents install
```

It detects `.claude/` and `.cursor/` (project and home) and installs a `using-wtm`
skill so your agent can drive every command via the [`--output json`](#machine-readable-output) contract — without being told how each session.

## Concepts

A few ideas explain how the commands fit together:

- **Worktree** — a checked-out branch in its own directory. wtm creates them under
  `base_path` (from `wtm init`) and records per-worktree metadata under
  `<git-common-dir>/wtm/`. See [`create`](docs/wtm_create.md), [`list`](docs/wtm_list.md), [`clean`](docs/wtm_clean.md).
- **Stacking** — every worktree records the parent branch it was created from
  (`source_branch`). [`sync`](docs/wtm_sync.md) rebases a worktree (and its descendants,
  in cascade) onto its parent; [`tree`](docs/wtm_tree.md) shows the parent→child forest and
  which branches need syncing; [`reparent`](docs/wtm_reparent.md) rewires the parent after a
  middle branch merges.
- **Env provisioning** — on `create`, wtm copies `.env` files into the new worktree using
  a **strategy** (`example` / `main` / `parent`). See [Env strategies](#env-strategies).
- **Shell integration** — `go` changes your current directory, which a child process can't
  do for its parent shell. `eval "$(wtm shell-init)"` installs a shell function that makes
  it work. Without it, use [`resolve`](docs/wtm_resolve.md) to get a path.
- **Dev jobs** *(experimental)* — long-running **services** (dev servers, docker) and
  one-shot **tasks** (migrations, seeds) declared in `run.toml` and grouped into
  **profiles**, run per-worktree by a background daemon. The flow is still stabilizing —
  `wtm go` is the recommended way to enter a worktree today. See [`run`](docs/wtm_run.md)
  and [Run config](#run-config--runtoml).

## Commands

Full flags live in `wtm <command> --help` and [`docs/`](docs/wtm.md). Overview:

### Worktrees

| Command | Purpose |
|---|---|
| [`create`](docs/wtm_create.md) | Create a new worktree (runs env provisioning + `on_create` hooks) |
| [`list`](docs/wtm_list.md) | List all worktrees |
| [`tree`](docs/wtm_tree.md) | Show the worktree forest (parent → child) |
| [`clean`](docs/wtm_clean.md) | Remove a worktree and its local branch |
| [`prune`](docs/wtm_prune.md) | Remove finished worktrees (merged / closed PR / gone) in one pass — merged/closed need `gh` |
| [`extract`](docs/wtm_extract.md) | Move uncommitted changes to another worktree (split an oversized PR) |
| [`env`](docs/wtm_env.md) | Detect and fix a worktree's `.env` drift against its template + value source |
| [`relocate`](docs/wtm_relocate.md) | Move worktrees to align with `base_path` and adopt external ones |

### Navigate

| Command | Purpose |
|---|---|
| [`go`](docs/wtm_go.md) | cd into a worktree |
| [`switch`](docs/wtm_switch.md) | cd into a worktree and start its dev services *(experimental)* |
| [`resolve`](docs/wtm_resolve.md) | Print a branch's worktree path (for scripts / agents) |

### Stacked branches

| Command | Purpose |
|---|---|
| [`sync`](docs/wtm_sync.md) | Rebase selected worktrees onto their parent, in cascade |
| [`reparent`](docs/wtm_reparent.md) | Change the parent a worktree is rebased onto |

### Dev jobs *(experimental)*

Per-worktree services + tasks. Functional, but the flow is still stabilizing. The
run module is **opt-in**: run `wtm run init` once to set it up (the global `wtm init`
no longer touches services). Until then, run commands stop with a hint pointing there.

| Command | Purpose |
|---|---|
| [`run init`](docs/wtm_run_init.md) | Set up run.toml (detect docker-compose + scripts) |
| [`run up`](docs/wtm_run_up.md) / [`down`](docs/wtm_run_down.md) | Start / stop a profile's jobs |
| [`run start`](docs/wtm_run_start.md) / [`stop`](docs/wtm_run_stop.md) | Start / stop a single job |
| [`run ps`](docs/wtm_run_ps.md) / [`list`](docs/wtm_run_list.md) | Running jobs / declared jobs + profiles |
| [`run logs`](docs/wtm_run_logs.md) | Stream a job's output |
| [`run export`](docs/wtm_run_export.md) / [`import`](docs/wtm_run_import.md) | Share a job layout between machines |
| [`run job`](docs/wtm_run_job.md) / [`profile`](docs/wtm_run_profile.md) | Add / remove / edit jobs and profiles |

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

## Machine-readable output

Every data command supports `--output json` for scripting and LLM agents. JSON is
pretty-printed on stdout with `snake_case` fields; human messages stay on stderr; exit
codes are unchanged.

**The payload *is* the schema** — its shape mirrors the command's Go type and stays
stable. Discover the exact fields by running the command once:

```bash
wtm list --output json | jq '.[] | select(.is_dirty).branch'
wtm checkout 42 --output json | jq '.path'
wtm run ps --output json | jq '.[] | select(.status=="running").name'
```

Non-interactive note: `--output json` never prompts, so destructive commands need an
explicit flag — `clean`/`prune` need `--yes` (or `--force`), and `sync` needs branch
args or `--all`. See each command's `--help`.

## Configuration

All wtm files live under `<git-common-dir>/wtm/` (`.git/wtm/` for a normal clone). Git
never commits anything inside `.git/`, so wtm is invisible to teammates and to
`git status` — worktree usage stays personal.

```
<git-common-dir>/wtm/
├── config.toml                    # project settings
├── run.toml                       # dev jobs + profiles
├── schemas/                       # JSON schemas for editor autocomplete
└── worktrees/<encoded-branch>/
    └── meta.json                  # per-worktree metadata (source branch, timestamp, env strategy)
```

Everything is plain TOML, validated at load time (unknown keys are rejected, not
silently ignored). Edit by hand, or use `wtm config show` / `wtm config edit` and the
`wtm run import` flow.

### Project config — `config.toml`

Generated by `wtm init`, per-clone, never committed.

```toml
[worktrees]
base_path   = "../.trees"   # where worktrees are created (relative to repo root)
base_branch = "main"        # default base for new worktrees

[env]
strategy = "example"        # example | main | parent (see below)

# Each detected value file and its committed template (schema). wtm distinguishes
# templates (.env.example / .dist / .sample / .template / .tmpl, committed) from
# value files (.env, gitignored). .env.local is detected and flagged local but
# stays syncable.
[[env.file]]
target   = ".env"
template = ".env.example"

[[env.file]]
target = ".env.local"
local  = true

[hooks]
on_create = [
  "pnpm install",                                             # string: runs from worktree root
  { cmd = "pnpm db:migrate", cwd = "apps/api" },              # object: runs from a subdir
  { cmd = "pnpm build", cwd = "apps/web", continue_on_error = true },  # non-fatal
]
on_clean = [
  "docker compose down",                                      # runs right before a worktree is removed
]
```

`on_create` hooks run after a worktree is created; `on_clean` hooks run in the worktree
just before it is removed by `clean`/`prune` (e.g. to tear down external resources). A
non-zero hook aborts the operation unless the entry sets `continue_on_error`. Hooks
interpolate `{{worktree}}`, `{{branch}}`, `{{root}}`, and (for `on_create`) `{{from_branch}}`.
A `cwd` is resolved against the worktree root when relative; absolute paths (including
`{{worktree}}`-interpolated ones) are used as-is.

`wtm init` seeds `on_create` from what it detects. A workspace monorepo
(`pnpm-workspace.yaml`, a `workspaces` field in `package.json`, `go.work`, or
turbo/nx/lerna) gets a **single** root install, since that already installs every
package. A repo with no workspace declaration but per-directory lockfiles — say
`front/package-lock.json` next to `api/requirements.txt` — gets one install per
directory instead, each using that directory's own package manager.

### Env strategies

| Strategy | Behavior |
|---|---|
| `example` | Copies `file.example` from the main worktree, renamed to `file`. Warns if `.example` is missing. |
| `main` | Copies the actual file from the main worktree. |
| `parent` | Copies from the source worktree (`--from`), falling back to `main`. |

The strategy is recorded per worktree. Later, [`wtm env`](docs/wtm_env.md) reconciles a
worktree's `.env` against its **template** (the committed schema) plus the **same value
source** — adding missing keys, and (with `--mode refresh`) settling values that drifted.
Override the source for a single run with `--from`; the report always shows which source
was used.

### Run config — `run.toml`

Optional and opt-in — created by `wtm run init` (which detects docker-compose files
and package scripts), not by the global `wtm init`. Declares dev **jobs** and groups
them into **profiles**. Per-clone, never committed — share layouts with
`wtm run export | wtm run import -`.

```toml
[[job]]
name = "docker"
kind = "service"            # long-running; with `stop` it's detached
cmd  = "docker compose up -d"
stop = "docker compose down"

[[job]]
name = "migrate"
kind = "task"               # one-shot; blocks the profile, streams output, non-zero aborts
cmd  = "pnpm migrate"

[[profile]]
name    = "full"
jobs    = ["docker", "migrate"]
default = true
```

Jobs are scoped per worktree at runtime: starting `docker` from worktree A runs it with
`cwd = A`; a separate process runs from worktree B. `wtm run down` only stops the current
worktree's jobs unless you pass `--all`.

### Global config — `~/.config/wtm/config.toml`

Created by `wtm init`, personal to each developer.

```toml
shell = "zsh"          # zsh | bash | fish
agent = "claude-code"  # claude-code | cursor | none
```

The project config can override `agent`; `shell` is always global.

## IDE autocomplete + validation

Every TOML file `wtm init` writes starts with a `#:schema ./schemas/...json` directive.
Pair it with [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml)
(or the bundled JetBrains TOML plugin) for autocomplete, hover docs, and real-time
validation. Schemas ship with the binary and are written locally by `wtm init` — no
internet required. Re-extract them after upgrading:

```bash
wtm schema dump            # <git-common-dir>/wtm/schemas/{run,project}.schema.json
wtm schema dump --global   # ~/.config/wtm/schemas/global.schema.json
```

## Contributing to the docs

The [`docs/`](docs/wtm.md) reference is generated from the Cobra command tree — never
edit it by hand. After changing any command or flag, regenerate:

```bash
make docs
```

## License

MIT
