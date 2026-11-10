import { HttpResponse, http } from 'msw'
import type { DomainEnvelope, DomainForm, SettingsDocument, WebCall, WebPlace } from '@/api/types.gen'
import { repo } from '@/test/acme'

// acme, served but not set up yet: what its files suggest, as init's questions answer them
export const notSetUp = { ...repo, set_up: false }

export const setUpForm = (): DomainForm => ({
  fields: [
    { kind: 'text', path: 'base_path', origin: 'default', section: 'worktrees', value: '../.trees' },
    {
      kind: 'select',
      path: 'base_branch',
      origin: 'detected',
      section: 'worktrees',
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
      origin: 'default',
      section: 'env_files',
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
      origin: 'detected',
      section: 'env_files',
      picked: ['apps/api/.env', 'apps/web/.env'],
      options: [
        { value: 'apps/api/.env', template: 'apps/api/.env.example' },
        { value: 'apps/web/.env', template: 'apps/web/.env.example' },
      ],
    },
    {
      kind: 'hooks',
      path: 'on_create',
      origin: 'detected',
      section: 'hooks',
      entries: [{ cmd: 'pnpm install' }],
      found: [{ cmd: 'pnpm install', file: 'pnpm-lock.yaml' }],
      dirs: ['apps', 'apps/api', 'apps/web'],
    },
    {
      kind: 'hooks',
      path: 'on_clean',
      origin: 'detected',
      section: 'hooks',
      entries: [{ cmd: 'docker compose down' }],
      found: [{ cmd: 'docker compose down', file: 'docker-compose.yml' }],
      dirs: ['apps', 'apps/api', 'apps/web'],
    },
  ],
  problems: [],
  sections: [
    { id: 'worktrees', state: 'answered' },
    { id: 'env_files', state: 'answered' },
    { id: 'hooks', state: 'answered' },
  ],
})

export const setUpSettings: SettingsDocument = {
  base_path: '../.trees',
  base_branch: 'main',
  env_strategy: 'example',
  env_files: [{ target: 'apps/api/.env', template: 'apps/api/.env.example' }],
  on_create: [{ cmd: 'pnpm install' }, { cmd: 'pnpm db:generate', cwd: 'apps/api' }],
  on_clean: [{ cmd: 'docker compose down' }],
}

const planned = (settings: SettingsDocument): DomainEnvelope => ({
  v: 2,
  command: 'init',
  ok: true,
  fingerprint: 'fp-1',
  plan: { path: '/acme/.git/wtm/config.toml', created: true, changed: [], settings },
  items: [],
  follow_ups: [],
  warnings: [],
})

const written: DomainEnvelope = {
  v: 2,
  command: 'init',
  ok: true,
  items: [
    {
      subject: '/acme/.git/wtm/config.toml',
      status: 'done',
      detail: { path: '/acme/.git/wtm/config.toml', created: true },
    },
  ],
  follow_ups: [
    { command: 'create', code: 'init.create', message: 'create a worktree to get started' },
    { command: 'run init', code: 'init.run_init', message: 'set up what runs in each worktree' },
  ],
  warnings: [],
}

const notWritten: DomainEnvelope = {
  v: 2,
  command: 'init',
  ok: false,
  items: [
    {
      subject: '/acme/.git/wtm/config.toml',
      status: 'failed',
      detail: {},
      error: {
        kind: 'internal',
        code: 'init.write_failed',
        message: 'permission denied on .git/wtm/config.toml: check who owns .git/wtm, then set it up again',
      },
    },
  ],
  follow_ups: [],
  warnings: [],
}

// acme answers not set up until its settings are written, then set up; sent holds each apply's call
export function answeringSetUp({ fails = false }: { fails?: boolean } = {}) {
  const state = { setUp: false, sent: [] as WebCall[] }
  const handlers = [
    http.get('/api/repos', () => HttpResponse.json([state.setUp ? repo : notSetUp])),
    http.post('/api/repos/r1/commands/init/questions', () => HttpResponse.json(setUpForm())),
    http.post('/api/repos/r1/commands/init', async ({ request }) => {
      const call = (await request.json()) as WebCall
      if (call.dry_run) return HttpResponse.json(planned(setUpSettings))
      state.sent.push(call)
      if (fails) return HttpResponse.json(notWritten)
      state.setUp = true
      return HttpResponse.json(written)
    }),
  ]
  return { state, handlers }
}

// where each typed path stands, as the server checks it
export const places: Record<string, WebPlace> = {
  '~/dev/billing-api': {
    path: '/Users/me/dev/billing-api',
    state: 'not_set_up',
    id: 'r9',
    root: '/Users/me/dev/billing-api',
    name: 'billing-api',
  },
  '/tmp/notes': { path: '/tmp/notes', state: 'not_a_repository' },
  '/Users/me/dev/payments': {
    path: '/Users/me/dev/payments',
    state: 'set_up',
    id: 'r8',
    root: '/Users/me/dev/payments',
    name: 'payments',
  },
  '/acme': { path: '/acme', state: 'known', id: 'r1', root: '/acme', name: 'acme' },
}

export function answeringPlaces() {
  const added: string[] = []
  const handlers = [
    http.post('/api/repos/check', async ({ request }) => {
      const { path } = (await request.json()) as { path: string }
      return HttpResponse.json(places[path] ?? { path, state: 'missing' })
    }),
    http.post('/api/repos', async ({ request }) => {
      const { path } = (await request.json()) as { path: string }
      added.push(path)
      const place = Object.values(places).find((candidate) => candidate.path === path)
      return HttpResponse.json(place?.state === 'set_up' ? { ...place, state: 'known' } : place)
    }),
    http.get('/api/repos', () =>
      HttpResponse.json([
        repo,
        ...(added.includes('/Users/me/dev/billing-api')
          ? [
              {
                id: 'r9',
                root: '/Users/me/dev/billing-api',
                common_dir: '/Users/me/dev/billing-api/.git',
                set_up: false,
              },
            ]
          : []),
      ]),
    ),
  ]
  return { added, handlers }
}
