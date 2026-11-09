import { expect, test } from 'vitest'
import { groupsOf } from '@/features/worktrees/running/utils/groups'
import { advance, jobsTodo, type Progress } from '@/features/worktrees/utils/progress'
import { psGroup, psJob, read } from '@/test/fixtures'
import { runUpPlanOf } from '@/test/run-up'

const billing = psGroup('feat/billing', [
  psJob('api', { state: 'crashed', exit_code: 1 }),
  psJob('web'),
  psJob('migrate', { kind: 'task', state: 'exited', exit_code: 0 }),
])

test('each group of the read keeps its order, its ended jobs set apart for its quiet row', () => {
  const [group] = groupsOf({ read: read('run-ps', [billing]), progress: undefined, plan: undefined })

  expect(group?.rows.map((row) => row.name)).toEqual(['api', 'web'])
  expect(group?.ended.map((job) => job.name)).toEqual(['migrate'])
  expect(group?.path).toBe('/acme.trees/feat-billing')
})

test('a worktree a start brings up is a group before the read lists its jobs, each job saying where it stands', () => {
  const plan = runUpPlanOf(['feat/search'])
  const progress = advance<'run-up'>(
    { units: [jobsTodo('feat/search', ['migrate', 'postgres', 'web', 'api'])] },
    { type: 'phase.started', subject: 'feat/search', phase: 'job', job: 'migrate', ts: '2026-10-25T12:00:00Z' },
  )

  const search = groupsOf({ read: read('run-ps', [billing]), progress, plan }).find(
    (group) => group.worktree === 'feat/search',
  )

  expect(search?.rows).toEqual([
    { name: 'migrate', said: 'starting' },
    { name: 'web', said: 'waits for migrate' },
    { name: 'api', said: 'waits for migrate' },
  ])
  expect(search?.starting).toBe(true)
  expect(search?.key, 'no branch, nor the Pinned fold, spells a fresh key').toBe('fresh:feat/search')
})

// A start in a worktree whose last run was stopped showed the stopped rows for its whole run.
test('while a start runs, a job it has not brought up yet says where it stands over its stale row', () => {
  const stopped = read('run-ps', [
    psGroup('feat/search', [psJob('web', { state: 'stopped' }), psJob('api', { state: 'stopped' })]),
  ])
  const progress = advance<'run-up'>(
    { units: [jobsTodo('feat/search', ['migrate', 'postgres', 'web', 'api'])] },
    { type: 'phase.started', subject: 'feat/search', phase: 'job', job: 'migrate', ts: '2026-10-25T12:00:00Z' },
  )

  const [search] = groupsOf({ read: stopped, progress, plan: runUpPlanOf(['feat/search']) })

  expect(search?.rows).toEqual([
    { name: 'migrate', said: 'starting' },
    { name: 'web', said: 'waits for migrate' },
    { name: 'api', said: 'waits for migrate' },
  ])
  expect(search?.ended).toEqual([])
})

// The new groups vanished for a moment at the end of a start, before the read after it landed.
test('what a start brought up stays greyed until a read lists it', () => {
  let progress: Progress<'run-up'> = { units: [jobsTodo('feat/search', ['migrate', 'postgres', 'web', 'api'])] }
  for (const name of ['migrate', 'postgres', 'web', 'api'])
    progress = advance(progress, {
      type: 'phase.finished',
      subject: 'feat/search',
      phase: 'job',
      job: name,
      failed: false,
      ts: '2026-10-25T12:00:00Z',
    })
  const fresh = read('run-ps', [psGroup('feat/search', [psJob('web'), psJob('api', { state: 'stopped' })])])

  expect(groupsOf({ read: fresh, progress, plan: runUpPlanOf(['feat/search']) })[0]?.rows).toEqual([
    { name: 'web', job: psJob('web') },
    { name: 'api', said: 'up' },
  ])
})
