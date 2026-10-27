import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { DomainAnswered, WebCall } from '@/api/types.gen'
import { envelope, followUp, statusDetail } from '@/test/fixtures'
import { candidates, prunePlan, pruneRun } from '@/test/prune'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// prune answered by the test, every call kept to read what the web asked
function answeringPrune(dryRun: (call: WebCall) => Response = prunePlan) {
  const calls: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/prune', async ({ request }) => {
      const call = (await request.json()) as WebCall
      calls.push(call)
      return call.dry_run ? dryRun(call) : pruneRun(call)
    }),
  )
  const answered = (call: WebCall | undefined, path: string) =>
    call?.answers?.find((answer: DomainAnswered) => answer.path === path)?.values
  return { calls, answered, applied: () => calls.find((call) => !call.dry_run) }
}

async function openFromTheHeader() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Prune' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Prune worktrees' }) }
}

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
    call.answers?.some((answer) => answer.path === 'force') && !call.answers.some((a) => a.path === 'no_fetch')
      ? prunePlan(call)
      : prunePlan(call, { found: candidates.filter((c) => c.worktree !== 'feat/billing') }),
  )
  const { user, dialog } = await openFromTheHeader()

  const gone = await within(dialog).findByRole('listitem', { name: 'feat/search-docs' })
  await user.click(within(gone).getByRole('checkbox', { name: 'Prune feat/search-docs' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText("No longer done with: it has commits past its pull request's head")).toBeTruthy()
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
