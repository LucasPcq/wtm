import { HttpResponse } from 'msw'
import type { DomainWarning, PruneCandidate, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

// feat/billing's pull request #214 merged; origin deleted feat/search-docs, which holds 1 commit no other branch does
export const merged: PruneCandidate = {
  worktree: 'feat/billing',
  branch: 'feat/billing',
  path: '/acme.trees/feat-billing',
  hooks: ['docker compose down'],
  phases: ['on_clean', 'remove'],
  blockers: [],
  reason: 'merged',
  pr: { number: 214, url: 'https://github.com/acme/acme/pull/214' },
}

export const gone: PruneCandidate = {
  worktree: 'feat/search-docs',
  branch: 'feat/search-docs',
  path: '/acme.trees/feat-search-docs',
  phases: ['remove'],
  blockers: ['unpushed'],
  unpushed: 1,
  reason: 'gone',
}

export const candidates = [merged, gone]

const fingerprint = 'p1'

const said = (call: WebCall, path: string) => call.answers?.some((a) => a.path === path && a.values[0] === 'true')
const named = (call: WebCall) => call.answers?.find((a) => a.path === 'worktree')?.values ?? []

// prune's own pick (no worktree named) keeps an unsafe candidate unless forced, as a plan entry with no phase and a
// warning; a worktree named that is no candidate is a field error on its entry
export function prunePlan(
  call: WebCall,
  { found = candidates, warnings = [] }: { found?: PruneCandidate[]; warnings?: DomainWarning[] } = {},
) {
  const names = named(call)
  const known = found.map((c) => c.worktree)
  const stranger = names.findIndex((name) => !known.includes(name))
  if (stranger >= 0)
    return HttpResponse.json(
      {
        ...envelope('prune', []),
        ok: false,
        error: {
          kind: 'invalid',
          code: 'domain.invalid',
          message: `${names[stranger]} is not done with: it has commits past its pull request's head`,
          fields: [
            {
              path: `worktree[${stranger}]`,
              code: 'prune.not_finished',
              flag: '<worktree>',
              params: { value: names[stranger], why: "it has commits past its pull request's head" },
              message: `${names[stranger]} is not done with: it has commits past its pull request's head`,
            },
          ],
        },
      },
      { status: 400 },
    )

  const force = said(call, 'force') ?? false
  const unsafeNamed = found.filter((c) => names.includes(c.worktree) && (c.blockers ?? []).length > 0)
  if (unsafeNamed.length > 0 && !force)
    return HttpResponse.json(unsafeRefusal(unsafeNamed.map((c) => c.worktree)), { status: 403 })
  const picked = names.length > 0 ? found.filter((c) => names.includes(c.worktree)) : found
  const planned = picked.map((c) =>
    names.length === 0 && !force && (c.blockers ?? []).length > 0 ? { ...c, phases: [], hooks: [] } : c,
  )
  const kept = planned.filter((c) => (c.phases ?? []).length === 0)
  const removed = planned.filter((c) => (c.phases ?? []).length > 0).map((c) => c.branch)
  const children = worktrees.filter((w) => removed.includes(w.parent) && !removed.includes(w.branch))
  return HttpResponse.json({
    ...envelope('prune', []),
    fingerprint,
    plan: {
      worktrees: planned,
      empty: kept.length === planned.length,
      moves: children.map((w) => ({ branch: w.branch, from: w.parent, to: 'main', base: true })),
      reasons: ['merged', 'closed', 'gone'],
      reparent_children: said(call, 'reparent_children') ?? false,
      force,
    },
    warnings: [
      ...warnings,
      ...kept.map((c, i) => ({
        code: 'prune.kept',
        message: `${c.worktree} is kept: unsafe to remove (unpushed)`,
        params: { worktree: c.worktree, blockers: 'unpushed' },
        ...(i === kept.length - 1 && {
          follow_up: {
            command: 'prune',
            answers: [
              { path: 'worktree', values: kept.map((one) => one.worktree) },
              { path: 'force', values: ['true'] },
              ...(call.answers ?? []).filter((a) => !['worktree', 'force', 'no_fetch'].includes(a.path)),
            ],
            code: 'prune.force',
            message: 'prune them anyway',
          },
        }),
      })),
    ],
  })
}

// the run applies the plan its answers make, each worktree it removes going through its phases
export function pruneRun(call: WebCall) {
  if (call.expect_fingerprint !== fingerprint)
    return streamed(
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('prune', []),
        ok: false,
        error: { kind: 'precondition', code: 'plan.stale', message: 'the facts changed since the preview' },
      })}\n\n`,
    )

  const names = named(call)
  const force = said(call, 'force') ?? false
  const going = (names.length > 0 ? candidates.filter((c) => names.includes(c.worktree)) : candidates).filter(
    (c) => names.length > 0 || force || (c.blockers ?? []).length === 0,
  )
  const keptOnes = candidates.filter((c) => names.length === 0 && !force && (c.blockers ?? []).length > 0)
  return streamed(
    [
      ...going.flatMap(({ worktree, phases }) => [
        line({ type: 'unit.started', subject: worktree }),
        ...(phases ?? []).flatMap((phase) => [
          line({ type: 'phase.started', subject: worktree, phase }),
          line({ type: 'phase.finished', subject: worktree, phase, failed: false }),
        ]),
        line({ type: 'unit.finished', subject: worktree, status: 'done' }),
      ]),
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('prune', []),
        items: [
          ...going.map(({ worktree, path, reason }) => ({
            subject: worktree,
            status: 'done',
            detail: { path, reason },
          })),
          ...keptOnes.map(({ worktree, path, reason }) => ({
            subject: worktree,
            status: 'skipped',
            reason: 'ruled_out',
            detail: { path, reason },
          })),
        ],
        plan: { worktrees: going, reasons: ['merged', 'closed', 'gone'], reparent_children: false, force },
        warnings: keptOnes.map((c) => ({
          code: 'prune.kept',
          message: `${c.worktree} is kept: unsafe to remove (unpushed)`,
          params: { worktree: c.worktree, blockers: 'unpushed' },
          follow_up: {
            command: 'prune',
            answers: [
              { path: 'worktree', values: [c.worktree] },
              { path: 'force', values: ['true'] },
            ],
            code: 'prune.force',
            message: 'prune them anyway',
          },
        })),
      })}\n\n`,
    ].join(''),
  )
}

// the core's refusal of a request naming a worktree unsafe to remove without force, before anything runs
export const unsafeRefusal = (worktrees: string[]) => ({
  ...envelope('prune', []),
  ok: false,
  error: {
    kind: 'refused',
    code: 'prune.unsafe',
    message: `nothing removed: ${worktrees.join(', ')} can't be removed safely`,
    fields: worktrees.map((worktree, i) => ({
      path: `worktree[${i}]`,
      code: 'prune.dirty',
      flag: '<worktree>',
      params: { value: worktree },
      message: `${worktree} has uncommitted changes`,
    })),
  },
})
