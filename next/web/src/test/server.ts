import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import type {
  DomainAnswered,
  DomainEnvelope,
  DomainField,
  DomainFollowUp,
  ListDetail,
  StatusDetail,
  TreeDetail,
  WebCall,
} from '@/api/types.gen'

// The server the component tests talk to: the API of `wtm-next ui` over one repository, acme, with a stack (feat/billing,
// feat/billing-ui behind it) and a worktree with a change (fix/login). A test changes what it answers with server.use.
export const repo = { id: 'r1', root: '/acme', common_dir: '/acme/.git' }

const list = (branch: string, over: Partial<ListDetail> = {}): ListDetail => ({
  actions: [],
  active: false,
  ahead: 0,
  behind: 0,
  branch,
  changes: { changed: 0, untracked: 0 },
  commits_ahead: 0,
  created_at: '2026-10-25T11:00:00Z',
  dirty: false,
  last_commit_at: '2026-10-25T11:50:00Z',
  locked: false,
  main: false,
  merged: false,
  missing: false,
  needs_sync: false,
  origin: null,
  parent: 'main',
  path: `/acme.trees/${branch.replace('/', '-')}`,
  pr: null,
  rebasing: false,
  services: [],
  urls: [],
  ...over,
})

export const worktrees: ListDetail[] = [
  list('main', { main: true, parent: '', ahead: null, behind: null, path: '/acme' }),
  list('feat/billing', { ahead: 1 }),
  list('feat/billing-ui', { parent: 'feat/billing', behind: 1, needs_sync: true }),
  list('fix/login', { dirty: true, changes: { changed: 0, untracked: 1 } }),
]

const sync = (branch: string): DomainFollowUp => ({
  command: 'sync',
  answers: [{ path: 'worktree', values: [branch] }],
  code: 'status.behind',
  message: 'sync it onto feat/billing',
})

// status is a worktree's status; feat/billing has an open pull request, which GitHub tells only when asked (with_prs)
const status = (branch: string, withPRs = false): StatusDetail => ({
  base: { name: 'main', ahead: 0, behind: 0 },
  branch,
  changes: { deletions: 0, files: [], insertions: 0, modified: 0, staged: 0, untracked: 0 },
  children: [],
  commits: [],
  created_at: '2026-10-25T11:00:00Z',
  env: { declared: 0, files: [] },
  isolation: 'isolated',
  jobs: [],
  locked: false,
  main: false,
  missing: false,
  origin: null,
  parent: 'feat/billing',
  path: `/acme.trees/${branch.replace('/', '-')}`,
  problems:
    branch === 'feat/billing-ui'
      ? [{ code: 'status.behind', message: 'feat/billing has 1 commit it lacks', fix: sync(branch) }]
      : [],
  rebasing: false,
  review:
    withPRs && branch === 'feat/billing'
      ? {
          number: 214,
          url: 'https://github.com/acme/acme/pull/214',
          state: 'open',
          title: 'Monthly invoices',
          fix: null,
        }
      : null,
  run: null,
  run_toml: 'absent',
})

const tree = (detail: ListDetail): TreeDetail => ({
  actions: detail.needs_sync
    ? [{ ...sync(detail.branch), code: 'tree.sync', message: 'Sync the worktree: its parent has commits it lacks' }]
    : [],
  ahead: detail.ahead,
  behind: detail.behind,
  branch: detail.branch,
  children: worktrees.filter((w) => w.parent === detail.branch).map((w) => w.branch),
  cycle: false,
  dirty: detail.dirty,
  locked: false,
  main: detail.main,
  missing: false,
  needs_sync: detail.needs_sync,
  origin: null,
  parent: detail.parent || null,
  path: detail.path,
  pr: null,
  rebasing: false,
  services: [],
  virtual: false,
})

const envelope = (command: string, details: object[]): DomainEnvelope => ({
  v: 2,
  ok: true,
  command,
  follow_ups: [],
  warnings: [],
  items: details.map((detail) => ({
    subject: (detail as { branch: string }).branch,
    status: 'done',
    detail: detail as never,
  })),
})

const field = (over: Partial<DomainField> & { kind: DomainField['kind']; path: string }): DomainField =>
  ({ flag: `--${over.path}`, label: over.path, help: '', ...over }) as DomainField

// questionsOf is create's form for answers: a branch is required, an existing one is not asked its environment.
export function questionsOf(answers: DomainAnswered[] = []) {
  const branches = answers.find((a) => a.path === 'branches')?.values ?? []
  return {
    fields: [
      field({
        kind: 'text_list',
        path: 'branches',
        label: 'Branches',
        help: 'One per line',
        entries: branches,
        options: [],
      }),
      field({
        kind: 'select',
        path: 'from',
        label: 'Source branch',
        value: 'main',
        origin: 'config',
        refreshable: true,
        closed: false,
        remember: false,
        rememberable: false,
        options: [
          { value: 'feat/billing', label: '' },
          { value: 'main', label: 'main (default)' },
        ],
      }),
      field({
        kind: 'select',
        path: 'env_from',
        label: 'Env strategy',
        value: 'config',
        origin: 'default',
        refreshable: false,
        closed: true,
        remember: false,
        rememberable: true,
        options: [
          { value: 'config', label: 'config default (example)' },
          { value: 'main', label: "main — the main checkout's .env" },
        ],
      }),
      field({
        kind: 'confirm',
        path: 'if_not_exists',
        label: 'If not exists',
        value: false,
        no_first: false,
        remember: false,
        rememberable: false,
        skip: 'flag_only',
        yes: { value: 'true', label: 'Yes' },
        no: { value: 'false', label: 'No' },
      }),
    ],
    problems: branches.length
      ? []
      : [{ path: 'branches', code: 'required', flag: '<branches>', message: 'branches is required' }],
  }
}

const line = (event: object) => `data: ${JSON.stringify({ v: 2, ts: '2026-10-25T12:00:00Z', ...event })}\n\n`

// createRun streams a create of branch: its phases, a line of its hook, then its envelope.
function createRun(branch: string) {
  const body = [
    line({ type: 'unit.started', subject: branch }),
    line({ type: 'phase.started', subject: branch, phase: 'worktree' }),
    line({ type: 'phase.finished', subject: branch, phase: 'worktree', failed: false }),
    line({ type: 'phase.started', subject: branch, phase: 'on_create' }),
    line({ type: 'phase.output', subject: branch, line: 'pnpm install: resolve' }),
    line({ type: 'phase.finished', subject: branch, phase: 'on_create', failed: false }),
    line({ type: 'unit.finished', subject: branch, status: 'done' }),
    `event: envelope\ndata: ${JSON.stringify({
      ...envelope('create', []),
      items: [{ subject: branch, status: 'done', detail: { path: `/acme.trees/${branch}` } }],
      follow_ups: [
        { command: 'go', answers: [{ path: 'worktree', values: [branch] }], code: 'create.go', message: 'jump in' },
      ],
    })}\n\n`,
  ].join('')
  return new HttpResponse(body, { headers: { 'Content-Type': 'text/event-stream', 'X-Wtm-Operation': 'op-1' } })
}

const spelled = (followUp: DomainFollowUp) => [
  'wtm',
  ...(followUp.command ?? '').split(' '),
  ...(followUp.answers ?? []).flatMap((a) => (a.path === 'worktree' ? a.values : [`--${a.path}`, ...a.values])),
]

export const handlers = [
  http.get('/api/version', () => HttpResponse.json({ version: 'test' })),
  http.get('/api/repos', () => HttpResponse.json([repo])),
  // the stream says nothing: a test reads the screen as the reads left it
  http.get('/api/events', () => new HttpResponse('', { headers: { 'Content-Type': 'text/event-stream' } })),
  http.post('/api/command-line', async ({ request }) =>
    HttpResponse.json({ argv: spelled((await request.json()) as DomainFollowUp) }),
  ),
  http.post('/api/repos/r1/commands/create/questions', async ({ request }) =>
    HttpResponse.json(questionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/:command', async ({ params, request }) => {
    const call = (await request.json()) as WebCall
    const asked = (path: string) => call.answers?.find((a) => a.path === path)?.values[0]
    switch (params.command) {
      case 'list':
        return HttpResponse.json(envelope('list', worktrees))
      case 'tree':
        return HttpResponse.json(envelope('tree', worktrees.map(tree)))
      case 'status':
        return HttpResponse.json(
          envelope('status', [status(asked('worktree') ?? 'main', asked('with_prs') === 'true')]),
        )
      case 'create': {
        const branch = asked('branches') ?? ''
        if (call.dry_run)
          return HttpResponse.json({
            ...envelope('create', []),
            fingerprint: 'f1',
            plan: {
              branches: [
                { branch, path: `/acme.trees/${branch}`, action: 'create', phases: ['worktree', 'on_create'] },
              ],
              creates: 1,
              from: 'main',
              env_from: 'example',
              isolation: 'isolated',
              recap: [{ key: 'branches', label: 'Branches', values: [{ value: branch, text: branch }] }],
            },
          })
        return createRun(branch)
      }
    }
    return HttpResponse.json({ error: { message: 'unknown command' } }, { status: 404 })
  }),
]

export const server = setupServer(...handlers)
