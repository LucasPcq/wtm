import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { answeringRunUp, noProfileQuestionsOf, refusedUntil, runUpPlan, runUpRun } from '@/test/run-up'
import { server } from '@/test/server'

type User = Awaited<ReturnType<typeof renderApp>>['user']

const fromTheMenu = async (user: User, worktree: string) => {
  await user.click(await screen.findByRole('button', { name: `Actions on ${worktree}` }))
  await user.click(await screen.findByRole('menuitem', { name: 'Start a profile' }))
  return screen.findByRole('dialog', { name: 'Start a profile' })
}

const enabled = (dialog: HTMLElement, name: string) =>
  waitFor(() => expect(within(dialog).getByRole('button', { name }).getAttribute('aria-disabled')).toBe('false'))

test('Start a profile asks the worktrees, then the profile, then confirms in order, and starts job by job', async () => {
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Start a profile' }))
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
  expect(within(shared).getByText('already up: both join it, a namespace each')).toBeDefined()
  const docs = within(dialog).getByRole('region', { name: 'feat/search-docs' })
  expect(
    within(docs)
      .getAllByRole('listitem')
      .map((row) => row.textContent),
  ).toEqual(['4migratea task: runs first, the others wait for it', '5web:3050', '6api:4050'])
  expect(within(dialog).getByText('The 2 worktrees start side by side; in each, migrate runs first.')).toBeDefined()
  expect(within(dialog).getByText('The jobs of feat/billing keep running.')).toBeDefined()
  expect(within(dialog).getByText('They keep running when this page closes; Stop jobs ends them.')).toBeDefined()

  await user.click(within(dialog).getByRole('button', { name: 'Start dev in 2 worktrees' }))
  expect(await within(dialog).findByText('dev started in 2 worktrees.')).toBeDefined()
  expect(within(dialog).getAllByText('web, api up · postgres joined · migrate done').length).toBe(2)
})

test("a worktree's menu opens on Confirm, its shared services first in its order, unnumbered", async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/search')
  const order = await within(dialog).findByRole('region', { name: 'In this order' })
  expect(
    within(order)
      .getAllByRole('listitem')
      .map((row) => row.textContent),
  ).toEqual([
    'postgresshared, already up: joins it, with acme_feat_search of its own',
    '1migratea task: runs first, the others wait for it',
    '2web:3040',
    '3api:4040',
  ])
  expect(within(dialog).getByText('profile dev · nothing running there yet')).toBeDefined()
  await enabled(dialog, 'Start dev')
})

test("a job that fails ends the worktree's start, said with its lines, and what follows it never starts", async () => {
  server.use(...answeringRunUp(undefined, runUpRun(['feat/search'])))
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/search')
  await enabled(dialog, 'Start dev')
  await user.click(within(dialog).getByRole('button', { name: 'Start dev' }))

  expect(await within(dialog).findByText("dev didn't start in 1 of 1.")).toBeDefined()
  expect(within(dialog).getByText('migrate failed: exit 1')).toBeDefined()
})

// From a worktree's menu, Confirm offered to start dev in a worktree that ran dev already.
test('a worktree that runs the default profile already is asked which profile to start', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/billing')
  expect(await within(dialog).findByText('dev', { selector: 'label' })).toBeDefined()
  expect(within(dialog).getByText(/already running in feat\/billing/)).toBeDefined()
})

// Through Profile with the default untouched, Next went back to Profile once.
test('a worktree that runs the profile picked is left as it is, the profile kept on Next', async () => {
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Start a profile' }))
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Start in feat/billing' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Start in feat/search' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const left = await within(dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('already runs dev')).toBeDefined()
  expect(within(dialog).getByRole('region', { name: 'In this order' })).toBeDefined()
  await enabled(dialog, 'Start dev')
})

test('a run.toml with no profile asks none: the start goes from the worktrees to Confirm', async () => {
  const planned: WebCall[] = []
  const plan = (call: WebCall) => {
    planned.push(call)
    return runUpPlan(call)
  }
  server.use(...answeringRunUp(plan, undefined, noProfileQuestionsOf))
  const { user } = await renderApp('/r/r1/running')

  await user.click(await screen.findByRole('button', { name: 'Start a profile' }))
  const dialog = await screen.findByRole('dialog', { name: 'Start a profile' })
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Start in feat/search' }))
  expect(within(dialog).queryByText('Profile')).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  await enabled(dialog, 'Start dev')
  expect(planned.at(-1)?.answers?.map((answer) => answer.path)).toEqual(['worktree'])
})

test('a start refused for foreign data is started anyway on consent, its new plan confirmed first', async () => {
  server.use(...answeringRunUp(refusedUntil({ code: 'run-up.unsafe', consent: 'force', followUp: true })))
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/search')
  expect(await within(dialog).findByText(/changes data feat\/search doesn't own/)).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Start anyway' }))

  expect(await within(dialog).findByRole('region', { name: 'In this order' })).toBeDefined()
  await enabled(dialog, 'Start dev')
})

test("a port clash is lifted by stopping the others' jobs only when the core offers it", async () => {
  server.use(...answeringRunUp(refusedUntil({ code: 'run-up.port_clash', consent: 'exclusive', followUp: true })))
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/search')
  await user.click(await within(dialog).findByRole('button', { name: "Stop the others' jobs" }))
  expect(await within(dialog).findByRole('region', { name: 'In this order' })).toBeDefined()
})

test('a port clash the core offers no way out of is said, with nothing to press', async () => {
  server.use(...answeringRunUp(refusedUntil({ code: 'run-up.port_clash', consent: 'exclusive', followUp: false })))
  const { user } = await renderApp('/r/r1/worktrees')

  const dialog = await fromTheMenu(user, 'feat/search')
  expect(await within(dialog).findByText('port 3040 is held by web of feat/billing')).toBeDefined()
  expect(within(dialog).queryByRole('button', { name: "Stop the others' jobs" })).toBeNull()
})
