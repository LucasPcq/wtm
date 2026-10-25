import { expect, test } from 'vitest'
import type { Row } from '@/features/worktrees/list/utils/rows'
import { countsOf, matches, searched } from '@/features/worktrees/utils/filters'
import { listDetail } from '@/test/fixtures'

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
