import { screen, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

async function openCreate() {
  const app = await renderApp('/r/r1/worktrees')
  await app.user.click(await screen.findByRole('button', { name: 'Create worktree' }))
  const dialog = await screen.findByRole('dialog')
  return { ...app, dialog, branches: await within(dialog).findByRole('textbox', { name: 'Branches' }) }
}

// A plan refused for one of its fields said only "the request is invalid", never which field nor why.
test("a plan the core refuses for a field says that field's problem", async () => {
  server.use(
    http.post('/api/repos/r1/commands/create', () =>
      HttpResponse.json(
        {
          ...envelope('create', []),
          ok: false,
          error: {
            kind: 'invalid',
            code: 'request.invalid',
            message: 'the request is invalid',
            fields: [
              {
                path: 'branches[0]',
                code: 'create.path_exists',
                flag: '<branches>',
                message: 'worktree path already exists: /acme.trees/feat/new',
              },
            ],
          },
        },
        { status: 400 },
      ),
    ),
  )
  const { user, dialog, branches } = await openCreate()
  await user.type(branches, 'feat/new')
  await user.click(within(dialog).getByRole('button', { name: 'Next' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Next' }))

  expect(await within(dialog).findByText('Worktree path already exists: /acme.trees/feat/new')).toBeTruthy()
})
