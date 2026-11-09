import { expect, test } from 'vitest'
import { groupsOf } from '@/features/worktrees/running/utils/groups'
import { pinOf, visible } from '@/features/worktrees/running/utils/visible'
import { calm } from '@/features/worktrees/store/ui-store'
import { psGroup, psJob, read } from '@/test/fixtures'

const groups = groupsOf({
  read: read('run-ps', [
    psGroup('feat/billing', [
      psJob('worker', { state: 'crashed', exit_code: 1 }),
      psJob('web'),
      psJob('migrate', { kind: 'task', state: 'exited' }),
    ]),
    psGroup('fix/login', [psJob('web')]),
    psGroup('feat/old', [psJob('api', { state: 'stopped' })]),
  ]),
  progress: undefined,
  plan: undefined,
})
const none = () => false
const shown = (over: Partial<Parameters<typeof visible>[0]> = {}) =>
  visible({ groups, filter: calm, search: '', pinned: none, quiet: none, ...over })

test('the calm default lists what runs and crashed, a group closing on its ended jobs, one with nothing else gone', () => {
  const { groups: listed, counts, hidden } = shown()

  expect(listed.map((group) => group.worktree)).toEqual(['feat/billing', 'fix/login'])
  expect(listed[0]?.ended.map((job) => job.name)).toEqual(['migrate'])
  expect(listed[0]?.endedShown).toEqual([])
  expect(counts).toEqual({ all: 5, running: 2, crashed: 1 })
  expect(hidden, 'the ended jobs are said by their group, not by the filter').toBe(0)
})

test('a chip keeps its state alone and says how many jobs the filter hides', () => {
  const { groups: listed, hidden } = shown({ filter: { ...calm, chip: 'crashed' } })

  expect(listed.map((group) => group.rows.map((row) => row.name))).toEqual([['worker']])
  expect(hidden).toBe(4)
})

test('a pinned job leaves its group for the top', () => {
  const { pinned, groups: listed } = shown({ pinned: (id) => id === pinOf('fix/login', 'web') })

  expect(pinned.map(({ group, row }) => `${group.worktree} ${row.name}`)).toEqual(['fix/login web'])
  expect(listed.map((group) => group.worktree)).toEqual(['feat/billing'])
})

test('a group asked to show its ended jobs lists them, and the Stopped and exited state lists them everywhere', () => {
  expect(
    shown({ quiet: (key) => key === '/acme:/acme.trees/feat-billing' }).groups[0]?.endedShown.map((job) => job.name),
  ).toEqual(['migrate'])

  const everything = shown({ filter: { ...calm, states: ['running', 'crashed', 'ended'] } })
  expect(everything.groups.map((group) => group.worktree)).toEqual(['feat/billing', 'fix/login', 'feat/old'])
  expect(everything.groups[2]?.endedShown.map((job) => job.name)).toEqual(['api'])
})

// A worktree on a branch called shared took the shared services' key: folding one folded both, and the filter took one
// for the other.
test('a group is keyed by its repository and path, so a branch called shared is its own group', () => {
  const tricky = groupsOf({
    read: read('run-ps', [
      psGroup('shared', [psJob('web')]),
      psGroup('main', [psJob('db')], { shared: true, profile: '' }),
    ]),
    progress: undefined,
    plan: undefined,
  })
  const [branchShared, services] = tricky

  expect(new Set(tricky.map((group) => group.key)).size).toBe(2)
  expect(services?.key).toBe('shared')
  const only = (worktree: string) =>
    visible({
      groups: tricky,
      filter: { ...calm, worktree },
      search: '',
      pinned: none,
      quiet: none,
    }).groups.map((group) => group.key)
  expect(only('shared')).toEqual(['shared'])
  expect(only(branchShared?.key ?? '')).toEqual([branchShared?.key])
})

test('a pinned job that ended stays on top, and a job starting counts as running', () => {
  const { pinned } = shown({ pinned: (id) => id === pinOf('feat/billing', 'migrate') })
  expect(pinned.map(({ row }) => `${row.name} ${row.job.state}`)).toEqual(['migrate exited'])

  const starting = groupsOf({
    read: read('run-ps', [psGroup('feat/x', [psJob('web', { state: 'starting' })])]),
    progress: undefined,
    plan: undefined,
  })
  const { counts, states } = visible({
    groups: starting,
    filter: calm,
    search: '',
    pinned: none,
    quiet: none,
  })
  expect(counts.running).toBe(1)
  expect(states.running).toBe(1)
})
