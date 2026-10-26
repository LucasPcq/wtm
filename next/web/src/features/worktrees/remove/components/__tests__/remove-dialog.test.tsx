import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { cleanPlan, cleanRun } from '@/test/clean'
import { envelope, followUp } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server, worktrees } from '@/test/server'

async function selectAndRemove(...names: string[]) {
  const app = await renderApp('/r/r1/worktrees')
  for (const name of names) await app.user.click(await screen.findByRole('checkbox', { name: `Select ${name}` }))
  const bar = screen.getByRole('toolbar', { name: 'Selection' })
  expect(within(bar).getByText(`${names.length} selected`)).toBeTruthy()
  await app.user.click(within(bar).getByRole('button', { name: /^Remove/ }))
  return { ...app, dialog: await screen.findByRole('dialog') }
}

test('an unsafe worktree waits for a choice; kept, the others go through the children and the plan, then run', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing', 'fix/login')

  const unsafe = await within(dialog).findByRole('region', { name: 'fix/login' })
  expect(within(unsafe).getByText('Uncommitted changes')).toBeTruthy()
  expect(within(dialog).getByText("1 of 2 can't be removed safely: choose for each before going on.")).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)

  await user.click(within(unsafe).getByRole('radio', { name: 'Keep it' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('feat/billing-ui is stacked on feat/billing, which goes.')).toBeTruthy()
  const children = within(dialog).getByRole('region', { name: 'Under feat/billing' })
  expect(within(children).getByText('stays on feat/billing, which goes')).toBeTruthy()
  await user.click(within(dialog).getByRole('radio', { name: /Move it onto the nearest worktree that stays/ }))
  expect(within(children).getByText('base branch')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const planned = await within(dialog).findByRole('region', { name: 'feat/billing' })
  expect(within(planned).getByText('feat/billing-ui moves onto main')).toBeTruthy()
  expect(within(planned).getByText('docker compose down')).toBeTruthy()
  expect(within(dialog).queryByRole('region', { name: 'fix/login' })).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: 'Remove feat/billing' }))

  expect(await within(dialog).findByText('feat/billing is removed')).toBeTruthy()
  expect(
    within(dialog).getByText('Removed, with its branch and its record; feat/billing-ui moved onto main.'),
  ).toBeTruthy()
  expect(within(dialog).queryByText(/still points at the removed parent/)).toBeNull()
  expect(screen.queryByRole('toolbar', { name: 'Selection' })).toBeNull()
})

test('removing an unsafe worktree anyway says what is lost and forces it', async () => {
  const { user, dialog } = await selectAndRemove('fix/login')

  const unsafe = await within(dialog).findByRole('region', { name: 'fix/login' })
  await user.click(within(unsafe).getByRole('radio', { name: 'Remove anyway' }))

  expect(within(unsafe).getByText('Removed anyway: its uncommitted changes are lost.')).toBeTruthy()
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  const planned = await within(dialog).findByRole('region', { name: 'fix/login' })
  expect(within(planned).getByText('Removed anyway')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Remove fix/login' }).hasAttribute('disabled')).toBe(false)
})

test("the detail's menu removes the worktree it shows, and a child left on it is named with its command", async () => {
  server.use(
    http.post('/api/repos/r1/commands/list', () =>
      HttpResponse.json(
        envelope(
          'list',
          worktrees.map((w) => ({
            ...w,
            actions: w.main
              ? []
              : [followUp('clean', w.branch, { code: 'list.clean', message: 'Clean (delete the worktree)' })],
          })),
        ),
      ),
    ),
  )
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  await user.click(await screen.findByRole('button', { name: 'More actions' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Remove worktree…' }))
  const dialog = await screen.findByRole('dialog')
  expect(await within(dialog).findByRole('heading', { name: 'Remove feat/billing' })).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing' }))

  expect(
    await within(dialog).findByText('feat/billing-ui still points at the removed parent feat/billing'),
  ).toBeTruthy()
  expect(await within(dialog).findByText('wtm reparent feat/billing-ui --to main')).toBeTruthy()
})

const answeringClean = (dryRun: (call: WebCall) => Response | Promise<Response>) =>
  http.post('/api/repos/r1/commands/clean', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? dryRun(call) : cleanRun(call)
  })

test('a run refused as stale says why, and does not say each worktree went', async () => {
  server.use(
    answeringClean(async (call) => HttpResponse.json({ ...(await cleanPlan(call).json()), fingerprint: 'c0' })),
  )
  const { user, dialog } = await selectAndRemove('feat/billing-ui')

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))

  expect(await within(dialog).findByText("Couldn't remove feat/billing-ui")).toBeTruthy()
  expect(within(dialog).getByText('the facts changed since the preview')).toBeTruthy()
  expect(within(dialog).queryByText('Each went with its local branch and its record.')).toBeNull()
})

test('closed once it ended, a run leaves the next Remove to its own worktrees', async () => {
  const { user, dialog } = await selectAndRemove('feat/billing-ui')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove feat/billing-ui' }))
  await within(dialog).findByText('feat/billing-ui is removed')

  await user.keyboard('{Escape}')
  await user.click(await screen.findByRole('checkbox', { name: 'Select fix/login' }))
  await user.click(within(screen.getByRole('toolbar', { name: 'Selection' })).getByRole('button', { name: /^Remove/ }))

  expect(
    await within(await screen.findByRole('dialog')).findByRole('heading', { name: 'Remove fix/login' }),
  ).toBeTruthy()
})

test('a plan that cannot be made says why instead of a Next that never wakes', async () => {
  server.use(
    answeringClean((call) =>
      call.answers?.some((a) => a.path === 'force')
        ? cleanPlan(call)
        : HttpResponse.json({ error: { message: 'git is busy' } }, { status: 500 }),
    ),
  )
  const { dialog } = await selectAndRemove('feat/billing-ui')

  expect(await within(dialog).findByText('git is busy')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)
})

test('a worktree read again unsafe on a later step brings the wizard back to its choice', async () => {
  const encoder = new TextEncoder()
  let push = (_line: string) => {}
  server.use(
    http.get(
      '/api/events',
      () =>
        new HttpResponse(
          new ReadableStream({
            start: (controller) => {
              controller.enqueue(encoder.encode(': open\n\n'))
              push = (line) => controller.enqueue(encoder.encode(line))
            },
          }),
          { headers: { 'Content-Type': 'text/event-stream' } },
        ),
    ),
  )
  const { user, dialog } = await selectAndRemove('feat/billing')
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await within(dialog).findByText('Children left behind')

  server.use(answeringClean((call) => cleanPlan(call, ['feat/billing'])))
  push('data: {"v":2,"type":"worktree.updated","repo":{"root":"/acme","common_dir":"/acme/.git"}}\n\n')

  expect(
    await within(dialog).findByText("1 of 1 can't be removed safely: choose for each before going on."),
  ).toBeTruthy()
  expect(
    within(within(dialog).getByRole('region', { name: 'feat/billing' })).getByText('Uncommitted changes'),
  ).toBeTruthy()
})
