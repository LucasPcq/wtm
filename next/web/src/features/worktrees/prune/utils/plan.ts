import type { DomainAnswered, PruneCandidate } from '@/api/types.gen'
import { unsafe } from '@/features/worktrees/utils/removal'

// opened on no worktree, a checked set equal to the default pick (every safe candidate, no unsafe one) is prune's own:
// the request names none, and prune keeps the unsafe ones with their warning. Any other pick names the checked ones,
// forced only when the person checked an unsafe one. Every plan reads origin as the review fetched it.
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
