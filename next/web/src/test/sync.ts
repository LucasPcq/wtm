import { HttpResponse, http } from 'msw'
import type { DomainPhase, SyncOnto, SyncRebase, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'

// acme for sync: main 3 behind origin, feat/billing 1 behind origin; feat/billing and fix/login lack main's commits,
// feat/billing-ui lacks feat/billing's (it follows it), fix/login has a change; feat/search and feat/search-docs stand
// on their parents
export const behindOrigin: Record<string, number> = { main: 3, 'feat/billing': 1 }
const own: Record<string, number> = { 'feat/billing': 4, 'feat/billing-ui': 2, 'fix/login': 2 }
const needs = ['feat/billing', 'feat/billing-ui', 'fix/login']
const dirtyOnes = worktrees.filter((w) => w.dirty).map((w) => w.branch)
const candidates = worktrees.filter((w) => !w.main).map((w) => w.branch)
const parentOf = (branch: string) => worktrees.find((w) => w.branch === branch)?.parent ?? 'main'
export const under = (branch: string): string[] =>
  candidates.filter((w) => parentOf(w) === branch).flatMap((child) => [child, ...under(child)])

const answerOf = (call: WebCall, path: string) => call.answers?.find((a) => a.path === path)?.values
export const yes = (call: WebCall, path: string) => answerOf(call, path)?.[0] === 'true'
const unlessNo = (call: WebCall, path: string) => answerOf(call, path)?.[0] !== 'false'

// the core's cascade order: parents first, in the order a reader lists them
const cascade = (names: string[]) => candidates.filter((w) => names.includes(w))

// synced: the worktrees a run synced already, which stand on their parents; dirty: those with a change
// dirtyOnto: branches rebased onto with a change; cycle: worktrees whose parents lead back to them; busy: worktrees
// mid-rebase
export type World = {
  synced?: string[]
  dirty?: string[]
  dirtyOnto?: string[]
  cycle?: string[]
  busy?: string[]
  diverged?: string[]
}

export function syncPlanOf(
  call: WebCall,
  { synced = [], dirty = dirtyOnes, dirtyOnto = [], busy = [], diverged = [] }: World = {},
) {
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
    if (busy.includes(branch) || diverged.includes(branch))
      return {
        worktree: branch,
        path: `/acme.trees/${branch.replace('/', '-')}`,
        parent: parentOf(branch),
        depth: 1,
        descendant: !roots.includes(branch),
        ...(busy.includes(branch)
          ? { standing: 'busy' as const, operation: 'rebasing' as const }
          : { standing: 'diverged' as const }),
        commits: 0,
        origin: null,
        from: 'a41c9e2aa1',
        blockers: [],
        phases: [],
      }
    // under a parent this sync leaves (busy, diverged), a worktree stays with it
    if (busy.includes(parentOf(branch)) || diverged.includes(parentOf(branch)))
      return {
        worktree: branch,
        path: `/acme.trees/${branch.replace('/', '-')}`,
        parent: parentOf(branch),
        depth: 2,
        descendant: !roots.includes(branch),
        standing: 'parent_left',
        commits: 0,
        origin: null,
        from: 'a41c9e2aa1',
        blockers: [],
        phases: [],
      }
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
    blockers: dirtyOnto.includes(branch) ? ['dirty'] : [],
    phases:
      behindOrigin[branch] && unlessNo(call, 'ff_parents') && (!dirtyOnto.includes(branch) || force)
        ? ['fast_forward']
        : [],
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

export const fingerprintOf = (call: WebCall) =>
  `sync:${JSON.stringify((call.answers ?? []).filter((answer) => answer.path !== 'no_fetch'))}`

// as the core keeps them: one warning per worktree kept, the last followed by one request forcing all
export const keptWarnings = (plan: ReturnType<typeof syncPlanOf>) => {
  const kept = (plan.worktrees ?? []).filter((r) => r.blockers?.length && !r.phases?.length)
  return kept.map((r, i) => ({
    code: 'sync.kept',
    message: `${r.worktree} is kept as it is: uncommitted changes`,
    params: { worktree: r.worktree, blockers: 'dirty' },
    ...(i === kept.length - 1 && {
      follow_up: {
        command: 'sync',
        code: 'sync.force',
        message: 'sync them anyway, their changes stashed around the rebase',
        answers: [
          { path: 'worktree', values: kept.map((one) => one.worktree) },
          { path: 'force', values: ['true'] },
        ],
      },
    }),
  }))
}

export function syncDryRun(call: WebCall, world: World = {}) {
  const plan = syncPlanOf(call, world)
  const named = answerOf(call, 'worktree') ?? []
  const cycled = named.filter((name) => world.cycle?.includes(name))
  if (cycled.length > 0)
    return HttpResponse.json(
      {
        ...envelope('sync', []),
        ok: false,
        error: {
          kind: 'refused',
          code: 'sync.cycle',
          message: `nothing synced: the parents of ${cycled.join(', ')} lead back to it`,
          params: { worktrees: cycled.join(', ') },
          follow_up: {
            command: 'reparent',
            code: 'sync.reparent',
            message: 'move it onto main',
            answers: [
              { path: 'worktree', values: [cycled[0]] },
              { path: 'to', values: ['main'] },
            ],
          },
        },
      },
      { status: 403 },
    )
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
    warnings: [
      ...keptWarnings(plan),
      ...plan.worktrees
        .filter((r) => r.standing === 'diverged')
        .map((r) => ({
          code: 'sync.diverged',
          message: `origin has commits on ${r.worktree} it never held: left as it is`,
          params: { worktree: r.worktree },
        })),
      ...plan.worktrees
        .filter((r) => r.standing === 'busy')
        .map((r) => ({
          code: 'sync.busy',
          message: `${r.worktree} is in the middle of an operation (rebasing): left as it is`,
          params: { worktree: r.worktree },
          follow_up: {
            command: 'go',
            code: 'sync.go',
            message: 'finish it in the worktree',
            answers: [{ path: 'worktree', values: [r.worktree] }],
          },
        })),
    ],
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

export const answeringSync = () => [
  http.post('/api/repos/r1/commands/sync/questions', () => HttpResponse.json(syncQuestions())),
]
