# Proof: `wtm create` (legacy `flow/`) vs `wtm-next create`

The reference is the legacy `wtm create` of 0.30.1. Its output differs by design (next speaks the v2 envelope and plans before it acts), so the proof compares what each leaves on disk and keeps both outputs side by side.

## How to run

From the repository root, after `make build next`:

```bash
cd next/docs/proof
wtm=../../../bin/wtm next=../../../bin/wtm-next
sandboxes=$(./create.sh "$wtm" "$next" create)   # the sandboxes it leaves behind, legacy then next
./tty.sh legacy "${sandboxes% *}" "$wtm" tty/legacy
./tty.sh next "${sandboxes#* }" "$next" tty/next
./extract.sh "$wtm" "$next" extract
./list.sh "$wtm" "$next" list
./resolve.sh "$wtm" "$next" resolve
./status.sh "$wtm" "$next" status
./tree.sh "$wtm" "$next" tree
./events.sh "$wtm" "$next" events
./clean.sh "$wtm" "$next" clean
./prune.sh "$wtm" "$next" prune
./fast-forward.sh "$wtm" "$next" fast-forward
./exec.sh "$wtm" "$next" exec
./ui.sh "$next" ui
git diff --stat .
```

The GIFs (`vhs`) are recorded in a fresh next sandbox, removed afterwards:

```bash
scratch=$(mktemp -d)
sandboxes=$(./create.sh "$wtm" "$next" "$scratch")
sb=${sandboxes#* }
cp "$next" "$sb/bin/wtm-next"
printf 'export HOME=%s/home PATH=%s/bin:$PATH\ncd %s/acme\n' "$sb" "$sb" "$sb" >"$sb/env.sh"
WTM_PROOF_SANDBOX=$sb vhs form.tape && WTM_PROOF_SANDBOX=$sb vhs hooks.tape
rm -rf "${sandboxes% *}" "$sb" "$scratch"
```

The scripts take the binaries by any path and refuse one that is not executable. Each run makes its own sandboxes (`mktemp`, `/tmp/wtm` and five random characters, as long as the paths the captures were taken with, since a screen wraps a path at 100 columns) and names its tmux sessions after them, so two runs at once, from two worktrees, never meet: no lock is needed. `tty.sh` replays in the sandbox `create.sh` printed and removes it when done; `extract.sh` removes its own. Paths are normalised to `<SB>`, so the captures do not depend on the run.

- `create.sh <wtm> <wtm-next> create`: two identical sandboxes (the `docs/demos` "acme" project with the same ports, a bare `origin`, a local `feature` and `topic`, a branch only `origin` has, and an `on_create` hook that fails on `feat/bad*` and sleeps on `feat/slow*`), the same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (branches, worktrees, every `meta.json`, every `.env`, `config.toml`). Paths, timestamps, op ids, fingerprints and durations are normalised. `create/states.diff` holds the differences of state.
  1. a new branch
  2. three branches with one failing hook
  3. an existing local branch without `--from`
  4. the same branch with `--from main`
  5. a branch only origin has
  6. `--if-not-exists`, first run
  7. `--if-not-exists`, second run
  8. `--env-from main`
  9. `--output json` success
  10. `--output json` invalid request
  11. `--output json` failing hook
  12. `-q` on a worktree that exists
  13. `--ff` on a source behind origin
  14. `env_strategy = "config"` remembered, as 0.30.1 writes "Use config default", over `env.strategy = "main"`
  15. `--dry-run` with `--remember env_from` (next only)
  16. `--expect-fingerprint` with that fingerprint (next only)
  17. `--expect-fingerprint` with a stale fingerprint (next only)
  18. `--output json --dry-run` of `--if-not-exists` on the worktree 9 made (next only)
  19. `--output json --expect-fingerprint` with that fingerprint: applied, the worktree unchanged (next only)
  20. `--output json --expect-fingerprint` with a stale fingerprint, its `follow_ups` the `--dry-run` that plans again (next only)
  21. a branch checked out in a worktree already, `feat/one`
  22. the same, `--output json`
  23. the run proxy off (`[proxy] enabled = false` in the global config) while run.toml asks for named addresses, its default
  24. the same, `--output json`
  25. `--dry-run` with a run.toml that does not parse (next only)
  26. an absolute `base_path` in config.toml; then the worktrees 23, 24 and 26 made are removed, so `tty.sh` finds the sandbox as before
- The states are identical for 1 to 15. The differences, all deliberate:
  - 3 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1. 12 likewise: next exits 2 where legacy exits 10.
  - 14 gives both the configured strategy, `main`.
  - 15 to 20: legacy has no dry run, fingerprint nor `--remember`, and refuses the flag (exit 2). Next plans 15 and 18 without applying them, applies 16 (`feat/dry`, `env_from` remembered as `main`) and 19 (nothing to change), and refuses 17 and 20 (exit 25).
  - 21 and 22 are refused by both, next with exit 2 (legacy 10, and text on stderr for 22 where next prints the envelope). Next's message no longer spells a command line: the field error carries its follow-up, `→ wtm go feat/one   jump in` after the failure, `error.fields[].follow_up` (`{command: "go", answers: [{path: "worktree", …}], code: "create.jump_in"}`) in the JSON, and in the form under the step's error (`tty/next/checked-out-elsewhere.txt`).
  - 23 and 24 write ports on both sides and say why: legacy's notice ("This project asks for named addresses … ports were written instead — `wtm run proxy status` reports what serves names", on stderr beside its JSON), next's warning of the worktree, `provision.ports_not_named`, its follow-up `→ wtm run proxy status   reports what serves names` (`follow_up` in the item's warning in JSON). Next says it only once a port was written: a project whose every link was left alone (its key missing from the `.env`) gets the ports-left-alone warning alone, where legacy said both.
  - 25: with a run.toml that does not parse, next's recap keeps its Isolation row, `isolated (run.toml unreadable)`, the isolation the worktree records (run.toml's own cannot be read: the one given, else the remembered one, else the default), and the form skips the question saying `run.toml could not be read` (`tty/next/run-unparsable-recap.txt`); legacy's recap has no isolation line and its form says `run.toml declares nothing a worktree isolates` (`tty/legacy/run-unparsable-recap.txt`).
  - 26, an approved deviation (#215): next refuses an absolute `base_path` (exit 1, `✗ invalid config.toml: base_path must be relative to the repository: <SB>/abs.trees`), where legacy says it created the worktree there and puts it under the main checkout instead, at the absolute path read as a relative one (`<SB>/acme/tmp/…/abs.trees/feat-abs`), an untracked directory of main.
- `tty.sh <side> <sandbox> <binary> tty/<side>`, after `create.sh`: tmux at 100×40 in that side's sandbox, `tty/legacy/` and `tty/next/` hold the screens.
  - The form under `--ask`: branches, then source, env strategy (the remembered answer ticked), isolation and the recap, then apply and the outcome. Next asks one step more (`If not exists`, shown settled) and draws its recap as the plan. Both offer the same env rows, "Use config default (main)" first; each ticks the answer its sandbox remembers: `config` on legacy, `main` on next, which 16 remembered (`--env-from main --remember env_from`, which legacy refuses).
  - One Ctrl+C during a hook: both exit 19, next saying `Cancelling — finishing on_create of feat/slow; Ctrl+C again to stop now`.
  - Two Ctrl+C: the run dies by SIGINT, exit 130 for both.
  - The extract form (`extract-*`), on a change made in `feat/form`: source, files, a new target `feat/moved`, its source branch, isolation, mode and the recap, then apply and the outcome. Next has one step more, the new target's env strategy, settled by the answer the create form remembered (`✓ Env strategy: main · remembered`); legacy's extract never asks it. Both skip the source update (main is not behind) and next shows the conflict question settled; legacy's target list is sorted, since it follows git's order, which varies between runs.
  - A branch checked out elsewhere entered in the form (`checked-out-elsewhere`): both refuse it at the step; next's error is followed by `→ wtm go feat/form   jump in` where legacy's message spells the command.
  - The recap with a run.toml that does not parse (`run-unparsable-recap`): see create's 25.
  - Each screen is captured once it shows what it waits for and holds still.
- `form.gif` and its still `form-recap.png` (the recap), and `apply-hooks.gif`, are recorded from `form.tape` and `hooks.tape` in a next sandbox (see How to run): the form through its recap and apply, and three branches whose hooks run, one failing.

# Proof: `wtm extract` (legacy `flow/`) vs `wtm-next extract`

The reference is the legacy `wtm extract` of 0.30.1. Its output differs by design — next speaks the v2 envelope, plans before it acts and refuses earlier — so the proof compares what each leaves on disk.

- `extract.sh <wtm> <wtm-next> extract`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`, a `feat/login` worktree holding a modified `README.md`, an untracked `apps/api/new/` directory, an untracked `apps/web/login.ts`, a deleted `apps/web/package.json` and a renamed `scripts/serve.py`, and a `topic` worktree), the same scenarios run by each binary; after each, the exit code, stdout, stderr and the state of every worktree (index, the bytes of each changed file, `meta.json`, branches). `extract/states.diff` holds the differences of state.
  1. a directory to a new branch
  2. one file copied (`--keep`) to an existing worktree
  3. a rename and a deletion to a new branch from `main`
  4. an existing local branch as target without `--from`
  5. a target whose working copy changed the same file
  6. the same with `--on-conflict resolve`
  7. a target whose branch committed another change to the same lines
  8. a source with nothing to extract
  9. `--output json`
  10. a new target with no `--env-from`, main's `apps/api/.env` now differing from its template (`FROM=main` appended); `10-env-default.env` holds the target's `.env` files
  11. the same with `--env-from main`; `11-env-from-main.env` likewise
  12. an untracked file whose same bytes the target already holds
  13. `--output json` without `--yes`
  14. `--isolation verbatim --ff` with an existing target, `topic`
  15. the same, `--output json`
  16. a new target with the run proxy off (`[proxy] enabled = false` in the global config) while run.toml asks for named addresses, its default
  17. the same, `--output json`
  18. `--dry-run` with a run.toml that does not parse (next only)
- The states are identical for 1, 2, 3, 5, 6 and 8. The differences, all deliberate:
  - 4 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1.
  - 5 is refused by both with exit 15; next's message ends `or extract with conflict markers` where legacy's names `--on-conflict resolve`, and its refusal carries the same request with `on_conflict=resolve` as its follow-up (`extract.resolve`, `error.follow_up` in the JSON), drawn as `→ wtm extract feat/login --files README.md --on-conflict resolve --to topic   write conflict markers in topic` (only the answers given, and the resolve); 7 likewise.
  - 7 is refused by both with exit 15, but legacy leaves conflict markers in `feat/c`'s `README.md` while saying it changed nothing (its 3-way check passes a conflict on git 2.50); next finds the conflict at plan time, so `feat/c` stays untouched. 8 to 11 differ by that file too.
  - 10 gives both the template's `.env` (`env.strategy = "example"`).
  - 11: next copies main's `.env` (`FROM=main`, `env_strategy: main` in its meta) where legacy has no `--env-from` and refuses the flag (exit 2). Next asks a new target its env strategy as create does, with its flag and its remembered answer — a deliberate departure from legacy, whose extract always follows `env.strategy`.
  - 12: both move the file; the states differ only by those of 7 and 11.
  - 13: legacy refuses `--output json` without `--yes` (exit 2); next runs it unattended and creates `feat/jny`.
  - 14 and 15: next warns that the answers an existing target cannot take are ignored, worded by the question (`fast-forward ignored: …`, `isolation verbatim ignored: topic already exists and stays isolated`, as `from ignored` and `env strategy ignored`) rather than by a flag, the isolation's followed by the command that switches it, `→ wtm env topic --isolation verbatim   switch it to verbatim` (`follow_up`, `extract.switch_isolation`, in the JSON); legacy says the same on stderr, by its flags (`--ff ignored: …`, `--isolation verbatim ignored: … — switch it with \`wtm env topic --isolation verbatim\``), as bare strings in its JSON's `warnings`.
  - 16 and 17: as create's 23 and 24, legacy's notice and next's `provision.ports_not_named` warning with its `wtm run proxy status` follow-up.
  - 18: as create's 25, next's recap keeps `Isolation  isolated (run.toml unreadable)`; legacy has no dry run (exit 2).

# Proof: `wtm list` (legacy `surface/cli/wt`) vs `wtm-next list`

The reference is the legacy `wtm list` of 0.30.1. `list` is a read: the proof checks that neither binary changes anything on disk, and keeps both outputs side by side.

- `list.sh <wtm> <wtm-next> list`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`; `feat/a` two commits ahead of main, one ahead of origin, with an untracked file; `feat/b` locked; `feat/gone` whose directory was deleted; `locked-gone`, locked, whose directory was deleted too; a detached worktree; `orphan`, on a branch with no commit; a git repository with no wtm config; a plain directory), the same scenarios run by each binary without a terminal; each records the exit code, stdout and stderr, and the state before and after it (every ref, `git worktree list`, each worktree's status, every `meta.json`, wtm's global directory) into `<side>/unchanged.diff`, empty for both sides. The sandbox's gh is not signed in (its HOME is the sandbox's), so `--with-prs` shows the pull requests unavailable, as they are there. Legacy lists a worktree's jobs in the daemon's order, which varies between runs: its JSON captures sort them. Next's `last_commit_at` is normalised to `<T>` and the run proxy's port in its `urls` to `<PROXY_PORT>`, so a rerun gives the same captures.
  1. the text
  2. `--output json`
  3. `--with-prs`
  4. `--with-prs --output json`
  5. from inside `feat/a`
  6. with `feat/a`'s profile up (`wtm run up feat/a -d`)
  7. the same, `--output json`
  8. `-q`
  9. an argument, `list feat/a`
  10. a repository with no wtm config
  11. the same, `--output json`
  12. a directory outside any repository, `--output json`
  13. after the picker, `feat/a`'s meta record not decoding
  14. the same, `--output json`
  15. `--fetch` with origin out of reach (legacy has no `--fetch`)
  16. the same, `--output json`
  17. `--with-prs` interrupted by SIGINT 2 seconds in, its gh a fake on the `PATH` that answers after 5 seconds
  18. the same, `--output json`
- Then the picker in tmux at 100×30, each side's binary being `wtm` on the shell's `PATH` (`tty-*.txt`): the worktrees once the pull requests had their say, `feat/a` picked and its actions, Go, and a picker left with esc. (1, 5 and 6 show the text; the warnings are in 3.)
- The differences, all approved by the owner:
  - **The JSON is the v2 envelope of a read** (`{v, command, ok, items[{subject, status, detail}], follow_ups, warnings}`, no `op`, `plan` nor `fingerprint`) where legacy prints a bare array (2, 4, 7). The detail renames `is_parent` to `main` (wtm's parent is the branch a worktree was created from), `is_dirty` to `dirty`, `is_locked` to `locked`; `rebase_in_progress` is the `condition` `rebasing`, beside the other operations git stops in the middle of; it adds `missing`, `active` (the worktree the caller stands in, which every surface gives: the command line its working directory, a call its directory, so the catalogue marks it too), and `actions`, every action of the worktree in one order (`go`, `open_pr`, `run_up`, `run_down`, `logs`, `fast_forward`, `sync`, `clean`), each `{action, follow_up, refused}`: offered, its follow-up is a wtm command and its answers (`{command, answers[{path, values}], code, message}`) or, for the pull request, its `url` and `params.number`; refused, `refused` says why (`missing`, `no_branch`, `main_worktree`, `base_branch`, `no_pull_request`, `not_on_origin`: origin has no branch of its name to fast-forward from). The picker offers the ones offered (`tty-2`, Sync among them); `created_at` is `null` without a meta record where legacy made one up from the directory's mtime (the main worktree, always); `services` is sorted, `null` when the run daemon cannot tell; `pr` is `{number, url, state}`, the branch's open pull request, else its newest whatever its state (`open`, `merged`, `closed`), drawn `PR #12` when open and, muted, `PR #9 merged` or `PR #9 closed` otherwise, where legacy read only the open ones; the picker offers to open any of them; the subject is the branch, else the path. A refusal is the envelope on stdout (11, 12) where legacy printed text on stderr and nothing on stdout. In text (10), next says `no wtm config found` followed by its next step `→ wtm init` where legacy said `no wtm config found — run \`wtm init\` first: config file not found`; the JSON carries it as `error.follow_up` (`{command: "init"}`).
  - **What cannot be asked is a warning, exit 0**: `--with-prs` without gh or signed out says ``! pull requests unavailable: run `gh auth login` `` (`list.prs_unavailable`, 3, 4) where legacy said nothing; GitHub is asked of the worktrees' branches, one query per 100 branches, so no listing is capped; with no daemon listening but jobs in its index, `list.jobs_unknown` and `services: null`; `--fetch` with origin out of reach, `list.fetch_failed`, the counts those of the refs last fetched (15, 16; `TestARefreshThatCannotFetchIsAWarning`); a meta record that does not decode, a warning on its item, `list.meta_unreadable`, its worktree listed with `created_at: null` (13, 14; `TestARecordThatDoesNotReadKeepsItsWorktreeAndIsAWarning`). An interrupted read ends cancelled, exit 19, even once every worktree was read: in text the worktrees already read drawn as they are and a closing `✗ interrupted` line, in JSON `ok: false` with `error.kind: "cancelled"`; it warns of none of these (17, 18; `TestAnInterruptedReadWarnsOfNothingItCouldNotAsk`); legacy listed the worktrees without their pull requests, exit 0 (the exit 19 approved by the owner). A read never starts the run daemon, where legacy started one to read its index back. A pull request from a fork is no worktree's, where legacy badged the worktree of the same branch name (a fork's `main`).
  - **The uniform text**: `main` for the main worktree (legacy `(parent)`), `• active` (legacy `● active`, missing from common monospace fonts), `! dirty`, `! rebasing`, `✓ clean` (legacy `⚠`), the columns aligned whatever the glyphs and a column empty in every row left out (legacy pads two columns too far after `●` and `↑`), a worktree whose directory is gone `✗ missing`, locked or not (legacy `✓ clean`), one on no branch named by its path from where the command runs (legacy: an empty name), the warnings under the table (3). The entry of a bare repository, which has no working tree, is not listed (legacy: an unnamed `(parent) ✓ clean` row).
  - **An argument is refused** (9): exit 2, where legacy listed and ignored it.
  - **`--fetch`**, which legacy lacks: the picker's refresh as a field of the request, so a surface other than the command line refreshes through the catalogue's answers alone (the owner's answers-only decision, LUC-355).
  - **The picker** (`tty-*`): opens only when stdin, stdout and stderr are terminals and `--quiet` is off (legacy: stdin alone, so `wtm list | cat` drew its wizard into the pipe); its rows badged as the table's, the worktrees not read and the warnings under its title instead of a box; it lists the worktrees that offer an action: `Go to the worktree`, `Open the pull request` (only for a worktree with one, where legacy offered it to every worktree), `Start the profile`, `Stop the profile`, `View the logs`, `Clean (delete the worktree)` (not for the main worktree; only this for a missing one, by its path on no branch; only Go for one on no branch); the action chosen ends the run on its command line, `→ wtm go feat/a   Go to the worktree`, where legacy ran it (Go printed the path); leaving it exits 19 (legacy 0). `r` fetches origin, as legacy's (`git fetch origin`), and reads every worktree again with its pull request.

# Proof: `wtm status` (legacy `flow/status`) vs `wtm-next status`

The reference is the legacy `wtm status` of 0.30.1. `status` is a read: the proof checks that neither binary changes anything on disk, and keeps both outputs side by side. Next makes it the complete read of one worktree, git and isolation together.

- `status.sh <wtm> <wtm-next> status`: two identical sandboxes (the `docs/demos` "acme" project on fixed ports with a bare `origin` and fixed commit dates; `feat/a` two commits ahead of main, which is two ahead of it, pushed then one more commit, a modified `README.md` and a staged `notes.txt`; `feat/b` without its `apps/web/.env`; `feat/old` whose record lost its isolation, as before the choice existed; `feat/rb` stopped mid-rebase on a conflict with main; a detached worktree; a git repository with no wtm config), the same scenarios run by each binary without a terminal; each records the exit code, stdout and stderr, and the state before and after it (every ref, `git worktree list`, each worktree's status and `.env` files, every `meta.json`, `run.toml`, `config.toml`) into `<side>/unchanged.diff`, empty for both sides. Commit shas, times and ages are normalised; a rerun leaves the captures as they are.
  1. the main worktree
  2. `feat/a`: ahead of and behind main, ahead of origin, uncommitted work
  3. `feat/b`: a missing `.env`
  4. `feat/old`: isolation not chosen
  5. from inside the detached worktree
  6. from inside `feat/rb`, mid-rebase
  7. `--all`
  8. `feat/a --output json`
  9. `--all --output json`
  10. `feat/old --output json`
  11. with `feat/a`'s profile up (`wtm run up feat/a -d`)
  12. the same, `--output json`
  13. `-q`
  14. `feat/a --all`
  15. a name no worktree answers, `status nope`
  16. a repository with no wtm config
  17. a malformed `run.toml` (an unknown key)
  18. `status .` from `feat/a/apps`
  19. `feat/b` once its directory is deleted (after the picker)
  20. `--all` then
  21. next only, its gh a fake on the `PATH` (a fixed answer: an open pull request on `feat/a`, one on `feat/b` merged at its tip; legacy's status reads none): `feat/b --with-prs`
  22. the same, `--output json`
  23. `--all --with-prs`
  24. `feat/a`'s meta record not decoding
  25. the same, `--output json`
  26. `feat/b` locked, its directory deleted
  27. `--all --output json` once the detached worktree, locked, has its directory deleted too: its `status.missing` has `fix: null`
  28. `--all` in a repository with its main worktree alone
  29. next only: `--all --with-prs` interrupted by SIGINT 2 seconds in, its gh a fake on the `PATH` that answers after 5 seconds
  30. the same, `--output json`
- Then the picker in tmux at 100×40, each side's binary being `wtm` on the shell's `PATH` (`tty-*.txt`), from inside `feat/a`: the picker on `feat/a`, enter reads it (1, 2); a picker left with esc (3, 4); then from main, `wtm status feat`, a part of a branch four worktrees hold: next's picker over the four, enter reads `feat/a` (5, 6), legacy's refusal, exit 1 (5).
- Crashed and shared jobs, a closed pull request and the env strategy fixes are not reachable in a sandbox (no crash on demand, no shared job in acme): the use case's tests cover them (`command/status/usecase_test.go`).
- The differences, each an approved deviation or a legacy bug fixed with its test:
  - **The JSON is the v2 envelope of a read** (`{v, command, ok, items[{subject, status, detail}], follow_ups, warnings}`, no `op`, `plan` nor `fingerprint`) where legacy prints a bare document, or a bare array with `--all` (8 to 10, 12). The detail is the whole worktree: `branch`, `path`, `main`, `locked`, `created_at`; `parent` and `children` (the stack); `base {name, ahead, behind}` (new: behind the base too; `null` for the main worktree); `origin {ahead, behind, state}`; `changes {staged, modified, untracked, insertions, deletions, files[{path, status, orig_path}]}` (`staged` counts the tracked files changed in the index, `modified` those changed in the working tree but not staged, a file in both counted in both; legacy's dashboard counted every tracked change as modified, staged ones twice over); the last five `commits[{sha, subject, author, at}]`; `review` with `--with-prs`, the branch's open pull request, else its newest whatever its state: `{number, url, state}`, `state` being `open`, `merged` or `closed`, and for an open one only `title`, `draft`, `checks{passed, failed, pending}` and `decision` too (22); `null` without one or without `--with-prs`; then legacy's isolation part reshaped: `isolation`, `run_toml` (`declared`, `absent`, `invalid`), `run {addressing, offset, ports}` (legacy's top-level `addressing`, `offset` and `run_config`; `ports` is which ports it binds, `base`, `offset`, `source`, `not_numbered`, `not_chosen` or `unknown`, which legacy's text computed), `jobs[{name, kind, state, url, port, exit_code, started_at, shared, owner{branch, path}}]`, `env {declared, files[{target, missing, keys{missing, conflicting, orphan}}]}` (legacy: `{declared, missing[]}`), and `problems[{code, message, fix}]`, whose codes take the command's prefix (`status.merged`, `status.env_missing`, `status.isolation_pending`, `status.job_crashed`) and whose `fix` is a follow-up, a command and its answers, where legacy printed a shell line. `next` has no consumer yet: no envelope version bump.
  - **The text reads the git part too** (2, 6, 11): under the isolation fields (`path`, `state` when rebasing or locked, `isolation`, `addressing`, `ports`, `env`), `base`, `origin`, `changes` with the diffstat, `last commit`, `parent`, `children`, and `review` with `--with-prs` (`#12 Add the login form · 2 passed · 1 pending · review required` when open, `#9 merged` or `#9 closed` otherwise, 21); a running job shows how long it has been up.
  - **A fix is the command line of its follow-up, without `--yes`** (`→ wtm env feat/b`, `→ wtm env feat/old --isolation isolated`, `→ wtm run start feat/a --job api --detach`; legacy: `--yes` and `-d`): the fix is data a surface runs, and the command line prints it to run at a terminal.
  - **A job not running shows the port it binds, not an address** (1 to 4, 6): `port 23470` where legacy showed the proxy url it would answer at; a running job shows the url the daemon gives (11). `port` is the job's url port plus the worktree's offset, the base port for a shared job.
  - **Every worktree is read, a detached one included** (5, 7, 9): bug 6, legacy offered a detached worktree in its picker then refused it (`worktree is on a detached HEAD`, exit 1), and `--all` left it out. It is named by its path (from where the command runs in the table), its picker row says `(detached)`, and it has nothing to fix: every fix names a branch.
  - **A worktree mid-rebase is read on its branch** (6): bug 7, legacy refused it from inside as detached.
  - **A worktree that cannot be read fails alone** (bug 8): with `--all`, one worktree that cannot be read is a failed item (`status.unreadable`), exit 1 after the others are drawn, where legacy failed the whole read; the headline counts it (`✗ … · 1 not read`) and never says `nothing to fix` then. A single worktree that cannot be read prints its error alone, exit 1.
  - **A worktree whose directory is gone is missing, prunable or locked** (19, 20, 26, 27): `missing: true`, nothing of its directory read (`changes`, `commits`, `base` and `env.files` `null`), and one problem, `status.missing` (`its directory is gone`, `→ wtm clean feat/b`), exit 0, the verdict `list` and `tree` give it (`TestAWorktreeWhoseDirectoryIsGoneIsMissingAndToldToBeCleanedLockedOrNot`); one on no branch raises the same problem, its fix naming it by its path (`→ wtm clean /…/detached`), as `list` offers it clean (the captures of `list.sh` and `status.sh` predate it); legacy read it as if it were there, every `.env` missing. git never calls a locked worktree prunable, so the disk is asked. What of a worktree read does not read (its diffstat, commits or base gap, which a branch with no commit cannot tell) is a warning on its item, `status.partial`; a cancel interrupts it.
  - **A worktree that has not chosen its isolation has `isolation: null`** (10): bug 1, legacy's JSON said `isolated`.
  - **`run` reflects `run.toml`**: bug 2, legacy said `no run.toml` for a file declaring no job or profile; next shows its addressing whatever it declares. A malformed `run.toml` (17) is a warning, `status.run_invalid`, exit 0, `run: null`, where legacy failed (bug 3, exit 1, while other commands swallowed it).
  - **Only an unnumbered worktree has no offset** (bug 4): legacy showed `not numbered` for any error reading the worktree's env; next only for an ordinal not allocated or held first by another branch. A record that does not read is a warning on its item (`status.meta_unreadable`: its isolation, parent, env strategy and offset are not known), its isolation `null` and its ports `unknown`, and it raises no isolation problem, whose fix would write over a record it could not read (24, 25; legacy: `isolation not chosen` and the fix that rewrites it).
  - **A held shared job reads as its owner's instance, once** (bugs 10, 13): its url the owner's instance's, one job per name; **a crashed shared job's fix starts it where it runs** (bug 11: legacy reported it on every holder with a fix targeting the holder).
  - **A missing `.env` under the `example` strategy with no template but a copy in main is rebuilt `--from main`** (bug 12: legacy sent it to `wtm config edit`). A missing file is rebuilt from the worktree's own strategy's source (main's copy for `main`; the parent's, else main's, for `parent`; the template for `example`), else `--from example` when a template exists, else `--from main`, else `wtm config edit`. The template is looked for in the worktree itself, as legacy and the `wtm env` its fix runs read it; the key drift is measured against it too.
  - **Key drift is counted** (`env.files[].keys`): missing keys no source gives a value, keys whose value differs from the strategy's source (`conflicting`), and orphan keys no source has; keys wtm writes (the `WTM_*` variables, `COMPOSE_PROJECT_NAME`, the keys `run.toml` links) are never drift. The keys `[[env_port]]` links are among them: wtm rewrites them per worktree, so they are never counted as conflicting, where legacy's `wtm env --check` compared them once their port offset was taken off. `keys.missing` counts only the keys no source can fill, the parent's or the main's copy having no value for them: a key a source fills is what `wtm env` adds, not a gap. Legacy's status only said which files are missing; its dashboard counted the same three in add mode, where nothing conflicts.
  - **What cannot be asked is a warning, exit 0**: with no daemon listening but jobs in its index, `status.jobs_unknown` and `jobs: null`, where legacy read the index and checked each process itself; `--with-prs` without gh or signed out, `status.prs_unavailable`, the pull requests found before a query failed kept; a base branch that does not exist, `status.base_missing`. A read never starts the run daemon. An interrupted read ends cancelled, exit 19, even once every worktree was read: in text the worktrees already read drawn as they are and a closing `✗ interrupted` line, in JSON `ok: false` with `error.kind: "cancelled"`; it warns of neither `status.jobs_unknown` nor `status.prs_unavailable` (29, 30; `TestAnInterruptedReadWarnsOfNothingItCouldNotAsk`), the policy of every read.
  - **A worktree is named by its branch first**, then by a part of its branch found in no other, then by a path, resolved through its symbolic links from where the command runs, that it holds (`status .` from a subdirectory reads the worktree holding it, 18; legacy: `worktree "." not found`, exit 1); a name that starts with a dot or a slash, as no branch does, is a path only. A part of a branch comes before a directory of the same name, so `status docs` standing beside `docs/` reads `feat/docs-site` (approved; `TestAQueryNamesTheWorktreeOnThatBranchElseEachBranchHoldingItElseThePathItStandsFor`). A repository with no worktree to read (a bare clone) is refused, `status.no_worktree`, exit 25, unless `--all`, which reads none.
  - **Exit codes follow next's uniform system**: `--all` with a worktree (14) and a name no worktree answers (15) are invalid requests, exit 2 (legacy: 2 and 1), each naming its field; no wtm config (16) exits 12 as legacy, its message without legacy's repeated cause.
  - **The picker** (`tty-*`): the same step as legacy's, titled `Worktree` with the field's help (legacy: `Select worktree`, `Which worktree to act on?`), the detached worktree offered, `main` badged on the main worktree beside legacy's `current` on the one you are in; leaving it prints `✗ interrupted` (legacy `= Aborted.`), both exit 19. It opens only when stdin, stdout and stderr are terminals, the output human and `--quiet` off; anywhere else the worktree you are in is read (legacy parity). There is no `--yes` (legacy's skipped the picker): a read takes none, and without a terminal nothing is asked.
  - **`--all`** draws the table however many worktrees the repository has, one included, as legacy (28; bug fixed: next drew the one-worktree layout, `TestAllDrawsTheTableEvenOfOneWorktree`); it counts the detached worktree (`6 worktrees`, legacy `5`); with `--with-prs` its table gains a `REVIEW` column (the number, the failed checks else the pending ones, the decision; `#9 merged` or `#9 closed` for one no longer open, 23) when a worktree has a pull request.
  - **A merged pull request is a problem, the first** (`status.merged`, `its pull request is merged`, `→ wtm clean feat/b`; 21 to 23), since cleaning the worktree makes every other fix moot; only while the worktree's branch stands at the commit the pull request merged (its `headRefOid`) or behind it (`git merge-base --is-ancestor <tip> <head>`, asked only when the tip is not the head), so a branch name reused or moved on since is not taken for merged, and one behind its pull request's head is; a head never fetched cannot be told behind, so it offers nothing; never on the main worktree nor the base branch, which outlive any pull request (the rule `tree` follows to offer its clean); a closed one is no problem. A branch with an open pull request and a newer closed one reads the open one. Legacy's status read no pull request.
  - **No sync is offered in the middle of an operation git stopped in** (6, 7, 20): a worktree behind its parent is told `! main has 2 commits it lacks → wtm sync <branch>` (`status.behind`), unless git stopped in the middle of a rebase, a bisect, a merge, a cherry-pick, a revert or a git am in it, which leaves its branch where it was; its state line says which (`state rebasing`, `merging`, `bisecting`, `cherry-picking`, `reverting`, `applying patches`), its `condition` the same (`rebasing`, `bisecting`, `merging`, `cherry_picking`, `reverting`, `applying`), and `list` and `tree` give it `needs_sync: false` (`TestAWorktreeBehindItsParentNeedsSyncUnlessGitStoppedInTheMiddleOfAnOperation`, `TestAWorktreeBehindItsParentIsToldToSyncOntoItUnlessGitStoppedInTheMiddleOfAnOperation`, `TestAnOperationGitStoppedInTheMiddleOfIsTheWorktreesStateAsARebaseIs`). git's marks of each are read per worktree, `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-apply/applying`; a worktree that cannot be probed is listed in the middle of nothing (`TestWorktreesReadsTheOperationGitStoppedInTheMiddleOf`, `TestAWorktreeThatCannotBeProbedIsListedInTheMiddleOfNothing`).
  - **A check whose status is `EXPECTED` is pending**, not failed as legacy's unknown values are: it waits for a status to report.
  - **The picker reads the questions only when it asks**: with a worktree named or `--all`, nothing is read before the read itself.
  - **An ambiguous name is picked among its matches on a terminal** (approved deviation, shared with `resolve`): a part of a branch several worktrees hold opens the picker over them, where legacy refused it; anywhere else it is refused, exit 2, its field code `ambiguous` (shared with `resolve`, was `status.ambiguous`) naming the matches (`TestOnATerminalAnAmbiguousNameIsPickedAmongTheWorktreesItMatches`).
  - **The other legacy bugs of the issue**: 5, the stale comment on a constant, has nothing to fix in next, whose status declares no such constant; 9, legacy listing the worktrees with `git worktree list` once per question, is one snapshot read once per run here.

# Proof: `wtm resolve` and the `wtm go` wrapper (legacy `surface/cli/resolve`) vs `wtm-next resolve`

The reference is the legacy `wtm resolve` of 0.30.1 and the `wtm go` function `wtm shell-init` prints, which runs `command wtm resolve "${@:2}"` and cds into what it prints. `resolve` is a read: the proof checks that neither binary changes anything on disk, keeps both outputs side by side, and enters worktrees through the wrapper for real.

- `resolve.sh <wtm> <wtm-next> resolve`: two identical sandboxes (the `docs/demos` "acme" project with `feat/login`, `feat/logout` and `fix/nav`; `feat/gone`, whose directory was deleted; a detached worktree; a git repository with no wtm config; a plain directory), the same scenarios run by each binary without a terminal; each records the exit code, stdout and stderr, and the state before and after it (every ref, `git worktree list`, each worktree's status, every `meta.json`, wtm's global directory) into `<side>/unchanged.diff`, empty for both sides.
  1. `feat/login`
  2. the same, `--output json`
  3. a part of a branch no other worktree holds, `logi`
  4. the same, `--output json`
  5. `main`, from inside `fix/nav`
  6. a part two worktrees hold, `feat/log`
  7. the same, `--output json`
  8. a name no worktree answers, `nope`
  9. the same, `--output json`
  10. no name, from inside `fix/nav`
  11. the same, `--output json`
  12. `feat/login -q`
  13. `nope -q`
  14. the detached worktree by its path
  15. two words, `feat login`
  16. `feat/gone`
  17. the same, `--output json`
  18. a repository with no wtm config
  19. the same, `--output json`
  20. a plain directory, `--output json`
- Then in tmux at 100×30, each side's binary being `wtm` on the shell's `PATH` (`tty-*.txt`): `dir=$(wtm resolve feat/log)`, stdout captured as the wrapper captures it, opens the picker over the two matches, enter prints the first (1, 2); `dir=$(wtm resolve)` from `fix/nav` opens it over every worktree, left with esc (3, 4); then the wrapper as `wtm shell-init zsh` prints it, sourced: `wtm go feat/login; pwd` enters it (5), `wtm go nope` stays where it is (6), `wtm go log` opens the picker, down and enter enters `feat/logout` (7, 8).
- The wrapper's contract is unchanged: the path alone on stdout and exit 0 when a worktree is resolved, nothing on stdout otherwise; the picker draws on stderr and opens while stdout is captured. `wtm shell-init` (LUC-336) ships the same function over `wtm resolve`; a `wtm go` run without it (legacy: a hint to install it) ships with `shell-init`.
- The differences, each an approved deviation or a legacy bug fixed with its test:
  - **The JSON is the v2 envelope of a read** (2, 4, 11, 17): one item whose detail is `{branch, path, missing}` where legacy printed `{path, branch}`; `missing` is the read policy for a worktree whose directory is gone (17), whose path is still printed (16), as legacy, so `cd` fails loudly. `next` has no consumer yet: no envelope version bump.
  - **A name no worktree answers is refused, exit 2, nothing on stdout** (8, 9, 13): bug 1, legacy warned on stderr and exited 0, so `cd "$(wtm resolve nope)"` went home, and `-q` did not silence the warning (`TestARefusalLeavesStdoutEmptyAndExitsUsage`).
  - **A failure with `--output json` is the envelope on stdout** (7, 9, 19, 20): bug 2, legacy printed framed text on stderr and no document (`TestARefusalWithJSONIsTheEnvelopeOnStdout`).
  - **An ambiguous name is picked among its matches on a terminal, refused anywhere else** (6, 7, `tty-1`, `tty-7`): bug 3, legacy tried the picker without a terminal (`could not open a new TTY`, exit 1) and, under JSON, refused `"" is ambiguous` without naming the matches; next refuses it, exit 2, field code `ambiguous` naming them (`TestOnATerminalWithStdoutCapturedTheWorktreeIsPicked`, `TestANameMatchingNoWorktreeOrSeveralIsRefused`). The picker is next's: titled `Worktree` with the field's help (legacy: `Select a worktree`, with each row's status and a GitHub banner), `main` and `current` badges, the detached worktree by its path.
  - **No name resolves the worktree you are in, unless a terminal asks** (10, 11, `tty-3`): legacy tried the picker without a terminal and failed; next resolves the worktree holding the working directory, else the main one, as `status` reads (`TestWithNoWorktreeNamedAndNoTerminalTheOneYouAreInIsResolved`). On a terminal the picker opens with the cursor on the worktree you are in: bug 7, legacy's cursor stayed on main (`TestOnATerminalWithStdoutCapturedTheWorktreeIsPicked`, `TestThePickerOffersEveryWorktreeTheCursorOnTheOneYouAreIn`).
  - **A worktree on no branch is resolved by its path** (14): bug 4, legacy could not name it, and listed it in its picker as a blank row (`TestAWorktreeOnNoBranchIsResolvedByItsPath`). A name is a branch first, then a part of a branch found in no other, then a path, read from where the command runs, that a worktree holds, as `status`; a name that starts with a dot or a slash is a path only. Standing in main beside its `docs/`, `wtm go docs` enters `feat/docs-site`, as legacy did (approved; `TestAPartOfABranchWinsOverADirectoryOfTheSameName`).
  - **One positional at most** (15): bug 5, legacy joined two words with a space and looked `feat login` up, which matches nothing; next refuses the second argument, exit 2.
  - **Leaving the picker is cancelled, exit 19** (`tty-4`): bug 6, legacy exited 0 with nothing printed; stdout stays empty, so the wrapper does not cd either way (`TestThePickerOffersEveryWorktreeAndLeavingItCancels`).
  - **A repository with no wtm config is refused** (18, 19): `resolve.config_not_found`, exit 12, its follow-up `wtm init`, as every read of next; legacy resolved there.
  - **`-q` still prints the path** (12), as legacy: the path is data a shell reads (`Query.Captured`), never silenced.
  - **A follow-up that enters a worktree names `go`**, the shell function over `resolve`, with resolve's answers (`TestEveryGoFollowUpIsARequestResolveTakes`): the command line spells it `wtm go feat/x`, a worktree on no branch `cd <path>`.
  - **The web** needs nothing of `resolve`: `list` and `status` give each worktree's path; `resolve` is in the catalogue all the same.

# Proof: `wtm tree` (legacy `surface/cli/wt`) vs `wtm-next tree`

The reference is the legacy `wtm tree` of 0.30.1. `tree` is a read: the proof checks that neither binary changes anything on disk, and keeps both outputs side by side.

- `tree.sh <wtm> <wtm-next> tree`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`; a stack of three levels, `feat/a` two commits past main and pushed, with an untracked file, `feat/a-ui` on it, two commits past it, `feat/a-ui-form` on that, which lacks `feat/a-ui`'s last commit; a fan-out under main, `feat/b`, locked; `feat/r` mid-rebase on a conflict; main one commit ahead of what its children started from; `feat/login-ui` whose parent `feat/login` was cleaned; `cyc-1` and `cyc-2`, each recorded as the other's source; `feat/gone`, whose directory was deleted; a detached worktree; a git repository with no wtm config; a plain directory), the same scenarios run by each binary without a terminal; each records the exit code, stdout and stderr, and the state before and after it into `<side>/unchanged.diff`, empty for both sides. The sandbox's gh is not signed in, so `--with-prs` shows the pull requests unavailable.
  1. the text
  2. `--output json`
  3. `--output mermaid`
  4. `--with-prs`
  5. `--with-prs --output json`
  6. from inside `feat/a`
  7. with `feat/a`'s profile up (`wtm run up feat/a -d`)
  8. the same, `--output json`
  9. `-q`
  10. `-q --output mermaid`
  11. an argument, `tree feat/a`
  12. an unknown format, `--output dot`
  13. a repository with no wtm config
  14. the same, `--output json`
  15. a directory outside any repository, `--output json`
  16. `feat/a-ui`'s meta record not decoding
  17. the same, `--output json`
  18. `feat/gone` locked
  19. `--with-prs` interrupted by SIGINT 2 seconds in, its gh a fake on the `PATH` that answers after 5 seconds
  20. the same, `--output json`
- The differences, each approved by the owner or a legacy bug fixed with its test:
  - **The JSON is the v2 envelope of a read** (2, 5, 8): one item per node, depth first (roots main first, then by creation, the virtual ones by name; children by creation then name), its subject the branch, its detail `{branch, path, main, virtual, parent, children, ahead, behind, needs_sync, dirty, locked, missing, condition, cycle, origin, pr, services, actions}` (a rebase in progress, or any operation git stopped in the middle of, is its `condition`), where legacy nests `{roots[{branch, path, is_main, is_virtual, status{…}, children[…]}]}`; `path` and `parent` are `null` for a virtual root and a root, `origin` is `{ahead, behind, state}` with the state spelled (`"ahead"`, legacy `origin_state: 3`), `null` when origin has no such branch. A refusal is the envelope on stdout (14, 15) where legacy printed text on stderr. In text (13), next says `no wtm config found` followed by its next step `→ wtm init` where legacy said `no wtm config found — run \`wtm init\` first: config file not found`; the JSON carries it as `error.follow_up` (`{command: "init"}`).
  - **Ahead and behind are counted against the parent** (bug fixed, `TestAheadAndBehindAreCountedAgainstTheParent`): `↑N` is the commits a branch has that its parent lacks, the base branch's for a root; `feat/a-ui` reads `↑2` where legacy said `base ↑4`, counting `feat/a`'s commits too, and `feat/a-ui-form` nothing where legacy said `base ↑3`. `behind`, the parent's commits the branch lacks, is what `needs sync` says. What is not counted is `null`, where legacy said `0`: the main worktree, a child whose parent is not a local branch (a virtual root, `origin/x`), a member of a cycle (it has no parent to sync onto); none of them needs sync, as in legacy. A root has no parent: its `ahead` is counted against the base branch (a worktree wtm did not create, say), its `behind` is `null`, and it never needs sync, as in legacy (`TestARootBehindTheBaseBranchIsNotFlagged`). A child is counted in one git process, `rev-list --left-right --count <parent>...<branch>`, where legacy ran a tip check and an ancestry check; `tree.base_missing` warns when the base branch does not exist. The label is `↑N` (legacy `base ↑N`: `base` names the base branch in `list`, one word one meaning).
  - **A worktree git stopped in the middle of an operation needs no sync** (1, 2, 18): `feat/r`, mid-rebase, reads `! rebasing` alone and `needs_sync: false`, where legacy said `⚠ rebasing  ⚠ needs sync`. A bisect, a merge, a cherry-pick, a revert or a git am is drawn the same way, `! merging` (`TestAWorktreeGitStoppedInTheMiddleOfAnOperationNeedsNoSync`).
  - **Each node carries its worktree's actions, in the JSON** (`actions`, the text prints none): the same `{action, follow_up, refused}` list as `list`'s, built by the same rule; none for a virtual root or a worktree git could not read. What a worktree needs first is status's problems, not the tree's.
  - **`--with-prs` reads the pull request of each branch, its open one else its newest in any state** (4, 5), `PR #N`, `PR #N merged`, `PR #N closed`, `pr: {number, url, state}`; gh missing, signed out or failing is a warning, ``! pull requests unavailable: run `gh auth login` `` (`tree.prs_unavailable`), exit 0, where legacy said nothing (bug fixed, `TestWhatCannotBeAskedIsAWarningNotAFailure`); a failing batch keeps the pull requests of the batches before it, where legacy dropped them all (`TestAFailingQueryKeepsThePullRequestsFoundBeforeIt`); a fork's pull request is no branch's, where legacy matched it by name (`TestTheNewestPullRequestOfEachBranchIsReadWhateverItsState`).
  - **The jobs up are shown** (7, 8): `services api, web` on the row (list's word, with the jobs' names), `services: ["api", "web"]` in the JSON, `null` with `tree.jobs_unknown` when the daemon cannot tell; legacy documented `▶ N running` and never filled it (`TestTheJobsUpInAWorktreeAreItsServices`). A read never starts the daemon.
  - **The uniform text**: `! dirty`, `! rebasing`, `! needs sync`, `! cycle`, `! locked` (legacy `⚠`, missing from common monospace fonts), the glyph alone coloured; a worktree whose directory is gone is `✗ missing`, locked or not (18), where legacy drew it clean (bug fixed, `TestAWorktreeWhoseDirectoryIsGoneIsMissingNotClean`); a worktree that cannot be read keeps its place, `✗ unreadable`, its error under the tree (`TestAWorktreeThatCannotBeReadFailsAloneKeepingItsPlace`); the warnings under the tree. The layout is legacy's: two-space indent, `├─ ` / `└─ `, `│  ` gutter, badges aligned two columns after the widest branch.
  - **A detached worktree is no node** (1, 3): it has no branch, so no place in a stack; legacy drew each as an empty root flagged `⚠ cycle`, and its Mermaid node `[" ⚠ cycle"]` had no id (bug fixed, `TestADetachedWorktreeIsNoNodeOfTheTree`).
  - **`--output mermaid` gives each branch its own node** (3, 10): ids by place (`n0`, `n1`…) where legacy's `feat/a`, `feat-a` and `feat.a` all became `feat_a` and a branch `end` broke the chart; `#`, quotes, angle brackets and backticks in a label are entities (`PR #35;12` renders `PR #12`); the warnings follow as `%%` comments (`TestMermaidGivesEachBranchItsOwnNode`). It is unframed and `-q` leaves it, as legacy.
  - **An argument is refused** (11): exit 2, where legacy drew the tree and ignored it. An unknown format (12) is refused by both, exit 2; next adds the way to `--help`.
  - **Each worktree is read once**: legacy listed the worktrees and read each meta record twice (`TestAReadChangesNothingAndReadsEachWorktreeOnce`). Whether a worktree is there and dirty, and the bounded reader that reads eight at once, are list's, shared through `service/repository` (`Present`, `Each`); each command keeps its own counting.
  - **A worktree not read is said so**: a meta record that does not decode is a warning on its node, `tree.meta_unreadable` (its parent is not known, so it stands as a root), the node read all the same (16, 17; `TestARecordThatDoesNotReadKeepsItsNodeAsARootAndIsAWarning`), the policy of every read; an interrupted read marks the nodes it interrupted `✗ interrupted`, cancelled, and ends cancelled, exit 19, never 0 with null counts, even once every node was read: in text the nodes already read drawn as they are and a closing `✗ interrupted` line (`%% ✗ interrupted` under `--output mermaid`), in JSON `ok: false` with `error.kind: "cancelled"` (19, 20, where every node was read: they stay done; legacy `= Aborted.`, exit 19; drawing what was read approved by the owner), and warns of nothing it could not ask (`TestACountInterruptedMakesItsNodeCancelled`, `TestAnInterruptedReadWarnsOfNothingItCouldNotAsk`); under `--output mermaid` the cause is a `%% ✗` comment (`TestAWorktreeNotReadIsDrawnSoInTextAndMermaid`).
  - **`clean` is never offered for the main worktree nor the base branch**, even with a merged pull request, and only while the worktree's branch stands at the commit the pull request merged (its `headRefOid`) or behind it (`git merge-base --is-ancestor`, asked only when the tip is not the head): a branch name reused or moved on since is not taken for merged, one behind its pull request's head is, and one whose head was never fetched is not; the rule `status.merged` follows.
  - Not fixed, legacy being frozen: legacy's `using-wtm/references/stacks.md` documents list's `origin` object for tree, and the guide's badge table describes `base ↑N` against the base branch and `▶ N running`.

# Proof: `wtm events` (legacy `surface/cli/events`) vs `wtm-next events`

The reference is the legacy `wtm events` of 0.30.1. `events` is a stream: the proof starts it in the background, reads it to its ready line, makes a change from another process of the same binary (a `create`), then interrupts it with SIGINT, as Ctrl+C does.

- `events.sh <wtm> <wtm-next> events`: two identical sandboxes (the `docs/demos` "acme" project with `feat/a`, a git repository with no wtm config, a plain directory), each side with its own HOME, so each binary starts and talks to its own daemon: legacy's run daemon, next's `wtm-next daemon`. `repos.json` is removed before the scenarios (`registry-before`) and recorded after them (`registry-after`). Times, op ids and sandbox paths are normalised (`<T>`, `<ID>`, `<SB>`).
  1. the stream of the current repository, as text, with a `create feat/text` in another process, then SIGINT
  2. the same, `--output json`, with `create feat/json`
  3. `--all --output json` from a directory outside any repository, with `create feat/all`
  4. `--output json` outside any repository, without `--all`: every repository all the same
  5. `--repo <acme> -q` from outside any repository
  6. `--output json | head -n1`, exit status of the stream
  7. `--all --repo`
  8. `--repo` naming no directory
  9. `--repo` naming a directory in no repository
  10. a repository with no wtm config
  11. the same, `--output json`
  12. `--output json`, its stdout a socket whose reader leaves after the ready line (`12-socket-reader-gone.stdout` says whether the stream ended within 3 seconds)
  13. `--all --output json` from outside any repository while a repository is initialized and a command runs in it (`repo.added`, then its snapshot), then de-initialized and another one registered, whose registration prunes it (`repo.removed`)
  14. `--repo acme --output json` from the sandbox's root, `$WTM_PROJECT_DIR` naming the plain directory: a relative `--repo` is read from the working directory, as a surface reads it from its call's directory; legacy's stream ends on its error before any ready line, which the script records without waiting for it
- The differences, all approved by the owner:
  - **The stream is v2** (`"v":2`): every field of an event flat beside the envelope (`command`, `surface`, `ok`, `code`, `subject`, `status` where v1 nested `operation` and `unit`), `op_id` on every line of an operation, and `operation.started`, `unit.started`, `unit.finished` and `operation.finished` on the stream (2, 3), which legacy's create never published. A snapshot's worktrees carry no `jobs` until the run module migrates (legacy: `jobs`, `[]` or `null`). The schema is `next/internal/adapter/events/events.v2.json`, open to types it does not list; legacy's v1 refused them. Whichever daemon holds the socket is the bus: a legacy daemon there hands next v1 lines, which next drops with a warning, never translated.
  - **next has its own bus**: `wtm-next events` starts the hidden `wtm-next daemon`, which hosts the event bus alone, on the socket and lock legacy's run daemon uses (`<UserConfigDir>/wtm/wtm.sock`, `wtm.lock`); the two cannot run at once on one machine, and a legacy `wtm ui` or `wtm events` does not see next's events. It idles out 30 seconds after its last subscriber and request. While it runs, `wtm-next list` cannot ask it for jobs it does not host: `list.jobs_unknown` (`run daemon: unknown action: list`), services `null`, until the run module joins it.
  - **A refusal before the stream is next's envelope** on stdout with `--output json` (11) where legacy printed text on stderr and nothing on stdout; an invalid request names its field (`--repo`) in next's words (7, 8, 9). The exit codes are legacy's: 2, 2, 21, 12.
  - **The text**: `✓ updated feat/text  ordinal=2` where legacy drew `~` (next's kit has no update glyph: a change made is `✓`); `no longer watching <root>` is a `!` warning. Operation and unit events show nothing to a person, as legacy (whose stream had none).
  - **A working directory that cannot be read (removed under the shell) is refused** without `$WTM_PROJECT_DIR`, exit 25 (`cli.no_working_directory`), for every `wtm-next` command: next cannot tell where its caller stands, and `events` would otherwise widen to every repository. Legacy ran `events --all` and `events --repo <absolute>` from there (`TestACommandLineThatCannotReadItsWorkingDirectoryIsRefused`).
  - **The registry**: every `wtm-next` command that opens an initialized repository adds it to `repos.json` (`registry-after`), in legacy's format, one with no wtm config is never added, so a global stream never sees it join; `repo.added` carries no `correlation_id` (legacy's did). The registry is left alone while the state directory is moved (`$WTM_STATE_DIR`): no repository joins it, as legacy, none leaves it, and a global stream lists none of it, nor joins one another process announces on the bus, rather than read each repository with the moved state's config and records, where legacy still pruned by each repository's own `.git/wtm/config.toml`, a second definition of "initialized" that a moved state makes meaningless.
- The legacy bugs fixed, each with its test:
  - **A stdout whose reader left killed the stream with SIGPIPE** (exit 141) when a write came before its 250 ms check: a write now fails with EPIPE and the stream exits 0 (`ui/cli` `TestAWriteToAStdoutWhoseReaderLeftFailsRatherThanKillTheProcess`). In 6 both exit 0: here the check wins the race.
  - **A reader gone was noticed on a pipe only**: a socket stdout (a Node child's stdio) kept legacy streaming forever, still running in 12; next ends at once (`ui/output` `TestTheReaderOfASocketLeavingIsNoticed`).
  - **`-q` silenced the warnings too**: they stay on stderr, only the events are silenced (5; `TestQuietSilencesTheEventsButNotTheWarnings`).
  - **A line the bus carried that could not be decoded was dropped silently**: a warning names it (`TestALineThatCannotBeReadIsDroppedWithAWarning`).
  - **The v1 schema refused a type it did not list** (`TestALineOfATypeTheSchemaDoesNotListIsValidAgainstTheEnvelope`).
  - **A stream never noticed its repository losing its wtm config**: it now ends with exit 12, checked every 30 seconds and on its own `repo.removed` (`TestAStreamEndsWhenItsRepositoryLosesItsConfig`, `TestAStreamEndsWhenItsRepositoryIsAnnouncedRemovedAndItsConfigIsGone`).
  - **`--repo` with `$WTM_PROJECT_DIR` subscribed to one repository and read the other's metadata**: `--repo` wins (`TestTheRepositoryNamedByRepoWinsOverTheDirectory`).
  - **A relative `--repo` with `$WTM_PROJECT_DIR` naming a directory in no repository failed** (`✗ git rev-parse --git-common-dir: exit status 128`, exit 1, 14): it is read from the working directory and streams that repository, as a surface reads it from its call's directory (`cmd/wtm-next` `TestARelativeRepoOnTheCommandLineIsReadFromTheWorkingDirectory`, `TestARelativeRepoIsReadFromWhereTheCallerStands`).
  - **An inherited `$GIT_DIR` redirected the per-repository stream's git calls**: the designated repository wins, and `--all` no longer unsets the process's variables (`cmd/wtm-next` `TestTheEventStreamOfASurfaceOpensOnTheCallsRepositoryWhateverGitDirSays`).
  - **An interrupt before the stream starts exits 19**, now documented in `--help` (`TestAnInterruptBeforeTheStreamStartsIsAnInterruption`); after its first line, 0 (1, 2, 3).
  - **A repository registered between the subscription and the first prune got a second snapshot** (`TestARepositoryAlreadySnapshottedIsNotSentAgainWhenItsJoiningArrives`).
  - **The global stream relayed the events of a repository it had sent no snapshot of** (`TestTheGlobalStreamDropsTheEventsOfARepositoryItSentNoSnapshotOf`).
  - **The first wait before subscribing again was 400 ms**, now 200 ms, doubling to 5 s (`TestTheFirstWaitBeforeSubscribingAgainIsTheShortestThenItDoublesUpToALimit`).
  - **`--help` did not list what the stream carries**: it names every type (`events_help.golden`).

# Proof: `wtm clean` (legacy `flow/clean`) vs `wtm-next clean`

The reference is the legacy `wtm clean` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it acts and refuses an unsafe worktree with exit 23), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `clean.sh <wtm> <wtm-next> clean`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin` and an `on_clean` hook that fails on `feat/bad*`; worktrees `feat/plain`, `feat/dirty` with an untracked file, `feat/unpushed` pushed then one commit more, `feat/local` holding a commit no other branch holds, `feat/locked`, `feat/parent` with `feat/child` and `feat/child2`, `feat/p2` with `feat/c2`, `feat/c` with an untracked file, `feat/detached` on no branch, and one worktree per other scenario), the same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (branches, worktrees and their locks, every `meta.json`, `repos.json`). Paths, op ids and fingerprints are normalised. After 7, 16, 21, 22 and 24, the worktree each left apart on purpose is removed on both sides, so each difference of state stays its own scenario's. `clean/states.diff` holds them. Then, in a fresh sandbox per side, the wizard in tmux at 100×40: `clean/tty/legacy/` and `clean/tty/next/`.
  1. a worktree, `--yes`
  2. a dirty worktree, `--yes`
  3. the same, `--force --yes`
  4. a worktree with an unpushed commit, `--yes`
  5. a locked worktree, `--yes`
  6. the same, `--force --yes`
  7. a branch holding a commit no other branch nor remote holds, never pushed, `--yes`
  8. a parent, its two children left orphaned
  9. a parent, `--reparent-children`
  10. two worktrees, one dirty, `--yes`
  11. two worktrees, `--yes`
  12. a name that is no worktree
  13. `main`
  14. a worktree whose `on_clean` hook fails
  15. `--output json --yes`
  16. `--output json` without `--yes`
  17. a dirty worktree, `--output json --yes`
  18. no worktree, without a terminal
  19. `-q`
  20. `--dry-run`
  21. `--keep-data --yes`
  22. a worktree on no branch, by its path
  23. `--dry-run --output json` (next only)
  24. `--expect-fingerprint` with that fingerprint (next only)
  25. `--expect-fingerprint` with a stale fingerprint (next only)
  26. a branch no worktree holds, as create's undo can leave one
- The states are identical but for 7, 16, 21, 22, 24 and 26. The differences, all deliberate:
  - 2, 4, 5, 10 and 17 are refused by both, nothing removed; next exits 23 (`clean.unsafe`, a safety refusal) where legacy exits 1, names what it refuses (`nothing removed: feat/dirty can't be removed safely`) and each reason on its line under the failure (`feat/dirty has uncommitted changes`, `feat/unpushed has 1 commit no other branch nor remote holds`, `feat/locked is locked`), and follows it with the same request forced, `→ wtm-next clean feat/dirty --force   remove anyway`. In JSON (17) each reason is a problem of its entry, `error.fields[]` (`worktree[0]`, `clean.dirty`), and the forced request is `error.follow_up` (`clean.force`); legacy prints its refusal as text on stderr and nothing on stdout.
  - The sandbox has no `gh`: next asks the forge about every branch it removes (a pull request squash-merged then deleted on origin holds its commits), so each such removal warns `clean.prs_unavailable`, legacy nothing.
  - 7, a legacy bug: legacy counts unpushed commits against `origin/<branch>`, none for a branch origin never had, so it ran the hook and removed the worktree, then `git branch -d` refused the unmerged branch: the worktree gone, its branch and `meta.json` left behind, exit 1. Next counts the commits no other branch nor remote holds, the branch's and those of the commit the worktree stands on (a commit on no branch, a rebase under way, its directory gone or not), and refuses the worktree before anything runs (exit 23); a branch whose merged pull request's head is its tip and the worktree's commit loses nothing, which GitHub keeps. Once the checks pass, next deletes the branch with `-D`, so a branch pushed but not merged locally goes whole too (legacy's `-d` refused it the same way).
  - 12: both report the name as already absent, exit 0; next counts it `1 unchanged` with an `Already absent` row.
  - 13: legacy warns `Cannot clean the parent worktree.` and exits 0; next refuses the entry, exit 2 (`clean.main_worktree`).
  - 14: both keep the worktree, exit 1; the hook view is next's uniform `on_clean · <branch>`.
  - 16: legacy refuses `--output json` without `--yes` (exit 2); next runs it unattended, as every next command.
  - 18: legacy exits 1; next exits 2, naming `<worktree>` as required.
  - 20 and 23 to 25: legacy has no dry run nor fingerprint (exit 2).
  - 21: next has no `--keep-data` nor `--drop-data` until the run module migrates (LUC-339): it never drops a namespace and names the data kept (`clean.data_kept`); it refuses the flag (exit 2).
  - 22: legacy takes only branches and says the path is already absent; next removes the worktree on no branch by its path, leaving the branch it was made from.
  - 26, an owner decision: legacy says the branch is already absent and keeps it, so create's `→ wtm clean feat/x --force` repaired nothing after an undo that left the branch; next deletes a branch no worktree holds, with its record, behind the same checks (refused when it holds commits no other branch nor remote holds, unless `--force`).
  - The tty: legacy's recap offers to delete the safe ones, to force all, or to cancel; next asks a `Force` step only when a worktree is unsafe, its help naming each reason (`feat/tty-dirty — uncommitted changes`), then a recap whose button names what goes (`Yes, remove 2 worktrees`). Next's `Reparent children` puts `Leave them orphaned` first (no by default), legacy `Reparent onto main (1)`. Leaving the wizard exits 19 on both.

# Proof: `wtm prune` (legacy `flow/prune`) vs `wtm-next prune`

The reference is the legacy `wtm prune` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it acts, keeps an unsafe worktree it was not asked for and refuses one it was, exit 23), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `prune.sh <wtm> <wtm-next> prune`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`, an `on_clean` hook that fails on `feat/bad*`, and a fake `gh`, signed in, that answers both binaries' query from the sandbox's `prs.json`: legacy's with its `--jq` applied, next's as GitHub's GraphQL answer). Its worktrees, each pushed: `feat/merged` (pull request 1 merged at its tip) with `feat/child` made from it, `feat/moved` (merged, one commit since), `feat/closed` (pull request 3 closed at its tip), `feat/gone` (origin deleted its branch), `feat/gone-work` (origin deleted its branch, then a commit only it holds), `feat/dirty` (merged, an untracked file), `feat/locked` (merged, locked), `feat/open` (pull request open), `feat/plain` (none), `feat/bad` (merged, its hook fails) and `feat/squash` (merged, its branch deleted on origin). The same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (branches, remote branches, worktrees and their locks, every `meta.json`). Paths, commits, op ids, fingerprints and durations are normalised; each worktree's `created_at` is set one second after the one before, in the order created, since legacy lists its candidates by creation time to the second. `prune/states.diff` holds the differences. Then, in a fresh sandbox per side, the wizard of `prune --merged` in tmux at 100×40, `feat/dirty` checked: `prune/tty/legacy/` and `prune/tty/next/`.
  1. `--dry-run`
  2. `--dry-run --output json`
  3. `--gone --dry-run`
  4. `--dry-run --force`
  5. `--closed --yes`
  6. `--merged --dry-run --reparent-children`
  7. `--merged --yes --reparent-children`, `feat/bad`'s hook failing
  8. `--yes --output json`
  9. `--yes` again
  10. `feat/dirty --yes`, named (next only: legacy takes no worktree)
  11. `feat/open --yes`, named
  12. `feat/dirty --force --yes`, named
  13. no `--yes`, without a terminal
- The states are identical for 1 to 6. The differences, all deliberate:
  - 1, 3 and 4, legacy bugs: legacy's `Would prune` lists `feat/gone-work`, a branch whose upstream is gone holding a commit no other branch holds (B2, which 8 then deletes with `git branch -D`, the commit lost), says it reparents `feat/child` without `--reparent-children` (B1), and its forced dry run lists `feat/dirty` as merged, nothing said of what forcing it loses (B7). Next keeps `feat/gone-work` (`Kept  feat/gone-work (deleted on origin; unpushed)`, `prune.kept`), recaps `feat/child (left on feat/merged)`, and marks each forced worktree with its blockers (`feat/dirty (merged #4; dirty, forced)`). Each unsafe worktree it keeps, which the command chose (no worktree named), is a plan entry with its blockers and no phase, an item `skipped` as `ruled_out` once applied (`! prune: 2 done · 1 skipped`), and a warning, the last followed by `→ wtm prune feat/dirty feat/gone-work feat/locked --force   prune them anyway`; legacy prints `Skipped … — pass --force`.
  - `feat/moved` is no candidate for next (D1): its branch moved past its merged pull request's head, the rule of list's `merged` and the tree's action. Legacy lists it as skipped for its unpushed commit.
  - 7, legacy stops at the first failure (D3, and B4: the worktrees after it are in no list of its report); next prunes `feat/merged` and `feat/squash` all the same, `feat/bad` failed under its own line, exit 1 on both.
  - 8: legacy prunes `feat/gone-work`, its commit lost (B2); next keeps it.
  - 10 to 12: legacy has no worktree argument (exit 2). Next refuses a named unsafe worktree with `prune.unsafe` (exit 23, followed by the same request forced), a named one not done with as an invalid request (`feat/open is not done with: its pull request is still open`, exit 2), and removes a named unsafe one forced.
  - 13: legacy needs a terminal or `--yes` (exit 1); next runs unattended on the safe candidates (D5), as every next command.
  - The tty: legacy lets `feat/dirty` be checked, counts it in `Will prune 4 worktree(s)`, then skips it after the confirmation (B3, `! Skipped feat/dirty (dirty — pass --force)`); next asks a `Force` step naming it (`feat/dirty — uncommitted changes`) and its recap marks it forced. Each candidate reads why it is done with (`feat/merged — merged #1`, `feat/dirty — merged #4 · uncommitted changes`). Next's `Reparent children` puts `Leave them orphaned` first, legacy reparents by default.
# Proof: `wtm fast-forward` (legacy `flow/fastforward`) vs `wtm-next fast-forward`

The reference is the legacy `wtm fast-forward` of 0.30.1. Its output differs by design (next speaks the v2 envelope, fetches origin once before it plans and refuses a named worktree it cannot move safely with exit 23), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `fast-forward.sh <wtm> <wtm-next> fast-forward`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin` a teammate pushed to, unfetched; worktrees `feat/behind`, `feat/uptodate`, `feat/ahead` one commit past origin, `feat/diverged` both, `feat/dirty` behind with an untracked file, `feat/clash` behind with an untracked `clash.txt` origin's commit adds, `feat/rebasing` behind and stopped mid-rebase, `feat/noremote` never pushed, the branch `feat/loose` behind and checked out nowhere, and one worktree per other scenario), the same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (each branch at origin's tip as origin has it or not, the worktrees, a rebase under way). Paths, op ids, commits and fingerprints are normalised. `fast-forward/states.diff` holds the differences. Then, in a fresh sandbox per side, fetched, the wizard in tmux at 100×40: `fast-forward/tty/legacy/` and `fast-forward/tty/next/`.
  1. a worktree behind origin, `--yes`
  2. one up to date
  3. one ahead of origin
  4. one diverged from origin
  5. one origin does not have
  6. one behind with uncommitted changes
  7. the same, `--force --yes`
  8. one whose untracked file origin's commit would overwrite, `--force --yes`
  9. one behind, stopped in the middle of a rebase
  10. a branch no worktree holds
  11. a worktree given twice, `--output json`
  12. a part of a branch's name
  13. `--output json --yes`
  14. `--output json` without `--yes`
  15. a dirty worktree behind, `--output json --yes`
  16. `-q`
  17. the alias `ff`, `--dry-run`
  18. `--dry-run --output json` (next only)
  19. `--expect-fingerprint` with that fingerprint (next only)
  20. `--expect-fingerprint` with a stale fingerprint (next only)
  21. no worktree, without a terminal
  22. `--all --yes`
- The states are identical but for 19 to 22, where legacy refuses a flag it lacks (19, 20) or the whole run (21, 22). The differences, all deliberate (the owner approved each, 2026-10-26):
  - 9, a legacy bug: legacy runs `git merge --ff-only` on the rebase's detached HEAD and reports `✓ 1 fast-forwarded` though the branch did not move; next leaves a worktree in the middle of an operation (a rebase, a bisect, a merge, a cherry-pick…) as it is, `skipped`, its warning followed by `→ wtm go feat/rebasing   finish it in the worktree` (`TestAWorktreeInTheMiddleOfAnOperationIsLeftAsItIsAndNamesHowToFinishIt`; `provision.Service.FastForward` refuses a worktree stopped in a merge or a cherry-pick too, `TestASourceWorktreeStoppedInAMergeIsLeftWhereItIs`).
  - 6, 15 and 22: legacy refuses a dirty worktree under `--yes` before anything moves (exit 1, text on stderr, nothing on stdout in JSON), naming only the first, and its wizard fails it as an item instead. Next refuses the request when the worktree is named, exit 23 (`fast-forward.unsafe`), one problem per blocker on its entry (`fast-forward.dirty`) and the request forced as its follow-up; with `--all` (22), or a request naming none (the wizard's default pick confirmed as it was), the command chose it, so it is kept (`fast-forward.kept`, the last one followed by one request forcing every kept worktree, `→ wtm-next fast-forward feat/clash --force`), and the others move. The default pick leaves a dirty worktree out; an unattended run naming none takes the default pick, the worktrees behind origin (21). A dirty worktree with nothing to move (up to date, ahead) is no refusal (legacy refused it).
  - 3: legacy calls an ahead branch `already up to date`; next says `ahead of origin`, `done`/`unchanged`. Legacy's picker tags ahead and diverged branches `already up to date` too; next's options carry where each stands as last fetched (`1 ahead`, `1 ahead, 1 behind`, `1 behind`).
  - 4 and 5: both skip, exit 0; next names why with what moves it on (`→ wtm sync feat/diverged   rebase it`), in every mode, where legacy hints at sync only in its wizard's recap.
  - 8: both fail the item, exit 1; next's line is a fixed message per code, git's own words after it, and in JSON the message alone with git's words under `error.params.cause` (`fast-forward.failed`).
  - 10 and 12: legacy matches a part of a branch's name (`ff uptodate` moves `feat/uptodate`) and refuses a branch no worktree holds with exit 11; next takes a worktree's branch or path only, as `clean`, and refuses any other name as an invalid request, exit 2.
  - 11: legacy fast-forwards and reports the worktree twice; next refuses the second entry (`distinct`, exit 2).
  - 14: legacy refuses `--output json` without `--yes` (exit 2); next runs it unattended, as every next command.
  - 21: legacy refuses a run naming no worktree without a terminal (exit 1); next fast-forwards the worktrees behind origin, its default pick, the dirty one kept (exit 0), an owner decision shared with prune.
  - 17 to 20: legacy has no dry run nor fingerprint (exit 2).
  - Next fetches origin once (`git fetch origin`) per run, before it plans, a dry run included, where legacy fetched each branch as it checked it; a fetch that fails is a warning (`fast-forward.fetch_failed`) and the plan stands on origin as last fetched. Each worktree then moves exactly to the commit the plan names, which the fingerprint covers, with no second fetch: a push to origin since the plan is left for the next run. Next takes each worktree's lock, and its first Ctrl+C lets the worktree being moved finish.
  - The tty: legacy's picker checks nothing, tags each worktree before its name and lists them in no stable order (the script picks every one); next's checks those behind origin as last fetched, a dirty one left out, and confirmed as it was it is the command's choice: the request names none, and the plan is the worktrees behind origin, a dirty one kept. Next asks a `Force` step only when a named worktree is unsafe, then a recap whose button names what moves (`Yes, fast-forward feat/tty-behind`). Esc reads `✗ interrupted`, exit 19, as every next command.

# Proof: `wtm exec` (legacy `flow/exec`) vs `wtm-next exec`

The reference is the legacy `wtm exec` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it runs and refuses a request with its field errors), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `exec.sh <wtm> <wtm-next> exec`: two identical sandboxes (the `docs/demos` "acme" project; worktrees `feat/a`, `feat/b`, `feat/c`, `det` on no branch, and `feat/gone` whose directory was deleted), the same scenarios run by each binary, most with a command that exits 3 in `feat/b` alone, saying `boom` on stderr; after each, the exit code, stdout, stderr and the state (branches, worktrees, every `meta.json`, the exec logs). Paths, op ids, fingerprints and durations are normalised. Each scenario runs its worktrees one at a time (`--jobs 1`) but 1 and 12, which run them at once, their progress lines sorted, so a rerun captures the same bytes. `exec/states.diff` holds the differences. Then, in a fresh sandbox per side, the wizard in tmux at 100×40: `exec/tty/legacy/` and `exec/tty/next/`.
  1. `--all`, a command that passes everywhere
  2. three worktrees, the command failing in `feat/b`
  3. the same in two, `--output json`
  4. the same, `--print`
  5. the same, `-q`
  6. a name that is no worktree, `--output json`
  7. no worktree, `--yes`
  8. no worktree, without a terminal
  9. no command
  10. a command the shell refuses (`if then`)
  11. the worktree whose directory is gone, by name
  12. `--all` with a worktree gone and one on no branch
  13. `PORT=9999` in the caller's environment, `--print -- 'echo "PORT=${PORT-unset}"'`
  14. `--jobs -1`
  15. a worktree named twice
  16. `--dry-run` (next only)
  17. `--dry-run --output json` (next only)
  18. `--expect-fingerprint` with that fingerprint (next only)
- The states differ only by the exec logs: legacy keeps one per branch, `exec/<branch>.log`, overwritten by each run; next a directory per worktree and a log per run, `exec/<worktree>/<op>.log`, the ended ones removed by the next run (two runs at once in one worktree keep theirs apart), the worktree on no branch included, which next runs in (12 and every `--all`). The differences, all deliberate (the spec's departures D1–D12, approved by the owner):
  - 2 to 5: both run in every worktree, exit 1. Next tallies `✗ exec: 2 done · 1 failed`, recaps the plan, names the failed worktree with its last lines and its log (`full output:`), and ends on the command again where it did not pass, `→ wtm exec feat/b -- '…'` (`exec.retry`); legacy draws `✗ <command> · 3 worktrees: 1 failed` and its own block. Its progress on stderr is one line per worktree as it ends, as legacy's. In JSON (3) each worktree is an item (`status`, `error.code` `exec.failed` with `params.exit_code`, `detail{path, exit_code, duration_ms, log, tail}`), and `--print` keeps `tail` beside `output`; legacy prints `{command, results, failed}`. With `-q` (5) next prints `✗ feat/b: exited 3` and the line that runs it again there; legacy only `aborted — rerun without --quiet for the report`.
  - 6 to 11, 14 and 15: next refuses each with the uniform field errors, exit 2, and the envelope under `--output json`; legacy prints text on stderr only, exits 11 for a name that is no worktree and for the worktree gone (`branch not found`), 1 for no worktree (two messages, with and without a terminal), and drops a name given twice silently (exit 0).
  - 12: legacy leaves out the worktree on no branch and the one gone without a word; next runs in the first, named by its path, and warns of the second (`exec.worktrees_gone`, `→ wtm-next prune`).
  - 13: legacy hands the caller's `PORT` to every worktree; next unsets the ports run.toml declares, and the variables wtm sets in a worktree: `PORT=unset`.
  - 16 to 18: legacy has no dry run nor fingerprint (exit 2); next shows the plan, its fingerprint and the line that applies it.
  - Next runs under an operation: its locks are shared on the repository and each worktree, so a `clean` of a worktree is refused while a command runs in it (`lock.held`, exit 24), and the recap shows the concurrency capped to the worktrees (legacy: one per CPU, 12 for 2 worktrees). A command's children left in the background are stopped once it returns, as legacy did. Next never allocates an ordinal: an isolated worktree without one runs without its run environment, warned `exec.env_unnumbered`.
  - The tty: both pick the caller's worktree first and ask the command; legacy recaps `Concurrency: <CPUs>`, next the uniform recap, capped, with `Yes, run in 3 worktrees`. Leaving the wizard exits 19 on both.

# Proof: `wtm-next ui`, the local web server

Next only: legacy's `wtm ui` is a terminal dashboard, not a server.

- `ui.sh <wtm-next> ui`: one sandbox with two repositories `A` and `B` (an `on_create` hook that sleeps on `slow/*`), its own HOME and fixed commit dates; the server started from `A` with `--no-open` and reused from `B`, every route called with curl, a create cancelled, forced and left by its client, the event stream of a commit and a create, the guard's refusals, the idle stop after 40 seconds and a restart, then `POST /api/stop`. Each step writes one transcript, `ui/NN-name.txt`: every command as `$ …`, then its output. The sandbox, the token, the repository ids (`<REPO_A>`, `<REPO_B>`), the two servers' pids (`<PID1>`, `<PID2>`), the port they listen on (`<PORT>`: 4777, or another when a server outside the sandbox holds it), the version and timestamps are placeholders; operation ids and fingerprints are numbered in order of first appearance (`<OP1>`, `<FP1>`), so a `DELETE` still names the operation it cancels.

# Deliberate differences from legacy

Each difference between `wtm` and `wtm-next` in `create` and `extract` is one of these, approved by the owner; any other is a bug.

- **The uniform human output.** Both commands print the same shape: a tally headline (`✓ create: 2 done`, `! extract: 1 done · with conflicts`, `= extract: nothing to do · no uncommitted changes in main`), then the plan as the dry run and the recap show it (each file of extract with its tag; an answer recalled from memory marked `· remembered`, one the run remembered for the next runs `· saved for next runs`), a row per worktree that exists (`feat/a  ../acme.trees/feat-a · 3 ports settled (offset +140)`), the anomalies as `!` lines, each item's read `subject  message` (`feat/other  apps/api/a.txt left with conflict markers in feat/other`): an item done's under the plan, then the run's own, and an unfinished item's right under its own line (an unfinished extract adds that nothing was moved or copied, unless the move was half done), then the next steps (`→ wtm go <branch>` for every worktree that exists, after a failure or an interruption too). Legacy prints `✓ Created worktree X` or `✓ Moved N files to X` with its own lines. The hook view is `on_create · <branch>` without echoing the command, its `✓`, duration or failure line drawn on a terminal only; a silent hook that fails there still names its log (`full output: <log>`), as legacy. Legacy's "Reused your existing local branch" line is the `(existing)` mark of the Branches row. (create 01–14, extract 01–11, tty `form-7`, `extract-9`, `interrupt-3`.)
- **`--output json` without `--yes` runs unattended**, as the command contract says; legacy refuses it with exit 2. (extract 13.)
- **A file the target already holds with the same bytes is not a conflict.** Legacy's `--help` says a file that already exists in the target counts as one, though it moves it all the same; next's help says "with other content". (extract 12.)
- **The JSON envelope is v2**: no `origins` nor `source_branch` fields, and the envelope is printed on an invalid request too, where legacy prints nothing on stdout (a legacy bug next fixes). (create 09, 10, extract 09.)
- **Extract's wizard**: its file step is titled `Files` (legacy: `Select files to extract`) with each file's tag as a badge at the row's end, a source is badged with its count of changes, and the branch name shows no `Branch name is required` before any input. (tty `extract-1`, `extract-2`, `extract-4`.)
- **Wording chosen over legacy's where legacy is odd or differs between commands.** The recap step is titled `Confirm` in both commands (legacy: `Confirm & create` in create, `Confirm` in extract). The source update not asked reads `nothing to fast-forward: the source is not behind origin` (legacy's `source already up to date` is false for a source that is ahead, diverged or not on origin). The fast-forward is a `Fast-forward  <branch>` row of the plan (legacy: an `Update:` line). A remembered answer that follows config.toml reads `config default (<strategy>)`, naming the strategy it stands for (legacy: `config default`). The recap's button reads `Yes, apply` when the plan creates no worktree (legacy: `Yes, create worktree`). A branch only origin has is offered as `origin/<branch>`, without legacy's `remote` badge, which would repeat the prefix.
- **`$WTM_PROJECT_DIR` naming a linked worktree resolves to its main worktree**, as every repository a use case opens.
