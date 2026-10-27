import { onlineManager, QueryClient, QueryObserver } from '@tanstack/react-query'
import { waitFor } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { planQuery } from '@/api/queries/commands'
import { envelope } from '@/test/fixtures'
import { server } from '@/test/server'

// A page asleep on Confirm asked its plan again on waking, on answers chosen on reads since gone.
test('a plan is not asked again when the browser comes back online', async () => {
  let asked = 0
  server.use(
    http.post('/api/repos/r1/commands/prune', () => {
      asked += 1
      return HttpResponse.json(envelope('prune', []))
    }),
  )
  const client = new QueryClient()
  client.mount()
  const observer = new QueryObserver(client, planQuery('r1', 'prune', { answers: [], readAt: 1 }))
  const unsubscribe = observer.subscribe(() => {})
  await waitFor(() => expect(asked).toBe(1))

  onlineManager.setOnline(false)
  onlineManager.setOnline(true)
  await delay(50)

  unsubscribe()
  client.unmount()
  expect(asked).toBe(1)
})
