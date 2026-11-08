import { expect, test } from 'vitest'
import { groupsOf } from '@/features/worktrees/running/utils/groups'
import { endedLine, endedShownLine, groupLine, jobSince } from '@/features/worktrees/running/utils/list-words'
import { psGroup, psJob, read } from '@/test/fixtures'

const now = Date.parse('2026-10-25T12:00:00Z')

test('a group says its profile and counts, its ended ones once folded, and the shared services their holders', () => {
  const [billing, shared] = groupsOf({
    read: read('run-ps', [
      psGroup('feat/billing', [
        psJob('web'),
        psJob('worker', { state: 'crashed' }),
        psJob('migrate', { state: 'exited' }),
      ]),
      psGroup('main', [psJob('postgres')], {
        shared: true,
        profile: '',
        counts: { running: 1, starting: 0, crashed: 0, exited: 0, stopped: 0, not_started: 0, holders: 3 },
      }),
    ]),
    progress: undefined,
    plan: undefined,
  })

  expect(billing && groupLine(billing, false)).toBe('profile dev · 1 up · 1 crashed')
  expect(billing && groupLine(billing, true)).toBe('profile dev · 1 up · 1 crashed · 1 exited')
  expect(shared && groupLine(shared, true)).toBe('1 up · held by 3 worktrees')
})

test('the quiet row names its jobs, each with its state once they differ', () => {
  expect(endedLine([psJob('migrate', { state: 'exited' })])).toBe('1 exited · migrate')
  expect(endedLine([psJob('api', { state: 'stopped' }), psJob('migrate', { state: 'exited' })])).toBe(
    '2 hidden · api stopped, migrate exited',
  )
  expect(endedShownLine([psJob('migrate', { state: 'exited' })])).toBe('Exited jobs shown')
})

test('a job says how long it runs, or how it ended and when', () => {
  expect(jobSince(psJob('web', { started_at: '2026-10-25T11:48:00Z' }), now)).toBe('up 12 min')
  expect(jobSince(psJob('worker', { state: 'crashed', exit_code: 1, ended_at: '2026-10-25T11:58:00Z' }), now)).toBe(
    'crashed · exit 1 · 2 min. ago',
  )
  expect(jobSince(psJob('migrate', { state: 'exited', exit_code: 0 }), now)).toBe('exited 0')
})
