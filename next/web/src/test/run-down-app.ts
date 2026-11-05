import { screen, waitFor, within } from '@testing-library/react'
import { expect } from 'vitest'
import type { renderApp } from '@/test/render'

type User = Awaited<ReturnType<typeof renderApp>>['user']

export const enabled = (dialog: HTMLElement, name: string) =>
  waitFor(() => expect(within(dialog).getByRole('button', { name }).getAttribute('aria-disabled')).toBe('false'))

export const disabled = (dialog: HTMLElement, name: string) =>
  waitFor(() => expect(within(dialog).getByRole('button', { name }).getAttribute('aria-disabled')).toBe('true'))

export const rows = (region: HTMLElement) =>
  within(region)
    .getAllByRole('listitem')
    .map((row) => row.textContent)

// from the page's header: the worktrees checked, Next, then Confirm
export async function confirm(user: User, worktrees: string[], primary: string) {
  await user.click(await screen.findByRole('button', { name: 'Stop jobs' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop jobs' })
  for (const worktree of worktrees)
    await user.click(await within(dialog).findByRole('checkbox', { name: `Stop in ${worktree}` }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await enabled(dialog, primary)
  return dialog
}

export const confirmBoth = (user: User) => confirm(user, ['feat/billing', 'fix/login'], 'Stop jobs in 2 worktrees')

// the stop of both started, then its dialog hidden
export async function startHidden(user: User) {
  const dialog = await confirmBoth(user)
  await user.click(within(dialog).getByRole('button', { name: 'Stop jobs in 2 worktrees' }))
  await within(dialog).findByText(/Stopping jobs in 2 worktrees/)
  await user.click(within(dialog).getByRole('button', { name: 'Hide' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
}
