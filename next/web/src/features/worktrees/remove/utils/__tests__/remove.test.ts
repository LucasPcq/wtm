import { expect, test } from 'vitest'
import type { TeardownRemoval } from '@/api/types.gen'
import { requestOf } from '@/features/worktrees/remove/utils/plan'

const removal = (worktree: string, blockers: TeardownRemoval['blockers'] = []): TeardownRemoval => ({
  worktree,
  branch: worktree,
  path: `/t/${worktree}`,
  phases: ['on_clean', 'remove'],
  blockers,
})

test('the request takes the checked worktrees and forces only when an unsafe one is among them', () => {
  const [a, b] = [removal('feat/a'), removal('feat/b', ['dirty'])]

  expect(requestOf({ going: [a], reparent: false })).toEqual([{ path: 'worktree', values: ['feat/a'] }])
  expect(requestOf({ going: [a, b], reparent: true })).toEqual([
    { path: 'worktree', values: ['feat/a', 'feat/b'] },
    { path: 'force', values: ['true'] },
    { path: 'reparent_children', values: ['true'] },
  ])
})
