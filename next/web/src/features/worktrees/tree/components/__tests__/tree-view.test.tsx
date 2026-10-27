import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, onTestFinished, test } from 'vitest'
import { envelope, treeDetail } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { liveEvents, server } from '@/test/server'

// The reads each POST asks, by command, with GitHub or without
function recordReads() {
  const asked: string[] = []
  server.events.on('request:start', async ({ request }) => {
    if (request.method !== 'POST' || !request.url.includes('/commands/')) return
    const command = new URL(request.url).pathname.split('/').at(-1) ?? ''
    asked.push((await request.clone().text()).includes('"with_prs"') ? `${command}+github` : command)
  })
  onTestFinished(() => server.events.removeAllListeners())
  return asked
}

test('a node shows its state, never its verbs: its actions are in its menu, a copied one says what it copied', async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')
  const node = (await screen.findByRole('link', { name: 'feat/billing-ui' })).parentElement as HTMLElement

  expect(within(node).queryByRole('button', { name: 'Sync' })).toBeNull()
  await user.pointer({ keys: '[MouseRight]', target: node })
  await user.click(await screen.findByRole('menuitem', { name: /Sync/ }))

  expect(await screen.findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

test("the main worktree's menu keeps every action in place, those it can't take disabled with why", async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')

  await user.pointer({ keys: '[MouseRight]', target: await screen.findByRole('link', { name: 'main' }) })

  const menu = await screen.findByRole('menu')
  const order = [
    'Fast-forward',
    'Sync',
    'Reparent',
    'Run a command',
    'Start the profile',
    'Stop the profile',
    'View the logs',
    'Go to the worktree',
    'Open the pull request',
    'Remove',
  ]
  const items = within(menu).getAllByRole('menuitem')
  expect(items.map((item) => order.find((label) => item.textContent?.startsWith(label)))).toEqual(order)
  const remove = within(menu).getByRole('menuitem', { name: /Remove/ })
  expect(remove.getAttribute('aria-disabled')).toBe('true')
  expect(remove.textContent).toContain("The main worktree can't be removed")
  expect(within(menu).getByRole('menuitem', { name: /Open the pull request/ }).textContent).toContain(
    'main has no pull request',
  )
  expect(within(menu).getByRole('menuitem', { name: /Reparent/ }).textContent).toContain('Not in the web yet')
})

test('the menu key on a focused node opens its menu', async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')

  ;(await screen.findByRole('link', { name: 'feat/search' })).focus()
  await user.keyboard('{Shift>}{F10}{/Shift}')

  expect(await screen.findByRole('menu')).toBeTruthy()
  const more = screen.getByRole('button', { name: 'Actions on feat/search', hidden: true })
  expect(more.getAttribute('aria-expanded')).toBe('true')
})

// The page asked GitHub for the list whatever the view: opening the Tree asked it twice, and Fetch origin twice more.
test('the Tree asks GitHub once, and Fetch origin once more', async () => {
  server.use(liveEvents)
  const asked = recordReads()
  const { user } = await renderApp('/r/r1/worktrees?view=tree')
  await screen.findByRole('button', { name: 'Actions on feat/billing-ui' })
  await waitFor(() => expect(asked.filter((read) => read.endsWith('+github'))).toEqual(['tree+github']))

  const fetch = await screen.findByRole('button', { name: 'Fetch origin' })
  await waitFor(() => expect((fetch as HTMLButtonElement).disabled).toBe(false))
  await user.click(fetch)

  await waitFor(() => expect(asked.filter((read) => read.endsWith('+github'))).toEqual(['tree+github', 'tree+github']))
})

// The page read the live list under the Tree too: two reads of git for each event of the stream.
test('the Tree reads the tree only, not the list besides', async () => {
  const asked = recordReads()

  await renderApp('/r/r1/worktrees?view=tree')
  await screen.findByRole('button', { name: 'Actions on feat/billing-ui' })

  expect(asked.filter((read) => read.startsWith('list'))).toEqual([])
})

// A worktree git could not read offered a menu whose every item said the web did not run it yet.
test('a node git could not read has no menu to open', async () => {
  server.use(
    http.post('/api/repos/r1/commands/tree', () =>
      HttpResponse.json(
        envelope('tree', [
          treeDetail('main', { main: true, parent: null }),
          treeDetail('feat/broken', { actions: [], condition: 'unreadable' }),
        ]),
      ),
    ),
  )
  await renderApp('/r/r1/worktrees?view=tree')

  await screen.findByRole('button', { name: 'Actions on main' })
  expect(screen.queryByRole('button', { name: 'Actions on feat/broken' })).toBeNull()
})

// Radix skips a disabled item: a keyboard never reached why the main worktree can't be removed.
test('the keyboard reaches a disabled item and its reason, and pressing it does nothing', async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')
  await user.pointer({ keys: '[MouseRight]', target: await screen.findByRole('link', { name: 'main' }) })
  await screen.findByRole('menu')

  await user.keyboard('{End}')

  const focused = document.activeElement as HTMLElement
  expect(focused.getAttribute('aria-disabled')).toBe('true')
  expect(focused.textContent).toContain("The main worktree can't be removed")
  await user.keyboard('{Enter}')
  expect(screen.queryByRole('dialog', { name: 'Remove worktrees' })).toBeNull()
  expect(screen.getByRole('menu')).toBeTruthy()
})

test("a node offers to open its pull request once GitHub's read lands", async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')

  await user.pointer({ keys: '[MouseRight]', target: await screen.findByRole('link', { name: 'feat/billing' }) })

  const open = await screen.findByRole('menuitem', { name: /Open the pull request/ })
  await waitFor(() => expect(open.getAttribute('aria-disabled')).toBeNull())
})
