---
name: web-front
description: How wtm's web front (next/web, the React app `wtm-next ui` serves) is written, laid out, tested and linted. Use it whenever you add, change or review anything under next/web — a screen, panel, column, dialog, form over a command's questions, query, the event stream, a store, a test, the Biome config or its GritQL rules — and whenever you run `make ui`, touch next/internal/web/dist, or wonder where front code goes or how a component should be written, even for a one-line fix.
---

# The web front

`next/web` is a thin surface over wtm's core: it shows what the catalogue's reads, forms and envelopes return, and runs commands of the catalogue through the server's API. **It decides nothing the core decides.** Everything below follows from that, and from one more goal: a reader opening any file understands it in one pass.

Stack: React 19, Vite, TypeScript 7 (strict, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`), shadcn/ui on Tailwind v4, TanStack Router (file routes), Query, Form and Table (v9), zustand, `@dnd-kit/react`, `eventsource-parser`, Biome (lint + format, GritQL plugins), Vitest + Testing Library + msw. Build, dev mode and embedding: `docs/dev/web-front.md`.

## Order of work

1. **Design in Pencil first.** A new screen, panel, state or control is drawn in `wtm-ui.pen` (the `pencil` MCP only: never Read or Grep the file) from its `wtm/` components, and validated by the owner before it is built. A deviation found while coding goes back into Pencil in the same change. The file holds design and components only, never proof screenshots (those go in the PR).
2. **Find the fact in the core.** A state, a count, a problem, an available action and its order arrive in a read or an envelope. If the screen needs one the core does not give, add it to the core (Go, `go-architecture` skill) with its test and golden, never compute it in the front.
3. **Build** it in its feature folder; write a failing test first for a bug (below).
4. **See it run**: `make ui-dev` against `wtm-next ui --no-open` in a sandbox (`wtm-sandbox` skill). In a sandbox, stop the server with `POST /api/stop` and its token, never by killing a pid.
5. **`make ui`**, then commit the sources with the `dist/` it writes.

Read `references/design.md` before designing or building any screen: the dialog anatomy, the actions model, colour, rows and pickers, callouts, empty states and outcomes, each with its `wtm/` component.

## Where code goes

```
src/
  api/                 the server, and only here: fetch.ts, token.ts, types.gen.ts, queries/<resource>.ts
  components/ui/       shadcn's code, added by its CLI or MCP, never edited
  hooks/               shadcn's hooks (use-mobile)
  components/          what two features share (page header, state pages, logo, command-form/)
  features/<feature>/  one area, in the shape below
  routes/              thin: a path and the feature it renders
  lib/                 pure helpers: no React, nothing of the app
  test/                the test kit: render.tsx, server.ts (msw), fixtures.ts, setup.ts
  main.tsx             the entry; routeTree.gen.ts is generated beside it
```

### Inside a feature: one shape at every level

```
features/worktrees/
  worktrees-page.tsx         the feature's entry, the one file at its root
  components/  hooks/  utils/  store/    what the feature's own entry and two or more sub-features use
  list/                      a sub-feature (list, tree, detail, create)
    list-view.tsx            its entry component, the one file at its root
    components/              .tsx only; components/__tests__/ holds the component tests
    hooks/                   use-*.ts only, never a test
    utils/                   plain functions (rows, steps, words…); utils/__tests__/ their tests
  store/ui-store.ts          at the feature's root only
```

- A file only one sub-feature uses stays in it; one that two sub-features, or the feature's entry and a sub-feature, use moves up to the feature's own `components/`, `hooks/` or `utils/`. There is no `shared/`.
- **A test lives in the `__tests__/` of `components/` or `utils/`**, beside the code it tests: never at a root, never beside its code, never in `hooks/` (a hook is tested through a component that uses it, in `components/__tests__/`), never in `store/`. A page-level test of an entry component goes in its sub-feature's `components/__tests__/`. Outside features a test sits in a `__tests__/` too: `api/queries/__tests__/`, `components/command-form/__tests__/`, `lib/__tests__/`.
- The `unknown-folder` lint refuses any other folder or kind of file in a feature (a root holds `.tsx` only, never a test), and any test outside a `__tests__/`. That a root holds one entry and nothing more is a review rule: GritQL sees one file at a time and cannot count.

- **Imports between folders go through `@/`; `./` stays for the same folder; `../` never.** Biome matches the raw import text and resolves nothing, so `@/features/x/…` is what lets the layer rules see where an import lands.
- **A layer imports only what it may**: `api/` nothing of the app; `lib/` nothing of the app, no React; a route the router, features and Query's types; `components/` no feature; a feature its own folder, `components/`, `lib/`, `api/`, `hooks/`, never another feature or a route. Code moves to `components/` or `lib/` on the second feature that needs it, not before.
- **A file is named after what it declares**, kebab-case (`node-row.tsx` declares `NodeRow`); a hook's file starts with `use-` and lives in `hooks/`.
- **One exported component per file.** A small sub-component only that component uses may live in its file, unexported, like a helper function (`ListView`'s empty states, the sidebar's `WorktreeCount`); once a second component uses it, it moves to its own file. A file stays under 300 lines (the lint enforces it): a big one splits by what it draws, as the detail panel is a panel plus one file per section.

## Writing a component

These are the owner's rules, and the reason behind each.

- **Props are destructured in the parameters**, so the signature says what the component uses:
  ```tsx
  export function Path({ path, className }: { path: string; className?: string }) {
  ```
- **A function only one component uses lives inside it**, beside what it serves. One used elsewhere gets its own file. Never a helper loose next to a component: the reader has to find out who calls it. A pure function worth a unit test (`progress.ts`, `rows.ts`) gets its own `.ts` file.
- **No nested ternary.** Use a small named function with early returns:
  ```tsx
  const title = () => {
    if (running) return progress.cancelling ? `Cancelling ${names}` : `Creating ${names}`
    if (cancelled) return `Cancelled ${names}`
    return `${names} is ready`
  }
  ```
- **Blank lines between blocks**: the early returns of intermediate states, the derived constants, the inner functions, then the JSX. A component reads like paragraphs.
- **Comments as few as possible.** These are not Go doc comments: no `// X is …` above a declaration, no comment that describes a constant or restates the code. Keep one only where the code cannot show its why: a CSS workaround, an ordering a reader would undo, a rule from the product. (The lint cannot check this one: it is a review rule.)
- **No effects.** Derive while rendering; data and side effects go through TanStack Query, Router and Form; a DOM hook (a listener, a scroll) goes in a ref callback that returns its cleanup; an outside store goes through `useSyncExternalStore` or zustand. The lint refuses `useEffect`, `useLayoutEffect` and `useInsertionEffect` by any spelling.
- **`useReducer` when several states move together** (a wizard's step and its remembered answers), one `useState` per independent value, never two that must stay in sync.
- **A closed set is a union, and what each value shows is a `Record` over it**, so a new value fails the build: `conditionLook: Record<WorktreeCondition, …>`, `phaseLabel: Record<DomainPhase, string>`. Classes from a state go through `cva` variants or such a Record, never a free string built by hand.
- **Reusable logic becomes a hook or a component.** `useRead(repo, command)` is a view's two reads, `useChoice` a viewer's choice per repository, `CommandLine` every follow-up to copy, `ChoicePopover` every show/hide chooser. Before writing a second copy of something, look in the feature's own `components/`, `hooks/` and `utils/`, then `src/components/`.
- **shadcn only through its CLI or MCP** (`npx shadcn@latest add <name>`); wrap or compose it in your feature, never edit `components/ui/`. Plain shadcn, the logo, `--primary` as the signature colour, Inter, the system's theme.

## Where state lives

| State | Lives in |
| -- | -- |
| anything the server says (reads, forms, dry runs, the event stream) | TanStack Query, through `src/api/queries/` |
| what is linkable (view, filter, search, the worktree open) | the route's search params (`features/worktrees/utils/search.ts`) |
| a viewer's own choices (columns, detail sections, other UI options) | a zustand store with `persist` (`features/worktrees/store/ui-store.ts`, key `wtm.ui`), per repository, the same in every tab |
| an operation this page runs (create's, clean's progress) | a zustand store without persist (`hooks/use-run.ts`), read by the dialog and the list |
| what one component alone needs for a moment (a popover open, typing not yet sent) | `useState` / `useReducer` |

Never server data in zustand, never a choice of the viewer in Query, never `localStorage` by hand. The persisted store's storage swallows a refused write (a private window): the choice then lives in memory until the page closes. A choice made in another tab reaches this one: the store's file listens to the `storage` event for its key and rehydrates (`persist.rehydrate()`), at module level, no effect.

## Showing what the core says

| The screen needs… | Take it from | Not |
| -- | -- | -- |
| what is wrong with a worktree and how to fix it | `status`'s `problems` (message and `fix`), in the detail | a check of your own (`if (detail.missing)`), a page banner |
| a worktree's condition and what to do first | the read's `condition` and `actions`, ranked by the core | a ranking of your own (`if (missing) … else if (rebasing)`) |
| one worktree over its live and GitHub reads | `overlaid(live, github)` (`features/worktrees/utils/condition.ts`), the one rule: a pull request GitHub says is merged wins over changed and clean, with that read; anything else is the live read | a second merge rule anywhere |
| a follow-up the web cannot run (clean, sync, go, reparent…) | `CommandLine` (the server spells it, `POST /api/command-line`), inline or as a block | a button that pretends to run it, or nothing |
| a pull request, its checks | `githubQuery`, asked once per view on screen, when the detail opens, and after Fetch origin | `with_prs` on every event, a timer |
| fresh data | the event stream invalidating `['repo', id]` | polling, `refetchInterval` on a read |
| the words of a field, an option, a reason a field is not asked | the screen's own exhaustive `Record` over the paths, values and skip codes the schema publishes (`create/utils/words.ts`) | a fallback to the path or the value; the core publishes no words, only paths, values, option facts and skip codes |

A problem shows where it is decided, never as a banner or a toast. Colour carries a meaning only (healthy, needs the person, failed, nothing to say), always beside its word (`Glyph`).

## Types

`src/api/types.gen.ts` (generated from the API schema by `npm run types`) holds every type of the server; write none by hand. An envelope is narrowed once, in `src/api/` (`narrowed()`, which also turns an item's empty `{}` detail into `undefined`), never cast in a component. Type at the source: `x as T` in a component is a bug waiting.

## Tests, briefly

- **Logic** in `utils/` gets a Vitest test in `utils/__tests__/` (`list/utils/__tests__/list.test.ts`), with builders from `src/test/fixtures.ts`.
- **Components, and the hooks they use,** are tested by rendering the whole app at an address (`await renderApp('/r/r1/worktrees')`) and using it as its user does: queries by role, label or text, actions through `user-event`, assertions on the screen or the address. The server is msw (`src/test/server.ts`); one test changes an answer with `server.use(...)`.
- **Never** `renderHook`, `vi.mock`/`vi.doMock`/`vi.stubGlobal`, or `vi.spyOn` on a module of the app: the lint refuses them. Spying on a browser object (`localStorage`) is fine.
- **A bug fix starts with a test that fails** for the bug's reason, named after what the user saw.

Read `references/testing.md` before writing or fixing a test: the kit, the msw patterns, the stores' reset, and the limits of happy-dom.

## The lint

`npm run lint` (`biome check --error-on-warnings .`) formats, organises imports and lints. It is part of `npm run build`, `make ui`, `make lint-ui` and the CI `ui` gate. `npm run format` writes the fixes. Read `references/lint.md` before changing `biome.json` or a GritQL rule in `lint/`, or when a rule refuses your code and you don't see why. Above all: **a GritQL plugin that breaks passes silently**, so `lint/rules.test.ts` holds a table of cases, each a small file and the plugin that must refuse it, which it lints with the real `biome.json`.

## The server, the stream, forms, operations

How the token works, the API layer, reads and GitHub on demand, follow-ups, the event stream, streamed operations and their cancel, forms over a command's questions and create's words and steps: `references/server.md`. Read it before touching `src/api/`, a query, a form or create.

## Common mistakes

- A state the core reports as a problem shown as a banner or a toast: show the problem and its fix in the detail.
- A front rule ("missing means broken", "behind means sync") in place of the core's `condition`, problem or action.
- A second view's read kept alive under the one on screen: only the view on screen reads (`useRead(…, enabled)`).
- A `../` import, or an `@/` path with `..` in it.
- A helper beside a component, props not destructured, a nested ternary, a doc comment restating a name.
- A server fact copied into zustand, or a viewer's choice in `localStorage` by hand.
- A test that mocks a module, or a fixture builder copied into a test file instead of `src/test/fixtures.ts`.
