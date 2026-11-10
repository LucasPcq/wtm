import { cleanup, screen, waitFor, within } from '@testing-library/react'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { useDaemonRuns } from '@/features/worktrees/daemon/hooks/use-daemon-runs'
import { useRuns } from '@/features/worktrees/hooks/use-run'
import {
  answeringRestart,
  answeringStopJobs,
  daemonDetail,
  daemonStatus,
  hangingRestart,
  twoRepos,
} from '@/test/daemon'
import { envelope } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { liveEvents, server } from '@/test/server'

const openPanel = async (user: Awaited<ReturnType<typeof renderApp>>['user']) => {
  await user.click(await screen.findByRole('button', { name: /^wtm's daemon/ }))
  return screen.findByRole('dialog')
}

test("the connection block opens wtm's daemon: up, the jobs of every repository, its proxy", async () => {
  server.use(daemonStatus())
  const { user } = await renderApp('/r/r1/worktrees')

  const panel = await openPanel(user)

  expect(await within(panel).findByText('Up · 0.31.0, this version')).toBeDefined()
  expect(within(panel).getByText('2 jobs and a stack')).toBeDefined()
  expect(within(panel).getByText('1 job')).toBeDefined()
  expect(within(panel).getByText('on :11080')).toBeDefined()
})

test("a daemon status that can't be read says why instead of loading forever", async () => {
  server.use(
    daemonStatus(undefined, { kind: 'internal', code: 'daemon-status.unreadable', message: 'the socket refused' }),
  )
  const { user } = await renderApp('/r/r1/worktrees')

  const panel = await openPanel(user)

  expect(await within(panel).findByText("Couldn't read wtm's daemon")).toBeDefined()
  expect(within(panel).getByText('the socket refused')).toBeDefined()
})

test('a daemon of another build turns the block amber and says a start is refused until it restarts', async () => {
  server.use(liveEvents, daemonStatus({ ...daemonDetail, daemon: '0.30.9', skew: 'build' }))
  const { user } = await renderApp('/r/r1/worktrees')

  expect(await screen.findByText("wtm's daemon runs another build")).toBeDefined()
  const panel = await openPanel(user)

  expect(await within(panel).findByText('Runs 0.30.9; this version is 0.31.0')).toBeDefined()
  expect(within(panel).getByText('Starting a job is refused until it restarts.')).toBeDefined()
})

test('Restart lists what comes back by repository, whose hold goes, what is taken over, then says it in a toast', async () => {
  server.use(daemonStatus(), answeringRestart())
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))

  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  expect(await within(dialog).findByText('postgres · shared')).toBeDefined()
  expect(within(dialog).getByText('feat/billing loses it for the cut')).toBeDefined()
  expect(within(dialog).getByText("fix/login's stack")).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  expect(await screen.findByText("Restarted wtm's daemon · 3 jobs are back; the stack was taken over.")).toBeDefined()
})

test("a restart whose new daemon didn't start says it in the panel, with its log to open", async () => {
  server.use(daemonStatus(), answeringRestart('start_failed'))
  const { user } = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  server.use(daemonStatus({ ...daemonDetail, running: false, repos: [] }))
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  panel = await openPanel(user)

  expect(await within(panel).findByText("The new daemon didn't start")).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Open its log' })).toBeDefined()
  expect(within(panel).getByText('not served')).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Restart' }), 'it can be tried again').toBeDefined()
})

test("a restart whose job didn't come back warns it restarted, says why, and offers Restart again", async () => {
  server.use(daemonStatus(), answeringRestart('not_back'))
  const { user } = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  expect(await screen.findByText("Restarted wtm's daemon · 1 job didn't come back")).toBeDefined()
  panel = await openPanel(user)
  expect(await within(panel).findByText("Restarted · 1 job didn't come back")).toBeDefined()
  expect(within(panel).getByText("didn't come back: web: port 3000 is taken")).toBeDefined()
  expect(within(panel).queryByText("The new daemon didn't start")).toBeNull()
  expect(within(panel).getByText('on :11080')).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Restart' })).toBeDefined()
})

test('Stop every job lists every repository, then stops them and says the proxy stays', async () => {
  server.use(daemonStatus(), answeringStopJobs())
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))

  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  expect(await within(dialog).findByText('its stack: mailpit, through its stop command')).toBeDefined()
  expect(within(dialog).getByText('feat/billing loses it')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Stop 3 jobs' }))

  expect(await within(dialog).findByRole('button', { name: 'Done' })).toBeDefined()
  expect(within(dialog).getByText('3 stopped. The proxy is still up.')).toBeDefined()
})

test("a stack that didn't stop is named, and Run again plans what still runs", async () => {
  server.use(daemonStatus(), answeringStopJobs(true))
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  await user.click(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' }))

  expect(await within(dialog).findByText(/couldn't stop mailpit/)).toBeDefined()
  expect(within(dialog).getByText('2 of 3 stopped. Run again retries mailpit in fix/login (acme).')).toBeDefined()
  await user.click(within(dialog).getByRole('button', { name: 'Run again' }))

  expect(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' })).toBeDefined()
})

test("a restart under way shows on another repository's page, which offers no second one meanwhile", async () => {
  server.use(daemonStatus(), hangingRestart, twoRepos)
  const first = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(first.user)
  await first.user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  await first.user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  cleanup()
  const second = await renderApp('/r/r2/worktrees')
  panel = await openPanel(second.user)

  expect(await within(panel).findByText('Restarting')).toBeDefined()
  await waitFor(() => {
    const phases = useRuns.getState()['daemon-restart:machine']?.progress.units[0]?.phases ?? []
    expect(phases.map((run) => `${run.job}:${run.state}`).sort()).toEqual(['postgres:done', 'web:done'])
  })
  expect(
    within(panel).getByText("3 jobs come back once it's up."),
    'the count known when Restart was clicked',
  ).toBeDefined()
  expect(within(panel).getByRole('button', { name: 'Restart' })).toHaveProperty('disabled', true)
})

test("a daemon up again after a restart that didn't start it is no longer said down", async () => {
  server.use(daemonStatus(), answeringRestart('start_failed'))
  const { user } = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  panel = await openPanel(user)

  expect(await within(panel).findByText('Up · 0.31.0, this version')).toBeDefined()
  expect(within(panel).queryByText("The new daemon didn't start")).toBeNull()
  expect(within(panel).queryByText("The restart didn't finish")).toBeNull()
})

test('a Stop every job hidden while it runs says how it ended, and Details opens it again', async () => {
  let release = () => {}
  const held = new Promise<void>((resolve) => {
    release = resolve
  })
  server.use(daemonStatus(), answeringStopJobs(true, held))
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  await user.click(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' }))
  await user.click(await within(dialog).findByRole('button', { name: 'Hide' }))

  release()

  expect(await screen.findByText("Jobs didn't stop in 1 worktree.")).toBeDefined()
  await user.click(screen.getByRole('button', { name: 'Details' }))
  expect(
    await within(await screen.findByRole('dialog', { name: 'Stop every job' })).findByText("Couldn't stop"),
  ).toBeDefined()
})

test('a Stop every job closed with Escape while it runs still says how it ended', async () => {
  let release = () => {}
  const held = new Promise<void>((resolve) => {
    release = resolve
  })
  server.use(daemonStatus(), answeringStopJobs(true, held))
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  await user.click(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' }))
  await within(dialog).findByRole('button', { name: 'Hide' })
  await user.keyboard('{Escape}')

  release()

  expect(await screen.findByText("Jobs didn't stop in 1 worktree.")).toBeDefined()
})

test('after a restart that did not start its daemon, a read from before the run never says it up', async () => {
  let release = () => {}
  const fresh = new Promise<void>((resolve) => {
    release = resolve
  })
  let reads = 0
  server.use(
    http.post('/api/repos/:repo/commands/daemon-status', async () => {
      reads += 1
      if (reads > 1) await fresh
      return HttpResponse.json({
        ...envelope('daemon-status', []),
        items: [{ subject: 'daemon', status: 'done', detail: daemonDetail }],
      })
    }),
    answeringRestart('start_failed'),
  )
  const { user } = await renderApp('/r/r1/worktrees')
  let panel = await openPanel(user)
  await within(panel).findByText('Up · 0.31.0, this version')
  await user.click(within(panel).getByRole('button', { name: 'Restart' }))
  const dialog = await screen.findByRole('dialog', { name: "Restart wtm's daemon" })
  await within(dialog).findByText('postgres · shared')
  await user.click(within(dialog).getByRole('button', { name: 'Restart' }))

  panel = await openPanel(user)
  expect(await within(panel).findByText("The new daemon didn't start")).toBeDefined()

  release()
  expect(await within(panel).findByText('Up · 0.31.0, this version'), 'the read after the run').toBeDefined()
})

test('a dialog left open when the panel goes never opens again on its own', async () => {
  server.use(daemonStatus(), answeringRestart())
  const first = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(first.user)
  await first.user.click(within(panel).getByRole('button', { name: 'Restart' }))
  await screen.findByRole('dialog', { name: "Restart wtm's daemon" })

  cleanup()
  await renderApp('/r/r1/worktrees')

  await screen.findByRole('button', { name: /^wtm's daemon/ })
  expect(screen.queryByRole('dialog', { name: "Restart wtm's daemon" })).toBeNull()
})

test("a toast's Details clicked where no panel was opens no dialog later", async () => {
  server.use(daemonStatus(), answeringStopJobs())
  useDaemonRuns.setState({ asking: 'stop' })

  await renderApp('/r/r1/worktrees')

  await screen.findByRole('button', { name: /^wtm's daemon/ })
  expect(screen.queryByRole('dialog', { name: 'Stop every job' })).toBeNull()
})

test('a stop the core offers no again for shows no Run again', async () => {
  server.use(daemonStatus(), answeringStopJobs(true, undefined, false))
  const { user } = await renderApp('/r/r1/worktrees')
  const panel = await openPanel(user)
  await user.click(within(panel).getByRole('button', { name: 'Stop every job' }))
  const dialog = await screen.findByRole('dialog', { name: 'Stop every job' })
  await user.click(await within(dialog).findByRole('button', { name: 'Stop 3 jobs' }))

  expect(await within(dialog).findByText('2 of 3 stopped.')).toBeDefined()
  expect(within(dialog).queryByRole('button', { name: 'Run again' })).toBeNull()
})
