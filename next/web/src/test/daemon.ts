import { HttpResponse, http } from 'msw'
import type { DaemonrestartPlan, DaemonstatusDetail, DaemonstopjobsPlan, WebCall } from '@/api/types.gen'
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

export const daemonStatus = (detail: DaemonstatusDetail = daemonDetail) =>
  http.post('/api/repos/r1/commands/daemon-status', () =>
    HttpResponse.json({ ...envelope('daemon-status', []), items: [{ subject: 'daemon', status: 'done', detail }] }),
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
        { branch: 'feat/billing', path: '/a/feat-billing', jobs: [{ name: 'postgres', hold: 'claim' }] },
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
export const answeringRestart = (failing = false) =>
  http.post('/api/repos/r1/commands/daemon-restart', async ({ request }) => {
    const call = (await request.json()) as WebCall
    if (call.dry_run)
      return HttpResponse.json({ ...envelope('daemon-restart', []), fingerprint: 'r:1', plan: restartPlan })
    const item = failing
      ? {
          subject: 'daemon',
          status: 'failed',
          error: {
            kind: 'internal',
            code: 'daemon-restart.start_failed',
            message: "the new daemon didn't start: wtm's daemon did not start within 5s",
          },
        }
      : {
          subject: 'daemon',
          status: 'done',
          detail: {
            back: [
              { name: 'web', path: '/Users/me/dev/acme', back: true },
              { name: 'postgres', path: '/a/feat-billing', back: true },
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

// stop-jobs' plan, then its apply: every job stops, but fix/login's stack when failing
export const answeringStopJobs = (failing = false) =>
  http.post('/api/repos/r1/commands/daemon-stop-jobs', async ({ request }) => {
    const call = (await request.json()) as WebCall
    if (call.dry_run)
      return HttpResponse.json({ ...envelope('daemon-stop-jobs', []), fingerprint: 's:1', plan: stopPlan })
    const targets = (stopPlan.repos ?? []).flatMap((repo) => repo.worktrees ?? [])
    return streamed(
      [
        ...targets.flatMap((target) => [
          line({ type: 'unit.started', subject: target.worktree }),
          ...(target.jobs ?? []).flatMap((job) => [
            line({ type: 'phase.started', subject: target.worktree, phase: 'job', job: job.name }),
            line({ type: 'phase.finished', subject: target.worktree, phase: 'job', job: job.name }),
          ]),
          line({ type: 'unit.finished', subject: target.worktree, status: 'done' }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('daemon-stop-jobs', []),
          plan: stopPlan,
          ok: !failing,
          items: targets.map((target) => {
            const failed = failing && target.worktree === 'fix/login (acme)'
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
