import { CircleCheck, CircleDashed, CircleSlash, CircleX, GitMerge, type LucideIcon, TriangleAlert } from 'lucide-react'
import type { WorktreeCondition } from '@/api/types.gen'
import type { Tone } from '@/features/worktrees/shared/glyph'

// overlaid is the read a worktree shows of its live read and what GitHub said of it when last asked: the live one,
// whose condition and actions the core decided, but for a pull request GitHub says is merged, which wins over changed
// and clean. It is the web's one rule over two reads, for the list, the tree and the detail alike.
export function overlaid<Read extends { condition: WorktreeCondition }>(live: Read, github: Read | undefined): Read {
  const merged = github?.condition === 'merged' && (live.condition === 'changed' || live.condition === 'clean')
  return merged ? github : live
}

export const conditionLook: Record<WorktreeCondition, { icon: LucideIcon; tone: Tone; label: string }> = {
  no_worktree: { icon: CircleDashed, tone: 'muted', label: 'branch without a worktree' },
  unreadable: { icon: CircleX, tone: 'failed', label: 'unreadable' },
  missing: { icon: CircleSlash, tone: 'failed', label: 'missing' },
  rebasing: { icon: TriangleAlert, tone: 'attention', label: 'rebasing' },
  merged: { icon: GitMerge, tone: 'muted', label: 'merged' },
  changed: { icon: CircleDashed, tone: 'attention', label: 'changed' },
  clean: { icon: CircleCheck, tone: 'ok', label: 'clean' },
}
