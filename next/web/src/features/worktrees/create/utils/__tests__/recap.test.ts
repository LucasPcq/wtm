import { expect, test } from 'vitest'
import { narrowed } from '@/api/queries/commands'
import type { CreateDetail, CreatePlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/create/utils/recap'
import { envelope, followUp } from '@/test/fixtures'

const go = followUp('go', 'feat/onboarding', { code: 'create.go', message: 'jump in' })

test('a create cancelled in a step says which, keeps its output, and still offers the step into what stands', () => {
  const cancelled = narrowed<CreateDetail, CreatePlan>({
    ...envelope('create', []),
    ok: false,
    items: [{ subject: 'feat/onboarding', status: 'cancelled', detail: {} }],
    follow_ups: [go],
  })
  const units = [
    {
      subject: 'feat/onboarding',
      phases: [
        { phase: 'worktree' as const, state: 'done' as const, startedAt: '2026-10-26T12:00:00Z' },
        { phase: 'on_create' as const, state: 'failed' as const, startedAt: '2026-10-26T12:00:01Z' },
      ],
      lines: ['Packages: +1204'],
    },
  ]

  const [failed] = recapOf({ units, envelope: cancelled })

  expect(failed?.rows).toEqual([
    {
      key: 'feat/onboarding',
      tone: 'stopped',
      title: 'feat/onboarding',
      message: 'Cancelled during the on_create hooks',
      output: { lines: ['Packages: +1204'], log: undefined },
      followUp: go,
    },
  ])
})
