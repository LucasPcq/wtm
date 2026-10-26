# Designing a screen

The rules every screen of the web front follows, drawn in `wtm-ui.pen` and built from its `wtm/` components. Read this before designing or building a screen, a dialog, a menu or a state. Each rule names the component that carries it; use that component, never a copy drawn by hand.

## The goal

Clean by default, detail when something goes wrong. A screen where everything went well shows a count and the next step. A problem is named one by one, where it is decided, with its fix beside it. Secondary detail (folders, steps, git's output) sits in a fold the reader opens.

Everything is plain shadcn (Radix underneath) on Tailwind's base colours, with `--primary` as the one signature colour, Inter, and the system's theme. No new token.

## Hierarchy and spacing

| Between | Space |
| -- | -- |
| two sections of a body | 24 |
| a section's label and its content | 8 |
| the parts of a row | 12 |
| a title and its second line | 4 |
| a dialog's edge and its content | 24 |
| a dialog footer's edge and its buttons | 16 vertical, 24 horizontal |

Type does the ranking: a title is 14 medium foreground, a second line 12–13 muted, a section label 12 medium muted. There is no glyph column: a glyph sits inline, before the text it qualifies.

## Dialogs

`wtm/Dialog`: header, band, body, footer.

```
┌─ Title (static) ─────────────────────────────── ✕ ┐
│ Description: what it does, or the state          │
│ Stepper or progress (the band)                   │
├──────────────────────────────────────────────────┤
│ Alert: a technical problem for the whole plan    │
│ The main list: what the command does             │
│ Left as they are: rows the plan leaves out       │
│ › Details                                        │
├──────────────────────────────────────────────────┤
│ Cancel                          Back   [Primary] │
└──────────────────────────────────────────────────┘
```

- **The title is the command's name**, one line, the same at every step and through the run: Create worktrees, Remove worktrees, Prune worktrees, Fast-forward worktrees, Run a command, Change parent. The subject lives in the body.
- **The description carries the state:** what the command does before the run, "Step 3 of 4 · Run the on_clean hooks" while it runs, one sentence of outcome at the end ("2 pruned.", "1 of 2 removed.").
- **The band always ends on its rule**, empty or not: a `wtm/Stepper` before the run, a `wtm/Progress` during it, nothing after. The header never ends on a hole.
- **The body, in this order:** a `wtm/Alert` for a technical problem of the whole plan; the main list; the **Left as they are** section (same list box, muted names: business states that keep a row out of the plan: diverged, ahead, mid-rebase, not on origin, already there, main, no longer finished); then `wtm/Details`, the one fold, always last.
- **A technical problem never sits in a list, a business state never sits in an Alert.** A failed fetch, GitHub unavailable or a lock held is about the tool; diverged or mid-rebase is about the branch.

### Buttons

- A label is a verb, sentence case, with no "…" and no trailing arrow: Prune, Run a command, Reparent, Remove.
- The primary button is the verb alone for one worktree ("Remove") and the verb with the count for several ("Remove 2 worktrees", "Run in 6 worktrees"); never a branch name.
- Secondary buttons are a verb alone: Back, Cancel, Hide, Done, Run again. The description says what "Run again" takes.
- One primary per screen. A destructive confirmation (Stop now) is the kit's destructive button.
- A disabled primary has a tooltip on its wrapper saying what would enable it ("Check feat/billing-ui to fast-forward it anyway").

## Actions

### One list, one order

Every surface draws its actions from one canonical list, in this order, in sections labelled with `wtm/Menu Label` (`DropdownMenuLabel`):

1. **Branch:** Fast-forward, Sync ⧉, Reparent.
2. **Run:** Run a command, Start the profile ⧉, Stop the profile ⧉, View the logs ⧉.
3. **Open:** Go to the worktree ⧉, Open the pull request.
4. **Remove**, alone after a separator, in the destructive colour.

⧉ marks a command the web does not run: the item copies it (`CommandLine`). In code the list is one `Record` over the commands, in order, that each surface renders.

- **Nothing is hidden or reordered by context.** An item that does not apply is disabled in place, with its reason on a second muted line ("main is the main worktree"); a tooltip would not show, since Radix disables pointer events on a disabled item. The reason is the core's field-error code for that worktree, worded in the screen's `Record`.
- A menu near the bottom of the window opens upward.

`wtm/Menu`, `wtm/Menu Item`, `wtm/Menu Label`.

### Five surfaces

| Surface | Holds | Component |
| -- | -- | -- |
| Page header | "Actions ▾", secondary, beside the one primary "Create worktree": Prune, Fast-forward, Run a command, then Remove. Each opens its dialog on a Worktrees picker prechecked by the core (Prune: the safe candidates; Fast-forward: the rows behind origin; Run, Remove: none). Reparent is absent: it needs a subject. | `wtm/Page Header`, `wtm/Menu` |
| Selection bar | Always the same four: Fast-forward, Run a command, Reparent, a divider, Remove. | `wtm/Selection Bar` |
| Row context menu | Right-click on a row in List or Tree, or the row's "⋯" (shown on hover and focus). | `wtm/Menu` |
| Detail menu | The "⋯" in the detail's header: the same menu for the open worktree. | `wtm/Menu` |
| Detail callout | The one action of the moment, with a button that runs it. | `wtm/Callout` |

- **A row shows its state, never its verbs:** no button in a List row or a Tree node.
- **The selection bar never changes its set.** In a narrow window the buttons keep their icon and lose their label, which moves to a tooltip; nothing folds into "…". An action that applies to some selected rows stays enabled and its dialog lists the others under Left as they are. An action that applies to none is disabled, with a tooltip on its wrapper. Prune is not there: it is a repository action.
- **A context menu on a selected row acts on the selection** and says so in its first label ("4 selected worktrees"); its one-worktree items are disabled: "One worktree at a time".
- Prune's icon is `git-branch-minus`, wherever Prune appears.

### Dialog or toast

| The action… | Runs | Then |
| -- | -- | -- |
| needs an answer (rows, a parent, a command), can lose work (Remove, Prune, anything forced), or runs long with output worth watching | in a dialog | the outcome in the dialog |
| needs no answer, loses nothing and is quick: Fast-forward of one clean row, Reparent by drop, a callout's fix, Remove's "Reparent all" | at once | the row greys with "Fast-forwarding" and its menu is disabled while the lock holds; a `wtm/Toast` says it is under way, then a success toast with Undo where the move is reversible and the next step to copy, or an error toast whose Details opens the outcome dialog |

If the dry run of a direct action finds anything to decide (a dirty worktree, nothing to take, a refusal), it opens its dialog on Confirm instead.

## Colour

One meaning per colour, always beside its word (`Glyph`):

| Colour | Means | Glyph |
| -- | -- | -- |
| green | done, healthy | circle-check |
| amber | the person has something to do: a choice pending, a follow-up | triangle-alert |
| red | failed, or work that will be lost | circle-x |
| grey | nothing to do: kept, skipped, unchanged, cancelled, not part of this run | circle-minus |
| grey | a merged pull request (a state, not a judgement) | git-merge |
| `--primary` | the one primary button, the progress bar | — |

A glyph never changes colour to mean "resolved": it changes icon.

## Rows

`wtm/Worktree Item` in a `wtm/Worktree Group` (the list box).

- **Line 1 is the name, alone.** The reason a row is here (a pull request merged, a branch deleted on origin) is line 2, 12 muted, with its glyph and a link icon to the pull request. No badge.
- **A leading checkbox means "in this run"**, in every list where a row can be left out. There is one control and one meaning: no segmented choice, no undecided state.
- **The safety rule:** an unsafe row starts unchecked, with its amber reason ("1 commit on no other branch"). Checking it turns that line into the red loss ("Its 1 commit on no other branch is lost.") and tags the row "anyway". The consequence shows where it is decided. A worktree is never touched unless asked for.
- A row under way has no checkbox.

### Pickers

Every multi-worktree list (Remove's, Prune's, the Fast-forward and Run pickers, Reparent's selection) has a `wtm/List Header` inside its box: a tri-state box acting on the visible rows, and "k of n selected".

- **From 8 rows**, a `wtm/Search Field` sits above the header, inside the same box. Below 8, the whole list fits and reading beats searching.
- A selected row the search hides stays selected: "2 of 9 selected · 2 hidden by the search". The header's box reflects the visible rows only.
- The body scrolls (`max-h` 90vh); the header and footer stay.

## Callout and Alert

`wtm/Callout`, in the detail panel: the one action of the moment. A tinted surface (the tone's fill at 8%, its stroke at 35%), an icon, a one-line title stating the fact.

- **A problem** is amber: the description, then a full-width primary button with the fix ("Reparent onto main"), then its consequence below in 12 muted ("It rebases onto main on its next sync.").
- **A failure** is red, with the command to copy when the web cannot run the fix (a crashed job).
- **A suggestion** is muted: the action is a small outline button on the title line ("Prune", "Fast-forward").

`wtm/Alert`, first in a dialog's body: a technical problem for the whole plan. The same tinted surface: an icon, a title, a description, and the fix as a `wtm/Command Line` when it is a command (`gh auth login`). Amber when the plan goes on without it (GitHub unavailable, a failed fetch), red when it refuses the plan (a lock held).

## Empty states

`wtm/Empty` (shadcn `Empty`): the command's icon, a title ("Nothing to prune"), a description that states the rule ("A worktree is prunable once its pull request is merged or closed, or origin deleted its branch, and nothing would be lost."), and an action when there is a way forward ("Show all worktrees", or a `CommandLine`). One technical warning goes above it as a `wtm/Alert`. A list that is still loading shows `wtm/Skeleton Row`s under its real header.

## Outcome

`RunOutcome` (`wtm/Run Outcome`, `wtm/Run Outcome Row`): the groups, in this order:

1. a `wtm/Alert` for a run-wide technical problem;
2. "Couldn't …": the items that failed;
3. "To check": business follow-ups (left as they are, orphans, jobs left), each with its action (Reparent, Reparent all);
4. the done group ("Pruned", "Fast-forwarded").

A lone group drops its label. Success contracts to a count in the description. A row folds: its folded line is the code's fixed message worded by the screen ("git refused to move the branch"); opened, it shows the output or `error.params.cause` verbatim, and the log's path when there is a log. Steps and folders sit in `wtm/Details`, folded.

## In Pencil

- **Build from `wtm/` components** and the shadcn kit; never redraw one by hand. A shape drawn twice becomes a component. Every component has at least one instance; one with none is deleted.
- **One area per feature, in flow order** (Components, App shell, Worktrees, Selection, Create, Remove, Prune, Fast-forward, Run a command, Reparent, Explorations), frames left to right in the order the person meets them, each named "Flow — state".
- **One data set:** the 9 worktrees of `acme` (main, feat/billing, feat/billing-ui, fix/login-redirect, chore/deps-october, feat/search, feat/search-docs, spike/gui-poc, feat/onboarding). Each flow fixes their state in its area's note and keeps it through every frame.
- **One matrix per flow:** every state the flow can reach has a frame, and the matrix lists them (data, what is shown, the primary button). A state with no frame is not designed.
- **Frames are dark**; a light frame is drawn where the theme changes something (the shell, the detail). The Components area stays light.
- **Explorations** keep the options weighed for a choice, the chosen one marked.
- The file holds design and components only; proof screenshots go in the PR.

### Pencil traps

Through the `pencil` MCP `execute` tool, never Read or Grep on the `.pen` file:

- Inserting, moving or deleting a node inside a component that sits in other components' slots can drop its instances from those slot overrides (every row of a list vanishes). Count the resolved nodes of the areas before and after; restructure by adding nodes and toggling `enabled` rather than moving them.
- Never edit a slot's default child once instances exist: it resets that slot in every instance.
- A slot whose override is a replacement is addressed by the replacement's own id (`instance/replacementId`), not the slot's; `Replace(path, node)` swaps it.
- A failed call rolls back but can still lose an instance's descendant overrides: re-read what it touched.
- A long call can time out at about 55 s yet apply its work: check before retrying, and keep exports in calls of their own.
- Globals keep strings, not functions: keep helpers in a string and `eval` it in each call.
- A nested instance's text is addressed by its full resolved path.
- The canvas cannot be exported as one node: export each area and stitch them.
