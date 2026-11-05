import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { answeringRunDown, heldRunDown, postgresEnded, runDownRun } from '@/test/run-down'
import { confirm, confirmBoth, rows, startHidden } from '@/test/run-down-app'
import { server } from '@/test/server'

test('a stop under way has no Cancel, and the stack unfolds on the lines of its stop command, live', async () => {
  const held = heldRunDown()
  server.use(...answeringRunDown(held.run))
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  expect(await within(dialog).findByText('Stopping jobs in 2 worktrees · 0 of 6 stopped')).toBeDefined()
  expect(within(dialog).queryByRole('button', { name: /Cancel/ })).toBeNull()
  expect(within(dialog).getByText('Closing this keeps it running.')).toBeDefined()
  const stack = within(within(dialog).getByRole('region', { name: 'fix/login' })).getByRole('listitem', {
    name: 'stack',
  })
  expect(within(stack).getByText('running its stop command')).toBeDefined()
  await user.click(within(stack).getByRole('button', { name: 'Output of stack' }))
  expect(within(stack).getByText('Container minio-1 Stopping')).toBeDefined()
  expect(rows(within(dialog).getByRole('region', { name: 'Shared services' }))).toEqual([
    'postgresreleased once both worktrees stop',
  ])

  await held.finish()
  expect(await within(dialog).findByText('Jobs stopped in 2 worktrees.')).toBeDefined()
})

test('hidden, the stop goes on in a toast that counts it and shows it again, then says it ended', async () => {
  const held = heldRunDown()
  server.use(...answeringRunDown(held.run))
  const { user } = await renderApp('/r/r1/running')

  await startHidden(user)

  expect(await screen.findByText('Stopping jobs in 2 worktrees')).toBeDefined()
  expect(screen.getByText('0 of 6 stopped')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Show' }))
  const shown = await screen.findByRole('dialog', { name: 'Stop jobs' })
  expect(within(shown).getByText(/Stopping jobs in 2 worktrees/)).toBeDefined()
  await user.click(within(shown).getByRole('button', { name: 'Hide' }))
  await held.finish()
  expect(await screen.findByText('Jobs stopped in 2 worktrees')).toBeDefined()
})

test("hidden, a stop that failed says it in a toast whose Details shows what didn't stop", async () => {
  const held = heldRunDown(['fix/login'])
  server.use(...answeringRunDown(held.run))
  const { user } = await renderApp('/r/r1/running')
  await startHidden(user)

  await held.finish()

  expect(await screen.findByText("Jobs didn't stop in 1 of 2.")).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Details' }))
  const shown = await screen.findByRole('dialog', { name: 'Stop jobs' })
  expect(within(shown).getByRole('region', { name: "Couldn't stop" })).toBeDefined()
})

test('the outcome says what was released and for whom, then what stopped in each worktree', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  expect(await within(dialog).findByText('Jobs stopped in 2 worktrees.')).toBeDefined()
  const released = within(dialog).getByRole('region', { name: 'Released' })
  expect(within(released).getByText('postgres')).toBeDefined()
  expect(within(released).getByText('keeps running for feat/search')).toBeDefined()
  const stopped = within(dialog).getByRole('region', { name: 'Stopped' })
  expect(within(stopped).getByText('web, api · worker cleared')).toBeDefined()
  expect(within(stopped).getByText('web, stack')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Done' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

// The plan saw the stop as postgres's last holds; another worktree claimed it before the apply, which released it:
// the outcome said "keeps running for " and no one.
test('a release the plan saw nobody keeping says another worktree keeps it', async () => {
  server.use(postgresEnded('released'), ...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirm(user, ['feat/billing', 'fix/login', 'feat/search'], 'Stop jobs in 3 worktrees')
  expect(rows(within(dialog).getByRole('region', { name: 'Shared services' }))).toEqual([
    'postgresthe 3 let go: it stops, no worktree keeps it',
  ])

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 3 worktrees' }))

  const released = await within(dialog).findByRole('region', { name: 'Released' })
  expect(within(released).getByText('keeps running for another worktree')).toBeDefined()
})

// Its holders found postgres stopped already: the outcome said nothing of it.
test('a shared service its holders found stopped already says so, muted, among what stopped', async () => {
  server.use(postgresEnded('not_running'), ...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  const stopped = await within(dialog).findByRole('region', { name: 'Stopped' })
  expect(within(stopped).getByText('already stopped')).toBeDefined()
  expect(within(dialog).queryByRole('region', { name: 'Released' })).toBeNull()
})

test("a stack that doesn't stop fails its worktree alone, said first", async () => {
  server.use(...answeringRunDown(runDownRun(['fix/login'])))
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  expect(await within(dialog).findByText('Jobs stopped in 1 of 2 worktrees.')).toBeDefined()
  const failed = within(dialog).getByRole('region', { name: "Couldn't stop" })
  expect(within(failed).getByText("stack didn't stop: exit 1")).toBeDefined()
  await user.click(within(failed).getByRole('button', { name: /Details of fix\/login/ }))
  expect(within(failed).getByText(/Container mailpit-1 +Stopped/), "the stack's own lines say why").toBeDefined()
  expect(
    within(within(dialog).getByRole('region', { name: 'Stopped' })).getByText('web, api · worker cleared'),
  ).toBeDefined()
})
