import { expect, test } from 'vitest'
import type { TeardownRemoval } from '@/api/types.gen'
import { unitsOf } from '@/features/worktrees/utils/removal'

const removal = (worktree: string, phases: TeardownRemoval['phases'] = ['on_clean', 'remove']): TeardownRemoval => ({
  worktree,
  branch: worktree,
  path: `/t/${worktree}`,
  phases,
  blockers: [],
})

test('each worktree of the plan is a unit of its own phases, none started', () => {
  expect(unitsOf({ worktrees: [removal('feat/a')] })).toEqual([
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

test('a worktree the plan leaves as it is, with no phase, is no unit of the run', () => {
  expect(unitsOf({ worktrees: [removal('feat/a', []), removal('feat/b')] }).map((unit) => unit.subject)).toEqual([
    'feat/b',
  ])
})
