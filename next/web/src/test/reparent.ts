import { HttpResponse, http } from 'msw'
import type { DomainAnswered, DomainField, DomainFieldError, ReparentMove, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// reparent over acme, as the core decides it: main is never moved, a worktree is refused as its own parent, and so is
// a branch under one it moves (a cycle); the record of a worktree in failing does not write
const asked = (answers: DomainAnswered[] | undefined, path: string) =>
  answers?.find((a) => a.path === path)?.values ?? []

// the parents a stateful fake moved, over acme's
type Moved = Map<string, string>

const parentOf = (branch: string, moved: Moved = new Map()) =>
  moved.get(branch) ?? worktrees.find((w) => w.branch === branch)?.parent ?? ''

const under = (branch: string, moving: string[]): string | undefined => {
  const parent = parentOf(branch)
  if (!parent) return undefined
  return moving.includes(parent) ? parent : under(parent, moving)
}

function refusalOf(to: string, moving: string[]): DomainFieldError | undefined {
  if (moving.includes(to))
    return {
      path: 'to',
      code: 'self_parent',
      flag: '--to',
      params: { value: to },
      message: `${to} cannot be its own parent: pass another value to --to`,
    }
  const root = under(to, moving)
  if (!root) return undefined
  return {
    path: 'to',
    code: 'reparent.cycle',
    flag: '--to',
    params: { value: to, under: root },
    message: `${to} sits under ${root}: moving ${root} onto it would close a cycle`,
  }
}

const movable = worktrees.filter((w) => !w.main)

export function reparentQuestionsOf(answers: DomainAnswered[] = []) {
  const moving = asked(answers, 'worktree')
  const branches = worktrees.map((w) => w.branch)
  return {
    fields: [
      {
        kind: 'multi_select',
        path: 'worktree',
        picked: moving,
        options: movable.map((w) => ({ value: w.branch, parent: w.parent })),
      },
      {
        kind: 'select',
        path: 'to',
        value: '',
        refreshable: true,
        closed: false,
        remember: false,
        rememberable: false,
        pinned: 'main',
        options: branches.filter((branch) => !refusalOf(branch, moving)).map((value) => ({ value })),
        refused: branches.flatMap((branch) => refusalOf(branch, moving) ?? []),
      },
    ] as DomainField[],
    problems: [],
  }
}

const movesOf = (call: WebCall, moved?: Moved): ReparentMove[] => {
  const [to = ''] = asked(call.answers, 'to')
  return asked(call.answers, 'worktree').map((branch) => ({
    branch,
    path: `/acme.trees/${branch.replace('/', '-')}`,
    from: parentOf(branch, moved),
    parent_gone: false,
    to,
    children: worktrees.filter((w) => w.parent === branch).map((w) => w.branch),
  }))
}

export function reparentPlan(call: WebCall, moved?: Moved) {
  const [to = ''] = asked(call.answers, 'to')
  const refusal = refusalOf(to, asked(call.answers, 'worktree'))
  if (refusal)
    return HttpResponse.json(
      {
        ...envelope('reparent', []),
        ok: false,
        error: { kind: 'invalid', code: 'request.invalid', message: 'the request is invalid', fields: [refusal] },
      },
      { status: 400 },
    )
  const moves = movesOf(call, moved)
  return HttpResponse.json({
    ...envelope('reparent', []),
    fingerprint: `rp:${JSON.stringify(call.answers)}`,
    plan: { moves, to, recap: [], empty: moves.every((move) => move.from === move.to) },
  })
}

export const reparentRun =
  (failing: string[] = [], moved?: Moved) =>
  (call: WebCall) => {
    const moves = movesOf(call, moved)
    const going = moves.filter((move) => move.from !== move.to)
    const items = moves.map((move) => {
      const detail = { path: move.path, from: move.from, to: move.to }
      if (move.from === move.to) return { subject: move.branch, status: 'done', reason: 'unchanged', detail }
      if (failing.includes(move.branch))
        return {
          subject: move.branch,
          status: 'failed',
          error: {
            kind: 'internal',
            code: 'reparent.write_failed',
            message: `couldn't write the record of ${move.branch}`,
            params: { cause: 'permission denied' },
          },
          detail: {},
        }
      return { subject: move.branch, status: 'done', detail }
    })
    const written = going.filter((move) => !failing.includes(move.branch))
    for (const move of written) moved?.set(move.branch, move.to)
    return streamed(
      [
        ...going.flatMap((move) => [
          line({ type: 'unit.started', subject: move.branch }),
          line({
            type: 'unit.finished',
            subject: move.branch,
            status: failing.includes(move.branch) ? 'failed' : 'done',
          }),
        ]),
        `event: envelope\ndata: ${JSON.stringify({
          ...envelope('reparent', []),
          ok: failing.length === 0,
          items,
          plan: { moves, to: moves[0]?.to ?? '', recap: [], empty: going.length === 0 },
          follow_ups:
            written.length === 0
              ? []
              : [
                  {
                    command: 'sync',
                    answers: [{ path: 'worktree', values: written.map((move) => move.branch) }],
                    code: 'reparent.sync',
                    message: 'rebase onto the new parent',
                  },
                  ...written.map((move) => ({
                    command: 'reparent',
                    answers: [
                      { path: 'worktree', values: [move.branch] },
                      { path: 'to', values: [move.from] },
                    ],
                    code: 'reparent.undo',
                    message: `move ${move.branch} back onto ${move.from}`,
                  })),
                ],
        })}\n\n`,
      ].join(''),
    )
  }

export const answeringReparent = (run: (call: WebCall) => Response = reparentRun()) => [
  http.post('/api/repos/r1/commands/reparent/questions', async ({ request }) =>
    HttpResponse.json(reparentQuestionsOf(((await request.json()) as WebCall).answers)),
  ),
  http.post('/api/repos/r1/commands/reparent', async ({ request }) => {
    const call = (await request.json()) as WebCall
    return call.dry_run ? reparentPlan(call) : run(call)
  }),
]

// a fake that remembers what it moved: a second run (an undo) plans from where the first left each worktree
export function rememberingReparent() {
  const moved: Moved = new Map()
  return [
    http.post('/api/repos/r1/commands/reparent', async ({ request }) => {
      const call = (await request.json()) as WebCall
      return call.dry_run ? reparentPlan(call, moved) : reparentRun([], moved)(call)
    }),
  ]
}
