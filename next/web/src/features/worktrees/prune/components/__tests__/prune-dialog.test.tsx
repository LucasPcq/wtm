import { screen, waitFor, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope, followUp, statusDetail } from '@/test/fixtures'
import { candidates, merged, prunePlan } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'
import { liveEvents } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

test("from the header, the unsafe candidate starts unchecked; prune's own pick keeps it, said where it is and after", async () => {
  const prune = answeringPrune()
  const { user, dialog } = await openFromTheHeader()

  const merged = await within(dialog).findByRole('listitem', { name: 'feat/billing' })
  expect(within(merged).getByText('PR #214 merged')).toBeTruthy()
  expect(within(merged).getByRole('link', { name: 'Open pull request #214' }).getAttribute('href')).toBe(
    'https://github.com/acme/acme/pull/214',
  )
  const gone = within(dialog).getByRole('listitem', { name: 'feat/search-docs' })
  expect(within(gone).getByText('Deleted on origin')).toBeTruthy()
  expect(within(gone).getByText('1 commit on no other branch')).toBeTruthy()
  expect(within(gone).getByRole('checkbox', { name: 'Prune feat/search-docs' }).getAttribute('aria-checked')).toBe(
    'false',
  )
  expect(within(dialog).getByText('1 of 2 selected')).toBeTruthy()
  expect(prune.answered(prune.calls[0], 'force'), 'the review lists every candidate').toEqual(['true'])
  expect(prune.answered(prune.calls[0], 'no_fetch'), 'the review fetches origin').toBeUndefined()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('radio', { name: 'Move it onto main' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('Kept: 1 commit on no other branch')).toBeTruthy()
  expect(within(dialog).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Prune' }))

  expect(await within(dialog).findByText(/^1 pruned\./)).toBeTruthy()
  const applied = prune.applied()
  expect(prune.answered(applied, 'worktree'), "prune's own pick names none").toBeUndefined()
  expect(prune.answered(applied, 'force')).toBeUndefined()
  expect(prune.answered(applied, 'no_fetch')).toEqual(['true'])
  expect(
    within(dialog).getByText('feat/search-docs is kept: unsafe to remove (unpushed)'),
    'a kept one is to check, with what prunes it anyway',
  ).toBeTruthy()
  expect(
    await within(dialog).findByRole('button', { name: 'Copy wtm prune feat/search-docs --force true' }),
  ).toBeTruthy()
})

test('checking the unsafe candidate turns its reason into the loss, and the run names both, forced', async () => {
  const prune = answeringPrune()
  const { user, dialog } = await openFromTheHeader()

  const gone = await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })
  await user.click(within(gone).getByRole('checkbox', { name: 'Prune feat/search-docs' }))
  expect(within(gone).getByText('Its 1 commit on no other branch is lost.')).toBeTruthy()
  expect(within(gone).getByText('anyway')).toBeTruthy()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Prune 2 worktrees' }))

  expect(await within(dialog).findByText(/^2 pruned\./)).toBeTruthy()
  expect(prune.answered(prune.applied(), 'worktree')).toEqual(['feat/billing', 'feat/search-docs'])
  expect(prune.answered(prune.applied(), 'force')).toEqual(['true'])
})

test('with nothing to prune, the dialog says the rule under the plan-wide warning and ends on Done', async () => {
  answeringPrune((call) =>
    prunePlan(call, {
      found: [],
      warnings: [{ code: 'prune.prs_unavailable', message: "couldn't ask for pull requests (gh: not logged in)" }],
    }),
  )
  const { user, dialog } = await openFromTheHeader()

  expect(await within(dialog).findByText('Nothing to prune')).toBeTruthy()
  expect(within(dialog).getByRole('alert').textContent).toContain("Couldn't ask for pull requests")
  expect(within(dialog).queryByRole('list', { name: 'Steps' })).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Done' }))
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('a worktree named that is no longer done with leaves the run, and Confirm says why', async () => {
  const prune = answeringPrune((call) =>
    call.answers?.some((answer) => answer.path === 'worktree')
      ? prunePlan(call, { found: candidates.filter((c) => c.worktree !== 'feat/billing') })
      : prunePlan(call),
  )
  const { user, dialog } = await openFromTheHeader()

  const gone = await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })
  await user.click(within(gone).getByRole('checkbox', { name: 'Prune feat/search-docs' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText("No longer finished: it has commits past its pull request's head")).toBeTruthy()
  expect(within(dialog).getByRole('listitem', { name: 'feat/search-docs' })).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Prune' }))

  expect(await within(dialog).findByText(/^1 pruned\./)).toBeTruthy()
  expect(prune.answered(prune.applied(), 'worktree')).toEqual(['feat/search-docs'])
})

test("the detail's callout prunes the merged worktree it shows, and that one only", async () => {
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
  const prune = answeringPrune()
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  const callout = await screen.findByRole('region', { name: 'To do' })
  await user.click(within(callout).getByRole('button', { name: 'Prune' }))
  const dialog = await screen.findByRole('dialog', { name: 'Prune worktrees' })

  expect(await within(dialog).findByText('1 of 1 selected')).toBeTruthy()
  expect(prune.answered(prune.calls[0], 'worktree')).toEqual(['feat/billing'])
})

// Only the review fetches origin: a fetch it failed was said nowhere once candidates were found.
test("origin the review couldn't fetch is the plan's Alert on the worktrees and on Confirm", async () => {
  const fetchFailed = {
    code: 'prune.fetch_failed',
    message: "couldn't fetch origin (network down): the branches it deleted are read as last fetched",
  }
  answeringPrune((call) =>
    call.answers?.some((answer) => answer.path === 'no_fetch')
      ? prunePlan(call)
      : prunePlan(call, { warnings: [fetchFailed] }),
  )
  const { user, dialog } = await openFromTheHeader()

  expect((await within(dialog).findByRole('alert')).textContent).toContain("Couldn't fetch origin")
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText(/is stacked on feat\/billing/)
  expect(within(dialog).getByRole('alert').textContent, 'the children step too').toContain("Couldn't fetch origin")
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Prune' })
  expect(within(dialog).getByRole('alert').textContent).toContain("Couldn't fetch origin")
})

// A dialog opened again showed the refusal it had kept from the last time before asking anew, and unchecked a worktree
// done with again, said "No longer finished".
test('a refusal kept from an earlier opening does not leave out a worktree done with again', async () => {
  let refusing = true
  answeringPrune((call) =>
    refusing && call.answers?.some((answer) => answer.path === 'worktree')
      ? prunePlan(call, { found: candidates.filter((c) => c.worktree !== 'feat/billing') })
      : delay(200).then(() => prunePlan(call)),
  )
  const { user, dialog } = await openFromTheHeader()
  await user.click(
    within(await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })).getByRole('checkbox', {
      name: 'Prune feat/search-docs',
    }),
  )
  expect(await within(dialog).findByText(/^No longer finished/)).toBeTruthy()
  expect(within(dialog).queryByText(/is not done with/), 'the refusal is said as the row left out').toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

  refusing = false
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Prune' }))
  const again = await screen.findByRole('dialog', { name: 'Prune worktrees' })
  await user.click(
    within(await within(again).findByRole('listitem', { name: 'feat/search-docs' })).getByRole('checkbox', {
      name: 'Prune feat/search-docs',
    }),
  )
  expect(within(again).queryByText(/is not done with/), 'the kept refusal is not drawn while asked anew').toBeNull()
  const next = async () => {
    const button = await within(again).findByRole('button', { name: 'Next' })
    await waitFor(() => expect(button.getAttribute('aria-disabled')).toBe('false'))
    await user.click(button)
  }
  await next()
  await next()

  expect(await within(again).findByRole('button', { name: 'Prune 2 worktrees' })).toBeTruthy()
  expect(within(again).queryByText(/is not done with/)).toBeNull()
  expect(within(again).queryByText(/^No longer finished/)).toBeNull()
})

// The review was never read again: a worktree that turned unsafe after it still read safe, checked, on its row.
test('a worktree that turns unsafe after the review shows its risk on its row, and starts unchecked', async () => {
  const events = liveEvents()
  let dirty = false
  answeringPrune((call) =>
    dirty && call.answers?.some((answer) => answer.path === 'no_fetch')
      ? prunePlan(call, { found: candidates.map((c) => (c === merged ? { ...c, blockers: ['dirty'] } : c)) })
      : prunePlan(call),
  )
  const { dialog } = await openFromTheHeader()
  const box = async () =>
    within(await within(dialog).findByRole('listitem', { name: 'feat/billing' })).getByRole('checkbox', {
      name: 'Prune feat/billing',
    })
  expect((await box()).getAttribute('aria-checked')).toBe('true')

  dirty = true
  events.changed()

  const row = await within(dialog).findByRole('listitem', { name: 'feat/billing' })
  await waitFor(() => expect(within(row).getByText('Uncommitted changes')).toBeTruthy())
  expect((await box()).getAttribute('aria-checked')).toBe('false')
})

// Read again on an event, the rows lost a worktree no longer done with before any plan refused it: it left the run
// with no word.
test('a worktree the person asked for that leaves the candidates is said under Left as they are', async () => {
  const events = liveEvents()
  let moved = false
  answeringPrune((call) =>
    moved && call.answers?.some((answer) => answer.path === 'no_fetch')
      ? prunePlan(call, { found: candidates.filter((c) => c !== merged) })
      : prunePlan(call),
  )
  const { user, dialog } = await openFromTheHeader()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Prune' })

  moved = true
  events.changed()

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  await waitFor(() => expect(within(left).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy())
  expect(within(left).getByText('No longer finished')).toBeTruthy()
})

// While a plan was refused and the next one asked, the button counted the refused plan: "Prune 0 worktrees".
test('the button never counts a plan refused or still asked', async () => {
  const events = liveEvents()
  let moved = false
  answeringPrune((call) => {
    const named = call.answers?.find((answer) => answer.path === 'worktree')?.values ?? []
    const found = moved ? candidates.filter((c) => c !== merged) : candidates
    if (moved && named.length === 1) return delay(300).then(() => prunePlan(call, { found }))
    // the rows answer after the refused plan, as they did in the sandbox
    if (moved && named.length === 0) return delay(100).then(() => prunePlan(call, { found }))
    return prunePlan(call, { found: call.answers?.some((answer) => answer.path === 'no_fetch') ? found : candidates })
  })
  const { user, dialog } = await openFromTheHeader()
  await user.click(
    within(await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })).getByRole('checkbox', {
      name: 'Prune feat/search-docs',
    }),
  )
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByRole('button', { name: 'Prune 2 worktrees' })

  // the wrong label lasted only while the next plan was asked: every label the button takes is kept
  const seen = new Set<string>()
  const watch = new MutationObserver(() => {
    for (const button of within(dialog).queryAllByRole('button', { name: /^Prune/ })) seen.add(button.textContent ?? '')
  })
  watch.observe(dialog, { subtree: true, childList: true, characterData: true })
  moved = true
  events.changed()

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  await waitFor(() => expect(within(left).getByRole('listitem', { name: 'feat/billing' })).toBeTruthy())
  expect(await within(dialog).findByRole('button', { name: 'Prune' })).toBeTruthy()
  watch.disconnect()
  expect([...seen], 'every label the button took while the plan was refused, then asked again').toEqual(['Prune'])
})
