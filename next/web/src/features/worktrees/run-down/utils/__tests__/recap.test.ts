import { expect, test } from 'vitest'
import type { Applied } from '@/api/queries/commands'
import type { RundownPlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/run-down/utils/recap'

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
