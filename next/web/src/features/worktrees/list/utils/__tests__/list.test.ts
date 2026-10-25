import { expect, test } from 'vitest'
import { rowsOf } from '@/features/worktrees/list/utils/rows'
import { listDetail, read } from '@/test/fixtures'

const rows = (live: Parameters<typeof rowsOf>[0]['live'], github?: Parameters<typeof rowsOf>[0]['github']) =>
  rowsOf({ live, github, readAt: 0, underWay: {} })

test('a row takes its pull request and whether it merged from the last GitHub read, by subject', () => {
  const pr = { number: 3, url: 'https://github.com/acme/acme/pull/3', state: 'merged' as const }
  const live = read('list', [listDetail('feat/a'), listDetail('feat/new')])
  const github = read('list', [listDetail('feat/a', { pr, merged: true })])

  const [a, created] = rows(live, github)

  expect(a?.pr).toEqual(pr)
  expect(a?.merged).toBe(true)
  expect(created?.pr, 'a worktree created since GitHub was asked has no pull request known yet').toBeUndefined()
  expect(rows(live)[0]?.merged).toBeUndefined()
})

test('a row is merged when GitHub says so over a changed worktree, as the one rule over two reads says', () => {
  const live = read('list', [listDetail('feat/a', { condition: 'changed', dirty: true })])
  const github = read('list', [listDetail('feat/a', { condition: 'merged', merged: true })])

  expect(rows(live, github)[0]?.condition).toBe('merged')
  expect(rows(live)[0]?.condition).toBe('changed')
})
