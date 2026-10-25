import type { Applied } from '@/api/queries/commands'
import type { OperationLine } from '@/api/queries/operations'
import type { CleanPlan, DomainItem, DomainPhase } from '@/api/types.gen'

export type Unit = { subject: string; phase?: DomainPhase; line?: string; status?: DomainItem['status'] }

export type Progress = {
  operation?: string
  units: Unit[]
  cancelling?: 'asked' | 'forced'
  envelope?: Applied<'clean'>
  refused?: string
}

export const planned = (plan: CleanPlan): Progress => ({
  units: (plan.worktrees ?? []).map(({ worktree }) => ({ subject: worktree })),
})

export function advance(progress: Progress, line: OperationLine): Progress {
  if (!('subject' in line) || typeof line.subject !== 'string') return progress

  const step = (unit: Unit): Unit => {
    switch (line.type) {
      case 'phase.started':
        return { ...unit, phase: line.phase }
      case 'phase.output':
        return { ...unit, line: line.line }
      case 'unit.finished':
        return { ...unit, status: line.status }
      default:
        return unit
    }
  }

  return { ...progress, units: progress.units.map((unit) => (unit.subject === line.subject ? step(unit) : unit)) }
}

// the envelope's item wins over the events: a worktree skipped once cancelled has no event of its own
export function settled(progress: Progress): Unit[] {
  const items = progress.envelope?.items ?? []
  return progress.units.map((unit) => {
    const status = items.find((item) => item.subject === unit.subject)?.status ?? unit.status
    return status ? { ...unit, status } : unit
  })
}
