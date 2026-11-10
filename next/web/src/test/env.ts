import { HttpResponse, http } from 'msw'
import type { EnvReconcile, EnvshowDetail, WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

const asked = (call: WebCall, path: string) => call.answers?.find((answer) => answer.path === path)?.values

// acme's .env files: feat/billing's web PORT out of step and a MAILER_URL that follows nothing, its secret withheld
export const billingFiles = (shown: boolean): EnvshowDetail => ({
  worktree: 'feat/billing',
  path: '/acme.trees/feat-billing',
  main: false,
  strategy: 'example',
  isolation: 'isolated',
  pending: false,
  apps: [
    {
      dir: 'apps/api',
      jobs: ['api'],
      keys: 3,
      following: 1,
      to_check: 1,
      files: [
        {
          target: 'apps/api/.env',
          name: '.env',
          origin: 'template',
          missing: false,
          keys: [
            {
              key: 'API_PORT',
              follows: 'port',
              job: 'api',
              port: 'PORT',
              main: '4000',
              here: '4020',
              writes: '4020',
              withheld: false,
              status: 'in_step',
              out_of_step: false,
            },
            {
              key: 'MAILER_URL',
              follows: 'nothing',
              job: 'stack',
              port: 'MAILPIT_UI',
              main: shown ? 'http://localhost:8025' : '',
              here: shown ? 'http://localhost:8025' : '',
              withheld: !shown,
              status: 'in_step',
              out_of_step: false,
            },
            {
              key: 'JWT_SECRET',
              follows: 'copied',
              main: shown ? 's3cret' : '',
              here: shown ? 's3cret' : '',
              withheld: !shown,
              status: 'in_step',
              out_of_step: false,
            },
          ],
        },
      ],
    },
    {
      dir: 'apps/web',
      jobs: ['web'],
      keys: 1,
      following: 1,
      to_check: 1,
      files: [
        {
          target: 'apps/web/.env',
          name: '.env',
          origin: 'template',
          missing: false,
          keys: [
            {
              key: 'PORT',
              follows: 'port',
              job: 'web',
              port: 'PORT',
              main: '3000',
              here: '3010',
              writes: '3020',
              withheld: false,
              status: 'in_step',
              out_of_step: true,
            },
          ],
        },
      ],
    },
    {
      dir: 'apps/docs',
      jobs: [],
      keys: 0,
      following: 0,
      to_check: 0,
      files: [{ target: 'apps/docs/.env', name: '.env', origin: 'none', missing: true, keys: [] }],
    },
  ],
  checks: [
    { file: 'apps/api/.env', key: 'MAILER_URL', kind: 'unlinked', job: 'stack', port: 'MAILPIT_UI' },
    { file: 'apps/web/.env', key: 'PORT', kind: 'out_of_step', job: 'web', port: 'PORT', here: '3010', writes: '3020' },
  ],
})

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

// answers env show and env: a read of the worktree asked, the questions, a dry run, and a run that writes each worktree
export function answeringEnv({ failing = [], noFile = false }: { failing?: string[]; noFile?: boolean } = {}) {
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
      const read = { ...detail, worktree, apps: noFile ? [] : detail.apps }
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
      if (call.dry_run) return HttpResponse.json({ ...envelope('env', []), plan, fingerprint: fingerprintOf(call) })

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
