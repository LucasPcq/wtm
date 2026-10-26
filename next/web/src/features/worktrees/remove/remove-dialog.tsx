import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { RemoveProgress } from '@/features/worktrees/remove/components/remove-progress'
import { RemoveWizard } from '@/features/worktrees/remove/components/remove-wizard'

export function RemoveDialog({ repo }: { repo: string }) {
  const { open, worktrees } = useRemoveDialog()
  const run = useRun(repo, 'clean')
  const selection = useSelection(repo)

  // a run that ended is read once: closed, the next Remove opens on its own worktrees
  const close = () => {
    if (!run.running) run.dismiss()
    useRemoveDialog.setState({ open: false })
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <RunDialogContent running={run.running}>
        {run.progress ? (
          <RemoveProgress
            progress={run.progress}
            running={run.running}
            onHide={close}
            onCancel={run.cancel}
            onDone={close}
          />
        ) : (
          <RemoveWizard
            key={worktrees.join('\n')}
            repo={repo}
            worktrees={worktrees}
            onStart={(start) => {
              selection.clear()
              run.start(start)
            }}
            onCancel={close}
          />
        )}
      </RunDialogContent>
    </Dialog>
  )
}
