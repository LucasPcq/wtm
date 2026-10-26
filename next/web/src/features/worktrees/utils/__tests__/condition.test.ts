import { expect, test } from 'vitest'
import type { DomainFollowUp, WorktreeCondition } from '@/api/types.gen'
import { overlaid } from '@/features/worktrees/utils/condition'

const said = (condition: WorktreeCondition, command: string) => ({
  condition,
  actions: [{ command, answers: [], code: `x.${command}`, message: command } satisfies DomainFollowUp],
})

test('a pull request GitHub says is merged wins over changed and clean, with its read actions', () => {
  expect(overlaid(said('clean', 'go'), said('merged', 'clean'))).toEqual(said('merged', 'clean'))
  expect(overlaid(said('changed', 'go'), said('merged', 'clean'))).toEqual(said('merged', 'clean'))
})

test('anything else is the live read, as the core decided it', () => {
  expect(overlaid(said('rebasing', 'go'), said('merged', 'clean'))).toEqual(said('rebasing', 'go'))
  expect(overlaid(said('merging', 'go'), said('merged', 'clean'))).toEqual(said('merging', 'go'))
  expect(overlaid(said('missing', 'clean'), said('merged', 'clean'))).toEqual(said('missing', 'clean'))
  expect(overlaid(said('changed', 'go'), said('clean', 'go'))).toEqual(said('changed', 'go'))
  expect(overlaid(said('clean', 'sync'), undefined)).toEqual(said('clean', 'sync'))
})
