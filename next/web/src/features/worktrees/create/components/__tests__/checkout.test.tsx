import { screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { answeringCheckout, type GitHub } from '@/test/checkout'
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

  const naming = calls.filter((call) => call.answers?.some((a) => a.path === 'pr'))
  const fetching = naming.filter((call) => !call.answers?.some((a) => a.path === 'no_fetch'))
  expect(fetching.length).toBe(1)
  expect(naming.at(-1)?.dry_run).toBeFalsy()
})

test('a number no open pull request has finds a closed one, which Confirm warns of', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '205')
  const found = await row(dialog, /#205 Old onboarding flow/)
  expect(within(found).getByText('lucas · into main · closed')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('listitem', { name: 'feat/old-onboarding' })
  expect(within(planned).getByText('#205 is closed: checked out as it was left')).toBeTruthy()
})

test('a pull request the core refuses shows why, and Next says what to do', async () => {
  const { user, dialog } = await openCheckout()

  await user.type(await within(dialog).findByRole('textbox', { name: 'Search pull requests' }), '198')

  expect(
    await within(dialog).findByText("Its branch feat/search-v1 is gone from origin, and isn't here either."),
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
