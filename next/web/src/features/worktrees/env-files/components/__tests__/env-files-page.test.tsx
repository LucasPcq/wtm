import { screen, waitFor, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { answeringEnv } from '@/test/env'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

test('the page reads the worktree in its toolbar: the keys to check first, then each key by app and file', async () => {
  await renderApp('/r/r1/env')

  const toCheck = await screen.findByRole('list', { name: 'To check' })
  expect(within(toCheck).getByText("follows nothing: looks like stack's MAILPIT_UI")).toBeTruthy()
  expect(within(toCheck).getByText('out of step: 3010 here, wtm writes 3020')).toBeTruthy()
  const port = screen.getByRole('row', { name: 'PORT' })
  expect(within(port).getByText("web's port")).toBeTruthy()
  expect(within(port).getByText('→ 3020')).toBeTruthy()
  expect(screen.getByText('not copied: its source has none')).toBeTruthy()
})

test('a value wtm does not write is withheld until Show values asks for it', async () => {
  const { user, router } = await renderApp('/r/r1/env')

  const secret = await screen.findByRole('row', { name: 'JWT_SECRET' })
  expect(within(secret).queryByText('s3cret')).toBeNull()
  expect(within(secret).getAllByText('••••••••')).toHaveLength(2)

  await user.click(screen.getByRole('switch', { name: 'Show values' }))

  await waitFor(() => expect(router.state.location.search).toEqual({ values: 'shown' }))
  expect(await within(await screen.findByRole('row', { name: 'JWT_SECRET' })).findAllByText('s3cret')).toHaveLength(2)
})

test('another worktree is picked from the toolbar, and the address keeps it', async () => {
  const { user, router } = await renderApp('/r/r1/env')

  await user.click(await screen.findByRole('button', { name: 'Worktree' }))
  await user.click(await screen.findByRole('menuitem', { name: 'fix/login' }))

  await waitFor(() => expect(router.state.location.search).toEqual({ worktree: 'fix/login' }))
  expect(await screen.findByRole('columnheader', { name: 'In fix/login' })).toBeTruthy()
})

test("main's read has one column of values, its own", async () => {
  await renderApp('/r/r1/env?worktree=main')

  expect(await screen.findByRole('columnheader', { name: 'In main' })).toBeTruthy()
  expect(screen.getAllByRole('columnheader')).toHaveLength(3)
  expect(within(screen.getByRole('row', { name: 'JWT_SECRET' })).getAllByText('••••••••')).toHaveLength(1)
})

test('a project that declares no .env file says where to declare one', async () => {
  server.use(...answeringEnv({ noFile: true }))
  await renderApp('/r/r1/env')

  expect(await screen.findByText('No .env file')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Reconcile' }).hasAttribute('disabled')).toBe(true)
})
