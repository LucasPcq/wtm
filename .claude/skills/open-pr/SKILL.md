---
name: open-pr
description: Open a pull request on the wtm repository (this Go CLI) the way its maintainer reviews them — base branch taken from the stack wtm records (`wtm tree`), the change shown rather than described (before/after terminal snapshots via tmux, GIFs via VHS, call-tree or diff sketches for internals), and a short what/why · proof · verify · risk body. Use this whenever the user asks to open, create, submit, raise or draft a PR / pull request / merge request on wtm, to "push and open a PR", to write or rewrite a PR description, or to add screenshots/GIFs/proof to a PR — even when they only say "ship it" or "PR this" at the end of a task in this repo.
---

# Opening a PR on wtm

A reviewer of this repo wants three things from a PR, in this order: which branch it lands on, what changes for someone typing `wtm`, and enough evidence to believe it without checking out the branch. Every step below serves one of those.

`S=.claude/skills/open-pr/scripts` (paths are relative to the repository root).

## 1. Pick the base branch

The base is decided in this order — stop at the first that applies:

1. **The user named a base** ("against release/v0.29.1", `--base x`): use it, always. If wtm records a different parent, use the user's base anyway and mention the mismatch in your final report (not in the PR body).
2. **wtm records a parent**: `$S/base-branch.sh` prints the parent of the current branch from `wtm tree --output json`. In a stack the parent is the right base: a PR against `main` would show the parent's commits as if they were this PR's.
3. **wtm has nothing** (exit 3: the branch is a root; exit 4: wtm does not manage it): fall back to `main`, and say so explicitly in the report — "wtm records no parent for X, opened against main". Never default to `main` silently.

Then check the base exists on the remote (`git ls-remote --exit-code --heads origin "$BASE"`). If the parent is local-only, it has to be pushed first, or its own PR opened first: ask the user rather than retargeting to `main`.

## 2. Understand the change

Read `git log --oneline origin/$BASE..HEAD` and `git diff origin/$BASE...HEAD --stat`, then the diff itself.

**Follow every changed symbol to every surface it reaches.** In this codebase one `domain` constant or `rules` function typically renders in several places — `wtm tree` text, `--output mermaid`, `--output json`, the `wtm ui` dashboard, a wizard recap, a `docs/demos/*.tape` that waits on the old text. `grep` each changed identifier and string; list the surfaces it lands on. Each one is either shown in the proof or named under Risk as not shown — a reviewer must never discover a surface you did not mention.

**Run the tests of the touched and dependent packages now** (`go test ./internal/<pkg>/...` for every package that references a changed symbol), before writing anything. A red test changes the PR: fix it, or — when the fix is a product decision — ask the user. Never write a body around a failure you have not resolved or surfaced.

Then classify the change — this decides what proof you need:

| The change… | Proof |
| -- | -- |
| alters what a command prints (text, recap, error, `--help`, exit code) | before/after **text snapshots** (tmux) |
| alters an interactive flow (wizard step, dashboard key, live view, spinner) | before/after **GIF** (VHS), or 1–2 tmux frames if a still says it |
| changes the `--output json` contract | a `diff` of the JSON document |
| is internal (refactor, layering, perf, tests, tooling) | a **shape sketch** (call tree / file tree / mermaid diff), no terminal capture — and the body says "no user-visible change" |
| fixes a bug | the failing scenario before, the same scenario after |

## 3. Show it (show-me, translated to a terminal)

Pick the **smallest view that makes the point** — one or two visuals, each placed next to the sentence it supports. Prose explains why; visuals show what.

- **Terminal output** — capture, don't retype. Before/after blocks, same command, same sandbox state:

  ````markdown
  **Before** (`main` @ abc1234)
  ```text
  ❯ wtm tree
  ┃  main
  ┃  └─ feat/login
  ```
  **After**
  ```text
  ❯ wtm tree
  ┃  main
  ┃  └─ feat/login  ↑2 needs sync
  ```
  ````

  When only a line or two moves, a `diff` block of the output is tighter than two full frames.
- **Runtime flow** — a call tree, as a `diff` when the shape already existed:

  ```diff
   commands/clean.go  runClean
     flow/clean.Run
       worktree.Remove
  +    events.Publish(WorktreeRemoved)
  ```
- **Files / layering** — a shallow file tree with one comment per entry, as a `diff` when it is a move.
- **Package or stack relations** — Mermaid. For a PR inside a stack, `wtm tree --output mermaid` gives the stack ready to paste; trim it to this branch's ancestors and children.
- **Motion** — a GIF only when the change *is* the movement. One idea per GIF, under ~10 s.

Capture recipe (binaries for both sides, isolated sandbox, tmux, VHS, hosting, cleanup): **read `references/proof.md` before capturing anything.** The short version: captures run in a throwaway sandbox under `/tmp`, never in this repository or the real `~/.config/wtm`; text goes inline in the body; GIFs/PNGs go to an orphan branch `pr-assets/<branch-slug>` via `$S/publish-proof.sh`, never into the PR's commits; everything is cleaned up afterwards.

## 4. Before pushing

The PR must not be the first place the gates run:

- `make lint` and `make test` pass — run the `build-validator` subagent (CLAUDE.md §11). Quote its one-line result in the body. If a gate is red, do not open the PR: fix it, or ask the user (and open as `--draft` only if they say so).
- The change carries its docs: `make docs` if a command/flag moved, the `using-wtm` agent skill if agent-relevant behaviour changed, `docs/guide/` or `docs/dev/` for the concept, `make demos` if a README GIF is now wrong, a `CHANGELOG.md` entry for a user-facing change (not for internal tooling). Missing ones are fixed now, not listed as TODO.
- Commits are in English with no `Co-Authored-By` trailer.

## 5. Write the body

Short enough to read in under a minute. Every line answers a reviewer question; anything that does not is cut. No headings for empty sections, no file-by-file changelog (the diff has it), no restating the title.

````markdown
<Only when the base is not main/release: "Stacked on #<N> (`<parent>`) — merge that first." Find N with `gh pr list --head <parent> --json number`.>

<What changes and why, 1–3 sentences, user-visible effect first. Linear key if the branch has one: (LUC-123).>

## Changes
- <one bullet per behaviour, not per file; ≤ 5 bullets>

## Proof
<the visuals from step 3 — or the shape sketch and "No user-visible change.">

## Verify
- `make lint` ✓ · `make test` ✓ (<N> tests) — build-validator
- <the 1–3 commands a reviewer can paste to see it themselves>

## Risk
<what could break and for whom, what is not covered; "Low — <reason>" when it is>
````

Title: Conventional Commits like the history (`fix(tui): …`, `feat(run): …`, `chore(skills): …`), with the Linear key at the end when there is one. Markdown is never hard-wrapped: one paragraph is one line.

## 6. Open it

```bash
git push -u origin HEAD
gh pr create --base "$BASE" --title "<title>" --body-file <body.md>
```

Write the body to a temp file (not in the repository) and pass `--body-file`, so backticks and code fences survive the shell. Never merge, never enable auto-merge. If a PR already exists for the branch (`gh pr view`), update it with `gh pr edit --body-file` instead of opening another.

## 7. Report

Tell the user: the PR URL, the base and *why* that base (user / wtm parent / fallback), what proof is attached, and anything you could not show (e.g. a behaviour that needs GitHub and so could not be captured in the sandbox).
