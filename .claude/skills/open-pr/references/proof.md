# Capturing terminal proof

How to produce the before/after evidence for a wtm PR: a sandbox, a pair of binaries, then tmux for text and VHS for motion. Every path below is relative to the repository root; `S=.claude/skills/open-pr/scripts`.

## 1. Two binaries

"Before" is the code the PR starts from, "after" is the working tree:

```bash
before_ref=$(git merge-base HEAD "origin/$BASE")
out=$(mktemp -d)
$S/build-pair.sh "$before_ref" "$out"     # → $out/before/wtm, $out/after/wtm
```

`build-pair.sh` uses `git archive`, so it registers no worktree and leaves `wtm tree` untouched. Never use the `wtm` on your PATH as "before": it is whatever release is installed, not the base of this PR.

## 2. A sandbox per side

```bash
sb_before=$($S/sandbox.sh "$out/before/wtm")
sb_after=$($S/sandbox.sh "$out/after/wtm")
```

Each is a fresh copy of the `docs/demos/setup.sh` project (repository `acme`, a web and an API service, `wtm init` done, a `run.toml`) with its **own HOME** and random ports, so nothing touches the real `~/.config/wtm`, the real daemon or the real worktrees. `source <dir>/env.sh` enters it. If the scenario needs more state (a stack, a dirty worktree, an unpushed branch), build it with the sandbox's `wtm` in the same scripted way in both sandboxes, before capturing, so the two sides differ only by the binary.

The sandbox has no GitHub remote: anything that needs `gh` (PR badges, `--with-prs`) cannot be shown there; say so in the PR rather than pointing a sandbox at the real repository.

## 3. Text snapshots with tmux (the default)

A text snapshot is the best proof for most changes: it is searchable, quotable in review, diffable, and it renders without hosting anything.

```bash
tmux new-session -d -s pr-after -x 100 -y 30 "zsh -f"     # -f: no personal rc, no prompt noise
tmux send-keys -t pr-after "source $sb_after/env.sh; clear" Enter
tmux send-keys -t pr-after "wtm tree" Enter
sleep 2                                                     # or poll capture-pane until the prompt returns
tmux capture-pane -p -t pr-after > after.txt                # -p plain text; never -e (ANSI codes do not render in Markdown)
```

Driving a TUI (wizard, `wtm ui`, `wtm run` view) is the same loop: `send-keys` one key (`Down`, `Enter`, `Escape`, `q`, a letter), wait, `capture-pane`. Capture the one or two frames that carry the point, not every step. Width matters: 100 columns keeps a frame readable in a PR without wrapping.

Trim blank trailing lines and the shell prompt noise, keep the command line so the reader knows what produced the frame.

## 4. Motion with VHS (when the point is a sequence)

Use a GIF only when the change *is* the movement: a spinner, a wizard step that now appears or disappears, a live refresh, a key that now does something. Write the tape in the sandbox, never in `docs/demos/`:

```tape
Output after.gif
Set Shell "zsh"
Set FontSize 16
Set Width 1200
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "source /tmp/wtm-pr.XXXXXX/env.sh; clear"
Enter
Show

Type "wtm create feat/login"
Enter
Wait+Screen@15s /Source branch/
Sleep 1.5s
Enter
Sleep 2s
```

Run `vhs after.tape` from the sandbox, once per side (same tape, other `env.sh`). `Screenshot frame.png` inside a tape gives a still PNG when one frame is enough but colour matters. Keep a GIF under ~10 s and ~2 MB: one idea per GIF. `docs/demos/*.tape` shows the house style (`Wait+Screen` on a visible string rather than fixed sleeps).

## 5. Where the assets go

- **Text snapshots** go inline in the PR body as fenced `text` blocks. Nothing to host.
- **GIFs / PNGs** go on an orphan branch `pr-assets/<branch-slug>`, one per PR, never into the PR branch:

  ```bash
  $S/publish-proof.sh "${BRANCH//\//-}" before.gif after.gif
  ```

  It commits them on `pr-assets/<branch-slug>` without touching the working tree or the current branch, pushes, and prints one `![name](https://raw.githubusercontent.com/…/<commit>/…)` line per file, pinned to that commit. Paste those lines into the body. The repository is public: the assets are too. Leave the branch in place after the merge, the links depend on it.
- **Exception**: when the change alters what a README GIF shows, `make demos` re-records `docs/assets/*.gif` from `docs/demos/*.tape`, and those files belong in the PR's commits (CLAUDE.md, Docs & README). That is documentation, not proof. `make demos` always rebuilds the fixed `/tmp/wtm-demo`, so it is not safe while another session records; if it cannot run now, update the tapes, say "README GIFs need `make demos`" under Risk, and move on — do not hand-patch private copies of the demo scripts.

## 6. Clean up

```bash
tmux kill-session -t pr-before; tmux kill-session -t pr-after
$S/sandbox.sh --clean "$sb_before"; $S/sandbox.sh --clean "$sb_after"   # stops its jobs and daemon, then deletes
rm -rf "$out"
```

`--clean` refuses a directory it did not create. Cleaning matters because a sandbox daemon keeps running and holding ports after the session ends.
