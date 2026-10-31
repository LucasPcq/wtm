import { expect, test } from 'vitest'
import { groupLine, groupsOf } from '@/features/worktrees/running/utils/groups'
import { advance, jobsTodo } from '@/features/worktrees/utils/progress'
import { read } from '@/test/fixtures'
import { runningWorktrees, runUpPlanOf } from '@/test/run-up'

const listed = read('list', runningWorktrees)

test('only the worktrees that hold jobs are groups, a crash first and the counts on their line', () => {
  const groups = groupsOf({ read: listed, progress: undefined, plan: undefined })

  const [billing] = groups
  expect(groups.map((group) => group.worktree)).toEqual(['feat/billing'])
  expect(billing?.rows.map((row) => row.name)).toEqual(['api', 'web', 'migrate'])
  expect(billing && groupLine(billing)).toBe('profile dev · 1 up · 1 crashed · 1 exited')
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
