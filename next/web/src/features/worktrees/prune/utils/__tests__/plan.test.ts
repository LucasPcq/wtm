import { expect, test } from 'vitest'
import { requestOf } from '@/features/worktrees/prune/utils/plan'
import { candidates, gone, merged } from '@/test/prune'

const both = candidates
const noFetch = { path: 'no_fetch', values: ['true'] }

test("the safe candidates as checked on opening are prune's own pick: it names none, so prune keeps the unsafe", () => {
  expect(requestOf({ named: [], candidates: both, going: [merged], reparent: false })).toEqual([noFetch])
})

test('any other pick names the checked ones, forced only when one of them is unsafe', () => {
  expect(requestOf({ named: [], candidates: both, going: [merged, gone], reparent: true })).toEqual([
    { path: 'worktree', values: ['feat/billing', 'feat/search-docs'] },
    { path: 'force', values: ['true'] },
    { path: 'reparent_children', values: ['true'] },
    noFetch,
  ])
  expect(requestOf({ named: [], candidates: [merged], going: [], reparent: false })).toEqual([
    { path: 'worktree', values: [] },
    noFetch,
  ])
})

test('a dialog opened on a worktree always names it', () => {
  expect(requestOf({ named: ['feat/billing'], candidates: [merged], going: [merged], reparent: false })).toEqual([
    { path: 'worktree', values: ['feat/billing'] },
    noFetch,
  ])
})
