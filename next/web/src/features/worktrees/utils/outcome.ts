import type { DomainFollowUp, DomainItem, DomainWarning } from '@/api/types.gen'
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
  // what to do, in prose, once the row opens
  note?: string | undefined
  output?: { lines: string[]; log?: string | undefined } | undefined
  followUp?: DomainFollowUp | undefined
  // a fix the web runs: a button in place of the command to copy, named by actLabel when the row names its own
  act?: (() => void) | undefined
  actLabel?: string | undefined
}

// the follow-ups of one command the web runs at once: each its own button, and all of them at once,
// busy while one of them runs: one run of the command at a time
export type Act = {
  command: string
  label: string
  all: string
  busy: boolean
  run: (followUps: DomainFollowUp[]) => void
}

// label: a group named its own way (sync's "Couldn't push", a second done group "Fast-forwarded"); action: one button
// on its label line for every row of it
export type OutcomeGroup = {
  kind: OutcomeKind
  rows: OutcomeRow[]
  label?: string | undefined
  action?: { label: string; run: () => void }
}

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

export const planWide = (warning: DomainWarning) => !warning.params?.worktree && !warning.params?.branch

// an item's own warnings name it: two alike on two items stay two. A warning a screen words its own way keeps the core's
// text behind its chevron
export const itemWarnings = (
  items: Pick<DomainItem, 'subject' | 'warnings'>[],
  words?: (warning: DomainWarning) => string,
): OutcomeRow[] =>
  items.flatMap((item) =>
    (item.warnings ?? []).map((warning) => {
      const message = words ? words(warning) : sentence(warning.message)
      return {
        key: `${item.subject}-${warning.code}-${warning.message}`,
        tone: 'attention' as const,
        title: item.subject,
        message,
        output: message === sentence(warning.message) ? undefined : { lines: [warning.message] },
        followUp: warning.follow_up,
      }
    }),
  )
