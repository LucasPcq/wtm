import { useQuery } from '@tanstack/react-query'
import { type ReactElement, useState } from 'react'
import { toast } from 'sonner'
import { type Applied, readQuery } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { type CardState, DaemonCard } from '@/features/worktrees/daemon/components/daemon-card'
import { RestartDialog } from '@/features/worktrees/daemon/components/restart-dialog'
import { StopJobsDialog } from '@/features/worktrees/daemon/components/stop-jobs-dialog'
import { useDaemonRuns } from '@/features/worktrees/daemon/hooks/use-daemon-runs'
import { cameBack, comingBack as comingBackOf } from '@/features/worktrees/daemon/utils/rows'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { jobsTodo } from '@/features/worktrees/utils/progress'

// wtm's daemon, the machine's: the connection block opens it; it is never stopped from here, the page would start it
// again at once, so its jobs are stopped instead
const notBack = (envelope: Applied<'daemon-restart'> | undefined) => {
  const count = (envelope?.items[0]?.detail?.back ?? []).filter((job) => !job.back).length
  return count === 1 ? "1 job didn't come back" : `${count} jobs didn't come back`
}

export function DaemonPanel({ repo, children }: { repo: string; children: ReactElement }) {
  const status = useQuery(readQuery(repo, 'daemon-status'))
  const restart = useRun(repo, 'daemon-restart')
  const comingBack = useDaemonRuns((runs) => runs.comingBack)
  const stopping = useRun(repo, 'daemon-stop-jobs').running
  const [asking, setAsking] = useState<'restart' | 'stop'>()
  const detail = status.data?.items[0]?.detail
  const unread = status.error?.message ?? status.data?.error?.message

  const state = (): CardState => {
    if (restart.running) return { kind: 'restarting', back: comingBack }
    if (stopping) return { kind: 'stopping' }
    if (!restart.progress) return { kind: 'read' }
    const envelope = restart.progress.envelope
    const error = envelope?.items[0]?.error
    const message = restart.progress.refused ?? error?.message ?? envelope?.error?.message
    if (!message) return { kind: 'read' }
    // down until a daemon answers again, which the page's own stream starts: then it reads as it stands
    if (error?.code === 'daemon-restart.start_failed' && detail?.running) return { kind: 'read' }
    const down = error?.code === 'daemon-restart.start_failed'
    // it restarted, some jobs short: a warning, not a failure
    if (error?.code === 'daemon-restart.not_back')
      return { kind: 'failed', title: `Restarted · ${notBack(envelope)}`, message, down, tone: 'attention' }
    return { kind: 'failed', title: "The restart didn't finish", message, down, tone: 'failed' }
  }

  const told = (envelope: Applied<'daemon-restart'> | undefined) => {
    if (envelope?.items[0]?.error?.code === 'daemon-restart.not_back') {
      toast.warning(`Restarted wtm's daemon · ${notBack(envelope)}`)
      return
    }
    if (!succeeded(envelope)) {
      toast.error("Couldn't restart wtm's daemon", { description: 'Its panel says why.' })
      return
    }
    const back = envelope?.plan ? cameBack(envelope.plan, envelope.items[0]?.detail?.back ?? []) : 0
    const stacks = (envelope?.plan?.taken_over ?? [])
      .flatMap((repo) => repo.worktrees ?? [])
      .flatMap((wt) => wt.jobs ?? [])
      .filter((job) => job.hold === 'detached').length
    const said = [
      back > 0 && (back === 1 ? '1 job is back' : `${back} jobs are back`),
      stacks > 0 && (stacks === 1 ? 'the stack was taken over' : `${stacks} stacks were taken over`),
    ].filter(Boolean)
    toast.success(said.length > 0 ? `Restarted wtm's daemon · ${said.join('; ')}.` : "Restarted wtm's daemon.")
    restart.dismiss()
  }

  return (
    <>
      <Popover>
        <PopoverTrigger asChild>{children}</PopoverTrigger>
        <PopoverContent side="top" align="start" className="w-96">
          <DaemonCard
            detail={detail}
            unread={unread}
            state={state()}
            onRestart={() => {
              restart.dismiss()
              setAsking('restart')
            }}
            onStopJobs={() => setAsking('stop')}
          />
        </PopoverContent>
      </Popover>

      <Dialog open={asking === 'restart'} onOpenChange={(open) => !open && setAsking(undefined)}>
        <RunDialogContent running={false}>
          {asking === 'restart' && (
            <RestartDialog
              repo={repo}
              onRestart={(fingerprint, plan) => {
                setAsking(undefined)
                useDaemonRuns.setState({ comingBack: comingBackOf(plan) })
                restart.start({ answers: [], fingerprint, units: [jobsTodo('daemon', [])] }, told)
              }}
              onCancel={() => setAsking(undefined)}
            />
          )}
        </RunDialogContent>
      </Dialog>
      <StopJobsDialog
        repo={repo}
        open={asking === 'stop'}
        onClose={() => setAsking(undefined)}
        onShow={() => setAsking('stop')}
      />
    </>
  )
}
