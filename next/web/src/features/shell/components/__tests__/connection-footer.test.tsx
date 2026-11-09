import { screen } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { liveEvents, server } from '@/test/server'

const daemonDown = http.get('/api/events', () => {
  const stream = new ReadableStream({
    start: (controller) =>
      controller.enqueue(
        new TextEncoder().encode(
          'event: warning\ndata: {"message":"the event bus did not start","code":"daemon.start_failed"}\n\n',
        ),
      ),
  })
  return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream' } })
})

test("a daemon that doesn't start disconnects the page, and Try again reconnects it", async () => {
  server.use(daemonDown)
  const { user } = await renderApp('/r/r1/worktrees')

  expect(await screen.findByText("wtm's daemon doesn't answer")).toBeDefined()
  expect(screen.getByText('Disconnected')).toBeDefined()
  expect(screen.queryByRole('button', { name: 'Fetch origin' }), 'nothing to fetch with no stream').toBeNull()

  server.use(liveEvents)
  await user.click(screen.getByRole('button', { name: 'Try again' }))

  expect(await screen.findByText('Live')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Fetch origin' })).toBeDefined()
})

test('the block names the panel it opens and keeps its visible words in its name', async () => {
  server.use(liveEvents)
  await renderApp('/r/r1/worktrees')

  expect(
    await screen.findByRole('button', { name: "wtm's daemon: Live Changes show here as they happen" }),
  ).toBeDefined()
  const status = screen.getByRole('status')
  expect(status.closest('button'), "a button's content is not announced: the live region sits beside it").toBeNull()
  expect(status.textContent).toBe('Live: Changes show here as they happen')
})

test('Try again reads Connecting until the stream is ready, never Live while the daemon may still not start', async () => {
  server.use(daemonDown)
  const { user } = await renderApp('/r/r1/worktrees')
  await screen.findByText("wtm's daemon doesn't answer")

  server.use(
    http.get(
      '/api/events',
      () =>
        new HttpResponse(
          new ReadableStream({ start: (controller) => controller.enqueue(new TextEncoder().encode(': open\n\n')) }),
          {
            headers: { 'Content-Type': 'text/event-stream' },
          },
        ),
    ),
  )
  await user.click(screen.getByRole('button', { name: 'Try again' }))

  expect(await screen.findByText('Connecting…')).toBeDefined()
  expect(screen.queryByText('Live')).toBeNull()
})
