import type { DomainAnswered, TeardownRemoval } from '@/api/types.gen'
import { unsafe } from '@/features/worktrees/utils/removal'

export function requestOf({ going, reparent }: { going: TeardownRemoval[]; reparent: boolean }): DomainAnswered[] {
  return [
    { path: 'worktree', values: going.map((removal) => removal.worktree) },
    ...(going.some(unsafe) ? [{ path: 'force', values: ['true'] }] : []),
    ...(reparent ? [{ path: 'reparent_children', values: ['true'] }] : []),
  ]
}
