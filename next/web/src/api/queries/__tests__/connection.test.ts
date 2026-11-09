import { QueryClient } from '@tanstack/react-query'
import { HttpResponse, http } from 'msw'
import { beforeEach, expect, test } from 'vitest'
import { HttpError } from '@/api/fetch'
import { connectionOf, eventsQuery, giveUpAfter, Unreachable } from '@/api/queries/events'
import { server } from '@/test/server'

beforeEach(() => localStorage.setItem('wtm.token', 't'))

test('a stream that cannot open counts its failed attempts in a row', async () => {
  const client = new QueryClient()
  server.use(http.get('/api/events', () => HttpResponse.error()))

  for (let attempt = 1; attempt <= 2; attempt++) {
    const error = await client.fetchQuery(eventsQuery).catch((failed: unknown) => failed)
    expect(error).toBeInstanceOf(Unreachable)
    expect((error as Unreachable).attempts).toBe(attempt)
  }
  server.use(http.get('/api/events', () => new HttpResponse('', { status: 503 })))
  const refused = await client.fetchQuery(eventsQuery).catch((failed: unknown) => failed)
  expect((refused as Unreachable).attempts, 'a server that answers but cannot stream counts too').toBe(3)
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
  expect(connectionOf({ status: 'error', fetchStatus: 'idle', error: new Unreachable(giveUpAfter - 1) })).toBe(
    'reconnecting',
  )
  expect(connectionOf({ status: 'error', fetchStatus: 'idle', error: new Unreachable(giveUpAfter) })).toBe(
    'server-down',
  )
  expect(
    connectionOf({ status: 'error', fetchStatus: 'fetching', error: new Unreachable(giveUpAfter) }),
    'Try again reconnects',
  ).toBe('reconnecting')
  const daemonDown = { operations: {}, warning: 'the event bus did not start', warningCode: 'daemon.start_failed' }
  expect(connectionOf({ status: 'success', fetchStatus: 'fetching', error: null, data: daemonDown })).toBe(
    'daemon-down',
  )
})

test("a daemon that doesn't start reads wtm's daemon again, which no longer says it up", async () => {
  const client = new QueryClient()
  client.setQueryData(['repo', 'r1', 'daemon-status', []], { ok: true })
  server.use(
    http.get(
      '/api/events',
      () =>
        new HttpResponse(
          'event: warning\ndata: {"message":"the event bus did not start","code":"daemon.start_failed"}\n\n',
        ),
    ),
  )

  await client.fetchQuery(eventsQuery)

  expect(client.getQueryState(['repo', 'r1', 'daemon-status', []])?.isInvalidated).toBe(true)
})
