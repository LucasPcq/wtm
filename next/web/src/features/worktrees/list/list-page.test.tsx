import { screen, waitFor } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, onTestFinished, test, vi } from 'vitest'
import { envelope, listDetail } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// Every row's link covered the whole table: a click on any row opened the last one.
test('a row opens its own worktree, not another', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))

  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'feat/billing' }))
})

test('Needs sync keeps the worktrees behind their parent, the search narrows the list, and both clear', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /fix\/login/ })
  expect(screen.getByText('1 changed'), 'an untracked file is a change').toBeTruthy()

  await user.click(screen.getByRole('radio', { name: /Needs sync/ }))
  expect(screen.queryByRole('link', { name: /fix\/login/ })).toBeNull()
  expect(screen.getByRole('link', { name: /feat\/billing-ui/ })).toBeTruthy()

  await user.type(screen.getByRole('textbox', { name: 'Search branches' }), 'nothing')
  expect(await screen.findByText('No worktree matches')).toBeTruthy()

  await user.click(screen.getByRole('link', { name: 'Clear the search and filters' }))
  expect(await screen.findByRole('link', { name: /fix\/login/ })).toBeTruthy()
  expect((screen.getByRole('textbox', { name: 'Search branches' }) as HTMLInputElement).value).toBe('')
})

test('a column hidden in the Columns panel leaves the list, and is stored for this repository', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })
  expect(screen.getByRole('columnheader', { name: 'Running' })).toBeTruthy()

  await user.click(screen.getByRole('button', { name: 'Columns' }))
  await user.click(await screen.findByRole('checkbox', { name: /^Running/ }))

  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()
  expect(localStorage.getItem('wtm.ui')).toContain('"r1":["state","active"]')
})

test('the list opens with the columns stored for its repository', async () => {
  localStorage.setItem('wtm.ui', JSON.stringify({ state: { columns: { r1: ['path'] }, sections: {} }, version: 0 }))

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByRole('columnheader', { name: 'Path' })).toBeTruthy()
  expect(screen.queryByRole('columnheader', { name: 'State' })).toBeNull()
})

// A private window refuses storage: the choice was dropped as soon as it was made, the page reading storage again.
test('a column hidden where the browser refuses storage stays hidden for the page', async () => {
  const setItem = localStorage.setItem.bind(localStorage)
  vi.spyOn(localStorage, 'setItem').mockImplementation((key, value) => {
    if (key === 'wtm.ui') throw new DOMException('refused', 'SecurityError')
    setItem(key, value)
  })
  onTestFinished(() => {
    vi.restoreAllMocks()
  })
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })

  await user.click(screen.getByRole('button', { name: 'Columns' }))
  await user.click(await screen.findByRole('checkbox', { name: /^Running/ }))

  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()
})

// A read cancelled leaves an item with an empty detail: it was taken for a worktree, and the page broke on it.
test('a worktree whose read was cancelled is listed as not read, the others as they are', async () => {
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json({
        ...envelope('list', [listDetail('feat/billing')]),
        items: [
          { subject: 'feat/billing', status: 'done', detail: listDetail('feat/billing') },
          { subject: 'feat/slow', status: 'cancelled', detail: {} },
        ],
      }),
    ),
  )

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByRole('link', { name: /feat\/slow/ })).toBeTruthy()
  expect(screen.getByText('not read')).toBeTruthy()
  expect(screen.getByRole('link', { name: /feat\/billing from main/ })).toBeTruthy()
})
