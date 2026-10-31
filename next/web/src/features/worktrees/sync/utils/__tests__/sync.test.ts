import { expect, test } from 'vitest'
import type { SyncPlan } from '@/api/types.gen'
import { requestOf, rootOf } from '@/features/worktrees/sync/utils/plan'
import { ranWell, recapOf, summaryOf } from '@/features/worktrees/sync/utils/recap'
import { searchedStack, stackOf } from '@/features/worktrees/sync/utils/stack'
import type { Progress } from '@/features/worktrees/utils/progress'
import { envelope } from '@/test/fixtures'
import { syncPlanOf, type World } from '@/test/sync'

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

const ran = (
  call: Parameters<typeof syncPlanOf>[0],
  items: {
    subject: string
    status: string
    reason?: string
    error?: { code: string; params?: Record<string, string> }
    detail?: { pushed: boolean }
  }[],
  ok = true,
  world: World = {},
) =>
  ({
    units: [],
    envelope: { ...envelope('sync', []), ok, items, plan: syncPlanOf(call, world) },
  }) as unknown as Progress<'sync'>

test('one sentence ends a run, in the dialog as in the toast: rebases counted, a push alone said so', () => {
  const pushOnly = {
    answers: [
      { path: 'worktree', values: ['feat/billing', 'feat/billing-ui'] },
      { path: 'descendants', values: ['false'] },
      { path: 'push', values: ['true'] },
    ],
  }
  const synced = ['feat/billing', 'feat/billing-ui']
  const pushed = synced.map((subject) => ({ subject, status: 'done', detail: { pushed: true } }))
  expect(summaryOf(ran(pushOnly, pushed, true, { synced }))).toBe('2 pushed.')

  const named = { answers: [{ path: 'worktree', values: ['feat/billing'] }] }
  const refused = [
    { subject: 'main', status: 'done' },
    { subject: 'feat/billing', status: 'failed', error: { code: 'sync.push_failed' } },
    { subject: 'feat/billing-ui', status: 'skipped' },
  ]
  expect(summaryOf(ran(named, refused, false))).toBe('1 of 2 synced.')
  expect(summaryOf(ran(named, [{ subject: 'feat/billing', status: 'cancelled' }], false))).toBe(
    'Cancelled: 0 of 2 synced.',
  )
})

test('a run whose one miss is a refused fast-forward first ran well; a cancel shows the branch it never reached', () => {
  const named = { answers: [{ path: 'worktree', values: ['feat/billing'] }] }
  const forwardRefused = [
    { subject: 'main', status: 'skipped', error: { code: 'sync.ff_failed', params: { children: 'feat/billing' } } },
    { subject: 'feat/billing', status: 'done', detail: { pushed: false } },
    { subject: 'feat/billing-ui', status: 'done', detail: { pushed: false } },
  ]
  expect(ranWell(ran(named, forwardRefused).envelope)).toBe(true)

  const cancelled = ['main', 'feat/billing', 'feat/billing-ui'].map((subject) => ({
    subject,
    status: 'skipped',
    reason: 'interrupted',
  }))
  const failed = recapOf(ran(named, cancelled, false), () => {}).groups.find((group) => group.kind === 'failed')
  expect(failed?.rows.map((row) => row.title)).toEqual(['main', 'feat/billing', 'feat/billing-ui'])
})
