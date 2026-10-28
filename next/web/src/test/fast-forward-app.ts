import { screen, within } from '@testing-library/react'
import { http } from 'msw'
import { onTestFinished } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { ffPlan, ffRun } from '@/test/fast-forward'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// the calls fast-forward is asked, dry runs and runs, in order
export function recorded() {
  const calls: WebCall[] = []
  const listen = async ({ request }: { request: Request }) => {
    if (request.url.endsWith('/commands/fast-forward')) calls.push((await request.clone().json()) as WebCall)
  }
  server.events.on('request:start', listen)
  onTestFinished(() => server.events.removeListener('request:start', listen))
  return { runs: () => calls.filter((call) => !call.dry_run), dryRuns: () => calls.filter((call) => call.dry_run) }
}

export async function fromHeader() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

export async function fromSelection(...names: string[]) {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  const bar = screen.getByRole('toolbar', { name: 'Selection' })
  await app.user.click(within(bar).getByRole('button', { name: 'Fast-forward' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

// answers fast-forward with the worktrees dirty that the test changes; held, the plans that fetch wait for release
export function answeringOn(dirty: string[]) {
  let hold = false
  let release = () => {}
  server.use(
    http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (!call.dry_run) return ffRun(call)
      const fetches = !call.answers?.some((answer) => answer.path === 'no_fetch')
      if (hold && fetches) await new Promise<void>((resolve) => (release = resolve))
      return ffPlan(call, dirty)
    }),
  )
  return { hold: () => (hold = true), release: () => release() }
}

export const fetching = (call: WebCall) => !call.answers?.some((answer) => answer.path === 'no_fetch')

export const answering = (run: (call: WebCall) => Response) =>
  http.post('/api/repos/r1/commands/fast-forward', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? ffPlan(call) : run(call)
  })

export async function fromMenu(worktree: string) {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: `Actions on ${worktree}` }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Fast-forward' }))
  return app
}
