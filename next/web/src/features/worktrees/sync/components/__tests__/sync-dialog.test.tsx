import { screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { syncDryRun, syncRun } from '@/test/sync'
import { fromHeader, fromMenu, order, recorded } from '@/test/sync-app'

test("a row's Sync opens on Confirm: its plan in run order, main's fast-forward unnumbered, the one under it following", async () => {
  const { dialog } = await fromMenu('feat/billing')

  expect(await order(dialog)).toEqual([
    'mainfast-forwarded to origin first · 3 commits',
    '1feat/billingonto main · 4 commits',
    '2feat/billing-uionto feat/billing · 2 commits · follows feat/billing',
  ])
  const runs = within(dialog).getByRole('region', { name: 'How it runs' })
  expect(within(runs).getByText('feat/billing-ui follows feat/billing.')).toBeTruthy()
  expect(within(runs).getByText('Not pushed: origin keeps the old tips until you push.')).toBeTruthy()
  expect(within(dialog).queryByRole('switch')).toBeNull()
  expect(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' })).toBeTruthy()
})

test("Back from a row's Confirm reaches step 1: the stack as a tree, the one under it checked and held", async () => {
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  const follower = await within(dialog).findByRole('checkbox', { name: 'Sync feat/billing-ui' })
  expect(follower.getAttribute('data-state')).toBe('checked')
  expect(follower.hasAttribute('disabled')).toBe(true)
  expect(within(dialog).getByText('onto feat/billing · 2 commits · follows feat/billing')).toBeTruthy()
  expect(within(dialog).getByRole('checkbox', { name: 'Fast-forward main first' }).getAttribute('data-state')).toBe(
    'checked',
  )
  expect(within(dialog).getByRole('switch', { name: 'Sync the 1 worktree under feat/billing' })).toBeTruthy()
})

test('Descendants off leaves the one under it, named under Left as they are, and the run syncs the named alone', async () => {
  const calls = recorded()
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  await user.click(await within(dialog).findByRole('switch', { name: 'Sync the 1 worktree under feat/billing' }))
  expect(await within(dialog).findByText('stays on feat/billing as it is · needs a sync after')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText("stays on feat/billing's old tip · needs a sync after")).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Sync' }))

  expect(await within(dialog).findByText('1 synced.')).toBeTruthy()
  expect(calls.runs()[0]?.answers).toEqual([
    { path: 'worktree', values: ['feat/billing'] },
    { path: 'descendants', values: ['false'] },
    { path: 'no_fetch', values: ['true'] },
  ])
})

test('main unchecked rebases onto it as it stands, and Confirm says so', async () => {
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward main first' }))
  expect(await within(dialog).findByText('rebased onto as it stands · 3 commits behind origin')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await order(dialog)).toEqual([
    '1feat/billingonto main as it stands · 4 commits',
    '2feat/billing-uionto feat/billing · 2 commits · follows feat/billing',
  ])
  expect(within(dialog).getByText("main isn't fast-forwarded first: each rebases onto it as it stands.")).toBeTruthy()
})

test('the header opens on every stack, the command picking those that need a sync; one with a change unchecked', async () => {
  const calls = recorded()
  const { user, dialog } = await fromHeader()

  const box = (name: string) => within(dialog).findByRole('checkbox', { name: `Sync ${name}` })
  expect((await box('feat/billing')).getAttribute('data-state')).toBe('checked')
  expect((await box('fix/login')).getAttribute('data-state')).toBe('unchecked')
  expect(within(dialog).getByText('2 uncommitted changes: check it to sync it anyway')).toBeTruthy()
  expect(within(dialog).getByText('2 of 5 selected')).toBeTruthy()

  // the command's own pick leaves out a worktree with a change: nothing names it
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await order(dialog)).not.toContain('fix/login')
  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('2 synced.')).toBeTruthy()
  expect(calls.runs()[0]?.answers).toEqual([{ path: 'no_fetch', values: ['true'] }])
})

test('a worktree with a change checked anyway is named and forced, its changes stashed around the rebase', async () => {
  const calls = recorded()
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('checkbox', { name: 'Sync fix/login' }))
  expect(await within(dialog).findByText('2 uncommitted changes: stashed, then put back')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await order(dialog)).toContain('3fix/loginonto main · 2 commits · its changes stashed')
  await user.click(await within(dialog).findByRole('button', { name: 'Sync 3 worktrees' }))

  expect(await within(dialog).findByText('3 synced.')).toBeTruthy()
  expect(calls.runs()[0]?.answers).toEqual([
    { path: 'worktree', values: ['feat/billing', 'feat/billing-ui', 'fix/login'] },
    { path: 'force', values: ['true'] },
    { path: 'no_fetch', values: ['true'] },
  ])
})

test('a worktree named with a change opens on step 1, refused until checked', async () => {
  const { user, dialog } = await fromMenu('fix/login')

  const box = await within(dialog).findByRole('checkbox', { name: 'Sync fix/login' })
  expect(box.getAttribute('data-state')).toBe('unchecked')
  const next = within(dialog).getByRole('button', { name: 'Next' })
  expect(next.getAttribute('aria-disabled')).toBe('true')
  await user.hover(next)
  expect((await screen.findAllByText('Check fix/login to sync it anyway')).length).toBeGreaterThan(0)

  await user.click(box)
  await waitFor(() => expect(next.getAttribute('aria-disabled')).toBe('false'))
})

test('several branches rebased onto share one answer: each line says so, and one box moves them all', async () => {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of ['feat/billing-ui', 'fix/login'])
    await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  await app.user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: 'Sync' }))
  const dialog = await screen.findByRole('dialog')

  expect(await within(dialog).findByText('fast-forwarded to origin first · 1 commit, with main')).toBeTruthy()
  expect(within(dialog).getByText('fast-forwarded to origin first · 3 commits, with feat/billing')).toBeTruthy()
  await app.user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward main first' }))

  const billing = within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing first' })
  await waitFor(() => expect(billing.getAttribute('data-state')).toBe('unchecked'))
  expect(within(dialog).getByText('rebased onto as it stands · 1 commit behind origin, with main')).toBeTruthy()
})

test("the detail's callout opens the plan of its worktree on Confirm", async () => {
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling-ui')

  await user.click(await screen.findByRole('button', { name: 'Sync it onto feat/billing' }))

  const dialog = await screen.findByRole('dialog')
  expect(await order(dialog)).toEqual([
    'feat/billingfast-forwarded to origin first · 1 commit',
    '1feat/billing-uionto feat/billing · 2 commits',
  ])
})

test('turning Descendants off leaves a descendant checked anyway, never synced onto a parent left behind', async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call, { dirty: ['feat/billing-ui'] }) : syncRun(call)
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Sync feat/billing-ui' }))
  expect(await within(dialog).findByText('uncommitted changes: stashed, then put back')).toBeTruthy()

  await user.click(within(dialog).getByRole('switch', { name: 'Sync the 1 worktree under feat/billing' }))

  const box = within(dialog).getByRole('checkbox', { name: 'Sync feat/billing-ui' })
  await waitFor(() => expect(box.getAttribute('data-state')).toBe('unchecked'))
  expect(box.hasAttribute('disabled')).toBe(true)
})

test('a descendant held by its parent says why on hover', async () => {
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  const follower = await within(dialog).findByRole('checkbox', { name: 'Sync feat/billing-ui' })
  await user.hover(follower.parentElement as HTMLElement)

  expect(
    (
      await screen.findAllByText(
        'Comes with feat/billing; turn off "Sync the 1 worktree under feat/billing" to leave it',
      )
    ).length,
  ).toBeGreaterThan(0)
})

test('from the header, a worktree under one picked follows it, on Confirm as on step 1', async () => {
  const { user, dialog } = await fromHeader()

  await user.click(await within(dialog).findByRole('checkbox', { name: 'Sync feat/billing-ui' }))
  expect(await within(dialog).findByText('onto feat/billing · 2 commits · follows feat/billing')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await order(dialog)).toContain('2feat/billing-uionto feat/billing · 2 commits · follows feat/billing')
  const runs = within(dialog).getByRole('region', { name: 'How it runs' })
  expect(within(runs).getByText('feat/billing-ui follows feat/billing.')).toBeTruthy()
})
