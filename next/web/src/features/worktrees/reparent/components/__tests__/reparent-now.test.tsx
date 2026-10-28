import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, onTestFinished, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { envelope, statusDetail } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { answeringReparent, rememberingReparent, reparentRun } from '@/test/reparent'
import { server } from '@/test/server'

const runsOf = () => {
  const runs: WebCall[] = []
  const listen = async ({ request }: { request: Request }) => {
    if (!request.url.endsWith('/commands/reparent')) return
    const call = (await request.clone().json()) as WebCall
    if (!call.dry_run) runs.push(call)
  }
  server.events.on('request:start', listen)
  onTestFinished(() => server.events.removeListener('request:start', listen))
  return runs
}

// status proposes the move: worktree's parent is gone, its fix the move onto to
const proposes = (worktree: string, to: string) =>
  http.post('/api/repos/r1/commands/status', () =>
    HttpResponse.json(
      envelope('status', [
        statusDetail(worktree, {
          problems: [
            {
              code: 'status.parent_gone',
              message: 'its parent feat/search is no longer a branch',
              fix: {
                command: 'reparent',
                answers: [
                  { path: 'worktree', values: [worktree] },
                  { path: 'to', values: [to] },
                ],
                code: 'status.reparent',
                message: `reparent onto ${to}`,
              },
            },
          ],
        }),
      ]),
    ),
  )

test("the callout's fix runs at once, and its toast's Undo moves the worktree back", async () => {
  const runs = runsOf()
  server.use(proposes('feat/search-docs', 'main'), ...rememberingReparent())
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fsearch-docs')

  const callout = await screen.findByRole('region', { name: 'To do' })
  await user.click(await within(callout).findByRole('button', { name: 'Reparent onto main' }))

  expect(await screen.findByText('feat/search-docs is on main')).toBeTruthy()
  expect(screen.getByText('It rebases onto it on its next sync.')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(screen.getByRole('button', { name: 'Copy wtm sync' })).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Undo' }))

  expect(await screen.findByText('feat/search-docs is on feat/search')).toBeTruthy()
  expect(runs.map((call) => call.answers)).toEqual([
    [
      { path: 'worktree', values: ['feat/search-docs'] },
      { path: 'to', values: ['main'] },
    ],
    [
      { path: 'worktree', values: ['feat/search-docs'] },
      { path: 'to', values: ['feat/search'] },
    ],
  ])
})

test('a move its dry run refuses ends on an error toast saying why, whose Details opens the dialog on it', async () => {
  const runs = runsOf()
  server.use(proposes('feat/billing', 'feat/billing-ui'))
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  const callout = await screen.findByRole('region', { name: 'To do' })
  await user.click(await within(callout).findByRole('button', { name: 'Reparent onto feat/billing-ui' }))

  expect(await screen.findByText("Couldn't reparent feat/billing")).toBeTruthy()
  expect(screen.getByText('feat/billing-ui sits under feat/billing')).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Details' }))
  const dialog = await screen.findByRole('dialog', { name: 'Change parent' })
  expect(await within(dialog).findByText('feat/billing-ui sits under feat/billing')).toBeTruthy()
  await waitFor(() => expect(runs).toEqual([]))
})

test('a failed outcome closed once read no longer stands in the way of the next move', async () => {
  server.use(proposes('feat/search-docs', 'main'), ...answeringReparent(reparentRun(['fix/login'])))
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fsearch-docs')

  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing-ui' }))
  await user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: 'Reparent' }))
  const dialog = await screen.findByRole('dialog', { name: 'Change parent' })
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'feat/search' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Reparent 2 worktrees' }))
  expect(await within(dialog).findByText('1 of 2 reparented.')).toBeTruthy()
  await user.keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

  const callout = await screen.findByRole('region', { name: 'To do' })
  await user.click(within(callout).getByRole('button', { name: 'Reparent onto main' }))

  expect(await screen.findByText('feat/search-docs is on main')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
})
