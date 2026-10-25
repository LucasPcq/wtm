import { http, HttpResponse } from 'msw'
import { expect, test, vi } from 'vitest'
import { server } from '@/test/server'
import { runStreamed } from './operations'

vi.mock('../token', () => ({ token: () => 't' }))

// An answer naming no operation was started as '': its cancel then went nowhere, silently.
test('an operation the server does not name is refused, since nothing could cancel it', async () => {
  server.use(
    http.post(
      '/api/repos/r1/commands/create',
      () => new HttpResponse('', { headers: { 'Content-Type': 'text/event-stream' } }),
    ),
  )
  const started = vi.fn()

  await expect(
    runStreamed({ repo: 'r1', command: 'create', call: { answers: [] }, started, line: () => {} }),
  ).rejects.toThrow('without naming it')
  expect(started).not.toHaveBeenCalled()
})
