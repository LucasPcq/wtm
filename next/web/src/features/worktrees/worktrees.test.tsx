import { cleanup, screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'

test('each worktree is listed, and its row opens the detail with the problem to fix first and its command', async () => {
  const { user } = renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing-ui/ }))

  const detail = await screen.findByRole('complementary', { name: 'feat/billing-ui' })
  expect(await within(detail).findByText('feat/billing has 1 commit it lacks')).toBeTruthy()
  expect(await within(detail).findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

// Every row's link covered the whole table: a click on any row opened the last one.
test('a row opens its own worktree, not another', async () => {
  const { user, router } = renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing from main/ }))

  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'feat/billing' }))
})

test('Needs sync keeps the worktrees behind their parent, the search narrows the list, and both clear', async () => {
  const { user } = renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /fix\/login/ })

  await user.click(screen.getByRole('radio', { name: /Needs sync/ }))
  expect(screen.queryByRole('link', { name: /fix\/login/ })).toBeNull()
  expect(screen.getByRole('link', { name: /feat\/billing-ui/ })).toBeTruthy()

  await user.type(screen.getByRole('textbox', { name: 'Search branches' }), 'nothing')
  expect(await screen.findByText('No worktree matches')).toBeTruthy()

  await user.click(screen.getByRole('link', { name: 'Clear the search and filters' }))
  expect(await screen.findByRole('link', { name: /fix\/login/ })).toBeTruthy()
  expect((screen.getByRole('textbox', { name: 'Search branches' }) as HTMLInputElement).value).toBe('')
})

test('create shows a problem only once asked to go on, then confirms the plan, runs it, and ends on where the worktree is and what next', async () => {
  const { user } = renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  await within(dialog).findByRole('textbox', { name: 'Branches' })
  expect(within(dialog).queryByText('branches is required')).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByText('branches is required')).toBeTruthy()

  await user.type(within(dialog).getByRole('textbox', { name: 'Branches' }), 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  // the web's own words, not the command line's; a question only a flag sets is not said to be skipped
  expect(await within(dialog).findByText('Env files')).toBeTruthy()
  expect(within(dialog).getByText('Project default')).toBeTruthy()
  expect(within(dialog).queryByText(/Not asked/)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('feat/new is ready')).toBeTruthy()
  expect(within(dialog).getByText('/acme.trees/feat/new')).toBeTruthy()
  expect(within(dialog).queryByText('Run the on_create hooks')).toBeNull()
  expect(within(dialog).getByText('wtm go feat/new')).toBeTruthy()
})

// A branch the core lists with no label of its own was a blank entry of the source branch.
test('an option of no label is offered by its value', async () => {
  const { user } = renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')

  await user.click(await within(dialog).findByRole('combobox', { name: 'Source branch' }))

  expect(await screen.findByRole('option', { name: 'feat/billing' })).toBeTruthy()
})

test("a node's action the web does not run yet shows its command line to copy", async () => {
  const { user } = renderApp('/r/r1/worktrees?view=tree')

  await user.click(await screen.findByRole('button', { name: 'Sync' }))

  expect(await screen.findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

test('a column hidden in the Columns panel leaves the list, and stays hidden on the next visit', async () => {
  const first = renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })
  expect(screen.getByRole('columnheader', { name: 'Running' })).toBeTruthy()

  await first.user.click(screen.getByRole('button', { name: 'Columns' }))
  await first.user.click(await screen.findByRole('checkbox', { name: /^Running/ }))
  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()

  cleanup()
  renderApp('/r/r1/worktrees')
  await screen.findByRole('link', { name: /feat\/billing-ui/ })
  expect(screen.queryByRole('columnheader', { name: 'Running' })).toBeNull()
  expect(screen.getByRole('columnheader', { name: 'State' })).toBeTruthy()
})
