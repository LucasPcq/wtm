import { waitFor, within } from '@testing-library/react'
import { HttpResponse } from 'msw'
import { expect, test } from 'vitest'
import { envelope } from '@/test/fixtures'
import { candidates, merged, prunePlan } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'
import { streamed } from '@/test/stream'

test('a service down that holds their data asks, on a Data step of its own, whether to drop it now; the plan and the run take the answer', async () => {
  const prune = answeringPrune((call) => {
    const start = call.answers?.some((a) => a.path === 'drop_data') ?? false
    const data = [{ job: 'db', name: 'acme_feat_billing', status: start ? ('start' as const) : ('defer' as const) }]
    return prunePlan(call, { found: [{ ...merged, data }, candidates[1] ?? merged] })
  })
  const { user, dialog } = await openFromTheHeader()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const steps = within(dialog).getByRole('list', { name: 'Steps' })
  expect(within(steps).getByText('Data').closest('li')?.getAttribute('aria-current')).toBe('step')
  expect(
    await within(dialog).findByText("db is down: the data these worktrees made there can't be dropped yet."),
  ).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: /^Start db and drop it now/ }))
  await waitFor(() =>
    expect(prune.answered(prune.calls.filter((call) => call.dry_run).at(-1), 'drop_data')).toEqual(['true']),
  )
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(within(dialog).queryByRole('radio')).toBeNull()
  await user.click(await within(dialog).findByRole('button', { name: /^Details/ }))
  expect(
    await within(dialog).findByText('acme_feat_billing in db is dropped: db is started for it, then stopped'),
  ).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Prune' }))

  await within(dialog).findByText(/^1 pruned\./)
  expect(prune.answered(prune.applied(), 'drop_data')).toEqual(['true'])
})

const owed = [
  { job: 'db', name: 'acme_feat_old', worktree: 'feat-old' },
  { job: 'db', name: 'acme_feat_older', worktree: 'feat-older' },
]

const debt = (name: string, worktree: string) => ({ job: 'db', name, worktree })

test("drops owed, the plan's only change, go straight to Confirm, and the run says what was dropped and what failed", async () => {
  const prune = answeringPrune(
    async (call) => {
      const answer = await prunePlan(call, { found: [] }).json()
      return HttpResponse.json({ ...answer, plan: { ...answer.plan, owed, empty: false } })
    },
    () =>
      streamed(
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('prune', []),
          ok: false,
          items: [
            { subject: 'acme_feat_old', status: 'done', detail: { owed: debt('acme_feat_old', 'feat-old') } },
            {
              subject: 'acme_feat_older',
              status: 'failed',
              error: {
                kind: 'internal',
                code: 'prune.drop_failed',
                message: "couldn't drop acme_feat_older from db, still owed: exit status 1",
              },
              detail: { owed: debt('acme_feat_older', 'feat-older') },
            },
          ],
          plan: { worktrees: [], owed, reasons: ['merged', 'closed', 'gone'], reparent_children: false, force: false },
        })}\n\n`,
      ),
  )
  const { user, dialog } = await openFromTheHeader()

  const list = await within(dialog).findByRole('region', { name: 'Owed' })
  expect(within(list).getByRole('listitem', { name: 'acme_feat_old' })).toBeTruthy()
  expect(within(dialog).queryByText('Nothing to prune')).toBeNull()
  await user.click(await within(dialog).findByRole('button', { name: 'Drop 2 owed namespaces' }))

  expect(await within(dialog).findByText('1 owed drop made · 1 owed drop failed.')).toBeTruthy()
  const done = within(dialog).getByRole('region', { name: 'Pruned' })
  expect(within(done).getByText('Dropped from db, owed since feat-old was removed')).toBeTruthy()
  expect(within(done).queryByText(/acme_feat_older/)).toBeNull()
  const failed = within(dialog).getByRole('region', { name: "Couldn't prune" })
  expect(within(failed).getByText(/couldn't drop acme_feat_older from db, still owed/i)).toBeTruthy()
  const applied = prune.applied()
  expect(prune.answered(applied, 'worktree'), 'prune names none: its own pick').toBeUndefined()
  expect(prune.answered(applied, 'force')).toBeUndefined()
})

test('the button says the plan it applies: owed-only rows, then a plan that also prunes a worktree, prune', async () => {
  answeringPrune(async (call) => {
    const forced = call.answers?.some((a) => a.path === 'force') ?? false
    const answer = await prunePlan(call, { found: forced ? [] : [merged] }).json()
    return HttpResponse.json({ ...answer, plan: { ...answer.plan, owed, empty: false } })
  })
  const { dialog } = await openFromTheHeader()

  expect(await within(dialog).findByRole('button', { name: 'Prune' })).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: /^Drop/ })).toBeNull()
})
