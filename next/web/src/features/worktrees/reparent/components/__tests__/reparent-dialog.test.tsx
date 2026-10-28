import { screen, waitFor, within } from '@testing-library/react'
import { expect, onTestFinished, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { answeringReparent, reparentRun } from '@/test/reparent'
import { server } from '@/test/server'

function recorded() {
  const calls: WebCall[] = []
  const listen = async ({ request }: { request: Request }) => {
    if (request.url.endsWith('/commands/reparent')) calls.push((await request.clone().json()) as WebCall)
  }
  server.events.on('request:start', listen)
  onTestFinished(() => server.events.removeListener('request:start', listen))
  return { runs: () => calls.filter((call) => !call.dry_run) }
}

async function fromMenu(worktree: string) {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: `Actions on ${worktree}` }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Reparent' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Change parent' }) }
}

async function fromSelection(...names: string[]) {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  const bar = screen.getByRole('toolbar', { name: 'Selection' })
  await app.user.click(within(bar).getByRole('button', { name: 'Reparent' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Change parent' }) }
}

test('one worktree from its menu opens on its new parent: the refused ones say why, Confirm shows the move', async () => {
  const runs = recorded()
  const { user, dialog } = await fromMenu('feat/billing')

  expect(await within(dialog).findByText('feat/billing is on main.')).toBeTruthy()
  await user.click(await within(dialog).findByRole('combobox'))
  const under = await screen.findByRole('option', { name: /feat\/billing-ui/ })
  expect(under.textContent).toContain('sits under feat/billing')
  expect(under.getAttribute('aria-disabled')).toBe('true')
  expect((await screen.findByRole('option', { name: /the worktree you're moving/ })).textContent).toMatch(
    /^feat\/billing/,
  )
  await user.click(screen.getByRole('option', { name: 'fix/login' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const move = await within(dialog).findByRole('listitem', { name: 'feat/billing' })
  expect(within(move).getByText('main → fix/login')).toBeTruthy()
  expect(within(move).getByText('feat/billing-ui follows')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Reparent' }))

  expect(await within(dialog).findByText('1 reparented. It rebases onto fix/login on its next sync.')).toBeTruthy()
  expect(await within(dialog).findByText('wtm sync feat/billing')).toBeTruthy()
  expect(runs.runs()[0]?.answers).toEqual([
    { path: 'worktree', values: ['feat/billing'] },
    { path: 'to', values: ['fix/login'] },
  ])
})

test('a worktree already on the parent chosen is left as it is, and Reparent says why it waits', async () => {
  const { user, dialog } = await fromMenu('feat/billing')

  await user.click(await within(dialog).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'main' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('already on main')).toBeTruthy()
  const reparent = within(dialog).getByRole('button', { name: 'Reparent' })
  expect(reparent.getAttribute('aria-disabled')).toBe('true')
  // the pointer still rests on the button Next was
  await user.hover(within(dialog).getByRole('heading', { name: 'Change parent' }))
  await user.hover(reparent)
  expect((await screen.findAllByText('feat/billing is already on main')).length).toBeGreaterThan(0)
})

test('from the selection: the worktrees first, those that cannot move left as they are with why', async () => {
  const { user, dialog } = await fromSelection('main', 'fix/login', 'feat/billing-ui', 'feat/search-docs')

  expect(await within(dialog).findByText('3 of 3 selected')).toBeTruthy()
  expect(within(dialog).getByRole('listitem', { name: 'feat/billing-ui' }).textContent).toContain('on feat/billing')
  const left = within(dialog).getByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('The main worktree has no parent')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'feat/search' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('already on feat/search')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Reparent 2 worktrees' }))

  expect(await within(dialog).findByText('2 reparented. Each rebases onto feat/search on its next sync.')).toBeTruthy()
  const done = within(dialog).getByRole('region', { name: 'Reparented' })
  expect(within(done).getByText('main → feat/search')).toBeTruthy()
  expect(within(done).getByText('already on feat/search')).toBeTruthy()
})

test('a record that cannot be written fails alone, named with why', async () => {
  server.use(...answeringReparent(reparentRun(['feat/billing-ui'])))
  const { user, dialog } = await fromSelection('fix/login', 'feat/billing-ui')

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'feat/search' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Reparent 2 worktrees' }))

  expect(await within(dialog).findByText('1 of 2 reparented.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't reparent" })
  expect(within(failed).getByText("couldn't write its record: permission denied")).toBeTruthy()
  expect(within(dialog).getByText('wtm sync fix/login')).toBeTruthy()
})

test('Back from Confirm keeps the parent chosen; one worktree has no step of its own to go back to', async () => {
  const { user, dialog } = await fromMenu('feat/search-docs')

  await user.click(await within(dialog).findByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'main' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Back' }))

  await waitFor(() => expect(within(dialog).getByRole('combobox').textContent).toContain('main'))
  expect(within(dialog).queryByRole('button', { name: 'Back' })).toBeNull()
})
