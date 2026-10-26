import type { DomainFollowUp } from '@/api/types.gen'
import type { Unit } from '@/features/worktrees/utils/progress'

// what needs the person first comes first: what failed, then what to check, then what went
export const outcomeOrder = ['failed', 'check', 'done'] as const

export type OutcomeKind = (typeof outcomeOrder)[number]

export type OutcomeTone = 'ok' | 'attention' | 'failed' | 'stopped'

export type OutcomeRow = {
  key: string
  tone: OutcomeTone
  title?: string
  message?: string | undefined
  go?: DomainFollowUp | undefined
  path?: string | undefined
  output?: { lines: string[]; log?: string | undefined } | undefined
  followUp?: DomainFollowUp | undefined
}

export type OutcomeGroup = { kind: OutcomeKind; rows: OutcomeRow[] }

// a subject's output, with the log of the step that failed: what explains its failure
export const outputOf = (unit: Unit | undefined) => {
  if (!unit || unit.lines.length === 0) return undefined
  return { lines: unit.lines, log: unit.phases.find((run) => run.state === 'failed')?.log }
}
