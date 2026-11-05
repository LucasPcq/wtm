import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { answeringRunJobs, runningJobs } from '@/test/run-jobs'
import { job } from '@/test/run-up'
import { line, server } from '@/test/server'

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
  // frame 14: what acts on a shared service first, its restart saying for whom
  expect(
    within(screen.getByRole('menu'))
      .getAllByRole('menuitem')
      .map((item) => item.textContent)
      .slice(0, 2),
  ).toEqual([
    'Restart postgresfor the 3 worktrees holding it',
    'Release main’s holdpostgres keeps running for the 2 others',
  ])
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

test('a job restarting is greyed in its row and its header, saying it restarts', async () => {
  // the restart's stream held open: it has begun, nothing more
  const held = http.post('/api/repos/r1/commands/run-start', async ({ request }) => {
    if (((await request.clone().json()) as WebCall).dry_run) return undefined
    const body = new ReadableStream({
      start: (controller) =>
        controller.enqueue(new TextEncoder().encode(line({ type: 'unit.started', subject: 'feat/billing' }))),
    })
    return new HttpResponse(body, { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } })
  })
  server.use(held, ...answeringRunJobs())
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web', 'feat/billing')

  await user.click(await within(panel).findByRole('button', { name: 'Restart web' }))

  const group = screen.getByRole('region', { name: 'feat/billing' })
  await waitFor(() => expect(within(group).getByRole('listitem', { name: 'web' }).textContent).toContain('restarting…'))
  expect(within(panel).getByRole('heading').textContent).toContain('restarting…')
  expect(within(panel).getByRole('button', { name: 'Stop web' }).getAttribute('aria-disabled')).toBe('true')
})

test('main restarts a shared service it runs for the others alone through a worktree holding it', async () => {
  const claimed = runningJobs.map((w) =>
    w.branch === 'feat/billing'
      ? { ...w, jobs: [...(w.jobs ?? []), job('postgres', { hold: 'claim', profile: '' })] }
      : w,
  )
  server.use(
    http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', claimed))),
    http.post('/api/repos/r1/commands/run-stop', () =>
      HttpResponse.json({ ...envelope('run-stop', []), fingerprint: 's1', plan: { job: 'postgres', worktrees: [] } }),
    ),
    http.post('/api/repos/r1/commands/run-start/questions', async ({ request }) => {
      const asked = answers((await request.json()) as WebCall)
      const problems =
        asked.worktree?.[0] === 'main' && asked.restart
          ? [{ path: 'restart', code: 'run-start.restart_shared', flag: '--restart', message: 'not held' }]
          : []
      return HttpResponse.json({ fields: [{ kind: 'select', path: 'job', options: [] }], problems })
    }),
    ...answeringRunJobs(),
  )
  const { user } = await renderApp('/r/r1/running?worktree=main&job=postgres')
  const panel = await panelOf('postgres', 'main')

  await waitFor(() =>
    expect(within(panel).getByRole('button', { name: 'Stop postgres' }).getAttribute('aria-disabled')).toBe('true'),
  )
  await user.click(within(panel).getByRole('button', { name: 'Restart postgres' }))

  const dialog = await screen.findByRole('dialog', { name: 'Restart postgres' })
  expect(await within(dialog).findByText('feat/billing')).toBeDefined()
})
