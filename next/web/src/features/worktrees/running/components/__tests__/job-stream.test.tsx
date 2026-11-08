import { screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { linesOf, listing, onLine, openStream, panelOf, shown } from '@/test/run-logs'
import { server } from '@/test/server'

const streaming = (stream: ReturnType<typeof openStream>) =>
  server.use(http.get('/api/repos/r1/jobs/:job/logs', () => stream.response()))

// The review of #270: the server held its answer until a first line, the page read on and on.
test('a running job that has printed nothing says no line yet', async () => {
  listing()
  streaming(openStream('feat/billing', 'web'))
  await renderApp('/r/r1/running?worktree=feat/billing&job=web')

  const panel = await panelOf('web')

  expect(await within(panel).findByText('No line yet.')).toBeDefined()
})

// The review of #270: a warning of the stream was kept and never shown.
test('what kept lines away is said above them, one line each', async () => {
  listing()
  const stream = openStream('feat/billing', 'web')
  stream.send(...linesOf('ready'), {
    type: 'warning',
    code: 'run-logs.lines_dropped',
    message: 'feat/billing · web fell behind: some of its lines were dropped',
  })
  streaming(stream)
  await renderApp('/r/r1/running?worktree=feat/billing&job=web')

  const panel = await panelOf('web')

  expect(await within(panel).findByText('feat/billing · web fell behind: some of its lines were dropped')).toBeDefined()
})

// The review of #270: a follow that broke ended silently, Follow still on.
test('a stream that ends while its job runs says it stopped following, and reconnects', async () => {
  listing()
  let asked = 0
  server.use(
    http.get('/api/repos/r1/jobs/:job/logs', () => {
      asked++
      const stream = openStream('feat/billing', 'web')
      stream.send(...linesOf(`run ${asked}`))
      stream.end()
      return stream.response()
    }),
  )
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')

  expect(await within(panel).findByText('Stopped following web')).toBeDefined()
  expect(within(panel).getByRole('switch', { name: 'Follow' }).hasAttribute('disabled')).toBe(true)
  await user.click(within(panel).getByRole('button', { name: 'Reconnect' }))

  await waitFor(() => expect(shown(panel)).toBe('run 2'))
  expect(asked).toBe(2)
})

// Seen in the sandbox: a daemon of another build follows its jobs, yet its warning hid a follow that stopped.
test('a follow that stops is said beside a daemon of another build, not beside the legacy one', async () => {
  listing()
  let skew = 'build'
  server.use(
    http.get('/api/repos/r1/jobs/:job/logs', () => {
      const stream = openStream('feat/billing', 'web')
      stream.send(...linesOf('ready'), {
        type: 'warning',
        code: 'run-logs.daemon_skew',
        params: { skew },
        message: `the daemon is ${skew === 'legacy' ? 'the legacy one' : 'of another build'}`,
      })
      stream.end()
      return stream.response()
    }),
  )
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')

  expect(await within(panel).findByText('Stopped following web')).toBeDefined()
  skew = 'legacy'
  await user.click(within(panel).getByRole('button', { name: 'Reconnect' }))

  expect(await within(panel).findByText('the daemon is the legacy one')).toBeDefined()
  expect(within(panel).queryByText('Stopped following web')).toBeNull()
})

// The review of #270: a connection broken mid-stream replaced every line held with a red failure, with no way back.
test('a stream that breaks off keeps the lines read and offers to reconnect', async () => {
  listing()
  let asked = 0
  server.use(
    http.get('/api/repos/r1/jobs/:job/logs', () => {
      asked++
      const stream = openStream('feat/billing', 'web')
      stream.send(...linesOf(`run ${asked}`))
      if (asked === 1) setTimeout(stream.fail, 100)
      return stream.response()
    }),
  )
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')

  expect(await within(panel).findByText('Stopped following web')).toBeDefined()
  expect(within(panel).getByText(/the stream broke off/)).toBeDefined()
  expect(shown(panel)).toBe('run 1')
  await user.click(within(panel).getByRole('button', { name: 'Reconnect' }))

  await waitFor(() => expect(shown(panel)).toBe('run 2'))
})

// The review of #270: a stream read again numbers its lines from 0, so the match stepped to became another line.
test('a reconnect forgets the match stepped to, its lines numbered again', async () => {
  listing()
  let asked = 0
  server.use(
    http.get('/api/repos/r1/jobs/:job/logs', () => {
      asked++
      const stream = openStream('feat/billing', 'web')
      stream.send(...linesOf(...(asked === 1 ? ['GET /a', 'GET /b'] : ['GET /x', 'GET /y'])))
      stream.end()
      return stream.response()
    }),
  )
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  await waitFor(() => expect(shown(panel)).toContain('GET /b'))
  await user.type(within(panel).getByRole('textbox', { name: 'Search this log' }), 'GET')
  await user.click(within(panel).getByRole('button', { name: 'Previous match' }))
  expect(onLine(panel)).toContain('GET /a')

  await user.click(within(panel).getByRole('button', { name: 'Reconnect' }))

  await waitFor(() => expect(shown(panel)).toContain('GET /y'))
  expect(onLine(panel)).toContain('GET /y')
})

// The review of #270: the match the view was on was counted from the last, so a new match moved it.
test('the match stepped to stays as new matching lines arrive', async () => {
  listing()
  const stream = openStream('feat/billing', 'web')
  stream.send(...linesOf('GET /invoices 200', 'GET /health 200', 'GET /invoices/2 200'))
  streaming(stream)
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  await waitFor(() => expect(shown(panel)).toContain('/invoices/2'))

  await user.type(within(panel).getByRole('textbox', { name: 'Search this log' }), 'invoices')
  await user.click(within(panel).getByRole('button', { name: 'Previous match' }))
  expect(onLine(panel)).toContain('GET /invoices 200')
  stream.send({ text: 'GET /invoices/3 200' })

  await waitFor(() => expect(within(panel).getByText('1 of 3')).toBeDefined())
  expect(onLine(panel)).toContain('GET /invoices 200')
})

// The review of #270: typing after a search of the whole log marked its old results.
test('a new search after one of the whole log reads the lines held again', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  await waitFor(() => expect(shown(panel)).toContain('GET /health'))
  const search = within(panel).getByRole('textbox', { name: 'Search this log' })
  await user.type(search, '500')
  await user.click(within(panel).getByRole('button', { name: 'Search the whole log' }))
  await waitFor(() => expect(shown(panel)).toBe('GET /invoices 500 error'))

  await user.clear(search)
  await user.type(search, 'health')

  await waitFor(() => expect(shown(panel)).toContain('GET /invoices/inv_0042'))
  expect(within(panel).getByText('1 of 1')).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Search the whole log' })).toBeDefined()
})

// The review of #270: the panel's keys acted from an open menu or dialog, and Alt keys from any field.
test("the panel's keys act from its own search, never from a menu or a dialog", async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')
  const wrap = within(panel).getByRole('button', { name: 'Wrap lines' })

  await user.click(within(panel).getByRole('button', { name: 'More for web' }))
  await screen.findByRole('menu')
  await user.keyboard('w')
  await user.keyboard('{Escape}')
  await user.click(within(panel).getByRole('button', { name: 'Keyboard' }))
  await screen.findByRole('dialog', { name: 'Keyboard' })
  await user.keyboard('w')
  await user.keyboard('{Escape}')
  expect(wrap.getAttribute('aria-pressed')).toBe('false')

  await user.click(within(panel).getByRole('textbox', { name: 'Search this log' }))
  await user.keyboard('{Alt>}c{/Alt}')
  expect(within(panel).getByRole('button', { name: 'Match case' }).getAttribute('aria-pressed')).toBe('true')
  await user.keyboard('w')
  expect(wrap.getAttribute('aria-pressed')).toBe('false')
})

test('the shortcuts are a list to read, not items to pick', async () => {
  listing()
  const { user } = await renderApp('/r/r1/running?worktree=feat/billing&job=web')
  const panel = await panelOf('web')

  await user.click(within(panel).getByRole('button', { name: 'Keyboard' }))

  const list = await screen.findByRole('dialog', { name: 'Keyboard' })
  expect(within(list).queryAllByRole('menuitem')).toEqual([])
  expect(
    within(list)
      .getAllByRole('term')
      .map((term) => term.textContent),
  ).toContain('Only matching')
})
