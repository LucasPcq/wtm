import { act, screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, onTestFinished, test, vi } from 'vitest'
import { operationCap } from '@/api/queries/events'
import { clockEvery, useRuns } from '@/features/worktrees/hooks/use-run'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

const bar = () => screen.getByRole('toolbar', { name: 'Selection' })

test('every worktree has a box, main included; the header checks them all, and says when only some are', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  const every = screen.getByRole('checkbox', { name: 'Select every worktree' })
  expect(every.getAttribute('aria-checked')).toBe('mixed')
  expect(within(bar()).queryByRole('checkbox')).toBeNull()

  await user.click(every)

  expect(screen.getByRole('checkbox', { name: 'Select main' }).getAttribute('aria-checked')).toBe('true')
  expect(within(bar()).getByText('6 selected')).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Clear' }))
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('a worktree filtered out of view leaves the selection', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select fix/login' }))

  await router.navigate({ to: '/r/$repo/worktrees', params: { repo: 'r1' }, search: { q: 'login' } })

  expect(await within(bar()).findByText('1 selected')).toBeTruthy()
})

test('a worktree a run is working on has no box', async () => {
  useRuns.setState({
    'create:r1': {
      running: true,
      progress: { units: [{ subject: 'feat/billing', phases: [{ phase: 'on_create', state: 'now' }], lines: [] }] },
    },
  })
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('checkbox', { name: 'Select every worktree' }))

  expect(screen.queryByRole('checkbox', { name: 'Select feat/billing' })).toBeNull()
  expect(within(bar()).getByText('5 selected')).toBeTruthy()
})

test('the selection bar keeps the same four actions whatever is selected, one it cannot run saying why', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select main' }))

  const buttons = within(bar())
    .getAllByRole('button')
    .map((button) => button.getAttribute('aria-label'))
  expect(buttons.filter(Boolean)).toEqual(['Fast-forward', 'Run a command', 'Reparent', 'Remove'])
  const remove = within(bar()).getByRole('button', { name: 'Remove' })
  expect(remove.hasAttribute('disabled')).toBe(true)

  await user.hover(remove.parentElement as HTMLElement)
  expect((await screen.findAllByText("The main worktree can't be removed")).length).toBeGreaterThan(0)
})

test('Remove on a selection takes what it can remove and leaves the main worktree as it is', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select main' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select feat/billing-ui' }))

  await user.click(within(bar()).getByRole('button', { name: 'Remove' }))

  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing-ui' })).toBeTruthy()
  const left = within(dialog).getByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText("The main worktree can't be removed")).toBeTruthy()

  // removed, the rows it opened on leave the selection, the one left as it is included
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))
  await within(dialog).findByText(/^1 removed\./)
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('a menu opened on a selected row acts on the selection, its one-worktree items say so', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select fix/login' }))

  await user.click(screen.getByRole('button', { name: 'Actions on fix/login' }))

  const menu = await screen.findByRole('menu')
  expect(within(menu).getByText('2 selected worktrees')).toBeTruthy()
  expect(within(menu).getByRole('menuitem', { name: /Go to the worktree/ }).textContent).toContain(
    'One worktree at a time',
  )
  expect(within(menu).getByRole('menuitem', { name: 'Remove' }).getAttribute('aria-disabled')).toBeNull()
})

test('a worktree another surface is removing shows it, has no box, and its menu stays closed until it ends', async () => {
  const acme = `"ts":"${new Date().toISOString()}","repo":{"root":"/acme","common_dir":"/acme/.git"},"op_id":"op-9"`
  server.use(
    http.get('/api/events', () => {
      const stream = new ReadableStream({
        start: (controller) => {
          const send = (line: string) => controller.enqueue(new TextEncoder().encode(`data: ${line}\n\n`))
          send(`{"v":2,"type":"operation.started",${acme},"command":"clean","surface":"cli"}`)
          send(`{"v":2,"type":"unit.started",${acme},"subject":"feat/search"}`)
        },
      })
      return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream' } })
    }),
  )
  await renderApp('/r/r1/worktrees')

  expect(await screen.findByText('Removing')).toBeTruthy()
  expect(screen.queryByRole('checkbox', { name: 'Select feat/search' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Actions on feat/search' }).hasAttribute('disabled')).toBe(true)
})

// The menu read the live list, which never asked GitHub: a worktree with an open pull request said it had none.
test('a row whose pull request GitHub knows offers to open it, as it lands', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on feat/billing' }))

  const open = await screen.findByRole('menuitem', { name: /Open the pull request/ })
  await waitFor(() => expect(open.getAttribute('aria-disabled')).toBeNull())
  expect(open.textContent).not.toContain('has no pull request')
})

// A menu opened before its worktree went under way stayed open, its Remove still there to press.
test('a menu open on a row closes when the row goes under way, and the header leaves it out', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions on feat/billing' }))
  expect(await screen.findByRole('menu')).toBeTruthy()

  act(() =>
    useRuns.setState({
      'create:r1': {
        running: true,
        progress: { units: [{ subject: 'feat/billing', phases: [{ phase: 'on_create', state: 'now' }], lines: [] }] },
      },
    }),
  )

  await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  await user.click(screen.getByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  await within(dialog).findByRole('listitem', { name: 'feat/search' })
  expect(within(dialog).queryByRole('listitem', { name: 'feat/billing' })).toBeNull()
})

// The cap was read on a render only: in a quiet repository, a process killed mid-way greyed its row for good.
test('a row under way from an operation that never finished comes back once the cap passes, with no event', async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  onTestFinished(() => {
    vi.useRealTimers()
  })
  const ts = new Date(Date.now() - operationCap + clockEvery / 2).toISOString()
  const of = `"ts":"${ts}","repo":{"root":"/acme","common_dir":"/acme/.git"},"op_id":"op-8"`
  server.use(
    http.get('/api/events', () => {
      const stream = new ReadableStream({
        start: (controller) => {
          const send = (line: string) => controller.enqueue(new TextEncoder().encode(`data: ${line}\n\n`))
          send(`{"v":2,"type":"operation.started",${of},"command":"clean","surface":"cli"}`)
          send(`{"v":2,"type":"unit.started",${of},"subject":"feat/search"}`)
        },
      })
      return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream' } })
    }),
  )
  await renderApp('/r/r1/worktrees')
  expect(await screen.findByText('Removing')).toBeTruthy()

  await act(() => vi.advanceTimersByTimeAsync(clockEvery))

  await waitFor(() => expect(screen.queryByText('Removing')).toBeNull())
  expect(screen.getByRole('checkbox', { name: 'Select feat/search' })).toBeTruthy()
})

const creating = (subject: string, running: boolean) =>
  act(() =>
    useRuns.setState({
      'create:r1': {
        running,
        progress: { units: [{ subject, phases: [{ phase: 'on_create', state: 'now' }], lines: [] }] },
      },
    }),
  )

// The menus kept their open state while closed: once the run ended, they opened again by themselves.
test('a menu closed by a run stays closed once the run ends: the row, its context menu, the detail', async () => {
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')
  const detail = await screen.findByRole('complementary', { name: 'feat/billing' })

  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/billing' }))
  expect(await screen.findByRole('menu')).toBeTruthy()
  creating('feat/billing', true)
  await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  creating('feat/billing', false)
  expect(screen.queryByRole('menu')).toBeNull()

  await user.pointer({ keys: '[MouseRight]', target: screen.getByRole('link', { name: /feat\/search from/ }) })
  expect(await screen.findByRole('menu')).toBeTruthy()
  creating('feat/search', true)
  await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  creating('feat/search', false)
  await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Select feat/search' })).toBeTruthy())
  expect(screen.queryByRole('menu')).toBeNull()
})

test('a menu on a selection leaves out the selected rows under way', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select fix/login' }))
  creating('feat/billing', true)

  await user.click(await screen.findByRole('button', { name: 'Actions on fix/login' }))

  const menu = await screen.findByRole('menu')
  expect(within(menu).queryByText(/selected worktrees/)).toBeNull()
  expect(
    within(menu)
      .getByRole('menuitem', { name: /Go to the worktree/ })
      .getAttribute('aria-disabled'),
  ).toBeNull()
})

test('a worktree an operation works on, whose start the page did not see, reads Busy', async () => {
  const of = `"ts":"${new Date().toISOString()}","repo":{"root":"/acme","common_dir":"/acme/.git"},"op_id":"op-7"`
  server.use(
    http.get('/api/events', () => {
      const stream = new ReadableStream({
        start: (controller) =>
          controller.enqueue(
            new TextEncoder().encode(`data: {"v":2,"type":"unit.started",${of},"subject":"feat/search"}\n\n`),
          ),
      })
      return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream' } })
    }),
  )

  await renderApp('/r/r1/worktrees')

  expect(await screen.findByText('Busy')).toBeTruthy()
  expect(screen.queryByRole('checkbox', { name: 'Select feat/search' })).toBeNull()
})

test("the selection bar's actions leave out the selected rows under way", async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select fix/login' }))
  creating('feat/billing', true)
  expect(within(bar()).getByText('2 selected')).toBeTruthy()

  await user.click(within(bar()).getByRole('button', { name: 'Remove' }))

  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByRole('listitem', { name: 'fix/login' })).toBeTruthy()
  expect(within(dialog).queryByRole('listitem', { name: 'feat/billing' })).toBeNull()
})
