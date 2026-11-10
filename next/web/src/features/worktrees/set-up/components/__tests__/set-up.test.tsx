import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { answeringSetUp } from '@/test/set-up'

async function openSetUp(options: { fails?: boolean } = {}) {
  const answering = answeringSetUp(options)
  server.use(...answering.handlers)
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Set up acme' }))
  const dialog = await screen.findByRole('dialog')
  return { ...app, dialog, state: answering.state }
}

test("a repository not set up opens on its gate, the sidebar's pages waiting for it", async () => {
  server.use(...answeringSetUp().handlers)
  await renderApp('/r/r1/worktrees')

  expect(await screen.findByText(/This repository isn't set up yet/)).toBeDefined()
  expect(screen.getByRole('button', { name: 'Set up acme' })).toBeDefined()
  expect(screen.getByRole('button', { name: 'All worktrees' }).hasAttribute('disabled')).toBe(true)
  expect(screen.queryByRole('button', { name: 'Create worktree' })).toBeNull()
})

test('each step shows what the repository suggests, then Confirm the settings the dry run settles', async () => {
  const { user, dialog } = await openSetUp()

  expect(await within(dialog).findByDisplayValue('../.trees')).toBeDefined()
  expect(within(dialog).getByText('The default: beside acme, outside its folder.')).toBeDefined()
  expect(
    within(dialog).getByText('Found in the repository. Changing it later only affects new worktrees.'),
  ).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const template = await within(dialog).findByRole('radio', { name: /Copy the template/ })
  expect(template.getAttribute('aria-checked')).toBe('true')
  expect(within(dialog).getByText('From its committed template. Found 2.')).toBeDefined()
  expect(within(dialog).getByText('apps/web/.env')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByDisplayValue('pnpm install')).toBeDefined()
  expect(within(dialog).getByText('Found in pnpm-lock.yaml.')).toBeDefined()
  expect(within(dialog).getByText('Found in docker-compose.yml.')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))

  const created = await within(dialog).findByRole('list', { name: 'When a worktree is created, in order' })
  expect(within(created).getByText('pnpm db:generate')).toBeDefined()
  expect(within(created).getByText('in apps/api')).toBeDefined()
  expect(within(created).getByText("in the worktree's root")).toBeDefined()
  expect(within(dialog).getByText('Copy the template')).toBeDefined()
})

test('a file removed and a command added are what the set-up sends, under the dry run it confirmed', async () => {
  const { user, dialog, state } = await openSetUp()

  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Remove apps/web/.env' }))
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  const created = await within(dialog).findByRole('list', { name: 'When a worktree is created' })
  await user.click(within(created).getByRole('button', { name: 'Add a command' }))
  const commands = within(created).getAllByRole('textbox', { name: 'Command' })
  await user.type(commands[1] as HTMLElement, 'pnpm db:generate')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Set up acme' }))

  await waitFor(() => expect(state.sent).toHaveLength(1))
  expect(state.sent[0]?.expect_fingerprint).toBe('fp-1')
  expect(state.sent[0]?.answers).toContainEqual({ path: 'env_files', values: ['apps/api/.env'] })
  expect(state.sent[0]?.answers).toContainEqual({ path: 'on_create', values: ['pnpm install', 'pnpm db:generate'] })
  expect(await screen.findByRole('button', { name: 'Create worktree' })).toBeDefined()
  expect(await screen.findByText('Next, set up run: what runs, its ports and addresses.')).toBeDefined()
})

test('a write that fails keeps the answers, says why, and offers to set up again', async () => {
  const { user, dialog } = await openSetUp({ fails: true })

  for (let step = 0; step < 3; step++) await user.click(await within(dialog).findByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Set up acme' }))

  expect(await within(dialog).findByText("Couldn't save acme's settings")).toBeDefined()
  expect(within(dialog).getByText(/Permission denied on .git\/wtm\/config.toml/)).toBeDefined()
  expect(within(dialog).getByText('Nothing was written.')).toBeDefined()
  expect(within(dialog).getByRole('button', { name: 'Set up again' })).toBeDefined()
})
