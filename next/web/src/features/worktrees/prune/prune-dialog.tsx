import { toast } from 'sonner'
import type { Applied } from '@/api/queries/commands'
import { Dialog } from '@/components/ui/dialog'
import { RemovalProgress } from '@/features/worktrees/components/removal-progress'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { usePruneDialog } from '@/features/worktrees/hooks/use-prune-dialog'
import { succeeded, useRun } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { PruneWizard } from '@/features/worktrees/prune/components/prune-wizard'
import { pruneWords, unsafeCode } from '@/features/worktrees/prune/utils/words'
import { changedSincePlan } from '@/features/worktrees/utils/changed'

// one prune at a time: opened while one runs, or one that failed is still to be read, the dialog shows it; what went
// leaves the selection
export function PruneDialog({ repo }: { repo: string }) {
  const { open, worktrees } = usePruneDialog()
  const run = useRun(repo, 'prune')
  const selection = useSelection(repo)
  // the run a changed worktree refused is read once the dialog closes, however it closes
  const notice = changedSincePlan(run, { code: unsafeCode, past: pruneWords.past })
  const close = () => {
    if (notice) run.dismiss()
    usePruneDialog.setState({ open: false })
  }

  // a run that did all it was asked but left something to do next (the kept ones' --force) says so in amber
  const told = (envelope: Applied<'prune'> | undefined, { pruned, asked }: { pruned: number; asked: number }) => {
    if (usePruneDialog.getState().open) return
    const details = { action: { label: 'Details', onClick: () => usePruneDialog.setState({ open: true }) } }
    if (!succeeded(envelope)) return void toast.error(`${pruned} of ${asked} pruned.`, details)
    if (envelope?.warnings.some((warning) => warning.follow_up)) return void toast.warning(`${pruned} pruned.`, details)
    toast.success(`${pruned} pruned.`)
  }

  const content = () => {
    if (run.progress && !notice)
      return (
        <RemovalProgress
          repo={repo}
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
        notice={notice}
        onStart={(start) =>
          run.start(start, (envelope) => {
            const pruned = (envelope?.items ?? []).filter((item) => item.status === 'done').map((item) => item.subject)
            told(envelope, { pruned: pruned.length, asked: start.units.length })
            selection.drop(pruned)
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
