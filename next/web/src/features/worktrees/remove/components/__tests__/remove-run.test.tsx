import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { cleanPlan, heldRun } from '@/test/clean'
import { envelope } from '@/test/fixtures'
import { answeringClean, selectAndRemove } from '@/test/remove'
import { server } from '@/test/server'

const removeFromBar = async (user: { click: (element: Element) => Promise<void> }) => {
  await user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: /^Remove/ }))
  return screen.findByRole('dialog')
}

test('a run refused as stale says why, does not say each worktree went, and leaves the rows checked', async () => {
  server.use(
    answeringClean(async (call) => HttpResponse.json({ ...(await cleanPlan(call).json()), fingerprint: 'c0' })),
  )
  const { user, dialog } = await selectAndRemove('feat/billing-ui')

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))

  expect(await within(dialog).findByText("Couldn't remove feat/billing-ui")).toBeTruthy()
  expect(within(dialog).getByText('the facts changed since the preview')).toBeTruthy()
  expect(within(dialog).queryByText('Each went with its local branch and its record.')).toBeNull()
  await user.keyboard('{Escape}')
  expect(within(await screen.findByRole('toolbar', { name: 'Selection' })).getByText('1 selected')).toBeTruthy()
})

test('closed once it ended, a run leaves the next Remove to its own worktrees', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))
  await within(dialog).findByText('feat/billing-ui is removed')

  await user.keyboard('{Escape}')
  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))

  expect(await within(await removeFromBar(user)).findByRole('heading', { name: 'Remove fix/login' })).toBeTruthy()
})

test('one removal at a time: Remove shows the one under way, and once it ended, the rows checked meanwhile', async () => {
  const held = heldRun()
  server.use(answeringClean(cleanPlan, held.respond))
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))

  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))
  const busy = await removeFromBar(user)
  expect(await within(busy).findByText('Removing feat/billing-ui')).toBeTruthy()
  await user.click(within(busy).getByRole('button', { name: 'Hide' }))
  held.end({ ...envelope('clean', []), items: [{ subject: 'feat/billing-ui', status: 'done', detail: {} }] })

  expect(await within(await removeFromBar(user)).findByRole('heading', { name: 'Remove fix/login' })).toBeTruthy()
})

test('a run cancelled, then stopped, says it was cancelled', async () => {
  const held = heldRun()
  const cancels: string[] = []
  server.use(
    answeringClean(cleanPlan, held.respond),
    http.delete('/api/repos/r1/operations/op-1', ({ request }) => {
      cancels.push(new URL(request.url).search)
      return new HttpResponse(null, { status: 202 })
    }),
  )
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))

  await user.click(await within(dialog).findByRole('button', { name: 'Cancel' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Stop now' }))
  held.end({
    ...envelope('clean', []),
    ok: false,
    items: [{ subject: 'feat/billing-ui', status: 'cancelled', reason: 'interrupted', detail: {} }],
  })

  expect(await within(dialog).findByText('Cancelled the removal of feat/billing-ui')).toBeTruthy()
  expect(cancels).toEqual(['', '?force=true'])
})
