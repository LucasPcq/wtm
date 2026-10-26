import { screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { useRuns } from '@/features/worktrees/hooks/use-run'
import { renderApp } from '@/test/render'

const bar = () => screen.getByRole('toolbar', { name: 'Selection' })

test('the main worktree has no box, the header checks every other one, and says when only some are', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  const every = screen.getByRole('checkbox', { name: 'Select every worktree' })
  expect(every.getAttribute('aria-checked')).toBe('mixed')
  expect(within(bar()).queryByRole('checkbox')).toBeNull()

  await user.click(every)

  expect(screen.queryByRole('checkbox', { name: 'Select main' })).toBeNull()
  expect(within(bar()).getByText('3 selected')).toBeTruthy()
  await user.click(screen.getByRole('button', { name: 'Clear' }))
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('a worktree filtered out of view leaves the selection', async () => {
  const { user, router } = await renderApp('/r/r1/worktrees')
  await user.click(await screen.findByRole('checkbox', { name: 'Select feat/billing' }))
  await user.click(screen.getByRole('checkbox', { name: 'Select fix/login' }))

  await router.navigate({ to: '/r/$repo/worktrees', params: { repo: 'r1' }, search: { q: 'login' } })

  expect(await within(bar()).findByText('1 selected')).toBeTruthy()
})

test('a worktree a run is working on has no box', async () => {
  useRuns.setState({
    'create:r1': {
      running: true,
      progress: { units: [{ subject: 'feat/billing', phases: [{ phase: 'on_create', state: 'now' }], lines: [] }] },
    },
  })
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('checkbox', { name: 'Select every worktree' }))

  expect(screen.queryByRole('checkbox', { name: 'Select feat/billing' })).toBeNull()
  expect(within(bar()).getByText('2 selected')).toBeTruthy()
})
