import { Button } from '@/components/ui/button'
import { Dialog, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunDialogContent } from '@/features/worktrees/components/run-dialog-content'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { setRunStopDialog, useRunStopDialog } from '@/features/worktrees/hooks/use-run-stop-dialog'
import { stoppedTold } from '@/features/worktrees/utils/job-told'

// a job's stop runs at once from its panel: this is its outcome, read from its toast's Details
export function RunStopDialog({ repo }: { repo: string }) {
  const { open, ran } = useRunStopDialog(repo)
  const run = useRun(repo, 'run-stop')
  const envelope = run.progress?.envelope
  const told = stoppedTold(ran, envelope, run.progress?.refused, undefined)

  const done = () => {
    if (!run.running) run.dismiss()
    setRunStopDialog(repo, { open: false })
  }

  const description = () => {
    if (run.running) return `Stopping ${ran.job} in ${ran.worktree}…`
    return told.tone === 'error' ? `${ran.job} didn’t stop in ${ran.worktree}.` : `${told.title}.`
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && done()}>
      <RunDialogContent running={false}>
        <DialogHeader>
          <DialogTitle>Stop {ran.job}</DialogTitle>
          <DialogDescription>{description()}</DialogDescription>
        </DialogHeader>
        <DialogBand />
        <DialogScroll>
          {!run.running && told.tone === 'error' && <RunFailure message={told.description} />}
        </DialogScroll>
        <DialogFooter>
          <Button onClick={done}>Done</Button>
        </DialogFooter>
      </RunDialogContent>
    </Dialog>
  )
}
