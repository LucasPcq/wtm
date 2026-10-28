import { waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { answeringExec, execPlan, lockHeldBy } from '@/test/exec'
import { execFromTheSelection, holderEvents, typeAndConfirm } from '@/test/exec-dialog'
import { server } from '@/test/server'

// A holder the page never saw under way was said to hold the lock, as if the page knew it still did.
test('a worktree another command holds sends the run back to Confirm, which names the holder it may still be', async () => {
  server.use(...answeringExec(execPlan, lockHeldBy('op-9')))
  const app = await execFromTheSelection('feat/billing')
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run' }))

  const alert = await within(app.dialog).findByRole('alert')
  expect(within(alert).getByText('feat/billing is locked')).toBeTruthy()
  expect(
    within(alert).getByText("A clean started from the terminal may still hold it. Try again once it's done."),
  ).toBeTruthy()
  expect(within(app.dialog).getByRole('button', { name: 'Run' }).getAttribute('aria-disabled')).toBe('false')
  expect(within(app.dialog).getByRole('region', { name: 'Command' }).textContent).toContain('pnpm test')
})

test('the lock alert holds Run while its holder runs, and both go once the holder ends', async () => {
  const holder = holderEvents('op-9')
  server.use(holder.handler, ...answeringExec(execPlan, lockHeldBy('op-9')))
  const app = await execFromTheSelection('feat/billing')
  holder.started()
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run' }))

  expect(await within(app.dialog).findByText('feat/billing is locked')).toBeTruthy()
  expect(
    within(app.dialog).getByText("A clean started from the terminal holds it. Try again once it's done."),
  ).toBeTruthy()
  expect(within(app.dialog).getByRole('button', { name: 'Run' }).getAttribute('aria-disabled')).toBe('true')

  holder.finished()

  await waitFor(() => expect(within(app.dialog).queryByText('feat/billing is locked')).toBeNull())
  expect(within(app.dialog).getByRole('button', { name: 'Run' }).getAttribute('aria-disabled')).toBe('false')
})

// A reconnect starts the stream again from nothing: the holder it no longer shows was taken for one that let go.
test('a holder seen, then lost by a reconnect, may still hold the lock: the Alert says so and Run is enabled', async () => {
  const holder = holderEvents('op-9')
  server.use(holder.handler, ...answeringExec(execPlan, lockHeldBy('op-9')))
  const app = await execFromTheSelection('feat/billing')
  holder.started()
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run' }))
  expect(
    await within(app.dialog).findByText("A clean started from the terminal holds it. Try again once it's done."),
  ).toBeTruthy()

  holder.drop()

  expect(
    await within(app.dialog).findByText(
      "A clean started from the terminal may still hold it. Try again once it's done.",
    ),
  ).toBeTruthy()
  expect(within(app.dialog).getByRole('button', { name: 'Run' }).getAttribute('aria-disabled')).toBe('false')
})
