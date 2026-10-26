import { expect, test } from 'vitest'
import type { OperationLine } from '@/api/queries/operations'
import { advance, current, doneOf, type Progress, seconds, todo, took } from '@/features/worktrees/utils/progress'

const line = (event: Record<string, unknown>, ts = '2026-10-25T12:00:00Z') =>
  ({ ts, ...event }) as unknown as OperationLine

const units = [todo('main', ['fast_forward']), todo('feat/a', ['worktree', 'env', 'meta', 'on_create'])]

test('each event moves the progress: a phase under way, ended or failed, the output of a hook', () => {
  let progress: Progress<'create'> = { units }
  for (const event of [
    line({ type: 'phase.started', subject: 'main', phase: 'fast_forward' }),
    line({ type: 'phase.finished', subject: 'main', phase: 'fast_forward', failed: false }, '2026-10-25T12:00:01.5Z'),
    line({ type: 'phase.started', subject: 'feat/a', phase: 'worktree' }),
    line({ type: 'phase.output', subject: 'feat/a', line: 'Packages: +1204' }),
    line({ type: 'operation.started', command: 'create', surface: 'api' }),
  ])
    progress = advance(progress, event)

  const a = progress.units[1]
  expect(progress.units[0]?.phases[0]).toEqual({
    phase: 'fast_forward',
    state: 'done',
    startedAt: '2026-10-25T12:00:00Z',
    endedAt: '2026-10-25T12:00:01.5Z',
  })
  expect(current(progress)).toMatchObject({
    subject: 'feat/a',
    run: { phase: 'worktree', state: 'now' },
    index: 1,
    total: 5,
  })
  expect(a?.lines).toEqual(['Packages: +1204'])
  expect(doneOf(progress)).toEqual({ done: 1, total: 5 })
})

test('an event of a subject or a phase the plan did not foresee is added, not dropped', () => {
  const progress = advance({ units }, line({ type: 'phase.started', subject: 'feat/b', phase: 'ports' }))

  expect(progress.units.at(-1)).toEqual({
    subject: 'feat/b',
    phases: [{ phase: 'ports', state: 'now', startedAt: '2026-10-25T12:00:00Z' }],
    lines: [],
  })
})

test('a phase is timed from a second up, the quick ones left blank', () => {
  expect(
    seconds({ phase: 'env', state: 'done', startedAt: '2026-10-25T12:00:00Z', endedAt: '2026-10-25T12:00:00.2Z' }),
  ).toBe('')
  expect(
    seconds({ phase: 'on_create', state: 'done', startedAt: '2026-10-25T12:00:00Z', endedAt: '2026-10-25T12:00:08Z' }),
  ).toBe('8.0 s')
  expect(seconds({ phase: 'on_create', state: 'now', startedAt: '2026-10-25T12:00:00Z' })).toBe('')
})

test('the whole run is timed from its first phase started to its last ended, across branches', () => {
  const run = (startedAt: string, endedAt: string) => ({
    phase: 'env' as const,
    state: 'done' as const,
    startedAt,
    endedAt,
  })
  expect(
    took({
      units: [
        { subject: 'feat/a', phases: [run('2026-10-25T12:00:01Z', '2026-10-25T12:00:03Z')], lines: [] },
        { subject: 'feat/b', phases: [run('2026-10-25T12:00:03Z', '2026-10-25T12:00:09Z')], lines: [] },
      ],
    }),
  ).toBe('8.0 s')
  expect(took({ units: [] })).toBe('')
})
