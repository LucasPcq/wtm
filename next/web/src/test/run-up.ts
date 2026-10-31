import { HttpResponse, http } from 'msw'
import type {
  DomainAnswered,
  DomainField,
  ListDetail,
  ListJob,
  RunupJobPlan,
  RunupPlan,
  RunupTarget,
  WebCall,
} from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

const asked = (answers: DomainAnswered[] | undefined, path: string) =>
  answers?.find((a) => a.path === path)?.values ?? []

// acme's run.toml: dev, the default, migrates first, joins the shared postgres with a database of its own, then
// starts web and api; full adds worker. feat/billing runs dev already
const profiles = [
  { value: 'dev', default: true, jobs: ['migrate', 'postgres', 'web', 'api'] },
  { value: 'full', jobs: ['migrate', 'postgres', 'web', 'api', 'worker'] },
]
const running: Record<string, { profile: string; up: string[] }> = {
  'feat/billing': { profile: 'dev', up: ['api', 'web'] },
}

export function runUpQuestionsOf(answers: DomainAnswered[] = []) {
  const picked = asked(answers, 'worktree')
  const [profile = 'dev'] = asked(answers, 'profile')
  return {
    fields: [
      {
        kind: 'multi_select',
        path: 'worktree',
        picked,
        options: worktrees.map((w) => ({ value: w.branch, ...running[w.branch] })),
      },
      {
        kind: 'select',
        path: 'profile',
        value: profile,
        options: profiles.map((option) => {
          const runningIn = picked.filter((worktree) => running[worktree]?.profile === option.value)
          return { ...option, ...(runningIn.length > 0 && { running_in: runningIn }) }
        }),
      },
    ] as DomainField[],
    problems: picked.length
      ? []
      : [{ path: 'worktree', code: 'required', flag: '<worktree>', message: 'worktree is required' }],
  }
}

const slug = (worktree: string) => worktree.replace(/\W/g, '_')

// each worktree's ports sit 10 above the one before it
const targetOf = (worktree: string, index: number, profile: string): RunupTarget => {
  const up = running[worktree]?.up ?? []
  const own = (name: string, step: number, ports?: Record<string, number>): RunupJobPlan => ({
    name,
    kind: 'service',
    action: up.includes(name) ? 'up' : 'start',
    step,
    waits_for: ['migrate'],
    ...(ports && { ports }),
  })
  const jobs: RunupJobPlan[] = [
    { name: 'migrate', kind: 'task', action: 'start', step: 1 },
    {
      name: 'postgres',
      kind: 'service',
      action: 'join',
      shared_in: 'main',
      namespace: `acme_${slug(worktree)}`,
      waits_for: ['migrate'],
    },
    own('web', 2, { PORT: 3040 + index * 10 }),
    own('api', 3, { API_PORT: 4040 + index * 10 }),
    ...(profile === 'full' ? [own('worker', 4)] : []),
  ]
  return { worktree, branch: worktree, path: `/acme.trees/${worktree.replace('/', '-')}`, numbered: true, jobs }
}

export const runUpPlanOf = (picked: string[], profile = 'dev'): RunupPlan => ({
  profile,
  exclusive: false,
  no_probe: false,
  others: null,
  worktrees: picked.map((worktree, index) => targetOf(worktree, index, profile)),
})

export const fingerprintOf = (picked: string[], profile: string) => `u:${profile}:${picked.join(',')}`

export const runUpPlan = (call: WebCall) => {
  const picked = asked(call.answers, 'worktree')
  const [profile = 'dev'] = asked(call.answers, 'profile')
  return HttpResponse.json({
    ...envelope('run-up', []),
    fingerprint: fingerprintOf(picked, profile),
    plan: runUpPlanOf(picked, profile),
  })
}

const resultOf = (job: RunupJobPlan, failing: boolean) => {
  if (failing && job.name === 'migrate')
    return { name: job.name, kind: job.kind, status: 'failed', exit_code: 1, lines: ['P1001: database unreachable'] }
  if (failing) return { name: job.name, kind: job.kind, status: 'not_started' }
  const status = { join: 'joined', up: 'already_running', start: job.kind === 'task' ? 'done' : 'started' }[job.action]
  return { name: job.name, kind: job.kind, status, ...(job.ports && { url: `http://${job.name}.acme.localhost` }) }
}

// each job starts and says a line; in the worktrees failing names, migrate fails and nothing after it starts
export const runUpRun =
  (failing: string[] = []) =>
  (call: WebCall) => {
    const picked = asked(call.answers, 'worktree')
    const [profile = 'dev'] = asked(call.answers, 'profile')
    const plan = runUpPlanOf(picked, profile)
    const targets = plan.worktrees ?? []
    const jobsRun = (target: RunupTarget) => {
      const jobs = target.jobs ?? []
      return failing.includes(target.worktree) ? jobs.slice(0, 1) : jobs
    }
    return streamed(
      [
        ...targets.flatMap((target) => [
          line({ type: 'unit.started', subject: target.worktree }),
          ...jobsRun(target).flatMap((job) => [
            line({ type: 'phase.started', subject: target.worktree, phase: 'job', job: job.name }),
            line({ type: 'phase.output', subject: target.worktree, job: job.name, line: `${job.name}: ready` }),
            line({
              type: 'phase.finished',
              subject: target.worktree,
              phase: 'job',
              job: job.name,
              failed: failing.includes(target.worktree),
            }),
          ]),
          line({
            type: 'unit.finished',
            subject: target.worktree,
            status: failing.includes(target.worktree) ? 'failed' : 'done',
          }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('run-up', []),
          ok: failing.length === 0,
          plan,
          items: targets.map((target) => {
            const failed = failing.includes(target.worktree)
            return {
              subject: target.worktree,
              status: failed ? 'failed' : 'done',
              detail: {
                path: target.path,
                profile,
                jobs: (target.jobs ?? []).map((job) => resultOf(job, failed)),
              },
              ...(failed && {
                error: { kind: 'internal', code: 'run-up.job_failed', message: 'migrate failed: exit 1' },
              }),
            }
          }),
        })}\n\n`,
      ].join(''),
    )
  }

export const answeringRunUp = (
  dryRun: (call: WebCall) => Response | Promise<Response> = runUpPlan,
  run: (call: WebCall) => Response = runUpRun(),
) => [
  http.post('/api/repos/r1/commands/run-up/questions', async ({ request }) =>
    HttpResponse.json(runUpQuestionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/run-up', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? dryRun(call) : run(call)
  }),
]

export const job = (name: string, over: Partial<ListJob> = {}): ListJob => ({
  name,
  kind: 'service',
  state: 'running',
  hold: 'own',
  profile: 'dev',
  started_at: '2026-10-25T11:50:00Z',
  ...over,
})

// acme with feat/billing running dev: web up, api crashed, migrate exited
export const runningWorktrees: ListDetail[] = worktrees.map((w) =>
  w.branch === 'feat/billing'
    ? {
        ...w,
        jobs: [
          job('api', { state: 'crashed', exit_code: 1, url: 'http://api.feat-billing.acme.localhost' }),
          job('migrate', { kind: 'task', state: 'exited' }),
          job('web', { url: 'http://web.feat-billing.acme.localhost' }),
        ],
      }
    : w,
)

// a start the test holds under way: migrate starts in feat/search, and the rest waits for finish()
export function heldRun() {
  const encoder = new TextEncoder()
  let finish = () => {}
  const run = (call: WebCall) => {
    const stream = new ReadableStream({
      start: (controller) => {
        controller.enqueue(
          encoder.encode(
            [
              line({ type: 'unit.started', subject: 'feat/search' }),
              line({ type: 'phase.started', subject: 'feat/search', phase: 'job', job: 'migrate' }),
            ].join(''),
          ),
        )
        finish = async () => {
          const rest = await runUpRun()(call).text()
          controller.enqueue(encoder.encode(rest))
          controller.close()
        }
      },
    })
    return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } })
  }
  return { run, finish: () => finish() }
}
