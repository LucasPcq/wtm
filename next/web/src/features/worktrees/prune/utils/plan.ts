import type { DomainAnswered, PruneCandidate } from '@/api/types.gen'
import { unsafe } from '@/features/worktrees/utils/removal'

// the safe candidates, as checked when the dialog opened on none named, are prune's own pick: the request names none,
// and prune keeps the unsafe ones with their warning; any other pick names the checked ones, forced only when the
// person checked an unsafe one. Every plan reads origin as the review fetched it, and the apply holds to it.
export function requestOf({
  named,
  candidates,
  going,
  reparent,
}: {
  named: string[]
  candidates: PruneCandidate[]
  going: PruneCandidate[]
  reparent: boolean
}): DomainAnswered[] {
  const safe = candidates.filter((candidate) => !unsafe(candidate))
  const own = named.length === 0 && going.length === safe.length && going.every((candidate) => !unsafe(candidate))

  return [
    ...(own ? [] : [{ path: 'worktree', values: going.map((candidate) => candidate.worktree) }]),
    ...(!own && going.some(unsafe) ? [{ path: 'force', values: ['true'] }] : []),
    ...(reparent ? [{ path: 'reparent_children', values: ['true'] }] : []),
    { path: 'no_fetch', values: ['true'] },
  ]
}

// a plan entry with no phase is a candidate prune leaves as it is
export const kept = (candidate: PruneCandidate) => (candidate.phases ?? []).length === 0
