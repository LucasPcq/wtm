import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { answeringRunDown, heldRunDown, runDownRun } from '@/test/run-down'
import { server } from '@/test/server'

type User = Awaited<ReturnType<typeof renderApp>>['user']

const enabled = (dialog: HTMLElement, name: string) =>
  waitFor(() => expect(within(dialog).getByRole('button', { name }).getAttribute('aria-disabled')).toBe('false'))

const rows = (region: HTMLElement) =>
  within(region)
    .getAllByRole('listitem')
    .map((row) => row.textContent)

// from the page's header: both worktrees checked, Next, then Confirm
const confirmBoth = async (user: User) => {
  await user.click(await screen.findByRole('button', { name: 'Stop jobs' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Stop in feat/billing' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Stop in fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await enabled(dialog, 'Stop jobs in 2 worktrees')
  return dialog
}

test('Stop jobs lists only the worktrees that run something, each with what it runs', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Stop jobs' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })

  expect(within(dialog).getByText('Stops every job of the worktrees you pick (run down).')).toBeDefined()
  const list = await within(dialog).findByRole('list', { name: 'Worktrees' })
  expect(rows(list)).toEqual([
    'mainholds postgres',
    'feat/billingrunning dev · 2 up · 1 crashed',
    'fix/loginrunning full · 2 up',
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
    'postgresboth let go: keeps running for main',
  ])
  expect(rows(within(dialog).getByRole('region', { name: 'How it runs' }))).toEqual([
    'Their databases stay: stopping drops nothing.',
    'Their logs stay here; Start a profile brings the jobs back.',
  ])
  expect(within(dialog).getByRole('button', { name: 'Back' })).toBeDefined()
})

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

test('the outcome says what was released and for whom, then what stopped in each worktree', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  expect(await within(dialog).findByText('Jobs stopped in 2 worktrees.')).toBeDefined()
  const released = within(dialog).getByRole('region', { name: 'Released' })
  expect(within(released).getByText('postgres')).toBeDefined()
  expect(within(released).getByText('keeps running for main')).toBeDefined()
  const stopped = within(dialog).getByRole('region', { name: 'Stopped' })
  expect(within(stopped).getByText('web, api · worker cleared')).toBeDefined()
  expect(within(stopped).getByText('web, stack')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Done' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test("a stack that doesn't stop fails its worktree alone, said first", async () => {
  server.use(...answeringRunDown(runDownRun(['fix/login'])))
  const { user } = await renderApp('/r/r1/running')
  const dialog = await confirmBoth(user)

  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))

  expect(await within(dialog).findByText('Jobs stopped in 1 of 2 worktrees.')).toBeDefined()
  const failed = within(dialog).getByRole('region', { name: "Couldn't stop" })
  expect(within(failed).getByText("stack didn't stop: exit 1")).toBeDefined()
  expect(
    within(within(dialog).getByRole('region', { name: 'Stopped' })).getByText('web, api · worker cleared'),
  ).toBeDefined()
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
    'postgresfeat/billing lets go: keeps running for main, fix/login',
  ])
  expect(within(dialog).queryByRole('button', { name: 'Back' })).toBeNull()
})

test("a group's menu offers Stop its jobs, saying the hold it releases", async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'More for fix/login' }))
  const item = await screen.findByRole('menuitem', { name: /Stop its jobs/ })
  expect(item.textContent).toBe('Stop its jobsrun down · releases its hold on postgres')
  await user.click(item)

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  expect(await within(dialog).findByRole('region', { name: 'profile full' })).toBeDefined()
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

test('the detail of a worktree that runs nothing says so on Confirm, nothing to press', async () => {
  server.use(...answeringRunDown())
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fsearch')
  const detail = await screen.findByRole('complementary', { name: 'feat/search' })

  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/search' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Stop its jobs' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(rows(left)).toEqual(['feat/searchnothing running'])
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Stop jobs' }).getAttribute('aria-disabled')).toBe('true'),
  )
})
