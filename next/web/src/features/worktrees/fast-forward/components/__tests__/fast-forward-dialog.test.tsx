import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { heldRun } from '@/test/clean'
import { advanceOf, ffPlan, ffRun } from '@/test/fast-forward'
import { answeringOn, fromHeader, fromSelection, recorded } from '@/test/fast-forward-app'
import { envelope } from '@/test/fixtures'
import { server } from '@/test/server'

test('the header opens on the worktrees behind origin, the one with a change left unchecked', async () => {
  const { dialog } = await fromHeader()

  const box = (name: string) => within(dialog).findByRole('checkbox', { name: `Fast-forward ${name}` })
  expect((await box('main')).getAttribute('data-state')).toBe('checked')
  expect((await box('feat/billing-ui')).getAttribute('data-state')).toBe('checked')
  expect((await box('fix/login')).getAttribute('data-state')).toBe('unchecked')
  expect(within(dialog).getByText('1 uncommitted change')).toBeTruthy()
  expect(within(dialog).getByText('diverged: 1 ahead, 4 behind')).toBeTruthy()
  expect(within(dialog).getByText('not on origin')).toBeTruthy()
  expect(within(dialog).getByText('2 of 6 selected')).toBeTruthy()
})

test('the precheck sent untouched lets the command pick, and keeps the worktree with a change as it is', async () => {
  const calls = recorded()
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('uncommitted changes: kept as it is')).toBeTruthy()
  expect(within(dialog).getByRole('listitem', { name: 'main' })).toBeTruthy()
  expect(within(dialog).queryByRole('checkbox', { name: 'Fast-forward main' })).toBeNull()

  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' }))

  expect(await within(dialog).findByText('2 fast-forwarded.')).toBeTruthy()
  expect(calls.runs()[0]?.answers).toEqual([{ path: 'no_fetch', values: ['true'] }])
  const toCheck = within(dialog).getByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText('uncommitted changes: kept as it is')).toBeTruthy()
  expect(within(toCheck).getByRole('button', { name: /Copy wtm fast-forward fix\/login --force true/ })).toBeTruthy()
})

test('a precheck touched names the worktrees checked: one with a change is forced only once checked anyway', async () => {
  const calls = recorded()
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const row = await within(dialog).findByRole('listitem', { name: 'fix/login' })
  expect(within(row).getByText('anyway')).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 3 worktrees' }))

  expect(await within(dialog).findByText('3 fast-forwarded.')).toBeTruthy()
  expect(calls.runs()[0]?.answers).toEqual([
    { path: 'worktree', values: ['main', 'feat/billing-ui', 'fix/login'] },
    { path: 'force', values: ['true'] },
    { path: 'no_fetch', values: ['true'] },
  ])
})

test('from the selection the dialog opens on Confirm, an unsafe row unchecked, the others left with why', async () => {
  const calls = recorded()
  const { user, dialog } = await fromSelection('main', 'fix/login', 'feat/search', 'feat/search-docs')

  const unsafe = await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' })
  expect(unsafe.getAttribute('data-state')).toBe('unchecked')
  const left = within(dialog).getByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('diverged from origin: 1 ahead, 4 behind')).toBeTruthy()
  expect(within(left).getByText('not on origin')).toBeTruthy()

  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward' }))

  expect(await within(dialog).findByText('1 fast-forwarded.')).toBeTruthy()
  // the rows left go with it, so the outcome says why each stayed; the unchecked unsafe one is never named
  expect(calls.runs()[0]?.answers).toEqual([
    { path: 'worktree', values: ['main', 'feat/search', 'feat/search-docs'] },
    { path: 'no_fetch', values: ['true'] },
  ])
  const toCheck = within(dialog).getByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText('diverged from origin: 1 ahead, 4 behind')).toBeTruthy()
})

test('with only an unsafe row, Fast-forward says what would enable it', async () => {
  const { user, dialog } = await fromSelection('fix/login')

  const button = await within(dialog).findByRole('button', { name: 'Fast-forward' })
  await waitFor(() => expect(button.getAttribute('aria-disabled')).toBe('true'))
  await user.hover(button)
  expect((await screen.findAllByText('Check fix/login to fast-forward it anyway')).length).toBeGreaterThan(0)
})

test('a plan with nothing to take ends on its empty state, with no confirmation', async () => {
  const calls = recorded()
  const { user, dialog } = await fromSelection('feat/billing', 'feat/search')

  expect(await within(dialog).findByText('Nothing to fast-forward')).toBeTruthy()
  expect(within(dialog).getByText('1 ahead of origin')).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: /^Fast-forward/ })).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Done' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(calls.runs()).toEqual([])
})

test('a fetch that failed is the plan’s alert, the plan going on as last fetched', async () => {
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) return ffRun(call)
      const planned = await ffPlan(call).json()
      const warning = { code: 'fast-forward.fetch_failed', message: "couldn't fetch origin: could not resolve host" }
      return HttpResponse.json({ ...planned, warnings: [warning, ...planned.warnings] })
    }),
  )
  const { dialog } = await fromSelection('main')

  expect((await within(dialog).findByRole('alert')).textContent).toContain("Couldn't fetch origin")
  expect(within(dialog).getByRole('listitem', { name: 'main' })).toBeTruthy()
})

test('a worktree git refused fails alone, its output in its fold', async () => {
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? ffPlan(call) : ffRun(call, ['feat/billing-ui'])
    }),
  )
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui')

  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' }))

  expect(await within(dialog).findByText('1 of 2 fast-forwarded.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't fast-forward" })
  expect(within(failed).getByText('git refused to move the branch')).toBeTruthy()
  await user.click(within(failed).getByRole('button', { name: /Details of feat\/billing-ui/ }))
  expect(within(failed).getByText(/would be overwritten by merge/)).toBeTruthy()
  expect(within(within(dialog).getByRole('region', { name: 'Fast-forwarded' })).getByText('3 commits')).toBeTruthy()
})

test('a cancelled run says so once, with no failure line of its own', async () => {
  const held = heldRun('fast_forward')
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? ffPlan(call) : held.respond(call)
    }),
  )
  const { user, dialog } = await fromSelection('main', 'feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward 2 worktrees' }))
  await within(dialog).findByText('0 of 2')

  held.end({
    ...envelope('fast-forward', []),
    ok: false,
    items: [
      { subject: 'main', status: 'done', detail: { path: '/acme', from: 'a', to: 'b' } },
      { subject: 'feat/billing-ui', status: 'skipped', reason: 'interrupted', detail: {} },
    ],
    plan: { worktrees: [advanceOf('main', false), advanceOf('feat/billing-ui', false)], force: false },
    error: { kind: 'cancelled', code: 'cancelled', message: 'interrupted' },
  })

  expect(await within(dialog).findByText('Cancelled: what is done stays.')).toBeTruthy()
  expect(within(dialog).getByText('not reached: cancelled first')).toBeTruthy()
  expect(within(dialog).queryByText('interrupted')).toBeNull()
})

test('a plan whose only worktrees behind origin have changes says what origin has waits on them', async () => {
  answeringOn(['main', 'feat/billing-ui', 'fix/login'])
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('Nothing to fast-forward')).toBeTruthy()
  expect(within(dialog).getByText('What origin has waits on uncommitted changes, kept as they are.')).toBeTruthy()
})

test('Back from Confirm keeps the worktrees checked', async () => {
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  const box = await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' })
  expect(box.getAttribute('data-state')).toBe('checked')
})
