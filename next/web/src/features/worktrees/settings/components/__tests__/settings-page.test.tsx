import { screen, waitFor } from '@testing-library/react'
import { expect, test } from 'vitest'
import { renderApp } from '@/test/render'
import { refuses, setting, settingsDetail, shows } from '@/test/settings'

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

// forgetting one remembered answer sent back another wtm cannot keep, and the refusal named neither

test('forgetting an answer sends back one that cannot be kept as it is', async () => {
  shows(
    settingsDetail({
      remembered: [
        { question: 'env_strategy', value: 'parent', entry: 'env_strategy=parent' },
        { question: 'isolation', value: 'bogus', entry: 'isolation=bogus' },
      ],
    }),
  )
  const calls = setting()
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('button', { name: "Forget Where .env files come from: the parent's" }))

  await waitFor(() => expect(calls).toHaveLength(2))
  expect(calls[1]?.answers?.[0]).toEqual({ path: 'remembered', values: ['isolation=bogus'] })
  expect(await screen.findByText('Saved')).toBeTruthy()
})

// a choice refused for what it does to another setting said only "the request is invalid"

test('a choice refused says what it would break', async () => {
  shows(settingsDetail())
  setting(() => ({
    kind: 'invalid',
    code: 'invalid',
    message: 'the request is invalid',
    fields: [
      {
        path: 'env_files',
        code: 'config-set.env_files_none',
        flag: '--env-file',
        params: { value: 'main' },
        message: 'env_files lists no file while they come from main',
      },
    ],
  }))
  const { user } = await renderApp('/r/r1/settings')

  await user.click(await screen.findByRole('radio', { name: /Copy main's/ }))

  expect(
    await screen.findByText('No .env file is picked while they come from main: pick one, or copy none.'),
  ).toBeTruthy()
})
