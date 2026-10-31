import type { ReactNode } from 'react'
import type { MutationCommand } from '@/api/queries/commands'
import { Details } from '@/features/worktrees/components/details'
import { Timeline } from '@/features/worktrees/components/timeline'
import { doneOf, type Progress, took } from '@/features/worktrees/utils/progress'
import { plural } from '@/lib/plural'

export function RunSteps<C extends MutationCommand>({
  progress,
  running,
  children,
}: {
  progress: Progress<C>
  running: boolean
  children?: ReactNode
}) {
  if (running) return <Timeline units={progress.units} />
  // refused before its apply, a run has no step to show
  if (!progress.units.some((unit) => unit.phases.some((run) => run.state !== 'todo'))) return null

  const { total } = doneOf(progress)
  return (
    <Details label={plural(total, 'step')} summary={took(progress)} quiet>
      <Timeline units={progress.units} settled />
      {children}
    </Details>
  )
}
