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
