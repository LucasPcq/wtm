import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RemovalProgress } from '@/features/worktrees/components/removal-progress'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { usePruneDialog } from '@/features/worktrees/hooks/use-prune-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { PruneWizard } from '@/features/worktrees/prune/components/prune-wizard'
import { pruneWords } from '@/features/worktrees/prune/utils/words'

// one prune at a time: opened while one runs, or one that failed is still to be read, the dialog shows it; what went
// leaves the selection
export function PruneDialog({ repo }: { repo: string }) {
  const { open, worktrees } = usePruneDialog()
  const run = useRun(repo, 'prune')
  const selection = useSelection(repo)
  const close = () => usePruneDialog.setState({ open: false })

  const told = (envelope: Applied<'prune'> | undefined, asked: number) => {
    if (usePruneDialog.getState().open) return
    const pruned = (envelope?.items ?? []).filter((item) => item.status === 'done').length
    if (succeeded(envelope)) return void toast.success(`${pruned} pruned.`)
    toast.error(`${pruned} of ${asked} pruned.`, {
      action: { label: 'Details', onClick: () => usePruneDialog.setState({ open: true }) },
    })
  }

  const content = () => {
    if (run.progress)
      return (
        <RemovalProgress
          words={pruneWords}
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

    // closing, the wizard is gone at once: its dry runs would plan worktrees already pruned
    if (!open) return null
    return (
      <PruneWizard
        key={worktrees.join('\n')}
        repo={repo}
        worktrees={worktrees}
        onStart={(start) =>
          run.start(start, (envelope) => {
            told(envelope, start.units.length)
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
