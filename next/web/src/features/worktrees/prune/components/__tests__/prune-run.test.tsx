import { screen, within } from '@testing-library/react'
import { HttpResponse } from 'msw'
import { expect, test } from 'vitest'
import { heldRun } from '@/test/clean'
import { envelope, followUp } from '@/test/fixtures'
import { prunePlan, unsafeRefusal } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'

// A run that pruned all it was asked, leaving the kept ones, toasted a red "1 of 1 pruned." and reopened on its Done.
test("a hidden run that did all it was asked says so in amber with what's left, and the next Prune opens anew", async () => {
  const held = heldRun()
  answeringPrune(prunePlan, held.respond)
  const { user, dialog } = await openFromTheHeader()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Prune' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))

  held.end({
    ...envelope('prune', []),
    items: [
      { subject: 'feat/billing', status: 'done', detail: {} },
      { subject: 'feat/search-docs', status: 'skipped', reason: 'ruled_out', detail: {} },
    ],
    warnings: [
      {
        code: 'prune.kept',
        message: 'feat/search-docs is kept: unsafe to remove (unpushed)',
        params: { worktree: 'feat/search-docs' },
        follow_up: followUp('prune', 'feat/search-docs', { code: 'prune.force', message: 'prune them anyway' }),
      },
    ],
  })

  const told = await screen.findByText('1 pruned.')
  expect(told.closest('[data-type]')?.getAttribute('data-type')).toBe('warning')
  expect(told.closest('[data-type]')?.className, 'its glyph in amber, the colour of something to do').toContain(
    'text-amber-500',
  )
  await user.click(await screen.findByRole('button', { name: 'Actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Prune' }))
  const again = await screen.findByRole('dialog', { name: 'Prune worktrees' })
  expect(await within(again).findByRole('listitem', { name: 'feat/search-docs' })).toBeTruthy()
  expect(within(again).queryByText(/pruned\./)).toBeNull()
})

// A cancelled run drew the envelope's own error, "interrupted", as the run's failure.
test('a cancelled run says what was cancelled, with no failure of its own', async () => {
  const held = heldRun()
  answeringPrune(prunePlan, held.respond)
  const { user, dialog } = await openFromTheHeader()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Prune' }))

  held.end({
    ...envelope('prune', []),
    ok: false,
    items: [{ subject: 'feat/billing', status: 'cancelled', reason: 'interrupted', detail: {} }],
    error: { kind: 'cancelled', code: 'domain.interrupted', message: 'interrupted' },
  })

  expect(await within(dialog).findByText(/pruned\. Those not started stay\.$/)).toBeTruthy()
  expect(within(dialog).queryByText(/^interrupted$/i)).toBeNull()
})

// A worktree that turned unsafe between the last plan and the apply refused the run, and the dialog ended on it.
test('an apply refused because a worktree changed goes back to the worktrees, saying which changed', async () => {
  answeringPrune(prunePlan, () => HttpResponse.json(unsafeRefusal(['feat/billing']), { status: 403 }))
  const { user, dialog } = await openFromTheHeader()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Prune' }))

  expect(
    await within(dialog).findByText(
      'feat/billing changed since the plan, so nothing was pruned. Check the worktrees again.',
    ),
  ).toBeTruthy()
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing' })).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' })).toBeTruthy()
})
