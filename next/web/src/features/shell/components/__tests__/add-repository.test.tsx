import { screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'
import { answeringPlaces, setUpForm } from '@/test/set-up'

async function openAdd() {
  const answering = answeringPlaces()
  server.use(
    ...answering.handlers,
    http.post('/api/repos/r9/commands/init/questions', () => HttpResponse.json(setUpForm())),
  )
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: /acme/ }))
  await app.user.click(await screen.findByRole('option', { name: 'Add a repository' }))
  const dialog = await screen.findByRole('dialog', { name: 'Add a repository' })
  return { ...app, dialog, added: answering.added }
}

test('each path typed is answered under the field, and the primary says what it does with it', async () => {
  const { user, dialog } = await openAdd()
  const path = within(dialog).getByRole('textbox', { name: 'Path' })
  const cases = [
    { typed: '/tmp/notes', line: 'Not a git repository: there is no .git in /tmp/notes.', primary: 'Add', off: true },
    { typed: '/acme/README.md', line: '/acme/README.md is a file, not a folder.', primary: 'Add', off: true },
    {
      typed: '/Users/me/dev/payments',
      line: "payments is set up already: it's added as it is.",
      primary: 'Add payments',
      off: false,
    },
    { typed: '/acme', line: 'acme is already here.', primary: 'Open acme', off: false },
  ]

  for (const { typed, line, primary, off } of cases) {
    await user.clear(path)
    await user.type(path, typed)

    expect(await within(dialog).findByText(line)).toBeDefined()
    expect(within(dialog).getByRole('button', { name: primary }).hasAttribute('disabled')).toBe(off)
  }
})

test('a repository not set up is added, then opens on its set-up dialog', async () => {
  const { user, dialog, added } = await openAdd()

  await user.type(within(dialog).getByRole('textbox', { name: 'Path' }), '~/dev/billing-api')
  expect(await within(dialog).findByText('A git repository, not set up yet.')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Set up billing-api' }))

  await waitFor(() => expect(added).toEqual(['/Users/me/dev/billing-api']))
  const setUp = await screen.findByRole('dialog', { name: 'Set up billing-api' })
  expect(await within(setUp).findByDisplayValue('../.trees')).toBeDefined()
})

test('an add the server refused is forgotten once another path is typed', async () => {
  const { user, dialog } = await openAdd()
  server.use(
    http.post('/api/repos', () => HttpResponse.json({ error: { message: 'the server is stopping' } }, { status: 503 })),
  )
  const path = within(dialog).getByRole('textbox', { name: 'Path' })

  await user.type(path, '~/dev/billing-api')
  await user.click(await within(dialog).findByRole('button', { name: 'Set up billing-api' }))
  expect(await within(dialog).findByText('the server is stopping')).toBeDefined()
  await user.type(path, 'x')

  await waitFor(() => expect(within(dialog).queryByText('the server is stopping')).toBeNull())
})

test('an address that points nowhere offers to add its repository again', async () => {
  const { user } = await renderApp('/r/gone/worktrees')

  expect(await screen.findByText("This repository isn't here")).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Add a repository' }))

  expect(await screen.findByRole('dialog', { name: 'Add a repository' })).toBeDefined()
})
