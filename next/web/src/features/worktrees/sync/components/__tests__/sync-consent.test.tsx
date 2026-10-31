import { screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { syncDryRun, type World } from '@/test/sync'
import { fromMenu, recorded } from '@/test/sync-app'

const answering = (world: World) =>
  server.use(
    http.post('/api/repos/r1/commands/sync', async ({ request }) =>
      syncDryRun((await request.json()) as WebCall, world),
    ),
  )

async function fromSelection(...names: string[]) {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  await app.user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: 'Sync' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

const nextOf = (dialog: HTMLElement) => within(dialog).getByRole('button', { name: 'Next' })

test("a dirty main unchecked turns the parents' fast-forward off: the run goes on without forcing it", async () => {
  answering({ dirtyOnto: ['main'] })
  const { user, dialog } = await fromMenu('fix/login')
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Sync fix/login' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward main first' }))
  await waitFor(() => expect(nextOf(dialog).getAttribute('aria-disabled')).toBe('false'))

  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward main first' }))

  await waitFor(() => expect(nextOf(dialog).getAttribute('aria-disabled')).toBe('false'))
  expect(await within(dialog).findByText('rebased onto as it stands · 3 commits behind origin')).toBeTruthy()
})

test("a dirty main's consent counts only while the parents are fast-forwarded: turned off elsewhere, no force", async () => {
  const calls = recorded()
  answering({ dirtyOnto: ['main'] })
  const { user, dialog } = await fromSelection('feat/billing-ui', 'fix/login')
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Fast-forward main first' }))

  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing first' }))

  await waitFor(() => {
    const last = calls.dryRuns().at(-1)?.answers ?? []
    expect(last).toContainEqual({ path: 'ff_parents', values: ['false'] })
    expect(last).not.toContainEqual({ path: 'force', values: ['true'] })
  })
})

test("a selection made only of a cycle's members is refused whole: the Alert, never the command's own pick", async () => {
  answering({ cycle: ['feat/search', 'feat/search-docs'] })
  const { dialog } = await fromSelection('feat/search', 'feat/search-docs')

  expect(await within(dialog).findByText("Can't sync")).toBeTruthy()
  expect(within(dialog).queryByRole('region', { name: "Can't sync" })).toBeNull()
  expect(within(dialog).queryByRole('checkbox', { name: 'Sync feat/billing' })).toBeNull()
})

test('the cycle refusal names one fix: only that member offers it, the others say why', async () => {
  answering({ cycle: ['feat/search', 'feat/search-docs'] })
  const { dialog } = await fromSelection('feat/billing', 'feat/search', 'feat/search-docs')

  const cant = await within(dialog).findByRole('region', { name: "Can't sync" })
  expect(within(cant).getAllByText('its parents lead back to it')).toHaveLength(2)
  expect(within(cant).getAllByRole('button', { name: 'Reparent onto main' })).toHaveLength(1)
})

test('Descendants off, each worktree left is listed once, with the reason that holds: an old tip only when its parent moves', async () => {
  answering({ diverged: ['feat/search'] })
  const { user, dialog } = await fromSelection('feat/billing', 'feat/search')
  await user.click(await within(dialog).findByRole('switch', { name: 'Sync the worktrees under the ones picked' }))
  await user.click(nextOf(dialog))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getAllByRole('listitem', { name: 'feat/search-docs' })).toHaveLength(1)
  expect(within(left).getByText('its parent feat/search is left as it is')).toBeTruthy()
  expect(within(left).getByText("stays on feat/billing's old tip · needs a sync after")).toBeTruthy()
  const runs = within(dialog).getByRole('region', { name: 'How it runs' })
  expect(within(runs).getByText('feat/billing-ui stays where it is; each needs a sync after.')).toBeTruthy()
})

test('a worktree left under a parent that only pushes stays on it as it is, on the list as in How it runs', async () => {
  answering({ synced: ['feat/billing'] })
  const { user, dialog } = await fromMenu('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))
  await user.click(await within(dialog).findByRole('switch', { name: 'Sync the 1 worktree under feat/billing' }))
  await user.click(within(dialog).getByRole('switch', { name: 'Push to origin' }))
  await user.click(nextOf(dialog))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('stays on feat/billing as it is · needs a sync after')).toBeTruthy()
  const runs = within(dialog).getByRole('region', { name: 'How it runs' })
  expect(within(runs).getByText('feat/billing-ui stays where it is; each needs a sync after.')).toBeTruthy()
})

test("unchecking a dirty main clears its consent: a clean branch's box never brings it back forced", async () => {
  const calls = recorded()
  answering({ dirtyOnto: ['main'] })
  const { user, dialog } = await fromSelection('feat/billing-ui', 'fix/login')
  const main = await within(dialog).findByRole('checkbox', { name: 'Fast-forward main first' })
  await user.click(main)
  await user.click(main)

  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward feat/billing first' }))

  await waitFor(() => {
    const last = calls.dryRuns().at(-1)?.answers ?? []
    expect(last).not.toContainEqual({ path: 'ff_parents', values: ['false'] })
    expect(last).not.toContainEqual({ path: 'force', values: ['true'] })
  })
  expect(main.getAttribute('data-state')).toBe('unchecked')
})
