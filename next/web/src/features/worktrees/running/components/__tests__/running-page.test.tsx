import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope, listDetail } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { answeringRunUp, heldRun, job, runningWorktrees } from '@/test/run-up'
import { server } from '@/test/server'

const listing = (details = runningWorktrees) =>
  server.use(http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', details))))

const startOne = async (user: Awaited<ReturnType<typeof renderApp>>['user'], worktree: string) => {
  await user.click(await screen.findByRole('button', { name: 'Start a profile' }))
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  await user.click(await within(dialog).findByRole('checkbox', { name: `Start in ${worktree}` }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const start = await within(dialog).findByRole('button', { name: 'Start dev' })
  await waitFor(() => expect(start.getAttribute('aria-disabled')).toBe('false'))
  await user.click(start)
  return dialog
}

test('nothing running says so in the header and the page, Stop jobs not offered yet and no count', async () => {
  await renderApp('/r/r1/running')

  expect(await screen.findByText('Nothing is running')).toBeDefined()
  expect(screen.getByText('Nothing running in acme')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Stop jobs' }).getAttribute('aria-disabled')).toBe('true')
  expect(screen.getAllByRole('button', { name: 'Start a profile' }).length, 'the page has one primary').toBe(1)
  expect(screen.getByRole('link', { name: /Running/ }).closest('li')?.textContent).toBe('Running')
})

test('each worktree with jobs up or crashed is a group, its crash first, and the sidebar counts the jobs up', async () => {
  listing()
  await renderApp('/r/r1/running')

  const group = await screen.findByRole('region', { name: 'feat/billing' })
  expect(within(group).getByText('profile dev · 1 up · 1 crashed')).toBeDefined()
  expect(
    within(group)
      .getAllByRole('listitem')
      .map((row) => row.getAttribute('aria-label')),
  ).toEqual(['api', 'web'])
  expect(within(group).getByText('crashed · exit 1')).toBeDefined()
  expect(screen.getByText('1 job up in 1 worktree · 1 crashed')).toBeDefined()
  expect(screen.getByRole('link', { name: /Running/ }).closest('li')?.textContent).toBe('Running1')
})

// After a start and a run down of both worktrees, the page kept their groups and said jobs ran there.
test('worktrees whose jobs were all stopped or exited leave the page empty', async () => {
  listing([
    listDetail('feat/search', { jobs: [job('web', { state: 'stopped' }), job('migrate', { state: 'exited' })] }),
    listDetail('feat/search-docs', { jobs: [job('api', { state: 'stopped' })] }),
  ])
  await renderApp('/r/r1/running')

  expect(await screen.findByText('Nothing is running')).toBeDefined()
  expect(screen.getByText('Nothing running in acme')).toBeDefined()
})

test("a worktree's menu offers its profiles, the one it runs disabled, and one picked opens on Confirm", async () => {
  listing()
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'More for feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))
  const dev = await screen.findByRole('menuitem', { name: /^dev/ })
  expect(dev.getAttribute('aria-disabled')).toBe('true')
  expect(dev.textContent).toContain('running')
  // happy-dom's pointer never settles in a submenu: the keyboard picks it, as a person may
  screen.getByRole('menuitem', { name: /^full/ }).focus()
  await user.keyboard('{Enter}')

  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  expect(await within(dialog).findByRole('button', { name: 'Start full' })).toBeDefined()
})

test('hidden while it starts, the run goes on in a toast that opens it again, the page greying what comes up', async () => {
  const held = heldRun()
  server.use(...answeringRunUp(undefined, held.run))
  const { user } = await renderApp('/r/r1/running')

  const dialog = await startOne(user, 'feat/search')
  expect(await within(dialog).findByText('Starting dev in feat/search · 0 of 4 up')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Hide' }))

  expect(await screen.findByText('Starting dev in feat/search')).toBeDefined()
  const group = await screen.findByRole('region', { name: 'feat/search' })
  expect(within(group).getByRole('listitem', { name: 'web' }).textContent).toContain('waits for migrate')

  await user.click(screen.getByRole('button', { name: 'Show' }))
  expect(await screen.findByRole('dialog', { name: 'Start a profile' })).toBeDefined()
  await waitFor(() => expect(screen.queryByText('Starting dev in feat/search')).toBeNull())
  await held.finish()
})

// An ended start greyed its rows again on every later read of the list, and stayed in the store.
test('a start that ended hidden leaves the page to the reads once the first lands, and the next opening fresh', async () => {
  const held = heldRun()
  server.use(...answeringRunUp(undefined, held.run))
  const { user, queryClient } = await renderApp('/r/r1/running')

  const dialog = await startOne(user, 'feat/search')
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))
  await screen.findByRole('region', { name: 'feat/search' })
  await held.finish()

  expect(await screen.findByText('dev started in feat/search')).toBeDefined()
  await waitFor(() => expect(screen.queryByRole('region', { name: 'feat/search' })).toBeNull())
  await queryClient.invalidateQueries({ queryKey: ['repo', 'r1', 'list'] })
  expect(screen.queryByRole('region', { name: 'feat/search' })).toBeNull()

  await user.click(screen.getByRole('button', { name: 'Start a profile' }))
  expect(await screen.findByRole('checkbox', { name: 'Start in feat/search' })).toBeDefined()
})
