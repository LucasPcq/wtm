import { expect, test } from 'vitest'
import { narrowed } from '@/api/queries/commands'
import type { CleanDetail, CleanPlan, PruneDetail, PrunePlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/utils/removal-recap'
import { envelope } from '@/test/fixtures'

const ended = narrowed<CleanDetail, CleanPlan>({
  ...envelope('clean', []),
  ok: false,
  items: [
    {
      subject: 'feat/billing',
      status: 'done',
      detail: {
        path: '/t/feat-billing',
        namespaces: [
          { job: 'db', name: 'acme_feat_billing', status: 'dropped' },
          { job: 'search', name: 'acme_feat_billing', status: 'deferred', reason: 'service_down' },
          { job: 'cache', name: 'acme_feat_billing', status: 'failed' },
        ],
      },
      warnings: [
        { code: 'clean.data_deferred', message: 'acme_feat_billing stays in search: search is down' },
        { code: 'clean.drop_failed', message: "couldn't drop acme_feat_billing from cache (exit status 1)" },
      ],
    },
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
    {
      code: 'clean.release_failed',
      message: "couldn't let go of feat/billing's shared services: they run until stopped",
      params: { worktree: 'feat/billing' },
    },
  ],
})

const units = [
  {
    subject: 'spike/gui-poc',
    phases: [{ phase: 'on_clean' as const, state: 'failed' as const, log: '/state/hooks/spike.log' }],
    lines: ['Error response from daemon: cannot stop container: permission denied'],
  },
]

test("a failure keeps its output and log with it, the run's warnings go to the outcome, and what went says where its children went", () => {
  const { groups, warnings } = recapOf({ units, envelope: ended }, 'clean')

  expect(warnings.map((warning) => warning.code)).toEqual(['clean.release_failed'])

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
          key: "feat/billing-clean.drop_failed-couldn't drop acme_feat_billing from cache (exit status 1)",
          tone: 'attention',
          title: 'feat/billing',
          message: "Couldn't drop acme_feat_billing from cache (exit status 1)",
          output: undefined,
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

test('what became of a removed worktree\'s data is nothing to act on: it goes with the steps, a failed drop to "To check"', () => {
  expect(recapOf({ units, envelope: ended }, 'clean').data).toEqual([
    'feat/billing: acme_feat_billing in db dropped',
    'feat/billing: acme_feat_billing in search stays until search is up, then the next run up or prune drops it',
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

  const [failed] = recapOf({ units: stopped, envelope: cancelled }, 'clean').groups

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

  const check = recapOf({ units: [], envelope: ended }, 'clean').groups.find((group) => group.kind === 'check')

  expect(check?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/a', 'Its directory is gone: the on_clean hooks did not run'],
    ['feat/b', 'Its directory is gone: the on_clean hooks did not run'],
  ])
  expect(new Set(check?.rows.map((row) => row.key)).size).toBe(2)
})

test('the drops owed a prune paid are said from what became of each: dropped done, failed with its reason', () => {
  const owed = { job: 'db', worktree: 'feat-old' }
  const paid = narrowed<PruneDetail, PrunePlan>({
    ...envelope('prune', []),
    ok: false,
    items: [
      { subject: 'acme_feat_old', status: 'done', detail: { owed: { ...owed, name: 'acme_feat_old' } } },
      {
        subject: 'acme_feat_older',
        status: 'failed',
        error: {
          kind: 'internal',
          code: 'prune.drop_failed',
          message: "couldn't drop acme_feat_older from db, still owed",
        },
        detail: { owed: { ...owed, name: 'acme_feat_older' } },
      },
      {
        subject: 'acme_feat_oldest',
        status: 'skipped',
        reason: 'not_reached',
        detail: { owed: { ...owed, name: 'acme_feat_oldest' } },
      },
    ],
    plan: { worktrees: [], reasons: ['merged'], reparent_children: false, force: false },
  })

  const { groups } = recapOf({ units: [], envelope: paid }, 'prune')

  expect(groups.find((group) => group.kind === 'done')?.rows).toEqual([
    {
      key: 'owed-acme_feat_old',
      tone: 'ok',
      title: 'acme_feat_old',
      message: 'Dropped from db, owed since feat-old was removed',
    },
  ])
  expect(groups.find((group) => group.kind === 'failed')?.rows).toEqual([
    {
      key: 'owed-acme_feat_older',
      tone: 'failed',
      title: 'acme_feat_older',
      message: "Couldn't drop acme_feat_older from db, still owed",
    },
    { key: 'owed-acme_feat_oldest', tone: 'muted', title: 'acme_feat_oldest', message: 'Not dropped: still owed' },
  ])
})
