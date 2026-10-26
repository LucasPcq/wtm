import { expect, test } from 'vitest'
import { narrowed } from '@/api/queries/commands'
import type { CleanDetail, CleanPlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/remove/utils/recap'
import { envelope } from '@/test/fixtures'

const ended = narrowed<CleanDetail, CleanPlan>({
  ...envelope('clean', []),
  ok: false,
  items: [
    { subject: 'feat/billing', status: 'done', detail: { path: '/t/feat-billing' } },
    {
      subject: 'spike/gui-poc',
      status: 'failed',
      detail: {},
      error: { kind: 'internal', code: 'clean.hook_failed', message: 'on_clean hook failed: exit status 1' },
    },
  ],
  plan: {
    worktrees: [],
    moves: [{ branch: 'feat/billing-ui', from: 'feat/billing', to: 'main' }],
    reparent_children: true,
    force: true,
  },
  warnings: [
    { code: 'clean.jobs_left', message: 'feat/billing is removed with its jobs still running: api, web' },
    { code: 'clean.data_kept', message: 'feat/billing keeps its data in shared services: acme_feat_billing' },
  ],
})

const units = [
  {
    subject: 'spike/gui-poc',
    phases: [{ phase: 'on_clean' as const, state: 'failed' as const, log: '/state/hooks/spike.log' }],
    lines: ['Error response from daemon: cannot stop container: permission denied'],
  },
]

test('a failure keeps its output and log with it, a warning is to check, and what went says where its children went', () => {
  const { groups } = recapOf({ units, envelope: ended })

  expect(groups).toEqual([
    {
      kind: 'failed',
      rows: [
        {
          key: 'spike/gui-poc',
          tone: 'failed',
          title: 'spike/gui-poc',
          message: 'on_clean hook failed: exit status 1',
          output: { lines: units[0]?.lines, log: '/state/hooks/spike.log' },
          followUp: undefined,
        },
      ],
    },
    {
      kind: 'check',
      rows: [
        {
          key: 'clean.jobs_left-feat/billing is removed with its jobs still running: api, web',
          tone: 'attention',
          message: 'feat/billing is removed with its jobs still running: api, web',
          followUp: undefined,
        },
      ],
    },
    {
      kind: 'done',
      rows: [{ key: 'feat/billing', tone: 'ok', title: 'feat/billing', message: 'feat/billing-ui moved onto main' }],
    },
  ])
})

test('the data a removed worktree keeps is nothing to act on: it goes with the steps, not in "To check"', () => {
  expect(recapOf({ units, envelope: ended }).kept).toEqual([
    'feat/billing keeps its data in shared services: acme_feat_billing',
  ])
})

test('a removal cancelled in its hook stays with its output, said where it stopped', () => {
  const cancelled = narrowed<CleanDetail, CleanPlan>({
    ...envelope('clean', []),
    ok: false,
    items: [{ subject: 'spike/gui-poc', status: 'cancelled', reason: 'interrupted', detail: {} }],
  })
  const stopped = [
    {
      subject: 'spike/gui-poc',
      phases: [{ phase: 'on_clean' as const, state: 'failed' as const, startedAt: '2026-10-26T12:00:00Z' }],
      lines: ['stopping spike/gui-poc'],
    },
  ]

  const [failed] = recapOf({ units: stopped, envelope: cancelled }).groups

  expect(failed?.rows).toEqual([
    {
      key: 'spike/gui-poc',
      tone: 'muted',
      title: 'spike/gui-poc',
      message: 'Cancelled during the on_clean hooks',
      output: { lines: ['stopping spike/gui-poc'], log: undefined },
      followUp: undefined,
    },
  ])
})

test("an item's warning names its worktree, and two alike stay two", () => {
  const skipped = { code: 'clean.hooks_skipped', message: 'its directory is gone: the on_clean hooks did not run' }
  const ended = narrowed<CleanDetail, CleanPlan>({
    ...envelope('clean', []),
    items: ['feat/a', 'feat/b'].map((subject) => ({
      subject,
      status: 'done' as const,
      detail: { path: `/t/${subject}` },
      warnings: [skipped],
    })),
  })

  const check = recapOf({ units: [], envelope: ended }).groups.find((group) => group.kind === 'check')

  expect(check?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/a', 'Its directory is gone: the on_clean hooks did not run'],
    ['feat/b', 'Its directory is gone: the on_clean hooks did not run'],
  ])
  expect(new Set(check?.rows.map((row) => row.key)).size).toBe(2)
})
