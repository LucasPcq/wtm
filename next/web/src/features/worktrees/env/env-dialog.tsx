import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { RunDialogContent } from '@/components/run-dialog-content'
import { Dialog } from '@/components/ui/dialog'
import { EnvProgress } from '@/features/worktrees/env/components/env-progress'
import { EnvWizard } from '@/features/worktrees/env/components/env-wizard'
import { openEnv, useEnvDialog } from '@/features/worktrees/hooks/use-env-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { openRunStart } from '@/features/worktrees/hooks/use-run-start-dialog'
import { subjectsOf } from '@/features/worktrees/utils/actions'

// one reconcile at a time: opened while one runs, or one that failed is still to be read, the dialog shows it
export function EnvDialog({ repo }: { repo: string }) {
  const { open, worktree, again } = useEnvDialog()
  const run = useRun(repo, 'env')
  const close = () => useEnvDialog.setState({ open: false })

  // hidden while it ran, the run ends on a toast; its Details opens the outcome
  const told = (envelope: Applied<'env'> | undefined, asked: number) => {
    if (useEnvDialog.getState().open) return
    const written = (envelope?.items ?? []).filter((item) => item.status === 'done' && item.reason !== 'unchanged')
    if (succeeded(envelope)) return void toast.success(`${written.length} reconciled.`)
    toast.error(`${written.length} of ${asked} reconciled.`, {
      action: { label: 'Details', onClick: () => useEnvDialog.setState({ open: true }) },
    })
  }

  const restart = (at: string, job: string) => {
    close()
    openRunStart(repo, { worktree: at, job, restart: true, step: 'confirm' })
  }

  const content = () => {
    if (run.progress)
      return (
        <EnvProgress
          progress={run.progress}
          running={run.running}
          onHide={close}
          onCancel={run.cancel}
          onRestart={restart}
          onAgain={(retry) => {
            run.dismiss()
            openEnv(repo, { worktree: '', again: subjectsOf(retry) })
          }}
          onDone={() => {
            run.dismiss()
            close()
          }}
        />
      )

    if (!open) return null
    return (
      <EnvWizard
        key={again ? again.join(',') : worktree}
        repo={repo}
        worktree={worktree}
        again={again}
        onStart={(start) => run.start(start, (envelope) => told(envelope, start.units.length))}
        onCancel={close}
      />
    )
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <RunDialogContent running={run.running}>{content()}</RunDialogContent>
    </Dialog>
  )
}
