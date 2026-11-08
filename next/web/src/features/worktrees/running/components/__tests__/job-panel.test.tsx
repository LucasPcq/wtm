import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { answeringRunLogs, listing, onLine, panelOf, shown } from '@/test/run-logs'
import { job, runningWorktrees } from '@/test/run-up'
import { server } from '@/test/server'

test('a job picked shows where it runs, its address, its folder and command, and its lines', async () => {
  listing()
  await renderApp('/r/r1/running?worktree=feat/billing&job=web')

  const panel = await panelOf('web')
  expect(within(panel).getByRole('navigation', { name: 'Where it runs' }).textContent).toBe('feat/billingprofile dev')
  expect(within(panel).getByText('web.feat-billing.acme.localhost')).toBeDefined()
  expect((await within(panel).findByRole('button', { name: 'Copy pnpm dev' })).textContent).toBe('apps/web · pnpm dev')
  await waitFor(() => expect(shown(panel)).toContain('GET /invoices/inv_0042 200 in 702ms'))
  expect(within(panel).getByText('4 lines')).toBeDefined()
})

test('the first job of the list is shown until another is picked, the one shown marked in the list', async () => {
  listing()
  const { user, router } = await renderApp('/r/r1/running')

  expect(await panelOf('api')).toBeDefined()
  const group = screen.getByRole('region', { name: 'feat/billing' })
  await user.click(within(group).getByRole('link', { name: /^web up/ }))

  expect(await panelOf('web')).toBeDefined()
  expect(router.state.location.search).toEqual({ worktree: 'feat/billing', job: 'web' })
  expect(
    within(group)
      .getByRole('link', { name: /^web up/ })
      .getAttribute('aria-current'),
  ).toBe('page')
})

test('a crashed job says so above its last lines, which end its log', async () => {
  listing()
  await renderApp('/r/r1/running?worktree=feat/billing&job=api')

  const panel = await panelOf('api')
  expect(within(panel).getByText('api crashed with exit code 1')).toBeDefined()
  await waitFor(() => expect(shown(panel)).toContain("Error: Cannot find module './jobs/send-invoice'"))
  expect(within(panel).getByText(/^ended .* · 3 lines$/)).toBeDefined()
})

test('a search counts its matches and steps through them; only matching says how many lines it hides', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  await waitFor(() => expect(shown(panel)).toContain('GET /health'))

  await user.type(within(panel).getByRole('textbox', { name: 'Search this log' }), 'invoice')
  expect(within(panel).getByText('2 of 2')).toBeDefined()
  expect(onLine(panel)).toContain('inv_0042')
  await user.click(within(panel).getByRole('button', { name: 'Previous match' }))
  expect(within(panel).getByText('1 of 2')).toBeDefined()

  await user.click(within(panel).getByRole('tab', { name: 'Only matching' }))
  expect(shown(panel)).toBe('··· 1 line hiddenGET /invoices 200··· 1 line hiddenGET /invoices/inv_0042 200 in 702ms')
  expect(within(panel).getByText('2 of 4 lines')).toBeDefined()
  expect(within(panel).getByText(/Only the lines matching invoice: a text match/)).toBeDefined()
})

test('the latest error is a search of text, said under the toolbar, also on its key', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=api')
  const panel = await panelOf('api')
  await waitFor(() => expect(shown(panel)).toContain('Error:'))

  await user.keyboard('e')

  expect(within(panel).getByText(/^Latest error: the last line matching error, fail, panic or exception/)).toBeDefined()
  expect(onLine(panel)).toContain('Cannot find module')
})

test('a search reads the whole log, beyond the lines held, for the lines that match', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  await waitFor(() => expect(shown(panel)).toContain('GET /health'))

  await user.type(within(panel).getByRole('textbox', { name: 'Search this log' }), '500')
  expect(within(panel).getByText('no match')).toBeDefined()
  await user.click(within(panel).getByRole('button', { name: 'Search the whole log' }))

  await waitFor(() => expect(shown(panel)).toBe('GET /invoices 500 error'))
  expect(within(panel).getByText('1 of 6 lines · the whole log')).toBeDefined()
  await user.click(within(panel).getByRole('button', { name: 'Back to the last lines' }))
  await waitFor(() => expect(shown(panel)).toContain('GET /health'))
})

test('a docker compose stack says its services log to Docker, and how to follow them', async () => {
  listing(
    runningWorktrees.map((w) =>
      w.branch === 'feat/billing' ? { ...w, jobs: [job('stack', { hold: 'detached', state: 'running' })] } : w,
    ),
  )
  server.use(
    ...answeringRunLogs({
      stack: {
        hold: 'detached',
        command: 'docker compose up -d',
        compose_logs: 'docker compose -p acme-feat-billing logs -f',
      },
    }),
  )
  await renderApp('/r/r1/running?worktree=feat/billing&job=stack')

  const panel = await panelOf('stack')
  expect(await within(panel).findByText('Its services log to Docker, not to wtm')).toBeDefined()
  expect(within(panel).getByText('docker compose -p acme-feat-billing logs -f')).toBeDefined()
  expect(within(panel).getByText(/^detached · /)).toBeDefined()
})

test("the job's menu offers its address, its log file, then what acts on the job", async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')

  await user.click(within(panel).getByRole('button', { name: 'More for web' }))

  const menu = await screen.findByRole('menu')
  expect(
    within(menu)
      .getAllByRole('menuitem')
      .map((item) => item.textContent),
  ).toEqual([
    'Open the address',
    'Copy the address',
    'Copy the log file’s path',
    'Download the log',
    'Restart',
    'Stop',
    'Pin to the top',
  ])
})

// In the sandbox, E pressed as the panel opened said no line was an error, then never looked at the lines that came.
test('the latest error asked before the lines arrive lands on it once they do', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=api')
  const panel = await panelOf('api')

  await user.keyboard('e')

  await waitFor(() => expect(onLine(panel)).toContain('Cannot find module'))
  expect(within(panel).getByText(/^Latest error: /)).toBeDefined()
})
