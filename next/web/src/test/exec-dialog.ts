import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect } from 'vitest'
import { renderApp } from '@/test/render'

type App = Awaited<ReturnType<typeof renderApp>> & { dialog: HTMLElement }

export async function execFromTheHeader(path = '/r/r1/worktrees'): Promise<App> {
  const app = await renderApp(path)
  await app.user.click(await screen.findByRole('button', { name: 'Actions' }))
  await app.user.click(await screen.findByRole('menuitem', { name: 'Run a command' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Run a command' }) }
}

export async function execFromTheSelection(...names: string[]): Promise<App> {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  const bar = screen.getByRole('toolbar', { name: 'Selection' })
  await app.user.click(within(bar).getByRole('button', { name: 'Run a command' }))
  return { ...app, dialog: await screen.findByRole('dialog', { name: 'Run a command' }) }
}

export const nextEnabled = (dialog: HTMLElement) =>
  waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('false'),
  )

// from the worktrees step to Confirm, the command typed and accepted by the shell
export async function typeAndConfirm({ user, dialog }: App, command: string) {
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.type(await within(dialog).findByRole('textbox', { name: 'Command' }), command)
  await nextEnabled(dialog)
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
}

// an event stream the test writes the lock's holder to: under way, then finished, or the stream dropped
export function holderEvents(op: string) {
  const encoder = new TextEncoder()
  let write = (_line: string) => {}
  let close = () => {}
  const send = (type: string, extra: object = {}) =>
    write(
      `data: ${JSON.stringify({ v: 2, type, ts: new Date().toISOString(), repo: { root: '/acme', common_dir: '/acme/.git' }, op_id: op, command: 'clean', surface: 'cli', ...extra })}\n\n`,
    )
  const handler = http.get(
    '/api/events',
    () =>
      new HttpResponse(
        new ReadableStream({
          start: (controller) => {
            controller.enqueue(encoder.encode(': open\n\n'))
            write = (line) => controller.enqueue(encoder.encode(line))
            close = () => controller.close()
          },
        }),
        { headers: { 'Content-Type': 'text/event-stream' } },
      ),
  )
  return {
    handler,
    started: () => {
      send('operation.started')
      send('unit.started', { subject: 'feat/billing' })
    },
    finished: () => send('operation.finished', { ok: true }),
    drop: () => close(),
  }
}
