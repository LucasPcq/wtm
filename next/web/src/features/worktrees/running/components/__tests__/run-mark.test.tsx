import { waitFor } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { linesOf, listing, openStream, panelOf, shown } from '@/test/run-logs'
import { server } from '@/test/server'

test('the run mark parts a restart’s new run from the one before, and starts a first run', async () => {
  listing()
  const stream = openStream('feat/billing', 'web')
  const marked = (text: string) => ({ text, mark: text === '[wtm] started' })
  stream.send(
    ...linesOf('[wtm] started', 'GET / 200', '[wtm] started', 'ready again').map((line) => ({
      ...line,
      ...marked(line.text),
    })),
  )
  server.use(http.get('/api/repos/r1/jobs/:job/logs', () => stream.response()))
  await renderApp('/r/r1/running?worktree=feat/billing&job=web')

  const panel = await panelOf('web')

  await waitFor(() => expect(shown(panel)).toBe('Started GET / 200Restarted ready again'))
})
