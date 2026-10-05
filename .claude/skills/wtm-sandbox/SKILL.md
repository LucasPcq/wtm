---
name: wtm-sandbox
description: Run the wtm binary you just built in a throwaway, isolated sandbox and drive it like a user — tmux to type keys and read the screen (wizards, `wtm ui`, `wtm run` views, plain output), VHS to record a GIF/MP4/PNG — optionally side by side with the binary from before the change. Use this whenever you need to see a wtm change working for real rather than only through tests, before saying a feature or fix is done, before running any mutating wtm command (create, clean, sync, prune, run up…) to try it — never against this repository's own worktrees — to reproduce a bug end to end, to compare output before/after, or to capture terminal proof for a PR or an issue — even if the user only says "check it works", "try it", "show me" or "does the wizard look right" in this repo.
---

# Driving wtm in a sandbox

Tests prove the units; this proves the product. A wtm change is not done until the binary has been run the way a user runs it — in a TTY, with the real wizard, on a repository with worktrees — and its screen read. Do it in a sandbox, never against the real `~/.config/wtm`, the real daemon or the worktrees of this repository: other sessions are working in them.

`S=.claude/skills/wtm-sandbox/scripts` (paths are relative to the repository root).

## 1. Build

```bash
out=$(mktemp -d)
go build -o "$out/after/wtm" .                    # just the change
$S/build-pair.sh "$(git merge-base HEAD origin/<base>)" "$out"   # or both sides: $out/before/wtm, $out/after/wtm
```

Build "before" only when the point is a comparison (a PR, a regression hunt). `build-pair.sh` uses `git archive`, so it registers no worktree. The `wtm` on your PATH is whatever release is installed — never the "before" of anything.

## 2. Sandbox

```bash
sb=$($S/sandbox.sh "$out/after/wtm")              # one per binary
```

A fresh copy of the `docs/demos/setup.sh` project — repository `acme`, a web and an API service, `wtm init` done, a `run.toml` — with its own HOME and random ports, under `/tmp` (short path: the daemon's unix socket has a 104-byte limit). `source $sb/env.sh` enters it. Build any extra state the scenario needs (a stack, a dirty worktree, a moved parent) with the sandbox's own `wtm`/`git`, scripted, identically in every sandbox, so two sides differ only by the binary. The sandbox has no GitHub remote: PR badges and anything else that needs `gh` cannot be shown — say so instead of pointing a sandbox at the real repository.

## 3. Drive and read

| You need to… | Use | Read |
| -- | -- | -- |
| check output, walk a wizard, press keys in `wtm ui`, read any screen | **tmux** — `send-keys`, wait for text, `capture-pane -p` | `references/tmux.md` |
| show motion: a spinner, a step appearing, a live refresh | **VHS** — a `.tape` → GIF/MP4, or `Screenshot` → PNG | `references/vhs.md` |

tmux is the default: it is how *you* see the screen, and its plain-text frames are also the best proof. Record with VHS only once tmux has shown the behaviour is right.

Check the paths a test rarely covers, as far as the change reaches: TTY vs `--yes`, `--output json`, `--quiet`, a narrow terminal (`-x 80`), `wtm ui` when the change touches something the dashboard renders.

## 4. Clean up — always, even on failure

```bash
tmux kill-session -t <name>
$S/sandbox.sh --clean "$sb"                       # stops its jobs and daemon, then deletes it
rm -rf "$out"
```

A forgotten sandbox keeps a daemon running and holding ports after the session ends. `--clean` refuses a directory it did not create.

## 5. Report what you saw

Quote the frame that proves the point (plain text, trimmed, with the command line that produced it) rather than describing it. If something could not be exercised in the sandbox, name it.
