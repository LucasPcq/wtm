import { CircleCheck, CircleDashed, CircleSlash, CircleX, GitMerge, TriangleAlert, type LucideIcon } from 'lucide-react'
import type { DomainFollowUp, WorktreeCondition } from '@/api/types.gen'
import type { Tone } from './glyph'

// What a read says of one worktree: its condition and its actions, both as the core decided them.
type Said = { condition: WorktreeCondition; actions: DomainFollowUp[] | null }

// overlaid is a worktree as the screen shows it from its live read and what GitHub said of it when last asked: the
// live read's condition and actions, the core's own, but a pull request GitHub says is merged wins over changed and
// clean, with that read's actions. It is the web's one rule over two reads, written here only.
export function overlaid(live: Said, github: Said | undefined): Said {
  const merged = github?.condition === 'merged' && (live.condition === 'changed' || live.condition === 'clean')
  return merged
    ? { condition: 'merged', actions: github.actions }
    : { condition: live.condition, actions: live.actions }
}

// How each condition the core decides looks, wherever a worktree's condition shows.
export const conditionLook: Record<WorktreeCondition, { icon: LucideIcon; tone: Tone; label: string }> = {
  no_worktree: { icon: CircleDashed, tone: 'muted', label: 'branch without a worktree' },
  unreadable: { icon: CircleX, tone: 'failed', label: 'unreadable' },
  missing: { icon: CircleSlash, tone: 'failed', label: 'missing' },
  rebasing: { icon: TriangleAlert, tone: 'attention', label: 'rebasing' },
  merged: { icon: GitMerge, tone: 'muted', label: 'merged' },
  changed: { icon: CircleDashed, tone: 'attention', label: 'changed' },
  clean: { icon: CircleCheck, tone: 'ok', label: 'clean' },
}
