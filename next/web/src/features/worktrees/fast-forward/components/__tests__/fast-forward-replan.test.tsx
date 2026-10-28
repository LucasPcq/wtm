import { act, screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { ffPlan, ffRun, ffUnsafe } from '@/test/fast-forward'
import { answeringOn, fetching, fromSelection, recorded } from '@/test/fast-forward-app'
import { liveEvents } from '@/test/remove'
import { server } from '@/test/server'

test('a tick plans again without fetching origin: one fetch per flow', async () => {
  const calls = recorded()
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui', 'feat/search')

  const box = await within(dialog).findByRole('checkbox', { name: 'Fast-forward feat/billing-ui' })
  await user.click(box)
  await user.click(box)
  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' }))

  expect(await within(dialog).findByText('2 fast-forwarded.')).toBeTruthy()
  expect(calls.dryRuns().filter(fetching)).toHaveLength(1)
  expect(calls.runs().filter(fetching)).toEqual([])
})

test('where each moves follows an untick at once, while the plan is made again', async () => {
  let held = false
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (held && !fetching(call)) await new Promise(() => {})
      return call.dry_run ? ffPlan(call) : ffRun(call)
    }),
  )
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui')
  await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' })

  held = true
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward feat/billing-ui' }))
  await user.click(within(dialog).getByRole('button', { name: /^Details/ }))

  expect(within(dialog).getByRole('region', { name: 'main' })).toBeTruthy()
  expect(within(dialog).queryByRole('region', { name: 'feat/billing-ui' })).toBeNull()
})

test('a tick given while a worktree was clean no longer holds once it has changes', async () => {
  const events = liveEvents()
  const dirty = ['fix/login']
  answeringOn(dirty)
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui')
  const box = await within(dialog).findByRole('checkbox', { name: 'Fast-forward feat/billing-ui' })
  await user.click(box)
  await user.click(box)
  expect(box.getAttribute('data-state')).toBe('checked')

  dirty.push('feat/billing-ui')
  act(() => events.changed())

  await waitFor(() =>
    expect(
      within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing-ui' }).getAttribute('data-state'),
    ).toBe('unchecked'),
  )
})

// a plan made again on a tick reads the worktrees afresh: it can see a change before the event that refreshes the review
test('a plan forced past a worktree the person did not check never enables Fast-forward', async () => {
  const dirty = ['fix/login']
  answeringOn(dirty)
  const { user, dialog } = await fromSelection('fix/login', 'feat/billing-ui', 'main')
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' }))
  const ready = await within(dialog).findByRole('button', { name: 'Fast-forward 3 worktrees' })
  await waitFor(() => expect(ready.getAttribute('aria-disabled')).toBe('false'))

  dirty.push('feat/billing-ui')
  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward main' }))
  const button = within(dialog).getByRole('button', { name: 'Fast-forward 2 worktrees' })
  await user.hover(button)

  // the reason shows once the plan made again has landed, feat/billing-ui forced in it unchecked
  expect((await screen.findAllByText('Checking the worktrees again')).length).toBeGreaterThan(0)
  expect(button.getAttribute('aria-disabled')).toBe('true')
})

test('a worktree that gains changes is never named without force, even by the plan made again on the event', async () => {
  const events = liveEvents()
  const dirty = ['fix/login']
  answeringOn(dirty)
  const calls = recorded()
  const { dialog } = await fromSelection('main', 'feat/billing-ui')
  const button = await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' })
  await waitFor(() => expect(button.getAttribute('aria-disabled')).toBe('false'))
  const before = calls.dryRuns().length

  dirty.push('feat/billing-ui')
  act(() => events.changed())

  const ready = await within(dialog).findByRole('button', { name: 'Fast-forward' })
  await waitFor(() => expect(ready.getAttribute('aria-disabled')).toBe('false'))
  const names = (call: WebCall) => call.answers?.find((a) => a.path === 'worktree')?.values ?? []
  const forced = (call: WebCall) => call.answers?.some((a) => a.path === 'force') ?? false
  const unforced = calls
    .dryRuns()
    .slice(before)
    .filter((call) => names(call).includes('feat/billing-ui') && !forced(call))
  expect(unforced).toEqual([])
  expect(within(dialog).queryByText(/can't be fast-forwarded safely/)).toBeNull()
})

test('an apply refused because a worktree changed goes back to the worktrees, saying which changed', async () => {
  const dirty = ['fix/login']
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (call.dry_run) return ffPlan(call, dirty)
      dirty.push('feat/billing-ui')
      return ffUnsafe(['feat/billing-ui'])
    }),
  )
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' }))

  const alert = await within(dialog).findByRole('alert')
  expect(alert.textContent).toBe(
    'feat/billing-ui changed since the plan, so nothing was fast-forwarded. Check the worktrees again.',
  )
  await waitFor(() =>
    expect(
      within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing-ui' }).getAttribute('data-state'),
      'the fresh rows show it changed, unchecked',
    ).toBe('unchecked'),
  )
  expect(within(dialog).queryByText(/fast-forwarded\.$/)).toBeNull()

  await user.keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  await user.click(
    within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: 'Fast-forward' }),
  )
  const again = await screen.findByRole('dialog')
  await within(again).findByRole('listitem', { name: 'main' })
  expect(within(again).queryByText(/changed since the plan/), 'closed however, the refusal is read').toBeNull()
})

test('back after a break in the stream, no plan names a worktree that gained changes without force', async () => {
  const events = liveEvents()
  const dirty = ['fix/login']
  answeringOn(dirty)
  const calls = recorded()
  const { dialog } = await fromSelection('main', 'feat/billing-ui')
  const button = await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' })
  await waitFor(() => expect(button.getAttribute('aria-disabled')).toBe('false'))
  const before = calls.dryRuns().length

  dirty.push('feat/billing-ui')
  act(() => events.broken())

  await waitFor(
    () =>
      expect(
        within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing-ui' }).getAttribute('data-state'),
      ).toBe('unchecked'),
    { timeout: 5000 },
  )
  const ready = await within(dialog).findByRole('button', { name: 'Fast-forward' })
  await waitFor(() => expect(ready.getAttribute('aria-disabled')).toBe('false'))
  const names = (call: WebCall) => call.answers?.find((a) => a.path === 'worktree')?.values ?? []
  const forced = (call: WebCall) => call.answers?.some((a) => a.path === 'force') ?? false
  const unforced = calls
    .dryRuns()
    .slice(before)
    .filter((call) => names(call).includes('feat/billing-ui') && !forced(call))
  expect(unforced).toEqual([])
}, 10_000)
