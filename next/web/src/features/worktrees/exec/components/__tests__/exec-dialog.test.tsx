import { screen, waitFor, within } from '@testing-library/react'
import { delay, HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { WebCall } from '@/api/types.gen'
import { answeringExec, execPlan, execQuestionsOf, execRun } from '@/test/exec'
import { execFromTheHeader, execFromTheSelection, nextEnabled, typeAndConfirm } from '@/test/exec-dialog'
import { liveEvents } from '@/test/remove'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

test('from the header, it opens on every worktree, none checked, then asks the command and shows it before it runs', async () => {
  const { user, dialog } = await execFromTheHeader()

  expect(await within(dialog).findByText('0 of 6 selected')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
  await user.click(within(dialog).getByRole('checkbox', { name: 'Run in feat/billing' }))
  await user.click(within(dialog).getByRole('checkbox', { name: 'Run in feat/search-docs' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('Run by sh in each of the 2 worktrees.')).toBeTruthy()
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
  await user.type(within(dialog).getByRole('textbox', { name: 'Command' }), 'pnpm test')
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('false'),
  )
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const command = await within(dialog).findByRole('region', { name: 'Command' })
  expect(command.textContent).toContain('pnpm test')
  expect(within(dialog).getByText('In 2 worktrees · 2 at once')).toBeTruthy()
  const docs = within(dialog).getByRole('listitem', { name: 'feat/search-docs' })
  expect(within(docs).getByText('Without its run env: it has no number yet')).toBeTruthy()

  await user.click(within(dialog).getByRole('button', { name: 'Run in 2 worktrees' }))

  expect(await within(dialog).findByText('pnpm test passed in all 2.')).toBeTruthy()
  const toCheck = within(dialog).getByRole('region', { name: 'To check' })
  expect(within(toCheck).getByText('Without its run env: it has no number yet')).toBeTruthy()
  expect(within(dialog).getByRole('region', { name: 'Passed' })).toBeTruthy()
})

test('the shell refusing the command says so under it, and Next waits for a line it accepts', async () => {
  const { user, dialog } = await execFromTheSelection('feat/billing')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  await user.type(await within(dialog).findByRole('textbox', { name: 'Command' }), 'pnpm test &&')

  expect(await within(dialog).findByText('sh: syntax error: unexpected end of file')).toBeTruthy()
  const input = within(dialog).getByRole('textbox', { name: 'Command' })
  const described = (input.getAttribute('aria-describedby') ?? '')
    .split(' ')
    .map((id) => document.getElementById(id)?.textContent)
  expect(described).toEqual(['Run by sh in feat/billing.', 'sh: syntax error: unexpected end of file'])
  expect(within(dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
})

test('from the selection, the rows selected start checked', async () => {
  const { dialog } = await execFromTheSelection('feat/billing', 'fix/login')

  expect(await within(dialog).findByText('2 of 6 selected')).toBeTruthy()
  const box = within(dialog).getByRole('checkbox', { name: 'Run in fix/login' })
  expect(box.getAttribute('aria-checked')).toBe('true')
})

test('the address never gives the command: a link that names one opens on an empty line', async () => {
  const { user, dialog } = await execFromTheHeader('/r/r1/worktrees?command=rm%20-rf%20.')
  await user.click(await within(dialog).findByRole('checkbox', { name: 'Run in feat/billing' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const input = await within(dialog).findByRole('textbox', { name: 'Command' })
  expect((input as HTMLInputElement).value).toBe('')
})

test("the detail's menu runs in the worktree it shows, checked", async () => {
  const { user } = await renderApp('/r/r1/worktrees?worktree=feat%2Fbilling')

  const detail = await screen.findByRole('complementary', { name: 'feat/billing' })
  await user.click(await within(detail).findByRole('button', { name: 'Actions on feat/billing' }))
  await user.click(await screen.findByRole('menuitem', { name: 'Run a command' }))

  const dialog = await screen.findByRole('dialog', { name: 'Run a command' })
  expect(await within(dialog).findByText('1 of 6 selected')).toBeTruthy()
  expect(within(dialog).getByRole('checkbox', { name: 'Run in feat/billing' }).getAttribute('aria-checked')).toBe(
    'true',
  )
})

// the security path: what runs is the line the plan last showed, with that plan's fingerprint, never an older one
test('Back, a new command, Next: Run waits for the new plan and sends its line and its fingerprint', async () => {
  const runs: WebCall[] = []
  let slow = false
  server.use(
    ...answeringExec(
      async (call) => {
        if (slow) await delay(200)
        return execPlan(call)
      },
      (call) => {
        runs.push(call)
        return execRun()(call)
      },
    ),
  )
  const app = await execFromTheSelection('feat/billing')
  await typeAndConfirm(app, 'pnpm test')
  const { user, dialog } = app
  await within(dialog).findByRole('region', { name: 'Command' })
  await user.click(within(dialog).getByRole('button', { name: 'Back' }))
  const input = await within(dialog).findByRole('textbox', { name: 'Command' })
  await user.clear(input)
  await user.type(input, 'pnpm lint')
  await nextEnabled(dialog)
  slow = true
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const run = within(dialog).getByRole('button', { name: 'Run' })
  expect(run.getAttribute('aria-disabled')).toBe('true')
  await user.hover(run)
  expect(await screen.findByRole('tooltip', { name: 'Planning…' })).toBeTruthy()
  expect((await within(dialog).findByRole('region', { name: 'Command' })).textContent).toContain('pnpm lint')
  await user.click(within(dialog).getByRole('button', { name: 'Run' }))

  expect(await within(dialog).findByText('pnpm lint passed in feat/billing.')).toBeTruthy()
  expect(
    runs.map((call) => [call.answers?.find((answer) => answer.path === 'command')?.values, call.expect_fingerprint]),
  ).toEqual([[['pnpm lint'], 'e:pnpm lint']])
})

// A worktree the plan ran in went (a clean removed it): Run counted 0, and back on Worktrees it had no row to uncheck.
test('a checked worktree that goes is left as it is, out of the request, and with none left Next waits', async () => {
  const events = liveEvents()
  const asked: WebCall[] = []
  const app = await execFromTheSelection('feat/billing', 'feat/search')
  await typeAndConfirm(app, 'pnpm test')
  await within(app.dialog).findByRole('region', { name: 'Command' })

  server.use(
    http.post('/api/repos/r1/commands/exec/questions', async ({ request }) => {
      const answered = execQuestionsOf(((await request.json()) as WebCall).answers)
      return HttpResponse.json({
        ...answered,
        fields: answered.fields.map((field) =>
          field.kind === 'multi_select'
            ? { ...field, options: field.options.filter((option) => option.value !== 'feat/billing') }
            : field,
        ),
      })
    }),
    http.post('/api/repos/r1/commands/exec', async ({ request }) => {
      const call = (await request.json()) as WebCall
      asked.push(call)
      return execPlan(call)
    }),
  )
  events.changed()

  const left = await within(app.dialog).findByRole('region', { name: 'Left as they are' })
  expect(within(left).getByText('No longer exists')).toBeTruthy()
  expect(await within(app.dialog).findByRole('button', { name: 'Run' })).toBeTruthy()
  expect(asked.at(-1)?.answers?.find((answer) => answer.path === 'worktree')?.values).toEqual(['feat/search'])

  await app.user.click(within(app.dialog).getByRole('button', { name: 'Back' }))
  await app.user.click(within(app.dialog).getByRole('button', { name: 'Back' }))

  expect(await within(app.dialog).findByText('1 of 5 selected')).toBeTruthy()
  expect(
    within(within(app.dialog).getByRole('region', { name: 'Left as they are' })).getByText('feat/billing'),
  ).toBeTruthy()
  await app.user.click(within(app.dialog).getByRole('checkbox', { name: 'Run in feat/search' }))
  expect(within(app.dialog).getByText('0 of 5 selected')).toBeTruthy()
  expect(within(app.dialog).getByRole('button', { name: 'Next' }).getAttribute('aria-disabled')).toBe('true')
})
