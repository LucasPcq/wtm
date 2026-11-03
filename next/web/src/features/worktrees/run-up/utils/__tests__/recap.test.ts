import { expect, test } from 'vitest'
import { narrowed } from '@/api/queries/commands'
import type { RunupDetail, RunupPlan } from '@/api/types.gen'
import { recapOf } from '@/features/worktrees/run-up/utils/recap'
import { envelope } from '@/test/fixtures'

test('a drop owed the start paid is a row of its own, said from what became of it, beside the worktrees', () => {
  const started = narrowed<RunupDetail, RunupPlan>({
    ...envelope('run-up', []),
    items: [
      {
        subject: 'feat/a',
        status: 'done',
        detail: { path: '/t/feat-a', profile: 'dev', jobs: [{ name: 'api', status: 'started' }] },
      },
      {
        subject: 'acme_feat_old',
        status: 'skipped',
        reason: 'not_reached',
        detail: { profile: '', jobs: [], owed: { job: 'db', name: 'acme_feat_old', worktree: 'feat-old' } },
      },
    ],
  })

  const { groups } = recapOf(started)

  expect(groups.find((group) => group.kind === 'done')?.rows.map((row) => row.key)).toEqual(['feat/a'])
  expect(groups.find((group) => group.kind === 'failed')?.rows).toEqual([
    { key: 'owed-acme_feat_old', tone: 'muted', title: 'acme_feat_old', message: 'Not dropped: still owed' },
  ])
})
