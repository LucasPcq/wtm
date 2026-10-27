import { screen, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { line, questionsOf, server, streamed } from '@/test/server'

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
  expect(await within(dialog).findByText('Env files')).toBeTruthy()
  expect(within(dialog).getByText('Project default')).toBeTruthy()
  expect(within(dialog).queryByText(/Not asked/)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  const planned = await within(dialog).findByRole('listitem', { name: 'feat/new' })
  expect(within(planned).getByText(/\.env from \.env\.example/)).toBeTruthy()
  expect(within(dialog).queryByText('Add the worktree, run the on_create hooks')).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: /^Details/ }))
  expect(within(dialog).getByText('Add the worktree, run the on_create hooks')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Create' }))

  expect(await within(dialog).findByText('feat/new is ready.')).toBeTruthy()
  expect(within(dialog).getByRole('heading', { name: 'Create worktrees' })).toBeTruthy()
  expect(within(dialog).queryByRole('heading', { name: 'Ready' })).toBeNull()
  expect(within(dialog).queryByText('/acme.trees/feat/new')).toBeNull()
  expect(within(dialog).queryByRole('button', { name: 'Copy wtm go feat/new' })).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Details of feat/new' }))
  expect(within(dialog).getByText('/acme.trees/feat/new')).toBeTruthy()
  expect(within(dialog).getByText('wtm go feat/new')).toBeTruthy()
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

  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing' })).toBeTruthy()
  expect(within(dialog).getByText('Environment · not asked')).toBeTruthy()
})

// A follow-up the command line cannot spell was left out of the done state: it is said, never hidden.
test('a next command the server cannot spell is said with what it meant, not hidden', async () => {
  server.use(http.post('/api/command-line', () => HttpResponse.json({ argv: [] })))
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create' }))

  await user.click(await within(dialog).findByRole('button', { name: 'Details of feat/new' }))
  expect(await within(dialog).findByText('No command line for this: jump in')).toBeTruthy()
})

// Cancelled while it planned, a create ends on the core's cancelled error and no item: it read as a failure.
test('a create cancelled before it made anything says it was cancelled, not that it failed, and opens nothing', async () => {
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
  const { user, router, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create' }))

  expect(await within(dialog).findByText(/^0 of \d+ steps done\. What is done stays/)).toBeTruthy()
  expect(within(dialog).queryByText(/Couldn't create/)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: 'Done' }))
  expect(router.state.location.search).toEqual({})
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

test('a create hidden while it runs says it is ready in a toast once it ends', async () => {
  let end = () => {}
  server.use(
    http.post('/api/repos/r1/commands/create', async ({ request }) => {
      if (((await request.clone().json()) as WebCall).dry_run) return undefined
      const stream = new ReadableStream({
        start: (controller) => {
          controller.enqueue(new TextEncoder().encode(line({ type: 'unit.started', subject: 'feat/new' })))
          end = () => {
            const ended = { ...envelope('create', []), items: [{ subject: 'feat/new', status: 'done', detail: {} }] }
            controller.enqueue(new TextEncoder().encode(`event: envelope\ndata: ${JSON.stringify(ended)}\n\n`))
            controller.close()
          }
        },
      })
      return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } })
    }),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))

  end()

  expect(await screen.findByText('feat/new is ready.')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('a warning is drawn where its subject is: a diverged source on the rows that start from it, one about no branch in the Alert', async () => {
  const diverged = 'branch main has diverged from origin (1 ahead, 2 behind)'
  const fallback = 'the "parent" env strategy needs the source branch checked out'
  server.use(
    http.post('/api/repos/r1/commands/create', async ({ request }) => {
      const call = (await request.clone().json()) as { dry_run?: boolean }
      if (!call.dry_run) return undefined
      return HttpResponse.json({
        ...envelope('create', []),
        fingerprint: 'f1',
        plan: {
          branches: [{ branch: 'feat/new', path: '/acme.trees/feat/new', action: 'create', phases: ['worktree'] }],
          creates: 1,
          from: 'main',
          env_from: 'example',
          recap: [],
        },
        warnings: [
          { code: 'create.source_diverged', message: diverged, params: { branch: 'main', ahead: '1', behind: '2' } },
          { code: 'create.env_parent_fallback', message: fallback },
        ],
      })
    }),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const row = await within(dialog).findByRole('listitem', { name: 'feat/new' })
  expect(within(row).getByText(/has diverged from origin/)).toBeTruthy()
  const alert = within(dialog).getByRole('alert')
  expect(alert.textContent).toContain('needs the source branch checked out')
  expect(alert.textContent).not.toContain('diverged')
})

// With no source branch, a plan-wide warning matched every created row (undefined === undefined) and its Alert vanished.
test('with no source branch, a plan-wide warning stays the Alert, and the row says where its .env really comes from', async () => {
  server.use(
    http.post('/api/repos/r1/commands/create', async ({ request }) => {
      const call = (await request.clone().json()) as { dry_run?: boolean }
      if (!call.dry_run) return undefined
      return HttpResponse.json({
        ...envelope('create', []),
        fingerprint: 'f1',
        plan: {
          branches: [{ branch: 'feat/new', path: '/acme.trees/feat/new', action: 'create', phases: ['worktree'] }],
          creates: 1,
          env_from: 'parent',
          recap: [],
        },
        warnings: [{ code: 'create.env_parent_fallback', message: 'without a worktree it comes from main' }],
      })
    }),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const row = await within(dialog).findByRole('listitem', { name: 'feat/new' })
  expect(within(dialog).getByRole('alert').textContent).toContain('a worktree it comes from main')
  expect(row.textContent).not.toContain('comes from main')
  expect(row.textContent).toContain('.env from the main checkout')
})

test("Create's Done keeps a warning of the whole run as its Alert, and one about a branch under To check", async () => {
  server.use(
    http.post('/api/repos/r1/commands/create', async ({ request }) => {
      const call = (await request.clone().json()) as { dry_run?: boolean }
      if (call.dry_run) return undefined
      return streamed(
        [
          line({ type: 'unit.started', subject: 'feat/new' }),
          line({ type: 'unit.finished', subject: 'feat/new', status: 'done' }),
          `event: envelope\ndata: ${JSON.stringify({
            ...envelope('create', []),
            items: [{ subject: 'feat/new', status: 'done', detail: { path: '/acme.trees/feat/new' } }],
            warnings: [
              { code: 'locks.unavailable', message: 'ran without operation locks' },
              {
                code: 'create.source_diverged',
                message: 'branch main has diverged from origin',
                params: { branch: 'main' },
              },
            ],
          })}\n\n`,
        ].join(''),
      )
    }),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Create' }))

  const toCheck = await within(dialog).findByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText(/has diverged from origin/)).toBeTruthy()
  expect(within(toCheck).queryByText(/without operation locks/)).toBeNull()
  expect(within(dialog).getByRole('alert').textContent).toContain('without operation locks')
})
