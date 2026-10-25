import type { OperationLine } from '@/api/queries/operations'
import type { Applied } from '@/api/queries/commands'
import type { CreatePlan, DomainItem } from '@/api/types.gen'
import type { Phase } from './phases'

export type PhaseRun = {
  phase: Phase
  state: 'todo' | 'now' | 'done' | 'failed'
  startedAt?: string
  endedAt?: string
  log?: string
}

// A Unit is one subject of the operation: its phases, the lines its hooks printed, and how it ended.
export type Unit = { phases: PhaseRun[]; lines: string[]; status?: DomainItem['status'] }

// Progress is a create as it runs: its operation, each subject's unit in the plan's order, whether a cancel was
// asked (then forced), and its envelope once it ended, or why the server refused it.
export type Progress = {
  operation?: string
  units: [string, Unit][]
  cancelling?: 'asked' | 'forced'
  envelope?: Applied<'create'>
  refused?: string
}

const kept = 200

// planned is the progress of plan before it starts: each subject once, in the plan's order, a source branch it
// fast-forwards first, then the phases of each branch it opens, none started. A branch both fast-forwarded and
// opened (checked out as it is, behind origin) is one unit, its fast-forward first.
export function planned(plan: CreatePlan): Progress {
  const units = new Map<string, Phase[]>()
  for (const subject of plan.fast_forward ?? []) units.set(subject, ['fast_forward'])
  for (const branch of plan.branches ?? [])
    if (branch.phases && branch.phases.length > 0)
      units.set(branch.branch, [...(units.get(branch.branch) ?? []), ...branch.phases])
  return {
    units: [...units].map(([subject, phases]): [string, Unit] => [
      subject,
      { phases: phases.map((phase) => ({ phase, state: 'todo' })), lines: [] },
    ]),
  }
}

// advance is progress once line happened; a line of a subject or a phase the plan did not foresee adds it.
export function advance(progress: Progress, line: OperationLine): Progress {
  if (!('subject' in line) || typeof line.subject !== 'string') return progress
  const subject = line.subject
  const units = progress.units.some(([s]) => s === subject)
    ? progress.units
    : [...progress.units, [subject, { phases: [], lines: [] }] as [string, Unit]]
  return {
    ...progress,
    units: units.map(([s, unit]) => [s, s === subject ? step(unit, line) : unit]),
  }
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
    case 'phase.finished': {
      const run = unit.phases.find((p) => p.phase === line.phase)
      return {
        ...unit,
        phases: upsert(unit.phases, {
          ...run,
          phase: line.phase,
          state: line.failed ? 'failed' : 'done',
          endedAt: line.ts,
        }),
      }
    }
    case 'phase.output':
      return { ...unit, lines: [...unit.lines, line.line].slice(-kept) }
    case 'unit.finished':
      return { ...unit, status: line.status }
    default:
      return unit
  }
}

function upsert(phases: PhaseRun[], run: PhaseRun): PhaseRun[] {
  return phases.some((p) => p.phase === run.phase)
    ? phases.map((p) => (p.phase === run.phase ? run : p))
    : [...phases, run]
}

// current is the phase under way in progress, else the next one to run, none once every one ended.
export function current(
  progress: Progress,
): { subject: string; run: PhaseRun; index: number; total: number } | undefined {
  const all = progress.units.flatMap(([subject, unit]) => unit.phases.map((run) => ({ subject, run })))
  const index = all.findIndex(({ run }) => run.state === 'now' || run.state === 'todo')
  const found = all[index]
  return found && { ...found, index, total: all.length }
}

// done counts the phases that ended, of all of them.
export function doneOf(progress: Progress): { done: number; total: number } {
  const all = progress.units.flatMap(([, unit]) => unit.phases)
  return { done: all.filter((run) => run.state === 'done' || run.state === 'failed').length, total: all.length }
}

// seconds is how long a phase took, to the tenth, nothing under a second: the time of what made the wait.
export function seconds(run: PhaseRun): string {
  const took = run.startedAt && run.endedAt ? Date.parse(run.endedAt) - Date.parse(run.startedAt) : 0
  return took < 1000 ? '' : `${(took / 1000).toFixed(1)} s`
}

// took is how long the whole run took, from its first phase started to its last ended, to the tenth.
export function took(progress: Progress): string {
  const runs = progress.units.flatMap(([, unit]) => unit.phases)
  const starts = runs.flatMap((run) => (run.startedAt ? [Date.parse(run.startedAt)] : []))
  const ends = runs.flatMap((run) => (run.endedAt ? [Date.parse(run.endedAt)] : []))
  if (starts.length === 0 || ends.length === 0) return ''
  return `${((Math.max(...ends) - Math.min(...starts)) / 1000).toFixed(1)} s`
}
