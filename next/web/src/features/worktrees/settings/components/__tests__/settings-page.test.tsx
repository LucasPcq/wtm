import { focusManager } from '@tanstack/react-query'
import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { ConfigshowDetail, DomainEnvelope, DomainError, WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { settingsDetail } from '@/test/settings'

const shows = (detail: ConfigshowDetail) =>
  server.use(
    http.post('/api/repos/r1/commands/config-show', () =>
      HttpResponse.json({
        v: 2,
        ok: true,
        command: 'config-show',
        warnings: [],
        follow_ups: [],
        items: [{ subject: 'config.toml', status: 'done', detail: detail as never }],
      } satisfies DomainEnvelope),
    ),
  )

const refuses = (status: number, error: DomainError) =>
  server.use(
    http.post('/api/repos/r1/commands/config-show', () =>
      HttpResponse.json({ v: 2, ok: false, command: 'config-show', error }, { status }),
    ),
  )

// each call config set receives, its questions answered from the document, its apply refused by refuse
const setting = (refuse?: (call: WebCall) => DomainError | undefined) => {
  const calls: WebCall[] = []
  const fields = settingsDetail().settings.fields
  server.use(
    http.post('/api/repos/r1/commands/config-set/questions', () => HttpResponse.json({ fields, problems: [] })),
    http.post('/api/repos/r1/commands/config-set', async ({ request }) => {
      const call = (await request.json()) as WebCall
      calls.push(call)
      const refusal = call.dry_run ? undefined : refuse?.(call)
      if (refusal) return HttpResponse.json({ v: 2, ok: false, command: 'config-set', error: refusal }, { status: 409 })
      const undo = {
        command: 'config-set',
        answers: [
          { path: 'env_strategy', values: ['example'] },
          { path: 'replaces', values: ['d2'] },
        ],
        code: 'config-set.undo',
        message: 'put back what it held',
      }
      return HttpResponse.json({
        v: 2,
        ok: true,
        command: 'config-set',
        fingerprint: 'fp',
        warnings: [],
        items: call.dry_run ? [] : [{ subject: 'config.toml', status: 'done', detail: {} }],
        follow_ups: call.dry_run ? [] : [undo],
      } satisfies DomainEnvelope)
    }),
  )
  return calls
}

test('the document shows each setting, its default and its remembered answers', async () => {
  shows(settingsDetail())
  await renderApp('/r/r1/settings')

  expect((await screen.findByRole('button', { name: 'Change Folder for new worktrees' })).textContent).toContain(
    '../.trees',
  )
  expect(screen.getByRole('button', { name: 'Change Folder for new worktrees' }).textContent).toContain('default')
  expect(screen.getByRole('radio', { name: /Copy the template/ }).getAttribute('aria-checked')).toBe('true')
  expect(screen.getByText('pnpm db:generate')).toBeTruthy()
  expect(screen.getByText('in packages/db')).toBeTruthy()
  expect(screen.getByText("Where .env files come from: the parent's")).toBeTruthy()
})

test('a choice saves at once on the file it was made on, its toast undoes it on the file it wrote', async () => {
  shows(settingsDetail())
  const calls = setting()
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('radio', { name: /Copy main's/ }))

  await waitFor(() => expect(calls).toHaveLength(2))
  expect(calls[1]).toEqual({
    answers: [
      { path: 'env_strategy', values: ['main'] },
      { path: 'replaces', values: ['d1'] },
    ],
    expect_fingerprint: 'fp',
  })
  await user.click(await screen.findByRole('button', { name: 'Undo' }))
  await waitFor(() => expect(calls).toHaveLength(4))
  expect(calls[3]?.answers).toEqual([
    { path: 'env_strategy', values: ['example'] },
    { path: 'replaces', values: ['d2'] },
  ])
})

test('forgetting a remembered answer keeps the others', async () => {
  shows(
    settingsDetail({
      remembered: [
        { question: 'env_strategy', value: 'parent', entry: 'env_strategy=parent' },
        { question: 'source_update', value: 'ff', entry: 'source_update=ff' },
      ],
    }),
  )
  const calls = setting()
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('button', { name: "Forget Where .env files come from: the parent's" }))

  await waitFor(() => expect(calls).toHaveLength(2))
  expect(calls[1]?.answers?.[0]).toEqual({ path: 'remembered', values: ['source_update=ff'] })
})

test('a file changed on disk while editing is not written over until Save mine', async () => {
  shows(settingsDetail())
  const calls = setting((call) =>
    call.answers?.some(({ path }) => path === 'replaces')
      ? { kind: 'conflict', code: 'config-set.changed_on_disk', message: 'config.toml changed on disk' }
      : undefined,
  )
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('button', { name: 'Change Folder for new worktrees' }))
  const dialog = await screen.findByRole('dialog')
  const input = within(dialog).getByRole('textbox')
  await user.clear(input)
  await user.type(input, '../worktrees')
  await user.click(within(dialog).getByRole('button', { name: 'Save' }))

  expect(await within(dialog).findByText('Changed on disk while you were editing')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Save mine' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  const last = calls.at(-1)
  expect(last?.answers).toEqual([{ path: 'base_path', values: ['../worktrees'] }])
})

test('settings a newer wtm wrote are read, not changed', async () => {
  shows(settingsDetail({ newer: true }))
  setting()
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('button', { name: 'Change Folder for new worktrees' }))
  const dialog = await screen.findByRole('dialog')

  expect(within(dialog).getByText('Written by a newer wtm')).toBeTruthy()
  expect(within(dialog).getByRole<HTMLButtonElement>('button', { name: 'Save' }).disabled).toBe(true)
})

test('no settings yet offers init', async () => {
  refuses(412, { kind: 'precondition', code: 'config-show.config_not_found', message: 'no wtm config found' })
  await renderApp('/r/r1/settings')

  expect(await screen.findByText('No settings yet')).toBeTruthy()
})

test("settings that don't parse say where and offer the edit", async () => {
  refuses(412, {
    kind: 'precondition',
    code: 'config-show.config_invalid',
    message: 'invalid config.toml',
    params: { line: '14', setting: 'base_branch' },
  })
  await renderApp('/r/r1/settings')

  expect(await screen.findByText("The settings don't parse")).toBeTruthy()
  expect(screen.getByText(/Line 14: “Branch they start from” doesn't parse/)).toBeTruthy()
})

// the dialog opened again after a hand edit started from the list it had read before: saved over the new file, it
// dropped the command added by hand
test('a list changed by hand since the dialog last opened is the one it opens on again', async () => {
  const onDisk = { digest: 'd1', hooks: [{ cmd: 'pnpm install' }] }
  const read = () => {
    const detail = settingsDetail({ digest: onDisk.digest })
    const fields = detail.settings.fields.map((field) =>
      field.path === 'on_create' ? { ...field, entries: onDisk.hooks } : field,
    )
    return { ...detail, settings: { ...detail.settings, fields } }
  }
  const calls: WebCall[] = []
  server.use(
    http.post('/api/repos/r1/commands/config-show', () =>
      HttpResponse.json({
        v: 2,
        ok: true,
        command: 'config-show',
        warnings: [],
        follow_ups: [],
        items: [{ subject: 'config.toml', status: 'done', detail: read() as never }],
      } satisfies DomainEnvelope),
    ),
    http.post('/api/repos/r1/commands/config-set/questions', () =>
      HttpResponse.json({ fields: read().settings.fields, problems: [] }),
    ),
    http.post('/api/repos/r1/commands/config-set', async ({ request }) => {
      calls.push((await request.json()) as WebCall)
      return HttpResponse.json({ v: 2, ok: true, command: 'config-set', fingerprint: 'fp', warnings: [], items: [] })
    }),
  )
  const { user } = await renderApp('/r/r1/settings')
  const change = 'Change the list: When a worktree is created'
  await user.click(await screen.findByRole('button', { name: change }))
  await within(await screen.findByRole('dialog')).findByDisplayValue('pnpm install')
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

  onDisk.digest = 'd2'
  onDisk.hooks = [{ cmd: 'pnpm install' }, { cmd: 'pnpm build' }]
  focusManager.setFocused(false)
  focusManager.setFocused(true)
  expect(await screen.findByText('pnpm build')).toBeTruthy()

  await user.click(screen.getByRole('button', { name: change }))
  const dialog = await screen.findByRole('dialog')
  expect(await within(dialog).findByDisplayValue('pnpm build')).toBeTruthy()
  await user.click(within(dialog).getByRole('button', { name: 'Add a command' }))
  await user.type(within(dialog).getAllByRole('textbox', { name: 'Command' }).at(-1) as HTMLElement, 'pnpm test')
  await user.click(within(dialog).getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(calls.some((call) => !call.dry_run)).toBe(true))
  const sent = calls.find((call) => !call.dry_run)?.answers ?? []
  expect(sent.find(({ path }) => path === 'replaces')?.values).toEqual(['d2'])
  expect(JSON.stringify(sent.find(({ path }) => path === 'on_create')?.values)).toContain('pnpm build')
})

// a remembered answer wtm cannot keep was listed as if it were one Create uses
test('a remembered answer that cannot be kept says so and offers the edit', async () => {
  shows(
    settingsDetail({
      remembered: [
        {
          question: 'env_strategy',
          value: 'copy',
          entry: 'env_strategy=copy',
          problem: {
            path: 'remembered.env_strategy',
            code: 'config-show.remembered_not_allowed',
            params: { value: 'copy' },
            flag: '',
            message: 'wizard.remembered.env_strategy cannot remember copy',
          },
        },
      ],
    }),
  )
  await renderApp('/r/r1/settings')

  expect(await screen.findByText('“copy” is no answer this question can remember.')).toBeTruthy()
  expect(await screen.findByRole('button', { name: 'Copy wtm config-edit' })).toBeTruthy()
})
