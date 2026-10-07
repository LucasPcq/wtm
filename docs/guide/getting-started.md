# Getting started

Ten minutes from install to two branches running side by side. The example is a small monorepo called `acme`, with a web app in `apps/web` and an API in `apps/api`, each started by `pnpm dev` and each with a `.env` copied from a committed `.env.example`:

```
acme/
├── apps/api/   package.json, .env.example (PORT=8787)
├── apps/web/   package.json, .env.example (PORT=5173, API_URL=http://localhost:8787)
├── package.json
└── pnpm-workspace.yaml
```

Your repository will differ; the steps do not. The outputs below are what wtm prints on that repository, trimmed to the lines that matter.

## 1. Install

```bash
brew install LucasPcq/tap/wtm
```

Or `go install github.com/LucasPcq/wtm@latest`, or a binary from the [releases](https://github.com/LucasPcq/wtm/releases): see [Installation](installation.md) for a specific version and [Platform support](platform-support.md) for Linux and WSL2. wtm needs `git`; [`gh`](https://cli.github.com) is optional and unlocks the GitHub features.

Then add the shell integration, which is what lets `wtm go` change your directory:

```bash
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc    # ~/.bashrc for bash
exec zsh
```

For fish, add `wtm shell-init | source` to `config.fish`.

## 2. Set up the repository

```bash
cd acme
wtm init
```

The wizard proposes where worktrees go, the base branch, how `.env` files are provisioned and which hooks to run (`pnpm install`, say). Accepting every proposal gives:

```console
$ wtm init --yes

  ✓ Project ready

  Created .git/wtm/config.toml

  base_path     ../.trees
  base_branch   main
  env_strategy  example

  Next steps
    → wtm create <branch>   create a worktree to get started
    → wtm relocate          adopt & align pre-existing worktrees
    → wtm run init          configure per-worktree services
```

The config lives in `.git/wtm/`, so it is never committed and each clone has its own. `wtm config edit` opens it later; [Configuration](configuration.md) covers every key.

## 3. Tell wtm how the app runs

This step is optional: without it wtm manages worktrees and nothing else. With it, every worktree gets its own ports and can run its own dev servers.

```bash
wtm run init
```

The wizard detects the package scripts (and `docker compose` files), turns the ones you keep into jobs, finds the port each one reads in its `.env`, and offers to link the `.env` keys holding those ports so every worktree gets its own. On `acme`:

```console
$ wtm run init --yes --link-env

  ✓ Configured run module → .git/wtm/run.toml   2 added

  ✓ 2 port(s) declared in run.toml

  ✓ 3 .env value(s) now follow a port
```

It wrote two jobs, `api-dev` and `web-dev`, grouped in a default profile `all`. `wtm run list` shows them, and the [`run.toml` reference](run-toml.md) explains the file.

## 4. Create a worktree

```bash
wtm create feat/login
```

The wizard asks the source branch, the env strategy and whether the new worktree is isolated (its own ports and data, the default), then shows a recap to confirm. The result:

```console
$ wtm create feat/login

  ✓ Created worktree feat/login

  from  main
  env   example · 3 ports settled (offset +10)
  path  ../.trees/feat-login

  → wtm go feat/login
```

"3 ports settled" means the `.env` files of the new worktree were copied from their templates and moved to its own ports. The first worktree gets `+10`, the next `+20`:

```console
$ cat ../.trees/feat-login/apps/web/.env
PORT=5183
API_URL=http://api-dev.feat-login.acme.localhost:11080
```

`API_URL` now points at this worktree's own API, under a name the run proxy serves (more on that in step 6).

## 5. Start its dev stack

```bash
wtm go feat/login
wtm run up
```

`run up` first shows the worktrees it can act on, the current one already checked: press Enter. It then starts the default profile and opens the run view, one pane per job with its output live. Press `q` to leave it; the jobs keep running in the background. `-d` starts them and gives the prompt back at once:

```console
$ wtm run up -d

  Profile all

  › [1/2] api-dev
  ✓ api-dev started · http://api-dev.feat-login.acme.localhost:11080

  › [2/2] web-dev
  ✓ web-dev started · http://web-dev.feat-login.acme.localhost:11080

  Where to reach it
    api-dev  http://api-dev.feat-login.acme.localhost:11080
    web-dev  http://web-dev.feat-login.acme.localhost:11080

  → wtm run logs   attach to the output
  → wtm run down   stop the jobs
```

## 6. A second branch, side by side

A bug report comes in while `feat/login` is running. No stash, no stopping anything:

```bash
wtm create fix/typo --yes
wtm run up fix/typo -d
```

The first time a second worktree starts while another one runs, wtm asks what to do about the first one: keep it running (parallel) or stop it (exclusive). It can remember the answer in `run.toml`. Keep it running, and both stacks run at once, each on its own ports and under its own name:

```console
$ wtm run ps

  NAME     KIND     STATUS   ADDRESS                                         UPTIME  WORKTREE
  api-dev  service  running  http://api-dev.feat-login.acme.localhost:11080  3s      feat/login
  web-dev  service  running  http://web-dev.feat-login.acme.localhost:11080  3s      feat/login
  api-dev  service  running  http://api-dev.fix-typo.acme.localhost:11080    2s      fix/typo
  web-dev  service  running  http://web-dev.fix-typo.acme.localhost:11080    2s      fix/typo
```

Open `http://web-dev.fix-typo.acme.localhost:11080` in a browser: each worktree has its own hostname, so the two apps do not share cookies either. The names answer while wtm runs the jobs; `wtm run url --raw` prints the plain `http://localhost:<port>` address instead. On macOS, `wtm run proxy install` serves the names on port 80, which drops the `:11080`. See [Named URLs](addressing.md).

`wtm list` shows every worktree and what it runs; `wtm ui` shows the same thing full screen, with PRs and logs:

```console
$ wtm list

  main        (parent)  ● active                    ✓ clean
  feat/login                        services      ✓ clean
  fix/typo                          services      ✓ clean
```

## 7. Clean up

The fix is merged. Remove its worktree and local branch; its jobs are stopped first:

```console
$ wtm clean fix/typo

  ✓ Stopped services on fix/typo

  ✓ Cleaned worktree and branch fix/typo
```

`clean` asks to confirm, and refuses a worktree with uncommitted or unpushed work unless you pass `--force`. Once several branches are merged, `wtm prune` removes all of their worktrees in one pass (with `gh`).

At the end of the day, stop what still runs:

```bash
wtm run down --all
```

## Next

- Several branches at once: `wtm create feat/a feat/b --yes`, then `wtm exec --all -- pnpm test` runs the tests in every worktree, in parallel, each on its own ports. See [Run a command across worktrees](recipes.md#run-a-command-across-worktrees).
- [Recipes](recipes.md): a turbo monorepo, a docker compose app, a shared postgres, AI agents in parallel, stacked PRs.
- [Troubleshooting](troubleshooting.md): what to do when a port is taken or a job crashes.
- The [user guide](README.md) explains isolation, jobs and profiles, and the proxy in depth.
- [Integrations](integrations.md): driving wtm from a script, an agent or another tool.
- `wtm <command> --help` shows every flag, with examples; the [command reference](../wtm.md) is the same text.
