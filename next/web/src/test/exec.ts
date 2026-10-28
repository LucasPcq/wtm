import { HttpResponse, http } from 'msw'
import type { DomainAnswered, DomainField, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

const asked = (answers: DomainAnswered[] | undefined, path: string) =>
  answers?.find((a) => a.path === path)?.values ?? []

// the shell refuses a line that ends on an operator, as sh -n does
const refusedByTheShell = (command: string) => /(&&|\|\||\|)\s*$/.test(command)

export function execQuestionsOf(answers: DomainAnswered[] = []) {
  const picked = asked(answers, 'worktree')
  const [command = ''] = asked(answers, 'command')
  const problems = [
    ...(picked.length
      ? []
      : [{ path: 'worktree', code: 'required', flag: '<worktree>', message: 'worktree is required' }]),
    ...(command.trim() === ''
      ? [{ path: 'command', code: 'required', flag: '-- <command>', message: 'command is required' }]
      : []),
    ...(refusedByTheShell(command)
      ? [
          {
            path: 'command',
            code: 'exec.command_invalid',
            flag: '-- <command>',
            params: { reason: 'sh: syntax error: unexpected end of file' },
            message: '-- <command>: sh: syntax error: unexpected end of file',
          },
        ]
      : []),
  ]
  return {
    fields: [
      { kind: 'multi_select', path: 'worktree', picked, options: worktrees.map((w) => ({ value: w.branch })) },
      { kind: 'text', path: 'command', value: command },
    ] as DomainField[],
    problems,
  }
}

// the plan's fingerprint covers its command: a run of another line than the plan's is refused as stale
export const fingerprintOf = (command: string) => `e:${command}`

const warningsOf = (picked: string[]) =>
  picked.includes('feat/search-docs')
    ? [
        {
          code: 'exec.env_unnumbered',
          message: 'feat/search-docs has no ordinal: the command runs without its ports and Compose project',
          params: { worktree: 'feat/search-docs' },
        },
      ]
    : []

// feat/search-docs is isolated but has no number yet: it runs without its run env
export const execPlan = (call: WebCall) => {
  const picked = asked(call.answers, 'worktree')
  const [command = ''] = asked(call.answers, 'command')
  return HttpResponse.json({
    ...envelope('exec', []),
    fingerprint: fingerprintOf(command),
    plan: {
      command,
      jobs: Math.min(4, picked.length),
      print: false,
      targets: picked.map((worktree) => ({
        worktree,
        branch: worktree,
        path: `/acme.trees/${worktree.replace('/', '-')}`,
      })),
    },
    warnings: warningsOf(picked),
  })
}

const detail = (worktree: string, exit: number | null, ms: number, tail: string[]) => ({
  path: `/acme.trees/${worktree}`,
  exit_code: exit,
  duration_ms: ms,
  log: `/state/exec/${worktree.replace('/', '-')}/op-1.log`,
  tail,
})

const retry = (worktrees: string[], command: string) => ({
  command: 'exec',
  answers: [
    { path: 'worktree', values: worktrees },
    { path: 'command', values: [command] },
  ],
  code: 'exec.retry',
  message: 'run it again where it did not pass',
})

// each worktree passes, but those failing names, which exit 1 on a failed test
export const execRun =
  (failing: string[] = []) =>
  (call: WebCall) => {
    const [command = ''] = asked(call.answers, 'command')
    if (call.expect_fingerprint !== fingerprintOf(command))
      return streamed(
        `event: envelope\ndata: ${JSON.stringify({ ...envelope('exec', []), ok: false, error: { kind: 'precondition', code: 'plan.stale', message: 'the facts changed since the preview' } })}\n\n`,
      )
    const picked = asked(call.answers, 'worktree')
    const failed = picked.filter((worktree) => failing.includes(worktree))
    const error = (worktree: string) => ({
      kind: 'internal',
      code: 'exec.failed',
      message: `${worktree}: exited 1`,
      params: { exit_code: '1' },
      follow_up: retry(failed, command),
    })
    return streamed(
      [
        ...picked.flatMap((worktree) => [
          line({ type: 'unit.started', subject: worktree }),
          line({ type: 'phase.started', subject: worktree, phase: 'exec', log: `/state/exec/${worktree}.log` }),
          line({
            type: 'phase.output',
            subject: worktree,
            line: `\u001b[32m✓\u001b[39m src/${worktree}.test.ts (12 tests)`,
          }),
          line({ type: 'phase.finished', subject: worktree, phase: 'exec', failed: failed.includes(worktree) }),
          line({ type: 'unit.finished', subject: worktree, status: failed.includes(worktree) ? 'failed' : 'done' }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('exec', []),
          ok: failed.length === 0,
          warnings: warningsOf(picked),
          plan: { command, jobs: Math.min(4, picked.length), print: false, targets: [] },
          items: picked.map((worktree) =>
            failed.includes(worktree)
              ? {
                  subject: worktree,
                  status: 'failed',
                  detail: detail(worktree, 1, 8400, [
                    "expected 'Monthly' to be 'Yearly'",
                    'Tests  1 failed | 41 passed',
                  ]),
                  error: error(worktree),
                }
              : { subject: worktree, status: 'done', detail: detail(worktree, 0, 3100, ['Tests  42 passed']) },
          ),
          ...(failed.length > 0 && { error: error(failed[0] ?? ''), follow_ups: [retry(failed, command)] }),
        })}\n\n`,
      ].join(''),
    )
  }

// exec answered by the test: its questions as acme's, its dry runs by dryRun, its runs by run
export const answeringExec = (
  dryRun: (call: WebCall) => Response | Promise<Response> = execPlan,
  run: (call: WebCall) => Response = execRun(),
) => [
  http.post('/api/repos/r1/commands/exec/questions', async ({ request }) =>
    HttpResponse.json(execQuestionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/exec', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? dryRun(call) : run(call)
  }),
]

// as the server answers it: the stream opens on its operation, whose locks are then refused, and the envelope says so
export const lockHeldBy = (op: string) => () =>
  streamed(
    `event: envelope\ndata: ${JSON.stringify({
      ...envelope('exec', []),
      ok: false,
      error: {
        kind: 'conflict',
        code: 'lock.held',
        message: 'feat/billing is locked by clean, started from cli (pid 4242)',
        params: {
          key: 'wt:feat/billing',
          scope: 'wt',
          name: 'feat/billing',
          op,
          command: 'clean',
          pid: '4242',
          surface: 'cli',
        },
      },
    })}\n\n`,
  )

// a run cancelled while feat/billing ran: feat/search was never reached
export const cancelledRun = (call: WebCall) => {
  const [command = ''] = asked(call.answers, 'command')
  return streamed(
    `event: envelope\ndata: ${JSON.stringify({
      ...envelope('exec', []),
      ok: false,
      plan: { command, jobs: 1, print: false, targets: [] },
      error: { kind: 'cancelled', code: 'cancelled', message: 'interrupted' },
      items: [
        {
          subject: 'feat/billing',
          status: 'cancelled',
          reason: 'interrupted',
          detail: detail('feat/billing', null, 900, []),
        },
        { subject: 'feat/search', status: 'skipped', reason: 'not_reached', detail: {} },
      ],
      follow_ups: [retry(['feat/billing', 'feat/search'], command)],
    })}\n\n`,
  )
}

// a run that passes where its .env could not be read: it ran without its run env, which the item says
export const unreadableEnvRun = (call: WebCall) => {
  const picked = asked(call.answers, 'worktree')
  const [command = ''] = asked(call.answers, 'command')
  const cause = 'open /acme.trees/feat-billing/.env: permission denied'
  return streamed(
    `event: envelope\ndata: ${JSON.stringify({
      ...envelope('exec', []),
      plan: { command, jobs: 1, print: false, targets: [] },
      items: picked.map((worktree) => ({
        subject: worktree,
        status: 'done',
        detail: detail(worktree, 0, 1200, []),
        warnings: [
          {
            code: 'exec.env_unavailable',
            message: `${cause} — the command runs without the worktree's environment`,
            params: { cause },
          },
        ],
      })),
    })}\n\n`,
  )
}

// a run one at a time whose first worktree starts and prints, then waits for the test to end it, or to drop it
export function heldExecRun() {
  const encoder = new TextEncoder()
  const esc = String.fromCharCode(27)
  let controller: ReadableStreamDefaultController<Uint8Array> | undefined
  const started = [
    { type: 'unit.started', subject: 'feat/billing' },
    { type: 'phase.started', subject: 'feat/billing', phase: 'exec', log: '/state/exec/a.log' },
    { type: 'phase.output', subject: 'feat/billing', line: `${esc}[32m✓${esc}[39m src/invoice.test.ts` },
  ]
  const respond = () =>
    new HttpResponse(
      new ReadableStream<Uint8Array>({
        start: (opened) => {
          controller = opened
          opened.enqueue(
            encoder.encode(
              started
                .map((event) => `data: ${JSON.stringify({ v: 2, ts: new Date().toISOString(), ...event })}\n\n`)
                .join(''),
            ),
          )
        },
      }),
      { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } },
    )
  const oneAtATime = async (call: WebCall) => {
    const planned = await execPlan(call).json()
    return HttpResponse.json({ ...planned, plan: { ...planned.plan, jobs: 1 } })
  }
  return {
    handlers: answeringExec(oneAtATime, respond),
    end: (ended: object) => {
      controller?.enqueue(encoder.encode(`event: envelope\ndata: ${JSON.stringify(ended)}\n\n`))
      controller?.close()
    },
    drop: () => controller?.close(),
  }
}

export const passed = (worktrees: string[], failing: string[] = []) => ({
  ...envelope('exec', []),
  ok: failing.length === 0,
  items: worktrees.map((worktree) =>
    failing.includes(worktree)
      ? { subject: worktree, status: 'failed', detail: detail(worktree, 1, 900, []) }
      : { subject: worktree, status: 'done', detail: detail(worktree, 0, 900, []) },
  ),
})
