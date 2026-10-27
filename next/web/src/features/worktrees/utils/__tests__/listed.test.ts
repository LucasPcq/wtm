import { expect, test } from 'vitest'
import { listed } from '@/features/worktrees/utils/listed'

test('names read as a sentence says them', () => {
  expect(listed([])).toBe('')
  expect(listed(['a'])).toBe('a')
  expect(listed(['a', 'b'])).toBe('a and b')
  expect(listed(['a', 'b', 'c'])).toBe('a, b and c')
})
