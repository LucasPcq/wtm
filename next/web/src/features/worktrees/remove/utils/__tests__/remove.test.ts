import { expect, test } from 'vitest'
import { advance, planned, settled } from '@/features/worktrees/remove/utils/progress'
import { reasonOf } from '@/features/worktrees/remove/utils/words'
import { envelope } from '@/test/fixtures'

const plan = {
  worktrees: [
    { worktree: 'feat/a', path: '/t/feat-a', blockers: [] },
    { worktree: 'feat/b', path: '/t/feat-b', blockers: [] },
  ],
  reparent_children: false,
  force: false,
}

test('a unit follows its phase and its last line, and a worktree never reached takes the status the envelope gives it', () => {
  let progress = planned(plan)
  progress = advance(progress, { type: 'phase.started', subject: 'feat/a', phase: 'on_clean', ts: '' })
  progress = advance(progress, { type: 'phase.output', subject: 'feat/a', line: 'docker compose down', ts: '' })
  expect(progress.units[0]).toEqual({ subject: 'feat/a', phase: 'on_clean', line: 'docker compose down' })

  progress = advance(progress, { type: 'unit.finished', subject: 'feat/a', status: 'done', ts: '' })
  const ended = {
    ...progress,
    envelope: {
      ...envelope('clean', []),
      plan: undefined,
      items: [
        { subject: 'feat/a', status: 'done' as const, detail: undefined },
        { subject: 'feat/b', status: 'skipped' as const, detail: undefined },
      ],
    },
  }
  expect(settled(ended).map((unit) => unit.status)).toEqual(['done', 'skipped'])
})

test('a reason of a known blocker reads in the web words, any other in the core message', () => {
  expect(
    reasonOf({ path: 'worktree[0]', code: 'clean.unpushed', params: { count: '2' }, flag: '<worktree>', message: 'm' }),
  ).toBe('2 commits no other branch nor remote holds')
  expect(reasonOf({ path: 'worktree[0]', code: 'clean.other', flag: '<worktree>', message: 'the core says' })).toBe(
    'the core says',
  )
})
