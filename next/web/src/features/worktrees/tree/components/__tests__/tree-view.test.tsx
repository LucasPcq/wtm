import { screen, waitFor } from '@testing-library/react'
import { expect, onTestFinished, test } from 'vitest'
import { renderApp } from '@/test/render'
import { liveEvents, server } from '@/test/server'

// The reads each POST asks, by command, with GitHub or without
function recordReads() {
  const asked: string[] = []
  server.events.on('request:start', async ({ request }) => {
    if (request.method !== 'POST' || !request.url.includes('/commands/')) return
    const command = new URL(request.url).pathname.split('/').at(-1) ?? ''
    asked.push((await request.clone().text()).includes('"with_prs"') ? `${command}+github` : command)
  })
  onTestFinished(() => server.events.removeAllListeners())
  return asked
}

test("a node's action the web does not run yet shows its command line to copy", async () => {
  const { user } = await renderApp('/r/r1/worktrees?view=tree')

  await user.click(await screen.findByRole('button', { name: 'Sync' }))

  expect(await screen.findByText('wtm sync feat/billing-ui')).toBeTruthy()
})

// The page asked GitHub for the list whatever the view: opening the Tree asked it twice, and Fetch origin twice more.
test('the Tree asks GitHub once, and Fetch origin once more', async () => {
  server.use(liveEvents)
  const asked = recordReads()
  const { user } = await renderApp('/r/r1/worktrees?view=tree')
  await screen.findByRole('button', { name: 'Sync' })
  await waitFor(() => expect(asked.filter((read) => read.endsWith('+github'))).toEqual(['tree+github']))

  const fetch = await screen.findByRole('button', { name: 'Fetch origin' })
  await waitFor(() => expect((fetch as HTMLButtonElement).disabled).toBe(false))
  await user.click(fetch)

  await waitFor(() => expect(asked.filter((read) => read.endsWith('+github'))).toEqual(['tree+github', 'tree+github']))
})

// The page read the live list under the Tree too: two reads of git for each event of the stream.
test('the Tree reads the tree only, not the list besides', async () => {
  const asked = recordReads()

  await renderApp('/r/r1/worktrees?view=tree')
  await screen.findByRole('button', { name: 'Sync' })

  expect(asked.filter((read) => read.startsWith('list'))).toEqual([])
})
