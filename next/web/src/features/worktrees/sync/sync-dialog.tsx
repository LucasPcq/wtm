import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { useReparentNow } from '@/features/worktrees/hooks/use-reparent-now'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { openSync, useSyncDialog } from '@/features/worktrees/hooks/use-sync-dialog'
import { SyncProgress } from '@/features/worktrees/sync/components/sync-progress'
import { SyncWizard } from '@/features/worktrees/sync/components/sync-wizard'
import { summaryOf } from '@/features/worktrees/sync/utils/recap'
import { changedSincePlan } from '@/features/worktrees/utils/changed'

// one sync at a time: opened while one runs, or one that failed is still to be read, the dialog shows it
export function SyncDialog({ repo }: { repo: string }) {
  const { open, answers, left, picking, deciding } = useSyncDialog()
  const run = useRun(repo, 'sync')
  const reparentNow = useReparentNow(repo)
  const notice = changedSincePlan(run, { code: 'sync.unsafe', past: 'synced' })
  const close = () => {
    if (notice) run.dismiss()
    useSyncDialog.setState({ open: false })
  }

  // hidden while it ran, the run ends on a toast in the words Done uses; its Details opens the outcome
  const told = (envelope: Applied<'sync'> | undefined, refused: string | undefined) => {
    if (useSyncDialog.getState().open) return
    const summary = summaryOf({ units: [], ...(envelope && { envelope }), ...(refused !== undefined && { refused }) })
    if (succeeded(envelope)) return void toast.success(summary)
    toast.error(summary, { action: { label: 'Details', onClick: () => useSyncDialog.setState({ open: true }) } })
  }

  // a run's next step opens the dialog again on its plan: sync is never a direct run
  const follow = (next: DomainAnswered[]) => {
    run.dismiss()
    openSync(repo, { answers: next, left: [], picking: false, deciding: false })
  }

  const content = () => {
    if (run.progress && !notice)
      return (
        <SyncProgress
          progress={run.progress}
          running={run.running}
          onHide={close}
          onCancel={run.cancel}
          onDone={() => {
            run.dismiss()
            close()
          }}
          onFollow={follow}
        />
      )

    // closing, the wizard is gone at once: its dry runs would plan worktrees already moved
    if (!open) return null
    return (
      <SyncWizard
        key={`${picking}\n${deciding}\n${JSON.stringify(answers)}`}
        repo={repo}
        answers={answers}
        left={left}
        picking={picking}
        deciding={deciding}
        notice={notice}
        onStart={(start) => run.start(start, told)}
        onReparent={(followUp) => void reparentNow([followUp])}
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
