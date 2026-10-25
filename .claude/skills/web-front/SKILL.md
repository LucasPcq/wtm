---
name: web-front
description: Use when changing wtm's web front (next/web): a screen, a panel, a button, a column, a dialog, a query, the event stream, a form over a command's questions, or reviewing such a change — anything under next/web/src, `make ui`, or next/internal/web/dist.
---

# Working on the web front

The web front is a thin surface over the core: it shows what the catalogue's reads, forms and envelopes return, and runs commands of the catalogue. Its architecture, stack and folders are in `docs/dev/web-front.md`: read it before writing code. This skill is how a change is made here.

## The order of work

1. **Draw it in Pencil first.** A new screen, panel, section, state or control is drawn in `wtm-ui.pen` (the `pencil` MCP) from its `wtm/` components, and validated by the owner, before it is built. A deviation found while coding goes back into Pencil. Pencil holds design only, never proof screenshots.
2. **Find the fact in the core.** Every state, count, problem, available action and its order comes from a read or an envelope. If the screen needs a fact the core does not give, add it to the core's read (Go, under `next/`, the `go-architecture` skill) with its test and golden; never compute it in the front.
3. **Build it** in its feature folder, its logic in plain `.ts` files with Vitest tests, what the user does with it in a component test (below).
4. **See it run**: `make ui-dev` against `wtm-next ui --no-open` in a sandbox (`wtm-sandbox` skill), click through it in a browser.
5. **`make ui`**, then commit the sources with the `dist/` it writes.

## Show what the core says

| The screen needs… | Take it from | Not |
| -- | -- | -- |
| what is wrong with a worktree and how to fix it | `status`'s `problems` (message + `fix`), shown in the detail's action of the moment | a check of your own (`if (detail.missing)`) or a banner at the top |
| what to do next with a worktree | the read's `actions` / follow-ups | a list of commands written in the front |
| a follow-up the catalogue cannot run (clean, sync, go, run …) | `CommandLine` (the server spells it, `POST /api/command-line`), or a menu item that copies it | a button that pretends to run it, or nothing |
| a pull request, its checks, merged | the GitHub tier: `githubQuery`, asked once on open and by Fetch origin | a read `with_prs` on every event, a timer |
| fresh data | the event stream invalidates `['repo', id]` | polling, `refetchInterval` on a read |
| the words of a field: its label, help, option titles, why it is not asked | the screen's own `Record` over the paths, closed values and skip codes the schema publishes (`create/words.ts`) | the core's `label`/`help`/option `label`/skip: the command line's copy |

A problem shows where it is decided, never as a page banner. Colour carries a meaning only (healthy, needs the person, failed), always beside its word.

## Types

The generated types (`src/api/types.gen.ts`) are the only types of the server. A loose envelope is narrowed once, in `src/api/` (`readQuery`'s `select`), never with a cast in a component. A closed set (a phase, a state, a column) is a union, and what each value shows is a `Record` over it, so a new value fails the build.

## Testing a component

Render the page at its address and use it as its user does; mock the server, never the app.

```tsx
test('a row opens its own worktree, not another', async () => {
  const { user, router } = renderApp('/r/r1/worktrees')            // src/test/render.tsx
  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))
  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'feat/billing' }))
})
```

- Queries by role, label or text; actions through `user-event`; assert what the screen shows or the address holds.
- The server's answers are msw handlers (`src/test/server.ts`); another answer for one test is `server.use(http.post(…))`.
- Not `renderHook`, not `vi.mock` of `src/api` or any app module, not a component rendered alone with props invented for it: the linter refuses the first two.

## What the linter refuses

`npm run lint` (part of `make ui`, `make lint-ui`, the CI `ui` gate) fails on: `useEffect`/`useLayoutEffect`/`useInsertionEffect`; `fetch`, `EventSource`, stream parsing outside `src/api/`; `src/api/` importing the app; a feature importing another feature (`@/features/…` or a relative path out of it); a route importing anything but the router and features; `components/` reaching into a feature; React or the app in `lib/`; `renderHook` in a test, `vi.mock` in a component test. A drift the linter misses becomes a rule in `.oxlintrc.json`.

## Common mistakes

- A banner or a toast for a state the core already reports as a problem: show the problem and its fix in the detail.
- A front rule ("missing means broken", "behind means sync") instead of the core's problem or action.
- `x as T` to satisfy a type in a component: type it at the source instead.
- A shadcn component edited in `src/components/ui/`: add it with `npx shadcn@latest add` and wrap it in your feature instead.
- A local state for what belongs in the address (view, filter, open detail) or in Query's cache (a running operation's progress).
