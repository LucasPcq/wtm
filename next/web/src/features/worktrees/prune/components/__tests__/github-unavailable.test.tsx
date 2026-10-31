import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope, statusDetail } from '@/test/fixtures'
import { prunePlan } from '@/test/prune'
import { answeringPrune, openFromTheHeader } from '@/test/prune-dialog'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

const signedOut = { reason: 'signed_out' }

// GitHub unavailable said the core's raw message, its fix in backticks: every surface now words it as one
test("prune's plan with gh signed out says what it goes without, its fix on the title line", async () => {
  answeringPrune((call) =>
    prunePlan(call, {
      found: [],
      warnings: [
        {
          code: 'prune.prs_unavailable',
          message: "couldn't ask for pull requests (pull requests unavailable: run `gh auth login`): …",
          params: signedOut,
        },
      ],
    }),
  )
  const { dialog } = await openFromTheHeader()

  const alert = await within(dialog).findByRole('alert')
  expect(within(alert).getByText("gh isn't signed in to GitHub")).toBeTruthy()
  expect(
    within(alert).getByText("Merged and closed pull requests aren't known, so only branches deleted on origin count."),
  ).toBeTruthy()
  expect(within(alert).getByRole('button', { name: 'Copy gh auth login' })).toBeTruthy()
})

test("the detail's review with gh signed out says it in GitHub's words", async () => {
  server.use(
    http.post('/api/repos/r1/commands/status', async ({ request }) => {
      const call = (await request.json()) as { answers?: { path: string; values: string[] }[] }
      const withPRs = call.answers?.some((a) => a.path === 'with_prs')
      return HttpResponse.json({
        ...envelope('status', [statusDetail('feat/billing')]),
        warnings: withPRs
          ? [
              {
                code: 'status.prs_unavailable',
                message: 'pull requests unavailable: run `gh auth login`',
                params: signedOut,
              },
            ]
          : [],
      })
    }),
  )
  await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  const review = await screen.findByRole('region', { name: 'Review' })
  expect(await within(review).findByText("gh isn't signed in to GitHub")).toBeTruthy()
  expect(within(review).getByText("Its pull request and its checks aren't known.")).toBeTruthy()
  expect(within(review).getByRole('button', { name: 'Copy gh auth login' })).toBeTruthy()
})
