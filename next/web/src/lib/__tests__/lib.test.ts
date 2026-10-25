import { expect, test } from 'vitest'
import { ago } from '@/lib/time'
import { withoutScheme } from '@/lib/url'

test('a moment is said in its largest whole unit before now', () => {
  const now = Date.parse('2026-10-25T12:00:00Z')
  expect(ago('2026-10-25T11:59:30Z', now)).toBe('just now')
  expect(ago('2026-10-25T11:48:00Z', now)).toBe('12 min. ago')
  expect(ago('2026-10-23T12:00:00Z', now)).toBe('2 days ago')
})

test('an address is shown without its scheme', () => {
  expect(withoutScheme('http://web.feat-a.localhost:1355')).toBe('web.feat-a.localhost:1355')
})
