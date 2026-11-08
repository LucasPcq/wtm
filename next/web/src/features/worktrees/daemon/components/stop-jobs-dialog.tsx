import { useState } from 'react'
import type { Planned } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { StopProgress } from '@/features/worktrees/components/stop-progress'
import { StopJobsConfirm } from '@/features/worktrees/daemon/components/stop-jobs-confirm'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { unitsOf } from '@/features/worktrees/utils/stop-plan'

// every repository's worktrees, as a stop of worktrees takes them: the progress and the outcome are run down's
const asStop = (plan: Planned<'daemon-stop-jobs'>) => ({
  worktrees: (plan.repos ?? []).flatMap((repo) => repo.worktrees ?? []),
})

export function StopJobsDialog({ repo, open, onClose }: { repo: string; open: boolean; onClose: () => void }) {
  const run = useRun(repo, 'daemon-stop-jobs')
  const [ran, setRan] = useState<Planned<'daemon-stop-jobs'>>()

  const content = () => {
    if (run.progress && ran)
      return (
        <StopProgress
          title="Stop every job"
          stopping="Stopping every job"
          progress={run.progress}
          running={run.running}
          ran={{ worktrees: [], plan: asStop(ran) }}
          onHide={onClose}
          onDone={() => {
            run.dismiss()
            onClose()
          }}
          onAgain={() => {
            run.dismiss()
            setRan(undefined)
          }}
        />
      )
    return (
      <StopJobsConfirm
        repo={repo}
        onStop={(fingerprint, plan) => {
          setRan(plan)
          run.start({ answers: [], fingerprint, units: unitsOf(asStop(plan)) })
        }}
        onCancel={onClose}
      />
    )
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <RunDialogContent running={run.running}>{content()}</RunDialogContent>
    </Dialog>
  )
}
