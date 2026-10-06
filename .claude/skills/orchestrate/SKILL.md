---
name: orchestrate
description: Run this session as the orchestrator of several Claude workers on the wtm repo — one wtm worktree and one real Claude session per Linear issue, each in its own herdr pane opened by herdr-wtm — so the user talks to this one session instead of switching tabs. Covers recon, spawning workers with a brief, subscribing to them, relaying their product questions to the user, verifying their PRs, merging and cleaning up on request. Use this whenever the user asks to orchestrate, dispatch or fan out issues, to "launch agents/workers on LUC-x from branch y", to work a list of Linear issues in parallel, to run a release loop (several fixes landing on a release branch), or to drive workers in herdr — even when they only list issues with their source branches and a target branch.
---

# Orchestrating workers on wtm

The user talks to one session — this one. Each issue gets its own wtm worktree and a **real Claude session in a herdr pane** (never an Agent-tool subagent: the user wants to be able to open any worker's pane, read it and type into it). This session spawns them, keeps them unblocked, carries every product decision to the user and back, and checks their claims before reporting.

`S=.claude/skills/orchestrate/scripts` (paths are relative to the repository root).

## Inputs

From the user: a list of issues, each with its **source branch** ("LUC-256 from feature/luc-255, LUC-248 from main"), and the **target branch** every PR lands on (usually a release branch such as `0.29.2`). Anything missing and not derivable (below) — ask once, all together.

## 1. Recon

```bash
test "${HERDR_ENV:-}" = 1   # not inside herdr → say so and stop
herdr --skill               # herdr's own agent doc: read it once per session
wtm tree; git fetch origin
git ls-remote --exit-code --heads origin <release>
gh pr list
```

- Each source branch is current: a local `main` behind `origin/main` gives the worker a stale base. Ask the user before moving it: the main checkout may be someone's working copy.
- A source branch that is not the PR target must not carry commits the target lacks: for "from main, PR to 0.30.1", `git log --oneline origin/<release>..origin/main` must be empty, or the patch release PR drags in unreleased work. If it is not, ask the user — recommend starting from the release branch.
- The release branch exists on `origin`; when a worktree must start from it, create the local tracking branch first: `git branch --track <release> origin/<release>`.
- Fetch each issue with the Linear MCP (`get_issue`, `includeRelations: true`). Blockers matter: an issue blocked by one whose PR is not merged is **stacked** on that issue's branch (source = blocker's branch, PR base = blocker's branch, retarget once it merges).
- Read the CHANGELOG on `origin/<release>`: whether it has a `<release>` header or only `[Unreleased]` decides where every worker writes its entry — tell them, or each guesses differently.
- Note which files each issue will obviously touch. Two in-flight workers on one file is a conflict you want to prevent in the briefs ("don't touch events.go — LUC-248 owns it"), not discover at merge.

## 2. Name this session

```bash
herdr agent rename "$HERDR_PANE_ID" orchestrator
```

Then call `ListAgents` (load it with ToolSearch if deferred): its header says "This session is `<name>`". That name is what workers pass to `SendMessage` — it goes into every brief.

## 3. Write the briefs

One file per issue in the scratchpad: the issue part, then the common part, both from `references/worker-brief.md` (template + a real example). The common part carries the process every worker follows — proposal as FYI, implement, build-validator + sandbox subagents in parallel, commit, PR with `open-pr`, report back — so the issue part only says what is specific: worktree, stack position, PR base, CHANGELOG spot, ticket summary, boundaries, what the sandbox proof must show.

## 4. Spawn a worker

```bash
$S/spawn-worker.sh feature/luc-256 feature/luc-255 luc-256 "$scratch/luc-256.md"
# {"agent":"luc-256","pane":"w1W:p1","workspace":"w1W","worktree":"/…/feature-luc-256","status":"working","brief_sent":true}
```

The script runs `wtm create <branch> --from <source> --yes`, waits for herdr-wtm to open the workspace for the new worktree (it matches `.worktree.checkout_path`), starts Claude in its shell pane (`--permission-mode auto`) and submits the brief. Use Linear's `gitBranchName` as the branch and the issue key in lowercase as the agent name.

- Never create herdr workspaces or tabs yourself: herdr-wtm already opens one per worktree and closes it on `wtm clean`; one you create is a duplicate it never closes. If the script times out waiting for it, the plugin is not running — tell the user. (`herdr workspace create` would not even do: only `herdr worktree open`, the plugin's own call, gives the workspace the `.worktree.checkout_path` the script matches.)
- `brief_sent: false` with `status: blocked`: Claude stopped on its folder-trust dialog for the new worktree. Read the pane (`herdr agent read <name> --source visible`), ask the user to answer it (or answer it if they told you to), then send the brief yourself: `herdr agent prompt <name> "$(cat <brief>)" --wait --until working --until blocked --timeout 30000`.
- `brief_sent: true` expects `status: working`. Anything else: `herdr agent read <name> --source recent-unwrapped --lines 80` before acting; a stalled prompt is not proof it was lost, so never re-send blindly.
- Move the Linear issue to In Progress.

Spawn independent issues back to back; a stacked one can start as soon as its source branch exists. While the parent is in flight, its worker's new pushes don't reach the child by themselves: when the parent changes something the child builds on, tell the child's worker to rebase onto it.

## 5. Subscribe, never poll

After spawning, `ListAgents` shows each worker's Claude session (its name derives from the worktree directory). Subscribe to each:

```
SendMessage(to: "<worker session>", notify_when_idle: true)   # no message: a pure subscription
```

The notice is **one-shot**: re-subscribe every time one arrives. Don't loop on `herdr agent get`, `ListAgents` or "are you done?" messages — workers message you when they have something, and the idle notice covers the rest.

## 6. Relay

Workers send four kinds of messages; each has one response.

| From the worker | You |
| -- | -- |
| FYI (proposal, progress) | a one-line ack; challenge only what contradicts the ticket, CLAUDE.md or another worker's scope |
| a business / product / UX question | relay it to the user **in the user's language**, with the options and the worker's recommendation; send the decision back faithfully; never decide it yourself |
| an orchestration need (branch, rebase, conflict, permission, out-of-worktree change) | handle it yourself — it's what you are for |
| a side bug outside its issue | report it to the user; if they want it fixed, create the Linear issue, a worktree and a worker for it — the finder doesn't widen its own scope |

A worker that went idle without a message: read its pane (`herdr agent read`) — it may be waiting on a permission prompt or a question the user should see.

## 7. Verify before reporting

"PR opened, all green" is a claim. Before telling the user a PR is ready:

```bash
gh pr view <N> --json baseRefName,title,state,mergeable,body
gh pr checks <N>
```

Check the base is the one you asked for (release, or the blocker's branch for a stacked PR), the title carries the issue key, the body has proof, and — for a bug — it names the regression test that fails on the pre-fix code (the brief requires one at the level that would have caught it; a generic guard alone doesn't count). Send the worker back for whatever is missing.

## 8. Merge — only when the user asks

Before merging, settle what depends on the PR: an open question about its code means it is not ready — say so to the user rather than merging. A PR stacked on it gets retargeted first, so deleting the merged branch cannot close it:

```bash
gh pr edit <child-PR> --base <release>      # only when a PR is stacked on <N>
gh pr checks <N> --watch
gh pr merge <N> --squash                    # the repo's convention
```

After each merge:

- Read `CHANGELOG.md` `[Unreleased]` on the release branch: every worker wrote there, and a merge-conflict resolution can drop or duplicate a line.
- The stacked child now lands on the release: `wtm reparent <child-branch> --to <release> --yes` (so `wtm tree` and `wtm sync` follow), then ask its worker to `wtm sync` its worktree, re-run the gates and push — the squash merge left the parent's commits in its history as duplicates.
- Tell the workers whose PRs now conflict to rebase.

## 9. Clean up — only when the user asks

A worker still waiting on a decision, or whose PR is not merged, is not finished: say so rather than exiting it. Reparent its stacked children (step 8) before cleaning a parent, or `wtm clean` leaves them orphaned.

```bash
herdr agent prompt luc-256 "/exit"          # each finished worker
wtm clean feature/luc-256 feature/luc-248 --yes
wtm tree; herdr workspace list              # worktrees gone, their workspaces closed by herdr-wtm
```

Move the Linear issues to their final state if the merge did not already.

## Report to the user

Short and in their language: per issue, the state (working / waiting on a decision / PR #N ready / merged), then the decisions you need from them, each with options and a recommendation. Never paste a worker's whole message — summarise what the user must know or decide.
