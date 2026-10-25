import { QueryClient } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { expect, test, vi } from 'vitest'
import { server } from '@/test/server'
import { connectionOf, eventsQuery, reconnectEvery } from './events'
import { HttpError } from '../fetch'

vi.mock('../token', () => ({ token: () => 't' }))

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

  expect(seen).toContainEqual({ warning: 'the bus was lost' })
  expect(client.getQueryData(eventsQuery.queryKey)).toEqual({})
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

test('back after a break, everything is read again but what GitHub answered, asked on demand only', async () => {
  const client = new QueryClient()
  client.setQueryData(['repos'], [repo])
  client.setQueryData(eventsQuery.queryKey, {})
  client.setQueryData(['repo', 'r1', 'list', []], { ok: true })
  client.setQueryData(['github', 'r1', 'list', []], { ok: true })
  serve(200)

  await client.fetchQuery({ ...eventsQuery, staleTime: 0 })

  expect(client.getQueryState(['repo', 'r1', 'list', []])?.isInvalidated).toBe(true)
  expect(client.getQueryState(['github', 'r1', 'list', []])?.isInvalidated).toBe(false)
})
