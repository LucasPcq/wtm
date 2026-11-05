import { expect, test } from 'vitest'
import { countOf, sharedOf, unitsOf } from '@/features/worktrees/run-down/utils/plan'
import { advance } from '@/features/worktrees/utils/progress'
import { runDownPlanOf } from '@/test/run-down'

test('a shared service is counted once, stopped only once every holder let it go', () => {
  const plan = runDownPlanOf(['feat/billing', 'fix/login', 'feat/search'])
  const finished = (job: string, subject: string) =>
    ({ type: 'phase.finished', subject, phase: 'job', job, failed: false, ts: '2026-11-05T12:00:00Z' }) as const
  const one = advance({ units: unitsOf(plan) }, finished('postgres', 'feat/billing'))

  expect(
    unitsOf(plan).map((unit) => unit.subject),
    'a worktree with nothing to stop has no unit',
  ).toEqual(['feat/billing', 'fix/login'])
  expect(sharedOf(plan)).toEqual([
    { name: 'postgres', holders: ['feat/billing', 'fix/login'], keptBy: ['main'], stops: false },
  ])
  expect(countOf(one.units, plan)).toEqual({ done: 0, total: 6 })
  expect(countOf(advance(one, finished('postgres', 'fix/login')).units, plan)).toEqual({ done: 1, total: 6 })
})
