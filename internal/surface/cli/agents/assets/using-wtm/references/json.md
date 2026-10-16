# The JSON contract

What `--output json` gives you, command by command. The payload mirrors wtm's Go structs with stable `snake_case` fields; the shapes below name the fields an agent branches on, and running a command once with `--output json` shows its exact, current schema. Trust what you see over this list.

## Contents

- [General rules](#general-rules)
- [The v2 envelope (`create`)](#the-v2-envelope-create)
- [Worktrees: `list`, `tree`, `resolve`](#worktrees-list-tree-resolve)
- [`create`, `extract`, `checkout`](#create-extract-checkout)
- [`env_ports` and the `ports` block](#env_ports-and-the-ports-block)
- [`env`](#env)
- [`status`](#status)
- [`clean`, `prune`](#clean-prune)
- [`exec`](#exec)
- [`relocate`](#relocate)
- [Stacks: `sync`, `fast-forward`, `reparent`](#stacks-sync-fast-forward-reparent)
- [The run module](#the-run-module)
- [Job result statuses](#job-result-statuses)
- [`version`](#version)
- [`upgrade`](#upgrade)

## General rules

- JSON goes to stdout; human text and warnings go to stderr.
- **Check the exit code, and parse stdout only if it is non-empty.** A command that got far enough to have per-item results writes its **whole** document and *then* exits `1` (`run up`, `run down`, `prune`, `sync`, `fast-forward`): read it either way, the entries say which item failed. One exception: `sync --push` whose push fails exits before writing the document. One that failed before that (no such job, daemon refused, config invalid) writes **nothing** on stdout and puts the reason on stderr.
- `--output json` requires `--yes` on every mutating command still on the v1 contract (`--dry-run` also does for `sync`, `prune`, `relocate`; `--check` for `env`, `upgrade`). Without it the command exits `2` before doing anything, with nothing on stdout. A command on the v2 envelope (`create`) is unattended in JSON already: `--yes` is accepted and changes nothing.
- `--quiet` never affects the JSON.
- One command streams instead of writing one document: `events`, one JSON object per line, never ending on its own. Its contract is in `references/events.md`.

## The v2 envelope (`create`)

A command on wtm's command engine writes one document on stdout for **every** run, a refusal before anything started included, and `"v": 2` says so. Today that is `create`; the other commands move to it one by one, each release saying which.

```json
{ "v": 2, "command": "create", "ok": true, "op": "3f2a…",
  "items": [ { "subject": "feat/x", "status": "done", "detail": { … },
               "warnings": [ { "code": "provision.ports_not_settled", "params": { … }, "message": "…" } ] } ],
  "follow_ups": [], "warnings": [] }
```

- `ok` is false whenever there is an `error`. `follow_ups` and `warnings` are always arrays; an item's `warnings` are about its subject, the top-level ones about the run.
- An applied run has `items`: one per subject, `status` `done` / `failed` / `skipped` / `cancelled`, and a `reason` when it says more: `unchanged` (done, nothing to do), `incomplete` (the worktree exists but a later phase, such as `on_create`, failed or was interrupted), `interrupted`, `not_reached`. A `failed` item carries its `error`.
- `--dry-run` writes `plan` (what an apply would do) and `fingerprint` instead of `items`. Pass that fingerprint back with `--expect-fingerprint <hash>` to apply exactly what was previewed: if the repository changed in between, the run is refused with `precondition` / `plan.stale` (exit `25`) and nothing is applied.
- `error` is `{kind, code, message, params?, follow_up?}`. `kind` is `invalid` (exit `2`), `conflict` (exit `24`: `lock.held`, another wtm operation holds this worktree or the repository; its `params` name the holder), `precondition` (exit `25`, or the command's own code: `create.config_not_found` exits `12`), `refused` (exit `23`), `cancelled` (exit `19`), `not_found` or `internal` (exit `1`). An `invalid` error lists its `fields`, each `{path, code, flag, message}`: `path` is the request field (`branches[1]`, `from`), `flag` how you set it (`--from`, `<branches>[1]`), `code` why (`required`, `not_found`, `create.checked_out_elsewhere`…). **Branch on `code`, show `message`.**
- Exit codes: `0` when no item failed; a failed item exits on its error; an interrupted run `19`.

## Worktrees: `list`, `tree`, `resolve`

- `list` and `tree`: per worktree, branch, path, PR, services, dirty state, plus:
  - `origin`: `{ahead, behind, state}` against `origin/<branch>`, or `null` when the branch has no origin counterpart. `state` is `up-to-date` / `behind` / `ahead` / `diverged`.
  - `commits_ahead`: commits against the **parent/base** branch (not origin).
  - `is_locked`: the worktree is locked (`git worktree lock`); `clean` and `prune` refuse it unless `--force`.
  - `tree` only: the parent to child nesting and `needs_sync` (the parent moved past this node).
- `resolve <branch>`: `{path, branch}`.

## `create`, `extract`, `checkout`

`create` writes the [v2 envelope](#the-v2-envelope-create): one item per branch, its `subject` the branch, its `detail`:

- `path`: the worktree; `metadata`: what wtm recorded (`source_branch`, the parent `wtm sync` follows; `env_strategy`; `isolation`, `isolated` or `verbatim`; `created_at`).
- `existing_branch: true` and `origin_state` (`up-to-date` / `behind` / `ahead` / `diverged`, with `origin_ahead` / `origin_behind`) when a same-named local branch was checked out as is.
- `upstream: "origin/<branch>"` when the branch existed only on origin: the local branch was created from it and tracks it.
- `ordinal`: the worktree's number, when this run needed one (its ports).
- `env_ports`: present when the port pass ran and the project links anything (shape below). Absent for a verbatim worktree, a project linking nothing, or a pass that could not run (an item warning then says why).
- An item `done` with `reason: "unchanged"` is `--if-not-exists` finding the worktree; its `detail.path` is where it is, possibly outside `base_path`, even the main checkout.
- The item's `warnings`: an `.env` that could not be copied, a `run.toml` link ignored because its `.env` is not a configured `[env]` target, a port pass that could not run ("ports not settled, run `wtm env <branch>` once …"). The run's `warnings`: a source that could not be fast-forwarded (`create.ff_failed`, `create.ff_dirty`), a diverged source, `--env-from parent` falling back to the main checkout's `.env`.
- A refusal before anything is created is the envelope's `error`, kind `invalid` (exit `2`), one `fields` entry per problem: `create.checked_out_elsewhere`, `create.name_clash`, `create.path_exists`, `create.branch_name_invalid` on `branches[i]`, `distinct` on a branch listed twice, `not_found` on an unknown `from`, `required` on `from` for a branch that already exists (`params.because: create.parent_required`). A missing config is kind `precondition`, code `create.config_not_found`, exit `12`.

`extract` and `checkout` are still on the v1 contract: they carry the same fields as `create`'s `detail`, at the top level, plus `already_exists: true` when the worktree was found, `isolation`, and `warnings` as strings. `extract` file entries may carry `"status": "renamed"` with `"orig_path"`. See `worktrees.md`.

## `env_ports` and the `ports` block

`create`'s `env_ports` and `env`'s `ports` share one shape:

```
{offset, addressing, public_port,
 entries: [{file, key, port, base, resolved, moves: [{port, job, base, resolved, origin}],
            addressing, status}],
 owned: [{file, key, value, changed}],
 applied}
```

- `addressing` at the top is what the project asked for (`names` / `ports`); each entry's `addressing` is how that one value was written.
- `public_port`: the port a named URL announces (absent when nothing serves names).
- `moves`: every port the value follows, `base` → `resolved`. `origin` is the address wtm writes for it under named addressing (`http://web.feat-x.app.localhost:1355`), absent for a port.
- Entry `status`: `rewrite` or `unchanged` are settled values. `missing_key`, `base_not_found`, `ambiguous`, `foreign_host`, `secure_scheme` are values wtm left alone (a refusal to report).
- `owned`: the values wtm writes whole (`COMPOSE_PROJECT_NAME`, `[[env]]`).
- No text of a value is written: every field comes from wtm's own plan (port numbers, the address it builds, run.toml names). `current_value` / `new_value`, and `foreign_host` (where a refused value pointed), appear only under `wtm env --show-values`, whole.
- `applied`: whether the rewrites were written.

## `env`

```
{branch, path, mode, check,
 files: [{target, strategy, source, applied, parent_branch, parent_fallback, unresolvable?, created?,
          diff: {mode, entries: [{key, status, current_value, resolved_value, placeholder, source, export, action?, redacted?}]}}],
 ports: {…see above…},
 isolation, isolation_adoption, isolation_changed,
 restored: [{file, key, ports, removed}],
 warnings}
```

- Key `status`: `resolved` / `missing_unresolved` / `conflict` / `orphan` — the drift the run found. Key `action` is what an apply did to it: `added`, `filled`, `overwritten`, `kept`, `pruned` or `skipped`; absent under `--check` and for a key left as it was (an unanswered `missing_unresolved` stays without one).
- Values are withheld for every key, wtm's own included: the values the reconciliation compares are read from the files, so they are the user's. An entry has no `current_value` / `resolved_value` and carries `"redacted": true` (absent when there was no value to withhold: an empty value, a missing key). What wtm writes is in `ports`: `owned[].value` whole (`COMPOSE_PROJECT_NAME`, `[[env]]`), and each link's `moves`. `placeholder` is the one text shown from a file: it comes from the committed template. An addition is still told apart by its `source`. `--show-values` writes every value, secrets included: never pass it in a context that is logged or shared.
- Key `source` (on an addition or a conflict) names the level the value came from. File `source` names the value source (`template (no <file> in the main checkout)` when the main checkout has no copy of the file, so its keys need a value). `parent_fallback: true` means main was used because the parent had no readable file; `parent_branch` names the parent. `unresolvable: true` flags a configured file that exists nowhere. `created: true` is a file the worktree lacked: an apply rebuilt it as `create` provisions it (`applied` too), a `--check` reports it as drift.
- `ports` is empty when the project declares no link, and always empty for a `verbatim` worktree.
- `isolation`: the worktree's. `isolation_adoption` appears only for a worktree created before the isolation choice: `not_adopted` (run values left alone, no `isolation` reported, empty `ports`) or `adopted` (this run recorded it and settled its values).
- `isolation_changed`: the recorded isolation changed. `restored`: the values `--isolation verbatim` put back to the source's (`removed` when the source lacked the key), with `ports`, the base ports run.toml declares for a linked key (where its value goes back to); `from` / `to`, the values whole, only under `--show-values`.

## `status`

`status [worktree]` writes one object, `status --all` an array of the same objects (main first): `branch`, `path`, `main`, `isolation`, `addressing` and `offset` (`null` without `run.toml`; `offset` `null` before a run numbers the worktree), `run_config`, `env` `{declared, missing[]}`, `jobs[]` `{name, kind, state, url?, exit_code?, shared?, owner?}` with `state` in the `job.*` vocabulary (`starting`, `running`, `crashed`, `exited`, `stopped`), and `problems[]` `{code, message, fix}`, always present. It exits `0` whatever it finds and needs no `--yes`. The codes and their fixes are in `references/state.md`.

## `clean`, `prune`

- `clean`: `{results: [{branch, path, already_absent}], failed: [{branch, path?, error, exit_code}], skipped, reparented, orphaned_children, namespaces}`, an envelope even for one worktree.
- `prune`: `pruned` lists the removed worktrees, with `reason` values: `pr_merged` / `pr_closed` / `gone`.
- `skipped` (both): unsafe worktrees left alone without `--force` (for `clean`, only when the user chose to delete the safe ones; under `--yes` it refuses instead), reason `locked` / `dirty` / `unpushed` / `open_pr`, or `interrupted` for a worktree an interrupt stopped the run before (untouched; exit `19`). When every match is unsafe, `prune` returns `pruned: []` and `skipped: []`: nothing was removed, not nothing matched.
- `failed`: `prune` stops at the first failure and reports it as one object `{branch, path, error}`; `clean` keeps going and reports an array. Exit non-zero either way.
- `namespaces` (both): one entry per namespace a removed worktree held: `{branch, job, name, status, reason?}`, `name` like `app_feat-x`, `status` one of:
  - `dropped`;
  - `deferred`: service down, a drop it refused, or a drop past its 30 s timeout; owed and paid next time wtm finds the service up (`reason` says which);
  - `kept`: `--keep-data`, or another live worktree reduces to the same name (`feat.x` beside `feat/x`), so the namespace is its too; never dropped.

## `exec`

- `{command, results: [{branch, path, status, exit_code?, duration_ms?, log?, tail?, output?, error?}], failed: [branch]}`, an envelope even for one worktree.
- `status`: `passed` / `failed` / `interrupted` / `not_started`. `exit_code`, `duration_ms` and `log` are absent for `not_started`; `exit_code` is absent for `interrupted`.
- `error`: the command could not start (e.g. the worktree directory is gone).
- `tail`: the last 20 lines of the combined stdout and stderr, as a terminal would show them (progress frames rewritten by `\r` collapsed, colours removed; the log keeps the raw bytes). With `--print`, `output` carries the whole output instead.
- `failed`: every branch whose status is not `passed`, `[]` when all passed.

## `relocate`

- `{base_path, base_path_updated, steps: [...]}`: `steps` is `[]`, never `null`, and `base_path` is always the one in effect after the run, also when nothing had to change.
- `blocked_jobs`: worktrees not moved because their jobs are running (exit non-zero; `--force` does not lift it).
- `blocked_name`: external worktrees left unadopted because their derived name is taken (exit non-zero).

## Stacks: `sync`, `fast-forward`, `reparent`

- `sync`: one step per branch with `status` (`conflict`, `error`, `diverged`, `cancelled` for a branch an interrupt reached first — rebase aborted, branch unchanged, exit `19`, nothing pushed; …), `path`, and `kept_in_progress: true` when `--keep-conflict` left a rebase paused there. Plus `base_targeted` (whether the base was fetched or fast-forwarded), `base_interrupted: true` when an interrupt cut the base's refresh short and `parent_updates: [{branch, status, old_tip, new_tip, behind, children, detail}]` with `status` `behind` / `fast_forwarded` / `diverged` / `ff_failed`. Exit non-zero on `conflict` or `error`; `diverged` keeps exit 0. Meanings in `stacks.md`.
- `fast-forward`: `[{branch, status, old_tip, new_tip, behind, detail?}]`, `status` one of `already up to date`, `fast-forwarded from origin`, `diverged`, `no origin counterpart`, `failed`, `interrupted, unchanged` (exit `19`). Only `failed` makes the exit non-zero.
- `reparent`: `{"reparented": [{branch, old_parent, new_parent}, …]}`.

## The run module

One contract across the module: **each command has one fixed shape, and the exit code follows the success.** A job object is keyed `name`; anything else pointing at a job calls it `job`. A worktree is always named by **`branch` and `path`** together, never `worktree` or `work_dir`.

Commands acting on worktrees answer with **an array of per-worktree documents**, even for one worktree, in the order you named them:

- `run up`: `[{branch, path, profile?, aborted, interrupted?, jobs: [{name, status, url?, held?, namespace?, …}]}]`; `interrupted: true` when an interrupt stopped that worktree's start (exit `19`, the jobs already started keep running). A failed job's entry is `{name, status: "error", message, output, exit_code}`: `message` is the daemon's one-line reason, `output` everything the job wrote before failing. A runner's `held` is `[{job, url}]`.
- `run down` / `run stop`: `[{branch, path, jobs: [{name, status, message?}]}]`. `run down --all`: one document per worktree it emptied.
- `run logs`: `[{branch, path, lines: [{job, at, text}]}]`. `at` is RFC3339 UTC; lines are grouped by job, so `at` is not monotonic across jobs. `lines: []` when nothing was recorded.

Single-subject commands answer with one object or array:

- `run start`: `{name, status, …}`, the job it started (with `namespace` when it carved one).
- `run job add|edit|rm`, `run profile add|edit|rm`: `{name, status, message?}`, `status` one of `added`, `updated`, `unchanged` (an edit that changed nothing), `removed`.
- `run ps`: `[{name, kind, status, pid, branch, path, project, started_at?, url?, exit_code?, held?}]`. `status` is a runtime status (`running`, `detached`, `crashed`, `stopped`, `joined`, `reaped`; see `run.md`). `pid` is `0` on a `joined` claim. `held` (`[{job, url}]`) is set on a runner that is up.
- `run url` / `run open`: `[{job, url}]`. With `--job`, that job alone (one-element array).
- `run addressing`: `{addressing, previous, changed, settled: [branch], pending: [branch], main_left?: branch}`.
- `run daemon status`: one object (up, build, PID, what it holds: `foreground_jobs` counts the jobs it supervises, which die with it — `run up -d` ones included —, `detached_jobs` the stacks that outlive it; a `joined` claim is in neither), with `index_frozen: true` when the index belongs to a newer wtm (absent otherwise).
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

## `version`

`{"version", "events"}`: the binary's version (`dev` for a local build), then one integer per versioned contract, today `events` (the schema version of `wtm events`). More keys may be added; ignore the ones you do not know.

## `upgrade`

Exactly `{"installed", "latest", "up_to_date", "method", "action"}`, where `method` is `homebrew` / `go-install` / `standalone` / `source` and `action` is `replaced` / `delegated` / `none` / `checked`.
