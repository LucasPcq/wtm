import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { heldRun } from '@/test/clean'
import { advanceOf, ffPlan, ffRun, ffUnsafe } from '@/test/fast-forward'
import { answering, fromMenu } from '@/test/fast-forward-app'
import { envelope, followUp, statusDetail } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// feat/billing-ui's status: origin has commits it lacks, its fix a fast-forward
const behindOrigin = http.post('/api/repos/r1/commands/status', () =>
  HttpResponse.json(
    envelope('status', [
      statusDetail('feat/billing-ui', {
        problems: [
          {
            code: 'status.behind_origin',
            message: 'origin has 2 commits it lacks',
            fix: followUp('fast-forward', 'feat/billing-ui', { code: 'status.fast_forward', message: 'take them' }),
          },
        ],
      }),
    ]),
  ),
)

test('one clean worktree from its menu runs at once and ends on a toast, no dialog', async () => {
  await fromMenu('feat/billing-ui')

  expect((await screen.findAllByText("feat/billing-ui is on origin's tip")).length).toBeGreaterThan(0)
  expect(screen.getAllByText('2 commits taken from origin').length).toBeGreaterThan(0)
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('while it runs the row is under way, its menu disabled, and a toast says so', async () => {
  const held = heldRun('fast_forward')
  server.use(answering(held.respond))
  await fromMenu('feat/billing-ui')

  expect(await screen.findByText('Fast-forwarding feat/billing-ui')).toBeTruthy()
  const row = await screen.findByRole('row', { name: /feat\/billing-ui/ })
  expect(within(row).getByText('Fast-forwarding')).toBeTruthy()
  expect(within(row).getByRole('button', { name: 'Actions on feat/billing-ui' }).hasAttribute('disabled')).toBe(true)

  held.end({
    ...envelope('fast-forward', []),
    items: [{ subject: 'feat/billing-ui', status: 'done', detail: { path: '/acme.trees/feat-billing-ui' } }],
    plan: { worktrees: [advanceOf('feat/billing-ui', false)], force: false },
  })
  expect((await screen.findAllByText("feat/billing-ui is on origin's tip")).length).toBeGreaterThan(0)
})

test('a failure ends on an error toast whose Details opens the outcome', async () => {
  server.use(answering((call) => ffRun(call, ['feat/billing-ui'])))
  const { user } = await fromMenu('feat/billing-ui')

  expect((await screen.findAllByText("Couldn't fast-forward feat/billing-ui")).length).toBeGreaterThan(0)
  await user.click(screen.getAllByRole('button', { name: 'Details' })[0] as HTMLElement)

  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText('0 of 1 fast-forwarded.')).toBeTruthy()
  expect(within(dialog).getByText('git refused to move the branch')).toBeTruthy()
})

test('a worktree with a change opens the dialog on Confirm instead, and nothing runs', async () => {
  const runs: WebCall[] = []
  server.use(
    answering((call) => {
      runs.push(call)
      return ffRun(call)
    }),
  )
  await fromMenu('fix/login')

  const dialog = await screen.findByRole('dialog')
  const box = await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' })
  expect(box.getAttribute('data-state')).toBe('unchecked')
  expect(runs).toEqual([])
})

test('a plan with a warning opens the dialog rather than run past it', async () => {
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) return ffRun(call)
      const planned = await ffPlan(call).json()
      const warning = { code: 'fast-forward.fetch_failed', message: "couldn't fetch origin: could not resolve host" }
      return HttpResponse.json({ ...planned, warnings: [warning] })
    }),
  )
  await fromMenu('feat/billing-ui')

  const dialog = await screen.findByRole('dialog')
  expect((await within(dialog).findByRole('alert')).textContent).toContain("Couldn't fetch origin")
})

test("the detail callout's Fast-forward runs at once", async () => {
  server.use(behindOrigin)
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling-ui')

  const callout = await screen.findByRole('region', { name: 'To do' })
  await user.click(within(callout).getByRole('button', { name: 'Fast-forward' }))

  // Sonner draws the loading toast and its successor side by side while it swaps them
  expect((await screen.findAllByText("feat/billing-ui is on origin's tip")).length).toBeGreaterThan(0)
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

// A callout of one line drew an empty row under it: its fix and its consequence, both absent, still took their place.
test('a callout whose action sits on its line has nothing under it', async () => {
  server.use(behindOrigin)
  await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling-ui')

  const callout = await screen.findByRole('region', { name: 'To do' })

  expect([...callout.children].every((child) => child.tagName === 'svg' || child.textContent !== '')).toBe(true)
})

test('a second click while the first runs opens it rather than run again', async () => {
  const runs: WebCall[] = []
  let release = () => {}
  const planned = new Promise<void>((resolve) => {
    release = resolve
  })
  server.use(
    behindOrigin,
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) {
        runs.push(call)
        return ffRun(call)
      }
      await planned
      return ffPlan(call)
    }),
  )
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling-ui')

  const button = within(await screen.findByRole('region', { name: 'To do' })).getByRole('button', {
    name: 'Fast-forward',
  })
  await user.click(button)
  await user.click(button)
  release()

  expect(await within(await screen.findByRole('dialog')).findByText('1 fast-forwarded.')).toBeTruthy()
  expect(runs).toHaveLength(1)
})

test('no request without force ever names a worktree behind origin with changes', async () => {
  const calls: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      calls.push(call)
      return call.dry_run ? ffPlan(call) : ffRun(call)
    }),
  )
  const { user } = await fromMenu('fix/login')

  const dialog = await screen.findByRole('dialog')
  await within(dialog).findByRole('checkbox', { name: 'Fast-forward fix/login' })
  await user.click(within(dialog).getByRole('checkbox', { name: 'Fast-forward fix/login' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Fast-forward' }))
  await within(dialog).findByText('1 fast-forwarded.')

  const names = (call: WebCall) => call.answers?.find((a) => a.path === 'worktree')?.values ?? []
  const forced = (call: WebCall) => call.answers?.some((a) => a.path === 'force') ?? false
  expect(calls.length).toBeGreaterThan(0)
  expect(calls.filter((call) => names(call).includes('fix/login') && !forced(call))).toEqual([])
})

test('a run whose plan went stale ends on an error toast that says so', async () => {
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) return ffRun(call)
      return HttpResponse.json({ ...(await ffPlan(call).json()), fingerprint: 'old' })
    }),
  )
  await fromMenu('feat/billing-ui')

  expect((await screen.findAllByText("Couldn't fast-forward feat/billing-ui")).length).toBeGreaterThan(0)
  expect(screen.getAllByText(/the facts changed since the preview/i).length).toBeGreaterThan(0)
})

test('a cancelled run says it was cancelled, not that it failed', async () => {
  const held = heldRun('fast_forward')
  let applying = false
  server.use(
    answering((call) => {
      applying = true
      return held.respond(call)
    }),
  )
  await fromMenu('feat/billing-ui')
  await waitFor(() => expect(applying).toBe(true))

  held.end({
    ...envelope('fast-forward', []),
    ok: false,
    items: [{ subject: 'feat/billing-ui', status: 'skipped', reason: 'interrupted', detail: {} }],
    plan: { worktrees: [advanceOf('feat/billing-ui', false)], force: false },
    error: { kind: 'cancelled', code: 'cancelled', message: 'interrupted' },
  })

  expect((await screen.findAllByText('Fast-forward of feat/billing-ui cancelled')).length).toBeGreaterThan(0)
  expect(screen.queryByText("Couldn't fast-forward feat/billing-ui")).toBeNull()
})

test('while the dialog shows the run, its end makes no toast of the same outcome', async () => {
  const held = heldRun('fast_forward')
  server.use(answering(held.respond))
  const { user } = await fromMenu('feat/billing-ui')
  await screen.findAllByText('Fast-forwarding feat/billing-ui')
  await user.click(await screen.findByRole('button', { name: 'Actions on main' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))
  const dialog = await screen.findByRole('dialog')

  held.end({
    ...envelope('fast-forward', []),
    items: [{ subject: 'feat/billing-ui', status: 'done', detail: { path: '/acme.trees/feat-billing-ui' } }],
    plan: { worktrees: [advanceOf('feat/billing-ui', false)], force: false },
  })

  expect(await within(dialog).findByText('1 fast-forwarded.')).toBeTruthy()
  expect(screen.queryByText("feat/billing-ui is on origin's tip")).toBeNull()
})

test('an apply refused because the worktree changed opens, from its Details, on that worktree with the notice', async () => {
  server.use(answering(() => ffUnsafe(['feat/billing-ui'])))
  const { user } = await fromMenu('feat/billing-ui')
  expect((await screen.findAllByText("Couldn't fast-forward feat/billing-ui")).length).toBeGreaterThan(0)

  await user.click(screen.getAllByRole('button', { name: 'Details' })[0] as HTMLElement)

  const dialog = await screen.findByRole('dialog')
  expect((await within(dialog).findByRole('alert')).textContent).toMatch(/^feat\/billing-ui changed since the plan/)
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing-ui' })).toBeTruthy()
})
