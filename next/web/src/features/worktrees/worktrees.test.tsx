import { cleanup, screen, waitFor, within } from '@testing-library/react'
import { delay, http, HttpResponse } from 'msw'
import { expect, test, vi } from 'vitest'
import { renderApp } from '@/test/render'
import { questionsOf, server } from '@/test/server'

test('each worktree is listed, and its row opens the detail with the problem to fix first and its command', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing-ui/ }))

  const detail = await screen.findByRole('complementary', { name: 'feat/billing-ui' })
  expect(await within(detail).findByText('feat/billing has 1 commit it lacks')).toBeTruthy()
  expect(await within(detail).findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

// Every row's link covered the whole table: a click on any row opened the last one.
test('a row opens its own worktree, not another', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))

  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'feat/billing' }))
})

test('Needs sync keeps the worktrees behind their parent, the search narrows the list, and both clear', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /fix\/login/ })

  await user.click(screen.getByRole('radio', { name: /Needs sync/ }))
  expect(screen.queryByRole('link', { name: /fix\/login/ })).toBeNull()
  expect(screen.getByRole('link', { name: /feat\/billing-ui/ })).toBeTruthy()

  await user.type(screen.getByRole('textbox', { name: 'Search branches' }), 'nothing')
  expect(await screen.findByText('No worktree matches')).toBeTruthy()

  await user.click(screen.getByRole('link', { name: 'Clear the search and filters' }))
  expect(await screen.findByRole('link', { name: /fix\/login/ })).toBeTruthy()
  expect((screen.getByRole('textbox', { name: 'Search branches' }) as HTMLInputElement).value).toBe('')
})

test('create shows a problem only once asked to go on, then confirms the plan, runs it, and ends on where the worktree is and what next', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  await within(dialog).findByRole('textbox', { name: 'Branches' })
  expect(within(dialog).queryByText('branches is required')).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByText('branches is required')).toBeTruthy()

  await user.type(within(dialog).getByRole('textbox', { name: 'Branches' }), 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  // the web's own words, not the command line's; a question only a flag sets is not said to be skipped
  expect(await within(dialog).findByText('Env files')).toBeTruthy()
  expect(within(dialog).getByText('Project default')).toBeTruthy()
  expect(within(dialog).queryByText(/Not asked/)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  // the confirmation reads the plan, not the command line's recap
  const planned = await within(dialog).findByRole('region', { name: 'feat/new' })
  expect(within(dialog).getByRole('heading', { name: 'Create feat/new' })).toBeTruthy()
  expect(within(planned).getByText('Copied from .env.example.')).toBeTruthy()
  expect(within(planned).getByText('Add the worktree · Run the on_create hooks')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('feat/new is ready')).toBeTruthy()
  expect(within(dialog).getByText('/acme.trees/feat/new')).toBeTruthy()
  expect(within(dialog).getByText('wtm go feat/new')).toBeTruthy()
  // a run that went well folds its steps away, one click from them
  expect(within(dialog).queryByText('Run the on_create hooks')).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: /2 steps/ }))
  expect(within(dialog).getByText('Run the on_create hooks')).toBeTruthy()
})

// A branch the core lists with no label of its own was a blank entry of the source branch.
test('an option of no label is offered by its value', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')

  await user.click(await within(dialog).findByRole('combobox', { name: 'Source branch' }))

  expect(await screen.findByRole('option', { name: 'feat/billing' })).toBeTruthy()
})

test("a node's action the web does not run yet shows its command line to copy", async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')

  await user.click(await screen.findByRole('button', { name: 'Sync' }))

  expect(await screen.findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

test('a column hidden in the Columns panel leaves the list, and stays hidden on the next visit', async () => {
  const first = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })
  expect(screen.getByRole('columnheader', { name: 'Running' })).toBeTruthy()

  await first.user.click(screen.getByRole('button', { name: 'Columns' }))
  await first.user.click(await screen.findByRole('checkbox', { name: /^Running/ }))
  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()

  cleanup()
  await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })
  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()
  expect(screen.getByRole('columnheader', { name: 'State' })).toBeTruthy()
})

// A follow-up the command line cannot spell was left out of the done state: it is said, never hidden.
test('a next command the server cannot spell is said with what it meant, not hidden', async () => {
  server.use(http.post('/api/command-line', () => HttpResponse.json({ argv: [] })))
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  await user.type(await within(dialog).findByRole('textbox', { name: 'Branches' }), 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('No command line for this: jump in')).toBeTruthy()
})

// The header read its link from the list's actions, which the live list, never asked GitHub, does not carry.
test("the detail's header links the worktree's pull request, as GitHub tells it on open", async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))

  const detail = await screen.findByRole('complementary', { name: 'feat/billing' })
  const pr = await within(detail).findByRole('link', { name: 'Open pull request #214 on GitHub' })
  expect(pr.getAttribute('href')).toBe('https://github.com/acme/acme/pull/214')
})

// A private window refuses storage: the choice was dropped as soon as it was made, the page reading storage again.
test('a column hidden where the browser refuses storage stays hidden for the page', async () => {
  const setItem = localStorage.setItem.bind(localStorage)
  vi.spyOn(localStorage, 'setItem').mockImplementation((key, value) => {
    if (key.startsWith('wtm.columns')) throw new DOMException('refused', 'SecurityError')
    setItem(key, value)
  })
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })

  await user.click(screen.getByRole('button', { name: 'Columns' }))
  await user.click(await screen.findByRole('checkbox', { name: /^Running/ }))

  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()
  vi.restoreAllMocks()
})

// Each pause in typing asks the core again: a slow answer to older typing overwrote a newer one.
test('an answer to older typing that lands last is dropped, the newer one stands', async () => {
  server.use(
    http.post('/api/repos/r1/commands/create/questions', async ({ request }) => {
      const answers = ((await request.json()) as { answers: { path: string; values: string[] }[] }).answers
      const branches = answers.find((answer) => answer.path === 'branches')?.values[0] ?? ''
      if (branches !== 'old') return HttpResponse.json(questionsOf(answers))
      await delay(700)
      const stale = questionsOf(answers)
      return HttpResponse.json({
        ...stale,
        problems: [{ path: 'branches', code: 'stale', message: 'an older answer' }],
      })
    }),
  )
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  const branches = await within(dialog).findByRole('textbox', { name: 'Branches' })

  await user.type(branches, 'old')
  await new Promise((resolve) => setTimeout(resolve, 400))
  await user.type(branches, 'er')
  await new Promise((resolve) => setTimeout(resolve, 900))

  expect(within(dialog).queryByText('an older answer')).toBeNull()
})

// The page asked GitHub for the list whatever the view: opening the Tree asked it twice, and Fetch origin twice more.
test('the Tree asks GitHub once, and Fetch origin once more', async () => {
  // Fetch origin is offered while the page is live: the stream stays open for the test
  server.use(
    http.get(
      '/api/events',
      () =>
        new HttpResponse(
          new ReadableStream({
            start: (c) => {
              c.enqueue(new TextEncoder().encode(': open\n\n'))
              setTimeout(() => c.close(), 3000)
            },
          }),
          {
            headers: { 'Content-Type': 'text/event-stream' },
          },
        ),
    ),
  )
  const asked: string[] = []
  server.events.on('request:start', async ({ request }) => {
    const body = request.method === 'POST' ? await request.clone().text() : ''
    if (body.includes('"with_prs"')) asked.push(new URL(request.url).pathname.split('/').at(-1) ?? '')
  })
  const { user } = await renderApp('/r/r1/worktrees?view=tree')
  await screen.findByRole('button', { name: 'Sync' })
  await waitFor(() => expect(asked).toEqual(['tree']))

  const fetch = await screen.findByRole('button', { name: 'Fetch origin' })
  await waitFor(() => expect((fetch as HTMLButtonElement).disabled).toBe(false))
  await user.click(fetch)

  await waitFor(() => expect(asked).toEqual(['tree', 'tree']))
  server.events.removeAllListeners()
})
