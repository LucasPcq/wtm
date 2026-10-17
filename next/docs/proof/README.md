# Proof: `wtm create` (legacy `flow/`) vs `wtm-next create`

The reference is the legacy `wtm create` of 0.30.1. Its output differs by design (next speaks the v2 envelope and plans before it acts), so the proof compares what each leaves on disk and keeps both outputs side by side.

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
- The states are identical for 1 to 15. The differences, all deliberate:
  - 3 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1. 12 likewise: next exits 2 where legacy exits 10.
  - 14 gives both the configured strategy, `main`.
  - 15 to 17: legacy has no dry run, fingerprint nor `--remember`, and refuses the flag (exit 2). Next plans 15 without applying it, applies 16 (`feat/dry`, `env_from` remembered as `main`) and refuses 17 (exit 25).
- `tty.sh <side> <binary> tty/<side>`, after `create.sh`: tmux at 100×40 in that side's sandbox, `tty/legacy/` and `tty/next/` hold the screens.
  - The form under `--ask`: branches, then source, env strategy (the remembered answer ticked), isolation and the recap, then apply and the outcome. Next asks one step more (`If not exists`, shown settled) and draws its recap as the plan; legacy offers a "Use config default" row next has not.
  - One Ctrl+C during a hook: both exit 19, next saying `Cancelling — finishing on_create of feat/slow; Ctrl+C again to stop now`.
  - Two Ctrl+C: the run dies by SIGINT, exit 130 for both.
- `form.gif`, `form-recap.png` and `apply-hooks.gif` are recorded from `form.tape` and `hooks.tape` in the next sandbox: the form through its recap and apply, and three branches whose hooks run, one failing.

# Proof: `wtm extract` (legacy `flow/`) vs `wtm-next extract`

The reference is the legacy `wtm extract` of 0.30.1. Its output differs by design — next speaks the v2 envelope, plans before it acts and refuses earlier — so the proof compares what each leaves on disk.

- `extract.sh <wtm> <wtm-next> extract`: two identical sandboxes (the `docs/demos` "acme" project with a bare `origin`, a `feat/login` worktree holding a modified `README.md`, an untracked `apps/api/new/` directory, an untracked `apps/web/login.ts`, a deleted `apps/web/package.json` and a renamed `scripts/serve.py`, and a `topic` worktree), the same nine scenarios run by each binary; after each, the exit code, stdout, stderr and the state of every worktree (index, the bytes of each changed file, `meta.json`, branches). `extract/states.diff` holds the differences of state.
  1. a directory to a new branch
  2. one file copied (`--keep`) to an existing worktree
  3. a rename and a deletion to a new branch from `main`
  4. an existing local branch as target without `--from`
  5. a target whose working copy changed the same file
  6. the same with `--on-conflict resolve`
  7. a target whose branch committed another change to the same lines
  8. a source with nothing to extract
  9. `--output json`
- The states are identical for 1, 2, 3, 5, 6 and 8. The differences, all deliberate:
  - 4 is refused by both; next exits 2 (an invalid request, its field naming `--from`) where legacy exits 1.
  - 7 is refused by both with exit 15, but legacy leaves conflict markers in `feat/c`'s `README.md` while saying it changed nothing (its 3-way check passes a conflict on git 2.50); next finds the conflict at plan time, so `feat/c` stays untouched. 8 and 9 differ only by that file.
- The human outputs keep the legacy shapes (the moved/copied headline, each file with its tag, the source line, the conflicts conclusion); the resolve conclusion's undo advice now names the right worktree.
