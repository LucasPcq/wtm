import { HttpResponse, http } from 'msw'
import type { DomainAnswered, DomainEnvelope, RunlogsJob, RunlogsLine, WebCall, WebJobLine } from '@/api/types.gen'

const asked = (answers: DomainAnswered[] | undefined, path: string) => answers?.find((a) => a.path === path)?.values[0]

const at = (second: number) => new Date(Date.UTC(2026, 10, 3, 13, 0, second)).toISOString()

export const linesOf = (...texts: string[]): RunlogsLine[] => texts.map((text, second) => ({ at: at(second), text }))

// what the jobs of runningWorktrees printed: web serves invoices, api crashed on a missing module
export const printed: Record<string, RunlogsLine[]> = {
  web: linesOf('ready in 312 ms', 'GET /invoices 200', 'GET /health 200', 'GET /invoices/inv_0042 200 in 702ms'),
  api: linesOf('starting', 'node dist/index.js', "Error: Cannot find module './jobs/send-invoice'"),
}

export const facts = (name: string, over: Partial<RunlogsJob> = {}): RunlogsJob => ({
  name,
  kind: 'service',
  state: 'running',
  hold: 'own',
  dir: `apps/${name}`,
  command: 'pnpm dev',
  log: `/acme/.git/wtm/logs/feat%2Fbilling/${name}.log`,
  lines: printed[name] ?? [],
  ...over,
})

export const runLogsRead = (worktree: string, job: RunlogsJob): DomainEnvelope => ({
  v: 2,
  ok: true,
  command: 'run-logs',
  follow_ups: [],
  warnings: [],
  items: [{ subject: worktree, status: 'done', detail: { path: `/acme.trees/${worktree}`, jobs: [job] } }],
})

const event = (line: WebJobLine) => `data: ${JSON.stringify(line)}\n\n`

// a job's stream: its lines, then how it ended when it ended
export const jobStream = (worktree: string, job: string, lines: RunlogsLine[], ended?: 'crashed' | 'exited') =>
  new HttpResponse(
    [
      ...lines.map((line) => event({ type: 'line', worktree, job, text: line.text, ...(line.at && { at: line.at }) })),
      ...(ended ? [event({ type: 'ended', worktree, job, state: ended, exit_code: 1 })] : []),
    ].join(''),
    { headers: { 'Content-Type': 'text/event-stream' } },
  )

// the read of a job's facts, and of its whole log when a search asks it: every line of its log holding the text
export const answeringRunLogs = (over: Record<string, Partial<RunlogsJob>> = {}) => [
  http.post('/api/repos/r1/commands/run-logs', async ({ request }) => {
    const call = (await request.json()) as WebCall
    const name = asked(call.answers, 'job') ?? 'web'
    const job = facts(name, over[name])
    const search = asked(call.answers, 'search')
    if (!search) return HttpResponse.json(runLogsRead(asked(call.answers, 'worktree') ?? 'main', job))
    const whole = [...linesOf('GET /invoices 500 error', 'old line'), ...(job.lines ?? [])]
    const lines = whole.flatMap((line, at) => (line.text.includes(search) ? [{ ...line, n: at + 1 }] : []))
    return HttpResponse.json(
      runLogsRead(asked(call.answers, 'worktree') ?? 'main', { ...job, lines, count: whole.length }),
    )
  }),
  http.get('/api/repos/r1/jobs/:job/logs', ({ params, request }) => {
    const job = String(params.job)
    const worktree = new URL(request.url).searchParams.get('worktree') ?? 'main'
    return jobStream(worktree, job, printed[job] ?? [], job === 'api' ? 'crashed' : undefined)
  }),
]
