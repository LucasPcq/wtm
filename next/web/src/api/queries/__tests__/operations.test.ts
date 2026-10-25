import { HttpResponse, http } from 'msw'
import { beforeEach, expect, test, vi } from 'vitest'
import { runStreamed } from '@/api/queries/operations'
import { server } from '@/test/server'

beforeEach(() => localStorage.setItem('wtm.token', 't'))

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
