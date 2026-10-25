import { expect, test } from 'vitest'
import type { Read } from '@/api/queries/commands'
import type { DomainFollowUp, TreeDetail } from '@/api/types.gen'
import { nodesOf } from './nodes'

const action = (command: string, branch: string): DomainFollowUp => ({
  command,
  answers: [{ path: 'worktree', values: [branch] }],
  code: `tree.${command}`,
  message: command,
})

const node = (branch: string, parent: string | null, over: Partial<TreeDetail> = {}): TreeDetail => ({
  actions: [],
  ahead: 0,
  behind: 0,
  branch,
  children: [],
  condition: 'clean',
  cycle: false,
  dirty: false,
  locked: false,
  main: false,
  missing: false,
  needs_sync: false,
  origin: null,
  parent,
  path: `/repo/.trees/${branch}`,
  pr: null,
  rebasing: false,
  services: [],
  virtual: false,
  ...over,
})

const read = (details: TreeDetail[]): Read<'tree'> => ({
  v: 2,
  ok: true,
  command: 'tree',
  follow_ups: [],
  warnings: [],
  items: details.map((detail) => ({ subject: detail.branch, status: 'done', detail })),
})

test('each node sits under its parent, a node whose parent is not in the read is a root', () => {
  const roots = nodesOf(
    read([node('main', null), node('feat/a', 'main'), node('feat/a-ui', 'feat/a'), node('x', 'gone')]),
    undefined,
  )

  expect(roots.map((root) => root.branch)).toEqual(['main', 'x'])
  expect(roots[0]?.children[0]?.children.map((child) => child.branch)).toEqual(['feat/a-ui'])
})

test('a worktree GitHub says is merged shows merged and its clean, unless the live read says more urgent', () => {
  const live = read([
    node('feat/a', null, { actions: [action('sync', 'feat/a')] }),
    node('feat/b', null, { condition: 'rebasing', rebasing: true, actions: [action('go', 'feat/b')] }),
    node('feat/c', null),
  ])
  const github = read([
    node('feat/a', null, { condition: 'merged', actions: [action('clean', 'feat/a')] }),
    node('feat/b', null, { condition: 'rebasing', actions: [action('go', 'feat/b')] }),
  ])

  const [a, b, c] = nodesOf(live, github)

  expect([a?.condition, a?.action?.command]).toEqual(['merged', 'clean'])
  expect([b?.condition, b?.action?.command]).toEqual(['rebasing', 'go'])
  expect([c?.condition, c?.action]).toEqual(['clean', undefined])
})
