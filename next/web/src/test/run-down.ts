import { HttpResponse, http } from 'msw'
import type {
  DomainAnswered,
  DomainField,
  ListDetail,
  RundownPlan,
  RundownTarget,
  RunjobsStopJob,
  WebCall,
} from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { job } from '@/test/run-up'
import { line, streamed } from '@/test/stream'

const asked = (answers: DomainAnswered[] | undefined, path: string) =>
  answers?.find((a) => a.path === path)?.values ?? []

// acme's jobs: main runs the shared postgres, feat/billing and fix/login hold it; feat/billing runs dev (web, api, its
// worker crashed), fix/login runs full (web and a compose stack); the others run nothing
const own: Record<string, RunjobsStopJob[]> = {
  'feat/billing': [
    { name: 'web', kind: 'service', hold: 'own', fate: 'stop' },
    { name: 'api', kind: 'service', hold: 'own', fate: 'stop' },
    { name: 'worker', kind: 'service', hold: 'own', fate: 'clear' },
  ],
  'fix/login': [
    { name: 'web', kind: 'service', hold: 'own', fate: 'stop' },
    { name: 'stack', kind: 'service', hold: 'detached', fate: 'stop', stop_command: 'docker compose down' },
  ],
}
const profiles: Record<string, string> = { 'feat/billing': 'dev', 'fix/login': 'full' }
const holders = ['feat/billing', 'fix/login']

// the core lists a claim with no state: its service runs in the main checkout
const claim = (name: string) => {
  const { state: _, ...listed } = job(name, { hold: 'claim', profile: '' })
  return listed
}

export const runningAcme: ListDetail[] = worktrees.map((w) => {
  if (w.main) return { ...w, jobs: [job('postgres', { profile: '' })] }
  const jobs = own[w.branch]
  if (!jobs) return w
  return {
    ...w,
    profile: profiles[w.branch] ?? '',
    jobs: [
      ...jobs.map(({ name, fate }) =>
        job(name, { profile: profiles[w.branch] ?? '', ...(fate === 'clear' && { state: 'crashed', exit_code: 1 }) }),
      ),
      claim('postgres'),
    ],
  }
})

export function runDownQuestionsOf(answers: DomainAnswered[] = []) {
  return {
    fields: [
      {
        kind: 'multi_select',
        path: 'worktree',
        picked: asked(answers, 'worktree'),
        options: worktrees.map((w) => {
          const jobs = own[w.branch] ?? []
          if (w.main) return { value: w.branch, holds: ['postgres'] }
          return {
            value: w.branch,
            ...(jobs.length > 0 && {
              profile: profiles[w.branch],
              up: jobs.filter((one) => one.fate === 'stop').map((one) => one.name),
              crashed: jobs.filter((one) => one.fate === 'clear').length,
            }),
          }
        }),
      },
    ] as DomainField[],
    problems: [],
  }
}

// both holders of postgres stopping together leave it to main alone; one of them leaves it to main and the other
const targetOf = (worktree: string, picked: string[]): RundownTarget => {
  const jobs = own[worktree] ?? []
  const keptBy = ['main', ...holders.filter((one) => one !== worktree && !picked.includes(one))]
  const holds: RunjobsStopJob[] = holders.includes(worktree)
    ? [{ name: 'postgres', kind: 'service', hold: 'claim', fate: 'release', kept_by: keptBy, shared_in: 'main' }]
    : []
  return {
    worktree,
    branch: worktree,
    path: `/acme.trees/${worktree.replace('/', '-')}`,
    ...(profiles[worktree] && { profile: profiles[worktree] }),
    jobs: [...jobs, ...holds],
  }
}

export const runDownPlanOf = (picked: string[]): RundownPlan => {
  const targets = picked.map((worktree) => targetOf(worktree, picked))
  return { worktrees: targets, ...(targets.every((t) => (t.jobs ?? []).length === 0) && { empty: true }) }
}

export const runDownPlan = (call: WebCall) => {
  const picked = asked(call.answers, 'worktree')
  return HttpResponse.json({
    ...envelope('run-down', []),
    fingerprint: `d:${picked.join(',')}`,
    plan: runDownPlanOf(picked),
  })
}

const statusOf = (stop: RunjobsStopJob, failing: boolean) => {
  if (failing && stop.name === 'stack')
    return { status: 'failed', message: 'the stop command of stack failed (exit 1)' }
  if (stop.fate === 'clear') return { status: 'cleared' }
  return { status: stop.fate === 'release' ? 'released' : 'stopped' }
}

// each job stops, the stack saying what docker compose does; in the worktrees failing names, the stack does not stop
export const runDownRun =
  (failing: string[] = []) =>
  (call: WebCall) => {
    const picked = asked(call.answers, 'worktree')
    const plan = runDownPlanOf(picked)
    const targets = plan.worktrees ?? []
    const reached = targets.filter((target) => (target.jobs ?? []).length > 0)
    return streamed(
      [
        ...reached.flatMap((target) => [
          line({ type: 'unit.started', subject: target.worktree }),
          ...(target.jobs ?? []).flatMap((stop) => [
            line({ type: 'phase.started', subject: target.worktree, phase: 'job', job: stop.name }),
            ...(stop.stop_command
              ? [
                  line({
                    type: 'phase.output',
                    subject: target.worktree,
                    job: stop.name,
                    line: 'Container mailpit-1  Stopping',
                  }),
                  line({
                    type: 'phase.output',
                    subject: target.worktree,
                    job: stop.name,
                    line: 'Container mailpit-1  Stopped',
                  }),
                ]
              : []),
            line({
              type: 'phase.finished',
              subject: target.worktree,
              phase: 'job',
              job: stop.name,
              failed: failing.includes(target.worktree) && stop.name === 'stack',
            }),
          ]),
          line({
            type: 'unit.finished',
            subject: target.worktree,
            status: failing.includes(target.worktree) ? 'failed' : 'done',
          }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('run-down', []),
          ok: failing.length === 0,
          plan,
          items: targets.map((target) => {
            const failed = failing.includes(target.worktree)
            const jobs = target.jobs ?? []
            return {
              subject: target.worktree,
              status: failed ? 'failed' : 'done',
              ...(jobs.length === 0 && { reason: 'unchanged' }),
              detail: { path: target.path, jobs: jobs.map((stop) => ({ name: stop.name, ...statusOf(stop, failed) })) },
              ...(failed && {
                error: { kind: 'internal', code: 'run-down.stop_failed', message: "stack didn't stop: exit 1" },
              }),
            }
          }),
        })}\n\n`,
      ].join(''),
    )
  }

export const answeringRunDown = (run: (call: WebCall) => Response = runDownRun()) => [
  http.post('/api/repos/r1/commands/list', () => HttpResponse.json(envelope('list', runningAcme))),
  http.post('/api/repos/r1/commands/run-down/questions', async ({ request }) =>
    HttpResponse.json(runDownQuestionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/run-down', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? runDownPlan(call) : run(call)
  }),
]

// a stop the test holds under way: fix/login's stack runs its stop command, and the rest waits for finish()
export function heldRunDown() {
  const encoder = new TextEncoder()
  let finish = () => {}
  const run = (call: WebCall) => {
    const stream = new ReadableStream({
      start: (controller) => {
        controller.enqueue(
          encoder.encode(
            [
              line({ type: 'unit.started', subject: 'fix/login' }),
              line({ type: 'phase.started', subject: 'fix/login', phase: 'job', job: 'stack' }),
              line({ type: 'phase.output', subject: 'fix/login', job: 'stack', line: 'Container minio-1  Stopping' }),
            ].join(''),
          ),
        )
        finish = async () => {
          controller.enqueue(encoder.encode(await runDownRun()(call).text()))
          controller.close()
        }
      },
    })
    return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } })
  }
  return { run, finish: () => finish() }
}
