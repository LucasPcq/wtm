import { expect, test } from 'vitest'
import { howItRuns, numberedOf, sharedOf, unitsOf } from '@/features/worktrees/run-up/utils/plan'
import { runUpPlanOf } from '@/test/run-up'

const two = runUpPlanOf(['feat/search', 'feat/search-docs'])

test('a shared service is said once, with the namespace each worktree gets in it', () => {
  expect(sharedOf(two)).toEqual([
    { name: 'postgres', holders: 2, namespaces: ['acme_feat_search', 'acme_feat_search_docs'] },
  ])
})

test("the jobs a worktree runs itself are numbered across the run, a shared one's join left out", () => {
  expect(
    numberedOf(two).map(({ worktree, jobs }) => [worktree, jobs.map((job) => `${job.number} ${job.name}`)]),
  ).toEqual([
    ['feat/search', ['1 migrate', '2 web', '3 api']],
    ['feat/search-docs', ['4 migrate', '5 web', '6 api']],
  ])
})

test('how it runs says the worktrees start side by side, what runs first and the namespaces', () => {
  expect(howItRuns(two)).toEqual([
    'The 2 worktrees start side by side; in each, migrate runs first.',
    'Each gets a namespace of its own in the shared postgres.',
    'They keep running when this page closes; run down stops them.',
  ])
  expect(howItRuns(runUpPlanOf(['feat/search']))[0]).toBe('It gets a namespace of its own in the shared postgres.')
})

test('each job, joins included, is a step of its worktree', () => {
  expect(unitsOf(two)[0]?.phases.map((run) => run.job)).toEqual(['migrate', 'postgres', 'web', 'api'])
})
