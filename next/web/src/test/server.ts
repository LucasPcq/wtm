import { HttpResponse, http } from 'msw'
import { setupServer } from 'msw/node'
import type { DomainAnswered, DomainField, DomainFollowUp, ListDetail, WebCall } from '@/api/types.gen'
import { envelope, followUp, listDetail, statusDetail, treeDetail } from '@/test/fixtures'

// The server the component tests talk to: the API of `wtm-next ui` over one repository, acme, with a stack (feat/billing,
// feat/billing-ui behind it) and a worktree with a change (fix/login). A test changes what it answers with server.use.
export const repo = { id: 'r1', root: '/acme', common_dir: '/acme/.git' }

export const worktrees: ListDetail[] = [
  listDetail('main', { main: true, parent: '', ahead: null, behind: null, path: '/acme' }),
  listDetail('feat/billing', { ahead: 1 }),
  listDetail('feat/billing-ui', { parent: 'feat/billing', behind: 1, needs_sync: true }),
  listDetail('fix/login', { dirty: true, changes: { changed: 0, untracked: 1 } }),
]

const sync = (branch: string) =>
  followUp('sync', branch, { code: 'status.behind', message: 'sync it onto feat/billing' })

// feat/billing has an open pull request, which GitHub tells only when asked (with_prs)
const status = (branch: string, withPRs: boolean) =>
  statusDetail(branch, {
    parent: 'feat/billing',
    problems:
      branch === 'feat/billing-ui'
        ? [{ code: 'status.behind', message: 'feat/billing has 1 commit it lacks', fix: sync(branch) }]
        : [],
    review:
      withPRs && branch === 'feat/billing'
        ? { number: 214, url: 'https://github.com/acme/acme/pull/214', state: 'open', title: 'Monthly invoices' }
        : null,
  })

const tree = (detail: ListDetail) =>
  treeDetail(detail.branch, {
    actions: detail.needs_sync
      ? [{ ...sync(detail.branch), code: 'tree.sync', message: 'Sync the worktree: its parent has commits it lacks' }]
      : [],
    ahead: detail.ahead,
    behind: detail.behind,
    condition: detail.condition,
    children: worktrees.filter((w) => w.parent === detail.branch).map((w) => w.branch),
    dirty: detail.dirty,
    main: detail.main,
    needs_sync: detail.needs_sync,
    parent: detail.parent || null,
    path: detail.path,
  })

const field = (over: Partial<DomainField> & { kind: DomainField['kind']; path: string }): DomainField =>
  ({ flag: `--${over.path}`, label: over.path, help: '', ...over }) as DomainField

// a branch is required; an existing one is not asked its environment
export function questionsOf(answers: DomainAnswered[] = []) {
  const branches = answers.find((a) => a.path === 'branches')?.values ?? []
  const exists = branches.some((branch) => worktrees.some((w) => w.branch === branch))
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
        ...(exists && { skip: 'exists' }),
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

export const line = (event: object) => `data: ${JSON.stringify({ v: 2, ts: '2026-10-25T12:00:00Z', ...event })}\n\n`

export const streamed = (body: string, operation: string | null = 'op-1') =>
  new HttpResponse(body, {
    headers: { 'Content-Type': 'text/event-stream', ...(operation && { 'X-Wtm-Operation': operation }) },
  })

const createRun = (branch: string) =>
  streamed(
    [
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
        follow_ups: [followUp('go', branch, { code: 'create.go', message: 'jump in' })],
      })}\n\n`,
    ].join(''),
  )

const plan = (branch: string) => ({
  ...envelope('create', []),
  fingerprint: 'f1',
  plan: {
    branches: [{ branch, path: `/acme.trees/${branch}`, action: 'create', phases: ['worktree', 'on_create'] }],
    creates: 1,
    from: 'main',
    env_from: 'example',
    isolation: 'isolated',
    recap: [],
  },
})

// a stream that stays open, for a test that needs the page live
export const liveEvents = http.get('/api/events', () => {
  let closing: ReturnType<typeof setTimeout>
  const stream = new ReadableStream({
    start: (controller) => {
      controller.enqueue(new TextEncoder().encode(': open\n\n'))
      closing = setTimeout(() => controller.close(), 3000)
    },
    cancel: () => clearTimeout(closing),
  })
  return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream' } })
})

const spelled = ({ command, answers }: DomainFollowUp) => [
  'wtm',
  ...(command ?? '').split(' '),
  ...(answers ?? []).flatMap((a) => (a.path === 'worktree' ? a.values : [`--${a.path}`, ...a.values])),
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
      case 'create':
        return call.dry_run ? HttpResponse.json(plan(asked('branches') ?? '')) : createRun(asked('branches') ?? '')
    }
    return HttpResponse.json({ error: { message: 'unknown command' } }, { status: 404 })
  }),
]

export const server = setupServer(...handlers)
