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
      tone: 'muted',
      title: 'feat/onboarding',
      message: 'Cancelled during the on_create hooks',
      output: { lines: ['Packages: +1204'], log: undefined },
      followUp: go,
    },
  ])
})

test("a create that failed says why with its output, an item's warning names its branch, and the groups keep their order", () => {
  const failed = narrowed<CreateDetail, CreatePlan>({
    ...envelope('create', []),
    ok: false,
    items: [
      {
        subject: 'feat/broken',
        status: 'failed',
        detail: {},
        error: { kind: 'internal', code: 'unit.phase_failed', message: 'on_create failed: exit status 1' },
      },
      {
        subject: 'feat/onboarding',
        status: 'done',
        detail: { path: '/t/feat-onboarding' },
        warnings: [
          {
            code: 'create.behind_origin',
            message: '2 commit(s) behind origin — the worktree starts from your local branch.',
          },
        ],
      },
    ],
    follow_ups: [go],
  })
  const units = [
    {
      subject: 'feat/broken',
      phases: [{ phase: 'on_create' as const, state: 'failed' as const }],
      lines: ['ERR_PNPM_FETCH_404'],
    },
  ]

  const groups = recapOf({ units, envelope: failed })

  expect(groups.map((group) => group.kind)).toEqual(['failed', 'check', 'done'])
  expect(groups[0]?.rows).toEqual([
    {
      key: 'feat/broken',
      tone: 'failed',
      title: 'feat/broken',
      message: 'on_create failed: exit status 1',
      output: { lines: ['ERR_PNPM_FETCH_404'], log: undefined },
      followUp: undefined,
    },
  ])
  expect(groups[1]?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/onboarding', '2 commit(s) behind origin — the worktree starts from your local branch.'],
  ])
})

test('a next step no item claims is said plainly among what went, never as a thing to check', () => {
  const next = followUp('run', 'feat/other', { code: 'create.next', message: 'start its profile' })
  const done = narrowed<CreateDetail, CreatePlan>({ ...envelope('create', []), follow_ups: [next] })

  const groups = recapOf({ units: [], envelope: done })

  expect(groups.find((group) => group.kind === 'check')?.rows).toEqual([])
  expect(groups.find((group) => group.kind === 'done')?.rows).toEqual([
    { key: 'create.next-start its profile', message: 'Start its profile', go: next },
  ])
})
