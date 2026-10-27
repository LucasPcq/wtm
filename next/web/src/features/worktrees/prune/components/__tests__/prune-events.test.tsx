import { screen, waitFor, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { PruneCandidate, WebCall } from '@/api/types.gen'
import { envelope, followUp, statusDetail } from '@/test/fixtures'
import { candidates, gone, merged, prunePlan } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'
import { liveEvents } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

const rowsCall = (call: WebCall) =>
  call.answers?.some((answer) => answer.path === 'no_fetch') &&
  !call.answers.some((answer) => answer.path === 'worktree')

const spike: PruneCandidate = {
  ...merged,
  worktree: 'spike/gui-poc',
  branch: 'spike/gui-poc',
  path: '/acme.trees/spike-gui-poc',
  pr: { number: 218, url: 'https://github.com/acme/acme/pull/218' },
}

// An event asked the plan again on its old answers before the rows changed them: a worktree that turned unsafe was
// named without force, and the core refused it.
test('an event never sends a worktree that turned unsafe, named without force', async () => {
  const events = liveEvents()
  let dirty = false
  const refusals: number[] = []
  answeringPrune(async (call) => {
    const billing = dirty ? { ...merged, blockers: ['dirty' as const] } : merged
    const answer = prunePlan(call, { found: [billing, spike, gone] })
    if (answer.status === 403) refusals.push(answer.status)
    return answer
  })
  const { user, dialog } = await openFromTheHeader()
  await user.click(
    within(await within(dialog).findByRole('listitem', { name: 'spike/gui-poc' })).getByRole('checkbox', {
      name: 'Prune spike/gui-poc',
    }),
  )
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Prune' })

  dirty = true
  events.changed()

  await waitFor(() => expect(within(dialog).getByText('Uncommitted changes')).toBeTruthy())
  expect(refusals).toEqual([])
  expect(within(dialog).queryByText(/can't be removed safely/)).toBeNull()
})

// Opened on a worktree that then stopped being done with, the rows named it and the core refused them all.
test('opened from the callout, a worktree no longer done with is said, not a refusal of the whole dialog', async () => {
  server.use(
    http.post('/api/repos/r1/commands/status', () =>
      HttpResponse.json(
        envelope('status', [
          statusDetail('feat/billing', {
            problems: [
              {
                code: 'status.merged',
                message: 'its pull request is merged',
                fix: followUp('prune', 'feat/billing', { code: 'status.merged', message: 'prune it' }),
              },
            ],
          }),
        ]),
      ),
    ),
  )
  const events = liveEvents()
  let moved = false
  answeringPrune((call) =>
    prunePlan(call, { found: moved && rowsCall(call) ? candidates.filter((c) => c !== merged) : candidates }),
  )
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')
  await user.click(within(await screen.findByRole('region', { name: 'To do' })).getByRole('button', { name: 'Prune' }))
  const dialog = await screen.findByRole('dialog', { name: 'Prune worktrees' })
  await within(dialog).findByText('1 of 1 selected')

  moved = true
  events.changed()

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  await waitFor(() => expect(within(left).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy())
  expect(within(dialog).queryByText(/not done with/)).toBeNull()
})

// One refetch of the rows that failed replaced the whole wizard by its error.
test('a refetch of the rows that fails keeps the wizard as it was read', async () => {
  const events = liveEvents()
  let broken = false
  let refetched = 0
  answeringPrune((call) => {
    if (broken && rowsCall(call)) {
      refetched += 1
      return HttpResponse.json({ error: { message: 'the server fell over' } }, { status: 500 })
    }
    return prunePlan(call)
  })
  const { dialog } = await openFromTheHeader()
  await within(dialog).findByRole('listitem', { name: 'feat/billing' })

  broken = true
  events.changed()

  await waitFor(() => expect(refetched).toBe(1))
  expect(within(dialog).queryByText('the server fell over')).toBeNull()
  expect(within(dialog).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy()
})

// While the rows were read again, a worktree said left out disappeared from Left as they are, then came back.
test('a worktree said left out stays said while the rows are read again', async () => {
  const events = liveEvents()
  let moved = false
  answeringPrune(async (call) => {
    if (moved && rowsCall(call)) await delay(150)
    return prunePlan(call, { found: moved && rowsCall(call) ? candidates.filter((c) => c !== merged) : candidates })
  })
  const { user, dialog } = await openFromTheHeader()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Prune' })
  moved = true
  events.changed()
  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  await waitFor(() => expect(within(left).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy())

  let lost = false
  const watch = new MutationObserver(() => {
    if (!within(dialog).queryByRole('listitem', { name: 'feat/billing' })) lost = true
  })
  watch.observe(dialog, { subtree: true, childList: true, characterData: true })
  events.changed()
  await delay(400)
  watch.disconnect()

  expect(lost).toBe(false)
})
