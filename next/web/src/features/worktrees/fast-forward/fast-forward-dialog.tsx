import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { FastForwardProgress } from '@/features/worktrees/fast-forward/components/fast-forward-progress'
import { FastForwardWizard } from '@/features/worktrees/fast-forward/components/fast-forward-wizard'
import { useFastForwardDialog } from '@/features/worktrees/hooks/use-fast-forward-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { changedSincePlan } from '@/features/worktrees/utils/changed'

// one fast-forward at a time: opened while one runs, or one that failed is still to be read, the dialog shows it
export function FastForwardDialog({ repo }: { repo: string }) {
  const { open, worktrees, left, picking } = useFastForwardDialog()
  const run = useRun(repo, 'fast-forward')
  const notice = changedSincePlan(run, { code: 'fast-forward.unsafe', past: 'fast-forwarded' })
  const close = () => {
    if (notice) run.dismiss()
    useFastForwardDialog.setState({ open: false })
  }

  // hidden while it ran, the run ends on a toast; its Details opens the outcome
  const told = (envelope: Applied<'fast-forward'> | undefined, asked: number) => {
    if (useFastForwardDialog.getState().open) return
    const moved = (envelope?.items ?? []).filter((item) => item.status === 'done' && item.reason !== 'unchanged')
    if (succeeded(envelope)) return void toast.success(`${moved.length} fast-forwarded.`)
    toast.error(`${moved.length} of ${asked} fast-forwarded.`, {
      action: { label: 'Details', onClick: () => useFastForwardDialog.setState({ open: true }) },
    })
  }

  const content = () => {
    if (run.progress && !notice)
      return (
        <FastForwardProgress
          progress={run.progress}
          running={run.running}
          onHide={close}
          onCancel={run.cancel}
          onDone={() => {
            run.dismiss()
            close()
          }}
        />
      )

    // closing, the wizard is gone at once: its dry runs would plan worktrees already moved
    if (!open) return null
    return (
      <FastForwardWizard
        key={`${picking}\n${worktrees.join('\n')}`}
        repo={repo}
        worktrees={worktrees}
        left={left}
        picking={picking}
        notice={notice}
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
