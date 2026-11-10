import { screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { answeringEnv } from '@/test/env'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

async function openReconcile() {
  const app = await renderApp('/r/r1/env')
  await app.user.click(await screen.findByRole('button', { name: 'Reconcile' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

test('Reconcile opens on the worktrees out of step, checked by the core, a verbatim one said so', async () => {
  const { dialog } = await openReconcile()

  const box = (name: string) => within(dialog).findByRole('checkbox', { name: `Reconcile ${name}` })
  expect((await box('feat/billing')).getAttribute('data-state')).toBe('checked')
  expect((await box('fix/login')).getAttribute('data-state')).toBe('checked')
  expect((await box('main')).getAttribute('data-state')).toBe('unchecked')
  expect(within(dialog).getByText('2 keys out of step')).toBeTruthy()
  expect(within(dialog).getByText('verbatim: its env is copied as it is')).toBeTruthy()
})

test('Confirm lists what each worktree rewrites, the job to restart and what is left; the run ends on what to check', async () => {
  const { user, dialog } = await openReconcile()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const rewrite = await within(dialog).findByRole('region', { name: 'To rewrite' })
  expect(within(rewrite).getByText('apps/web/.env · 3020')).toBeTruthy()
  expect(within(rewrite).getByText('web runs on the old values until restarted')).toBeTruthy()
  expect(within(dialog).getByText('apps/api/.env: in no source: kept, unless pruned')).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Reconcile 2 worktrees' }))

  expect(await within(dialog).findByText('2 worktrees reconciled.')).toBeTruthy()
  const toCheck = within(dialog).getByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText(/web runs on the old values until restarted/i)).toBeTruthy()
  expect(within(toCheck).getByRole('button', { name: /^Restart: / })).toBeTruthy()
})

test('a worktree that could not be written is named with its cause, the others reconciled', async () => {
  server.use(...answeringEnv({ failing: ['fix/login'] }))
  const { user, dialog } = await openReconcile()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Reconcile 2 worktrees' }))

  expect(await within(dialog).findByText('1 of 2 worktrees reconciled.')).toBeTruthy()
  const failed = within(dialog).getByRole('region', { name: "Couldn't reconcile" })
  expect(within(failed).getByText("couldn't write its .env files")).toBeTruthy()
})
