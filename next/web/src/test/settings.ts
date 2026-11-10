import { HttpResponse, http } from 'msw'
import type { ConfigshowDetail, DomainEnvelope, DomainError, WebCall } from '@/api/types.gen'
import { server } from '@/test/server'

// acme's config.toml as config show reads it: the folder and the branch at their default, .env from the template, two
// create hooks, one clean hook, the parent's .env remembered
export const settingsDetail = (over: Partial<ConfigshowDetail> = {}): ConfigshowDetail => ({
  digest: 'd1',
  newer: false,
  settings: {
    fields: [
      { kind: 'text', path: 'base_path', section: 'worktrees', origin: 'config', value: '../.trees' },
      {
        kind: 'select',
        path: 'base_branch',
        section: 'worktrees',
        origin: 'config',
        value: 'main',
        options: [{ value: 'main' }, { value: 'develop' }],
        rememberable: false,
        remember: false,
        refreshable: false,
        closed: false,
      },
      {
        kind: 'select',
        path: 'env_strategy',
        section: 'env_files',
        origin: 'config',
        value: 'example',
        options: [{ value: 'example' }, { value: 'main' }, { value: 'parent' }, { value: 'none' }],
        rememberable: false,
        remember: false,
        refreshable: false,
        closed: true,
      },
      {
        kind: 'multi_select',
        path: 'env_files',
        section: 'env_files',
        origin: 'config',
        picked: ['apps/api/.env', 'apps/web/.env'],
        options: [{ value: 'apps/api/.env' }, { value: 'apps/web/.env' }],
      },
      {
        kind: 'hooks',
        path: 'on_create',
        section: 'hooks',
        origin: 'config',
        entries: [{ cmd: 'pnpm install' }, { cmd: 'pnpm db:generate', cwd: 'packages/db' }],
        found: [],
        dirs: ['packages/db'],
      },
      {
        kind: 'hooks',
        path: 'on_clean',
        section: 'hooks',
        origin: 'config',
        entries: [{ cmd: 'docker compose down' }],
        found: [],
        dirs: [],
      },
    ],
    problems: [],
    sections: [
      { id: 'worktrees', state: 'answered' },
      { id: 'env_files', state: 'answered' },
      { id: 'hooks', state: 'answered' },
    ],
  },
  defaults: [
    { path: 'base_path', value: '../.trees', in_use: true },
    { path: 'base_branch', value: 'main', in_use: true },
    { path: 'env_strategy', value: 'example', in_use: true },
  ],
  remembered: [{ question: 'env_strategy', value: 'parent', entry: 'env_strategy=parent' }],
  ...over,
})

export const shows = (detail: ConfigshowDetail) =>
  server.use(
    http.post('/api/repos/r1/commands/config-show', () =>
      HttpResponse.json({
        v: 2,
        ok: true,
        command: 'config-show',
        warnings: [],
        follow_ups: [],
        items: [{ subject: 'config.toml', status: 'done', detail: detail as never }],
      } satisfies DomainEnvelope),
    ),
  )

export const refuses = (status: number, error: DomainError) =>
  server.use(
    http.post('/api/repos/r1/commands/config-show', () =>
      HttpResponse.json({ v: 2, ok: false, command: 'config-show', error }, { status }),
    ),
  )

// each call config set receives, its questions answered from the document, its apply refused by refuse
export const setting = (refuse?: (call: WebCall) => DomainError | undefined) => {
  const calls: WebCall[] = []
  const fields = settingsDetail().settings.fields
  server.use(
    http.post('/api/repos/r1/commands/config-set/questions', () => HttpResponse.json({ fields, problems: [] })),
    http.post('/api/repos/r1/commands/config-set', async ({ request }) => {
      const call = (await request.json()) as WebCall
      calls.push(call)
      const refusal = call.dry_run ? undefined : refuse?.(call)
      if (refusal) return HttpResponse.json({ v: 2, ok: false, command: 'config-set', error: refusal }, { status: 409 })
      const undo = {
        command: 'config-set',
        answers: [
          { path: 'env_strategy', values: ['example'] },
          { path: 'replaces', values: ['d2'] },
        ],
        code: 'config-set.undo',
        message: 'put back what it held',
      }
      return HttpResponse.json({
        v: 2,
        ok: true,
        command: 'config-set',
        fingerprint: 'fp',
        warnings: [],
        items: call.dry_run ? [] : [{ subject: 'config.toml', status: 'done', detail: {} }],
        follow_ups: call.dry_run ? [] : [undo],
      } satisfies DomainEnvelope)
    }),
  )
  return calls
}
