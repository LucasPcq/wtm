import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { answeringCheckout, type GitHub } from '@/test/checkout'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

async function openCheckout(github: GitHub = {}) {
  const calls: WebCall[] = []
  server.use(...answeringCheckout({ ...github, calls }))
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  await app.user.click(await within(dialog).findByRole('radio', { name: 'Pull request' }))
  return { ...app, dialog, calls }
}

const row = (dialog: HTMLElement, name: RegExp) => within(dialog).findByRole('button', { name })

test('the open pull requests are listed; a fork or one already checked out cannot be chosen but copies its command', async () => {
  const { dialog } = await openCheckout()

  expect(await row(dialog, /#231 Rate-limit the login endpoint/)).toHaveProperty('disabled', false)
  expect(within(dialog).getByText('dana · into main')).toBeTruthy()
  expect(within(dialog).getByText('lucas · into feat/search · draft')).toBeTruthy()
  expect((await row(dialog, /#227 Dark mode tokens/)).hasAttribute('disabled')).toBe(true)
  expect(within(dialog).getByText('From a fork: check it out with gh pr checkout 227')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Copy gh pr checkout 227' })).toBeTruthy()
  expect((await row(dialog, /#219 Login redirect after SSO/)).hasAttribute('disabled')).toBe(true)
  expect(within(dialog).getByText('Already in fix/login-redirect · draft')).toBeTruthy()
  expect(await within(dialog).findByRole('button', { name: 'Copy wtm go fix/login-redirect' })).toBeTruthy()
  expect(within(dialog).getByText("Set by the pull request's base")).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
})

test('a pull request chosen sets the parent to its base, then runs as Create does, its branch fetched once', async () => {
  const { user, dialog, calls } = await openCheckout()

  await user.click(await row(dialog, /#231/))
  expect(await within(dialog).findByText(/the pull request's base/)).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await within(dialog).findByText('Env files')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'fix/rate-limit' })
  expect(within(planned).getByText('#231 Rate-limit the login endpoint')).toBeTruthy()
  expect(within(planned).getByText('Fetched from origin')).toBeTruthy()
  expect(within(planned).getByText(/From main · isolated · \.env from \.env\.example/)).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Create' }))
  expect(await within(dialog).findByText('fix/rate-limit is ready.')).toBeTruthy()

  const reads = (call: WebCall) => !call.answers?.some((a) => a.path === 'no_fetch')
  const asks = calls.filter((call) => call.dry_run === undefined && call.expect_fingerprint === undefined)
  const naming = asks.filter((call) => call.answers?.some((a) => a.path === 'pr'))
  expect(naming.filter(reads).length, 'the form fetches its branch once').toBe(1)
  const planAndRun = calls.filter((call) => call.dry_run || call.expect_fingerprint)
  expect(planAndRun.length).toBe(2)
  expect(planAndRun.every(reads), 'the plan and the run fetch it again').toBe(true)
})

test('a number no open pull request has finds a closed one, which Confirm warns of', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '205')
  const found = await row(dialog, /#205 Old onboarding flow/)
  expect(within(found).getByText('lucas · into main · closed')).toBeTruthy()
  expect(found.getAttribute('aria-pressed'), 'found, not chosen').toBe('false')
  await user.click(found)
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'feat/old-onboarding' })
  expect(within(planned).getByText('#205 is closed: checked out as it was left')).toBeTruthy()
})

test('a pull request the core refuses shows why, and Next says what to do', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '198')
  await row(dialog, /#198 Search: first pass/)
  await user.keyboard('{Enter}')

  expect(
    await within(dialog).findByText("#198's branch is gone from origin: there is nothing to check out."),
  ).toBeTruthy()
  const next = within(dialog).getByRole('button', { name: 'Next' })
  expect(next.getAttribute('aria-disabled')).toBe('true')
  await user.hover(next)
  expect(await screen.findByRole('tooltip')).toHaveProperty('textContent', "#198 can't be checked out: choose another")
})

test('its branch behind origin asks whether to fast-forward it first, and Confirm says it is', async () => {
  const { user, dialog } = await openCheckout({ behind: 2 })

  await user.click(await row(dialog, /#231/))
  expect(await within(dialog).findByText('fix/rate-limit is 2 commits behind origin.')).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: 'Update it first' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'fix/rate-limit' })
  expect(within(planned).getByText(/fast-forwarded first/)).toBeTruthy()
  expect(within(planned).getByText('Existing branch')).toBeTruthy()
})

test('gh signed out is an Alert with its command, the parent stays greyed, and New branch leads back', async () => {
  const { user, dialog } = await openCheckout({ reason: 'signed_out' })

  const alert = await within(dialog).findByRole('alert')
  expect(within(alert).getByText("gh isn't signed in to GitHub")).toBeTruthy()
  expect(within(alert).getByRole('button', { name: 'Copy gh auth login' })).toBeTruthy()
  expect(within(dialog).getByText("Set by the pull request's base")).toBeTruthy()

  await user.click(within(alert).getByRole('button', { name: 'New branch' }))
  expect(await within(dialog).findByRole('textbox', { name: 'Branches' })).toBeTruthy()
})

test('each reason GitHub cannot be asked has its own words', async () => {
  const missing = await openCheckout({ reason: 'not_on_github' })

  expect(await within(missing.dialog).findByText("origin isn't on GitHub")).toBeTruthy()
})

test('no open pull request is its own empty state, a new branch its way on', async () => {
  const { user, dialog } = await openCheckout({ open: [] })

  expect(await within(dialog).findByText('No open pull request')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'New branch' }))
  expect(await within(dialog).findByRole('textbox', { name: 'Branches' })).toBeTruthy()
})

// A number typed then erased before the pause was still asked: the field showed an error for a number no longer there.
test('a number erased before the pause is not asked', async () => {
  const { user, dialog, calls } = await openCheckout()
  const search = await within(dialog).findByRole('textbox', { name: 'Search pull requests' })

  await user.type(search, '20')
  await user.clear(search)
  await new Promise((resolve) => setTimeout(resolve, 500))

  expect(calls.some((call) => call.answers?.some((a) => a.path === 'pr' && a.values[0] === '20'))).toBe(false)
})

// The search answered the number as soon as typing paused: a number only paused on replaced a pull request picked.
test('a number typed is shown, never chosen until Enter, and leaves the pull request picked alone', async () => {
  const { user, dialog, calls } = await openCheckout()
  await user.click(await row(dialog, /#231/))
  const search = within(dialog).getByRole('textbox', { name: 'Search pull requests' })

  await user.type(search, '198')
  const found = await row(dialog, /#198 Search: first pass/)
  expect(found.getAttribute('aria-pressed')).toBe('false')
  await user.clear(search)
  expect((await row(dialog, /#231/)).getAttribute('aria-pressed'), '#231 is still the one').toBe('true')
  expect(
    calls.some(
      (call) =>
        call.answers?.some((a) => a.path === 'pr' && a.values[0] === '198') &&
        !call.answers?.some((a) => a.path === 'no_fetch'),
    ),
  ).toBe(false)

  await user.type(search, '198{Enter}')
  await waitFor(async () => expect((await row(dialog, /#198/)).getAttribute('aria-pressed')).toBe('true'))
})

// A number that began a listed one (21 for #219) was never looked up.
test('a number that prefixes listed ones is looked up too, beside them', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '21')

  expect(await row(dialog, /#21 Lint the CLI/)).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: /#219/ })).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: /#214/ })).toBeTruthy()
})

test('a number GitHub does not know says so under the search', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '999')

  expect(await within(dialog).findByText('GitHub has no pull request #999.')).toBeTruthy()
})

// The plan read origin as the form had fetched it: an origin that could not be fetched was never said.
test('an origin that cannot be fetched is said on the row, whose tag claims no fetch', async () => {
  const { user, dialog } = await openCheckout({ offline: true })

  await user.click(await row(dialog, /#231/))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'fix/rate-limit' })
  expect(within(planned).getByText("couldn't fetch it from origin: where it stands is as last fetched")).toBeTruthy()
  expect(within(planned).getByText('From origin, as last fetched')).toBeTruthy()
  expect(within(planned).queryByText('Fetched from origin')).toBeNull()
})

// A check that failed was an unhandled rejection: the dialog said nothing.
test('GitHub failing on a later ask is said above the picker', async () => {
  const { user, dialog } = await openCheckout()
  server.use(
    http.post('/api/repos/r1/commands/checkout/questions', () =>
      HttpResponse.json(
        {
          ...envelope('checkout', []),
          ok: false,
          error: {
            kind: 'precondition',
            code: 'checkout.github_unavailable',
            message: 'pull requests unavailable: gh pr list: exit status 1: HTTP 502',
          },
        },
        { status: 412 },
      ),
    ),
  )

  await user.click(await row(dialog, /#231/))

  const alert = await within(dialog).findByRole('alert')
  expect(within(alert).getByText("Couldn't ask GitHub")).toBeTruthy()
  expect(within(alert).getByText(/HTTP 502/)).toBeTruthy()
})

test('a refusal of the form that is not GitHub is a red Alert, and Next says to look at it', async () => {
  server.use(
    http.post('/api/repos/r1/commands/checkout/questions', () =>
      HttpResponse.json(
        {
          ...envelope('checkout', []),
          ok: false,
          error: { kind: 'precondition', code: 'checkout.config_not_found', message: 'no wtm config here' },
        },
        { status: 412 },
      ),
    ),
  )
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  await app.user.click(await within(dialog).findByRole('radio', { name: 'Pull request' }))

  expect(await within(dialog).findByText('No wtm config here')).toBeTruthy()
  expect(within(dialog).queryByText("Couldn't ask GitHub")).toBeNull()
  await app.user.hover(within(dialog).getByRole('button', { name: 'Next' }))
  expect(await screen.findByRole('tooltip')).toHaveProperty('textContent', "Checkout can't start: see the error above")
})

test('each opening starts on a new branch', async () => {
  const { user, dialog } = await openCheckout()

  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  await user.click(await screen.findByRole('button', { name: 'Create worktree' }))

  expect(await within(await screen.findByRole('dialog')).findByRole('textbox', { name: 'Branches' })).toBeTruthy()
})
