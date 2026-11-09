import { useRef } from 'react'
import { toast } from 'sonner'
import type { Applied, Planned } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { StopProgress } from '@/features/worktrees/components/stop-progress'
import { StopJobsConfirm } from '@/features/worktrees/daemon/components/stop-jobs-confirm'
import { useDaemonRuns } from '@/features/worktrees/daemon/hooks/use-daemon-runs'
import { together } from '@/features/worktrees/daemon/utils/rows'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { unitsOf } from '@/features/worktrees/utils/stop-plan'

export function StopJobsDialog({
  repo,
  open,
  onClose,
  onShow,
}: {
  repo: string
  open: boolean
  onClose: () => void
  onShow: () => void
}) {
  const run = useRun(repo, 'daemon-stop-jobs')
  const ran = useDaemonRuns((runs) => runs.stopping)
  const setRan = (stopping: Planned<'daemon-stop-jobs'> | undefined) => useDaemonRuns.setState({ stopping })
  const hidden = useRef(false)
  const again = run.progress?.envelope?.follow_ups?.some((followUp) => followUp.code === 'daemon-stop-jobs.again')

  // every repository's worktrees, as a stop of worktrees takes them: the progress and the outcome are run down's
  const asStop = (plan: Planned<'daemon-stop-jobs'>) => ({
    worktrees: (plan.repos ?? []).flatMap((repo) => repo.worktrees ?? []),
  })

  // hidden while it stops, its end is said in a toast that opens it again
  const told = (envelope: Applied<'daemon-stop-jobs'> | undefined, refused: string | undefined) => {
    if (!hidden.current) return
    if (succeeded(envelope)) {
      toast.success('Stopped every job. The proxy is still up.')
      return run.dismiss()
    }
    const failed = (envelope?.items ?? []).filter((item) => item.status !== 'done').length
    const said = () => {
      if (failed === 1) return "Jobs didn't stop in 1 worktree."
      if (failed > 1) return `Jobs didn't stop in ${failed} worktrees.`
      return envelope?.error?.message ?? refused ?? 'Nothing stopped.'
    }
    toast.error(said(), {
      action: { label: 'Details', onClick: onShow },
    })
  }

  const content = () => {
    if (run.progress && ran)
      return (
        <StopProgress
          title="Stop every job"
          stopping="Stopping every job"
          ended={({ stopped, total }, failed) => {
            if (stopped === total && failed.length === 0) return `${stopped} stopped. The proxy is still up.`
            const retries = again && failed.length > 0 ? ` Run again retries ${together(failed)}.` : ''
            return `${stopped} of ${total} stopped.${retries}`
          }}
          progress={run.progress}
          running={run.running}
          ran={{ worktrees: [], plan: asStop(ran) }}
          onHide={() => {
            hidden.current = true
            onClose()
          }}
          onDone={() => {
            run.dismiss()
            onClose()
          }}
          onAgain={
            again
              ? () => {
                  run.dismiss()
                  setRan(undefined)
                }
              : undefined
          }
        />
      )
    return (
      <StopJobsConfirm
        repo={repo}
        onStop={(fingerprint, plan) => {
          setRan(plan)
          hidden.current = false
          run.start({ answers: [], fingerprint, units: unitsOf(asStop(plan)) }, told)
        }}
        onCancel={onClose}
      />
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) return
        hidden.current = run.running
        onClose()
      }}
    >
      <RunDialogContent running={run.running}>{content()}</RunDialogContent>
    </Dialog>
  )
}
