import { expect, test } from 'vitest'
import type { CleanRemoval } from '@/api/types.gen'
import { requestOf, unitsOf } from '@/features/worktrees/remove/utils/plan'

const removal = (worktree: string, blockers: CleanRemoval['blockers'] = []): CleanRemoval => ({
  worktree,
  branch: worktree,
  path: `/t/${worktree}`,
  phases: ['on_clean', 'remove'],
  blockers,
})

test('the request drops what is kept and forces only when an unsafe worktree is removed anyway', () => {
  const removals = [removal('feat/a'), removal('feat/b', ['dirty']), removal('feat/c', ['locked'])]

  expect(requestOf({ removals, choices: { 'feat/b': 'keep', 'feat/c': 'keep' }, reparent: false })).toEqual([
    { path: 'worktree', values: ['feat/a'] },
  ])
  expect(requestOf({ removals, choices: { 'feat/b': 'remove', 'feat/c': 'keep' }, reparent: true })).toEqual([
    { path: 'worktree', values: ['feat/a', 'feat/b'] },
    { path: 'force', values: ['true'] },
    { path: 'reparent_children', values: ['true'] },
  ])
})

test('each worktree of the plan is a unit of its own phases, none started', () => {
  expect(unitsOf({ worktrees: [removal('feat/a')], reparent_children: false, force: false })).toEqual([
    {
      subject: 'feat/a',
      phases: [
        { phase: 'on_clean', state: 'todo' },
        { phase: 'remove', state: 'todo' },
      ],
      lines: [],
    },
  ])
})
