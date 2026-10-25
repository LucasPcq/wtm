import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { envelope, followUp, statusDetail } from '@/test/fixtures'
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
  expect(await within(detail).findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

// The header read its link from the list's actions, which the live list, never asked GitHub, does not carry.
test("the detail's header links the worktree's pull request, as GitHub tells it on open", async () => {
  const { detail } = await openDetail('feat/billing')

  const pr = await within(detail).findByRole('link', { name: 'Open pull request #214 on GitHub' })
  expect(pr.getAttribute('href')).toBe('https://github.com/acme/acme/pull/214')
})

const merged = {
  code: 'status.merged',
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

  expect(await within(detail).findByText('Its pull request is merged')).toBeTruthy()
  expect(await within(detail).findByText('wtm clean feat/billing')).toBeTruthy()
})

test('a rebase under way stays first, whatever GitHub says of the pull request', async () => {
  serveStatus({ condition: 'rebasing', rebasing: true, problems: [] }, { condition: 'merged', problems: [merged] })

  const { detail } = await openDetail('feat/billing')

  // GitHub's read settled: the review says so, and the rule had both reads to choose from
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
