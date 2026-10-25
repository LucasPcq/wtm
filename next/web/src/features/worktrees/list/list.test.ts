import { expect, test } from 'vitest'
import type { Read } from '@/api/queries/commands'
import type { ListDetail } from '@/api/types.gen'
import { sentence } from '../shared/sentence'
import { shownColumns, toggled } from './columns'
import { countsOf, matches, searched } from './filters'
import { rowsOf, type Row } from './rows'
import { changedLabel, stateOf } from './state'

const detail = (over: Partial<ListDetail> = {}): ListDetail => ({
  actions: [],
  active: false,
  ahead: 0,
  behind: 0,
  branch: 'feat/a',
  changes: { changed: 0, untracked: 0 },
  commits_ahead: 0,
  created_at: null,
  dirty: false,
  last_commit_at: null,
  locked: false,
  main: false,
  merged: false,
  missing: false,
  needs_sync: false,
  origin: null,
  parent: 'main',
  path: '/repo/.trees/feat-a',
  pr: null,
  rebasing: false,
  services: [],
  urls: [],
  ...over,
})

const read = (items: [string, ListDetail][]): Read<'list'> => ({
  v: 2,
  ok: true,
  command: 'list',
  follow_ups: [],
  warnings: [],
  items: items.map(([subject, d]) => ({ subject, status: 'done', detail: d })),
})

const row = (over: Partial<Row> = {}): Row => ({
  subject: 'feat/a',
  detail: detail(),
  error: undefined,
  warnings: [],
  pr: undefined,
  merged: undefined,
  ...over,
})

test('a row takes its pull request and whether it merged from the last GitHub read, by subject', () => {
  const pr = { number: 3, url: 'https://github.com/acme/acme/pull/3', state: 'merged' as const }
  const live = read([
    ['feat/a', detail()],
    ['feat/new', detail({ branch: 'feat/new' })],
  ])
  const github = read([['feat/a', detail({ pr, merged: true })]])

  const [a, created] = rowsOf(live, github)

  expect(a?.pr).toEqual(pr)
  expect(a?.merged).toBe(true)
  expect(created?.pr, 'a worktree created since GitHub was asked has no pull request known yet').toBeUndefined()
  expect(rowsOf(live, undefined)[0]?.merged).toBeUndefined()
})

test('each filter keeps the rows the core says belong to it', () => {
  const running = row({ detail: detail({ services: ['web'] }) })
  const behind = row({ detail: detail({ needs_sync: true }) })
  const merged = row({ merged: true })
  const rows = [running, behind, merged, row()]

  expect(rows.filter((r) => matches(r, 'running'))).toEqual([running])
  expect(rows.filter((r) => matches(r, 'needs_sync'))).toEqual([behind])
  expect(rows.filter((r) => matches(r, 'prunable'))).toEqual([merged])
  expect(countsOf(rows, true)).toEqual({ all: 4, running: 1, needs_sync: 1, prunable: 1 })
  expect(countsOf(rows, false).prunable, 'not known before GitHub is asked').toBeUndefined()
})

test('the search finds a branch by any part of it, whatever its case', () => {
  expect(searched(row({ subject: 'feat/Billing-ui' }), ' billing ')).toBe(true)
  expect(searched(row({ subject: 'fix/login' }), 'billing')).toBe(false)
})

test('the state shown is the one that changes most what to do first', () => {
  expect(stateOf(detail({ missing: true, dirty: true }), true)).toBe('missing')
  expect(stateOf(detail({ rebasing: true, dirty: true }), true)).toBe('rebasing')
  expect(stateOf(detail({ dirty: true }), true)).toBe('merged')
  expect(stateOf(detail({ dirty: true }), undefined)).toBe('changed')
  expect(stateOf(detail(), false)).toBe('clean')
  expect(changedLabel(detail({ changes: { changed: 2, untracked: 1 } }))).toBe('3 changed')
})

test('the columns shown are those stored, in the table order, else the default', () => {
  expect(shownColumns(['path', 'state', 'nope'])).toEqual(['state', 'path'])
  expect(shownColumns(null)).toEqual(['state', 'running', 'active'])
  expect(toggled(['state', 'active'], 'running')).toEqual(['state', 'running', 'active'])
  expect(toggled(['state', 'active'], 'state')).toEqual(['active'])
})

test('a message of the core reads as a sentence, a name it opens on kept as spelled', () => {
  expect(sentence('its pull request is merged')).toBe('Its pull request is merged')
  expect(sentence('feat/billing has 1 commit it lacks')).toBe('feat/billing has 1 commit it lacks')
  expect(sentence('.env is missing')).toBe('.env is missing')
  expect(sentence("couldn't fetch origin")).toBe("Couldn't fetch origin")
})
