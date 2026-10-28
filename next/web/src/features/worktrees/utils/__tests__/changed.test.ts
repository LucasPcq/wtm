import { expect, test } from 'vitest'
import { changedSincePlan } from '@/features/worktrees/utils/changed'

const refused = (fields: { value: string }[]) => ({
  progress: {
    units: [],
    envelope: {
      error: {
        kind: 'refused' as const,
        code: 'clean.unsafe',
        message: 'nothing removed',
        fields: fields.map(({ value }, i) => ({
          path: `worktree[${i}]`,
          code: 'clean.dirty',
          flag: '<worktree>',
          params: { value },
          message: value,
        })),
      },
    },
  },
  running: false,
})

const words = { code: 'clean.unsafe', past: 'removed' }

test('a worktree with several blockers is named once', () => {
  expect(changedSincePlan(refused([{ value: 'a' }, { value: 'a' }, { value: 'b' }]), words)?.message).toBe(
    'a and b changed since the plan, so nothing was removed. Check the worktrees again.',
  )
})

test('a refusal with no field still says a worktree changed', () => {
  expect(changedSincePlan(refused([]), words)?.message).toMatch(/^A worktree changed since the plan/)
})

test('only its own code, and only once the run ended', () => {
  expect(changedSincePlan(refused([{ value: 'a' }]), { ...words, code: 'prune.unsafe' })).toBeUndefined()
  expect(changedSincePlan({ ...refused([{ value: 'a' }]), running: true }, words)).toBeUndefined()
  expect(changedSincePlan({ progress: undefined, running: false }, words)).toBeUndefined()
})
