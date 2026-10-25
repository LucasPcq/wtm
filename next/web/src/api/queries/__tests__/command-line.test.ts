import { expect, test } from 'vitest'
import { shellLine } from '@/api/queries/command-line'

test('an argument is quoted only when the shell would split or expand it', () => {
  expect(shellLine(['wtm', 'clean', 'feat/a'])).toBe('wtm clean feat/a')
  expect(shellLine(['wtm', 'create', 'it has spaces', "it's"])).toBe(`wtm create 'it has spaces' 'it'\\''s'`)
})
