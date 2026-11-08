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
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { jobsTodo } from '@/features/worktrees/utils/progress'

// wtm's daemon, the machine's: the connection block opens it; it is never stopped from here, the page would start it
// again at once, so its jobs are stopped instead
export function DaemonPanel({ repo, children }: { repo: string; children: ReactElement }) {
  const status = useQuery(readQuery(repo, 'daemon-status'))
  const restart = useRun(repo, 'daemon-restart')
  const [asking, setAsking] = useState<'restart' | 'stop'>()
  const detail = status.data?.items[0]?.detail

  const failure = () => {
    if (restart.running || !restart.progress) return undefined
    const item = restart.progress.envelope?.items[0]
    return restart.progress.refused ?? item?.error?.message ?? restart.progress.envelope?.error?.message
  }

  const state = (): CardState => {
    if (restart.running) return { kind: 'restarting' }
    const message = failure()
    return message ? { kind: 'failed', message } : { kind: 'read' }
  }

  const told = (envelope: Applied<'daemon-restart'> | undefined) => {
    if (!succeeded(envelope)) return
    const back = (envelope?.items[0]?.detail?.back ?? []).filter((job) => job.back).length
    const stacks = (envelope?.plan?.taken_over ?? [])
      .flatMap((r) => r.worktrees ?? [])
      .flatMap((w) => w.jobs ?? []).length
    const taken = () => {
      if (stacks === 0) return ''
      return stacks === 1 ? '; the stack was taken over' : `; ${stacks} stacks were taken over`
    }
    toast.success(`Restarted wtm's daemon · ${back === 1 ? '1 job is' : `${back} jobs are`} back${taken()}.`)
    restart.dismiss()
  }

  return (
    <>
      <Popover>
        <PopoverTrigger asChild>{children}</PopoverTrigger>
        <PopoverContent side="top" align="start" className="w-96">
          <DaemonCard
            detail={detail}
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
              onRestart={(fingerprint) => {
                setAsking(undefined)
                restart.start({ answers: [], fingerprint, units: [jobsTodo('daemon', [])] }, told)
              }}
              onCancel={() => setAsking(undefined)}
            />
          )}
        </RunDialogContent>
      </Dialog>
      <StopJobsDialog repo={repo} open={asking === 'stop'} onClose={() => setAsking(undefined)} />
    </>
  )
}
