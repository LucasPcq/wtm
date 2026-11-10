import { HttpResponse, http } from 'msw'
import type { DaemonrestartPlan, DaemonstatusDetail, DaemonstopjobsPlan, DomainError, WebCall } from '@/api/types.gen'
import { repo } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// the machine's daemon: acme's main runs web and postgres, which feat/billing holds, and fix/login runs a stack; wtm's
// main runs docs-site
export const daemonDetail: DaemonstatusDetail = {
  running: true,
  pid: 4242,
  build: '0.31.0',
  daemon: '0.31.0',
  protocol: 2,
  repos: [
    {
      root: '/Users/me/dev/acme',
      worktrees: [
        {
          branch: 'main',
          path: '/Users/me/dev/acme',
          jobs: [
            { name: 'postgres', kind: 'service', hold: 'own', shared: true, held_by: ['feat/billing'] },
            { name: 'web', kind: 'service', hold: 'own' },
          ],
        },
        {
          branch: 'feat/billing',
          path: '/a/feat-billing',
          jobs: [{ name: 'postgres', kind: 'service', hold: 'claim' }],
        },
        { branch: 'fix/login', path: '/a/fix-login', jobs: [{ name: 'mailpit', kind: 'service', hold: 'detached' }] },
      ],
    },
    {
      root: '/Users/me/dev/wtm',
      worktrees: [{ branch: 'main', path: '/w', jobs: [{ name: 'docs-site', kind: 'service', hold: 'own' }] }],
    },
  ],
  proxy: { port: 11080, public_port: 11080 },
  socket: '/cfg/wtm/wtm.sock',
  index: '/cfg/wtm/jobs.json',
}

// the daemon's read, or, given an error, a read refused with no item
export const daemonStatus = (detail: DaemonstatusDetail = daemonDetail, error?: DomainError) =>
  http.post('/api/repos/:repo/commands/daemon-status', () =>
    HttpResponse.json(
      error
        ? { ...envelope('daemon-status', []), ok: false, error }
        : { ...envelope('daemon-status', []), items: [{ subject: 'daemon', status: 'done', detail }] },
    ),
  )

const restartPlan: DaemonrestartPlan = {
  running: true,
  from: '0.31.0',
  to: '0.31.0',
  confirm: true,
  back: [
    {
      root: '/Users/me/dev/acme',
      worktrees: [
        {
          branch: 'main',
          path: '/Users/me/dev/acme',
          jobs: [
            { name: 'postgres', hold: 'own', shared: true, held_by: ['feat/billing'] },
            { name: 'web', hold: 'own' },
          ],
        },
        {
          branch: 'feat/billing',
          path: '/a/feat-billing',
          jobs: [
            { name: 'postgres', hold: 'claim' },
            { name: 'web', hold: 'own' },
          ],
        },
      ],
    },
  ],
  taken_over: [
    {
      root: '/Users/me/dev/acme',
      worktrees: [{ branch: 'fix/login', path: '/a/fix-login', jobs: [{ name: 'mailpit', hold: 'detached' }] }],
    },
  ],
  tasks: [],
  unrecorded: [],
  gone: [],
  unheld: [],
}

// a restart's plan, then its apply: every job back, or the new daemon failing to start
// a restart planned, whose apply never ends
export const hangingRestart = http.post('/api/repos/:repo/commands/daemon-restart', async ({ request }) => {
  const call = (await request.json()) as WebCall
  if (call.dry_run)
    return HttpResponse.json({ ...envelope('daemon-restart', []), fingerprint: 'r:1', plan: restartPlan })
  // the core names each phase by its job alone: web runs in two worktrees
  const phases = ['postgres', 'web', 'postgres', 'web'].flatMap((job) => [
    line({ type: 'phase.started', subject: 'daemon', phase: 'job', job }),
    line({ type: 'phase.finished', subject: 'daemon', phase: 'job', job }),
  ])
  return new HttpResponse(
    new ReadableStream({ start: (controller) => controller.enqueue(new TextEncoder().encode(phases.join(''))) }),
    { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } },
  )
})

// acme and a second repository, wtm, each with its own page
export const twoRepos = http.get('/api/repos', () =>
  HttpResponse.json([
    repo,
    { id: 'r2', root: '/Users/me/dev/wtm', common_dir: '/Users/me/dev/wtm/.git', set_up: true },
  ]),
)

export const answeringRestart = (failing?: 'start_failed' | 'not_back') =>
  http.post('/api/repos/:repo/commands/daemon-restart', async ({ request }) => {
    const call = (await request.json()) as WebCall
    if (call.dry_run)
      return HttpResponse.json({ ...envelope('daemon-restart', []), fingerprint: 'r:1', plan: restartPlan })
    const message = {
      start_failed: "the new daemon didn't start: wtm's daemon did not start within 5s",
      not_back: "didn't come back: web: port 3000 is taken",
    }
    const item = failing
      ? {
          subject: 'daemon',
          status: 'failed',
          error: { kind: 'internal', code: `daemon-restart.${failing}`, message: message[failing] },
          detail: { back: [{ name: 'web', path: '/Users/me/dev/acme', back: false }] },
        }
      : {
          subject: 'daemon',
          status: 'done',
          detail: {
            // the core starts main's shared postgres for its claimants: only the claim has an entry
            back: [
              { name: 'web', path: '/Users/me/dev/acme', back: true },
              { name: 'postgres', path: '/a/feat-billing', back: true },
              { name: 'web', path: '/a/feat-billing', back: true },
            ],
          },
        }
    return streamed(
      [
        line({ type: 'unit.started', subject: 'daemon' }),
        line({ type: 'unit.finished', subject: 'daemon', status: item.status }),
        `event: envelope\ndata: ${JSON.stringify({ ...envelope('daemon-restart', []), ok: !failing, plan: restartPlan, items: [item] })}\n\n`,
      ].join(''),
    )
  })

const stopPlan: DaemonstopjobsPlan = {
  repos: [
    {
      root: '/Users/me/dev/acme',
      worktrees: [
        {
          worktree: 'main (acme)',
          branch: 'main',
          path: '/Users/me/dev/acme',
          jobs: [
            {
              name: 'postgres',
              kind: 'service',
              hold: 'own',
              fate: 'stop',
              shared_in: '/Users/me/dev/acme',
              held_by: ['feat/billing'],
            },
            { name: 'web', kind: 'service', hold: 'own', fate: 'stop' },
          ],
        },
        {
          worktree: 'feat/billing (acme)',
          branch: 'feat/billing',
          path: '/a/feat-billing',
          jobs: [{ name: 'postgres', kind: 'service', hold: 'claim', fate: 'stop', shared_in: '/Users/me/dev/acme' }],
        },
        {
          worktree: 'fix/login (acme)',
          branch: 'fix/login',
          path: '/a/fix-login',
          jobs: [
            { name: 'mailpit', kind: 'service', hold: 'detached', fate: 'stop', stop_command: 'docker compose down' },
          ],
        },
      ],
    },
  ],
}

// stop-jobs' plan, then its apply: every job stops, but fix/login's stack when failing, offered again unless offered is
// false; given held, the apply answers once it settles
export const answeringStopJobs = (failing = false, held?: Promise<void>, offered = true) =>
  http.post('/api/repos/:repo/commands/daemon-stop-jobs', async ({ request }) => {
    const call = (await request.json()) as WebCall
    if (call.dry_run)
      return HttpResponse.json({ ...envelope('daemon-stop-jobs', []), fingerprint: 's:1', plan: stopPlan })
    await held
    const targets = (stopPlan.repos ?? []).flatMap((repo) => repo.worktrees ?? [])
    const fails = (worktree: string) => failing && worktree === 'fix/login (acme)'
    return streamed(
      [
        ...targets.flatMap((target) => [
          line({ type: 'unit.started', subject: target.worktree }),
          ...(target.jobs ?? []).flatMap((job) => [
            line({ type: 'phase.started', subject: target.worktree, phase: 'job', job: job.name }),
            line({
              type: 'phase.finished',
              subject: target.worktree,
              phase: 'job',
              job: job.name,
              failed: fails(target.worktree),
            }),
          ]),
          line({ type: 'unit.finished', subject: target.worktree, status: fails(target.worktree) ? 'failed' : 'done' }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('daemon-stop-jobs', []),
          plan: stopPlan,
          ok: !failing,
          follow_ups:
            failing && offered
              ? [{ command: 'daemon-stop-jobs', code: 'daemon-stop-jobs.again', message: 'stop the ones left' }]
              : [],
          items: targets.map((target) => {
            const failed = fails(target.worktree)
            return {
              subject: target.worktree,
              status: failed ? 'failed' : 'done',
              detail: {
                path: target.path,
                jobs: (target.jobs ?? []).map((job) => ({ name: job.name, status: failed ? 'failed' : 'stopped' })),
              },
              ...(failed && {
                error: {
                  kind: 'internal',
                  code: 'daemon-stop-jobs.stop_failed',
                  message: "couldn't stop mailpit: docker compose down exited 1",
                },
              }),
            }
          }),
        })}\n\n`,
      ].join(''),
    )
  })
