import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse } from 'msw'
import { expect, test } from 'vitest'
import { cleanPlan } from '@/test/clean'
import { answeringClean, liveEvents, selectAndRemove } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

test('an unsafe worktree starts unchecked with its reason; the others go through the children and the plan, then run', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing', 'fix/login')

  expect(within(dialog).getByRole('heading', { name: 'Remove worktrees' })).toBeTruthy()
  const unsafe = await within(dialog).findByRole('listitem', { name: 'fix/login' })
  expect(within(unsafe).getByText('Uncommitted changes')).toBeTruthy()
  expect(within(unsafe).getByRole('checkbox', { name: 'Remove fix/login' }).getAttribute('aria-checked')).toBe('false')
  expect(within(dialog).getByText('1 of 2 selected')).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(
    await within(dialog).findByText("feat/billing-ui is stacked on feat/billing, which you're removing."),
  ).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: 'Move it onto main' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'feat/billing' })
  expect(within(planned).getByText('feat/billing-ui moves onto main')).toBeTruthy()
  expect(within(dialog).queryByText('docker compose down')).toBeNull()
  expect(within(dialog).getByRole('button', { name: /^Details/ }).textContent).toContain('1 on_clean hook runs first')
  await user.click(within(dialog).getByRole('button', { name: /^Details/ }))
  expect(within(dialog).getByText('docker compose down')).toBeTruthy()
  expect(within(dialog).queryByRole('listitem', { name: 'fix/login' })).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

  expect(await within(dialog).findByText(/^1 removed\./)).toBeTruthy()
  expect(within(dialog).getByRole('heading', { name: 'Remove worktrees' })).toBeTruthy()
  expect(within(dialog).getByText('feat/billing-ui moved onto main')).toBeTruthy()
  expect(within(dialog).queryByText(/still points at the removed parent/)).toBeNull()
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('checking an unsafe worktree turns its reason into what is lost, tags it anyway, and forces it', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')

  const unsafe = await within(dialog).findByRole('listitem', { name: 'fix/login' })
  await user.click(within(unsafe).getByRole('checkbox', { name: 'Remove fix/login' }))
  expect(within(unsafe).getByText('Its uncommitted changes are lost.')).toBeTruthy()
  expect(within(unsafe).getByText('anyway')).toBeTruthy()
  expect(within(unsafe).queryByText('Uncommitted changes')).toBeNull()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const planned = await within(dialog).findByRole('listitem', { name: 'fix/login' })
  expect(within(planned).getByText('anyway')).toBeTruthy()
  expect(within(planned).getByText('Its uncommitted changes are lost.')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

  expect(await within(dialog).findByText(/^1 removed\./)).toBeTruthy()
})

test("the detail's menu removes the worktree it shows, and a child left on it is named with its command", async () => {
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  const detail = await screen.findByRole('complementary', { name: 'feat/billing' })
  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing' })).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove' }))

  expect(
    await within(dialog).findByText('feat/billing-ui still points at the removed parent feat/billing'),
  ).toBeTruthy()
  expect(
    await within(dialog).findByRole('button', { name: 'Copy wtm reparent feat/billing-ui --to main' }),
  ).toBeTruthy()
  const fold = within(dialog).getByRole('button', {
    name: 'Details of feat/billing-ui still points at the removed parent feat/billing',
  })
  expect(fold.getAttribute('aria-expanded')).toBe('false')
  await user.click(fold)
  expect(fold.getAttribute('aria-expanded')).toBe('true')
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
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
})

test('a worktree read again unsafe on a later step brings the wizard back to it, unchecked with its reason', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText(/is stacked on/)

  server.use(answeringClean((call) => cleanPlan(call, ['feat/billing'])))
  events.changed()

  const unsafe = await within(dialog).findByRole('listitem', { name: 'feat/billing' })
  expect(within(unsafe).getByText('Uncommitted changes')).toBeTruthy()
  expect(within(unsafe).getByRole('checkbox', { name: 'Remove feat/billing' }).getAttribute('aria-checked')).toBe(
    'false',
  )
})

test('children all gone since leave the wizard nothing to ask about them', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText(/is stacked on/)

  server.use(
    answeringClean(async (call) => {
      const planned = await cleanPlan(call).json()
      return HttpResponse.json({ ...planned, plan: { ...planned.plan, moves: [] } })
    }),
  )
  events.changed()

  expect(await within(dialog).findByRole('button', { name: 'Remove' })).toBeTruthy()
  expect(within(dialog).queryByText(/stacked on/)).toBeNull()
})

test('nothing checked leaves nothing to remove, and Next says what would wake it', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')
  await within(dialog).findByRole('listitem', { name: 'fix/login' })

  const next = within(dialog).getByRole('button', { name: 'Next' })
  expect(next.getAttribute('aria-disabled')).toBe('true')
  await user.hover(next)
  expect(await screen.findByRole('tooltip', { name: 'Check a worktree to remove it' })).toBeTruthy()
})

test('children of several parents preview where each goes, moved onto the nearest that stays or left where they are', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing', 'feat/search')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText("2 worktrees are stacked on worktrees you're removing.")).toBeTruthy()
  const preview = within(dialog).getByRole('region', { name: 'Where each goes' })
  const moved = within(preview).getByRole('listitem', { name: 'feat/search-docs' })
  expect(within(moved).getByText('stays on feat/search')).toBeTruthy()

  await user.click(within(dialog).getByRole('radio', { name: /Move each onto the nearest worktree that stays/ }))
  expect(within(moved).getByText('feat/search')).toBeTruthy()
  expect(within(moved).getByLabelText('onto')).toBeTruthy()
  expect(within(moved).getByText('main')).toBeTruthy()
  expect(within(preview).getAllByRole('listitem')).toHaveLength(2)
})

test('a Next waiting on a check says why to the keyboard too', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')
  await within(dialog).findByRole('listitem', { name: 'fix/login' })
  const next = within(dialog).getByRole('button', { name: 'Next' })

  for (let i = 0; i < 20 && document.activeElement !== next; i++) await user.tab()

  expect(document.activeElement).toBe(next)
  expect(await screen.findByRole('tooltip', { name: 'Check a worktree to remove it' })).toBeTruthy()
})

test('a press on a Next waiting for its plan goes nowhere, even once the plan lands', async () => {
  let release = () => {}
  const held = new Promise<void>((resolve) => {
    release = resolve
  })
  server.use(
    answeringClean(async (call) => {
      if (!call.answers?.some((a) => a.path === 'force')) await held
      return cleanPlan(call)
    }),
  )
  const { user, dialog } = await selectAndRemove('feat/search-docs')
  await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })
  const next = within(dialog).getByRole('button', { name: 'Next' })
  expect(next.getAttribute('aria-disabled')).toBe('true')

  await user.click(next)
  release()

  await waitFor(() => expect(next.getAttribute('aria-disabled')).toBe('false'))
  expect(within(dialog).getByRole('listitem', { name: 'feat/search-docs' })).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: 'Remove' })).toBeNull()
})
