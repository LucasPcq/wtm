import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { psEnvelope, psGroup, psJob } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// acme as the Running page reads it: feat/billing runs dev (worker crashed, migrate exited), fix/login runs full (web
// and a stack running mailpit), and main runs postgres for the three of them
const acme = () =>
  server.use(
    http.post('/api/repos/r1/commands/run-ps', () =>
      HttpResponse.json(
        psEnvelope([
          psGroup('feat/billing', [
            psJob('worker', {
              state: 'crashed',
              exit_code: 1,
              last_line: { text: "Cannot find module 'send-invoice'" },
            }),
            psJob('web', { address: 'localhost:3020' }),
            psJob('migrate', { kind: 'task', state: 'exited', exit_code: 0 }),
          ]),
          psGroup(
            'fix/login',
            [
              psJob('web', { address: 'localhost:3030' }),
              psJob('stack', {
                hold: 'detached',
                services: [{ name: 'mailpit', state: 'running', address: 'localhost:8055' }],
              }),
            ],
            { profile: 'full' },
          ),
          psGroup(
            'main',
            [
              psJob('postgres', {
                address: 'localhost:5432',
                holders: [
                  { branch: 'main', path: '/acme', namespace: 'acme_main' },
                  { branch: 'feat/billing', path: '/acme.trees/feat-billing', namespace: 'acme_feat_billing' },
                  { branch: 'fix/login', path: '/acme.trees/fix-login', namespace: 'acme_fix_login' },
                ],
              }),
            ],
            {
              shared: true,
              profile: '',
              counts: { running: 1, starting: 0, crashed: 0, exited: 0, stopped: 0, not_started: 0, holders: 3 },
            },
          ),
        ]),
      ),
    ),
  )

const names = (region: HTMLElement) =>
  within(region)
    .getAllByRole('listitem')
    .flatMap((row) => row.getAttribute('aria-label') ?? [])

test('a chip keeps its state, the toolbar says what the filter hides, and Clear brings everything back', async () => {
  acme()
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('radio', { name: /Crashed/ }))

  expect(names(screen.getByRole('region', { name: 'feat/billing' }))).toEqual(['worker'])
  expect(screen.queryByRole('region', { name: 'fix/login' })).toBeNull()
  expect(screen.getByText(/5 hidden by the filter/)).toBeDefined()

  await user.click(screen.getByRole('button', { name: 'Clear' }))
  expect(await screen.findByRole('region', { name: 'fix/login' })).toBeDefined()
  expect(screen.queryByText(/hidden by the filter/)).toBeNull()
})

test("a group closes on its ended jobs, shown on demand, and a worker's crash shows its last line", async () => {
  acme()
  const { user } = await renderApp('/r/r1/running')
  const billing = await screen.findByRole('region', { name: 'feat/billing' })

  expect(within(billing).getByText("Cannot find module 'send-invoice'")).toBeDefined()
  expect(within(billing).getByText('1 exited · migrate')).toBeDefined()
  await user.click(within(billing).getByRole('button', { name: 'Show' }))

  expect(names(billing)).toEqual(['worker', 'web', 'migrate'])
  expect(within(billing).getByText('Exited jobs shown')).toBeDefined()
})

test('the shared services are folded by default and open on their holders; a runner lists its services', async () => {
  acme()
  const { user } = await renderApp('/r/r1/running')
  const shared = await screen.findByRole('region', { name: 'Shared services' })

  expect(within(shared).getByText('1 up · held by 3 worktrees')).toBeDefined()
  expect(within(shared).queryByText('acme_main')).toBeNull()
  await user.click(within(shared).getByRole('button', { name: 'Open Shared services' }))

  expect(within(shared).getByText('acme_feat_billing')).toBeDefined()
  expect(within(screen.getByRole('region', { name: 'fix/login' })).getByText('localhost:8055')).toBeDefined()

  await user.click(within(shared).getByRole('link', { name: /postgres/ }))
  const where = await screen.findByRole('navigation', { name: 'Where it runs' })
  expect(where.textContent).toBe('Shared services')
  expect(await screen.findByText(/runs once, in main’s checkout/)).toBeDefined()
})

test('a group folded stays folded, kept in this browser for the repository', async () => {
  acme()
  const { user } = await renderApp('/r/r1/running')
  const login = await screen.findByRole('region', { name: 'fix/login' })

  await user.click(within(login).getByRole('button', { name: 'Fold fix/login' }))

  expect(within(login).queryByRole('listitem')).toBeNull()
  expect(within(login).getByText('profile full · 2 up')).toBeDefined()
  expect(JSON.parse(localStorage.getItem('wtm.ui') ?? '{}').state.folded).toEqual({ r1: ['fix/login'] })
})

test('a job pinned from its panel sits on top, out of its group', async () => {
  acme()
  const { user } = await renderApp('/r/r1/running?worktree=fix/login&job=web')

  await user.click(await screen.findByRole('button', { name: 'Pin web to the top' }))

  const pinned = await screen.findByRole('region', { name: 'Pinned' })
  expect(names(pinned)).toEqual(['web'])
  expect(within(pinned).getByText(/fix\/login/)).toBeDefined()
  await waitFor(() => expect(names(screen.getByRole('region', { name: 'fix/login' }))).toEqual(['stack']))

  // a window too narrow for the list picks it in the switcher
  await user.click(screen.getByRole('button', { name: 'Switch job' }))
  const menu = await screen.findByRole('menu')
  expect(within(menu).getByText('Pinned')).toBeDefined()
  expect(
    within(menu)
      .getAllByRole('menuitem')
      .map((item) => item.textContent),
  ).toContain('web')
})

test('this worktree is where the page was opened from', async () => {
  localStorage.setItem('wtm.here.r1', '/acme.trees/fix-login')
  acme()
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Filter' }))
  await user.click(await screen.findByRole('menuitemradio', { name: /This worktree/ }))

  await waitFor(() => expect(screen.queryByRole('region', { name: 'feat/billing' })).toBeNull())
  expect(screen.getByRole('region', { name: 'fix/login' })).toBeDefined()
})

test('the Pinned group unpins them all from its menu', async () => {
  acme()
  const { user } = await renderApp('/r/r1/running?worktree=fix/login&job=web')
  await user.click(await screen.findByRole('button', { name: 'Pin web to the top' }))
  const pinned = await screen.findByRole('region', { name: 'Pinned' })

  await user.click(within(pinned).getByRole('button', { name: 'More for Pinned' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Unpin all' }))

  await waitFor(() => expect(screen.queryByRole('region', { name: 'Pinned' })).toBeNull())
  expect(names(screen.getByRole('region', { name: 'fix/login' }))).toEqual(['web', 'stack'])
})
