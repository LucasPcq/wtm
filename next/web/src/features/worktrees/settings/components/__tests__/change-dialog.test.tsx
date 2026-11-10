import { focusManager } from '@tanstack/react-query'
import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import type { DomainEnvelope, WebCall } from '@/api/types.gen'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { setting, settingsDetail, shows } from '@/test/settings'

test('a file changed on disk while editing is not written over until Save mine, held to the file it shows', async () => {
  shows(settingsDetail())
  const calls = setting((call) => {
    if (!call.answers?.some(({ path, values }) => path === 'replaces' && values[0] === 'd1')) return undefined
    shows(settingsDetail({ digest: 'd2' }))
    return { kind: 'conflict', code: 'config-set.changed_on_disk', message: 'config.toml changed on disk' }
  })
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
  expect(last?.answers).toEqual([
    { path: 'base_path', values: ['../worktrees'] },
    { path: 'replaces', values: ['d2'] },
  ])
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
