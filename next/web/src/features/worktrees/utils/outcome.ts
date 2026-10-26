import type { DomainFollowUp, DomainItem } from '@/api/types.gen'
import type { Tone } from '@/features/worktrees/components/glyph'
import { phaseDuring } from '@/features/worktrees/utils/phases'
import type { Unit } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// what needs the person first comes first: what failed, then what to check, then what went
export const outcomeOrder = ['failed', 'check', 'done'] as const

export type OutcomeKind = (typeof outcomeOrder)[number]

// a row with no tone is a plain next step: nothing went wrong, nothing to check
export type OutcomeTone = Exclude<Tone, 'busy'>

// a fix (followUp) copies from the folded line; a next step (go) and a path only show once unfolded
export type OutcomeRow = {
  key: string
  tone?: OutcomeTone
  title?: string
  message?: string | undefined
  go?: DomainFollowUp | undefined
  path?: string | undefined
  output?: { lines: string[]; log?: string | undefined } | undefined
  followUp?: DomainFollowUp | undefined
}

export type OutcomeGroup = { kind: OutcomeKind; rows: OutcomeRow[] }

export const outputOf = (unit: Unit | undefined) => {
  if (!unit || unit.lines.length === 0) return undefined
  return { lines: unit.lines, log: unit.phases.find((run) => run.state === 'failed')?.log }
}

const stoppedIn = (unit: Unit | undefined) =>
  unit?.phases.find((run) => run.state === 'now' || run.state === 'failed') ??
  unit?.phases.findLast((run) => run.startedAt !== undefined)

export function stoppedMessage(unit: Unit | undefined) {
  const phase = stoppedIn(unit)?.phase
  return phase ? `Cancelled during ${phaseDuring[phase]}` : 'Cancelled before it started'
}

// an item's own warnings name it: two alike on two items stay two
export const itemWarnings = (items: Pick<DomainItem, 'subject' | 'warnings'>[]): OutcomeRow[] =>
  items.flatMap((item) =>
    (item.warnings ?? []).map((warning) => ({
      key: `${item.subject}-${warning.code}-${warning.message}`,
      tone: 'attention' as const,
      title: item.subject,
      message: sentence(warning.message),
      followUp: warning.follow_up,
    })),
  )
