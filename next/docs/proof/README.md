# Proof: `wtm create` (engine) vs `wtm-next create`

Two identical sandboxes (the `docs/demos` "acme" project with the same ports, a bare `origin`, a local `feature` and `topic`, and an `on_create` hook that fails on `feat/bad*` and sleeps on `feat/slow*`) are driven by each binary in turn. Paths, timestamps, op ids, fingerprints and durations are normalised, and `wtm-next create` is read as `wtm create`.

- `compare.sh <engine> <next> cli`: 15 command-line scenarios, then the state (branches, worktrees, every `meta.json`, every `.env`, `config.toml`). `cli/diff.txt` is empty: stdout, stderr, exit codes and state are byte-identical. `cli/engine/` keeps the captures; the next side, identical, is rebuilt by the script.
  1. a new branch
  2. three branches with one failing hook
  3. an existing local branch without `--from`
  4. the same branch with `--from main`
  5. a branch only origin has
  6. `--if-not-exists`, first run
  7. `--if-not-exists`, second run
  8. `--dry-run`
  9. `--expect-fingerprint` with that fingerprint
  10. `--expect-fingerprint` with a stale fingerprint
  11. `--output json` success
  12. `--output json` invalid request
  13. `--output json` failing hook
  14. `-q` on a refusal
  15. `--ff` on a source behind origin
- `tty.sh <side> <binary> tty/<side>`: tmux at 100×40.
  - The form: branches with a badge, then source, isolation and the recap, then apply and the outcome.
  - One Ctrl+C during a hook: the cancel message, then exit 19.
  - Two Ctrl+C: the run dies by SIGINT, exit 130.
  - `tty/diff.txt` holds the only differences, both deliberate:
    - the form's glyphs: `›` for `▸` and `❯`, `·` for `⊘` (fontcover refuses the engine's legacy runes in new code);
    - the first Ctrl+C message: `Cancelling — finishing on_create of feat/slow; Ctrl+C again to stop now` instead of `feat/slow · Cancelling…` (Decision 8).
- `form.gif`, `form-recap.png` and `apply-hooks.gif` are recorded from `form.tape` and `hooks.tape` in the next sandbox: the form through its recap and apply, and three branches whose hooks run, one failing.

# Proof: `wtm extract` (legacy `flow/`) vs `wtm-next extract`

`extract` has no engine version, so the reference is the legacy `wtm extract`. Its output differs by design — next speaks the v2 envelope, plans before it acts and refuses earlier — so the proof compares what each leaves on disk.

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
