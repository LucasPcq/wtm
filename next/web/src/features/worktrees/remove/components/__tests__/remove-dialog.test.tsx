import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope, followUp } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server, worktrees } from '@/test/server'

async function openRemove() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Remove…' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

test('remove asks which worktrees, shows the plan with the children it leaves, runs it, and names each child left with its command', async () => {
  const { user, dialog } = await openRemove()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByText('worktree is required')).toBeTruthy()

  await user.click(within(dialog).getByRole('checkbox', { name: 'feat/billing' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByRole('heading', { name: 'Remove feat/billing' })).toBeTruthy()
  expect(within(dialog).getByText('/acme.trees/feat-billing')).toBeTruthy()
  expect(within(dialog).getByText('feat/billing-ui still points at feat/billing.')).toBeTruthy()
  expect(within(dialog).getByRole('switch', { name: 'Move it onto main' })).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Remove feat/billing' }))

  expect(await within(dialog).findByText('1 worktree removed')).toBeTruthy()
  expect(within(dialog).getByText('Removed')).toBeTruthy()
  expect(within(dialog).getByText('feat/billing-ui still points at the removed parent feat/billing')).toBeTruthy()
  expect(await within(dialog).findByText('wtm reparent feat/billing-ui --to main')).toBeTruthy()
})

test('children moved onto the nearest worktree that stays leave no warning', async () => {
  const { user, dialog } = await openRemove()
  await user.click(await within(dialog).findByRole('checkbox', { name: 'feat/billing' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  await user.click(await within(dialog).findByRole('switch', { name: 'Move it onto main' }))
  await user.click(within(dialog).getByRole('button', { name: 'Remove feat/billing' }))

  expect(await within(dialog).findByText('1 worktree removed')).toBeTruthy()
  expect(within(dialog).queryByText(/still points at the removed parent/)).toBeNull()
})

test('an unsafe worktree is refused with its reason until removing it anyway is ticked', async () => {
  const { user, dialog } = await openRemove()
  await user.click(await within(dialog).findByRole('checkbox', { name: 'fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const refused = await within(dialog).findByRole('region', { name: 'Refused' })
  expect(within(refused).getByText('Uncommitted changes')).toBeTruthy()
  expect(
    within(dialog)
      .getByRole('button', { name: /^Remove/ })
      .hasAttribute('disabled'),
  ).toBe(true)

  await user.click(within(dialog).getByRole('checkbox', { name: 'Remove anyway: what they hold is lost' }))

  expect(await within(dialog).findByText('Uncommitted changes, removed anyway')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Remove fix/login' }).hasAttribute('disabled')).toBe(false)
})

test("the detail's menu removes the worktree it shows, straight to the plan", async () => {
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
  expect(within(dialog).queryByText('Clean (delete the worktree)')).toBeNull()
})
