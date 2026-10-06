# Worker brief

A brief is the only context a worker starts with: it does not see the conversation with the user, the other workers, or the Linear board unless told to look. Write it as **issue part + common part** in one file, then hand that file to `spawn-worker.sh`.

Fill every `<…>`. Keep the issue part to what the worker cannot get from `get_issue` alone: where it sits in the stack, which files another worker owns, what the sandbox proof must show.

## Issue part

```markdown
You are the developer for Linear issue <LUC-NNN> on the `wtm` Go CLI, for release <0.29.2>, dispatched by an orchestrator session.

Your worktree: <absolute worktree path> (branch `<feature/luc-nnn>`, created with wtm from `<source branch>`). <Only when stacked: "`<source branch>` = <LUC-MMM> (PR #<N> → base `<release>`, not merged yet): <one line on what it brings>. You build on it (`git log main..<source branch>`).">  Read CLAUDE.md<, `docs/dev/flow-layer.md`, `docs/dev/output.md` — whichever the issue touches> first.

PR base: `<release branch>`. <Stacked: "`<source branch>` (stacked, because #<N> isn't merged); say in the PR body it's stacked on #<N> and retargets to `<release>` once #<N> merges.">
CHANGELOG: <where the entry goes, checked on `origin/<release>` — e.g. "under [Unreleased] → Fixed (the release branch has no <release> header yet; <other workers> also write there, expect a trivial conflict)">.

Fetch the full ticket with the Linear MCP (`get_issue <LUC-NNN>`, plus comments<; parent LUC-xxx>). Summary — "<title>":
- <the symptom, with the files/functions already known>
- <the ticket's plan or the goal, in a few bullets>
- <boundaries: "Don't drift into <LUC-xxx>", "Don't touch <file> — <LUC-yyy> owns it">
- <product questions the worker must ask rather than decide: "whether to add X is a PRODUCT decision — ask the orchestrator with options + recommendation">
<For a bug: "Use superpowers:systematic-debugging to confirm the root cause." / "REPRODUCE IT IN THE SANDBOX before fixing.">
The sandbox check must <the concrete scenarios: which commands, with which flags (`--yes`, `--output json`, non-TTY), what is driven via tmux, what output/exit code proves it>.
```

## Common part

Append verbatim, with the three placeholders filled.

```markdown
## Process
1. Analyse the code, then write a short proposal (approach, files touched, deviations from the ticket, open questions). Send it to the orchestrator as an FYI (see Communication), then implement — don't wait for approval unless it contains a business/product/UX decision.
2. Implement with tests (TDD where practical). Every bug you fix or find gets an explicit regression test at the level that would have caught it before release — the flow test and, when feasible, the command-level entry (including the no-argument interactive one) — verified to fail on the pre-fix code, and listed in the PR body. A generic guard alone is not enough: the bug should never have shipped, the test is what makes sure it doesn't again. Respect CLAUDE.md strictly (Params structs, constants in internal/domain/constants.go, layer rules checked by `make lint`/archlint, minimal comments, markdown never hard-wrapped). Use the `go-cli` skill.
3. Docs per CLAUDE.md: `make docs` if flags change, `docs/guide/`, the agent skill `internal/commands/agents/assets/using-wtm/` when agent-relevant behaviour changes. CHANGELOG entry where the issue part says (follow `docs/dev/changelog.md`); if unclear, ask the orchestrator.
4. When done, launch TWO subagents in parallel (Agent tool):
   a. `build-validator`: make lint, make test (-race), deps hygiene. Fix everything it reports and re-run until green.
   b. a `general-purpose` subagent that uses the `wtm-sandbox` skill to verify the change end-to-end with the built binary in an isolated sandbox, both via the CLI with its flags (`--yes`, `--output json`, non-TTY…) and interactively via tmux, capturing snapshots as proof. Never run mutating wtm commands against this repository's real worktrees or the user's real registry.
   Fix whatever they find and re-validate.
5. Commit in English (conventional style like the repo history, NO Co-Authored-By trailer), push, open the PR with the `open-pr` skill (base given above, title includes the ticket id).
6. Notify the orchestrator with the PR URL and a short summary (what changed, validation results, open points).

## Scope
- Work only inside your worktree; never edit <main checkout path> or other worktrees.
- A bug you find outside your issue: report it to the orchestrator (symptom, repro, suspected cause) — don't fix it unless told to.
- Don't edit files another in-flight worker owns (the issue part names them); if your fix needs them, ask the orchestrator.

## Communication with the orchestrator
The orchestrator is the Claude Code session named `<orchestrator session name>` (herdr agent name `orchestrator`). Message it with the SendMessage tool: `to: "<orchestrator session name>"` (load SendMessage/ListAgents via ToolSearch if deferred; check ListAgents if the name doesn't resolve). Its replies arrive as `<cross-session-message>`.
- Business/product/UX decisions you can't settle from the ticket, CLAUDE.md or the code: send a crisp question with options and your recommendation. If it blocks you, stop and wait for the answer; otherwise continue on your recommended default and flag it.
- Orchestration needs (branch/rebase problems, conflicts, permissions, anything outside your worktree): message it too.
- Always message it at the end (PR opened) or if you get stuck.
```

## Example issue part (as sent for LUC-262)

```markdown
You are the developer for Linear issue LUC-262 on the `wtm` Go CLI, for bug-fix release 0.29.2, dispatched by an orchestrator session.

Your worktree: /Users/lucaspicque/Documents/Dev/wtm.worktrees/feature-luc-262 (branch `feature/luc-262`, created with wtm from `main`). Read CLAUDE.md first. PR base: `0.29.2` (release branch). The CHANGELOG on origin/0.29.2 has no 0.29.2 header, only [Unreleased] — put your entry under [Unreleased] → Fixed (LUC-248 and LUC-256 also write there; expect a trivial merge conflict).

Fetch the full ticket with the Linear MCP (`get_issue LUC-262`). Summary — "Mutually exclusive flags exit 1 instead of 2":
- `wtm clean --keep-data --drop-data`, `wtm run start|up --exclusive --parallel` (and likely every `MarkFlagsMutuallyExclusive` usage — inventory them, plus MarkFlagsRequiredTogether / MarkFlagsOneRequired if used) exit 1 instead of the usage exit code 2: cobra's group-validation error bypasses the FlagErrorFunc, so the mapping to ErrUsage / domain.ExitCodeUsage doesn't apply.
- Found during LUC-248 (in flight in another worktree), which adds `events --all` / `--repo` with an explicit ErrUsage check, same pattern as `exec --all <names>`. Don't touch events.go — LUC-248 owns it.
- Goal: every flag conflict exits 2 with a clear message (and the usage error envelope in --output json), via one centralised mechanism rather than per-command checks if cobra allows; a test covering every exclusive pair; check the exit-code table in the using-wtm reference.
Use superpowers:systematic-debugging to confirm where cobra's error escapes the mapping. The sandbox check must run each conflicting pair with the built binary (text and --output json, with/without --yes) and assert exit 2 + message, plus confirm a normal unknown-flag error still exits 2 and valid invocations are unaffected.
```
