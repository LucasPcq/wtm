import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { answeringRunUp, heldRun, runningWorktrees, runUpRun } from '@/test/run-up'
import { server } from '@/test/server'

const listing = () =>
  server.use(http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', runningWorktrees))))

test('nothing running says so, with Start a profile, and Stop jobs not offered yet', async () => {
  await renderApp('/r/r1/running')

  expect(await screen.findByText('Nothing is running')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Stop jobs' }).getAttribute('aria-disabled')).toBe('true')
  expect(screen.getAllByRole('button', { name: 'Start a profile' }).length).toBe(2)
})

test('each worktree that holds jobs is a group, its crash first, and the sidebar counts the jobs up', async () => {
  listing()
  await renderApp('/r/r1/running')

  const group = await screen.findByRole('region', { name: 'feat/billing' })
  expect(within(group).getByText('profile dev · 1 up · 1 crashed · 1 exited')).toBeDefined()
  expect(
    within(group)
      .getAllByRole('listitem')
      .map((row) => row.getAttribute('aria-label')),
  ).toEqual(['api', 'web', 'migrate'])
  expect(within(group).getByText('crashed · exit 1')).toBeDefined()
  expect(screen.getByText('1 job up in 1 worktree · 1 crashed')).toBeDefined()
  const running = screen.getByRole('link', { name: /Running/ })
  expect(running.closest('li')?.textContent).toContain('1')
})

test("a worktree's menu offers its profiles, the one it runs disabled, and one picked opens on Confirm", async () => {
  listing()
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'More for feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))
  const dev = await screen.findByRole('menuitem', { name: /^dev/ })
  expect(dev.getAttribute('aria-disabled')).toBe('true')
  expect(dev.textContent).toContain('running')
  // happy-dom's pointer never settles in a submenu: the keyboard picks it, as a person may
  screen.getByRole('menuitem', { name: /^full/ }).focus()
  await user.keyboard('{Enter}')

  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  expect(within(dialog).getByRole('listitem', { current: false, name: 'worker' })).toBeDefined()
  expect(await within(dialog).findByRole('button', { name: 'Start full' })).toBeDefined()
})

test('Start a profile asks the worktrees, then the profile, then confirms in order, and starts job by job', async () => {
  const { user } = await renderApp('/r/r1/running')

  await user.click((await screen.findAllByRole('button', { name: 'Start a profile' }))[0] as HTMLElement)
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Start in feat/search' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Start in feat/search-docs' }))
  expect(within(dialog).getByText('2 of 6 selected')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(
    await within(dialog).findByText('For feat/search and feat/search-docs · 2 profiles in run.toml.'),
  ).toBeDefined()
  expect(within(dialog).getByRole('radio', { name: 'dev' }).getAttribute('aria-checked')).toBe('true')
  expect(within(dialog).getByRole('textbox', { name: 'Search the profiles' })).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const shared = await within(dialog).findByRole('region', { name: 'Shared services, joined once' })
  expect(within(shared).getByText('runs once in main: the 2 join it, a namespace each')).toBeDefined()
  const docs = within(dialog).getByRole('region', { name: 'feat/search-docs' })
  expect(
    within(docs)
      .getAllByRole('listitem')
      .map((row) => row.textContent),
  ).toEqual(['4migratea task: runs first, the others wait for it', '5web:3050', '6api:4050'])
  expect(within(dialog).getByText('The 2 worktrees start side by side; in each, migrate runs first.')).toBeDefined()

  await user.click(within(dialog).getByRole('button', { name: 'Start dev in 2 worktrees' }))
  expect(await within(dialog).findByText('dev started in 2 worktrees.')).toBeDefined()
  expect(within(dialog).getAllByText('web, api up · postgres joined · migrate done').length).toBe(2)
})

test("a worktree's menu on the Worktrees page starts its profile from Confirm", async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on feat/search' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))

  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  expect(await within(dialog).findByRole('region', { name: 'In this order' })).toBeDefined()
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Start dev' }).getAttribute('aria-disabled')).toBe('false'),
  )
})

test('hidden while it starts, the run goes on in a toast that opens it again, the page greying what comes up', async () => {
  const held = heldRun()
  server.use(...answeringRunUp(undefined, held.run))
  const { user } = await renderApp('/r/r1/running')

  await user.click((await screen.findAllByRole('button', { name: 'Start a profile' }))[0] as HTMLElement)
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Start in feat/search' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const start = await within(dialog).findByRole('button', { name: 'Start dev' })
  await waitFor(() => expect(start.getAttribute('aria-disabled')).toBe('false'))
  await user.click(start)
  expect(await within(dialog).findByText('Starting dev in feat/search · 0 of 4 started')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Hide' }))

  expect(await screen.findByText('Starting dev in feat/search')).toBeDefined()
  const group = await screen.findByRole('region', { name: 'feat/search' })
  expect(within(group).getByRole('listitem', { name: 'web' }).textContent).toContain('waits for migrate')

  await user.click(screen.getByRole('button', { name: 'Show' }))
  expect(await screen.findByRole('dialog', { name: 'Start a profile' })).toBeDefined()
  await held.finish()
})

test("a job that fails ends the worktree's start, said with its lines, and what follows it never starts", async () => {
  server.use(...answeringRunUp(undefined, runUpRun(['feat/search'])))
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on feat/search' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  const start = await within(dialog).findByRole('button', { name: 'Start dev' })
  await waitFor(() => expect(start.getAttribute('aria-disabled')).toBe('false'))
  await user.click(start)

  expect(await within(dialog).findByText("dev didn't start in 1 of 1.")).toBeDefined()
  expect(within(dialog).getByText('migrate failed: exit 1')).toBeDefined()
})

// From a worktree's menu, Confirm offered to start dev in a worktree that ran dev already.
test('a worktree that runs the default profile already is asked which profile to start', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('button', { name: 'Actions on feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))

  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  expect(await within(dialog).findByText('dev', { selector: 'label' })).toBeDefined()
  expect(within(dialog).getByText(/already running in feat\/billing/)).toBeDefined()
})
