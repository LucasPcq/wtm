import { expect, test } from 'vitest'
import { duration, elapsed } from '@/features/worktrees/exec/utils/output'

test('a time is in seconds: one decimal once ended, whole seconds while it runs', () => {
  expect(duration(3140)).toBe('3.1 s')
  expect(duration(200)).toBe('0.2 s')
  expect(elapsed('2026-10-25T12:00:00Z', Date.parse('2026-10-25T12:00:08.6Z'))).toBe('8 s')
})
