# Talking to the server

Everything that reaches `wtm-next ui`'s server lives in `src/api/`: the generated types, `fetch.ts`, `token.ts`, and one `queries/<resource>.ts` per resource (`version`, `repos`, `commands`, `operations`, `command-line`, `events`). Nothing sits between a component and TanStack Query.

## The token

`wtm-next ui` opens `http://127.0.0.1:<port>/#repo=<id>&token=<token>`. `takeFragment()` keeps the token in `localStorage` and drops the fragment from the address. Every request carries it in `X-Wtm-Token` (`request()` in `fetch.ts`), never in a query string. All tabs of the origin share it; a restarted server's new token reaches them through the `storage` event, and a token change reloads the page so that a server of another version serves its own code. A page with no token says how to open it.

## Reads

| Query | What | Key |
| -- | -- | -- |
| `readQuery(repo, 'list' \| 'tree' \| 'status', answers)` | a read of the catalogue, without GitHub | `['repo', id, …]`: every event of the repository invalidates it |
| `githubQuery(repo, command, answers)` | the same read asking GitHub (`with_prs`) | `['github', id, …]`: no event, no reconnect invalidates it; `staleTime: Infinity` |
| `questionsQuery`, `questions()` | a command's form | under `['repo', id]` |
| `dryRunQuery(repo, 'create', answers)` | what an apply would do, its plan and fingerprint | under `['repo', id]` |
| `commandLineQuery(followUp)` | a follow-up as the line to type, `undefined` when it cannot be spelled | `['command-line', followUp]`, never stale |

Every envelope is typed by `narrowed()` in its `select`: an item's detail is the command's (`Read<'list'>`, `Applied<'create'>`), and `undefined` when the item was not read (a failed or cancelled item carries `{}`). A row of such an item says "not read" or "unreadable", never crashes on a missing field.

**GitHub on demand.** Each view asks GitHub once, the view on screen only: `useRead(repo, 'list' | 'tree', enabled)` holds a view's live and GitHub reads, and the Worktrees page enables only the one on screen. The detail asks each time it opens (`refetchOnMount: 'always'`). Fetch origin (`fetchOrigin`, a list read with `fetch` only) then invalidates both `['repo', id]` and `['github', id]`: the reads on screen ask again, one per view; one off screen asks when it shows. Never `with_prs` on an event, never a timer.

**Something that only counts** (the sidebar's worktree count) observes a read with `enabled: false`: it shows what the view on screen read last, and costs no read of its own.

## A worktree's condition

Each read, live or GitHub's, publishes a worktree's `condition` (`no_worktree`, `unreadable`, `missing`, `rebasing`, `merged`, `changed`, `clean`, the first that applies, ranked in Go) and its `actions`, already ordered. The screen maps a condition to its look (`conditionLook`) and shows the first action. Over the two reads of one worktree there is one rule, `overlaid(live, github)` in `features/worktrees/utils/condition.ts`, used by the list, the tree and the detail: it returns the GitHub read when that one says merged and the live one says changed or clean, the live read otherwise. The detail then shows the winning read's problems.

## Follow-ups

A follow-up the catalogue cannot run (clean, sync, go, reparent, run …) shows as its command line to copy: `CommandLine` (`features/worktrees/components/command-line.tsx`), a block or `inline`; a menu of them, `ActionsMenu`. One the server cannot spell says so with what it meant (`No command line for this: …`), never disappears.

## Operations

`runStreamed({ repo, command, call, started, line })` (`queries/operations.ts`) posts a command with `Accept: text/event-stream` and `expect_fingerprint` from the dry run. It hands the operation's id to `started` as soon as the server names it (`X-Wtm-Operation`), each event to `line`, and resolves with the narrowed envelope. An answer that names no operation is refused and its connection aborted: nothing could cancel it. `cancel(repo, operation, force)` is `DELETE …/operations/{id}` (`?force=true` the second time).

A mutation's run lives in one zustand store, keyed by command and repository (`useRun(repo, command)`, `useUnderWay` in `features/worktrees/hooks/use-run.ts`): the dialog shows it, closed and opened again, and the list dims the rows it works on. Each command turns its plan into units (create's `planned`, a fast-forward first; clean's `unitsOf`, one per worktree removed, both through `todo()`) and starts the run with them (`Start`); the shared `utils/progress.ts` turns each event into their phases, and `ending()` says how a run ended: cancelled (an item cancelled, or the envelope's error of kind `cancelled` when only the plan was under way), failed with why, or done. The dialogs share their frame in `features/worktrees/components/`: `RunDialogContent` (one size while a run is under way, no close button then), `DialogScroll` (the only scrolling area, never a native scrollbar), `RunSteps` (the timeline, folded into one line once a run went well) and `RunningFooter` (Hide, Cancel, then Stop now); each command keeps its own titles and outcome.

## Clean

- **Selection**: rows are checked in the table (`useSelection`, every row but the main worktree's and one a run works on), the header's box checking them all and showing when only some are; the selection is what the rows on screen check, so a row filtered out of view leaves it. While some are, `SelectionBar` stands in for the toolbar, its destructive action last and apart. The detail's "…" menu runs clean's own follow-up as "Remove worktree…", on that worktree alone; both open the one `RemoveDialog` of the page (`useRemoveDialog`).
- **Steps**: Worktrees, Children, Confirm. The first reviews a forced dry run of every worktree asked, so each unsafe one shows what keeps it (`blockers`) and asks keep it or remove anyway; the request is the worktrees not kept, forced when one is removed anyway. Children is skipped when no child is left behind; its one choice is `reparent_children`, each child listed under the parent that goes with where it lands (`moves`). Confirm and the apply hold to the dry run of that request; a choice plans again, and the buttons wait for the fresh plan. A plan that cannot be made shows why; a worktree read again unsafe on a later step brings the wizard back to its choice (the step is derived, the state is one `useReducer`). One removal at a time: opening Remove while one runs, or while one that failed even in part is still to be read, shows it, and the rows checked meanwhile wait; a run that ended well is dismissed when Remove opens again (`openRemove(repo, worktrees)`, `dismissSucceeded`), so the next one opens on its own worktrees. Its failure (a stale plan) shows as the envelope's error; a run that removed every worktree asked takes the rows it opened on out of the selection, one that removed some takes those out (`useSelection().drop`).

## The event stream

`eventsQuery` (`queries/events.ts`) is a `streamedQuery` over `GET /api/events`, read with `fetch` (`EventSource` cannot send the token header) and parsed by `EventSourceParserStream`. A line of a repository invalidates `['repo', id]`; `repo.added`/`repo.removed` the list of repositories; back after a break, every query but GitHub's is read again. It reconnects every 2 s once it ended or failed, and stops when the server refuses the token. `connectionOf` gives live, reconnecting, connecting or disconnected, which the sidebar's footer shows with the stream's warnings.

## Forms over a command's questions

`useCommandForm` (`components/command-form/`) builds a TanStack form from a command's `domain.Form` (`POST …/questions`): one value per field keyed by its escaped path (the form library reads `remember[0]` as nested), a text list as its raw text split when sent. The core validates: a change asks the questions again once typing pauses, `check()` asks at once, and only the latest answer is kept; the answer is the form shown (a field it no longer asks, a default that followed another answer) and its problems land under their fields once touched. The front re-encodes no rule (required, allowed options, branch names). Only the fields the person changed are answered, so a default or a remembered answer stays the core's to decide.

`CommandFields<P>` draws each field in its screen's words, a `Record<P, FieldWords>` over the paths of the fields it is given (`(DomainField & { path: P })[]`), which the core never gives: label (with Remember on the right when the core lets it be remembered), the input of its kind (`FieldInput`), where an untouched answer came from (from config, remembered), the help, the problems.

## Create

- **Words** (`create/utils/words.ts`): exhaustive Records over create's question paths (`createWords`), each closed select's values (`envFrom`, `isolation`, …), the skip codes (`skipWords`, `null` for a reason that means nothing in a browser), the plan's actions and phases. A question, option, reason or phase the core adds fails the build until it has words. `isAsked` narrows the form's fields once to create's questions (`Asked`, `DomainField & { path: Question }`), so `CommandFields` and `NotAsked` index `createWords` directly, never falling back to the path. An option of no words is data (a branch) and reads as its value; its facts (`tag`, `base`, `origin`…) are there to word when a screen shows them.
- **Steps** (`create/utils/steps.ts`): a Record from each question to its step, over the fields `isAsked` kept; a field no question of create answers is left out. A skipped field is said at the foot of its step (`NotAsked`) unless its reason means nothing here; a step that asks nothing is passed. Next goes by the steps of the core's answer to the typing, not the last answer before it.
- **Confirm**: built from the dry run's plan (one `BranchPlan` per branch), never from its recap, which is the command line's. Apply is held to the plan's fingerprint.
- **Run and done**: the dialog keeps a fixed height while it runs; then a run that went well folds its steps into a summary and lists each worktree with its path (copyable) and next command; a failed or cancelled one keeps its timeline open.
