import { screen, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { syncDryRun } from '@/test/sync'
import { fromMenu, order } from '@/test/sync-app'
import { syncRun } from '@/test/sync-run'

test('done, the run counts the rebases, lists main apart, and offers to push them in a plan of their own', async () => {
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('2 synced.')).toBeTruthy()
  const synced = within(dialog).getByRole('region', { name: 'Synced' })
  expect(within(synced).getByText('onto main · 4 commits')).toBeTruthy()
  const forwarded = within(dialog).getByRole('region', { name: 'Fast-forwarded' })
  expect(within(forwarded).getByText('3 commits from origin')).toBeTruthy()

  // synced, they stand on their parents: their plan only pushes
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) =>
      syncDryRun((await request.json()) as WebCall, { synced: ['feat/billing', 'feat/billing-ui'] }),
    ),
  )
  await user.click(within(dialog).getByRole('button', { name: /^Push them/ }))

  expect(await within(dialog).findByRole('button', { name: 'Push 2 worktrees' })).toBeTruthy()
  expect(within(dialog).getByText('Each worktree is pushed to origin, with force-with-lease.')).toBeTruthy()
  expect(await order(dialog)).toEqual(['feat/billingpushed to origin', 'feat/billing-uipushed to origin'])
  const runs = within(dialog).getByRole('region', { name: 'How it runs' })
  expect(within(runs).getByText('Pushed with force-with-lease: refused if origin moved since.')).toBeTruthy()
})

test('a conflict mid-stack fails its row, blocks the one under it, and runs again keeping the conflict', async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call) : syncRun(call, { conflict: 'feat/billing' })
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('0 of 2 synced.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't sync" })
  expect(within(failed).getByText('conflicts with main; nothing changed')).toBeTruthy()
  expect(within(failed).getByText("blocked: feat/billing didn't sync")).toBeTruthy()

  await user.click(within(failed).getByRole('button', { name: /^Run again, keep the conflict/ }))

  const runs = await within(dialog).findByRole('region', { name: 'How it runs' })
  expect(
    within(runs).getByText('A conflict stays in progress for you to finish in its worktree; the ones under it wait.'),
  ).toBeTruthy()
})

test("reparent's next step opens Sync on the worktree moved, never a command to copy", async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions on feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Reparent' }))
  const reparent = await screen.findByRole('dialog')
  await user.click(await within(reparent).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'fix/login' }))
  await user.click(within(reparent).getByRole('button', { name: 'Next' }))
  await user.click(await within(reparent).findByRole('button', { name: 'Reparent' }))

  await user.click(await within(reparent).findByRole('button', { name: 'Sync it' }))

  const sync = await screen.findByRole('dialog')
  expect(within(sync).getByRole('heading', { name: 'Sync worktrees' })).toBeTruthy()
})

test("main's fast-forward refused is one to check, never a failed run: the worktrees on it rebased as it stood", async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call) : syncRun(call, { ffFailed: true })
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('2 synced.')).toBeTruthy()
  const toCheck = within(dialog).getByRole('region', { name: 'To check' })
  expect(
    within(toCheck).getByText("couldn't take origin's commits: feat/billing rebased onto it as it stands"),
  ).toBeTruthy()
  expect(within(dialog).queryByRole('region', { name: "Couldn't sync" })).toBeNull()
  expect(within(dialog).queryByRole('region', { name: 'Fast-forwarded' })).toBeNull()
})

test('a push origin refuses fails its row, yet its rebase stands and counts as synced', async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call) : syncRun(call, { pushRefused: 'feat/billing' })
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))
  await user.click(await within(dialog).findByRole('switch', { name: 'Push to origin' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('1 of 2 synced.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't push" })
  expect(within(failed).getByText('rebased, but origin refused the push')).toBeTruthy()
  expect(within(failed).getByText("blocked: feat/billing's push was refused")).toBeTruthy()
})

test('"Sync anyway" opens its plan on step 1: the worktree with a change is checked again, by the person', async () => {
  const world = { dirty: ['feat/billing-ui'] }
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call, world) : syncRun(call, { world })
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Sync' }))

  await user.click(await within(dialog).findByRole('button', { name: /^Sync anyway/ }))

  const box = await within(dialog).findByRole('checkbox', { name: 'Sync feat/billing-ui' })
  expect(box.getAttribute('data-state')).toBe('unchecked')
  expect(within(dialog).getByText('uncommitted changes: check it to sync it anyway')).toBeTruthy()
})

test('"Run again, keep the conflict" after a forced run asks the consent again on step 1, never a skeleton', async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call) : syncRun(call, { conflict: 'fix/login' })
    }),
  )
  const { user, dialog } = await fromMenu('fix/login')
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Sync fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Sync' }))

  await user.click(await within(dialog).findByRole('button', { name: /^Run again, keep the conflict/ }))

  const box = await within(dialog).findByRole('checkbox', { name: 'Sync fix/login' })
  expect(box.getAttribute('data-state')).toBe('unchecked')
  expect(within(dialog).getByRole('switch', { name: 'Leave a conflict to resolve' }).getAttribute('data-state')).toBe(
    'checked',
  )
})

test("Descendants off ends with the one left out to sync in its own plan, the core's follow-up", async () => {
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))
  await user.click(await within(dialog).findByRole('switch', { name: 'Sync the 1 worktree under feat/billing' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Sync' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('needs a sync')).toBeTruthy()
  await user.click(within(left).getByRole('button', { name: 'Sync it' }))

  expect(await order(dialog)).toEqual([
    'feat/billingfast-forwarded to origin first · 1 commit',
    '1feat/billing-uionto feat/billing · 2 commits',
  ])
})

test('a warning of a worktree shows once on Done, in To check, beside the worktrees Descendants left', async () => {
  const world = { diverged: ['feat/search'] }
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call, world) : syncRun(call, { world })
    }),
  )
  const app = await renderApp('/r/r1/worktrees')
  for (const name of ['feat/billing', 'feat/search'])
    await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  await app.user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: 'Sync' }))
  const dialog = await screen.findByRole('dialog')
  await app.user.click(await within(dialog).findByRole('switch', { name: 'Sync the worktrees under the ones picked' }))
  await app.user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await app.user.click(await within(dialog).findByRole('button', { name: 'Sync' }))

  await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(dialog).getAllByText('Origin has commits on feat/search it never held: left as it is')).toHaveLength(1)
})
