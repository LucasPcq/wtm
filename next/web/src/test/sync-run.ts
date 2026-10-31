import type { SyncRebase, WebCall } from '@/api/types.gen'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'
import { behindOrigin, fingerprintOf, keptWarnings, syncPlanOf, under, type World, yes } from '@/test/sync'

// conflict: the worktree whose rebase stops on a conflict, aborted unless the run keeps it
// ffFailed: the fast-forward of the branches rebased onto fails, its children rebased onto them as they stand;
// pushRefused: origin refuses that worktree's push, its rebase standing and the ones under it blocked
export function syncRun(
  call: WebCall,
  {
    conflict,
    ffFailed = false,
    pushRefused,
    world = {},
  }: { conflict?: string; ffFailed?: boolean; pushRefused?: string; world?: World } = {},
) {
  const plan = syncPlanOf(call, world)
  const moving = [...plan.onto.filter((o) => o.phases?.length).map((o) => o.branch)]
  const failing = conflict ?? pushRefused
  const stopped = (r: SyncRebase) => failing !== undefined && under(failing).includes(r.worktree)
  const keep = yes(call, 'keep_conflict')

  const items = [
    ...moving.map((branch) =>
      ffFailed
        ? {
            subject: branch,
            status: 'skipped',
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
    ...(plan.left_out.length > 0 && !failing
      ? [
          {
            command: 'sync',
            code: 'sync.left_out',
            message: plan.left_out.length === 1 ? 'sync it too' : 'sync them too',
            answers: [{ path: 'worktree', values: plan.left_out }],
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
        ok: !failing,
        items,
        plan,
        follow_ups,
        warnings: [
          ...keptWarnings(plan),
          ...plan.worktrees
            .filter((one) => one.standing === 'diverged')
            .map((one) => ({
              code: 'sync.diverged',
              message: `origin has commits on ${one.worktree} it never held: left as it is`,
              params: { worktree: one.worktree },
            })),
        ],
      })}\n\n`,
    ].join(''),
  )
}
