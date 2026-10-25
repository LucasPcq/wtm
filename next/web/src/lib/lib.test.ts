import { expect, test } from 'vitest'
import { shellLine } from './shell'
import { ago } from './time'

test('an argument is quoted only when the shell would split or expand it', () => {
  expect(shellLine(['wtm', 'clean', 'feat/a'])).toBe('wtm clean feat/a')
  expect(shellLine(['wtm', 'create', 'it has spaces', "it's"])).toBe(`wtm create 'it has spaces' 'it'\\''s'`)
})

test('a moment is said in its largest whole unit before now', () => {
  const now = Date.parse('2026-10-25T12:00:00Z')
  expect(ago('2026-10-25T11:59:30Z', now)).toBe('just now')
  expect(ago('2026-10-25T11:48:00Z', now)).toBe('12 min. ago')
  expect(ago('2026-10-23T12:00:00Z', now)).toBe('2 days ago')
})
