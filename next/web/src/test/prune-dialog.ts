import { screen } from '@testing-library/react'
import { http } from 'msw'
import type { DomainAnswered, WebCall } from '@/api/types.gen'
import { prunePlan, pruneRun } from '@/test/prune'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// prune answered by the test, every call kept to read what the web asked
export function answeringPrune(
  dryRun: (call: WebCall) => Response | Promise<Response> = prunePlan,
  run: (call: WebCall) => Response = pruneRun,
) {
  const calls: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/prune', async ({ request }) => {
      const call = (await request.json()) as WebCall
      calls.push(call)
      return call.dry_run ? dryRun(call) : run(call)
    }),
  )
  const answered = (call: WebCall | undefined, path: string) =>
    call?.answers?.find((answer: DomainAnswered) => answer.path === path)?.values
  return { calls, answered, applied: () => calls.find((call) => !call.dry_run) }
}

export async function openFromTheHeader() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Prune' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Prune worktrees' }) }
}
