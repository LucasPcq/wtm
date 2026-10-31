import { HttpResponse, http } from 'msw'
import type { DomainPhase, SyncOnto, SyncRebase, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// acme for sync: main 3 behind origin, feat/billing 1 behind origin; feat/billing and fix/login lack main's commits,
// feat/billing-ui lacks feat/billing's (it follows it), fix/login has a change; feat/search and feat/search-docs stand
// on their parents
const behindOrigin: Record<string, number> = { main: 3, 'feat/billing': 1 }
const own: Record<string, number> = { 'feat/billing': 4, 'feat/billing-ui': 2, 'fix/login': 2 }
const needs = ['feat/billing', 'feat/billing-ui', 'fix/login']
const dirtyOnes = worktrees.filter((w) => w.dirty).map((w) => w.branch)
const candidates = worktrees.filter((w) => !w.main).map((w) => w.branch)
const parentOf = (branch: string) => worktrees.find((w) => w.branch === branch)?.parent ?? 'main'
const under = (branch: string): string[] =>
  candidates.filter((w) => parentOf(w) === branch).flatMap((child) => [child, ...under(child)])

const answerOf = (call: WebCall, path: string) => call.answers?.find((a) => a.path === path)?.values
const yes = (call: WebCall, path: string) => answerOf(call, path)?.[0] === 'true'
const unlessNo = (call: WebCall, path: string) => answerOf(call, path)?.[0] !== 'false'

// the core's cascade order: parents first, in the order a reader lists them
const cascade = (names: string[]) => candidates.filter((w) => names.includes(w))

// synced: the worktrees a run synced already, which stand on their parents; dirty: those with a change
export type World = { synced?: string[]; dirty?: string[] }

export function syncPlanOf(call: WebCall, { synced = [], dirty = dirtyOnes }: World = {}) {
  const all = yes(call, 'all')
  const named = answerOf(call, 'worktree') ?? []
  const force = yes(call, 'force')
  const descendants = unlessNo(call, 'descendants') && !all
  const picked = named.length > 0 ? named : needs.filter((w) => !dirty.includes(w))
  const roots = all ? candidates : picked
  const below = roots.flatMap(under).filter((w) => !roots.includes(w))
  const chosen = cascade([...roots, ...(descendants ? below : [])])
  const asked = named.length > 0 && !all

  const worktreesPlanned: SyncRebase[] = chosen.map((branch) => {
    const rebases =
      !synced.includes(branch) &&
      (needs.includes(branch) || (descendants && below.includes(branch) && chosen.includes(parentOf(branch))))
    const blockers = rebases && dirty.includes(branch) ? (['dirty'] as const) : []
    const kept = blockers.length > 0 && !force && !(asked && named.includes(branch))
    const push: DomainPhase[] = yes(call, 'push') ? ['push'] : []
    const phases = (): DomainPhase[] => {
      if (rebases && !kept) return ['rebase', ...push]
      return synced.includes(branch) ? push : []
    }
    return {
      worktree: branch,
      path: `/acme.trees/${branch.replace('/', '-')}`,
      parent: parentOf(branch),
      depth: 1,
      descendant: !roots.includes(branch),
      standing: rebases ? 'rebases' : 'on_parent',
      commits: own[branch] ?? 0,
      origin: { ahead: 0, behind: 0, state: 'up-to-date' },
      from: 'a41c9e2aa1',
      blockers: [...blockers],
      phases: phases(),
    }
  })
  const ontoNames = [...new Set(worktreesPlanned.filter((r) => r.standing === 'rebases').map((r) => r.parent))].filter(
    (b) => !chosen.includes(b),
  )
  const onto: SyncOnto[] = ontoNames.map((branch) => ({
    branch,
    children: worktreesPlanned.filter((r) => r.parent === branch).map((r) => r.worktree),
    origin: behindOrigin[branch] ? { ahead: 0, behind: behindOrigin[branch] ?? 0, state: 'behind' } : null,
    from: 'b52d0f3cc3',
    blockers: [],
    phases: behindOrigin[branch] && unlessNo(call, 'ff_parents') ? ['fast_forward'] : [],
  }))

  return {
    descendants,
    ff_parents: unlessNo(call, 'ff_parents'),
    force,
    keep_conflict: yes(call, 'keep_conflict'),
    push: yes(call, 'push'),
    left_out: asked && !descendants ? named.flatMap(under) : [],
    onto,
    worktrees: worktreesPlanned,
    empty: ![...onto, ...worktreesPlanned].some((row) => row.phases?.length),
  }
}

const fingerprintOf = (call: WebCall) =>
  `sync:${JSON.stringify((call.answers ?? []).filter((answer) => answer.path !== 'no_fetch'))}`

const keptWarnings = (plan: ReturnType<typeof syncPlanOf>) =>
  (plan.worktrees ?? [])
    .filter((r) => r.blockers?.length && !r.phases?.length)
    .map((r) => ({
      code: 'sync.kept',
      message: `${r.worktree} is kept as it is: uncommitted changes`,
      params: { worktree: r.worktree, blockers: 'dirty' },
    }))

export function syncDryRun(call: WebCall, world: World = {}) {
  const plan = syncPlanOf(call, world)
  const named = answerOf(call, 'worktree') ?? []
  const refused = plan.worktrees.filter((r) => named.includes(r.worktree) && r.blockers?.length && !plan.force)
  if (refused.length > 0)
    return HttpResponse.json(
      {
        ...envelope('sync', []),
        ok: false,
        error: {
          kind: 'refused',
          code: 'sync.unsafe',
          message: `nothing synced: ${refused[0]?.worktree} has uncommitted changes`,
        },
      },
      { status: 403 },
    )
  return HttpResponse.json({
    ...envelope('sync', []),
    fingerprint: fingerprintOf(call),
    plan,
    warnings: keptWarnings(plan),
  })
}

export const syncQuestions = () => ({
  fields: [
    {
      kind: 'multi_select',
      path: 'worktree',
      picked: needs.filter((w) => !dirtyOnes.includes(w)),
      options: candidates.map((w) => ({ value: w, parent: parentOf(w), changes: dirtyOnes.includes(w) ? 2 : 0 })),
    },
  ],
  problems: [],
})

// conflict: the worktree whose rebase stops on a conflict, aborted unless the run keeps it
// ffFailed: the fast-forward of the branches rebased onto fails, its children rebased onto them as they stand;
// pushRefused: origin refuses that worktree's push, its rebase standing and the ones under it blocked
export function syncRun(
  call: WebCall,
  { conflict, ffFailed = false, pushRefused }: { conflict?: string; ffFailed?: boolean; pushRefused?: string } = {},
) {
  const plan = syncPlanOf(call)
  const moving = [...plan.onto.filter((o) => o.phases?.length).map((o) => o.branch)]
  const failing = conflict ?? pushRefused
  const stopped = (r: SyncRebase) => failing !== undefined && under(failing).includes(r.worktree)
  const keep = yes(call, 'keep_conflict')

  const items = [
    ...moving.map((branch) =>
      ffFailed
        ? {
            subject: branch,
            status: 'failed',
            detail: {},
            error: {
              kind: 'conflict',
              code: 'sync.ff_failed',
              message: `couldn't fast-forward ${branch}: not a fast-forward`,
              params: { branch, cause: 'not a fast-forward', children: 'feat/billing' },
            },
          }
        : {
            subject: branch,
            status: 'done',
            detail: {
              path: '/acme',
              parent: '',
              from: 'b52d0f3cc3',
              commits: behindOrigin[branch] ?? 0,
              pushed: false,
            },
          },
    ),
    ...(plan.worktrees ?? []).map((r) => {
      if (!r.phases?.length) return { subject: r.worktree, status: 'skipped', reason: 'ruled_out', detail: {} }
      if (r.worktree === conflict)
        return {
          subject: r.worktree,
          status: 'failed',
          detail: {},
          error: {
            kind: 'conflict',
            code: 'sync.conflict',
            message: `the rebase onto ${r.parent} stopped on a conflict in src/billing/invoice.ts`,
            params: { worktree: r.worktree, parent: r.parent, files: 'src/billing/invoice.ts', kept: String(keep) },
          },
        }
      if (r.worktree === pushRefused)
        return {
          subject: r.worktree,
          status: 'failed',
          detail: { path: r.path, parent: r.parent, from: r.from, commits: r.commits, pushed: false },
          error: {
            kind: 'internal',
            code: 'sync.push_failed',
            message: 'git refused the push',
            params: { worktree: r.worktree, parent: r.parent, cause: '! [rejected] feat/billing (stale info)' },
          },
        }
      if (stopped(r)) return { subject: r.worktree, status: 'skipped', reason: 'blocked', detail: {} }
      return {
        subject: r.worktree,
        status: 'done',
        detail: { path: r.path, parent: r.parent, from: r.from, commits: r.commits, pushed: yes(call, 'push') },
      }
    }),
  ]
  const synced = items.filter((i) => i.status === 'done' && !moving.includes(i.subject)).map((i) => i.subject)
  const follow_ups = [
    ...(conflict && !keep
      ? [
          {
            command: 'sync',
            code: 'sync.resolve',
            message: 'resolve the conflict by hand',
            answers: [
              { path: 'worktree', values: [conflict] },
              { path: 'keep_conflict', values: ['true'] },
            ],
          },
        ]
      : []),
    ...(!yes(call, 'push') && synced.length > 0
      ? [
          {
            command: 'sync',
            code: 'sync.push',
            message: 'push them, force-with-lease',
            answers: [
              { path: 'worktree', values: synced },
              { path: 'descendants', values: ['false'] },
              { path: 'push', values: ['true'] },
            ],
          },
        ]
      : []),
  ]

  if (call.expect_fingerprint !== fingerprintOf(call))
    return streamed(
      `event: envelope\ndata: ${JSON.stringify({ ...envelope('sync', []), ok: false, error: { kind: 'precondition', code: 'plan.stale', message: 'the facts changed since the preview' } })}\n\n`,
    )
  return streamed(
    [
      ...moving.flatMap((branch) => [
        line({ type: 'phase.started', subject: branch, phase: 'fast_forward' }),
        line({ type: 'phase.finished', subject: branch, phase: 'fast_forward', failed: false }),
      ]),
      ...(plan.worktrees ?? [])
        .filter((r) => r.phases?.length && !stopped(r))
        .flatMap((r) => [
          line({ type: 'unit.started', subject: r.worktree }),
          line({ type: 'phase.started', subject: r.worktree, phase: 'rebase' }),
          line({ type: 'phase.finished', subject: r.worktree, phase: 'rebase', failed: r.worktree === conflict }),
          line({ type: 'unit.finished', subject: r.worktree, status: r.worktree === conflict ? 'failed' : 'done' }),
        ]),
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('sync', []),
        ok: !failing && !ffFailed,
        items,
        plan,
        follow_ups,
        warnings: keptWarnings(plan),
      })}\n\n`,
    ].join(''),
  )
}

export const answeringSync = () => [
  http.post('/api/repos/r1/commands/sync/questions', () => HttpResponse.json(syncQuestions())),
]
