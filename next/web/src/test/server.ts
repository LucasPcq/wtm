import { HttpResponse, http } from 'msw'
import { setupServer } from 'msw/node'
import type { DomainAnswered, DomainField, DomainFollowUp, ListDetail, WebCall } from '@/api/types.gen'
import { repo, worktrees } from '@/test/acme'
import { answeringCheckout } from '@/test/checkout'
import { cleanPlan, cleanQuestionsOf, cleanRun } from '@/test/clean'
import { answeringExec } from '@/test/exec'
import { ffPlan, ffQuestions, ffRun } from '@/test/fast-forward'
import { envelope, followUp, offers, statusDetail, treeDetail } from '@/test/fixtures'
import { prunePlan, pruneRun } from '@/test/prune'
import { answeringReparent } from '@/test/reparent'
import { answeringRunUp } from '@/test/run-up'
import { line, streamed } from '@/test/stream'

export { line, repo, streamed, worktrees }

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

// asked GitHub, feat/billing has its pull request, and so offers to open it
const withPR = (detail: ListDetail): ListDetail =>
  detail.branch === 'feat/billing'
    ? {
        ...detail,
        pr: { number: 214, url: 'https://github.com/acme/acme/pull/214', state: 'open' },
        actions: offers(detail.branch),
      }
    : detail

const tree = (detail: ListDetail) =>
  treeDetail(detail.branch, {
    actions: detail.actions,
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
  ({ ...over }) as DomainField

// a branch is required; an existing one is not asked its environment
export function questionsOf(answers: DomainAnswered[] = []) {
  const branches = answers.find((a) => a.path === 'branches')?.values ?? []
  const exists = branches.some((branch) => worktrees.some((w) => w.branch === branch))
  return {
    fields: [
      field({
        kind: 'text_list',
        path: 'branches',
        entries: branches,
        options: [],
      }),
      field({
        kind: 'select',
        path: 'from',
        value: 'main',
        origin: 'config',
        refreshable: true,
        closed: false,
        remember: false,
        rememberable: false,
        options: [{ value: 'feat/billing' }, { value: 'main', base: true }],
      }),
      field({
        kind: 'select',
        path: 'env_from',
        value: 'config',
        origin: 'default',
        refreshable: false,
        closed: true,
        remember: false,
        rememberable: true,
        ...(exists && { skip: 'exists' }),
        options: [{ value: 'config' }, { value: 'main' }],
      }),
      field({
        kind: 'confirm',
        path: 'if_not_exists',
        value: false,
        no_first: false,
        remember: false,
        rememberable: false,
        skip: 'flag_only',
      }),
    ],
    problems: branches.length
      ? []
      : [{ path: 'branches', code: 'required', flag: '<branches>', message: 'branches is required' }],
  }
}

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

const handlers = [
  http.get('/api/version', () => HttpResponse.json({ version: 'test' })),
  http.get('/api/repos', () => HttpResponse.json([repo])),
  // the stream says nothing: a test reads the screen as the reads left it
  http.get('/api/events', () => new HttpResponse('', { headers: { 'Content-Type': 'text/event-stream' } })),
  http.post('/api/command-line', async ({ request }) =>
    HttpResponse.json({ argv: spelled((await request.json()) as DomainFollowUp) }),
  ),
  http.post('/api/repos/r1/commands/clean/questions', async ({ request }) =>
    HttpResponse.json(cleanQuestionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/fast-forward/questions', () => HttpResponse.json(ffQuestions())),
  ...answeringExec(),
  ...answeringReparent(),
  ...answeringRunUp(),
  ...answeringCheckout(),
  http.post('/api/repos/r1/commands/create/questions', async ({ request }) =>
    HttpResponse.json(questionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/:command', async ({ params, request }) => {
    const call = (await request.json()) as WebCall
    const asked = (path: string) => call.answers?.find((a) => a.path === path)?.values[0]
    switch (params.command) {
      case 'list':
        return HttpResponse.json(envelope('list', asked('with_prs') === 'true' ? worktrees.map(withPR) : worktrees))
      case 'tree':
        return HttpResponse.json(
          envelope('tree', (asked('with_prs') === 'true' ? worktrees.map(withPR) : worktrees).map(tree)),
        )
      case 'status':
        return HttpResponse.json(
          envelope('status', [status(asked('worktree') ?? 'main', asked('with_prs') === 'true')]),
        )
      case 'create':
        return call.dry_run ? HttpResponse.json(plan(asked('branches') ?? '')) : createRun(asked('branches') ?? '')
      case 'clean':
        return call.dry_run ? cleanPlan(call) : cleanRun(call)
      case 'prune':
        return call.dry_run ? prunePlan(call) : pruneRun(call)
      case 'fast-forward':
        return call.dry_run ? ffPlan(call) : ffRun(call)
    }
    return HttpResponse.json({ error: { message: 'unknown command' } }, { status: 404 })
  }),
]

export const server = setupServer(...handlers)
