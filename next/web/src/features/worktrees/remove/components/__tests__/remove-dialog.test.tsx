import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { cleanPlan } from '@/test/clean'
import { envelope, followUp } from '@/test/fixtures'
import { answeringClean, liveEvents, selectAndRemove } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server, worktrees } from '@/test/server'

test('an unsafe worktree waits for a choice; kept, the others go through the children and the plan, then run', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing', 'fix/login')

  const unsafe = await within(dialog).findByRole('region', { name: 'fix/login' })
  expect(within(unsafe).getByText('Uncommitted changes')).toBeTruthy()
  expect(within(dialog).getByText("1 of 2 can't be removed safely: choose for each before going on.")).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)

  await user.click(within(unsafe).getByRole('radio', { name: 'Keep it' }))
  expect(within(unsafe).getByText('Kept: it stays as it is.')).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('feat/billing-ui is stacked on feat/billing, which goes.')).toBeTruthy()
  const children = within(dialog).getByRole('region', { name: 'Under feat/billing' })
  expect(within(children).getByText('stays on feat/billing, which goes')).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: /Move it onto the nearest worktree that stays/ }))
  expect(within(children).getByText('base branch')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('region', { name: 'feat/billing' })
  expect(within(planned).getByText('feat/billing-ui moves onto main')).toBeTruthy()
  expect(within(planned).getByText('docker compose down')).toBeTruthy()
  expect(within(dialog).queryByRole('region', { name: 'fix/login' })).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Remove feat/billing' }))

  expect(await within(dialog).findByText('feat/billing is removed')).toBeTruthy()
  expect(
    within(dialog).getByText('Removed, with its branch and its record; feat/billing-ui moved onto main.'),
  ).toBeTruthy()
  expect(within(dialog).queryByText(/still points at the removed parent/)).toBeNull()
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('removing an unsafe worktree anyway says what is lost and forces it', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')

  const unsafe = await within(dialog).findByRole('region', { name: 'fix/login' })
  await user.click(within(unsafe).getByRole('radio', { name: 'Remove anyway' }))

  expect(within(unsafe).getByText('Removed anyway: its uncommitted changes are lost.')).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const planned = await within(dialog).findByRole('region', { name: 'fix/login' })
  expect(within(planned).getByText('Removed anyway')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Remove fix/login' }))

  expect(await within(dialog).findByText('fix/login is removed')).toBeTruthy()
})

test("the detail's menu removes the worktree it shows, and a child left on it is named with its command", async () => {
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json(
        envelope(
          'list',
          worktrees.map((w) => ({
            ...w,
            actions: w.main
              ? []
              : [followUp('clean', w.branch, { code: 'list.clean', message: 'Clean (delete the worktree)' })],
          })),
        ),
      ),
    ),
  )
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  await user.click(await screen.findByRole('button', { name: 'More actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove worktree…' }))
  const dialog = await screen.findByRole('dialog')
  expect(await within(dialog).findByRole('heading', { name: 'Remove feat/billing' })).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing' }))

  expect(
    await within(dialog).findByText('feat/billing-ui still points at the removed parent feat/billing'),
  ).toBeTruthy()
  expect(await within(dialog).findByText('wtm reparent feat/billing-ui --to main')).toBeTruthy()
})

test('a plan that cannot be made says why instead of a Next that never wakes', async () => {
  server.use(
    answeringClean((call) =>
      call.answers?.some((a) => a.path === 'force')
        ? cleanPlan(call)
        : HttpResponse.json({ error: { message: 'git is busy' } }, { status: 500 }),
    ),
  )
  const { dialog } = await selectAndRemove('feat/billing-ui')

  expect(await within(dialog).findByText('git is busy')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)
})

test('a worktree read again unsafe on a later step brings the wizard back to its choice', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText('Children left behind')

  server.use(answeringClean((call) => cleanPlan(call, ['feat/billing'])))
  events.changed()

  expect(
    await within(dialog).findByText("1 of 1 can't be removed safely: choose for each before going on."),
  ).toBeTruthy()
  expect(
    within(within(dialog).getByRole('region', { name: 'feat/billing' })).getByText('Uncommitted changes'),
  ).toBeTruthy()
})

test('children all gone since leave the wizard nothing to ask about them', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText('Children left behind')

  server.use(
    answeringClean(async (call) => {
      const planned = await cleanPlan(call).json()
      return HttpResponse.json({ ...planned, plan: { ...planned.plan, moves: [] } })
    }),
  )
  events.changed()

  expect(await within(dialog).findByRole('heading', { name: 'Remove feat/billing' })).toBeTruthy()
  expect(within(dialog).queryByText(/stacked on/)).toBeNull()
  expect(within(dialog).getByRole('button', { name: 'Remove feat/billing' })).toBeTruthy()
})

test('every worktree kept leaves nothing to remove', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')

  await user.click(
    within(await within(dialog).findByRole('region', { name: 'fix/login' })).getByRole('radio', { name: 'Keep it' }),
  )

  expect(within(dialog).getByText('Every worktree is kept: nothing to remove.')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)
})
