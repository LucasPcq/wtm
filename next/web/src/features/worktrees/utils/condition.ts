import { CircleCheck, CircleDashed, CircleSlash, CircleX, GitMerge, type LucideIcon, TriangleAlert } from 'lucide-react'
import type { WorktreeCondition } from '@/api/types.gen'
import type { Tone } from '@/features/worktrees/components/glyph'

// the one rule over the live and GitHub reads, for every view: a merge GitHub reports wins over changed and clean
export function overlaid<Read extends { condition: WorktreeCondition }>(live: Read, github: Read | undefined): Read {
  const merged = github?.condition === 'merged' && (live.condition === 'changed' || live.condition === 'clean')
  return merged ? github : live
}

export const conditionLook: Record<WorktreeCondition, { icon: LucideIcon; tone: Tone; label: string }> = {
  no_worktree: { icon: CircleDashed, tone: 'muted', label: 'branch without a worktree' },
  deleted: { icon: CircleSlash, tone: 'muted', label: 'branch deleted' },
  unreadable: { icon: CircleX, tone: 'failed', label: 'unreadable' },
  missing: { icon: CircleSlash, tone: 'failed', label: 'missing' },
  rebasing: { icon: TriangleAlert, tone: 'attention', label: 'rebasing' },
  bisecting: { icon: TriangleAlert, tone: 'attention', label: 'bisecting' },
  merging: { icon: TriangleAlert, tone: 'attention', label: 'merging' },
  cherry_picking: { icon: TriangleAlert, tone: 'attention', label: 'cherry-picking' },
  reverting: { icon: TriangleAlert, tone: 'attention', label: 'reverting' },
  applying: { icon: TriangleAlert, tone: 'attention', label: 'applying patches' },
  merged: { icon: GitMerge, tone: 'muted', label: 'merged' },
  changed: { icon: CircleDashed, tone: 'attention', label: 'changed' },
  clean: { icon: CircleCheck, tone: 'ok', label: 'clean' },
}
