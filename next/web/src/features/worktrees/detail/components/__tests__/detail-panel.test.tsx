import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { envelope, followUp, statusDetail, unreadable } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

const answers = async (request: Request) => {
  const call = (await request.json()) as WebCall
  return (path: string) => call.answers?.find((answer) => answer.path === path)?.values[0]
}

async function openDetail(worktree: string) {
  const app = await renderApp(`/r/r1/worktrees?worktree=${encodeURIComponent(worktree)}`)
  return { ...app, detail: await screen.findByRole('complementary', { name: worktree }) }
}

test('each worktree is listed, and its row opens the detail with the problem to fix first and its command', async () => {
  const { user } = await renderApp('/r/r1/worktrees')

  await user.click(await screen.findByRole('link', { name: /feat\/billing-ui/ }))

  const detail = await screen.findByRole('complementary', { name: 'feat/billing-ui' })
  expect(await within(detail).findByText('feat/billing has 1 commit it lacks')).toBeTruthy()
  expect(await within(detail).findByRole('button', { name: 'Sync it onto feat/billing' })).toBeTruthy()
})

// The header read its link from the list's actions, which the live list, never asked GitHub, does not carry.
test("the detail's header links the worktree's pull request, as GitHub tells it on open", async () => {
  const { detail } = await openDetail('feat/billing')

  const pr = await within(detail).findByRole('link', { name: 'Open pull request #214 on GitHub' })
  expect(pr.getAttribute('href')).toBe('https://github.com/acme/acme/pull/214')
})

const merged = {
  code: 'status.merged' as const,
  message: 'its pull request is merged',
  fix: followUp('clean', 'feat/billing', { code: 'status.merged', message: 'clean it up' }),
}

function serveStatus(live: Parameters<typeof statusDetail>[1], github: Parameters<typeof statusDetail>[1]) {
  server.use(
    http.post('/api/repos/r1/commands/status', async ({ request }) => {
      const asked = await answers(request)
      const over = asked('with_prs') === 'true' ? github : live
      return HttpResponse.json(envelope('status', [statusDetail('feat/billing', over)]))
    }),
  )
}

// The panel put GitHub's merged problem first by a rule of its own, against the one rule the list and the tree follow.
test("a worktree GitHub says is merged shows GitHub's read first, as the one rule over two reads says", async () => {
  serveStatus({ condition: 'changed' }, { condition: 'merged', problems: [merged] })

  const { detail } = await openDetail('feat/billing')

  const callout = await within(detail).findByRole('region', { name: 'To do' })
  expect(within(callout).getByText('Its pull request is merged')).toBeTruthy()
  expect(within(callout).getByRole('button', { name: 'Remove' })).toBeTruthy()
})

test("a fix the web runs is the callout's button, and opens its dialog on that worktree", async () => {
  const missing = {
    code: 'status.missing' as const,
    message: 'its directory is gone',
    fix: followUp('clean', 'feat/billing', { code: 'status.missing', message: 'remove what is left of it' }),
  }
  serveStatus({ missing: true, problems: [missing] }, { missing: true, problems: [missing] })
  const { user, detail } = await openDetail('feat/billing')

  expect(await within(detail).findByText('Its branch and its record go with it; the remote branch stays.')).toBeTruthy()
  await user.click(within(detail).getByRole('button', { name: 'Remove what is left of it' }))

  const dialog = await screen.findByRole('dialog', { name: 'Remove worktrees' })
  expect(await within(dialog).findByRole('listitem', { name: 'feat/billing' })).toBeTruthy()
})

test('a fix the web does not run yet is said, with its command to copy', async () => {
  const crashed = {
    code: 'status.job_crashed' as const,
    message: 'worker crashed (exit 1)',
    fix: followUp('run start', 'feat/billing', { code: 'status.job_crashed', message: 'start it again' }),
  }
  serveStatus({ problems: [crashed] }, { problems: [crashed] })

  const { detail } = await openDetail('feat/billing')

  expect(await within(detail).findByText('Start it again from a terminal.')).toBeTruthy()
  expect(await within(detail).findByText('wtm run start feat/billing')).toBeTruthy()
})

test("the detail's menu is the same list, in the same order, as every other", async () => {
  const { user, detail } = await openDetail('feat/billing')

  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/billing' }))

  const menu = await screen.findByRole('menu')
  expect(
    within(menu)
      .getAllByText(/^(Branch|Run|Open)$/)
      .map((label) => label.textContent),
  ).toEqual(['Branch', 'Run', 'Open'])
  expect(within(menu).getByRole('menuitem', { name: 'Remove' }).getAttribute('aria-disabled')).toBeNull()
})

test('a rebase under way stays first, whatever GitHub says of the pull request', async () => {
  serveStatus({ condition: 'rebasing', problems: [] }, { condition: 'merged', problems: [merged] })

  const { detail } = await openDetail('feat/billing')

  await within(detail).findByText('No pull request.')
  expect(within(detail).queryByText('Its pull request is merged')).toBeNull()
})

// A .env file whose keys were all in step but for a few of its own read "0 missing, 0 differ".
test('a .env file says only how it drifts: keys missing, differing or its own', async () => {
  const env = {
    declared: 1,
    files: [{ target: '.env', missing: false, keys: { missing: 0, conflicting: 0, orphan: 2 } }],
  }
  serveStatus({ env }, { env })

  const { detail } = await openDetail('feat/billing')

  const links = await within(detail).findByRole('region', { name: 'Links' })
  expect(within(links).getByText('.env: 2 of its own')).toBeTruthy()
})

// A GitHub read that failed said "No pull request.", as if GitHub had answered none.
test('a GitHub read that failed says why in the review, not that there is no pull request', async () => {
  server.use(
    http.post('/api/repos/r1/commands/status', async ({ request }) => {
      const asked = await answers(request)
      if (asked('with_prs') !== 'true') return HttpResponse.json(envelope('status', [statusDetail('feat/billing')]))
      return HttpResponse.json({ error: { message: 'the server went away' } }, { status: 503 })
    }),
  )

  const { detail } = await openDetail('feat/billing')

  const review = await within(detail).findByRole('region', { name: 'Review' })
  expect(await within(review).findByText('the server went away')).toBeTruthy()
  expect(within(review).queryByText("GitHub couldn't be asked"), 'the server failed, not GitHub').toBeNull()
  expect(within(review).queryByText('No pull request.')).toBeNull()
})

// A status read with its worktree unreadable left the panel blank.
test('a worktree the read could not read says why in its panel', async () => {
  server.use(http.post('/api/repos/r1/commands/status', () => HttpResponse.json(unreadable('status', 'feat/billing'))))

  const { detail } = await openDetail('feat/billing')

  expect(await within(detail).findByText("Couldn't read /acme.trees/feat/billing")).toBeTruthy()
})
