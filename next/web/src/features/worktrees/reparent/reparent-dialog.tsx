import { Dialog } from '@/components/ui/dialog'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { useReparentDialog } from '@/features/worktrees/hooks/use-reparent-dialog'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { ReparentProgress } from '@/features/worktrees/reparent/components/reparent-progress'
import { ReparentWizard } from '@/features/worktrees/reparent/components/reparent-wizard'

// one reparent at a time: opened while one runs, or one that failed is still to be read, the dialog shows it
export function ReparentDialog({ repo }: { repo: string }) {
  const { open, worktrees, left, picking, to } = useReparentDialog()
  const run = useRun(repo, 'reparent')
  const close = () => {
    // an outcome shown is read: closed, it no longer holds the next reparent back
    if (run.progress && !run.running) run.dismiss()
    useReparentDialog.setState({ open: false })
  }

  const content = () => {
    if (run.progress)
      return (
        <ReparentProgress
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

    if (!open) return null
    return (
      <ReparentWizard
        key={`${picking}\n${to}\n${worktrees.join('\n')}`}
        repo={repo}
        worktrees={worktrees}
        left={left}
        picking={picking}
        to={to}
        onStart={(start) => run.start(start)}
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
