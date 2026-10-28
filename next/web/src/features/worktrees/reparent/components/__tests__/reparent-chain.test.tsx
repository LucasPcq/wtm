import { screen, waitFor, within } from '@testing-library/react'
import { expect, onTestFinished, test } from 'vitest'
import type { DomainAnswered, WebCall } from '@/api/types.gen'
import { cleanPlan } from '@/test/clean'
import { answeringClean, selectAndRemove } from '@/test/remove'
import { cleanLeaving, rememberingReparent } from '@/test/reparent'
import { server } from '@/test/server'

const runsOf = () => {
  const runs: DomainAnswered[][] = []
  const listen = async ({ request }: { request: Request }) => {
    if (!request.url.endsWith('/commands/reparent')) return
    const call = (await request.clone().json()) as WebCall
    if (!call.dry_run) runs.push(call.answers ?? [])
  }
  server.events.on('request:start', listen)
  onTestFinished(() => server.events.removeListener('request:start', listen))
  return runs
}

const move = (worktree: string, to: string) => [
  { path: 'worktree', values: [worktree] },
  { path: 'to', values: [to] },
]

// Remove takes feat/billing and feat/search, and leaves their children: feat/billing-ui onto main, feat/search-docs
// onto searchTo
async function leftOrphans(searchTo: string) {
  server.use(
    answeringClean(
      cleanPlan,
      cleanLeaving([
        { branch: 'feat/billing-ui', from: 'feat/billing', to: 'main' },
        { branch: 'feat/search-docs', from: 'feat/search', to: searchTo },
      ]),
    ),
  )
  const app = await selectAndRemove('feat/billing', 'feat/search')
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Next' }))
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Next' }))
  await app.user.click(await within(app.dialog).findByRole('button', { name: 'Remove 2 worktrees' }))
  const toCheck = await within(app.dialog).findByRole('region', { name: 'To check' })
  return { ...app, toCheck }
}

test('Reparent all over two parents is two runs, one after the other, one toast, and its Undo undoes both', async () => {
  const runs = runsOf()
  server.use(...rememberingReparent())
  const { user, toCheck } = await leftOrphans('fix/login')

  await user.click(within(toCheck).getByRole('button', { name: 'Reparent all onto the nearest that stays' }))

  expect(await screen.findByText('2 worktrees moved')).toBeTruthy()
  expect(runs).toEqual([move('feat/billing-ui', 'main'), move('feat/search-docs', 'fix/login')])
  await user.click(screen.getByRole('button', { name: 'Undo' }))

  expect(await screen.findByText('2 worktrees moved')).toBeTruthy()
  await waitFor(() => expect(runs).toHaveLength(4))
  expect(runs.slice(2)).toEqual([move('feat/billing-ui', 'feat/billing'), move('feat/search-docs', 'feat/search')])
})

test('a chain failing midway names what did not move, says what moved, and its Undo says what it undoes', async () => {
  const runs = runsOf()
  server.use(...rememberingReparent({ failing: ['feat/search-docs'] }))
  const { user, toCheck } = await leftOrphans('fix/login')

  await user.click(within(toCheck).getByRole('button', { name: 'Reparent all onto the nearest that stays' }))

  expect(await screen.findByText("Couldn't reparent feat/search-docs")).toBeTruthy()
  expect(screen.getByText("Couldn't write its record. feat/billing-ui moved onto main.")).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Undo 1 move' }))

  await waitFor(() => expect(runs).toHaveLength(3))
  expect(runs[2]).toEqual(move('feat/billing-ui', 'feat/billing'))
})

test('a chain that stops before a later run applies still says what moved before, with its Undo', async () => {
  server.use(...rememberingReparent())
  // feat/gone is no branch: the dry run of feat/search-docs refuses it
  const { user, toCheck } = await leftOrphans('feat/gone')

  await user.click(within(toCheck).getByRole('button', { name: 'Reparent all onto the nearest that stays' }))

  expect(await screen.findByText('feat/billing-ui is on main')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Undo' })).toBeTruthy()
  expect(await screen.findByText("Couldn't reparent feat/search-docs")).toBeTruthy()
  expect(screen.getByText('feat/gone no longer exists')).toBeTruthy()
})

test("while a reparent runs, To check's Reparent buttons wait: one run at a time", async () => {
  let release = () => {}
  server.use(...rememberingReparent({ held: new Promise<void>((resolve) => (release = resolve)) }))
  const { user, toCheck } = await leftOrphans('fix/login')

  await user.click(
    within(toCheck).getByRole('button', {
      name: 'Reparent: feat/billing-ui still points at the removed parent feat/billing',
    }),
  )

  const all = within(toCheck).getByRole('button', { name: 'Reparent all onto the nearest that stays' })
  await waitFor(() => expect(all.hasAttribute('disabled')).toBe(true))
  release()
  expect(await screen.findByText('feat/billing-ui is on main')).toBeTruthy()
})
