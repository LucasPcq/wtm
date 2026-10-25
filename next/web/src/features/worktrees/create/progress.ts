import type { Applied } from '@/api/queries/commands'
import type { OperationLine } from '@/api/queries/operations'
import type { CreatePlan, DomainItem, DomainPhase } from '@/api/types.gen'

export type PhaseRun = {
  phase: DomainPhase
  state: 'todo' | 'now' | 'done' | 'failed'
  startedAt?: string
  endedAt?: string
  log?: string
}

export type Unit = { subject: string; phases: PhaseRun[]; lines: string[]; status?: DomainItem['status'] }

export type Progress = {
  operation?: string
  units: Unit[]
  cancelling?: 'asked' | 'forced'
  envelope?: Applied<'create'>
  refused?: string
}

const kept = 200

// planned is plan before it starts, each subject once in the plan's order: a branch both fast-forwarded and opened
// (checked out as it is, behind origin) is one unit, its fast-forward first.
export function planned(plan: CreatePlan): Progress {
  const phases = new Map<string, DomainPhase[]>()
  for (const subject of plan.fast_forward ?? []) phases.set(subject, ['fast_forward'])
  for (const { branch, phases: opened } of plan.branches ?? [])
    if (opened && opened.length > 0) phases.set(branch, [...(phases.get(branch) ?? []), ...opened])

  return {
    units: [...phases].map(([subject, of]) => ({
      subject,
      phases: of.map((phase) => ({ phase, state: 'todo' })),
      lines: [],
    })),
  }
}

// advance is progress once line happened; a subject or a phase the plan did not foresee is added, not dropped.
export function advance(progress: Progress, line: OperationLine): Progress {
  if (!('subject' in line) || typeof line.subject !== 'string') return progress

  const subject = line.subject
  const units = progress.units.some((unit) => unit.subject === subject)
    ? progress.units
    : [...progress.units, { subject, phases: [], lines: [] }]
  return { ...progress, units: units.map((unit) => (unit.subject === subject ? step(unit, line) : unit)) }
}

function step(unit: Unit, line: OperationLine): Unit {
  switch (line.type) {
    case 'phase.started':
      return {
        ...unit,
        phases: upsert(unit.phases, {
          phase: line.phase,
          state: 'now',
          startedAt: line.ts,
          ...(line.log && { log: line.log }),
        }),
      }
    case 'phase.finished':
      return {
        ...unit,
        phases: upsert(unit.phases, {
          ...unit.phases.find((run) => run.phase === line.phase),
          phase: line.phase,
          state: line.failed ? 'failed' : 'done',
          endedAt: line.ts,
        }),
      }
    case 'phase.output':
      return { ...unit, lines: [...unit.lines, line.line].slice(-kept) }
    case 'unit.finished':
      return { ...unit, status: line.status }
    default:
      return unit
  }
}

const upsert = (phases: PhaseRun[], run: PhaseRun) =>
  phases.some((p) => p.phase === run.phase) ? phases.map((p) => (p.phase === run.phase ? run : p)) : [...phases, run]

export function current(progress: Progress) {
  const all = progress.units.flatMap((unit) => unit.phases.map((run) => ({ subject: unit.subject, run })))
  const index = all.findIndex(({ run }) => run.state === 'now' || run.state === 'todo')
  const found = all[index]
  return found && { ...found, index, total: all.length }
}

export function doneOf(progress: Progress) {
  const all = progress.units.flatMap((unit) => unit.phases)
  return { done: all.filter((run) => run.state === 'done' || run.state === 'failed').length, total: all.length }
}

// seconds is how long a phase took, nothing under a second: the time of what made the wait.
export function seconds(run: PhaseRun): string {
  const took = run.startedAt && run.endedAt ? Date.parse(run.endedAt) - Date.parse(run.startedAt) : 0
  return took < 1000 ? '' : `${(took / 1000).toFixed(1)} s`
}

export function took(progress: Progress): string {
  const runs = progress.units.flatMap((unit) => unit.phases)
  const starts = runs.flatMap((run) => (run.startedAt ? [Date.parse(run.startedAt)] : []))
  const ends = runs.flatMap((run) => (run.endedAt ? [Date.parse(run.endedAt)] : []))
  if (starts.length === 0 || ends.length === 0) return ''
  return `${((Math.max(...ends) - Math.min(...starts)) / 1000).toFixed(1)} s`
}

// ending is how a run that ended went: cancelled, though only the plan was under way (no item then), failed with
// why, or done.
export function ending(progress: Progress): { cancelled: boolean; failure: string | undefined } {
  const envelope = progress.envelope
  const cancelled =
    envelope?.error?.kind === 'cancelled' || (envelope?.items.some((item) => item.status === 'cancelled') ?? false)
  if (cancelled) return { cancelled, failure: undefined }

  const failure =
    progress.refused ?? envelope?.error?.message ?? envelope?.items.find((item) => item.error)?.error?.message
  return { cancelled, failure }
}
