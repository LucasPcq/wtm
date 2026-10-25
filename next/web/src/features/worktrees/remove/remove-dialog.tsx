import { Dialog, DialogContent } from '@/components/ui/dialog'
import { useRemoveDialog } from '@/features/worktrees/hooks/use-remove-dialog'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { RemoveProgress } from '@/features/worktrees/remove/components/remove-progress'
import { RemoveWizard } from '@/features/worktrees/remove/components/remove-wizard'
import { cn } from '@/lib/utils'

export function RemoveDialog({ repo }: { repo: string }) {
  const { open, worktrees } = useRemoveDialog()
  const run = useRun(repo, 'clean')
  const selection = useSelection(repo)
  const close = () => useRemoveDialog.setState({ open: false })

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent
        className={cn('flex max-h-[90vh] flex-col gap-6 sm:max-w-xl', run.running && 'h-[min(36rem,85vh)]')}
        showCloseButton={!run.running}
      >
        {run.progress ? (
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
      </DialogContent>
    </Dialog>
  )
}
