import { screen, waitFor, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { cleanPlan } from '@/test/clean'
import { envelope, listDetail } from '@/test/fixtures'
import { answeringClean, liveEvents, selectAndRemove } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server, worktrees } from '@/test/server'

test('from the header, Remove opens on every worktree it can remove, none checked, with its search', async () => {
  const many = ['feat/a', 'feat/b', 'feat/c', 'feat/d'].map((branch) => listDetail(branch))
  server.use(
    http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', [...worktrees, ...many]))),
  )
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))

  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByText('0 of 9 selected')).toBeTruthy()
  await user.click(within(dialog).getByRole('checkbox', { name: 'Remove feat/billing' }))
  await user.type(within(dialog).getByRole('textbox', { name: 'Search the worktrees' }), 'Feat/A')

  expect(within(dialog).getByText('1 of 9 selected · 1 hidden by the search')).toBeTruthy()
  const rows = within(dialog)
    .getAllByRole('listitem')
    .map((row) => row.getAttribute('aria-label'))
  expect(rows.filter(Boolean)).toEqual(['feat/a'])
  await user.click(within(dialog).getByRole('checkbox', { name: 'Select every worktree shown' }))
  expect(within(dialog).getByText('2 of 9 selected · 1 hidden by the search')).toBeTruthy()
})

// A row checked while safe stayed checked once it turned unsafe, and the run went out forced, chosen by nobody.
test('a row checked while safe that turns unsafe drops its tick and brings the wizard back to it', async () => {
  const events = liveEvents()
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Remove feat/search-docs' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Remove' })

  server.use(answeringClean((call) => cleanPlan(call, ['feat/search-docs'])))
  events.changed()

  expect(await within(dialog).findByText('Uncommitted changes')).toBeTruthy()
  const row = within(dialog).getByRole('listitem', { name: 'feat/search-docs' })
  expect(within(row).getByRole('checkbox', { name: 'Remove feat/search-docs' }).getAttribute('aria-checked')).toBe(
    'false',
  )
  expect(within(dialog).queryByRole('button', { name: 'Remove' })).toBeNull()
})

test('the box over the rows never checks an unsafe one: one click forces nothing', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  await within(dialog).findByRole('listitem', { name: 'fix/login' })

  await user.click(within(dialog).getByRole('checkbox', { name: 'Select every worktree shown' }))

  expect(within(dialog).getByText('4 of 5 selected')).toBeTruthy()
  const unsafe = within(dialog).getByRole('listitem', { name: 'fix/login' })
  expect(within(unsafe).getByRole('checkbox', { name: 'Remove fix/login' }).getAttribute('aria-checked')).toBe('false')
  expect(
    within(dialog).getByRole('checkbox', { name: 'Select every worktree shown' }).getAttribute('aria-checked'),
  ).toBe('true')
})

test('the box clears every row it shows, an unsafe one checked anyway included, and says some while only that one is', async () => {
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  const box = () => within(dialog).getByRole('checkbox', { name: 'Select every worktree shown' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Remove fix/login' }))

  expect(box().getAttribute('aria-checked')).toBe('mixed')
  await user.click(box())
  expect(within(dialog).getByText('5 of 5 selected')).toBeTruthy()
  expect(box().getAttribute('aria-checked')).toBe('true')

  await user.click(box())

  expect(within(dialog).getByText('0 of 5 selected')).toBeTruthy()
  expect(within(dialog).getByRole('checkbox', { name: 'Remove fix/login' }).getAttribute('aria-checked')).toBe('false')
  expect(box().getAttribute('aria-checked')).toBe('false')
})

test('a row the person unticked stays unticked when it turns safe', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/search-docs', 'fix/login')
  const box = await within(dialog).findByRole('checkbox', { name: 'Remove fix/login' })
  await user.click(box)
  await user.click(box)
  expect(within(dialog).getByText('1 of 2 selected')).toBeTruthy()

  server.use(answeringClean((call) => cleanPlan(call, [])))
  events.changed()

  await waitFor(() => expect(within(dialog).queryByText('Uncommitted changes')).toBeNull())
  expect(within(dialog).getByText('1 of 2 selected')).toBeTruthy()
  expect(within(dialog).getByRole('checkbox', { name: 'Remove fix/login' }).getAttribute('aria-checked')).toBe('false')
})

test('a row that joins the run once confirmed sends the wizard back to the worktrees', async () => {
  const events = liveEvents()
  const { user, dialog } = await selectAndRemove('feat/search-docs', 'fix/login')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Remove' })

  server.use(answeringClean((call) => cleanPlan(call, [])))
  events.changed()

  const joined = await within(dialog).findByRole('checkbox', { name: 'Remove fix/login' })
  expect(joined.getAttribute('aria-checked')).toBe('true')
  expect(within(dialog).getByText('2 of 2 selected')).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: 'Remove' })).toBeNull()
})

// The plan and the review land apart: a forced plan read first covered a row that had just turned unsafe.
test('no forced removal before the review agrees the person checked each unsafe row under its risk', async () => {
  const events = liveEvents()
  const { user } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Remove fix/login' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Remove feat/search-docs' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  const remove = await within(dialog).findByRole('button', { name: 'Remove 2 worktrees' })
  expect(remove.hasAttribute('disabled')).toBe(false)

  // the review asks every worktree, the plan only the two checked: the review lands last
  server.use(
    answeringClean(async (call) => {
      if ((call.answers?.find((a) => a.path === 'worktree')?.values.length ?? 0) > 2) await delay(1500)
      return cleanPlan(call, ['fix/login', 'feat/search-docs'])
    }),
  )
  events.changed()

  await waitFor(() => expect(within(dialog).getAllByText('Its uncommitted changes are lost.')).toHaveLength(2))
  expect(within(dialog).getByRole('button', { name: 'Remove 2 worktrees' }).hasAttribute('disabled')).toBe(true)
})

const pickFromHeader = async () => {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Remove worktrees' }) }
}

// Sent back to the worktrees on screen only, the wizard jumped to Confirm on the next tick: Remove sat where Next was.
test('sent back to the worktrees, the wizard stays there when the row is checked again, until Next', async () => {
  const events = liveEvents()
  const { user, dialog } = await pickFromHeader()
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Remove feat/search-docs' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Remove' })

  server.use(answeringClean((call) => cleanPlan(call, ['feat/search-docs'])))
  events.changed()
  const row = await within(dialog).findByRole('checkbox', { name: 'Remove feat/search-docs' })
  await user.click(row)

  expect(row.getAttribute('aria-checked')).toBe('true')
  expect(within(dialog).getByRole('button', { name: 'Next' })).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: 'Remove' })).toBeNull()
})

test('sent back to the worktrees, the wizard stays there when the row turns safe again of itself', async () => {
  const events = liveEvents()
  const { user, dialog } = await pickFromHeader()
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Remove feat/search-docs' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Remove' })

  server.use(answeringClean((call) => cleanPlan(call, ['feat/search-docs'])))
  events.changed()
  await within(dialog).findByText('Uncommitted changes')
  server.use(answeringClean((call) => cleanPlan(call, [])))
  events.changed()

  await waitFor(() => expect(within(dialog).queryByText('Uncommitted changes')).toBeNull())
  expect(within(dialog).getByRole('button', { name: 'Next' })).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: 'Remove' })).toBeNull()
})

// A short list had no search: the owner wants it there whatever the number of rows.
test('a picker of two rows has its search all the same', async () => {
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json(
        envelope('list', [listDetail('main', { main: true }), listDetail('feat/a'), listDetail('feat/b')]),
      ),
    ),
  )
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))

  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByText('0 of 2 selected')).toBeTruthy()
  expect(within(dialog).getByRole('textbox', { name: 'Search the worktrees' })).toBeTruthy()
})
