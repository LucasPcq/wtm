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
  visible({ groups, filter: calm, search: '', here: undefined, pinned: none, quiet: none, ...over })

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
  expect(shown({ quiet: (key) => key === 'feat/billing' }).groups[0]?.endedShown.map((job) => job.name)).toEqual([
    'migrate',
  ])

  const everything = shown({ filter: { ...calm, states: ['running', 'crashed', 'ended'] } })
  expect(everything.groups.map((group) => group.worktree)).toEqual(['feat/billing', 'fix/login', 'feat/old'])
  expect(everything.groups[2]?.endedShown.map((job) => job.name)).toEqual(['api'])
})

test('this worktree is the one the page was opened from, by its path', () => {
  const { groups: listed } = shown({ filter: { ...calm, worktree: 'here' }, here: '/acme.trees/fix-login' })

  expect(listed.map((group) => group.worktree)).toEqual(['fix/login'])
})
