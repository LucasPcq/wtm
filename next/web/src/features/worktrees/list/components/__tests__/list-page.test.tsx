import { act, screen, waitFor } from '@testing-library/react'
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

test('a row whose ⋯ menu is open stays marked, and the menu opens on its branch', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  const more = await screen.findByRole('button', { name: 'Actions on fix/login' })

  await user.click(more)

  const menu = await screen.findByRole('menu')
  expect(menu.firstElementChild?.textContent).toBe('fix/login')
  expect(more.closest('tr')?.hasAttribute('data-menu-open')).toBe(true)
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
  const refused = vi.spyOn(localStorage, 'setItem').mockImplementation((key, value) => {
    if (key === 'wtm.ui') throw new DOMException('refused', 'SecurityError')
    setItem(key, value)
  })
  onTestFinished(() => refused.mockRestore())
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

test('a worktree git stopped in the middle of a merge says merging, as a rebase says rebasing', async () => {
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json(
        envelope('list', [
          listDetail('feat/merging', { parent: 'main', behind: 2, needs_sync: false, condition: 'merging' }),
          listDetail('feat/rebasing', {
            parent: 'main',
            behind: 2,
            needs_sync: false,
            condition: 'rebasing',
          }),
        ]),
      ),
    ),
  )

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByText('merging')).toBeTruthy()
  expect(screen.getByText('rebasing')).toBeTruthy()
  expect(screen.queryByText(/2 behind/)).toBeNull()
})

// With Branch alone shown, a worktree not read had no cell left to say so.
test('a worktree not read says so under its branch when no other column is shown', async () => {
  localStorage.setItem('wtm.ui', JSON.stringify({ state: { columns: { r1: [] }, sections: {} }, version: 0 }))
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json({
        ...envelope('list', []),
        items: [
          { subject: 'main', status: 'done', detail: listDetail('main', { main: true }) },
          { subject: 'feat/slow', status: 'cancelled', detail: {} },
        ],
      }),
    ),
  )

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByRole('link', { name: /feat\/slow not read/ })).toBeTruthy()
})

// The columns chosen in another tab stayed as this tab had read them, until a reload.
test('a column choice made in another tab shows in this one', async () => {
  await renderApp('/r/r1/worktrees')
  await screen.findByRole('columnheader', { name: 'State' })

  localStorage.setItem('wtm.ui', JSON.stringify({ state: { columns: { r1: ['path'] }, sections: {} }, version: 0 }))
  act(() => {
    window.dispatchEvent(new StorageEvent('storage', { key: 'wtm.ui' }))
  })

  expect(await screen.findByRole('columnheader', { name: 'Path' })).toBeTruthy()
  expect(screen.queryByRole('columnheader', { name: 'State' })).toBeNull()
})

// Clear, pressed while a search was still to be sent, was undone by it a moment later.
test('clearing the search while typing is still to be sent keeps it cleared', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')
  const box = await screen.findByRole('textbox', { name: 'Search branches' })
  await user.type(box, 'nothing')
  await screen.findByText('No worktree matches')

  await user.type(box, 'x')
  await user.click(screen.getByRole('link', { name: 'Clear the search and filters' }))
  await new Promise((resolve) => setTimeout(resolve, 400))

  expect(router.state.location.search).toEqual({})
  expect((box as HTMLInputElement).value).toBe('')
})

// Back from ?q=abc to ?q=ab left "abc" in the box: only a search cleared reached it.
test('the search box follows the address when it changes from elsewhere, back and forward included', async () => {
  const { router } = await renderApp('/r/r1/worktrees?q=abc')
  const box = (await screen.findByRole('textbox', { name: 'Search branches' })) as HTMLInputElement
  expect(box.value).toBe('abc')

  await act(() => router.navigate({ to: '/r/$repo/worktrees', params: { repo: 'r1' }, search: { q: 'ab' } }))

  await waitFor(() => expect(box.value).toBe('ab'))
})

// GitHub out of reach answered no pull request for every row, and nothing said why.
test('a list whose GitHub could not be asked says why, and claims no pull request', async () => {
  localStorage.setItem('wtm.ui', JSON.stringify({ state: { columns: { r1: ['pr'] }, sections: {} }, version: 0 }))
  server.use(
    http.post('/api/repos/r1/commands/list', async ({ request }) => {
      if (!(await request.clone().text()).includes('with_prs')) return undefined
      return HttpResponse.json({
        ...envelope('list', [listDetail('main', { main: true }), listDetail('feat/billing')]),
        warnings: [{ code: 'list.prs_unavailable', message: 'pull requests unavailable: run `gh auth login`' }],
      })
    }),
  )

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByText('Pull requests unavailable: run `gh auth login`')).toBeTruthy()
  expect(screen.queryByText('none')).toBeNull()
})
