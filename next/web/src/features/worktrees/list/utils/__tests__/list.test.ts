import { expect, test } from 'vitest'
import { countsOf, matches, searched } from '@/features/worktrees/list/utils/filters'
import { type Row, rowsOf } from '@/features/worktrees/list/utils/rows'
import { sentence } from '@/features/worktrees/shared/utils/sentence'
import { listDetail, read } from '@/test/fixtures'

const rows = (live: Parameters<typeof rowsOf>[0]['live'], github?: Parameters<typeof rowsOf>[0]['github']) =>
  rowsOf({ live, github, readAt: 0, underWay: {} })

const row = (over: Partial<Row> = {}): Row => ({
  subject: 'feat/a',
  status: 'done',
  detail: listDetail('feat/a'),
  error: undefined,
  pr: undefined,
  merged: undefined,
  condition: 'clean',
  actions: [],
  underWay: undefined,
  readAt: 0,
  ...over,
})

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

test('each filter keeps the rows the core says belong to it', () => {
  const running = row({ detail: listDetail('feat/a', { services: ['web'] }) })
  const behind = row({ detail: listDetail('feat/a', { needs_sync: true }) })
  const merged = row({ merged: true })
  const all = [running, behind, merged, row()]

  expect(all.filter((r) => matches(r, 'running'))).toEqual([running])
  expect(all.filter((r) => matches(r, 'needs_sync'))).toEqual([behind])
  expect(all.filter((r) => matches(r, 'prunable'))).toEqual([merged])
  expect(countsOf(all, true)).toEqual({ all: 4, running: 1, needs_sync: 1, prunable: 1 })
  expect(countsOf(all, false).prunable, 'not known before GitHub is asked').toBeUndefined()
})

test('the search finds a branch by any part of it, whatever its case', () => {
  expect(searched('feat/Billing-ui', ' billing ')).toBe(true)
  expect(searched('fix/login', 'billing')).toBe(false)
})

test('a row is merged when GitHub says so over a changed worktree, as the one rule over two reads says', () => {
  const live = read('list', [listDetail('feat/a', { condition: 'changed', dirty: true })])
  const github = read('list', [listDetail('feat/a', { condition: 'merged', merged: true })])

  expect(rows(live, github)[0]?.condition).toBe('merged')
  expect(rows(live)[0]?.condition).toBe('changed')
})

test('a message of the core reads as a sentence, a name it opens on kept as spelled', () => {
  expect(sentence('its pull request is merged')).toBe('Its pull request is merged')
  expect(sentence('feat/billing has 1 commit it lacks')).toBe('feat/billing has 1 commit it lacks')
  expect(sentence('.env is missing')).toBe('.env is missing')
  expect(sentence("couldn't fetch origin")).toBe("Couldn't fetch origin")
})
