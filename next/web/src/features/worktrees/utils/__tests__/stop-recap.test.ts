import { expect, test } from 'vitest'
import type { Applied } from '@/api/queries/commands'
import type { RundownPlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/utils/stop-recap'

// feat/a runs a cache of its own; feat/b holds the shared cache main runs
const plan: RundownPlan = {
  worktrees: [
    { worktree: 'feat/a', path: '/a', jobs: [{ name: 'cache', hold: 'own', fate: 'stop' }] },
    {
      worktree: 'feat/b',
      path: '/b',
      jobs: [{ name: 'cache', hold: 'claim', fate: 'release', kept_by: ['feat/c'], shared_in: 'main' }],
    },
  ],
}

const done = (subject: string, status: 'stopped' | 'released') => ({
  subject,
  status: 'done' as const,
  detail: { path: `/${subject}`, jobs: [{ name: 'cache', status }] },
})

test("a worktree's own job named like a shared service is its own, said among what it stopped", () => {
  const envelope = {
    ok: true,
    items: [done('feat/a', 'stopped'), done('feat/b', 'released')],
  } as unknown as Applied<'run-down'>

  const { groups } = recapOf(envelope, plan)

  const [, , released, stopped] = groups
  expect(released?.rows.map((row) => [row.title, row.message])).toEqual([['cache', 'keeps running for feat/c']])
  expect(stopped?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/a', 'cache'],
    ['feat/b', 'let go of cache'],
  ])
})

test('a stop cut short before a shared service says it was not reached, never among what stopped', () => {
  const cut = {
    subject: 'feat/b',
    status: 'skipped' as const,
    reason: 'not_reached',
    detail: { path: '/b', jobs: [{ name: 'cache', status: 'not_reached' }] },
  }
  const envelope = { ok: false, items: [done('feat/a', 'stopped'), cut] } as unknown as Applied<'run-down'>

  const [failed, , released, stopped] = recapOf(envelope, plan).groups

  expect(failed?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/b', 'cache not reached'],
    ['cache', 'not reached'],
  ])
  expect(released?.rows).toEqual([])
  expect(stopped?.rows.map((row) => row.title)).toEqual(['feat/a'])
})
