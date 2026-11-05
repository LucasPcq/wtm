import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { answeringRunDown, questionsWith } from '@/test/run-down'
import { confirm, confirmBoth, disabled, enabled, rows } from '@/test/run-down-app'
import { server } from '@/test/server'

test('Stop jobs lists only the worktrees that run something, a claim alone included, each with what it runs', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Stop jobs' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })

  expect(within(dialog).getByText('Stops every job of the worktrees you pick (run down).')).toBeDefined()
  const list = await within(dialog).findByRole('list', { name: 'Worktrees' })
  expect(rows(list), 'main runs postgres for the others: nothing of its own to stop').toEqual([
    'feat/billingrunning dev · 2 up · 1 crashed',
    'fix/loginrunning full · 2 up',
    'feat/searchholds postgres',
  ])
  expect(within(dialog).getByText('0 of 3 selected · only the worktrees that run something')).toBeDefined()
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
})

test('Confirm says what stops in each worktree, the stack by its stop command, and the shared service released', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  const dialog = await confirmBoth(user)

  expect(within(dialog).getByText('their jobs stop · 4 up, 1 crashed')).toBeDefined()
  expect(rows(within(dialog).getByRole('region', { name: 'feat/billing · profile dev' }))).toEqual([
    'webstops',
    'apistops',
    'workercrashed: nothing to stop',
  ])
  expect(rows(within(dialog).getByRole('region', { name: 'fix/login · profile full' }))).toEqual([
    'webstops',
    'stackdocker compose down',
  ])
  expect(rows(within(dialog).getByRole('region', { name: 'Shared services, released' }))).toEqual([
    'postgresboth let go: keeps running for feat/search',
  ])
  expect(rows(within(dialog).getByRole('region', { name: 'How it runs' }))).toEqual([
    'Their databases stay: stopping drops nothing.',
    'Their logs stay here; Start a profile brings the jobs back.',
  ])
  expect(within(dialog).getByRole('button', { name: 'Back' })).toBeDefined()
})

test('a worktree that holds a claim alone confirms its release, nothing of its own', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  const dialog = await confirm(user, ['feat/search'], 'Stop jobs')

  expect(within(dialog).getByText('its jobs stop')).toBeDefined()
  expect(rows(within(dialog).getByRole('region', { name: 'Shared services, released' }))).toEqual([
    'postgresfeat/search lets go: keeps running for feat/billing, fix/login',
  ])
})

test('a plan the core refuses is said on Confirm, nothing to press', async () => {
  server.use(
    http.post('/api/repos/r1/commands/run-down', () =>
      HttpResponse.json(
        {
          ...envelope('run-down', []),
          ok: false,
          error: { kind: 'precondition', code: 'run-down.daemon_unavailable', message: 'no run daemon answers' },
        },
        { status: 412 },
      ),
    ),
    ...answeringRunDown(),
  )
  const { user } = await renderApp('/r/r1/running')
  await user.click(await screen.findByRole('button', { name: 'Stop jobs' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Stop in fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('no run daemon answers')).toBeDefined()
  await disabled(dialog, 'Stop jobs')
})

test("a group's stop button opens Confirm on that worktree, with no Back", async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  const group = await screen.findByRole('region', { name: 'feat/billing' })
  await user.click(within(group).getByRole('button', { name: 'Stop its jobs of feat/billing' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  await enabled(dialog, 'Stop jobs')
  expect(within(dialog).getByText('its jobs stop · 2 up, 1 crashed')).toBeDefined()
  expect(rows(within(dialog).getByRole('region', { name: 'Shared services, released' }))).toEqual([
    'postgresfeat/billing lets go: keeps running for fix/login, feat/search',
  ])
  expect(within(dialog).queryByRole('button', { name: 'Back' })).toBeNull()
})

test("a group's menu offers Stop its jobs, saying the hold it releases", async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'More for fix/login' }))
  const item = await screen.findByRole('menuitem', { name: /Stop its jobs/ })
  await waitFor(() => expect(item.textContent).toBe('Stop its jobsrun down · releases its hold on postgres'))
  await user.click(item)

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  expect(await within(dialog).findByRole('region', { name: 'profile full' })).toBeDefined()
})

// main held postgres itself: its group's menu said only "run down".
test("main's group names the shared service main holds itself", async () => {
  server.use(
    questionsWith((options) =>
      options.map((option) => (option.value === 'main' ? { ...option, holds: ['postgres'] } : option)),
    ),
    ...answeringRunDown(),
  )
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'More for main' }))

  const item = await screen.findByRole('menuitem', { name: /Stop its jobs/ })
  await waitFor(() => expect(item.textContent).toBe('Stop its jobsrun down · releases its hold on postgres'))
})

test("a worktree's menu on the Worktrees page opens Confirm on it", async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on fix/login' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Stop its jobs' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  await enabled(dialog, 'Stop jobs')
  expect(rows(within(dialog).getByRole('region', { name: 'profile full' }))).toEqual([
    'webstops',
    'stackdocker compose down',
  ])
})

// A worktree removed since the page read it was no longer an option: Confirm waited on "Planning…" for good.
test('a worktree no longer offered says it runs nothing on Confirm, nothing to press', async () => {
  server.use(
    questionsWith((options) => options.filter((option) => option.value !== 'fix/login')),
    ...answeringRunDown(),
  )
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on fix/login' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Stop its jobs' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(rows(left)).toEqual(['fix/loginnothing running'])
  await disabled(dialog, 'Stop jobs')
})

test('the detail of a worktree that runs nothing says so on Confirm, nothing to press', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling-ui')
  const detail = await screen.findByRole('complementary', { name: 'feat/billing-ui' })

  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/billing-ui' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Stop its jobs' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(rows(left)).toEqual(['feat/billing-uinothing running'])
  await disabled(dialog, 'Stop jobs')
})
