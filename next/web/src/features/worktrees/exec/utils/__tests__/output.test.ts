import { expect, test } from 'vitest'
import { duration, elapsed, terminalLine } from '@/features/worktrees/exec/utils/output'

const esc = String.fromCharCode(27)

test('a line keeps its text and drops its colours, its title and its cursor moves', () => {
  expect(terminalLine(`${esc}[32m✓${esc}[39m src/a.test.ts ${esc}[2m(12 tests)${esc}[22m`)).toBe(
    '✓ src/a.test.ts (12 tests)',
  )
  expect(terminalLine(`${esc}]0;vitest${String.fromCharCode(7)}RUN v2.1.4`)).toBe('RUN v2.1.4')
  expect(terminalLine(`${esc}[2K${esc}[1Gdone`)).toBe('done')
})

test('the short escapes go too: a cursor saved and restored, a charset chosen', () => {
  expect(terminalLine(`${esc}7saved${esc}8`)).toBe('saved')
  expect(terminalLine(`${esc}(Bplain`)).toBe('plain')
})

test('a line a progress bar redrew is its last frame', () => {
  expect(terminalLine('10%\r50%\r100%\r')).toBe('100%')
})

test('a time is in seconds: one decimal once ended, whole seconds while it runs', () => {
  expect(duration(3140)).toBe('3.1 s')
  expect(duration(200)).toBe('0.2 s')
  expect(elapsed('2026-10-25T12:00:00Z', Date.parse('2026-10-25T12:00:08.6Z'))).toBe('8 s')
})
