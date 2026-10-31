import { expect, test } from 'vitest'
import type { CreatePlan, DomainField, DomainForm, DomainSkip } from '@/api/types.gen'
import { checkoutPlanned, planned } from '@/features/worktrees/create/utils/progress'
import { createSteps, stepsBy } from '@/features/worktrees/create/utils/steps'
import { current, doneOf } from '@/features/worktrees/utils/progress'

const text = (path: string, skip?: DomainSkip): DomainField => ({
  kind: 'text',
  path,
  value: '',
  ...(skip && { skip }),
})

test('each question goes in its step, a skipped one is listed apart, a field no question of create answers is left out', () => {
  const form: DomainForm = {
    fields: [text('branches'), text('env_from', 'exists'), text('if_not_exists', 'flag_only'), text('new_one')],
    problems: [],
  }

  const [branch, environment] = stepsBy(form, createSteps)

  expect(branch?.asked.map((f) => f.path)).toEqual(['branches'])
  expect(environment?.asked.map((f) => f.path)).toEqual([])
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

test('the progress starts from the plan: what each branch will run, an open one left out', () => {
  const progress = { units: planned(plan) }

  expect(progress.units.map((unit) => unit.subject)).toEqual(['main', 'feat/a'])
  expect(doneOf(progress)).toEqual({ done: 0, total: 5 })
  expect(current(progress)?.run).toEqual({ phase: 'fast_forward', state: 'todo' })
})

// A branch checked out as it is, behind origin and fast-forwarded first, was two units: its phases counted twice.
test('a branch fast-forwarded then opened is one unit, its fast-forward first', () => {
  const progress = {
    units: planned({ branches: [{ branch: 'feat/x', phases: ['worktree', 'env'] }], fast_forward: ['feat/x'] }),
  }

  expect(progress.units.map((unit) => unit.subject)).toEqual(['feat/x'])
  expect(progress.units[0]?.phases.map((run) => run.phase)).toEqual(['fast_forward', 'worktree', 'env'])
  expect(doneOf(progress).total).toBe(3)
})

test("a pull request's branch is one unit, fast-forwarded first when the plan says so", () => {
  const units = checkoutPlanned({
    action: 'check_out',
    env_from: 'example',
    from: 'main',
    path: '/t/fix-rate-limit',
    phases: ['worktree', 'env'],
    fast_forward: ['fix/rate-limit'],
    pull_request: { number: 231, branch: 'fix/rate-limit', state: 'open', url: 'u' },
  })

  expect(units.map((unit) => [unit.subject, unit.phases.map((run) => run.phase)])).toEqual([
    ['fix/rate-limit', ['fast_forward', 'worktree', 'env']],
  ])
})
