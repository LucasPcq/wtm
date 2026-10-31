import { screen, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { syncDryRun, syncRun } from '@/test/sync'
import { fromMenu, order } from '@/test/sync-app'

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
  expect(await order(dialog)).toEqual(['1feat/billingpushed to origin', '2feat/billing-uipushed to origin'])
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

test("main's fast-forward refused fails its own row, the worktrees on it rebased as it stood", async () => {
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? syncDryRun(call) : syncRun(call, { ffFailed: true })
    }),
  )
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('button', { name: 'Sync 2 worktrees' }))

  expect(await within(dialog).findByText('2 of 2 synced.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't sync" })
  expect(
    within(failed).getByText("couldn't take origin's commits: feat/billing rebased onto it as it stands"),
  ).toBeTruthy()
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
  const failed = within(dialog).getByRole('region', { name: "Couldn't sync" })
  expect(within(failed).getByText('git refused the push; its rebase stands')).toBeTruthy()
})
