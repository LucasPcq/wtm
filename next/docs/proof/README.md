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
- `form.gif`, `form-recap.png` and `apply-hooks.gif` are recorded from `form.tape` and `hooks.tape` in a next sandbox (see How to run): the form through its recap and apply, and three branches whose hooks run, one failing.

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
- The states are identical for 1, 2, 3, 5, 6 and 8. The differences, all deliberate:
  - 4 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1.
  - 7 is refused by both with exit 15, but legacy leaves conflict markers in `feat/c`'s `README.md` while saying it changed nothing (its 3-way check passes a conflict on git 2.50); next finds the conflict at plan time, so `feat/c` stays untouched. 8 to 11 differ by that file too.
  - 10 gives both the template's `.env` (`env.strategy = "example"`).
  - 11: next copies main's `.env` (`FROM=main`, `env_strategy: main` in its meta) where legacy has no `--env-from` and refuses the flag (exit 2). Next asks a new target its env strategy as create does, with its flag and its remembered answer — a deliberate departure from legacy, whose extract always follows `env.strategy`.
- The human outputs keep the legacy shapes (the moved/copied headline, each file with its tag, the source line, the conflicts conclusion); the resolve conclusion's undo advice now names the right worktree.
