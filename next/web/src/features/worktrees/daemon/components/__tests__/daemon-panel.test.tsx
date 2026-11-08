import { screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { answeringRestart, answeringStopJobs, daemonDetail, daemonStatus } from '@/test/daemon'
import { renderApp } from '@/test/render'
import { liveEvents, server } from '@/test/server'

const openPanel = async (user: Awaited<ReturnType<typeof renderApp>>['user']) => {
  await user.click(await screen.findByRole('button', { name: "wtm's daemon" }))
  return screen.findByRole('dialog')
}

test("the connection block opens wtm's daemon: up, the jobs of every repository, its proxy", async () => {
  server.use(daemonStatus())
  const { user } = await renderApp('/r/r1/worktrees')

  const panel = await openPanel(user)

  expect(await within(panel).findByText('Up · 0.31.0, this version')).toBeDefined()
  expect(within(panel).getByText('2 jobs and a stack: postgres, web, mailpit')).toBeDefined()
  expect(within(panel).getByText('1 job: docs-site')).toBeDefined()
  expect(within(panel).getByText('on :11080')).toBeDefined()
})

test('a daemon of another build turns the block amber and says a start is refused until it restarts', async () => {
  server.use(liveEvents, daemonStatus({ ...daemonDetail, daemon: '0.30.9', skew: 'build' }))
  const { user } = await renderApp('/r/r1/worktrees')

  expect(await screen.findByText("wtm's daemon runs another build")).toBeDefined()
  const panel = await openPanel(user)

  expect(await within(panel).findByText('Runs 0.30.9; this version is 0.31.0')).toBeDefined()
  expect(within(panel).getByText('Starting a job is refused until it restarts.')).toBeDefined()
})

test('Restart lists what comes back by repository, whose hold goes, what is taken over, then says it in a toast', async () => {
  server.use(daemonStatus(), answeringRestart())
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))

  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  expect(await within(dialog).findByText('postgres · shared')).toBeDefined()
  expect(within(dialog).getByText('feat/billing lose them for the cut')).toBeDefined()
  expect(within(dialog).getByText("fix/login's stack")).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  expect(await screen.findByText("Restarted wtm's daemon · 2 jobs are back; the stack was taken over.")).toBeDefined()
})

test("a restart whose new daemon didn't start says it in the panel, with its log to open", async () => {
  server.use(daemonStatus(), answeringRestart(true))
  const { user } = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  panel = await openPanel(user)

  expect(await within(panel).findByText("The new daemon didn't start")).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Open its log' })).toBeDefined()
  expect(within(panel).getByText('not served')).toBeDefined()
})

test('Stop every job lists every repository, then stops them and says the proxy stays', async () => {
  server.use(daemonStatus(), answeringStopJobs())
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  expect(await within(dialog).findByText('and its stack: mailpit, through its stop command')).toBeDefined()
  expect(within(dialog).getByText('feat/billing lose them')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Stop 3 jobs' }))

  expect(await within(dialog).findByRole('button', { name: 'Done' })).toBeDefined()
})

test("a stack that didn't stop is named, and Run again plans what still runs", async () => {
  server.use(daemonStatus(), answeringStopJobs(true))
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  await user.click(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' }))

  expect(await within(dialog).findByText(/couldn't stop mailpit/)).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Run again' }))

  expect(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' })).toBeDefined()
})
