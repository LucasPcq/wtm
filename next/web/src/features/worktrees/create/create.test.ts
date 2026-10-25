import { expect, test } from 'vitest'
import type { OperationLine } from '@/api/queries/operations'
import type { CreatePlan, DomainField, DomainForm } from '@/api/types.gen'
import { advance, current, doneOf, planned } from './progress'
import { stepsOf } from './steps'

const text = (path: string, section?: string, skip?: string): DomainField => ({
  kind: 'text',
  path,
  flag: `--${path}`,
  label: path,
  help: '',
  value: '',
  ...(section && { section }),
  ...(skip && { skip }),
})

test('each section is a step, a field of no section goes with the last, a skipped one is listed apart', () => {
  const form: DomainForm = {
    sections: [
      { key: 'branch', title: 'Branch' },
      { key: 'environment', title: 'Environment' },
    ],
    fields: [
      text('branches', 'branch'),
      text('env_from', 'environment', 'the branch exists'),
      text('if_not_exists', undefined, 'set by its flag only'),
    ],
    problems: [],
  }

  const [branch, environment] = stepsOf(form)

  expect(branch?.asked.map((f) => f.path)).toEqual(['branches'])
  expect(environment?.asked).toEqual([])
  expect(environment?.skipped.map((f) => f.path)).toEqual(['env_from', 'if_not_exists'])
  expect(stepsOf({ ...form, sections: [] }).map((step) => step.fields.length)).toEqual([3])
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
