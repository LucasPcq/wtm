# Testing the web front

Vitest runs `src/**/*.test.{ts,tsx}` and `lint/*.test.ts` under happy-dom (`npm test`). Two kinds of tests, one kit.

## Logic tests (`*.test.ts`)

A non-trivial pure function — rows and filters, the tree's nodes, create's steps and progress, `overlaid`, a sentence — is called with values and its result compared. Build the values with `src/test/fixtures.ts` (`listDetail`, `treeDetail`, `statusDetail`, `followUp`, `read`, `envelope`): never a copy of a builder in a test file.

```ts
const live = read('list', [listDetail('feat/a', { condition: 'changed', dirty: true })])
const github = read('list', [listDetail('feat/a', { condition: 'merged', merged: true })])

expect(rowsOf({ live, github, readAt: 0, underWay: {} })[0]?.condition).toBe('merged')
```

## Component tests (`*.test.tsx`)

One file per screen or dialog, in its sub-feature's `components/__tests__/` (`list/components/__tests__/list-page.test.tsx`). A hook has no test of its own: the component that uses it is tested, and the hook with it. Render the whole app at an address and use it as its user does:

```tsx
test('a row opens its own worktree, not another', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))

  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'feat/billing' }))
})
```

- `renderApp(path)` (`src/test/render.tsx`) is **awaited**: it sets the token, rehydrates the persisted store from `localStorage` (what a page does when it opens), and loads the route's code-split modules before rendering, so a test's wait measures the screen, not the import.
- Query by role, label or text; act through `user-event`; assert what the screen shows or what the address holds. A helper that opens a dialog belongs at the top of its test file (`openCreate()`).
- Name a regression test after what the user saw: `a row opens its own worktree, not another`.
- Clean up inside the test with `onTestFinished(() => { … })`, never with a line at its end that a failed assertion skips.

## The server: msw

`src/test/server.ts` is the API of `wtm-next ui` over one repository, `acme`: a stack (`feat/billing`, `feat/billing-ui` behind it), a worktree with a change (`fix/login`), their list, tree and status reads (a pull request only when asked `with_prs`), create's questions (an existing branch is not asked its environment), dry run and stream, the spelling of a command line, and an event stream that says nothing. An unhandled request fails the test (`onUnhandledFrame: 'error'`).

- One test changes one answer: `server.use(http.post('/api/repos/r1/commands/status', …))`. A handler that returns `undefined` falls through to the next; read the body with `request.clone().json()` then, or the next handler finds it used.
- `liveEvents` keeps the event stream open (for a test that needs the page live, such as Fetch origin); `streamed(body, operation)` answers an operation's stream; `line(event)` writes one of its lines.
- Count requests with `server.events.on('request:start', …)` and remove the listener in `onTestFinished`.

## The stores

zustand stores outlive a render, as they outlive a page in the browser. `src/test/setup.ts` clears `localStorage` and resets every store to its initial state after each test (`useUi.setState(useUi.getInitialState(), true)`). A new store gets its reset line there. Another tab is a `storage` event: write the key, then `window.dispatchEvent(new StorageEvent('storage', { key: 'wtm.ui' }))` inside `act`.

A spy on a browser object is restored by its own handle (`const refused = vi.spyOn(…); onTestFinished(() => refused.mockRestore())`): `vi.restoreAllMocks()` let one leak into the next test.

## What is refused, and what to do instead

| Refused (by the lint) | Instead |
| -- | -- |
| `renderHook` | render the page that uses the hook and act on it |
| `vi.mock`, `vi.doMock` of any module | answer differently from msw |
| `vi.stubGlobal` | msw, or the real browser API happy-dom provides |
| `vi.spyOn` on a module of the app | msw; spying on a browser object (`localStorage`) is fine |

A token a test needs is set in `localStorage` (`localStorage.setItem('wtm.token', 't')`), not mocked.

## The limits of happy-dom and msw

- A response stream's `cancel()` makes happy-dom throw on its next chunk: close a stream from the client with an `AbortController` on the request instead.
- msw relays neither a client's cancel nor its abort to a handler that already answered: a test cannot see a connection close. Say so in the PR when a fix depends on it.
- There is no layout: what depends on CSS (a link above another, a truncation) is proved in a real browser (Chrome, `document.elementFromPoint`), in the PR.
- A test that spawns a process (the lint guard) gets a longer timeout: a cold start on a busy runner takes seconds.
