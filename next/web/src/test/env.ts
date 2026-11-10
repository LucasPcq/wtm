import { HttpResponse, http } from 'msw'
import type { EnvReconcile, WebCall } from '@/api/types.gen'
import { billingFiles } from '@/test/env-files'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

const asked = (call: WebCall, path: string) => call.answers?.find((answer) => answer.path === path)?.values

const worktrees = ['main', 'feat/billing', 'fix/login', 'feat/onboarding']

const reconcileOf = (worktree: string): EnvReconcile => {
  const base = {
    worktree,
    path: `/acme.trees/${worktree}`,
    main: worktree === 'main',
    missing: false,
    strategy: 'example' as const,
    isolation: 'isolated' as const,
    pending: false,
    numbered: true,
    restart: [],
    files: [],
    phases: [],
    ports: { offset: 20, entries: [], addressing: 'ports' as const, applied: false },
  }
  if (worktree === 'feat/billing')
    return {
      ...base,
      phases: ['env'],
      restart: ['web'],
      files: [
        {
          target: 'apps/api/.env',
          created: false,
          writes: false,
          keys: [{ key: 'OLD', status: 'orphan', settled: false }],
        },
      ],
      ports: {
        ...base.ports,
        entries: [
          {
            file: 'apps/web/.env',
            key: 'PORT',
            port: 'PORT',
            base: 3000,
            resolved: 3020,
            addressing: 'ports',
            status: 'rewrite',
          },
        ],
      },
    }
  if (worktree === 'fix/login')
    return {
      ...base,
      phases: ['env'],
      ports: {
        ...base.ports,
        offset: 10,
        entries: [
          {
            file: 'apps/api/.env',
            key: 'API_PORT',
            port: 'PORT',
            base: 4000,
            resolved: 4010,
            addressing: 'ports',
            status: 'rewrite',
          },
        ],
      },
    }
  if (worktree === 'feat/onboarding') return { ...base, isolation: 'verbatim' }
  return base
}

const planOf = (call: WebCall) => {
  const named = asked(call, 'worktree') ?? ['feat/billing', 'fix/login']
  return { overwrite: false, prune: false, show_values: false, worktrees: named.map(reconcileOf) }
}

const fingerprintOf = (call: WebCall) => `env:${JSON.stringify(call.answers ?? [])}`

// feat/onboarding has yet to choose its isolation: env leaves its ports, and says how to give it its own
const pendingWarning = {
  code: 'env.isolation_pending',
  message: 'feat/onboarding has yet to choose its isolation: its ports and the values wtm owns are left as they are',
  params: { worktree: 'feat/onboarding' },
  follow_up: {
    command: 'env',
    code: 'env.isolate',
    message: 'give it ports of its own',
    answers: [
      { path: 'worktree', values: ['feat/onboarding'] },
      { path: 'isolation', values: ['isolated'] },
    ],
  },
}

// answers env show and env: a read of the worktree asked, the questions, a dry run, and a run that writes each worktree;
// a worktree failing is offered again
export function answeringEnv({
  failing = [],
  noFile = false,
  pending = false,
}: {
  failing?: string[]
  noFile?: boolean
  pending?: boolean
} = {}) {
  return [
    http.post('/api/repos/r1/commands/env-show/questions', () =>
      HttpResponse.json({
        fields: [
          { kind: 'select', path: 'worktree', value: 'feat/billing', options: worktrees.map((value) => ({ value })) },
        ],
        problems: [],
      }),
    ),
    http.post('/api/repos/r1/commands/env-show', async ({ request }) => {
      const call = (await request.json()) as WebCall
      const detail = billingFiles(asked(call, 'show_values')?.[0] === 'true')
      const worktree = asked(call, 'worktree')?.[0] ?? 'feat/billing'
      const read = { ...detail, worktree, main: worktree === 'main', apps: noFile ? [] : detail.apps }
      return HttpResponse.json({
        ...envelope('env-show', []),
        items: [{ subject: worktree, status: 'done', detail: read }],
      })
    }),
    http.post('/api/repos/r1/commands/env/questions', () =>
      HttpResponse.json({
        fields: [
          {
            kind: 'multi_select',
            path: 'worktree',
            // unattended, the core picks the caller's worktree alone; a person is offered those out of step
            picked: [],
            prechecked: ['feat/billing', 'fix/login'],
            options: [
              { value: 'main' },
              { value: 'feat/billing', out_of_step: 2 },
              { value: 'fix/login', out_of_step: 1 },
              { value: 'feat/onboarding', isolation: 'verbatim' },
            ],
          },
        ],
        problems: [],
      }),
    ),
    http.post('/api/repos/r1/commands/env', async ({ request }) => {
      const call = (await request.json()) as WebCall
      const plan = planOf(call)
      const left = pending && plan.worktrees.some((r) => r.worktree === 'feat/onboarding') ? [pendingWarning] : []
      if (call.dry_run)
        return HttpResponse.json({ ...envelope('env', []), warnings: left, plan, fingerprint: fingerprintOf(call) })

      const going = plan.worktrees.filter((r) => (r.phases ?? []).length > 0)
      const failed = (subject: string) => failing.includes(subject)
      return streamed(
        [
          ...going.flatMap(({ worktree }) => [
            line({ type: 'unit.started', subject: worktree }),
            line({ type: 'phase.started', subject: worktree, phase: 'env' }),
            line({ type: 'phase.finished', subject: worktree, phase: 'env', failed: failed(worktree) }),
            line({ type: 'unit.finished', subject: worktree, status: failed(worktree) ? 'failed' : 'done' }),
          ]),
          `event: envelope\ndata: ${JSON.stringify({
            ...envelope('env', []),
            ok: !going.some((r) => failed(r.worktree)),
            plan,
            follow_ups: going.some((r) => failed(r.worktree))
              ? [
                  {
                    command: 'env',
                    code: 'env.retry',
                    message: 'reconcile them again',
                    answers: [{ path: 'worktree', values: going.map((r) => r.worktree).filter(failed) }],
                  },
                ]
              : [],
            items: going.map((r) =>
              failed(r.worktree)
                ? {
                    subject: r.worktree,
                    status: 'failed',
                    detail: {},
                    error: {
                      kind: 'internal',
                      code: 'env.failed',
                      message: "couldn't write fix/login's .env files",
                      params: { cause: 'open apps/api/.env: permission denied' },
                    },
                  }
                : {
                    subject: r.worktree,
                    status: 'done',
                    detail: { path: r.path, written: ['apps/web/.env'], ports: r.ports },
                    warnings: r.restart?.length
                      ? [
                          {
                            code: 'env.restart',
                            message: 'web runs on the old values until restarted',
                            params: { worktree: r.worktree, jobs: 'web' },
                            follow_up: {
                              command: 'run-start',
                              code: 'env.restart_job',
                              message: 'restart it',
                              answers: [
                                { path: 'worktree', values: [r.worktree] },
                                { path: 'job', values: ['web'] },
                                { path: 'restart', values: ['true'] },
                              ],
                            },
                          },
                        ]
                      : [],
                  },
            ),
          })}\n\n`,
        ].join(''),
      )
    }),
  ]
}
