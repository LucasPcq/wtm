import { expect, test } from 'vitest'
import { owedTally } from '@/features/worktrees/utils/owed'

const owed = { job: 'db', name: 'acme_feat_old', worktree: 'feat-old' }

test('the drops owed are counted apart from the worktrees, as the command line counts them', () => {
  expect(
    owedTally([
      { subject: 'feat/gone', status: 'done', detail: { path: '/t/feat-gone' } },
      { subject: 'acme_a', status: 'done', detail: { owed } },
      { subject: 'acme_b', status: 'done', detail: { owed } },
      { subject: 'acme_c', status: 'failed', detail: { owed } },
      { subject: 'acme_d', status: 'skipped', detail: { owed } },
    ]),
  ).toEqual(['2 owed drops made', '1 owed drop failed', '1 owed drop not made'])
})
