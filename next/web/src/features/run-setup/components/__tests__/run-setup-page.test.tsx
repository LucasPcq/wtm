import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { runInitApplied, runInitEnvelope, runInitPlan } from '@/test/run-init'
import { server } from '@/test/server'

type User = Awaited<ReturnType<typeof renderApp>>['user']

const parts = ['What runs', 'Shared or per worktree', 'Ports', 'Addresses', '.env files', 'Profiles']

// each part opened and continued in turn, from the overview to the Write block
const throughEveryPart = async (user: User) => {
  await user.click(await screen.findByRole('button', { name: 'Start with What runs' }))
  for (const [at, part] of parts.entries()) {
    await screen.findByRole('region', { name: part })
    await user.click(screen.getByRole('button', { name: `Continue to ${parts[at + 1] ?? 'Write'}` }))
  }
}

test('a first init opens on the overview, each part greyed until the one before is answered', async () => {
  await renderApp('/r/r1/run-setup')

  expect(await screen.findByRole('heading', { name: "What you'll set up" })).toBeDefined()
  expect(screen.getByText(/wtm read acme: 6 scripts, 3 services in docker-compose.yml, 5 .env files/)).toBeDefined()
  expect(
    within(screen.getByRole('region', { name: 'Shared or per worktree' })).getByText('After What runs'),
  ).toBeDefined()
  expect(screen.getByRole('button', { name: /Shared or per worktree/ }).hasAttribute('disabled')).toBe(true)
  expect(screen.getByRole('button', { name: 'Write 2 files' }).hasAttribute('disabled')).toBe(true)
})

test('the parts answered fold into their summary, and Write sends the answers with the plan it was shown', async () => {
  const calls: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/run-init', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) calls.push(call)
      return HttpResponse.json(call.dry_run ? runInitEnvelope(call) : runInitApplied)
    }),
  )
  const { user } = await renderApp('/r/r1/run-setup')

  await throughEveryPart(user)

  const runs = screen.getByRole('region', { name: 'What runs' })
  expect(within(runs).getByText('9 jobs: 3 from compose, 6 from scripts')).toBeDefined()
  expect(screen.getByText('postgres and mailpit shared; redis per worktree')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Write 2 files' }))

  expect(await screen.findByRole('heading', { name: 'Run is set up' })).toBeDefined()
  expect(screen.getByText('wtm run up starts the profile all in any worktree.')).toBeDefined()
  expect(calls).toEqual([{ answers: [], expect_fingerprint: 'f-run-init' }])
})

test('Use this command answers the problem with the command the core proposes, never one the page spells', async () => {
  const sent: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/run-init', async ({ request }) => {
      const call = (await request.json()) as WebCall
      sent.push(call)
      return HttpResponse.json(runInitEnvelope(call))
    }),
  )
  const command = runInitPlan.parts.commands?.[0]
  const { user } = await renderApp('/r/r1/run-setup')
  await user.click(await screen.findByRole('button', { name: 'Start with What runs' }))
  await user.click(await screen.findByRole('button', { name: 'Continue to Shared or per worktree' }))
  await user.click(await screen.findByRole('button', { name: 'Continue to Ports' }))

  const ports = await screen.findByRole('region', { name: 'Ports' })
  expect(within(ports).getByText('To check · 1')).toBeDefined()
  expect(within(ports).getByText(command?.proposed ?? '')).toBeDefined()
  await user.click(within(ports).getByRole('button', { name: 'Use this command' }))

  await waitFor(() => expect(within(ports).queryByText('To check · 1')).toBeNull())
  expect(sent.at(-1)?.answers).toEqual([{ path: 'commands', values: [command?.spelled] }])
})

test('the preview shows each port with how it is worked out, for a worktree never started', async () => {
  const { user } = await renderApp('/r/r1/run-setup')

  const preview = await screen.findByRole('complementary', { name: 'Preview' })
  expect(within(preview).getByText(/It gets worktree number 1 when it first runs/)).toBeDefined()
  expect(within(preview).getByText('Decided in Ports')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Start with What runs' }))
  await user.click(await screen.findByRole('button', { name: 'Continue to Shared or per worktree' }))
  await user.click(await screen.findByRole('button', { name: 'Continue to Ports' }))

  expect(await within(preview).findByText('3001 + 1 × 10')).toBeDefined()
  expect(within(preview).getAllByText('shared: the same in every worktree').length).toBe(3)
})

test('a file changed on disk since it was read writes nothing, and keeps the answers to read it again', async () => {
  server.use(
    http.post('/api/repos/r1/commands/run-init', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (call.dry_run) return HttpResponse.json(runInitEnvelope(call))
      return HttpResponse.json({
        ...runInitApplied,
        ok: false,
        items: [
          {
            subject: 'docker-compose.yml',
            status: 'failed',
            detail: {},
            error: { kind: 'conflict', code: 'run-init.file_changed', message: 'docker-compose.yml changed' },
          },
          { subject: 'run.toml', status: 'skipped', detail: {} },
        ],
      })
    }),
  )
  const { user } = await renderApp('/r/r1/run-setup')

  await throughEveryPart(user)
  await user.click(screen.getByRole('button', { name: 'Write 2 files' }))

  expect(await screen.findByText("Couldn't write: docker-compose.yml changed on disk since it was read")).toBeDefined()
  expect(screen.getByText('Nothing was written. Your answers are kept.')).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Read docker-compose.yml again' }))
  expect(await screen.findByRole('button', { name: 'Write 2 files' })).toBeDefined()
})
