# The JSON contract

What `--output json` gives you, command by command. The payload mirrors wtm's Go structs with stable `snake_case` fields; the shapes below name the fields an agent branches on, and running a command once with `--output json` shows its exact, current schema. Trust what you see over this list.

## Contents

- [General rules](#general-rules)
- [Worktrees: `list`, `tree`, `resolve`](#worktrees-list-tree-resolve)
- [`create`, `extract`, `checkout`](#create-extract-checkout)
- [`env_ports` and the `ports` block](#env_ports-and-the-ports-block)
- [`env`](#env)
- [`clean`, `prune`](#clean-prune)
- [`relocate`](#relocate)
- [Stacks: `sync`, `fast-forward`, `reparent`](#stacks-sync-fast-forward-reparent)
- [The run module](#the-run-module)
- [Job result statuses](#job-result-statuses)
- [`upgrade`](#upgrade)

## General rules

- JSON goes to stdout; human text and warnings go to stderr.
- **Check the exit code, and parse stdout only if it is non-empty.** A command that got far enough to have per-item results writes its **whole** document and *then* exits `1` (`run up`, `run down`, `prune`, `sync`, `fast-forward`): read it either way, the entries say which item failed. One exception: `sync --push` whose push fails exits before writing the document. One that failed before that (no such job, daemon refused, config invalid) writes **nothing** on stdout and puts the reason on stderr.
- `--output json` requires `--yes` on every mutating command (except `env --check`).
- `--quiet` never affects the JSON.

## Worktrees: `list`, `tree`, `resolve`

- `list` and `tree`: per worktree, branch, path, PR, services, dirty state, plus:
  - `origin`: `{ahead, behind, state}` against `origin/<branch>`, or `null` when the branch has no origin counterpart. `state` is `up-to-date` / `behind` / `ahead` / `diverged`.
  - `commits_ahead`: commits against the **parent/base** branch (not origin).
  - `tree` only: the parent to child nesting and `needs_sync` (the parent moved past this node).
- `resolve <branch>`: `{path, branch}`.

## `create`, `extract`, `checkout`

`create` wraps its results in an envelope, even for one branch: `{"results": [...], "failed": [{"branch", "path"?, "error", "exit_code"}...]}`. The fields below sit on each entry of `results`; `extract` and `checkout` carry them at the top level. A refusal before anything is created (a bad `--from`, a name clash, a repeated branch) writes no envelope: it is an error on stderr with its exit code. See `worktrees.md`.

- `already_exists: true` when `create --if-not-exists` found the worktree (with its path, possibly outside `base_path`, even the main checkout's).
- `existing_branch: true` and `origin_state` (`up-to-date` / `behind` / `ahead` / `diverged`) when a same-named local branch was reused as is.
- `isolation`: the worktree's, `isolated` or `verbatim` (for `extract`, the target's).
- `env_ports`: present when the port pass ran and the project links anything (shape below). Absent for a verbatim worktree, a project linking nothing, or a pass that could not run.
- `warnings`: why the port pass could not run ("ports not settled, run `wtm env <branch>` once run.toml is fixed"), or that `--isolation` differed from an existing worktree's and was ignored.
- `extract` file entries may carry `"status": "renamed"` with `"orig_path"`.

## `env_ports` and the `ports` block

`create`'s `env_ports` and `env`'s `ports` share one shape:

```
{offset, addressing, public_port,
 entries: [{file, key, port, base, resolved, moves: [{port, job, base, resolved}],
            addressing, status, current_value, new_value, foreign_host}],
 owned: [{file, key, value, changed}],
 applied}
```

- `addressing` at the top is what the project asked for (`names` / `ports`); each entry's `addressing` is how that one value was written.
- `public_port`: the port a named URL announces (absent when nothing serves names).
- `moves`: every port a value holding several origins follows.
- Entry `status`: `rewrite` or `unchanged` are settled values. `missing_key`, `base_not_found`, `ambiguous`, `foreign_host`, `secure_scheme` are values wtm left alone (a refusal to report).
- `foreign_host`: where a value pointed that the proxy does not serve.
- `owned`: the values wtm writes whole (`COMPOSE_PROJECT_NAME`, `[[env]]`).
- `applied`: whether the rewrites were written.

## `env`

```
{branch, mode, check,
 files: [{target, strategy, source, applied, parent_branch, parent_fallback, unresolvable?,
          diff: {mode, entries: [{key, status, current_value, resolved_value, placeholder, source, export}]}}],
 ports: {…see above…},
 isolation, isolation_adoption, isolation_changed,
 restored: [{file, key, from, to, removed}],
 warnings}
```

- Key `status`: `resolved` / `missing_unresolved` / `conflict` / `orphan`.
- `source` names the value source (`template (no .env to sync from)` on a fresh project). `parent_fallback: true` means main was used because the parent had no readable file; `parent_branch` names the parent. `unresolvable: true` flags a configured file that exists nowhere.
- `ports` is empty when the project declares no link, and always empty for a `verbatim` worktree.
- `isolation`: the worktree's. `isolation_adoption` appears only for a worktree created before the isolation choice: `not_adopted` (run values left alone, no `isolation` reported, empty `ports`) or `adopted` (this run recorded it and settled its values).
- `isolation_changed`: the recorded isolation changed. `restored`: the values `--isolation verbatim` put back to the source's (`removed` when the source lacked the key).

## `clean`, `prune`

- `clean`: `{results: [{branch, path, already_absent}], failed: [{branch, path?, error, exit_code}], skipped, reparented, orphaned_children, namespaces}`, an envelope even for one worktree.
- `prune`: `pruned` lists the removed worktrees, with `reason` values: `pr_merged` / `pr_closed` / `gone`.
- `skipped` (both): unsafe worktrees left alone without `--force` (for `clean`, only when the user chose to delete the safe ones; under `--yes` it refuses instead), reason `dirty` / `unpushed` / `open_pr`. When every match is unsafe, `prune` returns `pruned: []` and `skipped: []`: nothing was removed, not nothing matched.
- `failed`: `prune` stops at the first failure and reports it as one object `{branch, path, error}`; `clean` keeps going and reports an array. Exit non-zero either way.
- `namespaces` (both): one entry per namespace a removed worktree held: `{branch, job, name, status, reason?}`, `name` like `app_feat-x`, `status` one of:
  - `dropped`;
  - `deferred`: service down, a drop it refused, or a drop past its 30 s timeout; owed and paid next time wtm finds the service up (`reason` says which);
  - `kept`: `--keep-data`, or another live worktree reduces to the same name (`feat.x` beside `feat/x`), so the namespace is its too; never dropped.

## `relocate`

- `blocked_jobs`: worktrees not moved because their jobs are running (exit non-zero; `--force` does not lift it).
- `blocked_name`: external worktrees left unadopted because their derived name is taken (exit non-zero).

## Stacks: `sync`, `fast-forward`, `reparent`

- `sync`: one step per branch with `status` (`conflict`, `error`, `diverged`, …), `path`, and `kept_in_progress: true` when `--keep-conflict` left a rebase paused there. Plus `base_targeted` (whether the base was fetched or fast-forwarded) and `parent_updates: [{branch, status, old_tip, new_tip, behind, children, detail}]` with `status` `behind` / `fast_forwarded` / `diverged` / `ff_failed`. Exit non-zero on `conflict` or `error`; `diverged` keeps exit 0. Meanings in `stacks.md`.
- `fast-forward`: `[{branch, status, old_tip, new_tip, behind, detail?}]`, `status` one of `already up to date`, `fast-forwarded from origin`, `diverged`, `no origin counterpart`, `failed`. Only `failed` makes the exit non-zero.
- `reparent`: `{"reparented": [{branch, old_parent, new_parent}, …]}`.

## The run module

One contract across the module: **each command has one fixed shape, and the exit code follows the success.** A job object is keyed `name`; anything else pointing at a job calls it `job`. A worktree is always named by **`branch` and `path`** together, never `worktree` or `work_dir`.

Commands acting on worktrees answer with **an array of per-worktree documents**, even for one worktree, in the order you named them:

- `run up`: `[{branch, path, profile?, aborted, jobs: [{name, status, url?, held?, namespace?, …}]}]`. A failed job's entry is `{name, status: "error", message, output, exit_code}`: `message` is the daemon's one-line reason, `output` everything the job wrote before failing. A runner's `held` is `[{job, url}]`.
- `run down` / `run stop`: `[{branch, path, jobs: [{name, status, message?}]}]`. `run down --all`: one document per worktree it emptied.
- `run logs`: `[{branch, path, lines: [{job, at, text}]}]`. `at` is RFC3339 UTC; lines are grouped by job, so `at` is not monotonic across jobs. `lines: []` when nothing was recorded.

Single-subject commands answer with one object or array:

- `run start`: `{name, status, …}`, the job it started (with `namespace` when it carved one).
- `run job add|edit|rm`, `run profile add|edit|rm`: `{name, status, message?}`, `status` one of `added`, `updated`, `unchanged` (an edit that changed nothing), `removed`.
- `run ps`: `[{name, kind, status, pid, branch, path, project, started_at?, url?, exit_code?, held?}]`. `status` is a runtime status (`running`, `detached`, `crashed`, `stopped`, `joined`, `reaped`; see `run.md`). `pid` is `0` on a `joined` claim. `held` (`[{job, url}]`) is set on a runner that is up.
- `run url` / `run open`: `[{job, url}]`. With `--job`, that job alone (one-element array).
- `run addressing`: `{addressing, previous, changed, settled: [branch], pending: [branch], main_left?: branch}`.
- `run daemon status`: one object (up, build, PID, what it holds), with `index_frozen: true` when the index belongs to a newer wtm (absent otherwise).
- `run proxy status`: one object: the proxy's bind port, the public port announced in URLs, and whether the port-80 redirection is installed.
- `run export`: the layout document itself (what `run import` takes).

## Job result statuses

A job-result `status` in `run up` / `down` / `start` / `stop`:

| Status | Meaning |
|---|---|
| `started` | started by this run |
| `already_running` | the service was already up in that worktree; nothing was started, the run goes on |
| `joined` | a claim on a shared service already running (or started from a worktree other than main) |
| `done` | a task that ran to the end |
| `stopped` | stopped |
| `released` | a shared job let go of, still up for other worktrees |
| `not_running` | nothing was up under that name; nothing was stopped (exit 0) |
| `error` | failed; see `message` (and `output`, `exit_code` on `run up`) |

## `upgrade`

Exactly `{"installed", "latest", "up_to_date", "method", "action"}`, where `method` is `homebrew` / `go-install` / `standalone` / `source` and `action` is `replaced` / `delegated` / `none` / `checked`.
