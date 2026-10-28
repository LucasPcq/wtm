import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { cleanRun } from '@/test/clean'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

export async function selectAndRemove(...names: string[]) {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  const bar = screen.getByRole('toolbar', { name: 'Selection' })
  expect(within(bar).getByText(`${names.length} selected`)).toBeTruthy()
  await app.user.click(within(bar).getByRole('button', { name: /^Remove/ }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

// clean answered by the test: its dry runs by dryRun, its runs by run
export const answeringClean = (
  dryRun: (call: WebCall) => Response | Promise<Response>,
  run: (call: WebCall) => Response = cleanRun,
) =>
  http.post('/api/repos/r1/commands/clean', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? dryRun(call) : run(call)
  })

// an event stream the test writes to: each worktree line read invalidates the repository's reads
export function liveEvents() {
  const encoder = new TextEncoder()
  let push = () => {}
  let broken = () => {}
  server.use(
    http.get(
      '/api/events',
      () =>
        new HttpResponse(
          new ReadableStream({
            start: (controller) => {
              controller.enqueue(encoder.encode(': open\n\n'))
              broken = () => controller.close()
              push = () =>
                controller.enqueue(
                  encoder.encode(
                    'data: {"v":2,"type":"worktree.updated","repo":{"root":"/acme","common_dir":"/acme/.git"}}\n\n',
                  ),
                )
            },
          }),
          { headers: { 'Content-Type': 'text/event-stream' } },
        ),
    ),
  )
  return { changed: () => push(), broken: () => broken() }
}
