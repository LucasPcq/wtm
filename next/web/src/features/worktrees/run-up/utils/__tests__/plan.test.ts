import { expect, test } from 'vitest'
import { countOf, howItRuns, orderedOf, sharedOf, unitsOf } from '@/features/worktrees/run-up/utils/plan'
import { advance } from '@/features/worktrees/utils/progress'
import { runUpPlanOf } from '@/test/run-up'

const two = runUpPlanOf(['feat/search', 'feat/search-docs'])

test('a shared service is said once, with the namespace each worktree gets in it', () => {
  expect(sharedOf(two)).toEqual([
    { name: 'postgres', holders: 2, namespaces: ['acme_feat_search', 'acme_feat_search_docs'] },
  ])
})

test("the jobs that start are numbered across the run, a shared one's join and a job up already left unnumbered", () => {
  const said = (plan: typeof two) =>
    orderedOf(plan).map(({ worktree, jobs }) => [worktree, jobs.map((job) => `${job.number ?? '-'} ${job.name}`)])

  expect(said(two)).toEqual([
    ['feat/search', ['1 migrate', '2 web', '3 api']],
    ['feat/search-docs', ['4 migrate', '5 web', '6 api']],
  ])
  expect(said(runUpPlanOf(['feat/billing', 'feat/search']))).toEqual([
    ['feat/billing', ['1 migrate', '- web', '- api']],
    ['feat/search', ['2 migrate', '3 web', '4 api']],
  ])
})

test('how it runs says the worktrees start side by side, what runs first, the namespaces and who keeps running', () => {
  expect(howItRuns(two, ['feat/billing']).map(({ text }) => text)).toEqual([
    'The 2 worktrees start side by side; in each, migrate runs first.',
    'Each gets a namespace of its own in the shared postgres.',
    'The jobs of feat/billing keep running.',
    'They keep running when this page closes; Stop jobs ends them.',
  ])
  expect(howItRuns(runUpPlanOf(['feat/search']), [])[0]?.text).toBe(
    'It gets a namespace of its own in the shared postgres.',
  )
})

test('each job, joins included, is a step of its worktree', () => {
  expect(unitsOf(two)[0]?.phases.map((run) => run.job)).toEqual(['migrate', 'postgres', 'web', 'api'])
})

test('a start counts each worktree its own jobs and each shared service once, up once every worktree joined it', () => {
  const join = (subject: string) =>
    ({ type: 'phase.finished', subject, phase: 'job', job: 'postgres', failed: false, ts: '' }) as const
  const once = advance<'run-up'>({ units: unitsOf(two) }, join('feat/search'))

  expect(countOf(unitsOf(two), two)).toEqual({ done: 0, total: 7 })
  expect(countOf(once.units, two).done).toBe(0)
  expect(countOf(advance(once, join('feat/search-docs')).units, two).done).toBe(1)
})
