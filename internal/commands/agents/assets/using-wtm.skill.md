---
name: using-wtm
description: Use this skill whenever the user wants to create, list, or clean git worktrees; extract/move uncommitted changes from one worktree to another (e.g. to split an oversized PR); start, stop, or inspect per-worktree dev jobs (services + tasks); or check out a GitHub pull request into a worktree — even when they don't explicitly say "wtm". Always pass --output json on wtm data commands so you can parse results; never invoke wtm through an interactive picker, and never launch the `wtm ui` dashboard.
---

# Using wtm

wtm manages **git worktrees**, **per-worktree dev jobs** (long-running services +
one-shot tasks, via a background daemon), and **GitHub pull requests**. It's built to
be driven by an LLM: every data command takes `--output json` and prints machine-parseable
results to stdout, while human messages stay on stderr.

This skill is the **driving guide** — the rules and vocabulary you can't get from a
single `--help`. It is deliberately not an exhaustive flag reference, because wtm is
self-documenting:

- **`wtm <cmd> --help`** — the full, always-current flags for any command.
- **`wtm <cmd> --output json`** — run it once to learn a command's exact JSON schema.
  The payload mirrors wtm's Go structs (stable `snake_case` fields); trust what you see
  over any field list you remember.

## Driving rules

1. **Always pass arguments.** Without one, most commands drop into an interactive picker
   you can't navigate. Get the branch / PR number / profile / job name from a prior
   discovery call (below) first.
2. **Never launch a full-screen surface.** Two exist: `wtm ui` (the worktree dashboard) and
   the **run view** that `wtm run up`, `wtm run start --job <service>` and `wtm run logs` open on a
   terminal. Both take the terminal over until someone presses `q`, and there is no way for
   you to read one or get out of it. Treat them exactly like an interactive picker — do not
   run `wtm ui` to "look at" the worktrees; run `wtm list --output json` (or `wtm tree`)
   instead, and pass **`-d`** (or `--output json`) to every `run up` / `run start`. Suggest
   `wtm ui` to the *user* when they want to browse worktrees themselves. wtm defends itself
   here — no view ever opens under `--output json` or without a TTY — but don't rely on that.
3. **Always add `--output json`** on data commands. JSON goes to stdout; human text and
   warnings go to stderr — ignore stderr unless the exit code is non-zero. wtm may also
   print a one-line update notice there (at most once a day, never under `--output json`,
   never in CI or without a TTY); it never touches stdout, so parsing is unaffected.
4. **Trust exit codes.** `0` = success. Beyond generic `1`, wtm returns granular codes
   (table below) so you can branch precisely. On failure, surface the stderr text.
5. **JSON mode requires `--yes` on every mutating command.** JSON is non-interactive, so any
   command that changes state (`create`, `clean`, `prune`, `sync`, `relocate`, `reparent`,
   `extract`, `checkout`) needs `--yes` — it errors otherwise. Two orthogonal axes: **`--yes`**
   resolves confirmations/decisions from flags and safe defaults; **`--force`** only lifts
   safety refusals and does *not* imply `--yes` (`--force` alone is rejected in JSON). Required
   selections must still be passed explicitly: `clean` needs a branch (add `--force` to also
   remove unsafe worktrees); `sync` needs branch args or `--all` (`--yes` won't push — pass
   `--push` — and won't fast-forward parents — pass `--ff-parents`); `extract` needs the source arg, `--files`, and `--to` (`--yes` defaults
   on-conflict to abort — pass `--on-conflict resolve`); `reparent` needs worktrees and `--to`;
   `checkout` needs the PR `<number>`; `relocate` uses `--to` to change base_path; `create`
   needs `--from` when the branch already exists locally (its parent can't be inferred).
   Read-only data commands (`list`, `tree`, `resolve`, `config show`, `run list`/`ps`/`logs`)
   take `--output json` with no `--yes`. Check `--help` when unsure.
   Across the `run` module the machine surface follows one rule: **the shape follows the
   arity, the exit code follows the success.** A command acting on N jobs (`run up`,
   `run down`, `run logs`) gives an array, one acting on a single job (`run start`,
   `run stop`, `run job *`) gives an object — always that shape, whatever branch the
   command took — and any of them exits non-zero when what it attempted failed.
   The worktree axis obeys the same rule: `run up` and `run down` on **one** worktree keep
   the flat array of job results they have always given, `run stop` its single object; on
   **several** all three give one document per worktree,
   `{worktree, path, profile?, aborted?, jobs: [...]}`, in the order you named them. So
   branch on the shape only if you passed several worktrees. `run logs` keeps its flat array
   of lines either way and adds a `worktree` field to each above one. A command
   that got far enough to have per-job results writes its **whole** document and *then*
   exits non-zero (`run up`, `run down`: read the array, the `status: "error"` entries say
   which job); one that failed before that (no such job, daemon refused, config invalid)
   writes **nothing** on stdout and puts the reason on stderr. So: check the exit code,
   and parse stdout only if it is non-empty.
6. **Operations are idempotent — safe to retry.** `create --if-not-exists` no-ops on an
   existing worktree (including one holding the branch outside `base_path`, whose path it
   returns — even the **main** worktree's own path, if that's where the branch is checked
   out; it is still `already_exists: true`, just not a directory under `base_path`); `clean`
   no-ops on an absent one; `run up`/`down`/`stop` re-run cleanly. Note the flag is about
   the **worktree**, not the branch: an existing branch with no worktree is still created.
7. **An existing local branch is not an obstacle.** `create <branch>` and `checkout <number>`
   both check out a same-named local branch as-is, keeping commits that were never pushed —
   they never delete or reset it, so there is no `wtm clean` to run first. The response sets
   `existing_branch: true` and `origin_state`. Only a branch **another worktree already holds**
   is refused (exit `10`) — enter it with `wtm go <branch>` instead. What wtm will not do is
   **guess its parent**: `create` requires `--from` there, while `checkout` takes the PR's
   base branch (a fact, not a guess).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | generic error |
| `2` | bad usage / invalid flags |
| `10` | worktree (or its path) already exists — or the branch is checked out in another worktree |
| `11` | branch not found |
| `12` | config not found — repo not initialized (`wtm init`) |
| `14` | service/job not declared in `run.toml` |
| `15` | `extract`: selected changes conflict with the target worktree |
| `16` | run module not initialized — run `wtm run init` first |
| `17` | `upgrade`: this install cannot be upgraded — built from source, or the binary is not writable |

## Discovery first — get names before you act

| Goal | Command |
|---|---|
| All worktrees (branch, path, PR, services, dirty?) | `wtm list --output json` |
| Worktree **forest** (parent→child + which need sync) | `wtm tree --output json` |
| Open PRs | `gh pr list --json number,title,headRefName,state,isDraft,url` |
| Declared jobs + profiles | `wtm run list --output json` |
| Jobs running right now, every repo (+ `started_at`, `exit_code`, `url`, `work_dir`) | `wtm run ps --output json` |
| Where a job answers in a worktree | `wtm run url [worktree] --output json` |
| What serves the named URLs (bind port, public port, redirection) | `wtm run proxy status --output json` |
| What a `run up` started, with each job's `url` (plus `held` for a runner) | `wtm run up -d --output json` |
| What a job printed | `wtm run logs [worktree] --job <name> --output json` → `[{job, at, text}]` |
| Resolved project config | `wtm config show --output json` |
| A branch's worktree path | `wtm resolve <branch> --output json` |

## Command map

Reach for the right command, then read its `--help` for flags. Non-obvious behavior is
flagged; everything else is what the name implies.

**Worktrees**
- `wtm list` — flat inventory. `wtm tree` — the parent→child forest; use it (not `list`)
  when hierarchy/orchestration order matters. Its `needs_sync` flag is the key signal
  that a node's parent moved past it. Both expose an `origin` object
  (`{ahead, behind, state}`, or `null` when the branch has no origin counterpart)
  describing divergence from `origin/<branch>` — distinct from `commits_ahead`, which
  counts commits vs the **parent/base** branch. `state` is `up-to-date`/`behind`/`ahead`/`diverged`.
- `wtm create <branch> --from <base>` — new worktree + env provisioning + `on_create`
  hooks. `--from` accepts a remote ref (`origin/x`). Add `--if-not-exists` for idempotency.
  Add `--ff` to fast-forward a behind-only `--from` branch to origin first (so the worktree
  starts up to date); a diverged branch is left as-is (no prompt in JSON mode). `extract`
  accepts the same `--ff` for the parent branch of a newly-created target.
  **When `<branch>` already exists locally** it is checked out as-is, and `--from` stops
  being a start-point: it names the **parent to record for `wtm sync`**. That parent
  cannot be inferred (the branch was created outside wtm), so **`--from` is required
  there** — the command errors instead of guessing the base branch, because `sync` and
  `tree` would treat the guess as fact. Ask the user which branch it stacks on if you
  don't know. `--ff` switches subject too, updating `<branch>` itself rather than the
  source. The response adds `existing_branch: true` and `origin_state`
  (`up-to-date`/`behind`/`ahead`/`diverged`) so you can tell reuse from creation.
- `wtm clean <branch>` / `wtm prune [filters]` — remove one / batch-remove finished
  worktrees. **`clean` also gives back the tenants that worktree carved out of shared
  services** (it drops its database): pass `--keep-data` to withhold that, including under
  `--yes`; the interactive recap names each database it will drop. Only worktrees that
  actually started the shared job owe anything — one created and thrown away owes nothing.
  If the shared service is down the drop is deferred, and the next `wtm prune` settles it
  once the service is up again. **In JSON mode surviving children are left orphaned unless you pass
  `--reparent-children`** (they reparent onto the grandparent). `prune` decides "finished"
  from **GitHub PR state via the `gh` CLI** (not local commits): `--merged` = PR merged,
  `--closed` = PR closed without merging, `--gone` = remote branch deleted; no filter = all
  three. **`--merged`/`--closed` need `gh` installed + authenticated** — without it they match
  nothing and prune prints a notice on stderr; only `--gone` works offline. Prune `reason`
  values in JSON are `pr_merged` / `pr_closed` / `gone` (there is no plain `merged`). Both
  **refuse unsafe worktrees** — dirty, unpushed commits, or an open PR — unless you pass
  `--force`; in JSON/`--yes` mode those are reported under `skipped` (reason `dirty`/
  `unpushed`/`open_pr`) instead of being removed, so committed work is never silently lost.
  One exception for `prune`: when **every** match is unsafe, nothing survives the
  selection and the JSON is empty — `pruned: []` and `skipped: []` both. Read an empty
  result as "nothing was removed", not as "nothing matched".
  Both also run any configured **`on_clean`** hooks in the worktree just before removing it
  (e.g. `docker compose down`); a hook that exits non-zero **aborts the removal** unless its
  entry sets `continue_on_error`. For `prune`, every selected worktree is hooked before the
  first one is removed, so a hook failing partway aborts with nothing deleted — but the
  worktrees already hooked have had their teardown run, which is why `on_clean` hooks must
  be idempotent. If `git worktree remove` then fails on undeletable files
  (e.g. root-owned Docker files), interactive runs offer a `sudo rm -rf` fallback — this
  never triggers in JSON/`--yes` mode, where the failure is surfaced as an error.
- `wtm extract <source> --files <a,b> --to <branch>` — move part of the `<source>`
  worktree's uncommitted changes onto another branch (split an oversized PR). In JSON mode pass
  `--yes` plus the source arg, `--files`, and `--to` — all **required** (omitting any errors
  naming the missing flag; there is no picker). On conflict it changes nothing and exits `15`;
  retry with `--on-conflict resolve` to apply git conflict markers (`--yes` defaults on-conflict
  to abort). **When `--to` names a branch that already exists locally**, the same rule as
  `create` applies: its parent can't be inferred, so `--from` is **required** there too
  (the command errors naming the flag rather than guessing).
  When `--to` creates a worktree, its `.env` port values are settled onto the ports that
  worktree binds, exactly as `create` does — so its services do not collide with the main
  checkout's. Under `--yes` this applies without asking.
  `--files` takes paths exactly as reported (spaces and non-ASCII are never quoted or escaped),
  including each untracked file of a brand-new directory individually; pass a directory
  (`--files newmod/`) to take everything below it. Gitignored files are never listed — `.env`
  drift is `wtm env`'s job. A file entry may report `"status": "renamed"` with an extra
  `"orig_path"`: both paths move together, and `--files` names the new one.
- `wtm env [worktree]` — detect and fix a worktree's `.env` drift: reconcile it against its
  committed **template** (the expected keys, read from the worktree itself) plus a single
  **value source** chosen by the worktree's recorded strategy — never a silent mix:
  `example` → template placeholders only; `main` → the main worktree; `parent` → the parent
  worktree **only** (a key the parent lacks stays `missing_unresolved`, it is NOT pulled from
  main). The one exception (mirroring `wtm create`): when there is no readable parent file at
  all — the parent has no worktree, or that file isn't in it — it falls back to main for that
  file, flagged `parent_fallback:true` (with `parent_branch` naming the parent). The
  report/JSON `source` field names the source. `--from example|main|parent` is the only way to
  pull a different source for a run. If there is no `.env` to sync from at all yet (fresh
  project: templates detected by `init`, but no value files created), every expected key is
  `missing_unresolved` and `source` reads `template (no .env to sync from)` — filling them
  (interactively) scaffolds the `.env` from the template. A configured file that exists
  **nowhere** — no value anywhere, no template either — is flagged `unresolvable:true` and
  never counts as clean: it names a path the repository does not have, and the fix is in
  `config.toml`, not in any worktree. `--mode add` (default)
  only fills missing keys and never touches an existing value; `--mode refresh` also settles
  values that diverge from the source. `--check` is a read-only drift report (writes nothing).
  In JSON/`--yes` mode it is **report-only except safe additions**: keys missing from the child
  but resolved from a real source are added; **conflicts** stay unless you pass
  `--on-conflict overwrite` (default `keep`), **orphans** stay unless you pass `--prune`, and
  keys with no real source value (`missing_unresolved`) are never auto-filled (they need the
  interactive prompt). JSON requires `--yes` **except with `--check`** (read-only, never
  prompts, so `wtm env <wt> --check --output json` works on its own); omit a worktree arg only
  interactively (else it errors — there is no picker under `--yes`/JSON). JSON shape:
  `{branch,mode,check,files:[{target,strategy,source,applied,parent_branch,parent_fallback,diff:{mode,
  entries:[{key,status,current_value,resolved_value,placeholder,source,export}]}}],ports:{offset,
  applied,entries:[{file,key,port,base,resolved,status,current_value,new_value}]}}` where
  `status` is `resolved` / `missing_unresolved` / `conflict` / `orphan`. Round-trip is
  preserved: comments, ordering and formatting of the `.env` are kept; only decided keys change.
  The `ports` block is the `[[env_port]]` pass (below), empty when the project declares none;
  `ports.applied` says whether those rewrites were written, and the trailing summary counts
  them alongside the files (a run that only shifted a port still reports what it wrote).
- `wtm ui` — the full-screen dashboard (worktree state, divergence, PRs, plus create and
  delete). **Never invoke it** (see driving rule 2): it holds the terminal until a human
  quits it. Everything it shows is available to you as JSON via `wtm list` / `wtm tree`, and
  everything it does via `wtm create` / `wtm clean`.
- `wtm relocate` — realign worktrees with `base_path` and adopt externally-created ones.
  `--to <path>` sets a new `base_path` non-interactively; the interactive wizard also lets
  the user change it. You can't drive the wizard — in JSON mode pass `--yes` (and `--to` to
  change base_path).

**Stacked branches**
- `wtm fast-forward <branch…>` (alias `ff`) / `wtm fast-forward --all` — advance the
  selected worktrees to `origin/<branch>` and **nothing else**: no rebase onto the parent,
  no merge. Use it to pull down work pushed to a branch you already agree with; use `sync`
  when the branch has to be replayed onto its parent. With neither branch args nor `--all`
  it refuses naming `--all` rather than opening a picker (same rule as `sync` and `prune`),
  so always pass `--output json --yes` (driving rule 5). `--output json` requires `--yes`.
  Two refusals, only one of them liftable: a **diverged** branch is refused and `--force`
  does **not** lift it (advancing it would drop local commits — run `wtm sync` instead); a
  worktree with **uncommitted changes** is refused and `--force` does lift it, though git
  still refuses if a modified file would be overwritten. A run over several branches keeps
  going past the ones it could not move and reports every one.
  JSON: an array of `{branch, status, old_tip, new_tip, behind, detail?}`, where `status`
  is `already up to date`, `fast-forwarded from origin`, `diverged`, `no origin counterpart`,
  or `failed` (`detail` says why). A run in which any branch is `failed` exits non-zero;
  `diverged` and `no origin counterpart` are reported facts, not failures.
- `wtm sync <branch…>` / `wtm sync --all` — rebase the selected worktrees onto their
  recorded parent, in cascade (parents before children), fetching first. A conflict aborts
  that branch's rebase (its descendants are skipped) unless `--keep-conflict` leaves it in
  progress. Local only — in JSON mode pass `--yes` (and `--push` to force-push with lease).
  Without a TTY, in plain output, with neither `--yes` nor `--dry-run`, it refuses naming
  `--yes` instead of trying to open a picker (same rule as `prune`) — another reason to
  always pass `--output json --yes` (driving rule 5) rather than relying on a terminal.
  A parent **no step covers** — a branch with no worktree, or one left out of the
  selection — is not refreshed by the cascade. Every run reports what it found about
  those parents in `parent_updates`: `{branch, status, old_tip, new_tip, behind,
  children, detail}`, where `behind` counts the commits the local ref lacks and
  `children` names the worktrees rebased onto it. `status` is one of `behind` (left
  as is — re-run with `--ff-parents` to refresh it), `fast_forwarded`, `diverged`
  (no fast-forward exists; reconcile it by hand, the flag will never move it), or
  `ff_failed` (the refresh was asked for and could not happen — `detail` says why,
  e.g. a dirty parent worktree; passing the flag again will not help). Pass
  `--ff-parents` to refresh them first, `--no-ff-parents` to never. `--dry-run`
  reports them without any network call and never refreshes, so `--ff-parents` is a
  no-op there.
  The **base is only involved when it is actually a target**: a step rebases onto it,
  the selection names it, or the run covers everything (`--all`, which includes the
  root). Otherwise it is neither fetched nor fast-forwarded and `base_targeted` is
  `false` — an explicit selection whose every worktree hangs off another parent
  leaves the base completely alone.
- `wtm reparent <branch…> --to <parent>` — change the recorded parent of one or more
  worktrees to the same new parent (metadata only; the rebase happens on the next `sync`).
  Use after a middle branch merges. In JSON mode pass `--yes` with the worktrees and `--to`.
  JSON: `{"reparented":[{branch,old_parent,new_parent},…]}`.

**Navigate**
- `wtm go` needs the user's **shell integration** to `cd`, so you can't drive it. To get a
  path yourself, use `wtm resolve <branch> --output json` → `{path, branch}`. You rarely
  need to move at all: every `run` command takes the worktree as its first argument.

**Dev jobs (`wtm run`)** — jobs live in a per-clone `run.toml` (wtm-managed; never edit it
directly). Each is a `service` (long-running) or `task` (one-shot, blocks the profile,
non-zero exit aborts it); profiles are named, ordered job groups. The module is **opt-in**
and **experimental**: the global `wtm init` does not configure it.
- `run init` sets up `run.toml` from detection (docker-compose + package scripts). It is
  the only entry point that works before the module exists; every other run command exits
  `16` (run module not initialized) until at least one job/profile is declared. Non-TTY it
  auto-generates and **removes nothing**. `run job add` / `run profile add` also work before
  init (they create the first job).
- **A job may be shared across worktrees.** `scope = "shared"` in `run.toml` makes it run
  **once for the whole repository**, in the main checkout, instead of once per worktree — a
  postgres, a keycloak. Consequences you must expect: it takes **no port offset** (its
  declared port is the port it binds, in every worktree), its published URL carries **no
  worktree segment** (`db.projet.localhost`, not `db.feat-x.projet.localhost`), and its logs
  are the same stream whichever worktree you read them from. In `run ps` / `--output json`
  the worktrees holding it report status **`attached`** with `pid: 0`: that is a claim on the
  one running instance, not a second process — never count one service per worktree from it.
  `run stop` in a worktree releases only that worktree's claim; the service itself stops when
  the last one goes.
- **A shared job may carve out a tenant per worktree.** `[job.tenant]` names it (`name`,
  `attach`, `detach`, `env`) so each worktree keeps its own data — a database, a set of
  keycloak realms. wtm runs the declared commands and knows nothing else about them; they get
  the worktree's whole environment plus `$WTM_TENANT`, `$WTM_WORKTREE`, `$WTM_ORDINAL`.
  Configuration values use `{worktree}` / `{ordinal}`; commands use the `$WTM_*` variables.
  A shared job with **no** `[job.tenant]` is valid and means one instance with one set of data.
- **Re-running `run init` is symmetric.** Every step is pre-filled from the existing
  `run.toml`: what stays checked is kept, and what you uncheck is **removed** along with the
  profile entries and `[[env_port]]` links naming it — a profile left with no job goes too.
  Only jobs the wizard itself proposed can be removed: one added with `run job add` appears
  in no detected list, so it is never touched. The same symmetry holds for the URLs step (a
  job you unpublish stays unpublished) and the profiles step (deleting them all keeps them
  deleted). This only applies to interactive runs — `--non-interactive` never removes.
- **`run init` composes a startable configuration.** It proposes every compose file and
  package script but checks only scripts whose name contains `dev` — and not a root `dev`
  a workspace package also declares, which is an orchestrator (`turbo run dev`) that would
  double-start those packages. **Nothing unchecked becomes a job.** It then asks to review
  the detected ports, and to compose the **profiles** `run up` offers: one per package plus
  one gathering everything, editable (rename / merge / remove / new). Root-cwd jobs join
  every profile, tasks are ordered ahead of the services that depend on them, and past
  six packages only the `all` profile is proposed rather than a list to scroll. It then
  asks **which jobs answer under their own name** — every service declaring the port it
  listens on (`PORT`, or `<JOB>_PORT`) is proposed checked, and unchecking one withdraws
  the `url` it already had. A non-interactive run publishes the same set without asking.
  A port a job merely dials (`DB_PORT`, `REDIS_PORT`) is never proposed. Two
  packages whose directories end in the same name (`apps/a/back`, `apps/b/back`) get
  distinct profile names instead of being merged. Non-interactively it takes the same
  answers without asking. A checked
  script outside the `dev` ones gets its `kind` asked, because a task blocks its profile
  until it exits. **A service with no detected port is asked about too** — declaring one is
  what keeps a second worktree from binding the same port — and a job whose command never
  mentions the variable wtm injects gets that command offered for editing. The wizard ends
  on a **review step**: choosing "No, cancel" there aborts the run and writes nothing.
- `run init --link-env` also writes the `[[env_port]]` links without asking (see port
  isolation below).
- **`run init --write-port-keys` materializes a declared port as a `.env` key.** For every
  job whose port no `.env` carries, it writes `KEY=<base>` into the `.env` of the job's own
  `cwd` **and** into that file's committed template, adds the `[[env_port]]` link, and — when
  the project does not provision that file — adds the `[env]` target to `config.toml`. This
  is the route that isolates a job **both** under `wtm run` and when a developer starts it
  themselves, provided the project's own config reads the key (`server.port:
  Number(process.env.VITE_PORT)`); wtm writes the key and never touches project code. Like
  `--patch-compose` it writes tracked files, so it is never inferred: non-interactively it
  takes the flag, interactively it is the wizard's route step. In the wizard, each service
  declaring a port is asked **where it reads it** — its `.env` (pre-filled) or its command —
  and only the jobs left on the command route are offered command editing.
- `run init` also **pre-fills the ports** of the compose files it picks up. A mapping
  already reading a variable (`"${DB_PORT:-5432}:5432"`) is declared as-is. A literal one
  (`"5432:5432"`) binds the same port in every worktree, so it is **not** declared: wtm
  reports it with the line to write and the `run job edit --port` that follows. Pass
  **`--patch-compose`** to have wtm rewrite those mappings itself — it edits only the port
  value in place (comments and formatting survive) and the `:-default` keeps
  `docker compose up` working without wtm. Non-interactively no project file is ever
  touched without that flag. Re-running `run init` backfills the ports of a compose job
  that predates them, without overwriting a port already declared.
- `run init` also finds the **absolute names** a compose file pins: a service's
  `container_name`, or a top-level volume's or network's explicit `name`. These are
  resolved by the Docker daemon, not by the compose project, so `COMPOSE_PROJECT_NAME`
  never reaches them and a second worktree collides — Docker refuses a duplicate
  `container_name` outright, and a pinned volume or network is silently shared. wtm
  reports them and, under the same **`--patch-compose`**, fronts each with the project
  (`container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"`); the default reproduces
  the name the file used to pin, so `docker compose up` alone is unchanged. Ports and
  names are **one confirmation**, not two: accepting half still leaves two worktrees
  unable to run at once. A name under `external: true` or one already reading a variable
  is left alone. A volume that pinned its `name` gains one per worktree, **each starting
  empty** — the data already written stays under the old name; say so before patching.
- `run init` also pre-fills the ports of **dev server jobs** from the env files next to
  their `package.json`: a `PORT` (or `*_PORT`) entry with a numeric value, read from
  `.env.local`, else `.env`, else a committed `.env.example`. A job is matched to a
  directory by its `cwd`, so in a pnpm monorepo each package takes its own port and never
  inherits the root's. Only `kind = "service"` jobs from package scripts are considered.
  Nothing is written to the `.env` without `--write-port-keys` (above) and no command is
  ever rewritten — wtm declares the port and prints "Ports detected from .env" naming the
  source file. **Whether the command actually
  reads the variable is not checked**: `next dev` and most node servers read `PORT` from
  the environment, but a CLI that only takes a flag (vite) needs
  `--cmd 'pnpm dev --port ${PORT}'`. Never assume a declared port means an isolated one.
  This detection feeds the `[[env_port]]` pass below: the port it declares is the base
  those links then follow, so a `.env` holding `PORT=5173` and
  `VITE_API_URL=http://localhost:5173/api` ends up with **both** keys shifted per worktree.
  A key may follow **several** ports: a `CORS_ORIGIN=http://localhost:5173,http://localhost:5174`
  gets one link per front-end and every origin moves onto the port its own job binds. A port
  no job declares — an external origin — is left exactly as it was.
- What `run init` refuses to declare, and reports instead: port ranges, mappings with no
  host port, a `ports:` list carrying a YAML anchor or alias, `${VAR}` with no default,
  and a variable two services give two different defaults. It also **withdraws** a
  detected port when two bases differ by a multiple of the block — it would make
  `run.toml` unloadable — and names both sides. Compose and `.env` ports are arbitrated
  together, so either can be the one withdrawn; a base already written by hand always
  outranks a detected one. So a "Ports withdrawn" or "Ports left alone" section in the
  output is expected behavior, not a failure: read the fix it prints (a compose line, then
  a `run job edit --port` command).
- **Every `run` command takes the worktree as its first argument**, like the rest of the
  CLI: `run up [worktree]`, `run logs [worktree]`, `run start [worktree]`. Omit it and the
  current directory is used — which is what you want when you are working inside one
  worktree, and it never opens a picker on your paths (no TTY, or `--output json`). Name it
  to act on another worktree of the same repo without moving. The job or profile is a
  **flag**: `--job <name>`, `--profile <name>`.
- **`run up`, `run down`, `run stop` and `run logs` take several worktrees**:
  `run up feat-a feat-b -d`. They start concurrently and independently — one that aborts
  leaves the others running, and the command exits non-zero if any did. `run start` stays
  single-worktree. Above one worktree the JSON changes shape (see the arity rule below) and
  every human line names the worktree it came from.
- `run up [worktree] --profile <name>` / `run down [worktree]` — start / stop a profile.
  On `run up` **`--profile` is repeatable**: `--profile front --profile back` starts the
  union of both, in the order given, and a job several of them list starts once.
  `run down --profile` still takes one.
  `run start [worktree] --job <name>` / `run stop [worktree] --job <name>` — one job.
  `--job` is **required** on `start`/`stop` on your paths: without a terminal there is no
  picker to fall back on, and the command errors naming the flag. A failing job aborts the rest and exits non-zero, leaving started
  services up (fix and re-run). `run up` starts **every job the profile lists**, tasks
  included, in the listed order; with no profile declared at all it starts every declared
  job in declared order. The step counter (`[2/5]`) covers exactly those jobs, so a count
  smaller than the profile means the profile itself is short, never that wtm dropped
  something.
- **Another worktree already running jobs is not a conflict.** Each worktree has its own
  ports and resource names, so stacks cohabit; the question is about machine load, not
  about ports. `run up` asks about it once, and only on a terminal; on your paths (no TTY,
  `--output json`, or `--yes`) it resolves to leaving the others running and stops nothing.
  Force either way for one run with `--exclusive` (stop them first) or `--parallel`; the two
  are mutually exclusive. A project can settle it for good with
  `concurrency = "parallel" | "exclusive"` in `run.toml`, which is what the question's
  "always" answers write.
- **`--exclusive` and several worktrees contradict each other**, since it stops all but
  one: `run up a b --exclusive` errors. Where the project settled on `exclusive` and the run
  starts several anyway, your paths take the safe default — everything starts, nothing else
  is stopped — and a notice says the setting was set aside.
- **`--yes` is on every `run` command**, `run job`/`run profile`/`run list`/`run open`
  included (`run url` alone has none — it asks nothing), and is the confirmation axis: it runs unattended,
  never opens a picker, and resolves each question to its documented safe default. Where
  there is no safe default it errors naming the flag — `run start --yes` and `run stop
  --yes` require `--job`. There is no `--force` in the run module: nothing here refuses for
  safety, so there is nothing to lift — except `run job rm --force`, which lets a job go
  along with the profile references that name it.
- `run up` and `run start --job <service>` **attach by default**: on a terminal they open the
  full-screen run view. Always pass **`-d`** (or `--output json`, which never opens it) —
  `-d` starts the jobs and returns immediately, which is the behaviour you want. A `task`
  runs inline and blocks until it exits whatever you pass, so `run start --job <task>` needs no
  `-d`.
- `run url [worktree] --job <name>` writes a job's URL on stdout and nothing else, so it
  composes: `curl "$(wtm run url --job web)/health"`. **A job only has a URL if `run.toml` declares one**
  (`url = { port = "PORT" }` on that job, naming one of its own declared ports) — `wtm run
  init` declares it for the services it detects, and `run job add --url-port` for the rest.
  `run open --output json` writes the address it opened, in the same one-element array
  shape `run url --output json --job <name>` gives you.
  `--output json` lists every published job as `[{job, url}]` and never picks for you.
  In text mode, one published job needs no `--job`; **several and no `--job` is an error,
  never a picker** — name the job. `run url` never opens a picker on either axis, so it is
  always safe inside `$(…)`. **Backing out of a prompt exits non-zero** on every mutating
  `run` command (`up`, `down`, `start`, `stop`, `job add|edit|rm`, `profile add|edit|rm`,
  `open`) — an aborted run did not do what was asked. The two listings, `run list` and
  `run job|profile list`, exit 0 instead: nothing was asked for.
  `run open [worktree] --job <name>` opens the same URL in a
  browser; it may offer a picker, but only in a fully interactive run, so **always name
  the job**.
- **The URL is a name, not a port.** With the proxy on (the default), a published job
  answers at `http://<job>.<worktree>.<repo>.localhost:11080` — that order on purpose, so a
  cookie set on `.<worktree>.<repo>.localhost` stays inside that worktree. **That URL may
  carry no port at all**: `wtm run proxy install` redirects port 80 to the proxy, after
  which `run url` prints `http://<job>.<worktree>.<repo>.localhost`. Never assume a `:port`
  suffix is present — read the whole line `run url` gives you. The proxy runs
  inside the background daemon and dies with it. **`--raw` prints `http://localhost:<port>`
  instead** — no proxy has to be up, and every OS resolves it, so **prefer `--raw` for
  anything you dial yourself** (curl, a health check, a test runner). Two limits worth
  knowing: only HTTP jobs get a name (postgres and redis stay on their ports, by design),
  and outside a browser `*.localhost` is not guaranteed to resolve on Linux — one more
  reason `--raw` is the agent's form.
- `run logs [worktree] --job <name>` opens that same view on a terminal. Without one it writes every running
  job's output as `[job] line` on stdout and only ends when the jobs do — do not call it
  expecting it to return. **`--output json` is the agent's form**: it replays each job's
  last **1000** lines as `[{job, at, text}]` (`at` is RFC3339 UTC) and **never attaches**,
  so it returns even on a job that is still running. Entries are **grouped by job** and
  chronological within a job, so `at` goes backwards where one job ends and the next
  begins — never read the array as a single merged timeline. `--job` narrows it without
  changing its shape. What it replays is the job's **last start only**: starting a job
  clears its log, so one file is one run and a tail can never reach back into a previous
  one — including the log of a crash you just restarted past. The file itself is
  `<git-common-dir>/wtm/logs/<url-escaped-branch>/<url-escaped-job>.log` (rotated 5 MB x 3
  *within* a run) if you need more than the last 1000 lines.
- **It covers the jobs that live or have run in that worktree, not everything `run.toml`
  declares.** A job the worktree has never started has no log and no entries, and is left
  out entirely rather than replayed as an empty group — so an absent job means "never
  started here", not "started and silent". Its log file is created the moment it starts,
  so a job that ran and printed nothing *is* present, with no entries. Do not read the
  array as a roster of the project's jobs: `wtm run job list --output json` is that.
- **`status` has four values, and `detached` is not a weaker `running`.** A service with
  a `stop` command (a `docker compose up -d`) is reported `detached` from the moment its
  launcher exits: the real work runs outside wtm, nothing about it was verified, and
  there is **nothing to attach to** — `run logs` on it prints its persisted file and
  returns. It is up: it counts as a running job for `run down`, and `wtm run up` on it
  simply relaunches the launcher rather than refusing "already running". `running` is a
  foreground service the daemon holds a terminal for, `crashed` one whose process died,
  `stopped` one that was stopped.
- **`run ps` is the one global listing**, and the only `run` command that works from
  anywhere: it lists what the daemon holds across every repository, so it needs neither a
  run-initialized repo nor a worktree. It only ever lists — to act on those jobs, open the
  run view with `run logs`, which covers as many worktrees as you name.
- **The daemon survives nothing, and that is by design.** It exits ~30 s after the last
  *foreground* job, and detached services keep running without it. It records what it
  started in `~/.config/wtm/jobs.json`, so the next daemon picks those back up: after a
  reboot, `wtm run ps` still lists the detached stacks and `wtm run down` still stops
  them. `run down`, `clean` and `prune` start a daemon by themselves when that index
  holds something for the worktree they act on.
- `run daemon status` reports whether a daemon is up, its build, its PID and what it
  holds (`--output json` gives one object). `run daemon stop` ends it — detached services
  keep running — and `run daemon restart` hands its jobs to a daemon built from the
  current binary. Both only prompt when foreground services would be stopped; pass
  `--yes` (required without a terminal, and in JSON).
- **A version mismatch is refused, never worked around.** The daemon is what runs the
  jobs, so one built from another version of wtm keeps applying its own behavior. Every
  command refuses with a message naming both versions; the way out is
  `wtm run daemon restart`. Do not retry the command — it will refuse identically.
- `run proxy status` reports what actually serves those names: the proxy's bind port, the
  public port announced in URLs, and whether the port-80 redirection is installed.
  `--output json` gives the whole thing as one object. `run proxy install` needs no
  privilege — launchd binds port 80 and hands the socket to wtm — but it installs a
  LaunchAgent in the user's home, so **do not run it on your own initiative**: propose it,
  and let the user decide. Without a terminal it refuses unless you pass `--yes`.
- `run init` accepts `--yes` (its older `--non-interactive` still works) — the run module's
  own bootstrap, which writes run.toml and may rewrite compose files and .env.
- `run url --job <name> --output json` returns **that job alone**, not the whole array.
- `run export` / `run import` — share a layout as JSON. **`run import` replaces the whole
  `run.toml`**: jobs, profiles, `[[env_port]]` links and project settings alike, so what
  the file held is lost. **It always needs `--yes` on your paths**: without a terminal to
  confirm on — a piped payload included — it refuses rather than replacing silently, and
  `--output json` requires `--yes` too. Nothing is reconciled afterwards —
  tell the user to run `wtm env` if the `.env` values must follow.
- **`run job rm <name> --force`** takes the job out along with everything that named it:
  the profiles that started it, the `[[env_port]]` links that followed its ports, and the
  `runs` list of any job that started it itself. Each is reported on its own line, so you
  can tell the user what went with it.
- `run job` and `run profile` are fully agent-drivable with flags — `add`, `rm`, `edit`
  and `list` alike — and they take **`--yes`** like every other mutating command. Passing a
  flag pre-fills the matching question; **`--yes` is what skips the questions**, so drive
  them as `wtm run job add web --cmd '…' --yes`. Without a TTY (your usual case) they are
  already unattended and `--yes` changes nothing, but pass it anyway: it is the one axis
  that is true on every surface. Under `--yes`, a field with no flag and no safe default
  errors naming it — `run job add --yes` needs the name argument and `--cmd`,
  `run profile add --yes` needs the name argument and `--jobs`.
- `run job edit <name>` **patches**: a flag left out keeps that field, so
  `run job edit api --cmd '…'` changes the command alone and leaves kind, stop, cwd,
  ports and url intact (the job also keeps its position in the file). An explicit empty
  string clears — `--stop ''` drops the stop command, `--cwd ''` falls back to the
  project root, `--url-port ''` withdraws the published name. `--port NAME=PORT` merges
  into the declared ports (repeatable, so one entry changes without rewriting the
  others) and `--port-clear` empties the table. `--name` renames and rewrites what names the
  job elsewhere in the file — the profiles that start it and the `[[env_port]]` links
  that follow its ports. With no such flag it opens the form, so **always pass
  at least one flag**; under `--yes` or without a TTY it errors naming the flags it
  could have taken, and a missing job argument errors rather than opening a picker.
  The form never touches `runs`, `binds_no_port` or `probe` — the first two are
  flag-only, the third has no flag at all — and all three are kept as they are.
- `run profile edit <name>` patches the same way: `--name` renames, `--jobs` replaces
  the list (its order is the start order, so give it in full), `--default` /
  `--default=false` hands the default over or takes it away. Same rules as above: a flag
  left out keeps the field, no flag opens the form, and `--yes` or no TTY means an error
  rather than a picker.
- Every job — and every `on_create` / `on_clean` hook — runs with the worktree's identity
  in its environment, so parallel worktrees do not fight over the same resources:
  `WTM_BRANCH` (the branch verbatim), `WTM_WORKTREE` (its slug, safe as a Docker project
  or network name), `WTM_ORDINAL` (`0` for the main checkout, then the smallest free
  number, stable for the worktree's life), `WTM_PORT_OFFSET` (`WTM_ORDINAL` times the
  `port_offset_block` of run.toml, 10 by default), and `COMPOSE_PROJECT_NAME`
  (= `<repo>-<WTM_WORKTREE>`, left alone if the environment already sets it).
  `COMPOSE_PROJECT_NAME` is **also written into the `.env` of the directory each compose
  job runs from**, at `create` and at `wtm env`, because compose interpolates that file:
  a `docker compose up` typed by hand in a worktree therefore gets its own project, its own
  containers and its own volumes, with no wtm process involved. It is a wtm-owned key —
  the reconciliation never reports it as drift or as a conflict, whatever main holds.
  Docker isolation is automatic for everything compose names itself; a `container_name` or a
  volume/network pinned by `name` escapes it — see `run init --patch-compose` above.
- **A service can say it binds nothing, and which jobs it runs.** Two declarative
  keys on a `[[job]]`, both editable with `run job edit`:
  - `binds_no_port = true` (`--binds-no-port`) — this service listens on nothing by
    design: a build in watch mode, a worker, a runner whose children hold the ports.
    Without it wtm reads the silence as an oversight and keeps naming the job under
    "These jobs will bind the same port in every worktree".
  - `runs = ["web-dev", "api-dev"]` (`--runs`, repeatable, replaces the list) — the
    declared jobs this one starts itself, typically a root `turbo run dev` behind a
    filter. **wtm infers nothing from the command**: the relation is written, never
    detected. It **nests and fans out**: a runner may name another runner (`dev` runs
    `dev:shop`, which runs the shop apps — what a root holds is read through the whole
    chain), and two runners may name the same job (`dev` and `dev:shop` both starting
    `shop-web`). The only refusal is a cycle. It gives the runner its children's ports at start time (a variable name
    two children declare differently is left unresolved, as with the lifecycle hooks),
    lets the port check look at those ports, and makes `run up` **refuse** a profile
    asking for a runner and one of its own children — the same process twice on the
    same port. A job that runs others is not itself reported as portless.
    **A runner also publishes its children's names.** Starting it registers one proxy
    route per published job it runs, so `http://web.<worktree>.<repo>.localhost` answers
    while the only job the daemon holds is the runner. Those addresses are reported on
    the runner, never on the children: `run up --output json` carries them as
    `held: [{job, url}]` beside the runner's own `url`, and the human surfaces list them
    under its line. A child of a running runner has no row of its own in `wtm ui` — it
    is a subprocess, and its address is on the parent.
- **The addressing mode decides what a `.env` value pointing at another job holds.**
  `addressing` in run.toml, `"names"` (the default when absent) or `"ports"`, asked by
  `run init` whenever any job publishes a url. It is the one setting with a consequence
  outside wtm: **named urls are served by the run proxy, which lives in the run daemon**,
  so a value like `VITE_API_URL=http://api.feat-x.repo.localhost:11080` answers while
  `wtm run` is running that job and **not** when the developer starts it themselves —
  the routing table is a projection of what the daemon started. A project whose author
  launches dev servers by hand wants `"ports"`. When the proxy is disabled on the machine
  (`[proxy] port = 0`), wtm writes ports whatever the mode says, and reports it. Only the
  values of jobs that publish a url are affected: a `DATABASE_URL` or a bare `*_PORT`
  stays a port either way.
- **`run up` verifies the ports.** Declaring a port injects a variable; nothing forces
  the command to read it. After the jobs start, wtm dials each declared port and reports
  the silent ones under "Ports declared but not bound". When the *base* port answers
  instead, the variable never reached the process — a `--port`-only CLI, a hard-coded
  port, a `.env` that wins, or a task runner filtering env (**Turborepo's default
  `envMode: "strict"` does exactly this**: a root `turbo run dev` job needs
  `globalPassThroughEnv` in `turbo.json`). wtm never edits third-party config; report the
  finding and tell the user what to change. The check never fails the run and never
  changes the exit code — do not treat it as an error. `--no-probe` skips it;
  `port_probe_timeout` in run.toml sets the budget (default 15s, negative disables), and
  `probe = false` on a `[[job]]` silences it for that job alone. When the base port turns
  out to be held by another worktree — the main checkout running alongside — the report
  names that worktree instead of blaming the job's command.
- **Port isolation is declarative.** A job declares the ports it binds on the main
  checkout, and wtm injects `base + WTM_PORT_OFFSET` under that name — so the command
  needs no arithmetic of its own:
  `wtm run job add web --cmd "pnpm dev" --port PORT=3000` (repeat `--port` per variable);
  add `--url-port PORT` (and optionally `--url-host api.app-1`) to publish it under a name.
  The main checkout gets `PORT=3000`, the next worktree `PORT=3010`. For Docker, template
  the host side in `docker-compose.yml` (`"${DB_PORT}:5432"`) and declare
  `--port DB_PORT=5432`: the container port never moves, only the binding. `wtm run init
  --patch-compose` does both steps for you. A declaration
  **overrides** any inherited value for that variable, and the same ports are given to the
  job's `stop` command. `run up` / `run start` print what was bound (`web started · PORT=3010`).
  The `on_create` / `on_clean` hooks receive them too — so an `on_clean = "docker compose
  down"` reads the same `${DB_PORT}` the job bound — but only the names a **single** job
  declares: a variable two jobs declare on different bases has no answer outside a job and
  stays unset in a hook.
- **Next projects need one line before their own name reaches them.** When the proxy
  serves a job whose directory holds a `next.config.*` without `allowedDevOrigins`,
  `run up` prints the exact line to add under "Next dev origins". wtm never edits
  third-party config — report the finding and let the user apply it. Like the port check,
  it never fails the run and never changes the exit code. Vite needs nothing: it allows
  `.localhost` already.
- **`[proxy]` in the global config** tunes the proxy for the whole machine. That file sits
  under the OS config directory — `~/.config/wtm/` on Linux, `~/Library/Application Support/wtm/`
  on macOS — and `wtm run proxy status` prints its resolved path, so never spell it yourself:
  `port` (default `11080`) and `enabled` (default on). Switching it off is not a failure —
  every URL wtm prints falls back to the direct `http://localhost:<port>` form. Same if
  the port is already taken: the jobs still start, wtm prints the direct form and says
  once why the names are off. **The URL wtm reports is always one that works** — it comes
  from what the daemon is really serving, not from what the config asked for.
- **A job can publish one of its ports under a name.** `wtm run init` proposes this for
  every service that declares the port it listens on, so most jobs get it without an edit.
  `url = { port = "PORT" }` on a
  `[[job]]` says which of its declared ports speaks HTTP; `host` overrides the segment it
  is published under (defaulting to the job's name), and must be lowercase letters, digits
  and dashes, dot-separated. Two jobs claiming the same host makes `run.toml` refuse to
  load, naming both. A job with no `url` keeps no name and stays reachable by its port —
  that is the right answer for anything that does not speak HTTP (postgres, redis).
- **A job's `cmd` and `stop` are `/bin/sh` lines**, not whitespace-split argv: quotes, `&&`,
  pipes, redirections and globs work, and `${VAR}` expands from the job's environment. So a
  server that ignores `PORT` and only takes a CLI flag (vite) still gets isolated — pass the
  declared port back on the command line, quoting for **your** shell so wtm receives the
  variable verbatim: `wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000`.
  A `cmd` the shell cannot parse is refused when the job is written, naming the job. POSIX
  `sh` is always used, never the user's interactive shell: no `[[ ]]`, no process substitution.
- **A port hard-coded in a `.env` follows the worktree too**, via `[[env_port]]` links in
  `run.toml`. A link names a key, not a position: `{file = ".env", key = "DATABASE_URL",
  job = "db", port = "POSTGRES_PORT"}` tells wtm that this key's value carries that job's
  port, and wtm finds
  the declared base *inside* the value and shifts it — so `postgres://u:pw@localhost:5432/app`
  becomes `…:5442/app` while the rest of the value (credentials, path, query) is untouched.
  A bare `DB_PORT=5432` is the same mechanism. `wtm run init` scans the project's configured
  `.env` targets, offers the keys whose value holds a declared base, and writes the confirmed
  links; `--link-env` writes them without asking (nothing is ever inferred without one or the
  other). The rewrite then happens at `wtm create` (proposed interactively, **applied under
  `--yes`/no TTY** — a `.env` still pointing at another worktree's services is useless, not
  safe) and at `wtm env`, where the interactive recap offers "Apply, but leave the port values
  alone" beside the plain apply, and `--yes`/JSON apply it like `create` does. `--check` reports
  without writing, and counts a pending shift as drift.
  Three refusals, reported and never guessed: the key is absent from the file, the base
  appears **more than once** in the value, or **neither the base nor any offset of it** is
  there. A link naming a port no job declares, an invalid key, or the same `(file, key)`
  twice makes `run.toml` refuse to load; so does a link missing `job`, which is **required** —
  two apps may each declare a `PORT`, and the name alone would not say which base the key
  follows. The error names the jobs that do declare the port, so the fix is the line to
  write. A link naming a `.env` that is not a configured
  `[env]` target of `.wtm.toml` is refused too. `wtm env --mode refresh` compares linked
  values **modulo the offset**, so a worktree holding `5442` against a source holding `5432`
  is not a conflict — but a genuine difference in the same value still is.
- **A linked value that is a URL gets the job's whole address, not its port**, when the job
  it names publishes a `url` for that very port. `VITE_API_URL=http://localhost:4001` becomes
  `http://api-dev.feat-x.monorepo.localhost` (or `…localhost:11080` when the redirection is
  not installed). This is what makes named URLs usable at all: the browser sends a name as its
  `Origin`, so a `CORS_ORIGIN` holding a port blocks every cross-origin call. Both conditions
  are load-bearing — a bare `PORT=4011` stays a number even though its job publishes a name,
  and a `DATABASE_URL` stays on a port because Postgres has no name and never will. The same
  comparison rule applies: `wtm env --mode refresh` reads a port, another worktree's address
  and this worktree's address as **one setting**, so none of the three is a conflict against
  the others. Two refusals are reported and never guessed: an `https` value (the proxy serves
  plain HTTP) and a URL pointing at a host no job here serves.
- **`addressing` at the top of `run.toml`** picks between the two: `names` (the default) and
  `ports`. Setting `ports` is a real inverse — a later `wtm env` puts port numbers back into
  values wtm wrote as addresses. On a machine where the proxy is off (`[proxy] enabled = false`),
  ports are written whatever the project asked for, and the pass says so in one notice.
  **Under `names`, the named URL is the only working entrance** — the raw `localhost:<port>`
  sends an `Origin` the API no longer accepts, so always read the address from `run url`.
- **The main checkout is never provisioned, so under `names` its `.env` still holds ports.**
  wtm writes a worktree's `.env` at creation and on `wtm env`; nothing writes main's. Its jobs
  are still published under names, so a cross-origin call made through them is refused until
  `wtm env main` aligns it — the positional takes the main checkout like any other worktree.
  **The address every surface hands out follows the `.env`, not the setting**: while the file
  spells ports, `run up`, `run url`, `run open`, the run view and the `wtm ui` panel all give
  `http://localhost:<port>` — the entrance the app actually answers on — and the named URL
  comes back once `wtm env <worktree>` settles the file. The route is registered either way,
  so nothing restarts. One line says why: a band in the run view, a callout beside a stream,
  a note under the RUN rows in `wtm ui`. Only keys declared as `[[env_port]]` links are seen,
  so silence means nothing **linked** is out of step. A `.env` already on names whose port
  went stale keeps its names and is told they are out of step. Aligning main is a **choice**: it stops behaving as a
  checkout without wtm, and going back means `addressing = "ports"` → `wtm env main` → `names`
  again, `addressing` having no per-worktree scope. Same applies to a linked worktree whose
  port pass was declined.
- Two base ports must not differ by a multiple of the block, or two worktrees land on the
  same port: `3000` and `3010` are refused **when run.toml is read** (with both sides named),
  while `5434`/`5435`/`5436` are fine — a uniform offset preserves the gaps. Raise
  `port_offset_block` in run.toml if a project's ports genuinely need more room. A job that
  binds a port without declaring it still collides across worktrees.

**GitHub**
- `wtm checkout <number>` — fetch a PR's branch into a worktree; parent defaults to the
  PR's base. In JSON mode pass `--yes` and the PR `<number>` (both **required**; no picker);
  `--yes` resolves the parent → PR base and env → config default. A local branch of the
  PR's name is **reused as-is**, never reset — the response then sets `existing_branch: true`
  and `origin_state`; under `--yes` no ref is touched even when the branch is behind, so
  read `origin_state` and decide yourself. Fork PRs are out of scope — fall back to
  `gh pr checkout <number>`. Creating a PR is out of scope too — use `gh pr create`.

**Setup**
- `wtm config show` inspects config; `wtm config edit` and the `wtm init` wizard are
  interactive. Bootstrap non-interactively with
  `wtm init --non-interactive [--base-branch … --env-strategy … --install-command … --clean-command …]`, and
  reconfigure one section later with `wtm init --only env|hooks|worktrees --non-interactive --yes`.
  Services are **not** part of `wtm init` — configure them with `wtm run init`.
  See their `--help` for the full flag set.
- `wtm upgrade` updates **the CLI itself**, never worktrees — that is `wtm sync`. **Do not
  run it unless the user asked**: it replaces the binary you are driving. `--check` is the
  safe form — it reports availability and changes nothing. `--yes` skips the confirmation and
  is **required** with `--output json` unless `--check` is passed; without it the command
  exits `1` naming `--yes`. `--version <v>` pins a release and applies to a **standalone**
  install only — on a Homebrew or `go install` binary it errors. JSON is exactly
  `{"installed", "latest", "up_to_date", "method", "action"}`, where `method` is
  `homebrew|go-install|standalone|source` and `action` is `replaced|delegated|none|checked`.
  Exit `17` means the install cannot be upgraded (built from source, or the binary needs
  sudo); network and checksum failures exit `1`.

## Failure handling

On non-zero exit, read stderr, then:

- `12` (config not found) → repo not initialized. Run `wtm init --non-interactive` with
  flags, or ask the user to run interactive `wtm init`.
- `17` (`upgrade` unsupported) → nothing to retry. Report the message: a source build updates
  with `git pull && make install`, an unwritable binary needs the user to re-run with sudo.
- `11` (branch not found) → wrong name; re-run the relevant discovery call.
- `10` (worktree already exists) → either the path is taken, or the branch is checked out
  in another worktree. Enter it with `wtm go <branch>` (get its path from `wtm resolve
  <branch>`), or pick a different branch name. `--if-not-exists` turns this into a success
  returning the existing worktree's path.
- `14` (job not found) → not declared in `run.toml`; check `wtm run list`.
- `15` (extract conflict) → nothing changed; retry with `--on-conflict resolve` or a
  different `--to`. Covers both a file modified on both sides and one that merely already
  exists in the target. Exception: an untracked **binary** file already in the target cannot
  take conflict markers — `resolve` won't help, pick a different `--to`.
- `16` (run module not initialized) → run `wtm run init` (or `wtm run init --non-interactive`)
  to create `run.toml`, then re-run the command.
- `gh: …` → `gh` isn't authenticated; tell the user to run `gh auth login`.
- A `run up` job failed → its entry in the JSON array is `{"name", "status": "error",
  "message"}` plus **`output`** (everything the job wrote before it failed) and
  **`exit_code`**. `message` is the daemon's one-line reason; `output` is why. The document
  on stdout is always complete, exit code or not, so read it either way — branching on an
  entry with `status: "error"` tells you *which* job, which the exit code does not.
- A `sync` exited non-zero → some branch is `status: conflict`/`error` in the JSON. For a
  `conflict` (default mode) the rebase was aborted and the branch + descendants skipped —
  the user resolves it manually in its worktree, then re-run `sync`. `kept_in_progress:
  true` means the rebase is paused in that step's `path` for the user to finish with `git
  rebase --continue`. A `diverged` status (exit stays 0) needs the user to reconcile that
  branch before re-running.

## Escalate to the user when

- A command needs shell integration (`go` with no wrapper installed).
- The user wants to *browse* their worktrees rather than get an answer from you — tell them
  to run `wtm ui` themselves; never run it for them.
- A destructive action (`clean`/`prune --force`) wasn't explicitly authorized — don't add
  `--force` on your own initiative.
- `wtm config edit` is the natural answer — ask the user to run it, or read with `wtm
  config show` and write the change to the printed path if you have a file-edit tool and
  the user authorized it.
- You can't supply a value that non-interactive `wtm init` requires.
