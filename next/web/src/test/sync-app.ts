import { screen, within } from '@testing-library/react'
import { onTestFinished } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

// the calls sync is asked, dry runs and runs, in order
export function recorded() {
  const calls: WebCall[] = []
  const listen = async ({ request }: { request: Request }) => {
    if (request.url.endsWith('/commands/sync')) calls.push((await request.clone().json()) as WebCall)
  }
  server.events.on('request:start', listen)
  onTestFinished(() => server.events.removeListener('request:start', listen))
  return { runs: () => calls.filter((call) => !call.dry_run), dryRuns: () => calls.filter((call) => call.dry_run) }
}

export async function fromMenu(worktree: string) {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: `Actions on ${worktree}` }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Sync' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

export async function fromHeader() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Sync' }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

export const order = async (dialog: HTMLElement) =>
  within(await within(dialog).findByRole('region', { name: 'In this order' }))
    .getAllByRole('listitem')
    .map((row) => row.textContent)
