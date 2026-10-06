# Troubleshooting

Common problems, what causes them, and the command that fixes each one.

- [A port is already in use](#a-port-is-already-in-use)
- [A job is reported crashed](#a-job-is-reported-crashed)
- [The daemon is another version](#the-daemon-is-another-version)
- [A stack started by 0.27 is still running](#a-stack-started-by-027-is-still-running)
- [`wtm go` does not change directory](#wtm-go-does-not-change-directory)
- [No `run.toml` (exit 16)](#no-runtoml-exit-16)
- [A named URL does not answer](#a-named-url-does-not-answer)
- [A `.env` is out of date](#a-env-is-out-of-date)

## A port is already in use

**Symptom.** `wtm run up` reports a job started, then `wtm run ps` shows it `crashed`, and its log ends with `Address already in use` (or `EADDRINUSE`).

**Cause.** Something else listens on the port the worktree was given. Find it:

```bash
wtm run ps                              # another worktree's job?
lsof -nP -iTCP:5183 -sTCP:LISTEN        # any other process
```

**Fix**, depending on what holds it:

- **An app you started by hand** (often on the base port, in the main checkout): stop it, or let wtm run it with `wtm run start --job <name>`.
- **Another worktree's job**: every isolated worktree has its own ports, so this is usually a **verbatim** worktree and its source, which share their ports on purpose. `run up` offers to stop the other one; under `--yes`, pass `--exclusive`. To run both at once, give the worktree its own ports: `wtm env <branch> --isolation isolated`.
- **A port in another project** that happens to fall on one of yours: move this project's ports with `port_offset_block` at the top of `run.toml`, or change the base port with `wtm run job edit <job> --port PORT=<base> --yes`, then `wtm env <branch>` to settle each worktree's `.env`.

**A related warning: "Ports declared but not bound".** Nothing answers on the port wtm gave the job, often because something answers on the base port instead. The command never read its variable: pass it explicitly (`--cmd 'pnpm dev --port ${PORT}'`), check that the app's `.env` does not pin a port, and in a Turborepo let the variable through (`globalPassThroughEnv` in `turbo.json`). `probe = false` on a job silences the check. See [Checking the ports](jobs-and-profiles.md#checking-the-ports).

## A job is reported crashed

**Symptom.** `wtm run ps` lists a job as `crashed`, or `run up` says it exited right after starting.

**Fix.** Read its output. The log is kept after the process is gone:

```bash
wtm run logs --job api                    # the run view, focused on api
wtm run logs feat/login --output json     # the last 1000 lines of each job
```

The raw file is `.git/wtm/logs/<branch>/<job>.log`, cleared each time the job starts. Once the cause is fixed, `wtm run start --job api` starts it again; `wtm run ps --output json` gives the exit code (`exit_code`).

Frequent causes: a port in use (above), dependencies not installed in the new worktree (add `pnpm install` to the `on_create` hooks with `wtm init --only hooks`), or a command that only works from another directory (set the job's `cwd`).

## The daemon is another version

**Symptom.** After an upgrade, a `wtm run` command stops with a message naming two versions: the daemon holding the socket, and this wtm.

**Cause.** Jobs are run by one background daemon shared by every repository, and it outlives the command that started it. A daemon from the previous binary keeps its own behavior until it is replaced. One holding no job is replaced automatically.

**Fix.**

```bash
wtm run daemon status      # which build is running, and what it holds
wtm run daemon restart     # hand the jobs over to this binary's daemon
```

Detached services (the ones with a `stop` command, such as a compose stack) survive the restart; foreground ones are stopped, so start them again with `wtm run up`.

## A stack started by 0.27 is still running

**Symptom.** After upgrading from 0.27, `wtm run up` starts a second copy of a compose stack, or fails on a port a running container holds, while `wtm run ps` shows nothing for it.

**Cause.** Before 0.28, jobs ran without `COMPOSE_PROJECT_NAME`, so compose named each stack after its directory (`feat-login`). wtm now names it `<repo>-<worktree>` (`acme-feat-login`) and does not see the old one.

**Fix.** Stop each old stack once, by its old name:

```bash
docker compose ls                       # find the projects named after a directory
docker compose -p feat-login down       # no -v: the volumes stay
```

The new stack uses new volumes (`acme-feat-login_*`), so it starts empty. A worktree created by 0.27 is also refused by `run up` until you decide its isolation: `wtm env <branch> --isolation isolated` (its own ports and project) or `--isolation verbatim` (keep its source's). See [Migrating to 0.28](migrating-to-0.28.md).

## `wtm go` does not change directory

**Symptom.** `wtm go feat/login` prints `wtm go requires shell integration to change your working directory`, or prints nothing and you stay where you were.

**Cause.** A program cannot change its parent shell's directory; the shell function from `wtm shell-init` does it for it. It is missing from this shell.

**Fix.** Add it to your shell's startup file and open a new shell:

```bash
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc     # ~/.bashrc for bash
```

For fish, add `wtm shell-init | source` to `config.fish`. After an upgrade, open a new shell so the function matches the binary. In a script, where no startup file is read, use `cd "$(wtm resolve feat/login)"` instead.

## No `run.toml` (exit 16)

**Symptom.** `wtm run up` (or `start`, `list`, `url`…) fails with `no run.toml` and exit code `16`.

**Cause.** The `run` module is opt-in, and `run.toml` lives in `.git/wtm/`: it is per clone and never committed. A new clone has none, even when a teammate's does. Every worktree of a clone shares the same file.

**Fix.** Create it from detection, or copy it from a clone that has one:

```bash
wtm run init                                  # detect compose files and scripts

wtm run export > run.json                     # in the clone that has it
wtm run import run.json                       # in this one
```

After an import, `wtm env <branch>` settles each worktree's `.env` on it.

## A named URL does not answer

**Symptom.** `http://api.feat-login.acme.localhost:11080` does not load, while the job looks up.

Go through these in order:

1. **Is the job run by wtm?** Named URLs are served by the run proxy while `wtm run` runs the job. An app started by hand has none: use its port URL, printed by `wtm run url --job api --raw`. `wtm run ps` shows what wtm runs.
2. **Does the proxy listen?** `wtm run proxy status` prints the port it binds and the one URLs carry. When another program holds the port, the names are lost but the jobs still run: set another port in the global config (its path is in the status output) and `wtm run daemon restart`.

   ```toml
   [proxy]
   port = 11090
   ```

3. **Does the client resolve `*.localhost`?** Browsers and curl send `*.localhost` to the loopback; some other HTTP clients and tools do not. Use `wtm run url --raw` for those.
4. **Is the URL still current?** A branch's host is its slug (`feat/login` becomes `feat-login`). `wtm run url feat/login --job api` prints the exact one.

Without the port: on macOS, `wtm run proxy install` serves the names on port 80, then `wtm env <branch>` drops the port from the `.env` values. To use port URLs everywhere, `wtm run addressing ports`. See [Named URLs](addressing.md).

## A `.env` is out of date

**Symptom.** A worktree's `.env` misses a key the template gained, still carries a value from before, or points at the wrong port after `run.toml` changed.

**Fix.** Compare it with its source, then reconcile:

```bash
wtm env feat/login --check                                   # read-only report; exits 18 on drift
wtm env feat/login                                           # add missing keys, settle the ports
wtm env feat/login --mode refresh --on-conflict overwrite --yes   # also overwrite diverging values
wtm env feat/login --prune --yes                             # drop keys no source has any more
```

The values come from the strategy the worktree was created with (`example`, `main` or `parent`); `--from` overrides it for one run. The ports and addresses `run.toml` links are settled on the worktree's own at the same time. `wtm env main` does the same for the main checkout, keeping the addressing its `.env` spells unless `--addressing` says otherwise.
