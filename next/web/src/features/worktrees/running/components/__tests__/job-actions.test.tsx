import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { answeringRunJobs } from '@/test/run-jobs'
import { server } from '@/test/server'

const panelOf = (job: string, worktree: string) => screen.findByRole('region', { name: `${job} in ${worktree}` })

const answers = (call: WebCall | undefined) => Object.fromEntries((call?.answers ?? []).map((a) => [a.path, a.values]))

test('Restart starts the job again beside the others’ jobs and ends on a toast with no Undo', async () => {
  const calls: WebCall[] = []
  server.use(...answeringRunJobs({ calls }))
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web', 'feat/billing')

  await user.click(await within(panel).findByRole('button', { name: 'Restart web' }))

  expect(await screen.findByText('web restarted')).toBeDefined()
  expect(screen.getByText('feat/billing · up again on :3020')).toBeDefined()
  expect(screen.queryByRole('button', { name: 'Undo' })).toBeNull()
  expect(answers(calls[0])).toEqual({
    worktree: ['feat/billing'],
    job: ['web'],
    restart: ['true'],
    parallel: ['true'],
  })
})

test('Stop ends on a toast whose Undo starts the job again', async () => {
  const calls: WebCall[] = []
  server.use(...answeringRunJobs({ calls }))
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web', 'feat/billing')

  await user.click(await within(panel).findByRole('button', { name: 'Stop web' }))
  expect(await screen.findByText('web stopped')).toBeDefined()
  expect(screen.getByText('feat/billing · its log stays here')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Undo' }))

  expect(await screen.findByText('web started')).toBeDefined()
  expect(calls.map((call) => answers(call))).toEqual([
    { worktree: ['feat/billing'], job: ['web'] },
    { worktree: ['feat/billing'], job: ['web'], parallel: ['true'] },
  ])
})

test('a shared service main holds is released, the menu and the toast saying what keeps running', async () => {
  server.use(...answeringRunJobs())
  const { user } = await renderApp('/r/r1/running?worktree=main&job=postgres')
  const panel = await panelOf('postgres', 'main')

  await user.click(await within(panel).findByRole('button', { name: 'More for postgres' }))
  const release = await screen.findByRole('menuitem', { name: /Release main’s hold/ })
  expect(release.textContent).toContain('postgres keeps running for the 2 others')
  await user.click(release)

  expect(await screen.findByText('Released main’s hold on postgres')).toBeDefined()
  expect(screen.getByText('It keeps running for 2 worktrees.')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Undo' })).toBeDefined()
})

test('Restart of a shared service asks first, naming the worktrees it stops for', async () => {
  const calls: WebCall[] = []
  server.use(...answeringRunJobs({ calls }))
  const { user } = await renderApp('/r/r1/running?worktree=main&job=postgres')
  const panel = await panelOf('postgres', 'main')

  await user.click(await within(panel).findByRole('button', { name: 'Restart postgres' }))
  const dialog = await screen.findByRole('dialog', { name: 'Restart postgres' })
  expect(
    await within(dialog).findByText(
      'postgres stops for a moment in main, feat/billing and fix/login, then runs for them again, their namespaces kept.',
    ),
  ).toBeDefined()
  expect(calls).toHaveLength(0)
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Restart postgres' }).getAttribute('aria-disabled')).toBe(
      'false',
    ),
  )
  await user.click(within(dialog).getByRole('button', { name: 'Restart postgres' }))

  expect(await within(dialog).findByText('postgres restarted.')).toBeDefined()
  expect(answers(calls[0])).toMatchObject({ restart: ['true'], parallel: ['true'] })
})

test('a stop that fails ends on an error toast whose Details shows its cause', async () => {
  server.use(...answeringRunJobs({ failing: ['web'] }))
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web', 'feat/billing')

  await user.click(await within(panel).findByRole('button', { name: 'Stop web' }))
  expect(await screen.findByText('web didn’t stop')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Details' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop web' })
  expect(within(dialog).getByText('The stop command of web failed (exit 4)')).toBeDefined()
})

test('Start one job… offers the jobs not running in the worktree, then starts the one picked', async () => {
  server.use(...answeringRunJobs())
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const group = await screen.findByRole('region', { name: 'feat/billing' })

  await user.click(within(group).getByRole('button', { name: 'More for feat/billing' }))
  const entry = await screen.findByRole('menuitem', { name: /Start one job…/ })
  await waitFor(() => expect(entry.textContent).toContain('choose among the 3 jobs not running'))
  await user.click(entry)
  const dialog = await screen.findByRole('dialog', { name: 'Start one job' })
  expect(within(dialog).queryByRole('radio', { name: 'web' })).toBeNull()
  await user.click(within(dialog).getByRole('radio', { name: 'worker' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Start worker' }).getAttribute('aria-disabled')).toBe('false'),
  )
  await user.click(within(dialog).getByRole('button', { name: 'Start worker' }))

  expect(await within(dialog).findByText('worker started.')).toBeDefined()
})
