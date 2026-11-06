import { HttpResponse, http } from 'msw'
import type { DomainAnswered, ListDetail, RunjobsStopJob, WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import { job, runningWorktrees } from '@/test/run-up'
import { line, streamed } from '@/test/stream'

const asked = (answers: DomainAnswered[] | undefined, path: string) => answers?.find((a) => a.path === path)?.values[0]

const pathOf = (worktree: string) => `/acme.trees/${worktree.replace('/', '-')}`

// acme's jobs: feat/billing runs dev (web up, api crashed, migrate exited); main runs postgres, which it holds with
// feat/billing and fix/login
export const runningJobs: ListDetail[] = runningWorktrees.map((w) =>
  w.main ? { ...w, jobs: [job('postgres', { profile: '', url: 'http://localhost:5432' })] } : w,
)

const holders = ['main', 'feat/billing', 'fix/login']

// run.toml's jobs, each with the worktrees it is up in
const declared = [
  { value: 'web', kind: 'service', running_in: ['feat/billing'] },
  { value: 'api', kind: 'service' },
  { value: 'migrate', kind: 'task' },
  { value: 'worker', kind: 'service' },
] as const

// main lets go of postgres, which keeps running for the two others; any other job up stops
const fateOf = (worktree: string, name: string): RunjobsStopJob => {
  if (name !== 'postgres') return { name, kind: 'service', hold: 'own', fate: 'stop' }
  return {
    name,
    kind: 'service',
    hold: 'own',
    fate: 'release',
    kept_by: holders.filter((one) => one !== worktree),
    shared_in: 'main',
  }
}

const run = (command: string, worktree: string, name: string, failed: boolean, body: object) =>
  streamed(
    [
      line({ type: 'unit.started', subject: worktree }),
      line({ type: 'phase.started', subject: worktree, phase: 'job', job: name }),
      line({ type: 'phase.finished', subject: worktree, phase: 'job', job: name, failed }),
      line({ type: 'unit.finished', subject: worktree, status: failed ? 'failed' : 'done' }),
      `event: envelope\ndata: ${JSON.stringify({ ...envelope(command, []), ok: !failed, ...body })}\n\n`,
    ].join(''),
  )

const failure = (code: string, message: string) => ({ error: { kind: 'internal', code, message } })

// run stop and run start of one job; the jobs failing names do not stop, nor start
export const runJobsHandlers = ({ failing = [] as string[], calls = [] as WebCall[] } = {}) => [
  http.post('/api/repos/r1/commands/run-stop', async ({ request }) => {
    const call = (await request.json()) as WebCall
    const worktree = asked(call.answers, 'worktree') ?? ''
    const name = asked(call.answers, 'job') ?? ''
    const plan = { job: name, worktrees: [{ worktree, path: pathOf(worktree), jobs: [fateOf(worktree, name)] }] }
    if (call.dry_run) return HttpResponse.json({ ...envelope('run-stop', []), fingerprint: 's1', plan })
    calls.push(call)
    const failed = failing.includes(name)
    const status = fateOf(worktree, name).fate === 'release' ? 'released' : 'stopped'
    return run('run-stop', worktree, name, failed, {
      plan,
      items: [
        {
          subject: worktree,
          status: failed ? 'failed' : 'done',
          detail: { path: pathOf(worktree), jobs: [{ name, status: failed ? 'failed' : status }] },
          ...(failed && failure('run-stop.stop_failed', `the stop command of ${name} failed (exit 4)`)),
        },
      ],
      follow_ups: failed
        ? []
        : [
            {
              command: 'run-start',
              code: 'run-stop.start',
              message: 'start it again',
              answers: [
                { path: 'worktree', values: [worktree] },
                { path: 'job', values: [name] },
              ],
            },
          ],
    })
  }),
  http.post('/api/repos/r1/commands/run-start/questions', () =>
    HttpResponse.json({ fields: [{ kind: 'select', path: 'job', options: declared }], problems: [] }),
  ),
  http.post('/api/repos/r1/commands/run-start', async ({ request }) => {
    const call = (await request.json()) as WebCall
    const worktree = asked(call.answers, 'worktree') ?? ''
    const name = asked(call.answers, 'job') ?? ''
    const restart = asked(call.answers, 'restart') === 'true'
    const plan = {
      job: name,
      restart,
      exclusive: false,
      no_probe: false,
      others: [],
      ...(restart && name === 'postgres' && { held_by: holders }),
      worktrees: [
        {
          worktree,
          path: pathOf(worktree),
          numbered: true,
          jobs: [
            {
              name,
              kind: 'service',
              action: restart ? 'restart' : 'start',
              ports: { PORT: 3020 },
              ...(name === 'postgres' && { shared_in: 'main' }),
            },
          ],
        },
      ],
    }
    if (call.dry_run) return HttpResponse.json({ ...envelope('run-start', []), fingerprint: 'r1', plan })
    calls.push(call)
    const failed = failing.includes(name)
    return run('run-start', worktree, name, failed, {
      plan,
      items: [
        {
          subject: worktree,
          status: failed ? 'failed' : 'done',
          detail: {
            path: pathOf(worktree),
            jobs: [
              {
                name,
                kind: 'service',
                status: failed ? 'failed' : 'started',
                ports: [{ name: 'PORT', port: 3020, listening: true }],
                ...(failed && { lines: ['Error: listen EADDRINUSE :3020'] }),
              },
            ],
          },
          ...(failed && failure('run-start.job_failed', `${name} failed to start: exit status 1`)),
        },
      ],
    })
  }),
]

// the Running page over runningJobs, its jobs stopped and started as runJobsHandlers says
export const answeringRunJobs = (over: Parameters<typeof runJobsHandlers>[0] = {}) => [
  http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', runningJobs))),
  ...runJobsHandlers(over),
]
