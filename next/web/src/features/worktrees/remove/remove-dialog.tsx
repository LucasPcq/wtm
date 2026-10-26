import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { RemoveProgress } from '@/features/worktrees/remove/components/remove-progress'
import { RemoveWizard } from '@/features/worktrees/remove/components/remove-wizard'

// one removal at a time: opened while one runs, or one that failed is still to be read, the dialog shows it, and the
// rows checked wait; a run that removed every worktree it was asked takes the rows it opened on out of the selection,
// kept ones included, one that removed some takes those out, and rows checked since stay
export function RemoveDialog({ repo }: { repo: string }) {
  const { open, worktrees } = useRemoveDialog()
  const run = useRun(repo, 'clean')
  const selection = useSelection(repo)
  const close = () => useRemoveDialog.setState({ open: false })

  const content = () => {
    if (run.progress)
      return (
        <RemoveProgress
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

    // closing, the wizard is gone at once: its dry runs would plan worktrees already removed
    if (!open) return null
    return (
      <RemoveWizard
        key={worktrees.join('\n')}
        repo={repo}
        worktrees={worktrees}
        onStart={(start) =>
          run.start(start, (envelope) => {
            if (succeeded(envelope)) return selection.drop(worktrees)
            selection.drop((envelope?.items ?? []).filter((item) => item.status === 'done').map((item) => item.subject))
          })
        }
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
