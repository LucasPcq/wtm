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
./sync.sh "$wtm" "$next" sync
./exec.sh "$wtm" "$next" exec
./reparent.sh "$wtm" "$next" reparent
./checkout.sh "$wtm" "$next" checkout
./ui.sh "$next" ui
other=$(mktemp -d)/wtm-next && (cd ../../.. && go build -ldflags "-X main.version=0.31.0-other" -o "$other" ./next/cmd/wtm-next)
./daemon.sh "$wtm" "$next" "$other" daemon
./run-up.sh "$wtm" "$next" run-up
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
  - **The picker** (`tty-*`): opens only when stdin, stdout and stderr are terminals and `--quiet` is off (legacy: stdin alone, so `wtm list | cat` drew its wizard into the pipe); its rows badged as the table's, the worktrees not read and the warnings under its title instead of a box; it lists the worktrees that offer an action: `Go to the worktree`, `Open the pull request` (only for a worktree with one, where legacy offered it to every worktree), `Run a command` (which legacy lacked: `wtm-next exec <worktree>`, whose wizard asks the command), `Start the profile`, `Stop the profile`, `View the logs`, `Clean (delete the worktree)` (not for the main worktree; only this for a missing one, by its path on no branch; only Go and Run a command for one on no branch); the action chosen ends the run on its command line, `→ wtm go feat/a   Go to the worktree`, where legacy ran it (Go printed the path); leaving it exits 19 (legacy 0). `r` fetches origin, as legacy's (`git fetch origin`), and reads every worktree again with its pull request.

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
  - **What cannot be asked is a warning, exit 0**: with no daemon listening, the jobs its index holds: its detached stacks and claims are up, read as legacy read them, and a foreground service left in it (a daemon that died) makes them unknown, `status.jobs_unknown` and `jobs: null`, where legacy checked each process itself; `--with-prs` without gh or signed out, `status.prs_unavailable`, the pull requests found before a query failed kept; a base branch that does not exist, `status.base_missing`. A read never starts the run daemon. An interrupted read ends cancelled, exit 19, even once every worktree was read: in text the worktrees already read drawn as they are and a closing `✗ interrupted` line, in JSON `ok: false` with `error.kind: "cancelled"`; it warns of neither `status.jobs_unknown` nor `status.prs_unavailable` (29, 30; `TestAnInterruptedReadWarnsOfNothingItCouldNotAsk`), the policy of every read.
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
  - **A parent branch deleted reads so** (1, 2): a virtual root whose branch is neither local nor on origin, `feat/login` once cleaned, is `(branch deleted)` with the condition `deleted`, where legacy said `(no worktree)`; a branch of the stack no worktree holds keeps `(no worktree)` and `no_worktree` (`TestAWorktreeWhoseParentBranchIsGoneIsSaidSo`). The web words it "branch deleted" and refuses a drop on it.
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
  - **next has its own daemon**: `wtm-next events` starts the hidden `wtm-next daemon`, which hosts the event bus and the run daemon, on the socket and lock legacy's daemon uses (`<UserConfigDir>/wtm/wtm.sock`, `wtm.lock`); the two cannot run at once on one machine, and a legacy `wtm ui` or `wtm events` does not see next's events. It idles out 30 seconds after its last subscriber, request and process of a job (see the run daemon's proof below).
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
  - The sandbox's `gh` is signed out (the sandbox has its own HOME): next asks the forge about every branch it removes (a pull request squash-merged then deleted on origin holds its commits), so each such removal warns `clean.prs_unavailable`, its `reason` `signed_out` in its params (15, 16, 23), legacy nothing.
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
  - 4 and 5: both skip, exit 0; next names why with what moves it on (`→ wtm go feat/diverged   reconcile it with origin by hand`: sync rebases onto the parent, never onto origin), in every mode, where legacy hints at sync only in its wizard's recap.
  - 8: both fail the item, exit 1; next's line is a fixed message per code, git's own words after it, and in JSON the message alone with git's words under `error.params.cause` (`fast-forward.failed`).
  - 10 and 12: legacy matches a part of a branch's name (`ff uptodate` moves `feat/uptodate`) and refuses a branch no worktree holds with exit 11; next takes a worktree's branch or path only, as `clean`, and refuses any other name as an invalid request, exit 2.
  - 11: legacy fast-forwards and reports the worktree twice; next refuses the second entry (`distinct`, exit 2).
  - 14: legacy refuses `--output json` without `--yes` (exit 2); next runs it unattended, as every next command.
  - 21: legacy refuses a run naming no worktree without a terminal (exit 1); next fast-forwards the worktrees behind origin, its default pick, the dirty one kept (exit 0), an owner decision shared with prune.
  - 17 to 20: legacy has no dry run nor fingerprint (exit 2).
  - Next fetches origin once (`git fetch origin`) per run, before it plans, a dry run included, where legacy fetched each branch as it checked it; a fetch that fails is a warning (`fast-forward.fetch_failed`) and the plan stands on origin as last fetched. Each worktree then moves exactly to the commit the plan names, which the fingerprint covers, with no second fetch: a push to origin since the plan is left for the next run. Next takes each worktree's lock, and its first Ctrl+C lets the worktree being moved finish.
  - The tty: legacy's picker checks nothing, tags each worktree before its name and lists them in no stable order (the script picks every one); next's checks those behind origin as last fetched, a dirty one left out, and confirmed as it was it is the command's choice: the request names none, and the plan is the worktrees behind origin, a dirty one kept. Next asks a `Force` step only when a named worktree is unsafe, then a recap whose button names what moves (`Yes, fast-forward feat/tty-behind`). `fast-forward feat/tty-latest`, up to date: legacy asks to confirm a plan that moves nothing (left with esc, exit 19); next asks nothing and ends on `= fast-forward: 1 unchanged`, `Up to date  feat/tty-latest`, exit 0, as prune and reparent end a plan that changes nothing (4).

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

# Proof: `wtm reparent` (legacy `flow/reparent`) vs `wtm-next reparent`

The reference is the legacy `wtm reparent` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it acts and refuses a request problem as a field error, exit 2), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `reparent.sh <wtm> <wtm-next> reparent`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`), the stack `main → feat/a → feat/b → feat/c` and `feat/x` on `main` made by legacy's `create`, `raw` made by `git worktree add` (no record), a local `topic` no worktree holds and `remote-only`, a branch only origin has. The same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (branches, worktrees, every `meta.json`). Paths, op ids and fingerprints are normalised. `reparent/states.diff` holds the differences. Then, in a fresh sandbox per side, the wizard in tmux at 100×40, `feat/b` moved onto `main`: `reparent/tty/legacy/` and `reparent/tty/next/`.
  1. `feat/c --to main --dry-run`
  2. `feat/c --to main --yes`
  3. `feat/b feat/x --to main --yes`, `feat/x` already on `main`
  4. `feat/b --to feat/a --yes --output json`
  5. `feat/a --to feat/b --yes`, a cycle
  6. the same, `--output json`
  7. `main --to feat/a --yes`, the main worktree
  8. `raw --to main --yes`, a worktree wtm did not create
  9. `feat/c --to remote-only --yes`, a name only origin has
  10. `feat/c --to origin/remote-only --yes`
  11. `feat/c --yes`, no `--to`
  12. `--to main --yes` from inside `feat/c`, no worktree named
  13. `feat/c --to main --yes`, where 12 left the two sides apart
  14. `feat/c --to feat/x --yes` with a detached worktree in the repository
  15. the same, once it is removed
  16. `feat/x --to topic --yes`, with `feat/a` and `feat/b` hand-edited into a cycle
  17. `feat/a --to main --yes`, out of that cycle
  18. `feat/x --to topic --yes`, where 16 left the two sides apart
  19. `feat/x --to main --yes`, `base_branch = "feat/x"`
- The states are identical but where next fixes a legacy bug. The differences, all deliberate:
  - 1: legacy has no dry run nor fingerprint (exit 2).
  - 3, legacy bugs: it rewrites `feat/x`'s record and says `✓ Reparented feat/x: main → main` (B4), and its follow-up is a bare `wtm sync` (B8). Next leaves `feat/x` as it is (`reparent: 1 done · 1 unchanged`, `feat/x (already on main)`) and follows up with `wtm sync feat/b` and the undo `wtm reparent feat/b --to feat/a`.
  - 5 to 9, 11: next refuses each as an invalid request, exit 2, naming the field: `feat/b sits under feat/a: moving feat/a onto it would close a cycle` (legacy: `cycle detected in worktree parent chain at "feat/a"`, exit 1, B6), `main is the main worktree: it has no parent` (legacy: `worktree not found: main`, B10), `raw has no wtm record` (legacy: `worktree not found`, B3), `remote-only is no branch: pass a local branch or origin/<branch> to --to` (legacy exit 11), `--to is required` (legacy exit 1). 6 prints the envelope with the field error (`reparent.cycle`, params `value` and `under`); legacy prints text on stderr, stdout empty (B5).
  - 12: next moves the worktree the caller stands in (Q1); legacy needs one named (exit 1). 13 then finds next's `feat/c` already on `main`.
  - 14: one detached worktree makes legacy refuse every reparent, `cycle detected in worktree parent chain at ""` (B1); next moves `feat/c`.
  - 16: a cycle elsewhere makes legacy refuse every reparent (B2); next moves `feat/x`. 17, moving a member out of the cycle, passes on both; next offers no undo back into it.
  - 19: legacy reparents the base branch's worktree, a record `sync` never reads (B7); next refuses it, `feat/x is the base branch: sync never rebases it` (exit 2).
  - The tty: next's worktrees read with their parent (`feat/b  on feat/a`) in git's order, `raw` not offered (legacy lists it sorted by name, then refuses it, B3); its parent step names what it leaves out and why (`Left out: feat/b (moved), feat/c (under feat/b)`, legacy `Branches that would close a parent cycle are not listed`) and offers `origin/remote-only` without legacy's separator and `remote` badge, as create's source step; its recap gives each worktree its move (`feat/b (feat/a → main)`, B9) and the button names it (`Yes, reparent feat/b`); its outcome is the uniform tally, recap and follow-ups.

# Proof: `wtm checkout` (legacy `flow/checkout`) vs `wtm-next checkout`

The reference is the legacy `wtm checkout` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it acts and refuses a pull request it cannot check out as a field error of `<number>`, exit 2), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `checkout.sh <wtm> <wtm-next> checkout`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin` and a second clone that pushes to it) and a fake `gh`, signed in unless the sandbox holds `gh.signed-out`, that answers both binaries' `gh pr list` and `gh pr view` from the sandbox's `prs.json`. Its pull requests: 1 `feat/remote` (only origin has it), 2 `feat/local` (local, as origin has it), 3 `feat/behind` (local, a commit behind origin), 4 `feat/diverged` (local, a commit of its own and one of origin's), 5 a fork's `main`, 6 `feat/held` (checked out by a plain `git worktree`), 7 `feat/gone` (deleted on origin), 8 `feat/draft` (a draft into `feat/local`), 9 `feat/on-release` (into `release/1.0`, which only origin has), 10 `feat/merged` (merged), 11 `feat/own` (local), 12 `feat/auto-deleted` (merged, deleted on origin since the sandbox last fetched, its remote-tracking branch left). The same scenarios run by each binary; after each, the exit code, stdout, stderr and the state (branches with their upstream, worktrees, every `meta.json`). Paths, op ids, commits, fingerprints and durations are normalised. `checkout/states.diff` holds the differences. Then, in a fresh sandbox per side where origin's `feat/local` moved on since the last fetch, the wizard of `checkout --from main` in tmux at 100×40, pull request 2 picked and its branch kept as it is: `checkout/tty/legacy/` and `checkout/tty/next/`.
  1. `1 --yes`
  2. `2 --yes --output json`
  3. `3 --yes`
  4. `4 --yes --ff`
  5. `5 --yes`, a fork
  6. `6 --yes`, a branch a worktree holds
  7. `7 --yes`, a branch origin deleted
  8. `99 --yes`, no such pull request
  9. `--yes`, no number
  10. `abc`
  11. `11 --yes --from feat/own`, the pull request's own branch as its parent
  12. `9 --yes`
  13. `10 --yes`, a merged pull request
  14. `5 --yes --output json`
  15. `--mine --review --yes`
  16. `8 --yes`, gh signed out
  17. `8 --dry-run` (next only)
  18. `8 --yes`
  19. `12 --yes`, a branch origin deleted since the last fetch
- The states are identical but where next fixes a legacy bug. The differences, all approved by the owner:
  - 5 to 10, 15: next refuses each as an invalid request, exit 2, its problem on `<number>` (`pull request #5 is from a fork: wtm checks out only this repository's branches, …`, naming `gh pr checkout 5`, `branch feat/held of pull request #6 is checked out at <SB>/held` followed by `→ wtm go feat/held   jump in`, `branch feat/gone of pull request #7 is neither local nor on origin`, `<number>: no pull request #99`, `<number> is required`, `<number> must be a pull request number, not abc`) or on `--review` (`--review cannot be given with the filter of your own pull requests: pick one`). Legacy exits 1 for 5, 7, 8, 9 (B7) and 15, where it took `--review` alone (B9), 10 for 6, and dumps gh's command line for 8 (B6). 14 prints the envelope with the field error; legacy prints text on stderr, stdout empty (B8).
  - 11, legacy bug B1: legacy checks `feat/own` out recording it as its own parent; next refuses `feat/own cannot be its own parent: pass another value to --from` (exit 2). The worktrees legacy makes after it are numbered one further.
  - 12, Q5: next records the base only origin has as `origin/release/1.0`, as create's sources spell it; legacy records `release/1.0`, a branch the repository does not have (B2).
  - 13, Q3: both check the merged pull request out; next warns `pull request #10 is merged` (legacy never reads the state, B5).
  - 19: both refuse; next with `branch feat/auto-deleted of pull request #12 is neither local nor on origin` (exit 2), its stale remote-tracking branch no branch to track; legacy with git's `couldn't find remote ref` (exit 1).
  - 16: next refuses with exit 25 (`pull requests unavailable: run gh auth login`, `checkout.github_unavailable`, a precondition), legacy with exit 1.
  - 17: legacy has no dry run nor fingerprint (exit 2).
  - The output is the uniform tally, recap and follow-ups: legacy's `Reused your existing local branch` is the `(existing)` mark of the Branch row, a branch kept behind origin is the `!` warning `checkout.behind_origin` (legacy: a plain line, B11), and the plan's warnings come before the step into the worktree.
  - The tty: next's wizard has nine steps, `--no-fetch` (`no_fetch`, set by its flag only) among them in its trail; its picker tags the fork `fork` and the branch a worktree holds `worktree exists`, a draft `· draft`; legacy shows the fork named `main` as `linked` (B4). With `--from`, legacy reads origin as last fetched, since no branch list refreshed it, and skips the fast-forward (`source already up to date`) of a branch 1 commit behind (B3); next fetches the pull request's branch before it reads the repository, asks it, titled `Branch update` since it moves the pull request's own branch and not a source (legacy: `Source update`), and, kept, warns of it on the outcome. Next asks two steps more, its filters, shown set by their flag only. A pull request picked that cannot be checked out (a fork's, one a worktree holds) is refused at the recap, `Cannot apply this plan` with its problem and follow-up, where legacy disabled it in the list: the engine's select step does not check its field's problems.

# Proof: `wtm sync` (legacy `flow/sync`) vs `wtm-next sync`

The reference is the legacy `wtm sync` of 0.30.1. Its output differs by design (next speaks the v2 envelope, fetches origin once before it plans, takes the worktrees under the one named by default and refuses a named worktree it cannot rebase safely with exit 23), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `sync.sh <wtm> <wtm-next> sync`: two identical sandboxes (the `docs/demos` "acme" project with `shared.txt` on main, a bare `origin`, and two stacks pushed to it, main → `feat/s1` → `feat/s2` and main → `feat/k1` → `feat/k2` → `feat/k3`, then a teammate's change to `shared.txt` on origin's main, unfetched, which `feat/k2` changes its own way; `feat/dirty` → `feat/dirty-kid` with an untracked file in `feat/dirty`, `feat/solo`, and `feat/gone`, its directory deleted), the same scenarios run by each binary in order; after each, the exit code, stdout, stderr and the state (each branch's commits by subject and whether it stands at origin's tip, each worktree with a rebase under way and its uncommitted files). Paths, op ids, commits and fingerprints are normalised. `sync/states.diff` holds the differences. Then, in a fresh sandbox per side, the wizard in tmux at 100×40 on a stack main → `feat/tty-a` → `feat/tty-b`: `sync/tty/legacy/` and `sync/tty/next/`.
  1. `feat/s1 --dry-run`
  2. `feat/s1 --descendants=false --yes` (next only)
  3. `feat/s1 --yes`
  4. `feat/k1 --yes`: the conflict mid-stack
  5. `feat/k2 --yes --output json`
  6. `feat/k2 --keep-conflict --yes`
  7. `feat/k2 --yes` while its rebase is under way
  8. the conflict resolved by hand and continued, then `feat/k2 --yes`
  9. `feat/dirty --yes`
  10. `feat/dirty --force --yes` (next only)
  11. `feat/gone --yes`
  12. `main --yes`
  13. a name that is no worktree
  14. a part of a branch's name, `--dry-run`
  15. `--dry-run --output json`
  16. `--expect-fingerprint` with that fingerprint (next only)
  17. `--expect-fingerprint` with a stale fingerprint (next only)
  18. `feat/s1 --push --yes`
  19. `-q`
  20. no worktree, without a terminal
  21. `feat/solo --yes` beside a worktree on no branch
  22. `feat/solo --yes` with `feat/s1` and `feat/s2` each the other's parent
  23. `feat/s1 --yes`, named in that cycle
  24. `--all --yes`
- The differences, each listed in the spec (`spec-sync.md`, D1–D21) and approved by the owner (2026-10-31):
  - 1, a legacy bug (B3): legacy's dry run reads origin as last fetched and says `Base main … (already up to date / no fast-forward)`; next's fetches, so its plan fast-forwards main from origin first, as the apply does.
  - 2 and 3: legacy syncs the worktree named alone (`--descendants` is no flag of it, exit 2, so legacy's 3 rebases `feat/s1` and leaves `feat/s2`); next takes the worktrees under it by default and names them (`Descendants  feat/s2`), `--descendants=false` the named alone, its recap naming those left out.
  - 4, the conflict mid-stack: legacy, asked for `feat/k1` alone, rebases it and leaves the rest; next rebases `feat/k1`, stops on `feat/k2`'s conflict, aborts it (the worktree as it was) and skips `feat/k3` (`blocked`), exit 24 (`sync.conflict`), followed by `wtm-next sync feat/k2 --keep-conflict` to resolve it by hand. Legacy's conflict exits 1 (5, 6).
  - 6 and 7: both leave the rebase in progress; legacy's footer says how to continue, next's follow-ups enter the worktree then sync what is left (`→ wtm go feat/k2`, `→ wtm-next sync feat/k2`); with the rebase under way (7), both leave it, next naming `go` to finish it.
  - 8, a legacy bug (B1 and B4): once the conflict is resolved by hand, legacy reads `feat/k2` as diverged from origin and skips it and `feat/k3` for good, exit 0; next reads that `feat/k2` holds every commit origin has (its reflog), stands on its parent, and rebases `feat/k3` onto it from its fork point, replaying its own commit alone.
  - 9 and 10: legacy skips a dirty worktree, exit 0, with nothing to force it; next refuses it named, exit 23 (`sync.unsafe`), followed by `--force`, which rebases it with its changes stashed around the rebase, the worktree under it following; one the command chose (a descendant, `--all`, the default pick) is kept with `sync.kept`.
  - 11, a legacy bug (B8): legacy fails on a gone directory with git's words, exit 1; next leaves it, followed by `clean`.
  - 12: legacy refreshes the base alone; next refuses naming it, exit 2, followed by `wtm-next fast-forward main`.
  - 13 and 14: legacy matches a part of a branch's name and refuses an unknown one with exit 11; next takes a worktree's branch or path only, exit 2.
  - 16 and 17: legacy has no fingerprint (exit 2).
  - 18: both force-push; next leases on origin's tip as planned, a teammate's push since refusing it, and never pushes a branch origin has never had.
  - 20: legacy refuses a run without a terminal and without `--yes` (exit 1); next applies its default pick, the worktrees that need a sync, none at that point (`= sync: nothing to do`).
  - 21 to 24, legacy bugs (B2, B5): one worktree on no branch makes every legacy sync fail with `cycle detected in worktree parent chain at ""`, and a cycle anywhere refuses every run; next ignores the worktree on no branch, syncs `feat/solo` beside the cycle, refuses only the run naming its member (`sync.cycle`, exit 23, followed by `reparent <it> --to main`), and with `--all` leaves the cycle's members, each with that follow-up.
  - The tty: legacy asks On conflict, Parent branches (when a parent is behind origin), Confirm, then whether to push once the rebases ran; next asks Descendants (when a worktree named has some, naming them), On conflict, Parents and Push before the recap, which names every worktree it rebases. Next's wizard asks on origin as last fetched while its plan fetches first, so it asks each question the fetch could make matter: On conflict when a worktree may rebase, Parents when a branch rebased onto is on origin, Push when a worktree that may rebase is on origin, Force when a worktree named has uncommitted changes. 5 is a dry run on `feat/tty-b`, already on its parent; a run with nothing to do ends without a confirmation (`TestNothingToSyncEndsWithoutAskingToConfirm`).
  - Next takes each worktree's lock (legacy none) and its first Ctrl+C lets the git step under way finish.

# Proof: `wtm-next ui`, the local web server

Next only: legacy's `wtm ui` is a terminal dashboard, not a server.

- `ui.sh <wtm-next> ui`: one sandbox with two repositories `A` and `B` (an `on_create` hook that sleeps on `slow/*`), its own HOME and fixed commit dates; the server started from `A` with `--no-open` and reused from `B`, every route called with curl, a create cancelled, forced and left by its client, the event stream of a commit and a create, the guard's refusals, the idle stop after 40 seconds and a restart, then `POST /api/stop`. Each step writes one transcript, `ui/NN-name.txt`: every command as `$ …`, then its output. The sandbox, the token, the repository ids (`<REPO_A>`, `<REPO_B>`), the two servers' pids (`<PID1>`, `<PID2>`), the port they listen on (`<PORT>`: 4777, or another when a server outside the sandbox holds it), the version and timestamps are placeholders; operation ids and fingerprints are numbered in order of first appearance (`<OP1>`, `<FP1>`), so a `DELETE` still names the operation it cancels.

# Proof: the run daemon (legacy `service/process`) vs `wtm-next daemon`

The proof drives the daemon raw over its socket with `daemon.py`, a request per line, in one sandbox, and records each answer (`daemon/NN-*.ndjson`). Every daemon it runs it starts itself, as a child it holds the handle of, and stops through the protocol or kills by that handle: it never signals a pid read from a file or an answer. Pids, times, ports, next's build (`<NEXT>`) and the sandbox path are normalised.

- `daemon.sh <wtm> <wtm-next> <wtm-next of another build> daemon`, on the "acme" project with a detached stack `db` (its launcher leaves a loop that runs while a marker file is there; its stop removes the marker):
  1. legacy `wtm run up --profile all` starts the legacy daemon with `db`, `api` and `web` (`01`); `wtm-next status` reads its jobs through the legacy daemon's own listing (`02`); its raw `list` and its `stop` of a job it does not know (`03`, `04`)
  2. the legacy daemon asked to `shutdown` through its own protocol: its services stop, `db` stays in the index (`05`, `06`)
  3. next's daemon takes `db` over from that index and stops it with its stop command, the marker removed (`07`, `08`)
  4. a task's lines then its end, a service started, the same started again, a failing task, the listing, a stop of a job it does not know, the service stopped then started again, an unknown action, a line that is no JSON, a second daemon on the same socket (`09` to `19`); a service's start is recorded without its `last_line`, which races its first print
  4b. a job's lines: a service that echoes what is typed into it (`19a`), the last lines of another (`19b`), the listing with each job's `last_line` (`19c`), an attach that types into the first and reads its terminal, its history replayed first (`19d`), its log as the legacy wtm reads it (`19e`)
  5. a service deaf to SIGHUP started, then a `kill -9` of the daemon: the service it drained dies with its terminal, the deaf one survives (`20` to `22`); `wtm-next status` with no daemon and a foreground service in the index: the jobs are unknown (`23`); the daemon started again reaps the orphan and lists the other crashed, the index emptied (`24` to `26`)
  6. a daemon of another build (`0.31.0-other`) starts a service; `wtm-next status` reads it as it is (`27`, `28`)
  7. a shared stack (`scope = "shared"`, a namespace `app_{worktree}` whose create echoes it): claimed from two worktrees, it runs once in the main checkout (`29`, `30`, `31`), its log noting each worktree's namespace, read through a claim (`32`); the first worktree's stop releases it, the second's stops it, its stop command run (`33`, `34`), the index emptied (`35`)
  8. the proxy, on the sandbox's `[proxy] port`: a service (`python3 -m http.server`) started with a route `web.main.acme.localhost`, its `url` that name on the proxy's port, the listing's `proxy` (`36`, `37`); a request for that host reaches it (`38`), an unknown host is answered 404 with the names served (`39`), the probe with the port it bound (`40`); once stopped, its name is no longer served (`41`, `42`). Every listing of next's daemon carries `proxy` (`07`, `13`, `19c`, `24`, `31`)
- The differences, all approved by the owner:
  - **One daemon** hosts the run daemon and the bus, on the legacy socket, lock and index; the index keeps legacy's format, version 2, so each daemon takes over the jobs the other left (`06` → `07`).
  - **The protocol is v2**: every answer stamped with `protocol`, `version` and `pid` (legacy: `version`, `daemon_pid`); an error carries a code (`daemon.job_running`, `daemon.task_failed`, `daemon.unknown_action`, `daemon.invalid_request`); a job says `state`, `hold` (`own`, `detached`, `claim`) and `reaped`, beside legacy's `status` word, which a legacy client counts the jobs a daemon holds by, its `started_at` only while it is up and its `pid` only for a process of the daemon's own; a task's output comes as lines (`line`, `at`, `text`) where legacy sent raw bytes in base64; `stop` answers its `outcome`; the proxy is the listing's `proxy` alone, where legacy put `proxy_port` and `proxy_public_port` on every answer, a start's and a task's included.
  - **A launcher and a stop command run through a pipe**, a dumb terminal that asks for colours (legacy: a terminal), so their output reads as plain lines; a foreground service still runs under a terminal of its own.
  - **A client never signals a daemon**: a daemon of another build or of the legacy wtm holding no job is asked to `shutdown` and replaced, one holding jobs refused, `daemon.skew` or `daemon.legacy`, whose follow-up is `run daemon restart` (`wtm-next run daemon restart`) (`adapter/daemon` `TestEnsureRefusesASkewedDaemonHoldingJobs`); legacy SIGTERMed the pid holding the socket when it refused `shutdown`. A read warns of a daemon of another build or of the legacy wtm (`<command>.daemon_skew`, followed by `run daemon restart`): `02` and `28` show `status`'s.
  - **Job events are v2** (`job.started`, `job.stopped`, `job.crashed`, `job.exited` in `events.v2.json`), published by the daemon in its own process (`cmd/wtm-next` `TestTheDaemonRunsStopsAndAnnouncesTheJobsItsClientsAsk`).
  - **New: a job's lines and terminal for the web and the run view** (LUC-361, LUC-339): `last_line` in the listing, `lines` (tail, then follow, then `ok` carrying the job, the shape the spec now states), `attach` and `resize`; legacy had `attach`/`resize` (raw, the same shape) and read logs from their files only. A line's `at` on the wire is UTC to the millisecond; the log file keeps legacy's UTC seconds, its files 0600 and a directory made for them, or a legacy one holding the job's log, 0700 (legacy: 0755, 0644); any other directory a start names keeps its mode.
  - **Shared jobs**, as legacy: claims as the reference count, the main checkout's hold, a namespace created per worktree with retries and noted in the service's log, `released` where legacy answered `released: true`; a claim announces its own `job.started` and `job.stopped`, and a shared job's events carry `held_by`.
  - **The proxy**, as legacy, in the daemon's process, bound once it holds its lock and before it takes the index over, closed before it lets the lock go: a job's `url` its name on the public port, the listing's `proxy` (`port`, `public_port`) where legacy answered `proxy_port` and `proxy_public_port` beside the jobs (`36`, `37`); its 404 and 502 pages and its probe as legacy's (`38` to `40`), save that the 404 lists the names served only to a name of this machine, and the probe answers a token the daemon checks besides the port, so an unrelated server on port 80, or a stale proxy, is never taken for this one (legacy took any answer echoing the port; a process of the user's reading the token and echoing it is outside the threat model), a request's headers read within 10 seconds and an idle connection closed after 2 minutes (legacy: no bound). Only a service is served (legacy: any job started with routes, a task included), and a stack once its launcher succeeded (legacy: from its launch, withdrawn if it failed). A stack's stop command still running after 5 minutes is stopped and fails the stop, the stack left up and indexed, a bound legacy lacked (`daemon` `TestAStopCommandThatHangsIsStoppedAndLeavesTheStackUp`), and so is one still running as the daemon stops (`TestAStopCommandAShutdownCutsShortSaysSo`). A global config the daemon cannot read turns the proxy off, said in `daemon.log` (`cmd/wtm-next` `TestWhyTheDaemonServesNoProxyIsInItsLog`), where legacy's daemon refused to start.
  - **With no daemon answering, reads list the stacks and claims of the index** (`adapter/daemon` `TestWithNoDaemonTheStacksAndClaimsOfTheIndexAreUp`); only a foreground service left there makes the jobs unknown (`23`).
- The bugs fixed, each with its test:
  - **A daemon that could not start was reported as a timeout**, its cause lost (a socket path past the 104-byte limit: `daemon did not start within 5s`): the client refuses such a path before starting anything, `daemon.socket_path_too_long` (`TestASocketPathPastTheSystemsLimitIsRefusedBeforeAnythingStarts`), and the daemon's standard error goes to `daemon.log`, whose last line a start that times out names (`TestADaemonThatDoesNotStartSaysWhyFromItsLog`, `shell` `TestADetachedChildWritesItsStandardErrorToItsLogEmptiedFirst`).
  - **A stop of a job the daemon does not know answered `job nope stopped`** (`04`): next answers `not_running` (`14`; `daemon` `TestStoppingAJobThatIsNotUpSaysNothingWasRunning`).
  - **The client SIGTERMed whatever held the socket** when it refused `shutdown` (it killed `wtm-next daemon`): next names it and waits for it, `daemon.busy` (`TestEnsureWaitsForAProcessThatIsNoRunDaemonThenRefusesIt`). The legacy client keeps the bug: `internal/` is frozen.
  - **A job no longer up kept its start time**, so `run ps` drew a reaped job's uptime ticking: `started_at` only while up (`24`; `daemon` `TestAStartTimeIsGivenOnlyWhileTheJobIsUp`).
  - **A line that is no JSON got no answer** from next's bus: the run daemon refuses it, `daemon.invalid_request` (`18`; `TestARequestTheDaemonCannotReadIsRefusedWithItsCode`).

# Proof: `wtm run up` (legacy `flow/run/up`) vs `wtm-next run up`

The reference is the legacy `wtm run up` of 0.30.1. Its output differs by design (next speaks the v2 envelope, plans before it starts anything and refuses a request with its field errors), so the proof compares what each leaves on disk and keeps both outputs side by side.

- `run-up.sh <wtm> <wtm-next> run-up`: two identical sandboxes, one after the other, each with its own HOME (so its own run daemon, socket and index), torn down before the next starts (every job stopped, the compose stack's `docker compose down` run, the daemon stopped): the `docs/demos` "acme" project widened with a task `migrate` whose `touches` reach `db`, a shared service `db` with a namespace whose create appends it to `~/namespaces.txt`, a compose stack `cache` (redis, `docker compose up -d` / `down`), a service `crashy` that exits 1 at once, a task `fail` that exits 3; profiles `dev` (default: api, web), `full`, `broken`, `crash`; worktrees `feat/a`, `feat/b` and `feat/v` (verbatim, sharing main's ports). After each scenario the exit code, stdout, stderr and the state (the daemon's index of the jobs up with each one's hold and profile, every meta.json's ordinal, namespaces and isolation, the jobs' logs, the namespaces made, the compose containers); `run-up/states.diff` holds the differences. Then, in a fresh sandbox per side with feat/b's jobs up, the wizard in tmux at 100×40: `run-up/tty/legacy/` (its live view, left with q) and `run-up/tty/next/` (the recap, then the outcome).
  1. `dev` in feat/a, `-d`
  2. the same again: every job up
  3. `full` in feat/b, JSON: the task, the shared `db` joined (its namespace made and recorded), the stack, the services
  4. `broken` in feat/a: the task fails
  5. `crash` in feat/b: `crashy` exits once launched
  6. `full` in feat/v: `migrate` changes its source's data
  7. `dev` in main
  8. `dev` in feat/v, whose ports main's jobs hold
  9. an unknown profile
  10. `--exclusive` with two worktrees
  11. an unknown worktree, JSON
  12. `--exclusive` in feat/v: the other worktrees' jobs stopped first
  13. a dry run (next only)
- The states differ only by:
  - **the profile each job was started under**, kept in the index and listed by the daemon (Q4), which the legacy wtm neither writes nor reads;
  - **feat/v's ordinal**: legacy numbers a verbatim worktree when it starts its jobs (`ordinal: 3`); next numbers only a worktree whose ports derive from an ordinal (isolated, with a port, a namespace or compose declared), and a verbatim worktree's ports are its source's;
  - **the logs of jobs another profile ran**: legacy removes them before a start (`feat/b/migrate.log` gone at 5); next keeps them (a follow-up).
- The differences, all deliberate (the spec's departures D1–D16, approved by the owner):
  - **The uniform human output**: a line per job as it ends on stderr (`✓ feat/a · api (2ms)`), then the tally (`✓ run up: 1 done`, `= run up: 1 unchanged` when every job was up, 2), the recap (worktrees, profile, the jobs with `(up)` or `(joined, in main)`, each worktree's URLs), the anomalies named (a job that failed with its last lines and its log, a port nothing answers on), and the next steps: the start again where it did not pass with its worktree and profile (`run-up.retry`), `run logs` and `run down` of a worktree left with jobs up. Legacy prints each job's step, the task's raw output at column 0, `Where to reach it`, and `→ wtm run up` / `wtm run logs` / `wtm run down` naming no worktree nor profile.
  - **JSON is the v2 envelope**: an item per worktree whose detail lists every job of the profile with its status (`started`, `already_running`, `joined`, `done`, `failed`, `crashed`, `not_started`), URL, log and ports (3); legacy prints an array of `{branch, path, profile, aborted, jobs}` leaving the jobs never reached out, and prints its refusals as text even with `--output json` (11).
  - **A service that ends once launched fails its worktree** (`run-up.crashed`, exit 1, its last line in the detail; 5); legacy reports it `started`, exit 0. A crash past the moment the start checks is the events' and the live view's to show.
  - **The refusals under the shared rules**: a job changing foreign data is refused 23 (`run-up.unsafe`, a `run-up.foreign_data` problem per job, followed by the request forced; 6), a port another worktree holds 23 (`run-up.port_clash`, followed by the start with `--exclusive`; 8), an unknown profile or `--exclusive` beside several worktrees 2 (9, 10), an unknown worktree 2 (11; legacy 1, its names matched as substrings); legacy exits 1, 14 for the profile.
  - **A recap and a confirmation** before anything starts or stops, and `--dry-run` / `--expect-fingerprint` (13; the wizard in `tty/next/4-recap.txt`); legacy starts as soon as its last question is answered.
  - **No live view yet**: next returns once the jobs are started, `-d` or not (the run view is LUC-339's last PR); legacy opens it (`tty/legacy/4-view.txt`) and recaps when it is left.
  - **Reads warn of a daemon of another build**: list, tree and status now say `! the run daemon is the legacy wtm's: restart it to start jobs beside its own` (`<command>.daemon_skew`, followed by `run daemon restart`) where their proofs start jobs with the legacy wtm (list 06, 07, 13 to 16, its picker; tree 07, 08, 10, 16 to 18; status 11, 12; daemon 02, 28), and list's and tree's `run_up` action names `run-up`.
  - **Not yet in next**: legacy's notice that main's `.env` still spells ports (`addressing`), its Next.js dev-origins hint, its removal of other profiles' logs, and its offer to silence a port nothing answers on by writing `probe = false` into run.toml (dropped: a silent port is the warning `run-up.port_silent`).

# Deliberate differences from legacy

Each difference between `wtm` and `wtm-next` in `create` and `extract` is one of these, approved by the owner; any other is a bug.

- **The uniform human output.** Both commands print the same shape: a tally headline (`✓ create: 2 done`, `! extract: 1 done · with conflicts`, `= extract: nothing to do · no uncommitted changes in main`), then the plan as the dry run and the recap show it (each file of extract with its tag; an answer recalled from memory marked `· remembered`, one the run remembered for the next runs `· saved for next runs`), a row per worktree that exists (`feat/a  ../acme.trees/feat-a · 3 ports settled (offset +140)`), the anomalies as `!` lines, each item's read `subject  message` (`feat/other  apps/api/a.txt left with conflict markers in feat/other`): an item done's under the plan, then the run's own, and an unfinished item's right under its own line (an unfinished extract adds that nothing was moved or copied, unless the move was half done), then the next steps (`→ wtm go <branch>` for every worktree that exists, after a failure or an interruption too). Legacy prints `✓ Created worktree X` or `✓ Moved N files to X` with its own lines. The hook view is `on_create · <branch>` without echoing the command, its `✓`, duration or failure line drawn on a terminal only; a silent hook that fails there still names its log (`full output: <log>`), as legacy. Legacy's "Reused your existing local branch" line is the `(existing)` mark of the Branches row. (create 01–14, extract 01–11, tty `form-7`, `extract-9`, `interrupt-3`.)
- **`--output json` without `--yes` runs unattended**, as the command contract says; legacy refuses it with exit 2. (extract 13.)
- **A file the target already holds with the same bytes is not a conflict.** Legacy's `--help` says a file that already exists in the target counts as one, though it moves it all the same; next's help says "with other content". (extract 12.)
- **The JSON envelope is v2**: no `origins` nor `source_branch` fields, and the envelope is printed on an invalid request too, where legacy prints nothing on stdout (a legacy bug next fixes). (create 09, 10, extract 09.)
- **Extract's wizard**: its file step is titled `Files` (legacy: `Select files to extract`) with each file's tag as a badge at the row's end, a source is badged with its count of changes, and the branch name shows no `Branch name is required` before any input. (tty `extract-1`, `extract-2`, `extract-4`.)
- **Wording chosen over legacy's where legacy is odd or differs between commands.** The recap step is titled `Confirm` in both commands (legacy: `Confirm & create` in create, `Confirm` in extract). The source update not asked reads `nothing to fast-forward: the source is not behind origin` (legacy's `source already up to date` is false for a source that is ahead, diverged or not on origin). The fast-forward is a `Fast-forward  <branch>` row of the plan (legacy: an `Update:` line). A remembered answer that follows config.toml reads `config default (<strategy>)`, naming the strategy it stands for (legacy: `config default`). The recap's button reads `Yes, apply` when the plan creates no worktree (legacy: `Yes, create worktree`). A branch only origin has is offered as `origin/<branch>`, without legacy's `remote` badge, which would repeat the prefix.
- **`$WTM_PROJECT_DIR` naming a linked worktree resolves to its main worktree**, as every repository a use case opens.
