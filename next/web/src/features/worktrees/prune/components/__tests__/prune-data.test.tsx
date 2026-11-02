import { within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { candidates, merged, prunePlan } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'

test('a service down that holds their data asks whether to start it to drop the data now, and the run says so', async () => {
  const prune = answeringPrune((call) => {
    const start = call.answers?.some((a) => a.path === 'drop_data') ?? false
    const data = [{ job: 'db', name: 'acme_feat_billing', status: start ? ('start' as const) : ('defer' as const) }]
    return prunePlan(call, { found: [{ ...merged, data }, candidates[1] ?? merged] })
  })
  const { user, dialog } = await openFromTheHeader()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  expect(
    await within(dialog).findByText("db is down: the data these worktrees made there can't be dropped yet."),
  ).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: /^Start db and drop it now/ }))
  await user.click(within(dialog).getByRole('button', { name: /^Details/ }))
  expect(
    await within(dialog).findByText('acme_feat_billing in db is dropped: db is started for it, then stopped'),
  ).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Prune' }))

  await within(dialog).findByText(/^1 pruned\./)
  expect(prune.answered(prune.applied(), 'drop_data')).toEqual(['true'])
})
