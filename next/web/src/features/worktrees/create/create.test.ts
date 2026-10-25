import { expect, test } from 'vitest'
import type { OperationLine } from '@/api/queries/operations'
import type { CreatePlan, DomainField, DomainForm, DomainSkip } from '@/api/types.gen'
import { advance, current, doneOf, planned, seconds, took } from './progress'
import { stepsOf } from './steps'

const text = (path: string, skip?: DomainSkip): DomainField => ({
  kind: 'text',
  path,
  flag: `--${path}`,
  label: path,
  help: '',
  value: '',
  ...(skip && { skip }),
})

test('each question goes in its step, a skipped one is listed apart, one the build does not know goes last', () => {
  const form: DomainForm = {
    fields: [text('branches'), text('env_from', 'exists'), text('if_not_exists', 'flag_only'), text('new_one')],
    problems: [],
  }

  const [branch, environment] = stepsOf(form)

  expect(branch?.asked.map((f) => f.path)).toEqual(['branches'])
  expect(environment?.asked.map((f) => f.path)).toEqual(['new_one'])
  expect(environment?.skipped.map((f) => f.path)).toEqual(['env_from', 'if_not_exists'])
})

const plan: CreatePlan = {
  branches: [
    { branch: 'feat/a', path: '/t/feat-a', action: 'create', phases: ['worktree', 'env', 'meta', 'on_create'] },
    { branch: 'feat/open', path: '/t/open', action: 'unchanged', phases: [] },
  ],
  fast_forward: ['main'],
  creates: 1,
  env_from: 'example',
}

const line = (event: Record<string, unknown>, ts = '2026-10-25T12:00:00Z') =>
  ({ ts, ...event }) as unknown as OperationLine

test('the progress starts from the plan: what each branch will run, an open one left out', () => {
  const progress = planned(plan)

  expect(progress.units.map(([subject]) => subject)).toEqual(['main', 'feat/a'])
  expect(doneOf(progress)).toEqual({ done: 0, total: 5 })
  expect(current(progress)?.run).toEqual({ phase: 'fast_forward', state: 'todo' })
})

// A branch checked out as it is, behind origin and fast-forwarded first, was two units: its phases counted twice.
test('a branch fast-forwarded then opened is one unit, its fast-forward first', () => {
  const progress = planned({
    branches: [{ branch: 'feat/x', path: '/x', action: 'check_out', phases: ['worktree', 'env'] }],
    creates: 1,
    env_from: 'example',
    fast_forward: ['feat/x'],
  })

  expect(progress.units.map(([subject]) => subject)).toEqual(['feat/x'])
  expect(progress.units[0]?.[1].phases.map((run) => run.phase)).toEqual(['fast_forward', 'worktree', 'env'])
  expect(doneOf(progress).total).toBe(3)
})

test('each event moves the progress: a phase under way, ended or failed, the output of a hook', () => {
  let progress = planned(plan)
  for (const event of [
    line({ type: 'phase.started', subject: 'main', phase: 'fast_forward' }),
    line({ type: 'phase.finished', subject: 'main', phase: 'fast_forward', failed: false }, '2026-10-25T12:00:01.5Z'),
    line({ type: 'phase.started', subject: 'feat/a', phase: 'worktree' }),
    line({ type: 'phase.output', subject: 'feat/a', line: 'Packages: +1204' }),
    line({ type: 'operation.started', command: 'create', surface: 'api' }),
  ])
    progress = advance(progress, event)

  const [, a] = progress.units[1] ?? []
  expect(progress.units[0]?.[1].phases[0]).toEqual({
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
  const progress = advance(planned(plan), line({ type: 'phase.started', subject: 'feat/b', phase: 'ports' }))

  expect(progress.units.at(-1)).toEqual([
    'feat/b',
    { phases: [{ phase: 'ports', state: 'now', startedAt: '2026-10-25T12:00:00Z' }], lines: [] },
  ])
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
        ['feat/a', { phases: [run('2026-10-25T12:00:01Z', '2026-10-25T12:00:03Z')], lines: [] }],
        ['feat/b', { phases: [run('2026-10-25T12:00:03Z', '2026-10-25T12:00:09Z')], lines: [] }],
      ],
    }),
  ).toBe('8.0 s')
  expect(took({ units: [] })).toBe('')
})
