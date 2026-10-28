import { expect, test } from 'vitest'
import type { Applied } from '@/api/queries/commands'
import { recapOf } from '@/features/worktrees/exec/utils/recap'

const cancelled: Applied<'exec'> = {
  v: 2,
  ok: false,
  command: 'exec',
  plan: undefined,
  follow_ups: [],
  warnings: [],
  error: { kind: 'cancelled', code: 'cancelled', message: 'cancelled' },
  items: [
    {
      subject: 'main',
      status: 'done',
      detail: { path: '/acme', exit_code: 0, duration_ms: 3100, tail: ['ok'] },
    },
    {
      subject: 'feat/a',
      status: 'cancelled',
      reason: 'interrupted',
      detail: { path: '/t/a', exit_code: null, duration_ms: 900, tail: [] },
    },
    { subject: 'feat/b', status: 'skipped', reason: 'not_reached', detail: undefined },
  ],
}

test('a cancelled run groups what it stopped apart from what passed, the stopped ones first', () => {
  const { groups, labels } = recapOf(cancelled)

  expect(labels.failed).toBe('Stopped')
  expect(groups.find((group) => group.kind === 'failed')?.rows.map((row) => [row.title, row.message])).toEqual([
    ['feat/a', 'cancelled while running'],
    ['feat/b', 'not reached'],
  ])
  expect(groups.find((group) => group.kind === 'done')?.rows.map((row) => [row.title, row.message])).toEqual([
    ['main', '3.1 s'],
  ])
})

test("a run where some failed and some were stopped calls the first group what they share: they didn't pass", () => {
  const failed = {
    subject: 'feat/c',
    status: 'failed' as const,
    detail: { path: '/t/c', exit_code: 2, duration_ms: 5200, tail: [] },
  }

  expect(recapOf({ ...cancelled, items: [...cancelled.items, failed] }).labels.failed).toBe("Didn't pass")
})
