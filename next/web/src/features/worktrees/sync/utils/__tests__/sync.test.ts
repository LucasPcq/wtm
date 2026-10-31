import { expect, test } from 'vitest'
import type { SyncPlan } from '@/api/types.gen'
import { requestOf, rootOf } from '@/features/worktrees/sync/utils/plan'
import { searchedStack, stackOf } from '@/features/worktrees/sync/utils/stack'
import { syncPlanOf } from '@/test/sync'

const names = (nodes: ReturnType<typeof stackOf>): unknown[] =>
  nodes.map((node) => (node.children.length > 0 ? [node.name, names(node.children)] : node.name))

test('the stack reads as the tree it is: each branch rebased onto a root, each worktree under its parent', () => {
  const plan = syncPlanOf({ answers: [{ path: 'all', values: ['true'] }] })

  expect(names(stackOf(plan))).toEqual([
    ['main', [['feat/billing', ['feat/billing-ui']], 'fix/login', ['feat/search', ['feat/search-docs']]]],
  ])
})

test('two worktrees whose parents lead back to each other are both roots, never lost', () => {
  const plan = syncPlanOf({ answers: [{ path: 'all', values: ['true'] }] })
  const looped: Record<string, string> = { 'feat/search': 'feat/search-docs', 'feat/search-docs': 'feat/search' }
  const cycled: SyncPlan = {
    ...plan,
    worktrees: (plan.worktrees ?? []).map((rebase) => {
      const parent = looped[rebase.worktree]
      return parent ? { ...rebase, parent, standing: 'in_cycle' } : rebase
    }),
  }

  expect(names(stackOf(cycled))).toContain('feat/search')
  expect(names(stackOf(cycled))).toContain('feat/search-docs')
})

test('a search keeps the branches above a match, so the stack still reads', () => {
  const plan = syncPlanOf({ answers: [{ path: 'all', values: ['true'] }] })

  expect(names(searchedStack(stackOf(plan), 'billing-ui'))).toEqual([['main', [['feat/billing', ['feat/billing-ui']]]]])
})

test('a descendant follows the first of its parents the person named', () => {
  const plan = syncPlanOf({ answers: [{ path: 'worktree', values: ['feat/billing'] }] })

  expect(rootOf(plan.worktrees, 'feat/billing-ui')).toBe('feat/billing')
})

test('a switch is sent only off its default, and force only when a worktree with a change is checked', () => {
  expect(
    requestOf({
      chosen: false,
      named: ['feat/billing'],
      forced: false,
      switches: { descendants: true, push: false, keep_conflict: false, ff_parents: true },
    }),
  ).toEqual([{ path: 'worktree', values: ['feat/billing'] }])
  expect(
    requestOf({
      chosen: true,
      named: [],
      forced: true,
      switches: { descendants: false, push: true, keep_conflict: true, ff_parents: false },
    }),
  ).toEqual([
    { path: 'force', values: ['true'] },
    { path: 'descendants', values: ['false'] },
    { path: 'ff_parents', values: ['false'] },
    { path: 'push', values: ['true'] },
    { path: 'keep_conflict', values: ['true'] },
  ])
})
