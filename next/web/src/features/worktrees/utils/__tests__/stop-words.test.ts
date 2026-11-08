import { expect, test } from 'vitest'
import { sharedStopping } from '@/features/worktrees/utils/stop-words'

const done = { phase: 'job', job: 'postgres', state: 'done' } as const
const lastHolds = { name: 'postgres', holders: ['feat/a', 'feat/b'], keptBy: [], stops: true }

// The plan saw the stop's holds as postgres's last; another worktree claimed it meanwhile: its row said "stopped".
test("a shared service's row says what the stop did once the outcome is known", () => {
  expect(sharedStopping(done, lastHolds)).toBe('stopped')
  expect(sharedStopping(done, lastHolds, ['released'])).toBe('released: keeps running for another worktree')
  expect(sharedStopping(done, { ...lastHolds, stops: false, keptBy: ['feat/c'] }, ['stopped'])).toBe('stopped')
})
