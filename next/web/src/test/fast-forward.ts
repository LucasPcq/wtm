import { HttpResponse } from 'msw'
import type { FastforwardAdvance, WebCall, WorktreeOrigin } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// acme against origin: main 3 behind, feat/billing 1 ahead, feat/billing-ui 2 behind, fix/login 1 behind with its
// change (unsafe until forced), feat/search diverged, feat/search-docs not on origin
const against: Record<string, WorktreeOrigin | null> = {
  main: { ahead: 0, behind: 3, state: 'behind' },
  'feat/billing': { ahead: 1, behind: 0, state: 'ahead' },
  'feat/billing-ui': { ahead: 0, behind: 2, state: 'behind' },
  'fix/login': { ahead: 0, behind: 1, state: 'behind' },
  'feat/search': { ahead: 1, behind: 4, state: 'diverged' },
  'feat/search-docs': null,
}

const dirtyOnes = worktrees.filter((w) => w.dirty).map((w) => w.branch)

const standingOf = (origin: WorktreeOrigin | null): FastforwardAdvance['standing'] => {
  if (!origin) return 'no_origin'
  return origin.state === 'up-to-date' ? 'up_to_date' : origin.state
}

export const advanceOf = (branch: string, forced: boolean, dirty = dirtyOnes): FastforwardAdvance => {
  const origin = against[branch] ?? null
  const standing = standingOf(origin)
  const blockers = standing === 'behind' && dirty.includes(branch) ? (['dirty'] as const) : []
  return {
    worktree: branch,
    path: `/acme.trees/${branch.replace('/', '-')}`,
    standing,
    origin,
    from: 'a41c9e2aa1',
    ...(standing === 'behind' && { to: '7d3f1b4bb2' }),
    blockers: [...blockers],
    phases: standing === 'behind' && (blockers.length === 0 || forced) ? ['fast_forward'] : [],
  }
}

export const ffQuestions = () => ({
  fields: [
    {
      kind: 'multi_select',
      path: 'worktree',
      picked: worktrees
        .filter((w) => against[w.branch]?.state === 'behind' && !dirtyOnes.includes(w.branch))
        .map((w) => w.branch),
      options: worktrees.map((w) => ({
        value: w.branch,
        ...(against[w.branch] && { origin: against[w.branch] }),
        changes: dirtyOnes.includes(w.branch) ? 1 : 0,
      })),
    },
  ],
  problems: [],
})

const asked = (call: WebCall) => call.answers?.find((a) => a.path === 'worktree')?.values
const forcedIn = (call: WebCall) => call.answers?.some((a) => a.path === 'force' && a.values[0] === 'true') ?? false

// unnamed, the command picks those behind origin and keeps the unsafe ones
const advancesOf = (call: WebCall, dirty: string[]) => {
  const names = asked(call)
  const forced = forcedIn(call)
  if (names) return names.map((branch) => advanceOf(branch, forced, dirty))
  return worktrees.map((w) => advanceOf(w.branch, false, dirty)).filter((advance) => advance.standing === 'behind')
}

const leftWarnings = (advances: FastforwardAdvance[]) =>
  advances.flatMap((advance) => {
    const { worktree, origin } = advance
    if (advance.standing === 'diverged')
      return [
        {
          code: 'fast-forward.diverged',
          message: `${worktree} has diverged from origin (${origin?.ahead} ahead, ${origin?.behind} behind): left as it is`,
          params: { worktree, ahead: String(origin?.ahead), behind: String(origin?.behind) },
          follow_up: {
            command: 'go',
            answers: [{ path: 'worktree', values: [worktree] }],
            code: 'fast-forward.go',
            message: 'reconcile it with origin by hand',
          },
        },
      ]
    if (advance.standing === 'no_origin')
      return [
        {
          code: 'fast-forward.no_origin',
          message: `origin has no branch ${worktree}: nothing to fast-forward it to`,
          params: { worktree },
        },
      ]
    return []
  })

// as the core keeps them (rules.go kept): one warning per worktree kept, the last followed by one request forcing all
const keptWarnings = (advances: FastforwardAdvance[]) => {
  const kept = advances.filter((advance) => advance.blockers?.length && !advance.phases?.length)
  const names = kept.map((advance) => advance.worktree)
  return kept.map((advance, i) => ({
    code: 'fast-forward.kept',
    message: `${advance.worktree} is kept as it is: uncommitted changes`,
    params: { worktree: advance.worktree, blockers: 'dirty' },
    ...(i === kept.length - 1 && {
      follow_up: {
        command: 'fast-forward',
        answers: [
          { path: 'worktree', values: names },
          { path: 'force', values: ['true'] },
        ],
        code: 'fast-forward.force',
        message: 'fast-forward them anyway',
      },
    }),
  }))
}

const warningsOf = (advances: FastforwardAdvance[]) => [...leftWarnings(advances), ...keptWarnings(advances)]

// the plan's fingerprint is its request but whether it fetched, which the core's plan does not hold: a run that
// expects another is refused as stale
const fingerprintOf = (call: WebCall) =>
  `ff:${JSON.stringify((call.answers ?? []).filter((answer) => answer.path !== 'no_fetch'))}`

export const ffPlan = (call: WebCall, dirty = dirtyOnes) => {
  const advances = advancesOf(call, dirty)
  const unsafe = asked(call) && !forcedIn(call) ? advances.filter((advance) => advance.blockers?.length) : []
  if (unsafe.length > 0)
    return HttpResponse.json(
      {
        ...envelope('fast-forward', []),
        ok: false,
        error: {
          kind: 'refused',
          code: 'fast-forward.unsafe',
          message: `nothing fast-forwarded: ${unsafe.map((a) => a.worktree).join(', ')} can't be fast-forwarded safely`,
        },
      },
      { status: 403 },
    )

  return HttpResponse.json({
    ...envelope('fast-forward', []),
    fingerprint: fingerprintOf(call),
    plan: { worktrees: advances, force: forcedIn(call), empty: !advances.some((advance) => advance.phases?.length) },
    warnings: warningsOf(advances),
  })
}

const refusal = (error: object) =>
  streamed(`event: envelope\ndata: ${JSON.stringify({ ...envelope('fast-forward', []), ok: false, error })}\n\n`)

// as the core refuses an apply naming a worktree that gained changes since its plan (rules.go refusal)
export const ffUnsafe = (names: string[]) =>
  refusal({
    kind: 'refused',
    code: 'fast-forward.unsafe',
    message: `nothing fast-forwarded: ${names.join(', ')} can't be fast-forwarded safely`,
    fields: names.map((name, i) => ({
      path: `worktree[${i}]`,
      code: 'fast-forward.dirty',
      params: { value: name },
      message: `${name} has uncommitted changes`,
    })),
  })

const itemOf = (advance: FastforwardAdvance) => {
  if (advance.phases?.length)
    return {
      subject: advance.worktree,
      status: 'done',
      detail: { path: advance.path, from: advance.from, to: advance.to },
    }
  if (advance.standing === 'ahead' || advance.standing === 'up_to_date')
    return { subject: advance.worktree, status: 'done', reason: 'unchanged', detail: {} }
  return { subject: advance.worktree, status: 'skipped', reason: 'ruled_out', detail: {} }
}

export const ffRun = (call: WebCall, failing: string[] = []) => {
  if (call.expect_fingerprint !== fingerprintOf(call))
    return refusal({ kind: 'precondition', code: 'plan.stale', message: 'the facts changed since the preview' })

  const advances = advancesOf(call, dirtyOnes)
  const moving = advances.filter((advance) => advance.phases?.length)
  const error = (subject: string) => ({
    kind: 'internal',
    code: 'fast-forward.failed',
    message: 'git refused to move the branch',
    params: {
      branch: subject,
      cause: 'error: The following untracked working tree files would be overwritten by merge:\n\tsrc/auth/callback.ts',
    },
  })
  const failed = (subject: string) => failing.includes(subject)
  return streamed(
    [
      ...moving.flatMap(({ worktree }) => [
        line({ type: 'unit.started', subject: worktree }),
        line({ type: 'phase.started', subject: worktree, phase: 'fast_forward' }),
        line({ type: 'phase.finished', subject: worktree, phase: 'fast_forward', failed: failed(worktree) }),
        line({ type: 'unit.finished', subject: worktree, status: failed(worktree) ? 'failed' : 'done' }),
      ]),
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('fast-forward', []),
        ok: !moving.some((advance) => failed(advance.worktree)),
        items: advances.map((advance) =>
          failed(advance.worktree)
            ? { subject: advance.worktree, status: 'failed', detail: {}, error: error(advance.worktree) }
            : itemOf(advance),
        ),
        plan: { worktrees: advances, force: forcedIn(call) },
        warnings: warningsOf(advances),
      })}\n\n`,
    ].join(''),
  )
}
