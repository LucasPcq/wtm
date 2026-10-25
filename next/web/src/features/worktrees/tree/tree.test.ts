import { expect, test } from 'vitest'
import { nodesOf } from '@/features/worktrees/tree/nodes'
import { followUp as action, read as readOf, treeDetail } from '@/test/fixtures'

const node = (branch: string, parent: string | null, over: Parameters<typeof treeDetail>[1] = {}) =>
  treeDetail(branch, { parent, ...over })

const read = (details: ReturnType<typeof treeDetail>[]) => readOf('tree', details)

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
