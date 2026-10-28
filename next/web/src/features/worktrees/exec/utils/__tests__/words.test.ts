import { expect, test } from 'vitest'
import { lockWords, warningLine } from '@/features/worktrees/exec/utils/words'

test('a lock held names its worktree and its holder, as the person knows them', () => {
  expect(
    lockWords(
      {
        kind: 'conflict',
        code: 'lock.held',
        message: 'fix/login is locked by clean, started from cli (pid 4242)',
        params: { scope: 'wt', name: 'fix/login', command: 'clean', surface: 'cli', op: 'op-9' },
      },
      true,
    ),
  ).toEqual({
    title: 'fix/login is locked',
    description: "A clean started from the terminal holds it. Try again once it's done.",
  })
})

test('a holder whose record could not be read is another wtm; the repository is named as such', () => {
  expect(
    lockWords(
      {
        kind: 'conflict',
        code: 'lock.held',
        message: 'x',
        params: { scope: 'repo', name: '', command: 'unknown' },
      },
      false,
    ),
  ).toEqual({
    title: 'The repository is locked',
    description: "Another wtm may still hold it. Try again once it's done.",
  })
})

test('a warning the screen words is its words; another is the core message', () => {
  expect(warningLine({ code: 'exec.env_unnumbered', message: 'feat/a has no ordinal: …' })).toBe(
    'Without its run env: it has no number yet',
  )
  expect(warningLine({ code: 'exec.other', message: 'something else' })).toBe('Something else')
})
