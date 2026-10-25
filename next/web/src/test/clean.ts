import { HttpResponse } from 'msw'
import type { DomainAnswered, DomainField, WebCall } from '@/api/types.gen'
import { worktrees } from '@/test/acme'
import { envelope } from '@/test/fixtures'
import { line, streamed } from '@/test/stream'

const field = (over: Partial<DomainField> & { kind: DomainField['kind']; path: string }): DomainField =>
  ({ ...over }) as DomainField

// clean offers every worktree but main; fix/login, with a change, is unsafe until forced
export function cleanQuestionsOf(answers: DomainAnswered[] = []) {
  const picked = answers.find((a) => a.path === 'worktree')?.values ?? []
  return {
    fields: [
      field({
        kind: 'multi_select',
        path: 'worktree',
        picked,
        options: worktrees.filter((w) => !w.main).map((w) => ({ value: w.branch })),
      }),
    ],
    problems: picked.length
      ? []
      : [{ path: 'worktree', code: 'required', flag: '<worktree>', message: 'worktree is required' }],
  }
}

export const cleanPlan = (call: WebCall) => {
  const picked = call.answers?.find((a) => a.path === 'worktree')?.values ?? []
  const said = (path: string) => call.answers?.some((a) => a.path === path && a.values[0] === 'true') ?? false
  const unsafe = picked.filter((branch) => worktrees.find((w) => w.branch === branch)?.dirty)
  if (unsafe.length > 0 && !said('force'))
    return HttpResponse.json(
      {
        ...envelope('clean', []),
        ok: false,
        error: {
          kind: 'refused',
          code: 'clean.unsafe',
          message: `nothing removed: ${unsafe.length} worktree(s) can't be removed safely`,
          fields: unsafe.map((branch) => ({
            path: `worktree[${picked.indexOf(branch)}]`,
            code: 'clean.dirty',
            params: { value: branch },
            flag: '<worktree>',
            message: `${branch} has uncommitted changes`,
          })),
        },
      },
      { status: 403 },
    )

  const children = worktrees.filter((w) => picked.includes(w.parent) && !picked.includes(w.branch))
  return HttpResponse.json({
    ...envelope('clean', []),
    fingerprint: 'c1',
    plan: {
      worktrees: picked.map((branch) => ({
        worktree: branch,
        branch,
        path: `/acme.trees/${branch.replace('/', '-')}`,
        hooks: ['docker compose down'],
        phases: ['on_clean', 'remove'],
        blockers: unsafe.includes(branch) ? ['dirty'] : [],
      })),
      moves: children.map((w) => ({ branch: w.branch, from: w.parent, to: 'main', base: true })),
      reparent_children: said('reparent_children'),
      force: said('force'),
    },
  })
}

export const cleanRun = (call: WebCall) => {
  const picked = call.answers?.find((a) => a.path === 'worktree')?.values ?? []
  const reparent = call.answers?.some((a) => a.path === 'reparent_children') ?? false
  const orphans = reparent ? [] : worktrees.filter((w) => picked.includes(w.parent) && !picked.includes(w.branch))
  return streamed(
    [
      ...picked.flatMap((branch) => [
        line({ type: 'unit.started', subject: branch }),
        line({ type: 'phase.started', subject: branch, phase: 'on_clean' }),
        line({ type: 'phase.output', subject: branch, line: 'Container acme-api-1  Stopped' }),
        line({ type: 'phase.finished', subject: branch, phase: 'on_clean', failed: false }),
        line({ type: 'phase.started', subject: branch, phase: 'remove' }),
        line({ type: 'phase.finished', subject: branch, phase: 'remove', failed: false }),
        line({ type: 'unit.finished', subject: branch, status: 'done' }),
      ]),
      `event: envelope\ndata: ${JSON.stringify({
        ...envelope('clean', []),
        items: picked.map((branch) => ({ subject: branch, status: 'done', detail: { path: `/acme.trees/${branch}` } })),
        plan: {
          worktrees: [],
          moves: worktrees
            .filter((w) => picked.includes(w.parent) && !picked.includes(w.branch))
            .map((w) => ({ branch: w.branch, from: w.parent, to: 'main', base: true })),
          reparent_children: reparent,
          force: false,
        },
        warnings: orphans.map((w) => ({
          code: 'clean.orphaned',
          message: `${w.branch} still points at the removed parent ${w.parent}`,
          follow_up: {
            command: 'reparent',
            answers: [
              { path: 'worktree', values: [w.branch] },
              { path: 'to', values: ['main'] },
            ],
            code: 'clean.reparent',
            message: 'move it onto main',
          },
        })),
      })}\n\n`,
    ].join(''),
  )
}
