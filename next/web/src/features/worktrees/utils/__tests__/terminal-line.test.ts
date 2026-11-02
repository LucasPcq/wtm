import { expect, test } from 'vitest'
import { terminalLine } from '@/features/worktrees/utils/terminal-line'

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
