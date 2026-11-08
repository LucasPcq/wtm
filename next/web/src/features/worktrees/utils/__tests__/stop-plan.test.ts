import { expect, test } from 'vitest'
import { advance } from '@/features/worktrees/utils/progress'
import { countOf, sharedOf, unitsOf } from '@/features/worktrees/utils/stop-plan'
import { runDownPlanOf } from '@/test/run-down'

test('a shared service is counted once, stopped once every holder let it go; nothing to stop is no unit', () => {
  const plan = runDownPlanOf(['feat/billing', 'fix/login', 'feat/search', 'feat/search-docs'])
  const finished = (job: string, subject: string) =>
    ({ type: 'phase.finished', subject, phase: 'job', job, failed: false, ts: '2026-11-05T12:00:00Z' }) as const
  const one = advance({ units: unitsOf(plan) }, finished('postgres', 'feat/billing'))

  expect(
    unitsOf(plan).map((unit) => unit.subject),
    'a worktree with nothing to stop has no unit',
  ).toEqual(['feat/billing', 'fix/login', 'feat/search'])
  expect(sharedOf(plan)).toEqual([
    {
      name: 'postgres',
      sharedIn: 'main',
      holders: ['feat/billing', 'fix/login', 'feat/search'],
      keptBy: [],
      stops: true,
    },
  ])
  expect(countOf(one.units, plan)).toEqual({ done: 0, total: 6 })
  const two = advance(one, finished('postgres', 'fix/login'))
  expect(countOf(two.units, plan)).toEqual({ done: 0, total: 6 })
  expect(countOf(advance(two, finished('postgres', 'feat/search')).units, plan)).toEqual({ done: 1, total: 6 })
})

test("two repositories' shared services of one name are two services, each counted once", () => {
  const job = (shared_in: string) => ({ name: 'db', hold: 'claim' as const, fate: 'stop' as const, shared_in })
  const plan = {
    worktrees: [
      { worktree: 'feat/a (acme)', path: '/acme/.trees/a', jobs: [job('/acme')] },
      { worktree: 'feat/b (wtm)', path: '/wtm/.trees/b', jobs: [job('/wtm')] },
    ],
  }

  expect(sharedOf(plan).map((service) => [service.sharedIn, service.holders])).toEqual([
    ['/acme', ['feat/a (acme)']],
    ['/wtm', ['feat/b (wtm)']],
  ])
  expect(countOf(unitsOf(plan), plan)).toEqual({ done: 0, total: 2 })
})
