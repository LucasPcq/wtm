import { screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { heldRun } from '@/test/clean'
import { advanceOf, ffPlan, ffRun, ffUnsafe } from '@/test/fast-forward'
import { answering, fromMenu } from '@/test/fast-forward-app'
import { envelope } from '@/test/fixtures'
import { server } from '@/test/server'

test('a second row asked while the first one plans opens the run under way, and starts none', async () => {
  const runs: WebCall[] = []
  let release = () => {}
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) {
        runs.push(call)
        return ffRun(call)
      }
      await new Promise<void>((resolve) => (release = resolve))
      return ffPlan(call)
    }),
  )
  const { user } = await fromMenu('main')

  const row = await screen.findByRole('row', { name: /^main/ })
  expect(within(row).getByText('Fast-forwarding')).toBeTruthy()
  await user.click(await screen.findByRole('button', { name: 'Actions on feat/billing-ui' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))

  expect(await screen.findByRole('dialog')).toBeTruthy()
  release()
  await waitFor(() => expect(runs).toHaveLength(1))
  expect(runs[0]?.answers?.find((a) => a.path === 'worktree')?.values).toEqual(['main'])
})

test('a failure nobody has read yet is shown, not replaced, by the next row asked', async () => {
  const runs: WebCall[] = []
  server.use(
    answering((call) => {
      runs.push(call)
      return ffRun(call, ['feat/billing-ui'])
    }),
  )
  const { user } = await fromMenu('feat/billing-ui')
  expect((await screen.findAllByText("Couldn't fast-forward feat/billing-ui")).length).toBeGreaterThan(0)

  await user.click(await screen.findByRole('button', { name: 'Actions on main' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))

  const dialog = await screen.findByRole('dialog')
  expect(await within(dialog).findByText('0 of 1 fast-forwarded.')).toBeTruthy()
  expect(within(dialog).getByText('git refused to move the branch')).toBeTruthy()
  expect(runs).toHaveLength(1)
})

test('a cancelled run, already told, does not stop the next row from running at once', async () => {
  const held = heldRun('fast_forward')
  let applying = false
  server.use(
    answering((call) => {
      if (applying) return ffRun(call)
      applying = true
      return held.respond(call)
    }),
  )
  const { user } = await fromMenu('feat/billing-ui')
  await waitFor(() => expect(applying).toBe(true))
  held.end({
    ...envelope('fast-forward', []),
    ok: false,
    items: [{ subject: 'feat/billing-ui', status: 'skipped', reason: 'interrupted', detail: {} }],
    plan: { worktrees: [advanceOf('feat/billing-ui', false)], force: false },
    error: { kind: 'cancelled', code: 'cancelled', message: 'interrupted' },
  })
  await screen.findAllByText('Fast-forward of feat/billing-ui cancelled')

  await user.click(await screen.findByRole('button', { name: 'Actions on main' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))

  expect((await screen.findAllByText("main is on origin's tip")).length).toBeGreaterThan(0)
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('a refusal not yet read opens on its own worktree when another row is asked', async () => {
  server.use(answering(() => ffUnsafe(['feat/billing-ui'])))
  const { user } = await fromMenu('feat/billing-ui')
  await screen.findAllByText("Couldn't fast-forward feat/billing-ui")

  await user.click(await screen.findByRole('button', { name: 'Actions on main' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))

  const dialog = await screen.findByRole('dialog')
  expect((await within(dialog).findByRole('alert')).textContent).toMatch(/^feat\/billing-ui changed since the plan/)
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing-ui' })).toBeTruthy()
  expect(within(dialog).queryByRole('listitem', { name: 'main' })).toBeNull()
})
