import { QueryClient, QueryObserver } from '@tanstack/react-query'
import { waitFor } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { beforeEach, expect, onTestFinished, test } from 'vitest'
import { HttpError } from '@/api/fetch'
import {
  connectionOf,
  current,
  eventsQuery,
  type Live,
  operationCap,
  reconnectEvery,
  track,
} from '@/api/queries/events'
import { server } from '@/test/server'

beforeEach(() => localStorage.setItem('wtm.token', 't'))

function serve(status: number, body = '') {
  server.use(
    http.get('/api/events', ({ request }) => {
      expect(request.headers.get('X-Wtm-Token')).toBe('t')
      return new HttpResponse(body, { status })
    }),
  )
}

const repo = { id: 'r1', root: '/acme', common_dir: '/acme/.git' }

test('a line of a repository invalidates what it answers, and a warning shows until the next line', async () => {
  const client = new QueryClient()
  client.setQueryData(['repos'], [repo])
  client.setQueryData(['repo', 'r1', 'list', []], { ok: true })
  client.setQueryData(['repo', 'r2', 'list', []], { ok: true })
  serve(
    200,
    'event: warning\ndata: {"message":"the bus was lost"}\n\n' +
      'data: {"v":2,"type":"worktree.created","repo":{"root":"/acme","common_dir":"/acme/.git"}}\n\n',
  )

  const seen: unknown[] = []
  client.getQueryCache().subscribe(() => seen.push(client.getQueryData(eventsQuery.queryKey)))
  await client.fetchQuery(eventsQuery)

  expect(seen).toContainEqual({ warning: 'the bus was lost', operations: {} })
  expect(client.getQueryData(eventsQuery.queryKey)).toEqual({ operations: {} })
  expect(client.getQueryState(['repo', 'r1', 'list', []])?.isInvalidated).toBe(true)
  expect(client.getQueryState(['repo', 'r2', 'list', []])?.isInvalidated).toBe(false)
})

test('the stream reconnects after it ends or fails, never once the token is refused', async () => {
  const client = new QueryClient()
  const interval = eventsQuery.refetchInterval as (query: unknown) => number | false
  const after = (error: unknown) => interval({ state: { error } })

  serve(401)
  const refused = await client.fetchQuery(eventsQuery).catch((error: unknown) => error)
  expect(refused).toBeInstanceOf(HttpError)
  expect(after(refused)).toBe(false)
  expect(after(new HttpError(503, 'stopping'))).toBe(reconnectEvery)
  expect(after(new TypeError('Failed to fetch'))).toBe(reconnectEvery)
  expect(after(null)).toBe(reconnectEvery)
})

test('the connection reads the query of the stream', () => {
  expect(connectionOf({ status: 'pending', fetchStatus: 'fetching', error: null })).toBe('connecting')
  expect(connectionOf({ status: 'success', fetchStatus: 'fetching', error: null })).toBe('live')
  expect(connectionOf({ status: 'success', fetchStatus: 'idle', error: null })).toBe('reconnecting')
  expect(connectionOf({ status: 'error', fetchStatus: 'idle', error: new TypeError('Failed to fetch') })).toBe(
    'reconnecting',
  )
  expect(connectionOf({ status: 'error', fetchStatus: 'idle', error: new HttpError(401, 'refused') })).toBe(
    'disconnected',
  )
})

test('back after a break, everything is read again but what GitHub answered and a review that fetches origin', async () => {
  const client = new QueryClient()
  client.setQueryData(['repos'], [repo])
  client.setQueryData(eventsQuery.queryKey, { operations: {} })
  client.setQueryData(['repo', 'r1', 'list', []], { ok: true })
  client.setQueryData(['github', 'r1', 'list', []], { ok: true })
  client.setQueryData(['review', 'r1', 'prune', []], { ok: true })
  serve(200)

  await client.fetchQuery({ ...eventsQuery, staleTime: 0 })

  expect(client.getQueryState(['repo', 'r1', 'list', []])?.isInvalidated).toBe(true)
  expect(client.getQueryState(['github', 'r1', 'list', []])?.isInvalidated).toBe(false)
  expect(client.getQueryState(['review', 'r1', 'prune', []])?.isInvalidated, 'a review fetches origin').toBe(false)
})

test('an operation of any surface holds the worktrees it started on until it finishes', () => {
  const ts = '2026-10-27T10:00:00Z'
  const line = (type: string, more: Record<string, string> = {}) => ({
    v: 2 as const,
    type,
    ts,
    op_id: 'op-1',
    ...more,
  })

  const started = track({}, line('operation.started', { command: 'clean', surface: 'cli' }), 'r1')
  const working = track(started, line('unit.started', { subject: 'feat/a' }), 'r1')
  const done = track(working, line('unit.finished', { subject: 'feat/a', status: 'done' }), 'r1')

  expect(done).toEqual({ 'op-1': { repo: 'r1', command: 'clean', subjects: ['feat/a'], since: Date.parse(ts) } })
  expect(track(done, line('operation.finished', { command: 'clean' }), 'r1')).toEqual({})
  expect(track({}, { v: 2, type: 'unit.started', ts, subject: 'feat/a' }, 'r1')).toEqual({})
})

// An operation already running when the page opened, or across a reconnect, never showed its worktrees.
test('a unit of an operation whose start the stream did not see is under way all the same, its command unknown', () => {
  const unit = { v: 2 as const, type: 'unit.started', ts: '2026-10-27T10:00:00Z', op_id: 'op-2', subject: 'feat/b' }

  expect(track({}, unit, 'r1')).toEqual({
    'op-2': { repo: 'r1', command: '', subjects: ['feat/b'], since: Date.parse(unit.ts) },
  })
})

// A process killed mid-operation never says it finished: its worktrees stayed greyed for good.
test("an operation is under way no longer than the core's own cap", () => {
  const since = Date.parse('2026-10-27T10:00:00Z')
  const operations = { 'op-3': { repo: 'r1', command: 'clean', subjects: ['feat/c'], since } }

  expect(current(operations, since + operationCap - 1)).toHaveLength(1)
  expect(current(operations, since + operationCap)).toEqual([])
})

// operation.finished cleared the worktrees under way before the read without them landed: a removed row came back.
test('an operation finished leaves its worktrees under way until the read after it lands', async () => {
  const client = new QueryClient()
  client.setQueryData(['repos'], [repo])
  let reads = 0
  const observer = new QueryObserver(client, {
    queryKey: ['repo', 'r1', 'list', []],
    queryFn: async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
      reads += 1
      return reads
    },
  })
  onTestFinished(observer.subscribe(() => {}))
  await waitFor(() => expect(reads).toBe(1))
  const of = '"ts":"2026-10-27T10:00:00Z","repo":{"root":"/acme","common_dir":"/acme/.git"},"op_id":"op-1"'
  serve(
    200,
    `data: {"v":2,"type":"operation.started",${of},"command":"clean","surface":"cli"}\n\n` +
      `data: {"v":2,"type":"unit.started",${of},"subject":"feat/a"}\n\n` +
      `data: {"v":2,"type":"operation.finished",${of},"command":"clean","surface":"cli","ok":true}\n\n`,
  )

  const seen: { held: number; reads: number }[] = []
  client.getQueryCache().subscribe(() => {
    const live = client.getQueryData<Live>(eventsQuery.queryKey)
    if (live) seen.push({ held: Object.keys(live.operations).length, reads })
  })
  await client.fetchQuery(eventsQuery)

  const released = seen.findIndex((at, i) => at.held === 0 && seen.slice(0, i).some((before) => before.held > 0))
  expect(seen[released]?.reads).toBe(2)
})
