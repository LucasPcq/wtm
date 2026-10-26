import type { ReactNode } from 'react'
import type { MutationCommand } from '@/api/queries/commands'
import { Details } from '@/features/worktrees/components/details'
import { Timeline } from '@/features/worktrees/components/timeline'
import { doneOf, type Progress, took } from '@/features/worktrees/utils/progress'

// a run under way shows its steps; once it ended, the outcome comes first and the steps fold last, as its audit trail
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

  const { total } = doneOf(progress)
  return (
    <Details label={`${total} step${total === 1 ? '' : 's'}`} summary={took(progress)} quiet>
      <Timeline units={progress.units} settled />
      {children}
    </Details>
  )
}
