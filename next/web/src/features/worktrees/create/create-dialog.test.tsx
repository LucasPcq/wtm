import { screen, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { questionsOf, server, streamed } from '@/test/server'

async function openCreate() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  return { ...app, dialog, branches: await within(dialog).findByRole('textbox', { name: 'Branches' }) }
}

test('create shows a problem only once asked to go on, then confirms the plan, runs it, and ends on where the worktree is and what next', async () => {
  const { user, dialog, branches } = await openCreate()
  expect(within(dialog).queryByText('branches is required')).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByText('branches is required')).toBeTruthy()

  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  // the web's own words, not the command line's; a question only a flag sets is not said to be skipped
  expect(await within(dialog).findByText('Env files')).toBeTruthy()
  expect(within(dialog).getByText('Project default')).toBeTruthy()
  expect(within(dialog).queryByText(/Not asked/)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  // the confirmation reads the plan, not the command line's recap
  const planned = await within(dialog).findByRole('region', { name: 'feat/new' })
  expect(within(dialog).getByRole('heading', { name: 'Create feat/new' })).toBeTruthy()
  expect(within(planned).getByText('Copied from .env.example.')).toBeTruthy()
  expect(within(planned).getByText('Add the worktree · Run the on_create hooks')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('feat/new is ready')).toBeTruthy()
  expect(within(dialog).getByText('/acme.trees/feat/new')).toBeTruthy()
  expect(within(dialog).getByText('wtm go feat/new')).toBeTruthy()
  // a run that went well folds its steps away, one click from them
  expect(within(dialog).queryByText('Run the on_create hooks')).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: /2 steps/ }))
  expect(within(dialog).getByText('Run the on_create hooks')).toBeTruthy()
})

// A branch the core lists with no label of its own was a blank entry of the source branch.
test('an option of no label is offered by its value', async () => {
  const { user, dialog } = await openCreate()

  await user.click(await within(dialog).findByRole('combobox', { name: 'Source branch' }))

  expect(await screen.findByRole('option', { name: 'feat/billing' })).toBeTruthy()
})

// Next went by the steps of the last answer, older than the typing: a step the core no longer asks was shown empty.
test('Next passes over a step the core asks nothing of once the branch is typed, without waiting for a pause', async () => {
  const { user, dialog, branches } = await openCreate()

  await user.type(branches, 'feat/billing')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByRole('heading', { name: 'Create feat/billing' })).toBeTruthy()
  expect(within(dialog).getByText('Environment · not asked')).toBeTruthy()
})

// A follow-up the command line cannot spell was left out of the done state: it is said, never hidden.
test('a next command the server cannot spell is said with what it meant, not hidden', async () => {
  server.use(http.post('/api/command-line', () => HttpResponse.json({ argv: [] })))
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('No command line for this: jump in')).toBeTruthy()
})

// Cancelled while it planned, a create ends on the core's cancelled error and no item: it read as a failure.
test('a create cancelled before it made anything says it was cancelled, not that it failed', async () => {
  const cancelled = {
    ...envelope('create', []),
    ok: false,
    error: { kind: 'cancelled', code: 'cancelled', message: 'cancelled' },
  }
  server.use(
    http.post('/api/repos/r1/commands/create', async ({ request }) => {
      const call = (await request.clone().json()) as { dry_run?: boolean }
      if (call.dry_run) return undefined
      return streamed(`event: envelope\ndata: ${JSON.stringify(cancelled)}\n\n`)
    }),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create worktree' }))

  expect(await within(dialog).findByText('Cancelled feat/new')).toBeTruthy()
  expect(within(dialog).queryByText(/Couldn't create/)).toBeNull()
})

// Each pause in typing asks the core again: a slow answer to older typing overwrote a newer one.
test('an answer to older typing that lands last is dropped, the newer one stands', async () => {
  server.use(
    http.post('/api/repos/r1/commands/create/questions', async ({ request }) => {
      const answers = ((await request.json()) as { answers: { path: string; values: string[] }[] }).answers
      const typed = answers.find((answer) => answer.path === 'branches')?.values[0] ?? ''
      if (typed !== 'old') return HttpResponse.json(questionsOf(answers))
      await delay(700)
      return HttpResponse.json({
        ...questionsOf(answers),
        problems: [{ path: 'branches', code: 'stale', message: 'an older answer' }],
      })
    }),
  )
  const { user, dialog, branches } = await openCreate()

  await user.type(branches, 'old')
  await delay(400)
  await user.type(branches, 'er')
  await delay(900)

  expect(within(dialog).queryByText('an older answer')).toBeNull()
})
