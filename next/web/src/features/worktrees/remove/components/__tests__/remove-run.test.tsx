import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { cleanPlan, failedRun, heldRun, partlyFailedRun } from '@/test/clean'
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
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))

  expect(await within(dialog).findByText('0 of 1 removed.')).toBeTruthy()
  expect(within(dialog).getByText('the facts changed since the preview')).toBeTruthy()
  expect(within(dialog).queryByText('Each went with its local branch and its record.')).toBeNull()
  await user.keyboard('{Escape}')
  expect(within(await screen.findByRole('toolbar', { name: 'Selection' })).getByText('1 selected')).toBeTruthy()
})

test('closed once it ended, a run leaves the next Remove to its own worktrees', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))
  await within(dialog).findByText(/^1 removed\./)

  await user.keyboard('{Escape}')
  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))

  expect(await within(await removeFromBar(user)).findByRole('listitem', { name: 'fix/login' })).toBeTruthy()
})

test('one removal at a time: Remove shows the one under way, and once it ended, the rows checked meanwhile', async () => {
  const held = heldRun()
  server.use(answeringClean(cleanPlan, held.respond))
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))

  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))
  const busy = await removeFromBar(user)
  expect(await within(busy).findByRole('progressbar', { name: 'Progress' })).toBeTruthy()
  await user.click(within(busy).getByRole('button', { name: 'Hide' }))
  held.end({ ...envelope('clean', []), items: [{ subject: 'feat/billing-ui', status: 'done', detail: {} }] })

  expect(await within(await removeFromBar(user)).findByRole('listitem', { name: 'fix/login' })).toBeTruthy()
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
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))

  await user.click(await within(dialog).findByRole('button', { name: 'Cancel' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Stop now' }))
  held.end({
    ...envelope('clean', []),
    ok: false,
    items: [{ subject: 'feat/billing-ui', status: 'cancelled', reason: 'interrupted', detail: {} }],
  })

  expect(await within(dialog).findByText('0 of 1 removed. Those not started stay.')).toBeTruthy()
  expect(cancels).toEqual(['', '?force=true'])
})

test('a hidden run that partly failed is shown on the next Remove, and only what went leaves the selection', async () => {
  const held = heldRun()
  server.use(answeringClean(cleanPlan, held.respond))
  const { user, dialog } = await selectAndRemove('feat/billing', 'feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove 2 worktrees' }))
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Cancel' }).hasAttribute('disabled')).toBe(false),
  )
  await user.click(within(dialog).getByRole('button', { name: 'Hide' }))
  held.end({
    ...envelope('clean', []),
    ok: false,
    items: [
      { subject: 'feat/billing', status: 'done', detail: {} },
      {
        subject: 'feat/billing-ui',
        status: 'failed',
        error: { kind: 'failed', code: 'clean.remove_failed', message: "couldn't remove the worktree" },
        detail: {},
      },
    ],
  })

  expect(await within(screen.getByRole('toolbar', { name: 'Selection' })).findByText('1 selected')).toBeTruthy()
  await user.click(await screen.findByRole('button', { name: 'Details' }))
  const failed = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(failed).findByText('1 of 2 removed.')).toBeTruthy()
  await user.click(within(failed).getByRole('button', { name: 'Done' }))

  expect(await within(await removeFromBar(user)).findByRole('listitem', { name: 'feat/billing-ui' })).toBeTruthy()
})

test('a removal that failed is one line, its output and log one click under it, its steps folded last', async () => {
  server.use(answeringClean(cleanPlan, failedRun))
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))

  expect(await within(dialog).findByText('0 of 1 removed.')).toBeTruthy()
  expect(within(dialog).getByText('on_clean hook failed: exit status 1')).toBeTruthy()
  expect(within(dialog).queryByText('Error response from daemon: permission denied')).toBeNull()

  await user.click(
    within(dialog).getByRole('button', { name: 'Details of feat/billing-ui: on_clean hook failed: exit status 1' }),
  )
  expect(within(dialog).getByText('Error response from daemon: permission denied')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: "Copy the log's path" })).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: /^2 steps/ })).toBeTruthy()
})

test('a finished removal says a warning of the whole run first, as its Alert, then the groups that need the person first', async () => {
  server.use(answeringClean(cleanPlan, partlyFailedRun))
  const { user, dialog } = await selectAndRemove('feat/billing-ui', 'feat/search')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText(/is stacked on/)
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove 2 worktrees' }))

  await within(dialog).findByText('1 of 2 removed.')
  const alert = within(dialog).getByRole('alert')
  expect(alert.textContent).toContain("Couldn't ask the run daemon which jobs are up")
  expect(
    within(dialog)
      .getAllByRole('heading', { level: 3 })
      .map((heading) => heading.textContent),
  ).toEqual(["Couldn't remove", 'Removed'])
  expect(
    alert.compareDocumentPosition(within(dialog).getByRole('heading', { name: "Couldn't remove" })) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
})
