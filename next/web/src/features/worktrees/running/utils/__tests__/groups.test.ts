import { expect, test } from 'vitest'
import { groupLine, groupsOf } from '@/features/worktrees/running/utils/groups'
import { advance, jobsTodo, type Progress } from '@/features/worktrees/utils/progress'
import { listDetail, read } from '@/test/fixtures'
import { job, runningWorktrees, runUpPlanOf } from '@/test/run-up'

const listed = read('list', runningWorktrees)

test('only the worktrees that hold jobs are groups, a crash first and the counts on their line', () => {
  const groups = groupsOf({ read: listed, progress: undefined, plan: undefined })

  const [billing] = groups
  expect(groups.map((group) => group.worktree)).toEqual(['feat/billing'])
  expect(billing?.rows.map((row) => row.name)).toEqual(['api', 'web'])
  expect(billing && groupLine(billing)).toBe('profile dev · 1 up · 1 crashed')
})

test('a worktree a start brings up is a group before the read lists its jobs, each job saying where it stands', () => {
  const plan = runUpPlanOf(['feat/search'])
  const progress = advance<'run-up'>(
    { units: [jobsTodo('feat/search', ['migrate', 'postgres', 'web', 'api'])] },
    { type: 'phase.started', subject: 'feat/search', phase: 'job', job: 'migrate', ts: '2026-10-25T12:00:00Z' },
  )

  const search = groupsOf({ read: listed, progress, plan }).find((group) => group.worktree === 'feat/search')

  expect(search?.rows).toEqual([
    { name: 'migrate', said: 'starting' },
    { name: 'web', said: 'waits for migrate' },
    { name: 'api', said: 'waits for migrate' },
  ])
  expect(search && groupLine(search)).toBe('profile dev · starting')
})

// A start in a worktree whose last run was stopped showed the stopped rows for its whole run.
test('while a start runs, a job it has not brought up yet says where it stands over its stale row', () => {
  const stopped = read('list', [
    listDetail('feat/search', { jobs: [job('web', { state: 'stopped' }), job('api', { state: 'stopped' })] }),
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
  const plan = runUpPlanOf(['feat/search'])
  const fresh = read('list', [listDetail('feat/search', { jobs: [job('web'), job('api', { state: 'stopped' })] })])

  expect(groupsOf({ read: fresh, progress, plan })[0]?.rows).toEqual([
    { name: 'web', job: job('web') },
    { name: 'api', said: 'up' },
  ])
})

// After a start and a run down of both worktrees, the page kept their groups and said jobs ran there.
test('a worktree whose jobs are all stopped or exited is no group', () => {
  const down = read('list', [
    listDetail('feat/search', { jobs: [job('web', { state: 'stopped' }), job('migrate', { state: 'exited' })] }),
  ])

  expect(groupsOf({ read: down, progress: undefined, plan: undefined })).toEqual([])
})
