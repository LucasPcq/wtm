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
./tree.sh "$wtm" "$next" tree
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
- The states are identical for 1 to 15. The differences, all deliberate:
  - 3 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1. 12 likewise: next exits 2 where legacy exits 10.
  - 14 gives both the configured strategy, `main`.
  - 15 to 20: legacy has no dry run, fingerprint nor `--remember`, and refuses the flag (exit 2). Next plans 15 and 18 without applying them, applies 16 (`feat/dry`, `env_from` remembered as `main`) and 19 (nothing to change), and refuses 17 and 20 (exit 25).
- `tty.sh <side> <sandbox> <binary> tty/<side>`, after `create.sh`: tmux at 100×40 in that side's sandbox, `tty/legacy/` and `tty/next/` hold the screens.
  - The form under `--ask`: branches, then source, env strategy (the remembered answer ticked), isolation and the recap, then apply and the outcome. Next asks one step more (`If not exists`, shown settled) and draws its recap as the plan. Both offer the same env rows, "Use config default (main)" first; each ticks the answer its sandbox remembers: `config` on legacy, `main` on next, which 16 remembered (`--env-from main --remember env_from`, which legacy refuses).
  - One Ctrl+C during a hook: both exit 19, next saying `Cancelling — finishing on_create of feat/slow; Ctrl+C again to stop now`.
  - Two Ctrl+C: the run dies by SIGINT, exit 130 for both.
  - The extract form (`extract-*`), on a change made in `feat/form`: source, files, a new target `feat/moved`, its source branch, isolation, mode and the recap, then apply and the outcome. Next has one step more, the new target's env strategy, settled by the answer the create form remembered (`✓ Env strategy: main · remembered`); legacy's extract never asks it. Both skip the source update (main is not behind) and next shows the conflict question settled; legacy's target list is sorted, since it follows git's order, which varies between runs.
  - Each screen is captured once it shows what it waits for and holds still.
- `form.gif` and its still `form-recap.png` (the recap), and `apply-hooks.gif`, are recorded from `form.tape` and `hooks.tape` in a next sandbox (see How to run): the form through its recap and apply, and three branches whose hooks run, one failing.

# Proof: `wtm extract` (legacy `flow/`) vs `wtm-next extract`

The reference is the legacy `wtm extract` of 0.30.1. Its output differs by design — next speaks the v2 envelope, plans before it acts and refuses earlier — so the proof compares what each leaves on disk.

- `extract.sh <wtm> <wtm-next> extract`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`, a `feat/login` worktree holding a modified `README.md`, an untracked `apps/api/new/` directory, an untracked `apps/web/login.ts`, a deleted `apps/web/package.json` and a renamed `scripts/serve.py`, and a `topic` worktree), the same eleven scenarios run by each binary; after each, the exit code, stdout, stderr and the state of every worktree (index, the bytes of each changed file, `meta.json`, branches). `extract/states.diff` holds the differences of state.
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
- The states are identical for 1, 2, 3, 5, 6 and 8. The differences, all deliberate:
  - 4 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1.
  - 7 is refused by both with exit 15, but legacy leaves conflict markers in `feat/c`'s `README.md` while saying it changed nothing (its 3-way check passes a conflict on git 2.50); next finds the conflict at plan time, so `feat/c` stays untouched. 8 to 11 differ by that file too.
  - 10 gives both the template's `.env` (`env.strategy = "example"`).
  - 11: next copies main's `.env` (`FROM=main`, `env_strategy: main` in its meta) where legacy has no `--env-from` and refuses the flag (exit 2). Next asks a new target its env strategy as create does, with its flag and its remembered answer — a deliberate departure from legacy, whose extract always follows `env.strategy`.
  - 12: both move the file; the states differ only by those of 7 and 11.
  - 13: legacy refuses `--output json` without `--yes` (exit 2); next runs it unattended and creates `feat/jny`.

# Proof: `wtm list` (legacy `surface/cli/wt`) vs `wtm-next list`

The reference is the legacy `wtm list` of 0.30.1. `list` is a read: the proof checks that neither binary changes anything on disk, and keeps both outputs side by side.

- `list.sh <wtm> <wtm-next> list`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`; `feat/a` two commits ahead of main, one ahead of origin, with an untracked file; `feat/b` locked; `feat/gone` whose directory was deleted; `locked-gone`, locked, whose directory was deleted too; a detached worktree; `orphan`, on a branch with no commit; a git repository with no wtm config; a plain directory), the same scenarios run by each binary without a terminal; each records the exit code, stdout and stderr, and the state before and after it (every ref, `git worktree list`, each worktree's status, every `meta.json`, wtm's global directory) into `<side>/unchanged.diff`, empty for both sides. The sandbox's gh is not signed in (its HOME is the sandbox's), so `--with-prs` shows the pull requests unavailable, as they are there. Legacy lists a worktree's jobs in the daemon's order, which varies between runs: its JSON captures sort them.
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
- Then the picker in tmux at 100×30 (`tty-*.txt`): the worktrees once the pull requests had their say, `feat/a` picked and its actions, Go, and a picker left with esc. (1, 5 and 6 show the text; the warnings are in 3.)
- The differences, all approved by the owner:
  - **The JSON is the v2 envelope of a read** (`{v, command, ok, items[{subject, status, detail}], follow_ups, warnings}`, no `op`, `plan` nor `fingerprint`) where legacy prints a bare array (2, 4, 7). The detail renames `is_parent` to `main` (wtm's parent is the branch a worktree was created from), `is_dirty` to `dirty`, `is_locked` to `locked`, `rebase_in_progress` to `rebasing`; it adds `missing`, and `actions`, what the picker offers on the worktree, each a wtm command and its answers (`{command, answers[{path, values}], code, message}`) or, for the pull request, its `url` and `params.number`; `created_at` is `null` without a meta record where legacy made one up from the directory's mtime (the main worktree, always); `services` is sorted, `null` when the run daemon cannot tell; `pr` is `{number, url}`, without legacy's `state`, always `open` since only open pull requests are listed; the subject is the branch, else the path. A refusal is the envelope on stdout (11, 12) where legacy printed text on stderr and nothing on stdout. In text (10), next says `no wtm config found — run \`wtm init\` first` where legacy appended the cause, `: config file not found`.
  - **What cannot be asked is a warning, exit 0**: `--with-prs` without gh or signed out says ``! pull requests unavailable: run `gh auth login` `` (`list.prs_unavailable`, 3, 4) where legacy said nothing; a listing capped at the 50 most recent says so (`list.prs_capped`); with no daemon listening but jobs in its index, `list.jobs_unknown` and `services: null`. A read never starts the run daemon, where legacy started one to read its index back. A pull request from a fork is no worktree's, where legacy badged the worktree of the same branch name (a fork's `main`).
  - **The uniform text**: `main` for the main worktree (legacy `(parent)`), `• active` (legacy `● active`, missing from common monospace fonts), `! dirty`, `! rebasing`, `✓ clean` (legacy `⚠`), the columns aligned whatever the glyphs and a column empty in every row left out (legacy pads two columns too far after `●` and `↑`), a worktree whose directory is gone `✗ missing`, locked or not (legacy `✓ clean`), one on no branch named by its path from where the command runs (legacy: an empty name), the warnings under the table (3). The entry of a bare repository, which has no working tree, is not listed (legacy: an unnamed `(parent) ✓ clean` row).
  - **An argument is refused** (9): exit 2, where legacy listed and ignored it.
  - **`--fetch`**, which legacy lacks: the picker's refresh as a field of the request, so a surface other than the command line refreshes through the catalogue's answers alone (the owner's answers-only decision, LUC-355).
  - **The picker** (`tty-*`): opens only when stdin, stdout and stderr are terminals and `--quiet` is off (legacy: stdin alone, so `wtm list | cat` drew its wizard into the pipe); its rows badged as the table's, the worktrees not read and the warnings under its title instead of a box; it lists the worktrees that offer an action: `Go to the worktree`, `Open the pull request` (only for a worktree with one, where legacy offered it to every worktree), `Start the profile`, `Stop the profile`, `View the logs`, `Clean (delete the worktree)` (not for the main worktree; only this for a missing one; none for a missing one on no branch, which `wtm clean` cannot name; only Go for one on no branch); the action chosen ends the run on its command line, `→ wtm go feat/a   Go to the worktree`, where legacy ran it (Go printed the path); leaving it exits 19 (legacy 0). `r` fetches origin, as legacy's (`git fetch origin`), and reads every worktree again with its pull request.

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
- The differences, each approved by the owner or a legacy bug fixed with its test:
  - **The JSON is the v2 envelope of a read** (2, 5, 8): one item per node, depth first (roots main first, then by creation, the virtual ones by name; children by creation then name), its subject the branch, its detail `{branch, path, main, virtual, parent, children, ahead, behind, needs_sync, dirty, locked, rebasing, missing, cycle, origin, pr, services, actions}` where legacy nests `{roots[{branch, path, is_main, is_virtual, status{…}, children[…]}]}`; `path` and `parent` are `null` for a virtual root and a root, `origin` is `{ahead, behind, state}` with the state spelled (`"ahead"`, legacy `origin_state: 3`), `null` when origin has no such branch. A refusal is the envelope on stdout (14, 15) where legacy printed text on stderr. In text (13), next says `no wtm config found — run \`wtm init\` first` where legacy appended the cause, `: config file not found`.
  - **Ahead and behind are counted against the parent** (bug fixed, `TestAheadAndBehindAreCountedAgainstTheParent`): `↑N` is the commits a branch has that its parent lacks, the base branch's for a root; `feat/a-ui` reads `↑2` where legacy said `base ↑4`, counting `feat/a`'s commits too, and `feat/a-ui-form` nothing where legacy said `base ↑3`. `behind`, the parent's commits the branch lacks, is what `needs sync` says. What is not counted is `null`, where legacy said `0`: the main worktree, a child whose parent is not a local branch (a virtual root, `origin/x`), a member of a cycle (it has no parent to sync onto); none of them needs sync, as in legacy. A root has no parent: its `ahead` is counted against the base branch (a worktree wtm did not create, say), its `behind` is `null`, and it never needs sync, as in legacy (`TestARootBehindTheBaseBranchIsNotFlagged`). A child is counted in one git process, `rev-list --left-right --count <parent>...<branch>`, where legacy ran a tip check and an ancestry check; `tree.base_missing` warns when the base branch does not exist. The label is `↑N` (legacy `base ↑N`: `base` names the base branch in `list`, one word one meaning).
  - **Each node offers the action it needs, in the JSON** (`actions`, the text prints none): a rebase in progress `wtm go <branch>`, a merged pull request `wtm clean <branch>` (not for the main worktree), a parent with commits it lacks `wtm sync <branch>`, the first that applies; none otherwise, nor for a virtual root.
  - **`--with-prs` reads the newest pull request of each branch in any state** (4, 5), `PR #N`, `PR #N merged`, `PR #N closed`, `pr: {number, url, state}`; gh missing, signed out or failing is a warning, ``! pull requests unavailable: run `gh auth login` `` (`tree.prs_unavailable`), exit 0, where legacy said nothing (bug fixed, `TestWhatCannotBeAskedIsAWarningNotAFailure`); a failing batch keeps the pull requests of the batches before it, where legacy dropped them all (`TestAFailingQueryKeepsThePullRequestsFoundBeforeIt`); a fork's pull request is no branch's, where legacy matched it by name (`TestTheNewestPullRequestOfEachBranchIsReadWhateverItsState`).
  - **The jobs up are shown** (7, 8): `services api, web` on the row (list's word, with the jobs' names), `services: ["api", "web"]` in the JSON, `null` with `tree.jobs_unknown` when the daemon cannot tell; legacy documented `▶ N running` and never filled it (`TestTheJobsUpInAWorktreeAreItsServices`). A read never starts the daemon.
  - **The uniform text**: `! dirty`, `! rebasing`, `! needs sync`, `! cycle`, `! locked` (legacy `⚠`, missing from common monospace fonts), the glyph alone coloured; a worktree whose directory is gone is `✗ missing` where legacy drew it clean (bug fixed, `TestAWorktreeWhoseDirectoryIsGoneIsMissingNotClean`); a worktree that cannot be read keeps its place, `✗ unreadable`, its error under the tree (`TestAWorktreeThatCannotBeReadFailsAloneKeepingItsPlace`); the warnings under the tree. The layout is legacy's: two-space indent, `├─ ` / `└─ `, `│  ` gutter, badges aligned two columns after the widest branch.
  - **A detached worktree is no node** (1, 3): it has no branch, so no place in a stack; legacy drew each as an empty root flagged `⚠ cycle`, and its Mermaid node `[" ⚠ cycle"]` had no id (bug fixed, `TestADetachedWorktreeIsNoNodeOfTheTree`).
  - **`--output mermaid` gives each branch its own node** (3, 10): ids by place (`n0`, `n1`…) where legacy's `feat/a`, `feat-a` and `feat.a` all became `feat_a` and a branch `end` broke the chart; `#`, quotes, angle brackets and backticks in a label are entities (`PR #35;12` renders `PR #12`); the warnings follow as `%%` comments (`TestMermaidGivesEachBranchItsOwnNode`). It is unframed and `-q` leaves it, as legacy.
  - **An argument is refused** (11): exit 2, where legacy drew the tree and ignored it. An unknown format (12) is refused by both, exit 2; next adds the way to `--help`.
  - **Each worktree is read once**: legacy listed the worktrees and read each meta record twice (`TestAReadChangesNothingAndReadsEachWorktreeOnce`). Whether a worktree is there and dirty, and the bounded reader that reads eight at once, are list's, shared through `service/repository` (`Present`, `Each`); each command keeps its own counting.
  - **A worktree not read is said so**: a meta record that does not decode fails its node alone, `✗ unreadable` with its cause, rather than making it a root counted against the base branch (`TestARecordThatCannotBeReadFailsItsNodeAlone`); an interrupted read marks its nodes `✗ interrupted`, cancelled, exit 19, never 0 with null counts, and warns of nothing it could not ask (`TestACountInterruptedMakesItsNodeCancelled`, `TestAnInterruptedReadWarnsOfNothingItCouldNotAsk`); under `--output mermaid` the cause is a `%% ✗` comment (`TestAWorktreeNotReadIsDrawnSoInTextAndMermaid`).
  - **`clean` is never offered for the main worktree nor the base branch**, even with a merged pull request.
  - Not fixed, legacy being frozen: legacy's `using-wtm/references/stacks.md` documents list's `origin` object for tree, and the guide's badge table describes `base ↑N` against the base branch and `▶ N running`.

# Deliberate differences from legacy

Each difference between `wtm` and `wtm-next` in `create` and `extract` is one of these, approved by the owner; any other is a bug.

- **The uniform human output.** Both commands print the same shape: a tally headline (`✓ create: 2 done`, `! extract: 1 done · with conflicts`, `= extract: nothing to do · no uncommitted changes in main`), then the plan as the dry run and the recap show it (each file of extract with its tag; an answer recalled from memory marked `· remembered`, one the run remembered for the next runs `· saved for next runs`), a row per worktree that exists (`feat/a  ../acme.trees/feat-a · 3 ports settled (offset +140)`), the anomalies as `!` lines under the plan (extract's conflicts left with markers and how to undo), each unfinished item on its own line (an unfinished extract adds that nothing was moved or copied, unless the move was half done), then the next steps (`→ wtm go <branch>` for every worktree that exists, after a failure or an interruption too). Legacy prints `✓ Created worktree X` or `✓ Moved N files to X` with its own lines. The hook view is `on_create · <branch>` without echoing the command, its `✓`, duration or failure line drawn on a terminal only; a silent hook that fails there still names its log (`full output: <log>`), as legacy. Legacy's "Reused your existing local branch" line is the `(existing)` mark of the Branches row. (create 01–14, extract 01–11, tty `form-7`, `extract-9`, `interrupt-3`.)
- **`--output json` without `--yes` runs unattended**, as the command contract says; legacy refuses it with exit 2. (extract 13.)
- **A file the target already holds with the same bytes is not a conflict.** Legacy's `--help` says a file that already exists in the target counts as one, though it moves it all the same; next's help says "with other content". (extract 12.)
- **The JSON envelope is v2**: no `origins` nor `source_branch` fields, and the envelope is printed on an invalid request too, where legacy prints nothing on stdout (a legacy bug next fixes). (create 09, 10, extract 09.)
- **Extract's wizard**: its file step is titled `Files` (legacy: `Select files to extract`) with each file's tag as a badge at the row's end, a source is badged with its count of changes, and the branch name shows no `Branch name is required` before any input. (tty `extract-1`, `extract-2`, `extract-4`.)
- **Wording chosen over legacy's where legacy is odd or differs between commands.** The recap step is titled `Confirm` in both commands (legacy: `Confirm & create` in create, `Confirm` in extract). The source update not asked reads `nothing to fast-forward: the source is not behind origin` (legacy's `source already up to date` is false for a source that is ahead, diverged or not on origin). The fast-forward is a `Fast-forward  <branch>` row of the plan (legacy: an `Update:` line). A remembered answer that follows config.toml reads `config default (<strategy>)`, naming the strategy it stands for (legacy: `config default`). The recap's button reads `Yes, apply` when the plan creates no worktree (legacy: `Yes, create worktree`). A branch only origin has is offered as `origin/<branch>`, without legacy's `remote` badge, which would repeat the prefix.
- **`$WTM_PROJECT_DIR` naming a linked worktree resolves to its main worktree**, as every repository a use case opens.
