import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { answeringExec, cancelledRun, execPlan, execRun, heldExecRun, passed, unreadableEnvRun } from '@/test/exec'
import { execFromTheSelection, typeAndConfirm } from '@/test/exec-dialog'
import { server } from '@/test/server'

test('while it runs, each row is queued or running, folded, its live output behind its chevron', async () => {
  const held = heldExecRun()
  server.use(...held.handlers)
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  const { user, dialog } = app
  await user.click(await within(dialog).findByRole('button', { name: 'Run in 2 worktrees' }))

  const queued = await within(dialog).findByRole('listitem', { name: 'feat/search' })
  expect(within(queued).getByText('queued')).toBeTruthy()
  const running = within(dialog).getByRole('listitem', { name: 'feat/billing' })
  expect(within(running).queryByText('✓ src/invoice.test.ts')).toBeNull()
  await user.click(within(running).getByRole('button', { name: 'Output of feat/billing' }))
  expect(await within(running).findByText('✓ src/invoice.test.ts')).toBeTruthy()
  expect(within(dialog).getByText('pnpm test · 0 of 2 done · 1 at once')).toBeTruthy()
})

test("a run that partly failed names the failed first, and Run again shows the command again on the ones that didn't pass", async () => {
  const runs: WebCall[] = []
  server.use(
    ...answeringExec(execPlan, (call) => {
      runs.push(call)
      return execRun(['feat/billing'])(call)
    }),
  )
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  const { user, dialog } = app
  await user.click(await within(dialog).findByRole('button', { name: 'Run in 2 worktrees' }))

  expect(
    await within(dialog).findByText("pnpm test failed in 1 of 2. Run again takes the 1 that didn't pass."),
  ).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: 'Failed' })
  expect(within(failed).getByText('exited 1 · 8.4 s')).toBeTruthy()
  await user.click(within(failed).getByRole('button', { name: /^Details of feat\/billing/ }))
  expect(within(failed).getByText(/expected 'Monthly' to be 'Yearly'/)).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Run again' }))

  const command = await within(dialog).findByRole('region', { name: 'Command' })
  expect(command.textContent).toContain('pnpm test')
  expect(
    within(dialog)
      .getAllByRole('listitem')
      .map((row) => row.getAttribute('aria-label'))
      .filter(Boolean),
  ).toEqual(['feat/billing'])
  await user.click(within(dialog).getByRole('button', { name: 'Run' }))
  await waitFor(() => expect(runs).toHaveLength(2))
  expect(runs[1]?.answers).toEqual([
    { path: 'worktree', values: ['feat/billing'] },
    { path: 'command', values: ['pnpm test'] },
  ])
})

test('a cancelled run says what is done stays, and names no failure of its own', async () => {
  server.use(...answeringExec(execPlan, cancelledRun))
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run in 2 worktrees' }))

  expect(
    await within(app.dialog).findByText("Cancelled: what is done stays. Run again takes the 2 that didn't pass."),
  ).toBeTruthy()
  const stopped = within(app.dialog).getByRole('region', { name: 'Stopped' })
  expect(within(stopped).getByText('cancelled while running')).toBeTruthy()
  expect(within(app.dialog).queryByText('interrupted')).toBeNull()
})

const runningHidden = async (held: ReturnType<typeof heldExecRun>) => {
  server.use(...held.handlers)
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run in 2 worktrees' }))
  await within(app.dialog).findByRole('listitem', { name: 'feat/search' })
  await app.user.click(within(app.dialog).getByRole('button', { name: 'Hide' }))
  return app
}

// The list said "Running a command · 0/1" of a worktree whose one phase is the command.
test('a worktree the command runs in says so in the list, with no count of its one step', async () => {
  await runningHidden(heldExecRun())

  expect(await screen.findAllByText('Running a command')).toHaveLength(2)
  expect(screen.queryByText(/Running a command ·/)).toBeNull()
})

test('Cancel stops what runs and says the queued one will not start; Stop now asks the server to force it', async () => {
  const held = heldExecRun()
  const cancels: string[] = []
  server.use(
    http.delete('/api/repos/r1/operations/op-1', ({ request }) => {
      cancels.push(new URL(request.url).search)
      return new HttpResponse(null, { status: 202 })
    }),
  )
  server.use(...held.handlers)
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  const { user, dialog } = app
  await user.click(await within(dialog).findByRole('button', { name: 'Run in 2 worktrees' }))

  await user.click(await within(dialog).findByRole('button', { name: 'Cancel' }))
  expect(
    await within(dialog).findByText("Stopping the commands under way; the queued one won't start. What is done stays."),
  ).toBeTruthy()
  expect(within(within(dialog).getByRole('listitem', { name: 'feat/search' })).getByText("won't start")).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Stop now' }))

  await waitFor(() => expect(cancels).toEqual(['', '?force=true']))
})

// "To check" said the screen's words of an .env it could not read, and the core's cause was nowhere.
test("a worktree that ran without its run env is a row to check, the core's cause behind its chevron", async () => {
  server.use(...answeringExec(execPlan, unreadableEnvRun))
  const app = await execFromTheSelection('feat/billing')
  await typeAndConfirm(app, 'pnpm test')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Run' }))

  const toCheck = await within(app.dialog).findByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText("Without its run env: its .env files couldn't be read")).toBeTruthy()
  await app.user.click(within(toCheck).getByRole('button', { name: /^Details of feat\/billing/ }))
  expect(within(toCheck).getByText(/permission denied/)).toBeTruthy()
})

test('hidden, a run that passed says so in a toast', async () => {
  const held = heldExecRun()
  await runningHidden(held)

  held.end(passed(['feat/billing', 'feat/search']))

  expect(await screen.findByText('pnpm test passed in all 2.')).toBeTruthy()
})

test('hidden, a run that partly failed says how many of those asked did not pass, and Details opens it again', async () => {
  const held = heldExecRun()
  const { user } = await runningHidden(held)

  held.end(passed(['feat/billing', 'feat/search'], ['feat/search']))

  expect(await screen.findByText("pnpm test didn't pass in 1 of 2.")).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Details' }))
  const dialog = await screen.findByRole('dialog', { name: 'Run a command' })
  expect(within(dialog).getByRole('region', { name: 'Failed' })).toBeTruthy()
})

// A run that lost its server said "didn't pass in 0 of 0".
test('hidden, a run that ended on no envelope says what stopped it', async () => {
  const held = heldExecRun()
  await runningHidden(held)

  held.drop()

  expect(await screen.findByText('the operation stopped answering before it ended')).toBeTruthy()
  expect(screen.queryByText(/0 of 0/)).toBeNull()
})
